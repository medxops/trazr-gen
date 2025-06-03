// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"errors"

	"github.com/spf13/pflag"

	"github.com/medxops/trazr-gen/internal/common"
)

// Config holds all logs subcommand configuration for CLI and config file.
// All fields must have a `mapstructure` tag matching the CLI/config key (dashed, lower-case).
type Config struct {
	common.Config  `mapstructure:",squash"`
	NumLogs        int    `mapstructure:"logs"`
	Body           string `mapstructure:"body"`
	SeverityText   string `mapstructure:"severity-text"`
	SeverityNumber string `mapstructure:"severity-number"`
	TraceID        string `mapstructure:"trace-id"`
	SpanID         string `mapstructure:"span-id"`
}

func NewConfig() *Config {
	cfg := &Config{}
	cfg.SetDefaults()
	return cfg
}

// Flags registers config flags.
func (c *Config) Flags(fs *pflag.FlagSet) {
	c.CommonFlags(fs)

	fs.StringVar(&c.HTTPPath, "otlp-http-url-path", c.HTTPPath, "OTLP HTTP URL path (default: /v1/logs)")

	fs.IntVar(&c.NumLogs, "logs", c.NumLogs, "Number of logs to generate per worker (default: 1)")
	fs.StringVar(&c.Body, "body", c.Body, "Log body message")
	fs.StringVar(&c.SeverityText, "severity-text", c.SeverityText, "Log severity text (e.g., Info, Debug)")
	fs.StringVar(&c.SeverityNumber, "severity-number", c.SeverityNumber, "Log severity number (1-24)")
	fs.StringVar(&c.TraceID, "trace-id", c.TraceID, "TraceID for the log (hex string)")
	fs.StringVar(&c.SpanID, "span-id", c.SpanID, "SpanID for the log (hex string)")
}

// SetDefaults sets the default values for the configuration
// This is called before parsing the command line flags and when
// calling NewConfig()
func (c *Config) SetDefaults() {
	c.Config.SetDefaults()
	c.HTTPPath = "/v1/logs"
	c.NumLogs = 1
	c.Body = "Log message"
	c.SeverityText = "Info"
	c.SeverityNumber = "9"
	c.TraceID = ""
	c.SpanID = ""
}

// Validate validates the test scenario parameters.
func (c *Config) Validate() error {
	if c.TotalDuration <= 0 && c.NumLogs <= 0 {
		return errors.New("either `logs` or `duration` must be greater than 0")
	}

	if c.TraceID != "" {
		if err := common.ValidateTraceID(c.TraceID); err != nil {
			return err
		}
	}

	if c.SpanID != "" {
		if err := common.ValidateSpanID(c.SpanID); err != nil {
			return err
		}
	}

	return nil
}

func (c *Config) GetHeaders() map[string]string {
	return c.Config.GetHeaders()
}

// IsMockDataEnabled returns true if mock data is enabled.
func (c *Config) IsMockDataEnabled() bool {
	return c.MockData
}

// InitAttributes performs one-time initialization of attribute maps for logs config.
func (c *Config) InitAttributes() error {
	return c.Config.InitAttributes()
}
