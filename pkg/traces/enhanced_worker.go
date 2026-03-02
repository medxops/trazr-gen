package traces

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

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/log/logtest"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"github.com/medxops/trazr-gen/internal/common"
	"github.com/medxops/trazr-gen/pkg/logs"
	"github.com/medxops/trazr-gen/pkg/metrics"
)

// safeInt64FromUint64 safely converts uint64 to int64 for small values
// This is safe for small bounded values like bucket counts
func safeInt64FromUint64(val uint64) int64 {
	if val > math.MaxInt64 {
		return math.MaxInt64 // Cap at max int64
	}
	return int64(val)
}

const (
	// Default values for signal generation
	defaultSpanDuration = time.Millisecond
	microsecondOffset   = 50 * time.Microsecond
	maxSignalsPerSpan   = 3
	minSignalsPerSpan   = 1

	// Service name for resource attributes
	serviceName = "trazr-gen"

	// Default severity levels
	severityDebug = "debug"
	severityInfo  = "info"
	severityWarn  = "warn"
	severityError = "error"
)

// SpanSignals represents all signals generated within a span
type SpanSignals struct {
	span     *Span
	otelSpan oteltrace.Span
	logs     []*logs.Config
	metrics  []*metrics.Config
	events   []SpanEvent
}

type EnhancedWorker struct {
	config             *Config
	logConfig          *logs.Config
	metricConfig       *metrics.Config
	logger             *zap.Logger
	traceGenerator     *TraceGenerator
	attributeGenerator *AttributeGenerator
	// Worker state
	numTraces      int
	totalDuration  time.Duration
	limitPerSecond rate.Limit
	running        *atomic.Bool
	wg             *sync.WaitGroup
	tracesCounter  *atomic.Int64
	progressCh     chan struct{}
	progressCb     func(string)
	tracer         oteltrace.Tracer

	// Direct exporters for enhanced mode
	logExporter    sdklog.Exporter
	metricExporter sdkmetric.Exporter
	resource       *resource.Resource

	// Statistics counters
	spansGenerated   *atomic.Int64
	logsGenerated    *atomic.Int64
	eventsGenerated  *atomic.Int64
	errorsGenerated  *atomic.Int64
	logsBySeverity   map[string]*atomic.Int64
	metricsGenerated *atomic.Int64
	metricsByType    map[string]*atomic.Int64
}

func NewEnhancedWorker(
	config *Config,
	logConfig *logs.Config,
	metricConfig *metrics.Config,
	logger *zap.Logger,
	numTraces int,
	totalDuration time.Duration,
	limitPerSecond rate.Limit,
	running *atomic.Bool,
	wg *sync.WaitGroup,
	tracesCounter *atomic.Int64,
	progressCh chan struct{},
	progressCb func(string),
) *EnhancedWorker {
	// Initialize severity counters
	logsBySeverity := make(map[string]*atomic.Int64)
	for _, severity := range []string{severityDebug, severityInfo, severityWarn, severityError} {
		logsBySeverity[severity] = &atomic.Int64{}
	}

	// Initialize resource with common attributes
	attrs := []attribute.KeyValue{
		semconv.ServiceName(serviceName), // Default service name
	}
	if config.MockSeed != 0 {
		attrs = append(attrs, attribute.String("trazrgen.random.seed", strconv.FormatInt(config.MockSeed, 10)))
	} else {
		attrs = append(attrs, attribute.String("trazrgen.random.seed", "timestamp-based"))
	}

	// Add trazr.run.flags attribute when trace-bound feature is enabled
	if config.TrazrTraceBound {
		attrs = append(attrs, attribute.String("trazr.run.flags", "tracebound"))
	}

	res, _ := resource.New(context.Background(), resource.WithAttributes(attrs...))

	// Initialize direct exporters for enhanced mode
	var logExporter sdklog.Exporter
	var metricExporter sdkmetric.Exporter

	// Only create exporters if we're in enhanced mode
	if config.Enhanced {
		// Create log exporter using the same endpoint configuration as traces
		if logConfig != nil {
			logExporter = createLogExporter(config, logConfig, logger)
		}

		// Create metric exporter using the same endpoint configuration as traces
		if metricConfig != nil {
			metricExporter = createMetricExporter(config, metricConfig, logger)
		}
	}

	return &EnhancedWorker{
		config:             config,
		logConfig:          logConfig,
		metricConfig:       metricConfig,
		logger:             logger,
		traceGenerator:     NewTraceGenerator(config, logger),
		attributeGenerator: NewAttributeGenerator(config, metricConfig, logger),
		numTraces:          numTraces,
		totalDuration:      totalDuration,
		limitPerSecond:     limitPerSecond,
		running:            running,
		wg:                 wg,
		tracesCounter:      tracesCounter,
		progressCh:         progressCh,
		progressCb:         progressCb,
		tracer:             otel.Tracer(serviceName),
		logExporter:        logExporter,
		metricExporter:     metricExporter,
		resource:           res,
		spansGenerated:     &atomic.Int64{},
		logsGenerated:      &atomic.Int64{},
		eventsGenerated:    &atomic.Int64{},
		errorsGenerated:    &atomic.Int64{},
		logsBySeverity:     logsBySeverity,
		metricsGenerated:   &atomic.Int64{},
		metricsByType:      make(map[string]*atomic.Int64),
	}
}

func (w *EnhancedWorker) Start(ctx context.Context) error {
	defer w.wg.Done()
	limiter := rate.NewLimiter(w.limitPerSecond, 1)
	traceCount := 0

	// Set up duration-based stopping if totalDuration is specified
	var stopTime time.Time
	if w.totalDuration > 0 {
		stopTime = time.Now().Add(w.totalDuration)
	}

	for w.running.Load() {
		// Check if we've exceeded the total duration
		if !stopTime.IsZero() && time.Now().After(stopTime) {
			w.logger.Debug("stopping due to total duration reached")
			break
		}

		if err := limiter.Wait(ctx); err != nil {
			w.logger.Error("limiter wait failed", zap.Error(err))
			continue
		}

		if err := w.generateAndExportTrace(ctx); err != nil {
			w.logger.Error("failed to generate and export trace", zap.Error(err))
			continue
		}

		traceCount++
		if w.tracesCounter != nil {
			w.tracesCounter.Add(1)
		}
		if w.progressCh != nil {
			w.progressCh <- struct{}{}
		}

		if w.numTraces != 0 && traceCount >= w.numTraces {
			w.logger.Debug("stopping due to numTraces reached", zap.Int("traces_generated", traceCount))
			break
		}
	}

	w.logger.Info("worker finished", zap.Int("traces_generated", traceCount))
	w.logger.Info("worker Start function returning")
	return nil
}

func (w *EnhancedWorker) generateAndExportTrace(ctx context.Context) error {
	w.logger.Debug("starting generateAndExportTrace")

	trace, err := w.traceGenerator.GenerateTrace(ctx)
	if err != nil {
		return fmt.Errorf("failed to generate trace: %w", err)
	}

	w.logger.Debug("trace generated, processing spans", zap.Int("num_spans", len(trace.Spans)))

	// Track spans and events for this trace
	spansInTrace := 0
	eventsInTrace := 0
	errorsInTrace := 0

	// --- BEGIN FIX: propagate parent context for correct trace IDs (user feedback version) ---
	spanContexts := make(map[string]context.Context)
	spanOtelSpans := make(map[string]oteltrace.Span)

	traceCtx := ctx
	isFirstRoot := true

	// First, create all root spans (spans with no parent)
	for _, span := range trace.Spans {
		if span.ParentID == "" {
			var otelSpan oteltrace.Span
			spanName := w.generateSpanName(span)
			spanKind := w.getSpanKindFromString(span.Attributes["span.kind"])
			if isFirstRoot {
				traceCtx, otelSpan = w.tracer.Start(ctx, spanName, oteltrace.WithSpanKind(spanKind))
				isFirstRoot = false
			} else {
				_, otelSpan = w.tracer.Start(traceCtx, spanName, oteltrace.WithSpanKind(spanKind))
			}
			spanContexts[span.ID] = oteltrace.ContextWithSpan(ctx, otelSpan)
			spanOtelSpans[span.ID] = otelSpan
		}
	}

	// Now, create all child spans
	for _, span := range trace.Spans {
		if span.ParentID != "" {
			parentOtelSpan, ok := spanOtelSpans[span.ParentID]
			parentCtx := ctx
			if ok {
				parentCtx = oteltrace.ContextWithSpan(ctx, parentOtelSpan)
			}
			spanName := w.generateSpanName(span)
			spanKind := w.getSpanKindFromString(span.Attributes["span.kind"])
			_, otelSpan := w.tracer.Start(parentCtx, spanName, oteltrace.WithSpanKind(spanKind))
			spanContexts[span.ID] = oteltrace.ContextWithSpan(ctx, otelSpan)
			spanOtelSpans[span.ID] = otelSpan
		}
	}

	// Now, process all spans for signals, logs, metrics, etc.
	spanSignals := make([]SpanSignals, 0, len(trace.Spans))
	for spanIndex, span := range trace.Spans {
		otelSpan := spanOtelSpans[span.ID]
		spanCtx := spanContexts[span.ID]

		// Debug logging
		w.logger.Debug("created span",
			zap.String("span_name", span.Name),
			zap.String("trace_id", otelSpan.SpanContext().TraceID().String()),
			zap.String("span_id", otelSpan.SpanContext().SpanID().String()),
			zap.Time("start_time", span.StartTime),
			zap.Time("end_time", span.EndTime),
			zap.Duration("duration", span.EndTime.Sub(span.StartTime)))

		// Set span attributes
		w.setSpanAttributes(otelSpan, span)

		// Count span events
		spanEvents := len(span.Events)
		eventsInTrace += spanEvents
		w.eventsGenerated.Add(int64(spanEvents))

		w.addSpanEvents(otelSpan, span)

		// Collect signals for this span
		spanData := SpanSignals{
			span:     span,
			otelSpan: otelSpan,
			logs:     make([]*logs.Config, 0),
			metrics:  make([]*metrics.Config, 0),
			events:   span.Events,
		}

		// Generate and collect logs for this span
		w.collectLogsForSpan(spanCtx, span, otelSpan, spanIndex, &spanData)

		// Generate and collect metrics for this span using the span's actual trace ID
		w.collectMetricsForSpan(spanCtx, span, otelSpan, spanIndex, &spanData)

		// Check if span has error status
		if span.Status == codes.Error {
			errorsInTrace++
			w.errorsGenerated.Add(1)
		}

		spanSignals = append(spanSignals, spanData)
		spansInTrace++
	}
	// --- END FIX ---

	// Now display all spans sequentially with their signals
	w.displaySpansSequentially(spanSignals)

	// Sleep for the calculated duration to make the span last the expected time
	// (we'll do this for each span individually during display)
	for _, spanData := range spanSignals {
		spanDuration := spanData.span.EndTime.Sub(spanData.span.StartTime)
		if spanDuration > 0 {
			w.logger.Debug("sleeping for span duration",
				zap.String("span_name", spanData.span.Name),
				zap.Duration("duration", spanDuration))
			time.Sleep(spanDuration)
		}

		// Debug logging before ending span
		w.logger.Debug("ending span",
			zap.String("span_name", spanData.span.Name),
			zap.String("trace_id", spanData.otelSpan.SpanContext().TraceID().String()),
			zap.String("span_id", spanData.otelSpan.SpanContext().SpanID().String()))

		spanData.otelSpan.End()
	}

	w.spansGenerated.Add(int64(spansInTrace))

	// Update progress with detailed statistics
	if w.progressCb != nil {
		w.progressCb(fmt.Sprintf("Generated trace with %d spans, %d events, %d errors",
			spansInTrace, eventsInTrace, errorsInTrace))
	}

	w.logger.Debug("generateAndExportTrace completed")
	return nil
}

func (w *EnhancedWorker) setSpanAttributes(span oteltrace.Span, spanData *Span) {
	// Use AttributeGenerator to add service attributes
	serviceAttrs := w.attributeGenerator.GenerateSpanAttributes(spanData, 10)

	// Update spanData.Attributes with the generated attributes
	spanData.Attributes = serviceAttrs

	// Add all attributes from the AttributeGenerator
	for key, value := range serviceAttrs {
		// Skip redundant attributes that are handled elsewhere
		if key == "service.name" || key == "span.kind" {
			continue
		}

		switch v := value.(type) {
		case string:
			span.SetAttributes(attribute.String(key, v))
		case int:
			span.SetAttributes(attribute.Int64(key, int64(v)))
		case int64:
			span.SetAttributes(attribute.Int64(key, v))
		case float64:
			span.SetAttributes(attribute.Float64(key, v))
		case bool:
			span.SetAttributes(attribute.Bool(key, v))
		default:
			span.SetAttributes(attribute.String(key, fmt.Sprintf("%v", v)))
		}
	}

	// Set span status based on the span's Status field
	if spanData.Status == codes.Error {
		span.SetStatus(codes.Error, "")
	} else {
		span.SetStatus(codes.Ok, "")
	}
}

func (w *EnhancedWorker) addSpanEvents(span oteltrace.Span, spanData *Span) {
	for _, event := range spanData.Events {
		attrs := make([]attribute.KeyValue, 0, len(event.Attributes))
		for key, value := range event.Attributes {
			switch v := value.(type) {
			case string:
				attrs = append(attrs, attribute.String(key, v))
			case int:
				attrs = append(attrs, attribute.Int64(key, int64(v)))
			case int64:
				attrs = append(attrs, attribute.Int64(key, v))
			case float64:
				attrs = append(attrs, attribute.Float64(key, v))
			case bool:
				attrs = append(attrs, attribute.Bool(key, v))
			default:
				attrs = append(attrs, attribute.String(key, fmt.Sprintf("%v", v)))
			}
		}
		span.AddEvent(event.Name, oteltrace.WithAttributes(attrs...), oteltrace.WithTimestamp(event.Timestamp))
	}
}

func (w *EnhancedWorker) generateLogMessage(span *Span, spanIndex int, logIndex int) string {
	messages := []string{
		"Processing " + span.Name + " in " + span.Name + " service",
		"Executing " + span.Name + " operation",
		"Handling request in " + span.Name + " service",
		"Span " + span.Name + " started",
		"Operation " + span.Name + " completed",
	}
	return messages[(spanIndex+logIndex)%len(messages)]
}

func (w *EnhancedWorker) generateLogSeverity(spanIndex int, logIndex int) string {
	severities := []string{severityDebug, severityInfo, severityWarn, severityError}
	return severities[(spanIndex+logIndex*2)%len(severities)]
}

func (w *EnhancedWorker) generateMetricName(spanIndex int, metricIndex int) string {
	names := []string{
		"span.duration",
		"span.events",
		"span.errors",
		"span.attributes",
		"span.attributes.count",
	}
	return names[(spanIndex+metricIndex)%len(names)]
}

func (w *EnhancedWorker) generateMetricType(spanIndex int, metricIndex int) metrics.MetricType {
	types := []metrics.MetricType{
		metrics.MetricTypeGauge,
		metrics.MetricTypeSum,
		metrics.MetricTypeHistogram,
	}
	return types[(spanIndex+metricIndex)%len(types)]
}

func (w *EnhancedWorker) GetStatistics() map[string]int64 {
	stats := map[string]int64{
		"spans":   w.spansGenerated.Load(),
		"logs":    w.logsGenerated.Load(),
		"events":  w.eventsGenerated.Load(),
		"errors":  w.errorsGenerated.Load(),
		"metrics": w.metricsGenerated.Load(),
	}

	// Add logs by severity
	for severity, counter := range w.logsBySeverity {
		stats["logs_"+severity] = counter.Load()
	}

	// Add metrics by type
	for metricType, counter := range w.metricsByType {
		stats["metrics_"+metricType] = counter.Load()
	}

	return stats
}

func (w *EnhancedWorker) exportLogViaLogsSystem(logCfg *logs.Config, logTime time.Time, span *Span) error {
	// Use the existing logs system logic but for single log export
	// This reuses the same logic as simulateLogs() without the goroutine loop

	// Get processed attribute KeyValues (including mock marker logic)
	attrKVs, err := logCfg.GetTelemetryAttrWithMockMarker()
	if err != nil {
		return fmt.Errorf("failed to process telemetry attributes: %w", err)
	}

	// Process log body with gofakeit templating (if mock data is enabled)
	var body string
	body = logCfg.Body
	logBodyExpanded := false
	if logCfg.MockData {
		expanded, expandErr := common.ProcessMockTemplate(body, nil)
		if expandErr != nil {
			return fmt.Errorf("failed to process mock template for body: %w", expandErr)
		}
		if expanded != body {
			body = expanded
			logBodyExpanded = true
		}
	}

	// If log body was expanded, append log-body to the marker
	if logBodyExpanded {
		found := false
		for i, attr := range attrKVs {
			if attr.Key == "trazrgen.mock.data" {
				val := attr.Value.AsString()
				val += ",Body"
				attrKVs[i] = attribute.String("trazrgen.mock.data", val)
				found = true
				break
			}
		}
		if !found {
			attrKVs = append(attrKVs, attribute.String("trazrgen.mock.data", "log-body"))
		}
	}

	// Convert to log.KeyValue
	attrs := common.AttrToLogKeyValue(attrKVs)

	// Process severity with gofakeit templating (if mock data is enabled)
	severityNumberStr := logCfg.SeverityNumber
	if logCfg.MockData && len(severityNumberStr) > 0 && (strings.Contains(severityNumberStr, "{{") && strings.Contains(severityNumberStr, "}}")) {
		parsed, parseErr := common.ProcessMockTemplate(severityNumberStr, nil)
		if parseErr != nil {
			w.logger.Error("failed to process mock template for severity-number", zap.Error(parseErr))
			// fallback to default
		} else {
			severityNumberStr = parsed
		}
	}

	// Convert severity text to number
	severityNumberInt := common.SeverityTextToNumber(logCfg.SeverityText)
	if severityNumberStr != "" {
		if parsed, err := strconv.ParseInt(severityNumberStr, 10, 32); err == nil && parsed >= 1 && parsed <= 24 {
			// parsed is constrained to 1-24, so safe to convert to int32
			severityNumberInt = int32(parsed)
		}
	}

	// severityNumberInt is already an int32, so no clamping needed
	safeSeverityNumberInt := severityNumberInt

	mappedSeverityText := common.SeverityTextFromNumber(safeSeverityNumberInt)
	severityText := mappedSeverityText
	severityEnum := log.Severity(safeSeverityNumberInt)

	// Parse trace and span IDs
	var tid oteltrace.TraceID
	var sid oteltrace.SpanID
	if logCfg.SpanID != "" {
		if b, err := hex.DecodeString(logCfg.SpanID); err == nil {
			sid = oteltrace.SpanID(b)
		}
	}
	if logCfg.TraceID != "" {
		if b, err := hex.DecodeString(logCfg.TraceID); err == nil {
			tid = oteltrace.TraceID(b)
		}
	}

	// Create log record using the same logic as the logs system
	rf := logtest.RecordFactory{
		Timestamp:         logTime,
		ObservedTimestamp: time.Now(), // Set to current time when log is observed
		Severity:          severityEnum,
		SeverityText:      severityText,
		Body:              log.StringValue(body),
		Attributes:        attrs,
		TraceID:           tid,
		SpanID:            sid,
		Resource:          w.getResourceForSpan(span), // Use dynamic resource
		DroppedAttributes: 1,
	}

	logRecord := rf.NewRecord()

	// Export the log record
	return w.logExporter.Export(context.Background(), []sdklog.Record{logRecord})
}

func (w *EnhancedWorker) exportMetricViaMetricsSystem(cfg *metrics.Config, metricTime time.Time, span *Span, spanIndex int, metricIndex int) error {
	// Use the existing metrics system logic but for single metric export
	// This reuses the same logic as simulateMetrics() without the goroutine loop

	// Get processed attribute KeyValues (including mock marker logic)
	attrKVs, err := cfg.GetTelemetryAttrWithMockMarker()
	if err != nil {
		return fmt.Errorf("failed to process telemetry attributes: %w", err)
	}

	// Parse trace and span IDs for exemplars
	var tid oteltrace.TraceID
	var sid oteltrace.SpanID
	if cfg.SpanID != "" {
		if b, err := hex.DecodeString(cfg.SpanID); err == nil {
			sid = oteltrace.SpanID(b)
		}
	}
	if cfg.TraceID != "" {
		if b, err := hex.DecodeString(cfg.TraceID); err == nil {
			tid = oteltrace.TraceID(b)
		}
	}

	// Create exemplars from trace context
	var exemplars []metricdata.Exemplar[int64]
	if tid != [16]byte{} && sid != [8]byte{} {
		exemplars = []metricdata.Exemplar[int64]{
			{
				TraceID: tid[:],
				SpanID:  sid[:],
				Value:   int64(spanIndex + metricIndex), // Use deterministic value
				Time:    metricTime,
			},
		}
	}

	// Create metric data using the same logic as the metrics system
	var metrics []metricdata.Metrics
	value := int64(spanIndex + metricIndex) // Deterministic value

	switch cfg.MetricType {
	case "Gauge":
		metrics = append(metrics, metricdata.Metrics{
			Name: cfg.MetricName,
			Data: metricdata.Gauge[int64]{
				DataPoints: []metricdata.DataPoint[int64]{
					{
						Time:       metricTime,
						Value:      value,
						Attributes: attribute.NewSet(attrKVs...),
						Exemplars:  exemplars,
					},
				},
			},
		})
	case "Sum":
		metrics = append(metrics, metricdata.Metrics{
			Name: cfg.MetricName,
			Data: metricdata.Sum[int64]{
				IsMonotonic: true,
				Temporality: metricdata.CumulativeTemporality,
				DataPoints: []metricdata.DataPoint[int64]{
					{
						StartTime:  metricTime.Add(-time.Second),
						Time:       metricTime,
						Value:      value,
						Attributes: attribute.NewSet(attrKVs...),
						Exemplars:  exemplars,
					},
				},
			},
		})
	case "Histogram":
		// Use deterministic histogram data
		bucketCounts := []uint64{0, 0, 1, 0, 0, 0, 3, 4, 1, 1, 0, 0, 0, 0, 0, 0}
		var totalCount uint64
		for _, count := range bucketCounts {
			totalCount += count
		}
		metrics = append(metrics, metricdata.Metrics{
			Name: cfg.MetricName,
			Data: metricdata.Histogram[int64]{
				Temporality: metricdata.CumulativeTemporality,
				DataPoints: []metricdata.HistogramDataPoint[int64]{
					{
						StartTime:    metricTime.Add(-time.Second),
						Time:         metricTime,
						Attributes:   attribute.NewSet(attrKVs...),
						Exemplars:    exemplars,
						Count:        totalCount,
						Sum:          safeInt64FromUint64(totalCount) * 100, // totalCount is small and bounded, safe for int64
						Bounds:       []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
						BucketCounts: bucketCounts,
					},
				},
			},
		})
	default:
		return fmt.Errorf("unknown metric type: %s", cfg.MetricType)
	}

	// Create resource metrics
	rm := metricdata.ResourceMetrics{
		Resource:     w.getResourceForSpan(span), // Use dynamic resource
		ScopeMetrics: []metricdata.ScopeMetrics{{Metrics: metrics}},
	}

	// Export the metric
	if err := w.metricExporter.Export(context.Background(), &rm); err != nil {
		return fmt.Errorf("failed to export metric: %w", err)
	}

	// Output metric information to terminal if enabled
	if cfg.TerminalOutput {
		for _, m := range metrics {
			w.displayMetricRecord(m, cfg)
		}
	}

	w.logger.Debug("metric with exemplars successfully exported",
		zap.String("metric_name", cfg.MetricName),
		zap.String("trace_id", cfg.TraceID),
		zap.String("span_id", cfg.SpanID))

	return nil
}

// displayMetricRecord displays a single metric record in terminal output
func (w *EnhancedWorker) displayMetricRecord(m metricdata.Metrics, cfg *metrics.Config) {
	fmt.Println("Generated Metric Record:")
	fmt.Printf("  Name: %s\n", m.Name)

	switch data := m.Data.(type) {
	case metricdata.Gauge[int64]:
		w.displayDataPoints("Gauge", data.DataPoints, cfg)
	case metricdata.Sum[int64]:
		w.displayDataPoints("Sum", data.DataPoints, cfg)
	case metricdata.Histogram[int64]:
		w.displayHistogramDataPoints(data.DataPoints, cfg)
	}

	// Display resource attributes
	fmt.Println("  Resource Attributes:")
	resAttrs, _ := cfg.GetResourceAttrWithMockMarker()
	for _, attr := range resAttrs {
		key := string(attr.Key)
		val := common.FormatAttrValue(attr.Value.AsInterface())
		fmt.Printf("    - %s: %s\n", key, common.FormatSensitiveValue(key, val))
	}
	fmt.Println("--------------------------------------------------")
}

// displayDataPoints displays data points for Gauge and Sum metrics
func (w *EnhancedWorker) displayDataPoints(metricType string, dataPoints []metricdata.DataPoint[int64], _ *metrics.Config) {
	for _, dp := range dataPoints {
		fmt.Printf("  Type: %s\n", metricType)
		if metricType == "Sum" {
			fmt.Printf("  Value: %v\n", dp.Value)
			fmt.Printf("  StartTime: %s\n", dp.StartTime.Format(time.RFC3339Nano))
		} else {
			fmt.Printf("  Value: %v\n", dp.Value)
		}
		fmt.Printf("  Time: %s\n", dp.Time.Format(time.RFC3339Nano))
		w.displayAttributes(dp.Attributes)
		w.displayExemplars(dp.Exemplars)
	}
}

// displayHistogramDataPoints displays data points for Histogram metrics
func (w *EnhancedWorker) displayHistogramDataPoints(dataPoints []metricdata.HistogramDataPoint[int64], _ *metrics.Config) {
	for _, dp := range dataPoints {
		fmt.Printf("  Type: Histogram\n")
		fmt.Printf("  Count: %v\n", dp.Count)
		fmt.Printf("  Sum: %v\n", dp.Sum)
		fmt.Printf("  StartTime: %s\n", dp.StartTime.Format(time.RFC3339Nano))
		fmt.Printf("  Time: %s\n", dp.Time.Format(time.RFC3339Nano))
		w.displayAttributes(dp.Attributes)
		w.displayExemplars(dp.Exemplars)
	}
}

// displayAttributes displays metric attributes
func (w *EnhancedWorker) displayAttributes(attrs attribute.Set) {
	fmt.Println("  Attributes:")
	for i := 0; i < attrs.Len(); i++ {
		kv := attrs.ToSlice()[i]
		key := string(kv.Key)
		val := common.FormatAttrValue(kv.Value.AsInterface())
		fmt.Printf("    - %s: %s\n", key, common.FormatSensitiveValue(key, val))
	}
}

// displayExemplars displays metric exemplars with trace and span IDs
func (w *EnhancedWorker) displayExemplars(exemplars []metricdata.Exemplar[int64]) {
	if len(exemplars) > 0 {
		ex := exemplars[0]
		traceID := "-"
		if len(ex.TraceID) == 16 {
			traceID = hex.EncodeToString(ex.TraceID)
		}
		spanID := "-"
		if len(ex.SpanID) == 8 {
			spanID = hex.EncodeToString(ex.SpanID)
		}
		fmt.Printf("  TraceID: %s\n", traceID)
		fmt.Printf("  SpanID: %s\n", spanID)
	}
}

// collectLogsForSpan generates and collects logs for a span without outputting them immediately
func (w *EnhancedWorker) collectLogsForSpan(_ context.Context, span *Span, otelSpan oteltrace.Span, spanIndex int, spanData *SpanSignals) {
	// Generate deterministic number of logs per span
	numLogs := (spanIndex % maxSignalsPerSpan) + minSignalsPerSpan

	// Calculate span duration for realistic log timing
	spanDuration := span.EndTime.Sub(span.StartTime)
	if spanDuration <= 0 {
		spanDuration = defaultSpanDuration
	}

	for logIndex := 0; logIndex < numLogs; logIndex++ {
		// Calculate realistic log timestamp within span duration
		logOffset := time.Duration(spanIndex+logIndex) * microsecondOffset
		if logOffset > spanDuration {
			logOffset = spanDuration / 2
		}
		logTime := span.StartTime.Add(logOffset)

		// Generate log config using the span's trace ID and span ID
		logCfg := &logs.Config{
			Body:         w.generateLogMessage(span, spanIndex, logIndex),
			SeverityText: w.generateLogSeverity(spanIndex, logIndex),
			TraceID:      otelSpan.SpanContext().TraceID().String(),
			SpanID:       otelSpan.SpanContext().SpanID().String(),
		}
		// Set terminal output from the embedded config
		logCfg.TerminalOutput = w.config.TerminalOutput

		// Track log statistics
		w.logsGenerated.Add(1)
		if counter, exists := w.logsBySeverity[logCfg.SeverityText]; exists {
			counter.Add(1)
		}

		// Export log to OpenTelemetry using the existing logs system logic
		if w.logExporter != nil {
			if err := w.exportLogViaLogsSystem(logCfg, logTime, span); err != nil {
				w.logger.Error("failed to export log to OpenTelemetry", zap.Error(err))
			}
		}

		// Also add log as span event for trace UI visibility
		otelSpan.AddEvent("log.generated", oteltrace.WithAttributes(
			attribute.String("log.severity", logCfg.SeverityText),
			attribute.String("log.message", logCfg.Body),
			attribute.String("log.timestamp", logTime.Format(time.RFC3339Nano)),
		))

		// If log is error severity, mark span as error
		if logCfg.SeverityText == "error" {
			otelSpan.SetStatus(codes.Error, logCfg.Body)
			otelSpan.AddEvent("log.error", oteltrace.WithAttributes(
				attribute.String("log.message", logCfg.Body),
				attribute.String("log.timestamp", logTime.Format(time.RFC3339Nano)),
			))
		}

		// Collect the log config for later display
		spanData.logs = append(spanData.logs, logCfg)
	}
}

// collectMetricsForSpan generates and collects metrics for a span without outputting them immediately
func (w *EnhancedWorker) collectMetricsForSpan(_ context.Context, span *Span, otelSpan oteltrace.Span, spanIndex int, spanData *SpanSignals) {
	// Generate deterministic number of metrics per span
	numMetrics := (spanIndex % maxSignalsPerSpan) + minSignalsPerSpan

	// Calculate span duration for realistic metric timing
	spanDuration := span.EndTime.Sub(span.StartTime)
	if spanDuration <= 0 {
		spanDuration = defaultSpanDuration
	}

	for metricIndex := 0; metricIndex < numMetrics; metricIndex++ {
		// Calculate realistic metric timestamp within span duration
		metricOffset := time.Duration(spanIndex+metricIndex) * microsecondOffset
		if metricOffset > spanDuration {
			metricOffset = spanDuration / 2
		}
		metricTime := span.StartTime.Add(metricOffset)

		// Generate metric config using the existing metrics system
		// Use the span's trace ID and span ID for exemplars
		metricCfg := &metrics.Config{
			MetricName: w.generateMetricName(spanIndex, metricIndex),
			MetricType: w.generateMetricType(spanIndex, metricIndex),
			TraceID:    otelSpan.SpanContext().TraceID().String(),
			SpanID:     otelSpan.SpanContext().SpanID().String(),
		}
		// Set terminal output from the embedded config
		metricCfg.TerminalOutput = w.config.TerminalOutput

		// Track metric statistics
		w.metricsGenerated.Add(1)
		if counter, exists := w.metricsByType[metricCfg.MetricType.String()]; exists {
			counter.Add(1)
		}

		// Export metric using the existing metrics system logic
		if w.metricExporter != nil {
			if err := w.exportMetricViaMetricsSystem(metricCfg, metricTime, span, spanIndex, metricIndex); err != nil {
				w.logger.Error("failed to export metric to OpenTelemetry", zap.Error(err))
			}
		}

		// Also add metric as span event for trace UI visibility
		otelSpan.AddEvent("metric.generated", oteltrace.WithAttributes(
			attribute.String("metric.name", metricCfg.MetricName),
			attribute.String("metric.type", metricCfg.MetricType.String()),
			attribute.String("metric.timestamp", metricTime.Format(time.RFC3339Nano)),
		))

		// Collect the metric config for later display
		spanData.metrics = append(spanData.metrics, metricCfg)
	}
}

// displaySpansSequentially displays all spans with their associated signals in a sequential, organized manner
func (w *EnhancedWorker) displaySpansSequentially(spanSignals []SpanSignals) {
	if !w.config.TerminalOutput || !w.config.PrintDetails {
		return
	}

	for _, spanData := range spanSignals {
		span := spanData.span
		otelSpan := spanData.otelSpan

		// Display span header in the same format as regular traces
		traceIDStr := otelSpan.SpanContext().TraceID().String()
		if traceIDStr == "00000000000000000000000000000000" || traceIDStr == "" {
			traceIDStr = "-"
		}
		spanIDStr := otelSpan.SpanContext().SpanID().String()
		if spanIDStr == "0000000000000000" || spanIDStr == "" {
			spanIDStr = "-"
		}

		// Get parent span ID if available
		parentSpanID := "-"
		if span.ParentID != "" {
			parentSpanID = span.ParentID
		}

		fmt.Printf("Generated Trace Record")
		if span.ParentID != "" {
			fmt.Printf(" (Child Span)")
		}
		fmt.Printf(":\n")
		fmt.Printf("  Name: %s\n", span.Name)
		fmt.Printf("  Service: %s\n", span.Service)
		fmt.Printf("  Kind: %s\n", span.Attributes["span.kind"])
		fmt.Printf("  TraceID: %s\n", traceIDStr)
		fmt.Printf("  SpanID: %s\n", spanIDStr)
		fmt.Printf("  ParentSpanID: %s\n", parentSpanID)
		fmt.Printf("  Status: %s\n", span.Status.String())
		fmt.Printf("  Start: %s\n", span.StartTime.Format(time.RFC3339))
		fmt.Printf("  End:   %s\n", span.EndTime.Format(time.RFC3339))
		fmt.Printf("  Duration: %v\n", span.EndTime.Sub(span.StartTime))

		// Display span attributes
		if len(span.Attributes) > 0 {
			fmt.Printf("  Attributes:\n")
			for key, value := range span.Attributes {
				if key != "span.kind" { // Already displayed above
					val := common.FormatAttrValue(value)
					fmt.Printf("    - %s: %s\n", key, common.FormatSensitiveValue(key, val))
				}
			}
		} else {
			fmt.Printf("  Attributes:\n")
		}

		// Display span events
		if len(spanData.events) > 0 {
			fmt.Printf("  Events:\n")
			for j, event := range spanData.events {
				fmt.Printf("    %d. %s at %s\n", j+1, event.Name, event.Timestamp.Format(time.RFC3339Nano))
				if len(event.Attributes) > 0 {
					for key, value := range event.Attributes {
						val := common.FormatAttrValue(value)
						fmt.Printf("      - %s: %s\n", key, common.FormatSensitiveValue(key, val))
					}
				}
			}
		}

		// Display logs generated within this span
		if len(spanData.logs) > 0 {
			fmt.Printf("  Logs:\n")
			for j, logCfg := range spanData.logs {
				fmt.Printf("    %d. [%s] %s\n", j+1, strings.ToUpper(logCfg.SeverityText), common.FormatSensitiveBody(logCfg.Body))
			}
		}

		// Display metrics generated within this span
		if len(spanData.metrics) > 0 {
			fmt.Printf("  Metrics:\n")
			for j, metricCfg := range spanData.metrics {
				fmt.Printf("    %d. %s (%s)\n", j+1, metricCfg.MetricName, metricCfg.MetricType.String())

				// Show detailed metric information including exemplars
				fmt.Printf("      TraceID: %s\n", metricCfg.TraceID)
				fmt.Printf("      SpanID: %s\n", metricCfg.SpanID)

				// Show exemplar information
				if metricCfg.TraceID != "" && metricCfg.SpanID != "" {
					fmt.Printf("      Exemplar: TraceID=%s, SpanID=%s\n", metricCfg.TraceID, metricCfg.SpanID)
				}
			}
		}

		// Display resource attributes (consistent with other outputs)
		fmt.Printf("  Resource Attributes:\n")
		spanResource := w.getResourceForSpan(span)
		resAttrs := spanResource.Attributes()
		for _, attr := range resAttrs {
			key := string(attr.Key)
			val := common.FormatAttrValue(attr.Value.AsInterface())
			fmt.Printf("    - %s: %s\n", key, common.FormatSensitiveValue(key, val))
		}

		fmt.Println("--------------------------------------------------")
	}
}

// generateSpanName generates a proper span name following OpenTelemetry conventions
func (w *EnhancedWorker) generateSpanName(span *Span) string {
	// Check if this is an HTTP span
	if method, hasMethod := span.Attributes["http.method"]; hasMethod {
		if route, hasRoute := span.Attributes["http.route"]; hasRoute {
			return fmt.Sprintf("%s %s", method, route)
		}
		if url, hasURL := span.Attributes["http.url"]; hasURL {
			return fmt.Sprintf("%s %s", method, url)
		}
	}
	// For non-HTTP spans, use the service name + operation
	return fmt.Sprintf("%s.%s", span.Service, span.Name)
}

// getSpanKindFromString converts string span kind to OpenTelemetry span kind
func (w *EnhancedWorker) getSpanKindFromString(kindStr any) oteltrace.SpanKind {
	if kind, ok := kindStr.(string); ok {
		switch kind {
		case "SERVER":
			return oteltrace.SpanKindServer
		case "CLIENT":
			return oteltrace.SpanKindClient
		case "PRODUCER":
			return oteltrace.SpanKindProducer
		case "CONSUMER":
			return oteltrace.SpanKindConsumer
		default:
			return oteltrace.SpanKindInternal
		}
	}
	return oteltrace.SpanKindInternal
}

// getResourceForSpan creates a resource with the correct service name for the given span
func (w *EnhancedWorker) getResourceForSpan(span *Span) *resource.Resource {
	attrs := []attribute.KeyValue{
		semconv.ServiceName(span.Service), // Use the actual service name from the span
	}

	// Add service attributes from span data if available
	if namespace, ok := span.Attributes["service.namespace"]; ok {
		if ns, ok := namespace.(string); ok {
			attrs = append(attrs, semconv.ServiceNamespace(ns))
		}
	}
	if version, ok := span.Attributes["service.version"]; ok {
		if ver, ok := version.(string); ok {
			attrs = append(attrs, semconv.ServiceVersion(ver))
		}
	}
	if instanceID, ok := span.Attributes["service.instance.id"]; ok {
		if id, ok := instanceID.(string); ok {
			attrs = append(attrs, semconv.ServiceInstanceID(id))
		}
	}

	if w.config.MockSeed != 0 {
		attrs = append(attrs, attribute.String("trazrgen.random.seed", strconv.FormatInt(w.config.MockSeed, 10)))
	} else {
		attrs = append(attrs, attribute.String("trazrgen.random.seed", "timestamp-based"))
	}

	// Add trazr.run.flags attribute when trace-bound feature is enabled
	if w.config.TrazrTraceBound {
		attrs = append(attrs, attribute.String("trazr.run.flags", "tracebound"))
	}

	res, _ := resource.New(context.Background(), resource.WithAttributes(attrs...))
	return res
}

// Helper functions to create exporter options using the same configuration as traces

// ExporterConfig holds common configuration for creating exporters
type ExporterConfig struct {
	traceConfig *Config
	endpoint    string
	httpPath    string
	insecure    bool
	caFile      string
	clientAuth  common.ClientAuth
	skipVerify  bool
	headers     map[string]string
}

// newExporterConfig creates a common exporter configuration from trace config
func newExporterConfig(traceConfig *Config, httpPath string) (*ExporterConfig, error) {
	headers, err := traceConfig.GetHeadersWithMockMarker()
	if err != nil {
		return nil, err
	}

	return &ExporterConfig{
		traceConfig: traceConfig,
		endpoint:    traceConfig.Endpoint(),
		httpPath:    httpPath,
		insecure:    traceConfig.Insecure,
		caFile:      traceConfig.CaFile,
		clientAuth:  traceConfig.ClientAuth,
		skipVerify:  traceConfig.InsecureSkipVerify,
		headers:     headers,
	}, nil
}

func createLogHTTPExporterOptions(traceConfig *Config, logConfig *logs.Config) ([]otlploghttp.Option, error) {
	cfg, err := newExporterConfig(traceConfig, logConfig.HTTPPath)
	if err != nil {
		return nil, err
	}

	httpExpOpt := []otlploghttp.Option{
		otlploghttp.WithEndpoint(cfg.endpoint),
		otlploghttp.WithURLPath(cfg.httpPath),
	}

	if cfg.insecure {
		httpExpOpt = append(httpExpOpt, otlploghttp.WithInsecure())
	} else {
		tlsCfg, err := common.GetTLSCredentialsForHTTPExporter(
			cfg.caFile, cfg.clientAuth, cfg.skipVerify,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to get TLS credentials: %w", err)
		}
		httpExpOpt = append(httpExpOpt, otlploghttp.WithTLSClientConfig(tlsCfg))
	}

	if len(cfg.headers) > 0 {
		httpExpOpt = append(httpExpOpt, otlploghttp.WithHeaders(cfg.headers))
	}

	return httpExpOpt, nil
}

func createLogGRPCExporterOptions(traceConfig *Config, _ *logs.Config) ([]otlploggrpc.Option, error) {
	cfg, err := newExporterConfig(traceConfig, "")
	if err != nil {
		return nil, err
	}

	grpcExpOpt := []otlploggrpc.Option{
		otlploggrpc.WithEndpoint(cfg.endpoint),
	}

	if cfg.insecure {
		grpcExpOpt = append(grpcExpOpt, otlploggrpc.WithInsecure())
	} else {
		credentials, err := common.GetTLSCredentialsForGRPCExporter(
			cfg.caFile, cfg.clientAuth, cfg.skipVerify,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to get TLS credentials: %w", err)
		}
		grpcExpOpt = append(grpcExpOpt, otlploggrpc.WithTLSCredentials(credentials))
	}

	if len(cfg.headers) > 0 {
		grpcExpOpt = append(grpcExpOpt, otlploggrpc.WithHeaders(cfg.headers))
	}

	return grpcExpOpt, nil
}

func createMetricHTTPExporterOptions(traceConfig *Config, metricConfig *metrics.Config) ([]otlpmetrichttp.Option, error) {
	cfg, err := newExporterConfig(traceConfig, metricConfig.HTTPPath)
	if err != nil {
		return nil, err
	}

	httpExpOpt := []otlpmetrichttp.Option{
		otlpmetrichttp.WithEndpoint(cfg.endpoint),
		otlpmetrichttp.WithURLPath(cfg.httpPath),
	}

	if cfg.insecure {
		httpExpOpt = append(httpExpOpt, otlpmetrichttp.WithInsecure())
	} else {
		tlsCfg, err := common.GetTLSCredentialsForHTTPExporter(
			cfg.caFile, cfg.clientAuth, cfg.skipVerify,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to get TLS credentials: %w", err)
		}
		httpExpOpt = append(httpExpOpt, otlpmetrichttp.WithTLSClientConfig(tlsCfg))
	}

	if len(cfg.headers) > 0 {
		httpExpOpt = append(httpExpOpt, otlpmetrichttp.WithHeaders(cfg.headers))
	}

	return httpExpOpt, nil
}

func createMetricGRPCExporterOptions(traceConfig *Config, _ *metrics.Config) ([]otlpmetricgrpc.Option, error) {
	cfg, err := newExporterConfig(traceConfig, "")
	if err != nil {
		return nil, err
	}

	grpcExpOpt := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(cfg.endpoint),
	}

	if cfg.insecure {
		grpcExpOpt = append(grpcExpOpt, otlpmetricgrpc.WithInsecure())
	} else {
		credentials, err := common.GetTLSCredentialsForGRPCExporter(
			cfg.caFile, cfg.clientAuth, cfg.skipVerify,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to get TLS credentials: %w", err)
		}
		grpcExpOpt = append(grpcExpOpt, otlpmetricgrpc.WithTLSCredentials(credentials))
	}

	if len(cfg.headers) > 0 {
		grpcExpOpt = append(grpcExpOpt, otlpmetricgrpc.WithHeaders(cfg.headers))
	}

	return grpcExpOpt, nil
}

// createLogExporter creates a log exporter using the trace configuration
func createLogExporter(config *Config, logConfig *logs.Config, logger *zap.Logger) sdklog.Exporter {
	var err error
	var logExporter sdklog.Exporter

	if config.UseHTTP {
		var exporterOpts []otlploghttp.Option
		exporterOpts, err = createLogHTTPExporterOptions(config, logConfig)
		if err == nil {
			logExporter, err = otlploghttp.New(context.Background(), exporterOpts...)
		}
	} else {
		var exporterOpts []otlploggrpc.Option
		exporterOpts, err = createLogGRPCExporterOptions(config, logConfig)
		if err == nil {
			logExporter, err = otlploggrpc.New(context.Background(), exporterOpts...)
		}
	}
	if err != nil {
		logger.Error("failed to create log exporter", zap.Error(err))
	}

	return logExporter
}

// createMetricExporter creates a metric exporter using the trace configuration
func createMetricExporter(config *Config, metricConfig *metrics.Config, logger *zap.Logger) sdkmetric.Exporter {
	var err error
	var metricExporter sdkmetric.Exporter

	if config.UseHTTP {
		var exporterOpts []otlpmetrichttp.Option
		exporterOpts, err = createMetricHTTPExporterOptions(config, metricConfig)
		if err == nil {
			metricExporter, err = otlpmetrichttp.New(context.Background(), exporterOpts...)
		}
	} else {
		var exporterOpts []otlpmetricgrpc.Option
		exporterOpts, err = createMetricGRPCExporterOptions(config, metricConfig)
		if err == nil {
			metricExporter, err = otlpmetricgrpc.New(context.Background(), exporterOpts...)
		}
	}
	if err != nil {
		logger.Error("failed to create metric exporter", zap.Error(err))
	}

	return metricExporter
}
