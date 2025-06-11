// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
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

	var exp *otlptrace.Exporter
	if cfg.UseHTTP {
		var exporterOpts []otlptracehttp.Option

		logger.Info("starting HTTP exporter")
		exporterOpts, err := httpExporterOptions(cfg)
		if err != nil {
			logger.Error("failed to process OTLP HTTP", zap.Error(err))
			return err
		}
		exp, err = otlptracehttp.New(context.Background(), exporterOpts...)
		if err != nil {
			logger.Error("failed to obtain OTLP HTTP exporter", zap.Error(err))
			return err
		}
	} else {
		var exporterOpts []otlptracegrpc.Option

		logger.Info("starting gRPC exporter")
		exporterOpts, err := grpcExporterOptions(cfg)
		if err != nil {
			logger.Error("failed to process OTLP gRPC", zap.Error(err))
			return err
		}
		exp, err = otlptracegrpc.New(context.Background(), exporterOpts...)
		if err != nil {
			logger.Error("failed to obtain OTLP gRPC exporter", zap.Error(err))
			return err
		}
	}
	defer func() {
		logger.Info("stopping the exporter")
		if tempError := exp.Shutdown(context.Background()); tempError != nil {
			logger.Error("failed to stop the exporter", zap.Error(tempError))
		}
	}()

	var ssp sdktrace.SpanProcessor
	if cfg.Batch {
		ssp = sdktrace.NewBatchSpanProcessor(exp, sdktrace.WithBatchTimeout(time.Second))
		defer func() {
			logger.Info("stop the batch span processor")

			if tempError := ssp.Shutdown(context.Background()); tempError != nil {
				logger.Error("failed to stop the batch span processor", zap.Error(tempError))
			}
		}()
	}

	attrs, err := cfg.GetResourceAttrWithMockMarker()
	if err != nil {
		logger.Error("failed to process resource attributes", zap.Error(err))
		return err
	}
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(resource.NewWithAttributes(semconv.SchemaURL, attrs...)),
	)

	if cfg.Batch {
		tracerProvider.RegisterSpanProcessor(ssp)
	}

	otel.SetTracerProvider(tracerProvider)

	if cfg.TerminalOutput {
		fmt.Println("Starting traces generator")
	}
	logger.Info("starting the traces generator with configuration", zap.Any("config", cfg))

	if err := run(cfg, logger); err != nil {
		logger.Error("failed to run the traces generator", zap.Error(err))
		return err
	}
	return nil
}

// run executes the test scenario.
func run(c *Config, logger *zap.Logger) error {
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

	var totalTraces int64

	var progressCb func(string)
	if c.TerminalOutput {
		progressCb = func(msg string) { fmt.Println(msg) }
	}

	for i := 0; i < c.WorkerCount; i++ {
		wg.Add(1)

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
			progressCb:       progressCb,
		}

		go w.simulateTraces(c)
	}

	// Progress reporting ticker (only if interval > 0)
	var ticker *time.Ticker
	var done chan struct{}
	if c.ReportingInterval > 0 {
		ticker = time.NewTicker(c.ReportingInterval)
		defer ticker.Stop()
		done = make(chan struct{})
		go func() {
			for {
				select {
				case <-ticker.C:
					logger.Info("progress", zap.Int64("traces_generated", atomic.LoadInt64(&totalTraces)))
					if atomic.LoadInt64(&totalTraces) > 0 && c.TerminalOutput {
						fmt.Println("Traces generated:", atomic.LoadInt64(&totalTraces))
					}
				case <-done:
					return
				}
			}
		}()
	}

	if c.TotalDuration > 0 {
		time.Sleep(c.TotalDuration)
		running.Store(false)
	}
	wg.Wait()
	if done != nil {
		close(done)
	}
	logger.Info("final count", zap.Int64("traces_generated", atomic.LoadInt64(&totalTraces)))
	if atomic.LoadInt64(&totalTraces) > 0 && c.TerminalOutput {
		fmt.Println("Traces generated (final count):", atomic.LoadInt64(&totalTraces))
	}
	return nil
}
