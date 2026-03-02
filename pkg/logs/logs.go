// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"github.com/medxops/trazr-gen/internal/common"
)

const logsHelpTemplate = `
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

// SetHelpTemplateForCmd sets the custom help template for the logs command.
func SetHelpTemplateForCmd(cmd interface{ SetHelpTemplate(string) }) {
	cmd.SetHelpTemplate(logsHelpTemplate)
}

// Start starts the log telemetry generator
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

	var exporter sdklog.Exporter

	if noExport {
		exporter = &noopLogExporter{}
	} else {
		// Root context canceled on SIGINT/SIGTERM — scoped for exporter creation retry
		expCtx, expCancel := context.WithTimeout(context.Background(), 1*time.Minute)
		defer expCancel()

		deadline := time.Now().Add(1 * time.Minute)
		backoffDelay := 500 * time.Millisecond
		maxDelay := 10 * time.Second
		for {
			var err error
			exporter, err = createExporter(cfg, logger)
			if err == nil {
				break
			}
			if time.Now().After(deadline) || expCtx.Err() != nil {
				logger.Error("giving up creating logs exporter", zap.Error(err))
				return err
			}
			logger.Warn("failed to create logs exporter, will retry", zap.Error(err), zap.Duration("next_backoff", backoffDelay))
			select {
			case <-time.After(backoffDelay):
				backoffDelay *= 2
				if backoffDelay > maxDelay {
					backoffDelay = maxDelay
				}
			case <-expCtx.Done():
				logger.Error("canceled while creating logs exporter", zap.Error(expCtx.Err()))
				return expCtx.Err()
			}
		}

		// Ensure exporter is shut down once after generation completes
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			logger.Info("stopping the exporter")
			if tempError := exporter.Shutdown(shutdownCtx); tempError != nil {
				logger.Error("failed to stop the exporter", zap.Error(tempError))
			}
		}()
	}

	// Root context canceled on SIGINT/SIGTERM
	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info("starting the logs generator with configuration", zap.Any("config", cfg))
	if cfg.TerminalOutput {
		fmt.Println("Starting logs generator")
	}

	if err := run(rootCtx, cfg, exporter, logger); err != nil {
		logger.Error("failed to run logs generator", zap.Error(err))
		return err
	}

	return nil
}

// run executes the test scenario.
func run(rootCtx context.Context, c *Config, exporter sdklog.Exporter, logger *zap.Logger) error {
	if err := c.Validate(); err != nil {
		return err
	}

	if c.TotalDuration > 0 {
		c.NumLogs = 0
	}

	limit := rate.Limit(c.Rate)
	if c.Rate == 0 {
		limit = rate.Inf
		logger.Info("generation of logs isn't being throttled")
	} else {
		logger.Info("generation of logs is limited", zap.Float64("per-second", float64(limit)))
	}

	wg := sync.WaitGroup{}
	attrs, err := c.GetResourceAttrWithMockMarker()
	if err != nil {
		logger.Fatal("failed to process resource attributes", zap.Error(err))
		return err
	}

	// Add trazr.run.flags attribute when time-bound feature is enabled
	if c.TrazrTimeBound {
		attrs = append(attrs, attribute.String("trazr.run.flags", "timebound"))
	}

	res := resource.NewWithAttributes(semconv.SchemaURL, attrs...)

	running := &atomic.Bool{}
	running.Store(true)
	// Stop promptly on Ctrl-C
	go func() {
		<-rootCtx.Done()
		running.Store(false)
		logger.Info("received interrupt signal, stopping all workers")
	}()

	var totalLogs int64

	progressCh := make(chan struct{})
	go func() {
		count := 0
		for range progressCh {
			count++
			if c.TerminalOutput && c.ShowProgressCounter {
				fmt.Println("Logs generated:", count)
			}
		}
		if c.TerminalOutput && c.ShowProgressCounter {
			fmt.Println("Logs generated (final count):", count)
		}
	}()

	// Generate session ID once for all workers to ensure consistency
	sessionID := GenerateTrazrTimeBoundName(c.MockSeed)

	for i := 0; i < c.WorkerCount; i++ {
		wg.Add(1)
		var progressCb func(string)
		if c.TerminalOutput {
			progressCb = func(msg string) { fmt.Println(msg) }
		}
		w := worker{
			numLogs:            c.NumLogs,
			limitPerSecond:     limit,
			body:               c.Body,
			severityText:       c.SeverityText,
			severityNumber:     c.SeverityNumber,
			totalDuration:      c.TotalDuration,
			running:            running,
			wg:                 &wg,
			logger:             logger.With(zap.Int("worker", i+1)),
			index:              i,
			traceID:            c.TraceID,
			spanID:             c.SpanID,
			logsCounter:        &totalLogs,
			progressCh:         progressCh,
			progressCb:         progressCb,
			timeBoundSessionID: sessionID,
		}
		go func() {
			w.simulateLogs(rootCtx, c, res, exporter)
		}()
	}

	if c.TotalDuration > 0 {
		select {
		case <-time.After(c.TotalDuration):
		case <-rootCtx.Done():
		}
		running.Store(false)
	}
	wg.Wait()
	close(progressCh)
	logger.Info("final count", zap.Int64("logs_generated", atomic.LoadInt64(&totalLogs)))

	return nil
}

// noopLogExporter discards all records. Used when no collector is
// reachable so the existing run() logic still drives generation.
type noopLogExporter struct{}

func (n *noopLogExporter) Export(_ context.Context, _ []sdklog.Record) error { return nil }
func (n *noopLogExporter) Shutdown(_ context.Context) error                  { return nil }
func (n *noopLogExporter) ForceFlush(_ context.Context) error                { return nil }

func createExporter(cfg *Config, logger *zap.Logger) (sdklog.Exporter, error) {
	var exp sdklog.Exporter
	var err error
	if cfg.UseHTTP {
		var exporterOpts []otlploghttp.Option

		logger.Info("starting HTTP exporter")
		exporterOpts, err = httpExporterOptions(cfg)
		if err != nil {
			return nil, err
		}
		exp, err = otlploghttp.New(context.Background(), exporterOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to obtain OTLP HTTP exporter: %w", err)
		}
	} else {
		var exporterOpts []otlploggrpc.Option

		logger.Info("starting gRPC exporter")
		exporterOpts, err = grpcExporterOptions(cfg)
		if err != nil {
			return nil, err
		}
		exp, err = otlploggrpc.New(context.Background(), exporterOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to obtain OTLP gRPC exporter: %w", err)
		}
	}
	return exp, err
}

// severityTextFromNumber returns the canonical OpenTelemetry severity text for a given number.
func severityTextFromNumber(severityNumber int32) string {
	return common.SeverityTextFromNumber(severityNumber)
}

// OutputLog outputs a single log entry using the same logic as the logs worker.
func OutputLog(cfg *Config, logger *zap.Logger) {
	msg := cfg.Body
	fields := []zap.Field{
		zap.String("trace_id", cfg.TraceID),
		zap.String("span_id", cfg.SpanID),
		zap.String("severity", cfg.SeverityText),
	}

	// Output to logger
	switch cfg.SeverityText {
	case "debug":
		logger.Debug(msg, fields...)
	case "warn":
		logger.Warn(msg, fields...)
	case "error":
		logger.Error(msg, fields...)
	default:
		logger.Info(msg, fields...)
	}

	// Output to terminal if enabled
	if cfg.TerminalOutput {
		traceIDStr := cfg.TraceID
		if traceIDStr == "00000000000000000000000000000000" || traceIDStr == "" {
			traceIDStr = "-"
		}
		spanIDStr := cfg.SpanID
		if spanIDStr == "0000000000000000" || spanIDStr == "" {
			spanIDStr = "-"
		}

		fmt.Printf("[%s] %s (trace_id: %s, span_id: %s)\n",
			strings.ToUpper(cfg.SeverityText), msg, traceIDStr, spanIDStr)
	}
}
