// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.13.0"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

const metricsHelpTemplate = `
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

// SetHelpTemplateForCmd sets the custom help template for the metrics command.
func SetHelpTemplateForCmd(cmd interface{ SetHelpTemplate(string) }) {
	cmd.SetHelpTemplate(metricsHelpTemplate)
}

// Start starts the metric telemetry generator
func Start(cfg *Config, logger *zap.Logger) error {
	if err := cfg.InitAttributes(); err != nil {
		logger.Error("failed to initialize attributes", zap.Error(err))
		return err
	}

	expF := exporterFactory(cfg, logger)
	exp, err := expF()
	if err != nil {
		logger.Error("failed to create exporter", zap.Error(err))
		return err
	}

	logger.Info("starting the metrics generator with configuration", zap.Any("config", cfg))
	if cfg.TerminalOutput {
		fmt.Println("Starting metrics generator")
	}

	if err = run(cfg, exp, logger); err != nil {
		logger.Error("failed to run metrics generator", zap.Error(err))
		return err
	}
	return nil
}

// run executes the test scenario.
func run(c *Config, exporter sdkmetric.Exporter, logger *zap.Logger) error {
	if err := c.Validate(); err != nil {
		return err
	}

	if c.TotalDuration > 0 {
		c.NumMetrics = 0
	}

	limit := rate.Limit(c.Rate)
	if c.Rate == 0 {
		limit = rate.Inf
		logger.Info("generation of metrics isn't being throttled")
	} else {
		logger.Info("generation of metrics is limited", zap.Float64("per-second", float64(limit)))
	}

	attrs, err := c.GetResourceAttrWithMockMarker()
	if err != nil {
		logger.Fatal("failed to process resource attributes", zap.Error(err))
		return err
	}
	res := resource.NewWithAttributes(semconv.SchemaURL, attrs...)

	wg := sync.WaitGroup{}

	running := &atomic.Bool{}
	running.Store(true)

	var totalMetrics int64

	progressCh := make(chan struct{})
	go func() {
		count := 0
		for range progressCh {
			count++
			if c.TerminalOutput {
				fmt.Println("Metrics generated:", count)
			}
		}
		if c.TerminalOutput {
			fmt.Println("Metrics generated (final count):", count)
		}
	}()

	for i := 0; i < c.WorkerCount; i++ {
		wg.Add(1)
		w := worker{
			numMetrics:             c.NumMetrics,
			metricName:             c.MetricName,
			metricType:             c.MetricType,
			aggregationTemporality: c.AggregationTemporality,
			exemplars:              exemplarsFromConfig(c),
			limitPerSecond:         limit,
			totalDuration:          c.TotalDuration,
			running:                running,
			wg:                     &wg,
			logger:                 logger.With(zap.Int("worker", i+1)),
			index:                  i,
			clock:                  &realClock{},
			metricsCounter:         &totalMetrics,
			progressCh:             progressCh,
		}
		defer func() {
			w.logger.Info("stopping the exporter")
			if tempError := exporter.Shutdown(context.Background()); tempError != nil {
				w.logger.Error("failed to stop the exporter", zap.Error(tempError))
			}
		}()
		go w.simulateMetrics(res, exporter, c)
	}

	if c.TotalDuration > 0 {
		time.Sleep(c.TotalDuration)
		running.Store(false)
	}
	wg.Wait()
	close(progressCh)
	logger.Info("final count", zap.Int64("metrics_generated", atomic.LoadInt64(&totalMetrics)))
	return nil
}

type exporterFunc func() (sdkmetric.Exporter, error)

func exporterFactory(cfg *Config, logger *zap.Logger) exporterFunc {
	return func() (sdkmetric.Exporter, error) {
		return createExporter(cfg, logger)
	}
}

func createExporter(cfg *Config, logger *zap.Logger) (sdkmetric.Exporter, error) {
	var exp sdkmetric.Exporter
	var err error
	if cfg.UseHTTP {
		var exporterOpts []otlpmetrichttp.Option

		logger.Info("starting HTTP exporter")
		exporterOpts, err = httpExporterOptions(cfg)
		if err != nil {
			logger.Error("failed to process OTLP HTTP", zap.Error(err))
			return nil, err
		}
		exp, err = otlpmetrichttp.New(context.Background(), exporterOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to obtain OTLP HTTP exporter: %w", err)
		}
	} else {
		var exporterOpts []otlpmetricgrpc.Option

		logger.Info("starting gRPC exporter")
		exporterOpts, err = grpcExporterOptions(cfg)
		if err != nil {
			logger.Error("failed to process OTLP gRPC", zap.Error(err))
			return nil, err
		}
		exp, err = otlpmetricgrpc.New(context.Background(), exporterOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to obtain OTLP gRPC exporter: %w", err)
		}
	}
	return exp, err
}

func exemplarsFromConfig(c *Config) []metricdata.Exemplar[int64] {
	if c.TraceID != "" || c.SpanID != "" {
		var exemplars []metricdata.Exemplar[int64]

		exemplar := metricdata.Exemplar[int64]{
			Value: 1,
			Time:  time.Now(),
		}

		if c.TraceID != "" {
			// we validated this already during the Validate() function for config
			traceID, _ := hex.DecodeString(c.TraceID)
			exemplar.TraceID = traceID
		}

		if c.SpanID != "" {
			// we validated this already during the Validate() function for config
			spanID, _ := hex.DecodeString(c.SpanID)
			exemplar.SpanID = spanID
		}

		return append(exemplars, exemplar)
	}
	return nil
}
