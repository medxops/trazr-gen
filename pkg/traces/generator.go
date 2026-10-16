// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// Trace represents a complete trace with its spans
type Trace struct {
	TraceID  trace.TraceID
	Spans    []*Span
	Services []string
}

// Span represents a span within a trace
type Span struct {
	ID         string
	Name       string
	Service    string
	ParentID   string
	StartTime  time.Time
	EndTime    time.Time
	Status     codes.Code
	Attributes map[string]any
	Events     []SpanEvent
	Children   []*Span
	Depth      int
	TraceID    trace.TraceID // Added for recursive assignment
}

// SpanEvent represents an event within a span
type SpanEvent struct {
	Name       string
	Timestamp  time.Time
	Attributes map[string]any
}

// safeUint64FromInt64 safely converts int64 to uint64 for seed values
// Seeds are typically positive, so this conversion is safe
func safeUint64FromInt64(seed int64) uint64 {
	if seed < 0 {
		return uint64(-seed) // Use absolute value for negative seeds
	}
	return uint64(seed)
}

// TraceGenerator handles the generation of realistic traces
type TraceGenerator struct {
	config   *Config
	logger   *zap.Logger
	patterns map[string]Pattern
	rng      *rand.Rand // Seeded random source for reproducible generation
}

// Pattern interface for different trace structures
type Pattern interface {
	GenerateSpans(config *Config, traceID trace.TraceID, services []string) []*Span
}

// NewTraceGenerator creates a new trace generator
func NewTraceGenerator(config *Config, logger *zap.Logger) *TraceGenerator {
	tg := &TraceGenerator{
		config:   config,
		logger:   logger,
		patterns: make(map[string]Pattern),
		rng:      rand.New(rand.NewPCG(safeUint64FromInt64(config.Seed), 0)), // Initialize seeded random source
	}

	// Register patterns
	tg.patterns["simple"] = &SimplePattern{}
	tg.patterns["chain"] = &ChainPattern{}
	tg.patterns["fan-out"] = &FanOutPattern{}
	tg.patterns["complex"] = &ComplexPattern{}
	tg.patterns["random"] = &RandomPattern{}

	return tg
}

// GenerateTrace generates a single trace with realistic timing, attributes, events, and errors
func (tg *TraceGenerator) GenerateTrace(_ context.Context) (*Trace, error) {
	// Generate a new trace ID
	traceID := trace.TraceID{}
	// Generate a random trace ID using math/rand for reproducibility
	for i := range traceID {
		traceID[i] = byte(tg.rng.IntN(256)) //nolint:gosec // IntN bounds the value to one byte.
	}

	// Select pattern based on config
	patternName := tg.config.Pattern
	if patternName == "random" {
		patterns := []string{"simple", "chain", "fan-out", "complex"}
		// Use a deterministic pattern selection based on the trace ID
		patternIndex := int(traceID[0]) % len(patterns)
		patternName = patterns[patternIndex]
	}
	pattern, exists := tg.patterns[patternName]
	if !exists {
		tg.logger.Warn("unknown pattern, falling back to simple", zap.String("pattern", patternName))
		pattern = tg.patterns["simple"]
	}

	// Generate spans using the selected pattern
	spans := pattern.GenerateSpans(tg.config, traceID, tg.config.Services)

	// Assign real SpanIDs (hex) and fix ParentIDs
	if len(spans) > 0 {
		idGen := 0
		tg.assignSpanIDsAndTraceID(spans[0], "", traceID, &idGen)
		// Flatten the tree into a slice
		flatSpans := make([]*Span, 0)
		var flatten func(s *Span)
		flatten = func(s *Span) {
			flatSpans = append(flatSpans, s)
			for _, c := range s.Children {
				flatten(c)
			}
		}
		flatten(spans[0])
		spans = flatSpans
	}
	tg.fixSpanParentIDs(spans)

	// Add realistic timing and attributes
	tg.addRealisticTiming(spans)
	tg.addRealisticAttributes(spans)
	tg.addRealisticEvents(spans)
	tg.addRealisticErrors(spans)
	tg.fixSpanStatus(spans)

	return &Trace{
		TraceID:  traceID,
		Spans:    spans,
		Services: tg.config.Services,
	}, nil
}

// Recursive function to assign real SpanIDs and ParentIDs, and set TraceID for all spans
func (tg *TraceGenerator) assignSpanIDsAndTraceID(span *Span, parentID string, traceID [16]byte, idGen *int) {
	*idGen++
	span.ID = fmt.Sprintf("%016x", *idGen)
	span.ParentID = parentID
	span.TraceID = traceID
	for _, child := range span.Children {
		tg.assignSpanIDsAndTraceID(child, span.ID, traceID, idGen)
	}
}

// addRealisticTiming adds realistic timing to spans
func (tg *TraceGenerator) addRealisticTiming(spans []*Span) {
	if len(spans) == 0 {
		return
	}
	// Start with the root span
	rootSpan := spans[0]
	rootSpan.StartTime = time.Now()
	// Calculate timing for all spans recursively
	tg.staggerChildren(rootSpan, rootSpan.StartTime, rootSpan.StartTime.Add(tg.getSpanDuration(rootSpan)))
}

// getSpanDuration returns a realistic duration for a span, respecting config and jitter
func (tg *TraceGenerator) getSpanDuration(span *Span) time.Duration {
	baseDuration := tg.config.SpanDuration
	if baseDuration <= 0 {
		baseDuration = time.Millisecond
	}
	jitterPercentage := tg.config.DurationJitter
	if jitterPercentage < 0 {
		jitterPercentage = 0.25
	}
	if jitterPercentage > 1.0 {
		jitterPercentage = 1.0
	}
	seed := int64(0)
	for i, char := range span.ID {
		seed += int64(char) * int64(i+1)
	}
	jitterMultiplier := 1.0 + (float64(seed%1000)/1000.0-0.5)*2*jitterPercentage
	dur := time.Duration(float64(baseDuration) * jitterMultiplier)
	if dur < time.Millisecond {
		dur = time.Millisecond
	}
	return dur
}

// staggerChildren staggers child spans within the parent span's duration
func (tg *TraceGenerator) staggerChildren(parent *Span, parentStart, parentEnd time.Time) {
	parentDuration := parentEnd.Sub(parentStart)
	if parentDuration <= 0 {
		parentDuration = time.Millisecond
	}
	// Set parent span's start/end
	if parent.StartTime.IsZero() {
		parent.StartTime = parentStart
	}
	if parent.EndTime.IsZero() {
		parent.EndTime = parentEnd
	}
	children := parent.Children
	n := len(children)
	if n == 0 {
		return
	}
	// Divide parent duration into n+1 segments for n children
	segment := parentDuration / time.Duration(n+1)
	for i, child := range children {
		// Each child starts after parentStart + (i+1)*segment/2
		childStart := parentStart.Add(time.Duration(i+1) * segment / 2)
		childDuration := tg.getSpanDuration(child)
		// End before parentEnd, but after start
		childEnd := childStart.Add(childDuration)
		if childEnd.After(parentEnd) {
			childEnd = parentEnd
		}
		child.StartTime = childStart
		child.EndTime = childEnd
		// Recurse for grandchildren
		tg.staggerChildren(child, childStart, childEnd)
	}
}

// addRealisticAttributes adds realistic attributes to spans
func (tg *TraceGenerator) addRealisticAttributes(spans []*Span) {
	for spanIndex, span := range spans {
		span.Attributes = make(map[string]any)

		// Set span kind based on depth
		span.Attributes["span.kind"] = tg.getSpanKind(span.Depth)

		// Add HTTP attributes for API spans (depth 0 or 1)
		if span.Depth == 0 || span.Depth == 1 {
			span.Attributes["http.method"] = tg.getDeterministicHTTPMethod(spanIndex)
			span.Attributes["http.url"] = tg.getDeterministicURL(spanIndex)
			span.Attributes["http.status_code"] = tg.getDeterministicStatusCode(spanIndex)
			span.Attributes["http.route"] = tg.getDeterministicRoute(spanIndex)
		}

		// Add database attributes for deeper spans
		if span.Depth >= 2 {
			span.Attributes["db.system"] = tg.getDeterministicDatabase(spanIndex)
			span.Attributes["db.operation"] = tg.getDeterministicDBOperation(spanIndex)
			span.Attributes["db.statement"] = tg.getDeterministicSQLStatement(spanIndex)
		}

		// Add proper ID attributes as strings
		span.Attributes["request.id"] = tg.getDeterministicRequestID(spanIndex)
		span.Attributes["correlation.id"] = tg.getDeterministicCorrelationID(spanIndex)
		span.Attributes["session.id"] = tg.getDeterministicSessionID(spanIndex)
		span.Attributes["user.id"] = tg.getDeterministicUserID(spanIndex)
		span.Attributes["tenant.id"] = tg.getDeterministicTenantID(spanIndex)

		// Add custom attributes based on config
		for i := 0; i < tg.config.AttributesPerSpan; i++ {
			key := tg.getDeterministicAttributeKey(spanIndex, i)
			value := tg.getDeterministicAttributeValue(spanIndex, i)
			span.Attributes[key] = value
		}
	}
}

// addRealisticEvents adds realistic events to spans
func (tg *TraceGenerator) addRealisticEvents(spans []*Span) {
	for spanIndex, span := range spans {
		span.Events = make([]SpanEvent, 0, tg.config.EventsPerSpan)
		spanDuration := span.EndTime.Sub(span.StartTime)
		if spanDuration <= 0 {
			spanDuration = time.Millisecond
		}
		hasCacheHit := (spanIndex % 3) == 0
		for i := 0; i < tg.config.EventsPerSpan; i++ {
			eventProgress := float64(i+1) / float64(tg.config.EventsPerSpan+1)
			eventOffset := time.Duration(float64(spanDuration) * eventProgress)
			eventTime := span.StartTime.Add(eventOffset)
			if eventTime.After(span.EndTime) {
				eventTime = span.EndTime.Add(-time.Millisecond)
			}
			var eventName string
			if i == 0 {
				if hasCacheHit {
					eventName = "cache.hit"
				} else {
					eventName = "cache.miss"
				}
			}
			if hasCacheHit {
				break // No further events after cache.hit
			}
			allEvents := []string{"db.query", "external.call", "validation.failed"}
			eventName = allEvents[(spanIndex+i)%len(allEvents)]
			event := SpanEvent{
				Name:       eventName,
				Timestamp:  eventTime,
				Attributes: make(map[string]any),
			}
			event.Attributes["event.type"] = "custom"
			event.Attributes["event.source"] = span.Service
			tg.addEventSpecificAttributes(&event, spanIndex, i)
			span.Events = append(span.Events, event)
		}
	}
}

// addEventSpecificAttributes adds realistic attributes based on event type
func (tg *TraceGenerator) addEventSpecificAttributes(event *SpanEvent, spanIndex, eventIndex int) {
	switch event.Name {
	case "cache.hit":
		event.Attributes["cache.key"] = fmt.Sprintf("key-%d-%d", spanIndex, eventIndex)
		event.Attributes["cache.ttl"] = 3600
	case "cache.miss":
		event.Attributes["cache.key"] = fmt.Sprintf("key-%d-%d", spanIndex, eventIndex)
		event.Attributes["cache.reason"] = "not_found"
	case "db.query":
		event.Attributes["db.system"] = tg.getDeterministicDatabase(spanIndex)
		event.Attributes["db.operation"] = tg.getDeterministicDBOperation(spanIndex)
		event.Attributes["db.statement"] = tg.getDeterministicSQLStatement(spanIndex)
	case "external.call":
		event.Attributes["http.method"] = tg.getDeterministicHTTPMethod(spanIndex)
		event.Attributes["http.url"] = fmt.Sprintf("https://api.example.com/v1/resource-%d", spanIndex)
		event.Attributes["http.status_code"] = tg.getDeterministicStatusCode(spanIndex)
	case "validation.failed":
		event.Attributes["validation.field"] = fmt.Sprintf("field-%d", eventIndex)
		event.Attributes["validation.error"] = "invalid_format"
	}
}

// addRealisticErrors adds realistic errors to spans based on error rate and HTTP status codes
func (tg *TraceGenerator) addRealisticErrors(spans []*Span) {
	for spanIndex, span := range spans {
		// Check if span has HTTP status code
		if statusCode, hasHTTP := span.Attributes["http.status_code"]; hasHTTP {
			if code, ok := statusCode.(int); ok {
				// Set status based on HTTP status code
				if code >= 200 && code < 400 {
					span.Status = codes.Ok
				} else {
					span.Status = codes.Error
					span.Attributes["error"] = true
					span.Attributes["error.message"] = tg.getDeterministicErrorMessage(spanIndex)
				}
			} else {
				span.Status = codes.Ok
			}
		} else {
			// For non-HTTP spans, use error rate
			errorThreshold := float64(spanIndex) / float64(len(spans))
			if errorThreshold < tg.config.ErrorRate {
				span.Status = codes.Error
				span.Attributes["error"] = true
				span.Attributes["error.message"] = tg.getDeterministicErrorMessage(spanIndex)
			} else {
				span.Status = codes.Ok
			}
		}
	}
}

// fixSpanStatus sets span status based on events and logs
func (tg *TraceGenerator) fixSpanStatus(spans []*Span) {
	for _, span := range spans {
		span.Status = codes.Ok
		for _, event := range span.Events {
			if event.Name == "validation.failed" {
				span.Status = codes.Error
			}
			if code, ok := event.Attributes["http.status_code"]; ok {
				if v, ok := code.(int); ok && v >= 400 {
					span.Status = codes.Error
				}
			}
		}
		if logs, ok := span.Attributes["logs"].([]string); ok {
			for _, log := range logs {
				if log == "ERROR" {
					span.Status = codes.Error
				}
			}
		}
	}
}

// After generating all spans, update ParentID to real SpanID
func (tg *TraceGenerator) fixSpanParentIDs(spans []*Span) {
	idMap := make(map[string]string) // symbolic ID -> real SpanID
	for _, span := range spans {
		idMap[span.ID] = span.ID // ID is already the real SpanID
	}
	for _, span := range spans {
		if span.ParentID != "" {
			if realID, ok := idMap[span.ParentID]; ok {
				span.ParentID = realID
			}
		}
	}
}

// Helper methods for generating realistic data
func (tg *TraceGenerator) getSpanKind(depth int) string {
	kinds := []string{"SERVER", "CLIENT", "INTERNAL", "PRODUCER", "CONSUMER"}
	if depth == 0 {
		return "SERVER" // Root spans are typically servers
	}
	return kinds[tg.rng.IntN(len(kinds))]
}

// Remove the unused random helper functions

// Helper methods for generating deterministic data
func (tg *TraceGenerator) getDeterministicHTTPMethod(spanIndex int) string {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	return methods[spanIndex%len(methods)]
}

func (tg *TraceGenerator) getDeterministicURL(spanIndex int) string {
	urls := []string{
		"/api/users", "/api/orders", "/api/products", "/api/payments",
		"/api/auth", "/api/config", "/api/health", "/api/metrics",
	}
	return urls[spanIndex%len(urls)]
}

func (tg *TraceGenerator) getDeterministicRoute(spanIndex int) string {
	routes := []string{
		"/api/users", "/api/orders", "/api/products", "/api/payments",
		"/api/auth", "/api/config", "/api/health", "/api/metrics",
	}
	return routes[spanIndex%len(routes)]
}

func (tg *TraceGenerator) getDeterministicStatusCode(spanIndex int) int {
	codes := []int{200, 201, 204, 400, 401, 403, 404, 500}
	return codes[spanIndex%len(codes)]
}

func (tg *TraceGenerator) getDeterministicDatabase(spanIndex int) string {
	dbs := []string{"postgresql", "mysql", "mongodb", "redis", "elasticsearch"}
	return dbs[spanIndex%len(dbs)]
}

func (tg *TraceGenerator) getDeterministicDBOperation(spanIndex int) string {
	ops := []string{"SELECT", "INSERT", "UPDATE", "DELETE", "EXECUTE"}
	return ops[spanIndex%len(ops)]
}

func (tg *TraceGenerator) getDeterministicSQLStatement(spanIndex int) string {
	statements := []string{
		"SELECT * FROM users WHERE id = ?",
		"INSERT INTO orders (user_id, amount) VALUES (?, ?)",
		"UPDATE products SET stock = stock - ? WHERE id = ?",
		"DELETE FROM sessions WHERE expired_at < NOW()",
	}
	return statements[spanIndex%len(statements)]
}

func (tg *TraceGenerator) getDeterministicRequestID(spanIndex int) string {
	return fmt.Sprintf("req-%d-%d", spanIndex, tg.rng.Int64()%10000)
}

func (tg *TraceGenerator) getDeterministicCorrelationID(spanIndex int) string {
	return fmt.Sprintf("corr-%d-%d", spanIndex, tg.rng.Int64()%10000)
}

func (tg *TraceGenerator) getDeterministicSessionID(spanIndex int) string {
	return fmt.Sprintf("sess-%d-%d", spanIndex, tg.rng.Int64()%10000)
}

func (tg *TraceGenerator) getDeterministicUserID(spanIndex int) string {
	return fmt.Sprintf("user-%d", spanIndex+1000)
}

func (tg *TraceGenerator) getDeterministicTenantID(spanIndex int) string {
	return fmt.Sprintf("tenant-%d", spanIndex+100)
}

func (tg *TraceGenerator) getDeterministicAttributeKey(spanIndex int, attrIndex int) string {
	keys := []string{"custom.key", "app.version", "deployment.env", "instance.id", "region"}
	return keys[(spanIndex+attrIndex)%len(keys)]
}

func (tg *TraceGenerator) getDeterministicAttributeValue(spanIndex int, attrIndex int) any {
	values := []any{"v1.0.0", "production", "us-east-1", "instance-1", "us-west-2"}
	return values[(spanIndex+attrIndex)%len(values)]
}

func (tg *TraceGenerator) getDeterministicErrorMessage(spanIndex int) string {
	messages := []string{"Database connection failed", "Invalid input", "Rate limit exceeded", "Authentication failed"}
	return messages[spanIndex%len(messages)]
}
