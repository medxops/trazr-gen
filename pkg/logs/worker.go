// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"context"
	"encoding/hex"
	"fmt"
	"math"
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
	running        *atomic.Bool    // pointer to shared flag that indicates it's time to stop the test
	numLogs        int             // how many logs the worker has to generate (only when duration==0)
	body           string          // the body of the log
	severityNumber string          // the severityNumber of the log (string, for templating)
	severityText   string          // the severityText of the log
	totalDuration  time.Duration   // how long to run the test for (overrides `numLogs`)
	limitPerSecond rate.Limit      // how many logs per second to generate
	wg             *sync.WaitGroup // notify when done
	logger         *zap.Logger     // logger
	index          int             // worker index
	traceID        string          // traceID string
	spanID         string          // spanID string
	logsCounter    *int64          // pointer to shared logs counter
	progressCb     func(string)    // optional callback for terminal output
}

// Helper to convert []attribute.KeyValue to []log.KeyValue
func attrToLogKeyValue(attrs []attribute.KeyValue) []log.KeyValue {
	result := make([]log.KeyValue, len(attrs))
	for i, attr := range attrs {
		var v log.Value
		switch attr.Value.Type() {
		case attribute.BOOL:
			v = log.BoolValue(attr.Value.AsBool())
		case attribute.INT64:
			v = log.Int64Value(attr.Value.AsInt64())
		case attribute.FLOAT64:
			v = log.Float64Value(attr.Value.AsFloat64())
		case attribute.STRING:
			v = log.StringValue(attr.Value.AsString())
		default:
			v = log.StringValue(attr.Value.Emit())
		}
		result[i] = log.KeyValue{
			Key:   string(attr.Key),
			Value: v,
		}
	}
	return result
}

func (w worker) reportProgressf(format string, args ...any) {
	if w.progressCb != nil {
		w.progressCb(fmt.Sprintf(format, args...))
	}
}

func (w worker) simulateLogs(cfg *Config, res *resource.Resource, exporter sdklog.Exporter) {
	limiter := rate.NewLimiter(w.limitPerSecond, 1)
	var i int64

	for w.running.Load() {
		var tid trace.TraceID
		var sid trace.SpanID

		if w.spanID != "" {
			b, _ := hex.DecodeString(w.spanID)
			sid = trace.SpanID(b)
		}
		if w.traceID != "" {
			b, _ := hex.DecodeString(w.traceID)
			tid = trace.TraceID(b)
		}

		// --- Get processed attribute KeyValues (including mock marker logic) ---
		attrKVs, err := cfg.GetTelemetryAttrWithMockMarker()
		if err != nil {
			w.reportProgressf("Failed to process telemetry attributes: %v", err)
			w.logger.Fatal("failed to process telemetry attributes", zap.Error(err))
			break
		}

		// --- Process log body with gofakeit templating ---
		var body string
		body = w.body
		logBodyExpanded := false
		if cfg.MockData {
			expanded, expandErr := common.ProcessMockTemplate(body, nil)
			if expandErr != nil {
				break
			}

			if expanded != body {
				body = expanded
				logBodyExpanded = true
			}
		}

		// --- If log body was expanded, append log-body to the marker ---
		if logBodyExpanded {
			found := false
			for i, attr := range attrKVs {
				if attr.Key == "trazr.mock.data" {
					val := attr.Value.AsString()
					val += ",Body"
					attrKVs[i] = attribute.String("trazr.mock.data", val)
					found = true
					break
				}
			}
			if !found {
				attrKVs = append(attrKVs, attribute.String("trazr.mock.data", "log-body"))
			}
		}

		// --- Convert to log.KeyValue and add service.name (only once) ---
		attrs := attrToLogKeyValue(attrKVs)
		attrs = append([]log.KeyValue{log.String("service.name", cfg.ServiceName)}, attrs...)

		// --- Process severity number with gofakeit templating per log entry ---
		severityNumberStr := w.severityNumber
		if cfg.MockData && len(severityNumberStr) > 0 && (strings.Contains(severityNumberStr, "{{") && strings.Contains(severityNumberStr, "}}")) {
			parsed, parseErr := common.ProcessMockTemplate(severityNumberStr, nil)
			if parseErr != nil {
				w.reportProgressf("Failed to process mock template for severity-number: %v", parseErr)
				w.logger.Error("failed to process mock template for severity-number", zap.Error(parseErr))
				// fallback to default
			} else {
				severityNumberStr = parsed
			}
		}
		severityNumberInt, err := strconv.Atoi(severityNumberStr)
		if err != nil && severityNumberInt < 1 && severityNumberInt > 24 {
			severityNumberInt = 9 // fallback to Info if parsing fails
		}
		// Clamp severityNumberInt to int32 range to avoid overflow (gosec: G109)
		var safeSeverityNumberInt int32
		switch {
		case severityNumberInt > math.MaxInt32:
			safeSeverityNumberInt = math.MaxInt32
		case severityNumberInt < math.MinInt32:
			safeSeverityNumberInt = math.MinInt32
		default:
			safeSeverityNumberInt = int32(severityNumberInt) //nolint:gosec // checked range above
		}
		severityText, severityNumber, err := parseSeverity(w.severityText, safeSeverityNumberInt)
		if err != nil {
			severityText = w.severityText
			severityNumber = log.Severity(safeSeverityNumberInt)
		}

		rf := logtest.RecordFactory{
			Timestamp:         time.Now(),
			Severity:          severityNumber,
			SeverityText:      severityText,
			Body:              log.StringValue(body),
			Attributes:        attrs,
			TraceID:           tid,
			SpanID:            sid,
			Resource:          res,
			DroppedAttributes: 1,
		}

		logs := []sdklog.Record{rf.NewRecord()}

		if err := limiter.Wait(context.Background()); err != nil {
			w.reportProgressf("Limiter wait failed: %v", err)
			w.logger.Fatal("limiter wait failed, retry", zap.Error(err))
		}

		if err := exporter.Export(context.Background(), logs); err != nil {
			w.reportProgressf("Exporter failed: %v", err)
			w.logger.Fatal("exporter failed", zap.Error(err))
		}

		i++
		if w.logsCounter != nil {
			atomic.AddInt64(w.logsCounter, 1)
		}
		if w.numLogs != 0 && i >= int64(w.numLogs) {
			break
		}
	}

	w.wg.Done()
}
