// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"github.com/medxops/trazr-gen/internal/common"
)

type worker struct {
	running          *atomic.Bool    // pointer to shared flag that indicates it's time to stop the test
	numTraces        int             // how many traces the worker has to generate (only when duration==0)
	numChildSpans    int             // how many child spans the worker has to generate per trace
	propagateContext bool            // whether the worker needs to propagate the trace context via HTTP headers
	statusCode       codes.Code      // the status code set for the child and parent spans
	totalDuration    time.Duration   // how long to run the test for (overrides `numTraces`)
	limitPerSecond   rate.Limit      // how many spans per second to generate
	wg               *sync.WaitGroup // notify when done
	loadSize         int             // desired minimum size in MB of string data for each generated trace
	spanDuration     time.Duration   // duration of generated spans
	logger           *zap.Logger
	tracesCounter    *int64        // pointer to shared traces counter
	progressCb       func(string)  // optional callback for terminal output
	progressCh       chan struct{} // channel for centralized progress reporting
}

const (
	fakeIP string = "1.2.3.4"

	charactersPerMB = 1024 * 1024 // One character takes up one byte of space, so this number comes from the number of bytes in a megabyte
)

func (w worker) reportProgressf(format string, args ...any) {
	if w.progressCb != nil {
		w.progressCb(fmt.Sprintf(format, args...))
	}
}

// printSpanDebug prints detailed debug output for a span, its attributes, and resource attributes.
func printSpanDebug(label, name, parentSpanID string, kind trace.SpanKind, spanCtx trace.SpanContext, status codes.Code, start, end time.Time, attrs []attribute.KeyValue, resAttrs []attribute.KeyValue) {
	traceID := spanCtx.TraceID().String()
	if traceID == "00000000000000000000000000000000" || traceID == "" {
		traceID = "-"
	}
	spanID := spanCtx.SpanID().String()
	if spanID == "0000000000000000" || spanID == "" {
		spanID = "-"
	}
	if parentSpanID == "0000000000000000" || parentSpanID == "" {
		parentSpanID = "-"
	}
	fmt.Printf("Generated Trace Record%s:\n", label)
	fmt.Printf("  Name: %s\n", name)
	fmt.Printf("  Kind: %s\n", kind.String())
	fmt.Printf("  TraceID: %s\n", traceID)
	fmt.Printf("  SpanID: %s\n", spanID)
	fmt.Printf("  ParentSpanID: %s\n", parentSpanID)
	fmt.Printf("  Status: %s\n", status.String())
	fmt.Printf("  Start: %s\n", start.Format(time.RFC3339Nano))
	fmt.Printf("  End:   %s\n", end.Format(time.RFC3339Nano))
	fmt.Println("  Attributes:")
	for _, attr := range attrs {
		key := string(attr.Key)
		val := common.FormatAttrValue(attr.Value.AsInterface())
		fmt.Printf("    - %s: %s\n", key, common.FormatSensitiveValue(key, val))
	}
	fmt.Println("  Resource Attributes:")
	for _, attr := range resAttrs {
		key := string(attr.Key)
		val := common.FormatAttrValue(attr.Value.AsInterface())
		fmt.Printf("    - %s: %s\n", key, common.FormatSensitiveValue(key, val))
	}
	fmt.Println("--------------------------------------------------")
}

func (w worker) simulateTraces(ctx context.Context, cfg *Config) {
	tracer := otel.Tracer("trazr-gen")
	limiter := rate.NewLimiter(w.limitPerSecond, 1)
	var i int

	for w.running.Load() && ctx.Err() == nil {
		spanStart := time.Now()
		spanEnd := spanStart.Add(w.spanDuration)

		if err := limiter.Wait(ctx); err != nil {
			if ctx.Err() != nil {
				w.logger.Info("context cancelled during rate limiting, stopping")
				break
			}
			w.reportProgressf("Limiter wait failed: %v", err)
			w.logger.Fatal("limiter waited failed, retry", zap.Error(err))
		}

		// Build a fresh set of telemetry attributes for each trace/span
		telemetryAttrs, err := cfg.GetTelemetryAttrWithMockMarker()
		if err != nil {
			w.reportProgressf("Failed to process telemetry attributes: %v", err)
			w.logger.Fatal("failed to process telemetry attributes", zap.Error(err))
		}

		parentSpanName := "lets-go"
		spanCtx, sp := tracer.Start(ctx, parentSpanName, trace.WithAttributes(
			semconv.NetSockPeerAddr(fakeIP),
			semconv.PeerService("trazr-gen-server"),
		),
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithTimestamp(spanStart),
		)
		sp.SetAttributes(telemetryAttrs...)
		for j := 0; j < w.loadSize; j++ {
			sp.SetAttributes(attribute.String(fmt.Sprintf("load-%v", j), string(make([]byte, charactersPerMB))))
		}

		childCtx := spanCtx
		if w.propagateContext {
			header := propagation.HeaderCarrier{}
			// simulates going remote
			otel.GetTextMapPropagator().Inject(childCtx, header)

			// simulates getting a request from a client
			childCtx = otel.GetTextMapPropagator().Extract(childCtx, header)
		}
		var endTimestamp trace.SpanEventOption

		for j := 0; j < w.numChildSpans; j++ {
			if err := limiter.Wait(ctx); err != nil {
				if ctx.Err() != nil {
					w.logger.Info("context cancelled during rate limiting, stopping")
					break
				}
				w.reportProgressf("Limiter wait failed: %v", err)
				w.logger.Fatal("limiter waited failed, retry", zap.Error(err))
			}

			// Build a fresh set of telemetry attributes for each child span
			childAttrs, err := cfg.GetTelemetryAttrWithMockMarker()
			if err != nil {
				w.reportProgressf("Failed to process telemetry attributes: %v", err)
				w.logger.Fatal("failed to process telemetry attributes", zap.Error(err))
				break
			}

			childSpanName := "okey-dokey-" + strconv.Itoa(j)
			parentSpanID := sp.SpanContext().SpanID().String()
			_, child := tracer.Start(childCtx, childSpanName, trace.WithAttributes(
				semconv.NetSockPeerAddr(fakeIP),
				semconv.PeerService("trazr-gen-client"),
			),
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithTimestamp(spanStart),
			)
			child.SetAttributes(childAttrs...)

			endTimestamp = trace.WithTimestamp(spanEnd)
			child.SetStatus(w.statusCode, "")
			child.End(endTimestamp)

			if cfg.TerminalOutput && cfg.PrintDetails {
				spanCtx := child.SpanContext()
				resAttrs, _ := cfg.GetResourceAttrWithMockMarker()
				printSpanDebug(" (Child Span)", childSpanName, parentSpanID, trace.SpanKindServer, spanCtx, w.statusCode, spanStart, spanEnd, childAttrs, resAttrs)
			}

			// Reset the start and end for next span
			spanStart = spanEnd
			spanEnd = spanStart.Add(w.spanDuration)
		}
		sp.SetStatus(w.statusCode, "")
		sp.End(endTimestamp)

		if cfg.TerminalOutput && cfg.PrintDetails {
			spanCtx := sp.SpanContext()
			resAttrs, _ := cfg.GetResourceAttrWithMockMarker()
			printSpanDebug("", parentSpanName, "-", trace.SpanKindClient, spanCtx, w.statusCode, spanStart, spanEnd, telemetryAttrs, resAttrs)
		}

		i++
		if w.tracesCounter != nil {
			atomic.AddInt64(w.tracesCounter, 1)
		}
		if w.progressCh != nil {
			select {
			case w.progressCh <- struct{}{}:
			case <-ctx.Done():
				w.logger.Info("context cancelled during progress reporting, stopping")
				break
			}
		}
		if w.numTraces != 0 {
			if i >= w.numTraces {
				break
			}
		}
	}
	w.logger.Info("traces generated", zap.Int("traces", i))
	w.wg.Done()
}
