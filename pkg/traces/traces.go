// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"github.com/medxops/trazr-gen/pkg/logs"
	"github.com/medxops/trazr-gen/pkg/metrics"
)

const tracesHelpTemplate = `
{{with (or .Long .Short)}}{{. | trimTrailingWhitespaces}}{{end}}
{{if .Runnable}}
Usage:
  {{.UseLine}}
{{end}}
{{if .HasAvailableSubCommands}}
Available Commands:
{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}  {{rpad .Name .NamePadding }} {{.Short}}
{{end}}{{end}}
{{end}}
{{if .HasExample}}
Examples:
{{.Example}}
{{end}}
{{if .HasAvailableLocalFlags}}
Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}
{{end}}
{{if .HasAvailableInheritedFlags}}
Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}
{{end}}
{{if .HasHelpSubCommands}}
Additional help topics:
{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}
{{end}}{{end}}
{{end}}
Tip: Use "--mock-data" to generate fake values for attributes and headers!`

// SetHelpTemplateForCmd sets the custom help template for the traces command.
func SetHelpTemplateForCmd(cmd interface{ SetHelpTemplate(string) }) {
	cmd.SetHelpTemplate(tracesHelpTemplate)
}

func Start(cfg *Config, logger *zap.Logger) error {
	if err := cfg.InitAttributes(); err != nil {
		logger.Error("failed to initialize attributes", zap.Error(err))
		return err
	}

	noExport := !cfg.IsEndpointReachable()
	if noExport {
		logger.Info("no collector reachable, skipping export", zap.String("endpoint", cfg.Endpoint()))
		if cfg.TerminalOutput {
			fmt.Printf("No collector at %s — skipping export\n", cfg.Endpoint())
		}
	}

	var ssp sdktrace.SpanProcessor

	if noExport {
		// Use a no-op span exporter so the full run() pipeline still executes
		// (rate limiting, duration, mock data, terminal output) without network I/O
		ssp = sdktrace.NewSimpleSpanProcessor(&noopSpanExporter{})
	} else {
		var exp *otlptrace.Exporter
		// Exporter creation bounded by timeout and signal-aware
		expCtx, expCancel := context.WithTimeout(context.Background(), 1*time.Minute)
		defer expCancel()

		deadline := time.Now().Add(1 * time.Minute)
		backoffDelay := 500 * time.Millisecond
		maxDelay := 10 * time.Second
		for {
			var err error
			if cfg.UseHTTP {
				var exporterOpts []otlptracehttp.Option
				logger.Info("starting HTTP exporter")
				exporterOpts, err = httpExporterOptions(cfg)
				if err == nil {
					exp, err = otlptracehttp.New(context.Background(), exporterOpts...)
				}
			} else {
				var exporterOpts []otlptracegrpc.Option
				logger.Info("starting gRPC exporter")
				exporterOpts, err = grpcExporterOptions(cfg)
				if err == nil {
					exp, err = otlptracegrpc.New(context.Background(), exporterOpts...)
				}
			}
			if err == nil {
				break
			}

			if time.Now().After(deadline) || expCtx.Err() != nil {
				logger.Error("giving up creating trace exporter", zap.Error(err))
				return err
			}
			logger.Warn("failed to create trace exporter, will retry", zap.Error(err), zap.Duration("next_backoff", backoffDelay))
			select {
			case <-time.After(backoffDelay):
				backoffDelay *= 2
				if backoffDelay > maxDelay {
					backoffDelay = maxDelay
				}
			case <-expCtx.Done():
				logger.Error("canceled while creating trace exporter", zap.Error(expCtx.Err()))
				return expCtx.Err()
			}
		}

		if cfg.Batch {
			ssp = sdktrace.NewBatchSpanProcessor(exp, sdktrace.WithBatchTimeout(time.Second))
		} else {
			ssp = sdktrace.NewSimpleSpanProcessor(exp)
		}

		// Ensure exporter is shut down after generation completes
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			logger.Info("stopping the exporter")
			if tempError := exp.Shutdown(shutdownCtx); tempError != nil {
				logger.Error("failed to stop the exporter", zap.Error(tempError))
			}
		}()
	}

	attrs, err := cfg.GetResourceAttrWithMockMarker()
	if err != nil {
		logger.Error("failed to process resource attributes", zap.Error(err))
		return err
	}

	// Add trazr.run.flags attribute when trace-bound feature is enabled
	if cfg.TrazrTraceBound {
		attrs = append(attrs, attribute.String("trazr.run.flags", "tracebound"))
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(resource.NewWithAttributes(semconv.SchemaURL, attrs...)),
	)

	// Always register the span processor to ensure traces are exported
	tracerProvider.RegisterSpanProcessor(ssp)

	otel.SetTracerProvider(tracerProvider)

	// Shutdown tracer provider after generation completes
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		logger.Info("shutting down tracer provider")
		if tempError := tracerProvider.Shutdown(shutdownCtx); tempError != nil {
			logger.Error("failed to shutdown tracer provider", zap.Error(tempError))
		}
	}()

	// Root context canceled on SIGINT/SIGTERM
	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if cfg.TerminalOutput {
		fmt.Println("Starting traces generator")
	}
	logger.Info("starting the traces generator", zap.Int("workers", cfg.WorkerCount))

	if err := run(rootCtx, cfg, logger); err != nil {
		logger.Error("failed to run the traces generator", zap.Error(err))
		return err
	}

	logger.Info("traces generation completed, returning from Start function")
	return nil
}

// run executes the test scenario.
func run(ctx context.Context, c *Config, logger *zap.Logger) error {
	if err := c.Validate(); err != nil {
		return err
	}

	if c.TotalDuration > 0 {
		c.NumTraces = 0
	}

	limit := rate.Limit(c.Rate)
	if c.Rate == 0 {
		limit = rate.Inf
		logger.Info("generation of traces isn't being throttled")
	} else {
		logger.Info("generation of traces is limited", zap.Float64("per-second", float64(limit)))
	}

	// Check if we should use enhanced trace generation
	useEnhanced := c.Enhanced

	if useEnhanced {
		logger.Info("using enhanced trace generation with new parameters")
		return runEnhanced(ctx, c, logger, limit)
	}

	// Use existing worker for backward compatibility
	return runLegacy(ctx, c, logger, limit)
}

// runEnhanced uses the new enhanced worker with realistic trace generation
func runEnhanced(ctx context.Context, c *Config, logger *zap.Logger, limit rate.Limit) error {
	wg := sync.WaitGroup{}
	running := &atomic.Bool{}
	running.Store(true)
	// Stop promptly on Ctrl-C
	go func() {
		<-ctx.Done()
		running.Store(false)
		logger.Info("received interrupt signal, stopping all workers")
	}()
	totalTraces := &atomic.Int64{}

	progressCh := make(chan struct{})
	go func() {
		count := 0
		for range progressCh {
			count++
			if c.TerminalOutput && c.ShowProgressCounter {
				fmt.Println("Enhanced traces generated:", count)
			}
		}
		if c.TerminalOutput && c.ShowProgressCounter {
			fmt.Println("Enhanced traces generated (final count):", count)
		}
	}()

	// Create log and metric configs for integration
	logConfig := &logs.Config{}
	logConfig.SetDefaults()
	metricConfig := &metrics.Config{}
	metricConfig.SetDefaults()

	// Store enhanced workers to access their statistics later
	enhancedWorkers := make([]*EnhancedWorker, 0, c.WorkerCount)

	for i := 0; i < c.WorkerCount; i++ {
		wg.Add(1)
		var progressCb func(string)
		if c.TerminalOutput {
			progressCb = func(msg string) { fmt.Println(msg) }
		}

		enhancedWorker := NewEnhancedWorker(
			c,
			logConfig,
			metricConfig,
			logger.With(zap.Int("worker", i+1)),
			c.NumTraces,
			c.TotalDuration,
			limit,
			running,
			&wg,
			totalTraces,
			progressCh,
			progressCb,
		)
		enhancedWorkers = append(enhancedWorkers, enhancedWorker)

		// Start enhanced worker in a goroutine
		go func() {
			if err := enhancedWorker.Start(ctx); err != nil {
				logger.Error("enhanced worker failed", zap.Error(err))
			}
		}()
	}

	if c.TotalDuration > 0 {
		select {
		case <-time.After(c.TotalDuration):
		case <-ctx.Done():
		}
		running.Store(false)
	}
	wg.Wait()
	close(progressCh)

	// Get final statistics from the enhanced worker
	if len(enhancedWorkers) > 0 {
		stats := enhancedWorkers[0].GetStatistics()
		if c.TerminalOutput {
			fmt.Printf("Enhanced traces generated (final count): %d\n", totalTraces.Load())
			fmt.Printf("Logs generated: %d\n", stats["logs"])
			// Show logs by severity
			for severity, count := range stats {
				if strings.HasPrefix(severity, "logs_") && count > 0 {
					severityName := strings.TrimPrefix(severity, "logs_")
					fmt.Printf("  - %s logs: %d\n", severityName, count)
				}
			}
			fmt.Printf("Metrics generated: %d\n", stats["metrics"])
			// Show metrics by type
			for metricType, count := range stats {
				if strings.HasPrefix(metricType, "metrics_") && count > 0 {
					typeName := strings.TrimPrefix(metricType, "metrics_")
					fmt.Printf("  - %s metrics: %d\n", typeName, count)
				}
			}
			fmt.Printf("Events generated: %d\n", stats["events"])
		}
		logger.Info("final enhanced count", zap.Int64("traces_generated", totalTraces.Load()))
		logger.Info("final statistics", zap.Any("stats", stats))
	}

	logger.Info("runEnhanced function completed, returning")
	return nil
}

// runLegacy uses the existing worker for backward compatibility
func runLegacy(ctx context.Context, c *Config, logger *zap.Logger, limit rate.Limit) error {
	var statusCode codes.Code

	switch strings.ToLower(c.StatusCode) {
	case "0", "unset", "":
		statusCode = codes.Unset
	case "1", "error":
		statusCode = codes.Error
	case "2", "ok":
		statusCode = codes.Ok
	default:
		return fmt.Errorf("expected `status-code` to be one of (Unset, Error, Ok) or (0, 1, 2), got %q instead", c.StatusCode)
	}

	wg := sync.WaitGroup{}
	running := &atomic.Bool{}
	running.Store(true)
	// Stop promptly on Ctrl-C
	go func() {
		<-ctx.Done()
		running.Store(false)
		logger.Info("received interrupt signal, stopping all workers")
	}()
	var totalTraces int64

	progressCh := make(chan struct{})
	go func() {
		count := 0
		for range progressCh {
			count++
			if c.TerminalOutput && c.ShowProgressCounter {
				fmt.Println("Traces generated:", count)
			}
		}
		if c.TerminalOutput && c.ShowProgressCounter {
			fmt.Println("Traces generated (final count):", count)
		}
	}()

	for i := 0; i < c.WorkerCount; i++ {
		wg.Add(1)
		var progressCb func(string)
		if c.TerminalOutput {
			progressCb = func(msg string) { fmt.Println(msg) }
		}
		w := worker{
			numTraces:        c.NumTraces,
			numChildSpans:    int(math.Max(1, float64(c.NumChildSpans))),
			propagateContext: c.PropagateContext,
			statusCode:       statusCode,
			limitPerSecond:   limit,
			totalDuration:    c.TotalDuration,
			running:          running,
			wg:               &wg,
			logger:           logger.With(zap.Int("worker", i+1)),
			loadSize:         c.LoadSize,
			spanDuration:     c.SpanDuration,
			tracesCounter:    &totalTraces,
			progressCh:       progressCh,
			progressCb:       progressCb,
		}
		go w.simulateTraces(ctx, c)
	}

	if c.TotalDuration > 0 {
		select {
		case <-time.After(c.TotalDuration):
		case <-ctx.Done():
		}
		running.Store(false)
	}

	logger.Info("waiting for workers to complete")
	wg.Wait()
	logger.Info("all workers completed")
	close(progressCh)
	logger.Info("final count", zap.Int64("traces_generated", atomic.LoadInt64(&totalTraces)))
	return nil
}

// noopSpanExporter discards all spans. Used when no collector is
// reachable so the existing run() logic still drives generation.
type noopSpanExporter struct{}

func (n *noopSpanExporter) ExportSpans(_ context.Context, _ []sdktrace.ReadOnlySpan) error {
	return nil
}
func (n *noopSpanExporter) Shutdown(_ context.Context) error { return nil }
