// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
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

	exporter, err := createExporter(cfg, logger)
	if err != nil {
		logger.Error("failed to process OTLP exporter", zap.Error(err))
		return err
	}

	logger.Info("starting the logs generator with configuration", zap.Any("config", cfg))
	if cfg.TerminalOutput {
		fmt.Println("Starting logs generator")
	}

	if err := run(cfg, exporter, logger); err != nil {
		logger.Error("failed to run logs generator", zap.Error(err))
		return err
	}

	return nil
}

// run executes the test scenario.
func run(c *Config, exporter sdklog.Exporter, logger *zap.Logger) error {
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
	res := resource.NewWithAttributes(semconv.SchemaURL, attrs...)

	running := &atomic.Bool{}
	running.Store(true)

	var totalLogs int64

	progressCh := make(chan struct{})
	go func() {
		count := 0
		for range progressCh {
			count++
			if c.TerminalOutput {
				fmt.Println("Logs generated:", count)
			}
		}
		if c.TerminalOutput {
			fmt.Println("Logs generated (final count):", count)
		}
	}()

	for i := 0; i < c.WorkerCount; i++ {
		wg.Add(1)
		w := worker{
			numLogs:        c.NumLogs,
			limitPerSecond: limit,
			body:           c.Body,
			severityText:   c.SeverityText,
			severityNumber: c.SeverityNumber,
			totalDuration:  c.TotalDuration,
			running:        running,
			wg:             &wg,
			logger:         logger.With(zap.Int("worker", i+1)),
			index:          i,
			traceID:        c.TraceID,
			spanID:         c.SpanID,
			logsCounter:    &totalLogs,
			progressCh:     progressCh,
		}
		defer func() {
			w.logger.Info("stopping the exporter")
			if tempError := exporter.Shutdown(context.Background()); tempError != nil {
				w.logger.Error("failed to stop the exporter", zap.Error(tempError))
			}
		}()
		go w.simulateLogs(c, res, exporter)
	}

	if c.TotalDuration > 0 {
		time.Sleep(c.TotalDuration)
		running.Store(false)
	}
	wg.Wait()
	close(progressCh)
	logger.Info("final count", zap.Int64("logs_generated", atomic.LoadInt64(&totalLogs)))

	return nil
}

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

func parseSeverity(severityText string, severityNumber int32) (string, log.Severity, error) {
	sn := log.Severity(severityNumber)
	if sn < log.SeverityTrace1 || sn > log.SeverityFatal4 {
		return "", log.SeverityUndefined, errors.New("severity-number is out of range, the valid range is [1,24]")
	}

	// If severityText is empty, set it based on severityNumber
	if severityText == "" {
		severityText = severityTextFromNumber(severityNumber)
	}

	// severity number should match well-known severityText
	switch severityText {
	case plog.SeverityNumberTrace.String():
		if severityNumber < 1 || severityNumber > 4 {
			return "", 0, fmt.Errorf("severity text %q does not match severity number %d, the valid range is [1,4]", severityText, severityNumber)
		}
	case plog.SeverityNumberDebug.String():
		if severityNumber < 5 || severityNumber > 8 {
			return "", 0, fmt.Errorf("severity text %q does not match severity number %d, the valid range is [5,8]", severityText, severityNumber)
		}
	case plog.SeverityNumberInfo.String():
		if severityNumber < 9 || severityNumber > 12 {
			return "", 0, fmt.Errorf("severity text %q does not match severity number %d, the valid range is [9,12]", severityText, severityNumber)
		}
	case plog.SeverityNumberWarn.String():
		if severityNumber < 13 || severityNumber > 16 {
			return "", 0, fmt.Errorf("severity text %q does not match severity number %d, the valid range is [13,16]", severityText, severityNumber)
		}
	case plog.SeverityNumberError.String():
		if severityNumber < 17 || severityNumber > 20 {
			return "", 0, fmt.Errorf("severity text %q does not match severity number %d, the valid range is [17,20]", severityText, severityNumber)
		}
	case plog.SeverityNumberFatal.String():
		if severityNumber < 21 || severityNumber > 24 {
			return "", 0, fmt.Errorf("severity text %q does not match severity number %d, the valid range is [21,24]", severityText, severityNumber)
		}
	}

	return severityText, sn, nil
}

// severityTextFromNumber returns the canonical OpenTelemetry severity text for a given number.
var severityNumberToText = map[int32]string{
	1:  "Trace",
	2:  "Trace2",
	3:  "Trace3",
	4:  "Trace4",
	5:  "Debug",
	6:  "Debug2",
	7:  "Debug3",
	8:  "Debug4",
	9:  "Info",
	10: "Info2",
	11: "Info3",
	12: "Info4",
	13: "Warn",
	14: "Warn2",
	15: "Warn3",
	16: "Warn4",
	17: "Error",
	18: "Error2",
	19: "Error3",
	20: "Error4",
	21: "Fatal",
	22: "Fatal2",
	23: "Fatal3",
	24: "Fatal4",
}

func severityTextFromNumber(severityNumber int32) string {
	return severityNumberToText[severityNumber]
}
