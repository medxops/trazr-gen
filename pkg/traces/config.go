// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces

import (
	"errors"
	"time"

	"github.com/spf13/pflag"

	"github.com/medxops/trazr-gen/internal/common"
)

// Config holds all traces subcommand configuration for CLI and config file.
// All fields must have a `mapstructure` tag matching the CLI/config key (dashed, lower-case).
type Config struct {
	common.Config    `mapstructure:",squash"`
	NumTraces        int           `mapstructure:"traces"`
	NumChildSpans    int           `mapstructure:"child-spans"`
	PropagateContext bool          `mapstructure:"marshal"`
	StatusCode       string        `mapstructure:"status-code"`
	Batch            bool          `mapstructure:"batch"`
	LoadSize         int           `mapstructure:"size"`
	SpanDuration     time.Duration `mapstructure:"span-duration"`
	DurationJitter   float64       `mapstructure:"duration-jitter"` // Percentage of base duration for jitter

	// New trace generation parameters (following telemetrygen best practices)
	SpansPerTrace     int      `mapstructure:"spans-per-trace"`
	MaxDepth          int      `mapstructure:"max-depth"`
	Pattern           string   `mapstructure:"pattern"`
	ErrorRate         float64  `mapstructure:"error-rate"`
	Concurrency       int      `mapstructure:"concurrency"`
	EventsPerSpan     int      `mapstructure:"events-per-span"`
	AttributesPerSpan int      `mapstructure:"attributes-per-span"`
	Services          []string `mapstructure:"services"`
	Enhanced          bool     `mapstructure:"enhanced"`       // Enable enhanced trace generation
	EmitOTLPLogs      bool     `mapstructure:"emit-otlp-logs"` // Emit logs as OTLP log records

	// Child span control parameters
	MinChildrenPerSpan   int     `mapstructure:"min-children-per-span"`  // Minimum children per span
	MaxChildrenPerSpan   int     `mapstructure:"max-children-per-span"`  // Maximum children per span
	ChildSpanProbability float64 `mapstructure:"child-span-probability"` // Probability of creating child spans (0.0 to 1.0)

	// Error control presets
	Clean  bool `mapstructure:"clean"`  // Generate clean traces (no errors)
	Stress bool `mapstructure:"stress"` // Generate stress traces (high error rate)
	Chaos  bool `mapstructure:"chaos"`  // Generate chaos traces (very high error rate)

	// Trazr trace-bound feature
	TrazrTraceBound bool `mapstructure:"trazr-trace-bound"` // Enable trazr trace bound for trace generation

	// Output control
	PrintDetails bool `mapstructure:"print-details"` // Print detailed record output to terminal for each item

	// Progress counter control
	ShowProgressCounter bool `mapstructure:"show-progress-counter"` // Show progress counter (Traces generated: X) for each trace

	// Seed for reproducible random generation
	Seed int64 `mapstructure:"seed"` // Seed for reproducible random generation

	// Track if new parameters were explicitly set
	useEnhancedGeneration bool
}

func NewConfig() *Config {
	cfg := &Config{}
	cfg.SetDefaults()
	return cfg
}

// Flags registers config flags.
func (c *Config) Flags(fs *pflag.FlagSet) {
	c.CommonFlags(fs)

	fs.StringVar(&c.HTTPPath, "otlp-http-url-path", c.HTTPPath, "Which URL path to write to")

	// Existing flags (unchanged)
	fs.IntVar(&c.NumTraces, "traces", c.NumTraces, "Number of traces to generate in each worker (ignored if duration is provided)")
	fs.IntVar(&c.NumChildSpans, "child-spans", c.NumChildSpans, "Number of child spans to generate for each trace")
	fs.BoolVar(&c.PropagateContext, "marshal", c.PropagateContext, "Whether to marshal trace context via HTTP headers")
	fs.StringVar(&c.StatusCode, "status-code", c.StatusCode, "Status code to use for the spans, one of (Unset, Error, Ok) or the equivalent integer (0,1,2)")
	fs.BoolVar(&c.Batch, "batch", c.Batch, "Whether to batch traces")
	fs.IntVar(&c.LoadSize, "size", c.LoadSize, "Desired minimum size in MB of string data for each trace generated. This can be used to test traces with large payloads, i.e. when testing the OTLP receiver endpoint max receive size.")
	fs.DurationVar(&c.SpanDuration, "span-duration", c.SpanDuration, "The duration of each generated span.")
	fs.Float64Var(&c.DurationJitter, "duration-jitter", c.DurationJitter, "Percentage of base duration for jitter (0.0 to 1.0, default 0.25)")

	// Enhanced trace generation flags
	fs.BoolVar(&c.Enhanced, "enhanced", c.Enhanced, "Enable enhanced trace generation with realistic patterns, events, attributes, logs, and metrics")
	fs.BoolVar(&c.EmitOTLPLogs, "emit-otlp-logs", c.EmitOTLPLogs, "Emit logs as OpenTelemetry log records (in addition to span events)")

	// Error control flags (now visible)
	fs.Float64Var(&c.ErrorRate, "error-rate", c.ErrorRate, "Probability of span failure (0.0 to 1.0)")
	fs.BoolVar(&c.Clean, "clean", c.Clean, "Generate clean traces with no errors and minimal warnings")
	fs.BoolVar(&c.Stress, "stress", c.Stress, "Generate stress traces with high error rate (20%) for testing error handling")
	fs.BoolVar(&c.Chaos, "chaos", c.Chaos, "Generate chaos traces with very high error rate (50%) for chaos testing")

	// Advanced flags (now visible for better control)
	fs.IntVar(&c.SpansPerTrace, "spans-per-trace", c.SpansPerTrace, "Number of spans per trace (overrides child-spans for enhanced generation)")
	fs.IntVar(&c.MaxDepth, "max-depth", c.MaxDepth, "Maximum nesting depth for spans")
	fs.StringVar(&c.Pattern, "pattern", c.Pattern, "Trace structure pattern: simple, chain, fan-out, complex, random")
	fs.IntVar(&c.Concurrency, "concurrency", c.Concurrency, "Maximum parallel child spans")
	fs.IntVar(&c.EventsPerSpan, "events-per-span", c.EventsPerSpan, "Number of events per span")
	fs.IntVar(&c.AttributesPerSpan, "attributes-per-span", c.AttributesPerSpan, "Number of attributes per span")
	fs.StringSliceVar(&c.Services, "services", c.Services, "Service names to use in trace generation")

	// Child span control flags
	fs.IntVar(&c.MinChildrenPerSpan, "min-children-per-span", c.MinChildrenPerSpan, "Minimum number of children per span (0 = no minimum)")
	fs.IntVar(&c.MaxChildrenPerSpan, "max-children-per-span", c.MaxChildrenPerSpan, "Maximum number of children per span")
	fs.Float64Var(&c.ChildSpanProbability, "child-span-probability", c.ChildSpanProbability, "Probability of creating child spans (0.0 to 1.0)")

	// Hide child span control flags by default (advanced feature)
	_ = fs.MarkHidden("min-children-per-span")
	_ = fs.MarkHidden("max-children-per-span")
	_ = fs.MarkHidden("child-span-probability")

	// Trazr trace-bound flag
	fs.BoolVar(&c.TrazrTraceBound, "trazr-trace-bound", c.TrazrTraceBound, "Enable trazr trace bound for trace generation (default: false)")

	// Output control flag
	fs.BoolVar(&c.PrintDetails, "print-details", c.PrintDetails, "Print detailed record output to terminal for each item (default: true)")

	// Progress counter flag
	fs.BoolVar(&c.ShowProgressCounter, "show-progress-counter", c.ShowProgressCounter, "Show progress counter (Traces generated: X) for each trace (default: true)")
}

// SetDefaults sets the default values for the configuration
// This is called before parsing the command line flags and when
// calling NewConfig()
func (c *Config) SetDefaults() {
	c.Config.SetDefaults()
	c.HTTPPath = "/v1/traces"
	c.NumTraces = 1
	c.NumChildSpans = 1
	c.PropagateContext = false
	c.StatusCode = "0"
	c.Batch = true
	c.LoadSize = 0
	c.SpanDuration = 100 * time.Millisecond
	c.DurationJitter = 0.25 // Default to 25% jitter

	// New defaults for enhanced trace generation
	c.SpansPerTrace = 5
	c.MaxDepth = 3
	c.Pattern = "random"
	c.ErrorRate = 0.05
	c.Concurrency = 3
	c.EventsPerSpan = 3
	c.AttributesPerSpan = 10
	c.Services = []string{"api-gateway", "user-service", "payment-service", "database-service", "cache-service"}
	c.Enhanced = false     // Default to false
	c.EmitOTLPLogs = false // Default to false

	// Child span control defaults
	c.MinChildrenPerSpan = 0     // Default to no minimum
	c.MaxChildrenPerSpan = 3     // Default to max 3 children per span
	c.ChildSpanProbability = 0.7 // Default to 70% chance of creating child spans

	// Error control presets default to false
	c.Clean = false
	c.Stress = false
	c.Chaos = false

	// Trazr trace-bound feature defaults to false
	c.TrazrTraceBound = false

	// Output control defaults to true
	c.PrintDetails = true

	// Progress counter defaults to true
	c.ShowProgressCounter = true
}

// Validate validates the test scenario parameters.
func (c *Config) Validate() error {
	if c.TotalDuration <= 0 && c.NumTraces <= 0 {
		return errors.New("either `traces` or `duration` must be greater than 0")
	}

	// Apply error control presets
	switch {
	case c.Clean:
		c.ErrorRate = 0.0
	case c.Stress:
		c.ErrorRate = 0.2
	case c.Chaos:
		c.ErrorRate = 0.5
	}

	// Validate enhanced parameters if enhanced mode is enabled or if any enhanced parameters are set
	useEnhancedParams := c.Enhanced || c.SpansPerTrace > 0 || c.MaxDepth > 0 || c.Pattern != "" || c.ErrorRate > 0 ||
		c.Concurrency > 0 || c.EventsPerSpan > 0 || c.AttributesPerSpan > 0 || len(c.Services) > 0 ||
		c.MinChildrenPerSpan > 0 || c.MaxChildrenPerSpan > 0 || c.ChildSpanProbability > 0

	if useEnhancedParams {
		// Validate enhanced parameters
		if c.ErrorRate < 0.0 || c.ErrorRate > 1.0 {
			return errors.New("error-rate must be between 0.0 and 1.0")
		}

		if c.MaxDepth < 1 {
			return errors.New("max-depth must be at least 1")
		}

		if c.SpansPerTrace < 1 {
			return errors.New("spans-per-trace must be greater than 0")
		}

		if c.Concurrency < 1 {
			return errors.New("concurrency must be greater than 0")
		}

		if c.EventsPerSpan < 0 {
			return errors.New("events-per-span must be non-negative")
		}

		if c.AttributesPerSpan < 0 {
			return errors.New("attributes-per-span must be non-negative")
		}

		// Validate child span parameters
		if c.MinChildrenPerSpan < 0 {
			return errors.New("min-children-per-span must be non-negative")
		}

		if c.MaxChildrenPerSpan < 0 {
			return errors.New("max-children-per-span must be non-negative")
		}

		if c.MinChildrenPerSpan > c.MaxChildrenPerSpan {
			return errors.New("min-children-per-span cannot be greater than max-children-per-span")
		}

		if c.ChildSpanProbability < 0.0 || c.ChildSpanProbability > 1.0 {
			return errors.New("child-span-probability must be between 0.0 and 1.0")
		}

		validPatterns := map[string]bool{
			"simple": true, "chain": true, "fan-out": true, "complex": true, "random": true,
		}
		if !validPatterns[c.Pattern] {
			return errors.New("pattern must be one of: simple, chain, fan-out, complex, random")
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

// InitAttributes performs one-time initialization of attribute maps for traces config.
func (c *Config) InitAttributes() error {
	return c.Config.InitAttributes()
}

// UseEnhancedGeneration returns true if new parameters were explicitly set
func (c *Config) UseEnhancedGeneration() bool {
	return c.useEnhancedGeneration
}

// SetEnhancedGeneration marks that new parameters were explicitly set
func (c *Config) SetEnhancedGeneration() {
	c.useEnhancedGeneration = true
}
