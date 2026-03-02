// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/log/logtest"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"github.com/medxops/trazr-gen/internal/common"
)

type worker struct {
	running            *atomic.Bool    // pointer to shared flag that indicates it's time to stop the test
	numLogs            int             // how many logs the worker has to generate (only when duration==0)
	body               string          // the body of the log
	severityNumber     string          // the severityNumber of the log (string, for templating)
	severityText       string          // the severityText of the log
	totalDuration      time.Duration   // how long to run the test for (overrides `numLogs`)
	limitPerSecond     rate.Limit      // how many logs per second to generate
	wg                 *sync.WaitGroup // notify when done
	logger             *zap.Logger     // logger
	index              int             // worker index
	traceID            string          // traceID string
	spanID             string          // spanID string
	logsCounter        *int64          // pointer to shared logs counter
	progressCb         func(string)    // optional callback for terminal output
	progressCh         chan struct{}   // channel for centralized progress reporting
	timeBoundSessionID string          // cached session ID for time-bound attributes to ensure consistency
}

// Helper to convert []attribute.KeyValue to []log.KeyValue
func attrToLogKeyValue(attrs []attribute.KeyValue) []log.KeyValue {
	return common.AttrToLogKeyValue(attrs)
}

// GenerateTrazrTimeBoundName generates a deterministic unique name based on the mock seed
func GenerateTrazrTimeBoundName(mockSeed int64) string {
	if mockSeed == 0 {
		// If no seed, use current timestamp
		mockSeed = time.Now().UnixNano()
	}

	// Create a deterministic hash from the seed
	seedStr := strconv.FormatInt(mockSeed, 10)
	hash := sha256.Sum256([]byte(seedStr))
	hashHex := hex.EncodeToString(hash[:8]) // Use first 8 bytes for shorter name

	return "session-" + hashHex
}

// generateRandomDiffAttribute adds a random attribute for baseline testing
func generateRandomDiffAttribute() log.KeyValue {
	// Generate a random UUID-like string
	diffID := fmt.Sprintf("diff-%x", rand.Uint64())
	return log.String("trazr.diff.id", diffID)
}

// generateRandomDiffBody modifies the log body with a random suffix
func generateRandomDiffBody(originalBody string) string {
	// Generate a random suffix
	diffSuffix := fmt.Sprintf(" [diff-%x]", rand.Uint64())
	return originalBody + diffSuffix
}

// generateRandomDiffSeverity changes the severity to a random valid level
func generateRandomDiffSeverity() (int32, string) {
	// Valid severity levels: 1-24
	severities := []struct {
		number int32
		text   string
	}{
		{1, "Trace"},
		{5, "Debug"},
		{9, "Info"},
		{13, "Warn"},
		{17, "Error"},
		{21, "Fatal"},
		{24, "Panic"},
	}

	// Pick a random severity
	randomIndex := rand.IntN(len(severities))
	return severities[randomIndex].number, severities[randomIndex].text
}

// selectRandomDiffType selects one random diff type from the enabled types
func selectRandomDiffType(enabledTypes []string) string {
	if len(enabledTypes) == 0 {
		return "attribute" // fallback
	}
	randomIndex := rand.IntN(len(enabledTypes))
	return enabledTypes[randomIndex]
}

func (w worker) reportProgressf(format string, args ...any) {
	if w.progressCb != nil {
		w.progressCb(fmt.Sprintf(format, args...))
	}
}

// logData holds all the data needed to create a log record
type logData struct {
	body           string
	attrs          []log.KeyValue
	severityNumber int32
	severityText   string
	severityEnum   log.Severity
	traceID        trace.TraceID
	spanID         trace.SpanID
}

// processTraceAndSpanIDs processes trace and span IDs from strings
func (w worker) processTraceAndSpanIDs() (trace.TraceID, trace.SpanID) {
	var tid trace.TraceID
	var sid trace.SpanID

	if w.spanID != "" {
		if b, err := hex.DecodeString(w.spanID); err == nil {
			sid = trace.SpanID(b)
		}
	}
	if w.traceID != "" {
		if b, err := hex.DecodeString(w.traceID); err == nil {
			tid = trace.TraceID(b)
		}
	}

	return tid, sid
}

// processLogBody processes the log body with mock data templating
func (w worker) processLogBody(cfg *Config) (string, bool) {
	body := w.body
	logBodyExpanded := false

	if cfg.MockData {
		if expanded, expandErr := common.ProcessMockTemplate(body, nil); expandErr == nil && expanded != body {
			body = expanded
			logBodyExpanded = true
		}
	}

	return body, logBodyExpanded
}

// processAttributes processes telemetry attributes and adds mock data markers
// Returns both attribute.KeyValue (for pretty printing) and []log.KeyValue (for record creation)
func (w worker) processAttributes(cfg *Config, logBodyExpanded bool) ([]attribute.KeyValue, []log.KeyValue, error) {
	attrKVs, err := cfg.GetTelemetryAttrWithMockMarker()
	if err != nil {
		return nil, nil, err
	}

	// Add mock data marker if body was expanded
	if logBodyExpanded {
		attrKVs = w.addMockDataMarker(attrKVs, "log-body")
	}

	return attrKVs, attrToLogKeyValue(attrKVs), nil
}

// addMockDataMarker adds or updates the mock data marker
func (w worker) addMockDataMarker(attrKVs []attribute.KeyValue, marker string) []attribute.KeyValue {
	found := false
	for i, attr := range attrKVs {
		if attr.Key == "trazrgen.mock.data" {
			val := attr.Value.AsString()
			val += "," + marker
			attrKVs[i] = attribute.String("trazrgen.mock.data", val)
			found = true
			break
		}
	}
	if !found {
		attrKVs = append(attrKVs, attribute.String("trazrgen.mock.data", marker))
	}
	return attrKVs
}

// addTimeBoundAttributes adds time-bound attributes to the log
func (w *worker) addTimeBoundAttributes(cfg *Config, attrs []log.KeyValue, logIndex int64, totalLogs int) []log.KeyValue {
	if !cfg.TrazrTimeBound {
		return attrs
	}

	// For logs > 10, add overlapping batch names
	if totalLogs > 10 {
		// Batch 1: logs 0-14 (first 15 logs)
		if logIndex >= 0 && logIndex <= 14 {
			if logIndex == 0 {
				attrs = append(attrs, log.String("trazr.run.start", "batch-1"))
			}
			if logIndex == 14 {
				attrs = append(attrs, log.String("trazr.run.stop", "batch-1"))
			}
		}

		// Batch 2: logs 15-24 (next 10 logs, if available)
		if logIndex >= 15 && logIndex <= 24 {
			if logIndex == 15 {
				attrs = append(attrs, log.String("trazr.run.start", "batch-2"))
			}
			if logIndex == 24 || logIndex == int64(totalLogs-1) {
				attrs = append(attrs, log.String("trazr.run.stop", "batch-2"))
			}
		}

		// Batch 3: logs 5-19 (overlaps both batch 1 and batch 2)
		if logIndex >= 5 && logIndex <= 19 {
			if logIndex == 5 {
				attrs = append(attrs, log.String("trazr.run.start", "batch-3"))
			}
			if logIndex == 19 {
				attrs = append(attrs, log.String("trazr.run.stop", "batch-3"))
			}
		}
	} else {
		// For 10 or fewer logs, use original behavior
		if logIndex == 0 {
			attrs = append(attrs, log.String("trazr.run.start", w.timeBoundSessionID))
		}
		if totalLogs > 0 && logIndex == int64(totalLogs-1) {
			attrs = append(attrs, log.String("trazr.run.stop", w.timeBoundSessionID))
		}
	}

	return attrs
}

// addRandomDiff applies random diff modifications to the log data
func (w worker) addRandomDiff(cfg *Config, data *logData, logIndex int64) {
	if !cfg.TrazrRandomDiff || !cfg.TrazrTimeBound || logIndex != 0 {
		return
	}

	diffType := selectRandomDiffType(cfg.TrazrRandomDiffTypes)

	switch diffType {
	case "attribute":
		data.attrs = append(data.attrs, generateRandomDiffAttribute())
	case "body":
		data.body = generateRandomDiffBody(data.body)
	case "severity":
		data.severityNumber, data.severityText = generateRandomDiffSeverity()
		data.severityEnum = log.Severity(data.severityNumber)
	}
}

// processSeverity processes severity with mock data templating and random diff support
func (w worker) processSeverity(cfg *Config, data *logData) {
	severityNumberStr := w.severityNumber

	// Process mock data templating for severity
	if cfg.MockData && len(severityNumberStr) > 0 && (strings.Contains(severityNumberStr, "{{") && strings.Contains(severityNumberStr, "}}")) {
		if parsed, parseErr := common.ProcessMockTemplate(severityNumberStr, nil); parseErr == nil {
			severityNumberStr = parsed
		}
	}

	// Parse and validate severity
	severityNumberInt64, err := strconv.ParseInt(severityNumberStr, 10, 32)
	if err != nil || severityNumberInt64 < 1 || severityNumberInt64 > 24 {
		severityNumberInt64 = 9 // fallback to Info
	}

	// severityNumberInt64 is already constrained to 1-24, so safe to convert to int32
	safeSeverityNumberInt := int32(severityNumberInt64)

	mappedSeverityText := common.SeverityTextFromNumber(safeSeverityNumberInt)

	// Update data if not already set by random diff
	if data.severityNumber == 0 {
		data.severityNumber = safeSeverityNumberInt
		data.severityText = mappedSeverityText
		data.severityEnum = log.Severity(safeSeverityNumberInt)
	}
}

// createLogRecord creates a log record from the processed data
func (w worker) createLogRecord(data *logData, res *resource.Resource) sdklog.Record {
	rf := logtest.RecordFactory{
		Timestamp:         time.Now(),
		ObservedTimestamp: time.Now(),
		Severity:          data.severityEnum,
		SeverityText:      data.severityText,
		Body:              log.StringValue(data.body),
		Attributes:        data.attrs,
		TraceID:           data.traceID,
		SpanID:            data.spanID,
		Resource:          res,
		DroppedAttributes: 1,
	}

	return rf.NewRecord()
}

func (w worker) simulateLogs(ctx context.Context, cfg *Config, res *resource.Resource, exporter sdklog.Exporter) {
	limiter := rate.NewLimiter(w.limitPerSecond, 1)
	var i int64

	for w.running.Load() && ctx.Err() == nil {
		// Rate limit at the top of the loop so output pacing is visible
		if err := limiter.Wait(ctx); err != nil {
			if ctx.Err() != nil {
				w.logger.Info("context cancelled during rate limiting, stopping")
				break
			}
			w.reportProgressf("Limiter wait failed: %v", err)
			w.logger.Fatal("limiter wait failed, retry", zap.Error(err))
		}

		// Process trace and span IDs
		tid, sid := w.processTraceAndSpanIDs()

		// Process log body with mock data templating
		body, logBodyExpanded := w.processLogBody(cfg)

		// If using the default body, append a running number (Log Message #<n>)
		// Only do this when the body wasn't expanded via mock templating.
		if !logBodyExpanded {
			// Treat default body case-insensitively to be resilient to minor casing differences
			if strings.EqualFold(strings.TrimSpace(body), "Log message") {
				body = fmt.Sprintf("Log Message #%d", i+1)
			}
		}

		// Process attributes
		origAttrs, attrs, err := w.processAttributes(cfg, logBodyExpanded)
		if err != nil {
			w.reportProgressf("Failed to process telemetry attributes: %v", err)
			w.logger.Fatal("failed to process telemetry attributes", zap.Error(err))
		}

		// Add time-bound attributes
		attrs = w.addTimeBoundAttributes(cfg, attrs, i, w.numLogs)

		// Initialize log data
		data := &logData{
			body:    body,
			attrs:   attrs,
			traceID: tid,
			spanID:  sid,
		}

		// Add random diff modifications
		w.addRandomDiff(cfg, data, i)

		// Process severity
		w.processSeverity(cfg, data)

		// Create log record
		logRecord := w.createLogRecord(data, res)

		// Optionally print detailed output to terminal
		if cfg.TerminalOutput && cfg.PrintDetails {
			w.printLogDetails(cfg, data, origAttrs, data.attrs, res)
		}

		if err := exporter.Export(ctx, []sdklog.Record{logRecord}); err != nil {
			if ctx.Err() != nil {
				w.logger.Info("context cancelled during export, stopping")
				break
			}
			w.reportProgressf("Exporter failed, will retry in 5s: %v", err)
			w.logger.Warn("exporter failed, will retry in 5s", zap.Error(err))
			// Use context-aware sleep
			select {
			case <-time.After(5 * time.Second):
			case <-ctx.Done():
				w.logger.Info("context cancelled during retry sleep, stopping")
				break
			}
			continue
		}

		i++
		if w.logsCounter != nil {
			atomic.AddInt64(w.logsCounter, 1)
		}
		if w.progressCh != nil {
			select {
			case w.progressCh <- struct{}{}:
			case <-ctx.Done():
				w.logger.Info("context cancelled during progress reporting, stopping")
				break
			}
		}
		if w.numLogs != 0 && i >= int64(w.numLogs) {
			break
		}
	}

	w.wg.Done()
}

// printLogDetails prints a human-readable representation of the generated log
func (w worker) printLogDetails(cfg *Config, data *logData, attrs []attribute.KeyValue, dataAttrs []log.KeyValue, res *resource.Resource) {
	// Trace/Span IDs as strings or "-"
	traceIDStr := data.traceID.String()
	if traceIDStr == "00000000000000000000000000000000" || traceIDStr == "" {
		traceIDStr = "-"
	}
	spanIDStr := data.spanID.String()
	if spanIDStr == "0000000000000000" || spanIDStr == "" {
		spanIDStr = "-"
	}

	fmt.Println("Generated Log Record:")
	fmt.Printf("  Body: %s\n", common.FormatSensitiveBody(strings.TrimSpace(data.body)))
	fmt.Printf("  Severity Number (int): %d\n", data.severityNumber)
	fmt.Printf("  Severity Text: %s\n", data.severityText)
	fmt.Printf("  TraceID: %s\n", traceIDStr)
	fmt.Printf("  SpanID: %s\n", spanIDStr)
	fmt.Println("  Attributes:")
	for _, kv := range attrs {
		key := string(kv.Key)
		val := attrValueToString(kv.Value)
		fmt.Printf("    - %s: %s\n", key, common.FormatSensitiveValue(key, val))
	}
	// Also include any runtime-added attributes (e.g., time-bound markers) that may not be in attrs
	for _, kv := range dataAttrs {
		k := fmt.Sprintf("%v", kv.Key)
		v := fmt.Sprintf("%v", kv.Value)
		dup := false
		for _, orig := range attrs {
			if string(orig.Key) == k {
				dup = true
				break
			}
		}
		if !dup {
			fmt.Printf("    - %s: %s\n", k, common.FormatSensitiveValue(k, v))
		}
	}
	fmt.Println("  Resource Attributes:")
	for _, kv := range res.Attributes() {
		key := string(kv.Key)
		val := attrValueToString(kv.Value)
		fmt.Printf("    - %s: %s\n", key, common.FormatSensitiveValue(key, val))
	}
	fmt.Println("--------------------------------------------------")
}

// attrValueToString renders an attribute.Value as a string sensibly
func attrValueToString(v attribute.Value) string {
	switch v.Type() {
	case attribute.BOOL:
		if v.AsBool() {
			return "true"
		}
		return "false"
	case attribute.INT64:
		return fmt.Sprintf("%d", v.AsInt64())
	case attribute.FLOAT64:
		return fmt.Sprintf("%g", v.AsFloat64())
	case attribute.STRING:
		return strings.TrimSpace(v.AsString())
	default:
		return fmt.Sprintf("%v", v)
	}
}
