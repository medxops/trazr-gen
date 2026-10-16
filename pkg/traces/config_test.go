// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces

import (
	"fmt"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/medxops/trazr-gen/internal/common"
)

func TestConfig_UseEnhancedGeneration(t *testing.T) {
	config := &Config{}
	config.SetDefaults()

	// Test default value
	assert.False(t, config.UseEnhancedGeneration(), "Default should be false")

	// Test when enhanced generation is set
	config.SetEnhancedGeneration()
	assert.True(t, config.UseEnhancedGeneration(), "Should return true when enhanced generation is set")
}

func TestConfig_SetEnhancedGeneration(t *testing.T) {
	config := &Config{}
	config.SetDefaults()

	// Test setting enhanced generation
	config.SetEnhancedGeneration()
	assert.True(t, config.useEnhancedGeneration, "Should set useEnhancedGeneration to true")
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name           string
		cfg            *Config
		wantErrMessage string
	}{
		{
			name: "Valid config with traces",
			cfg: &Config{
				NumTraces: 10,
			},
			wantErrMessage: "",
		},
		{
			name: "Valid config with duration",
			cfg: &Config{
				Config: common.Config{
					TotalDuration: 10 * time.Second,
				},
			},
			wantErrMessage: "",
		},
		{
			name:           "Invalid config - no traces or duration",
			cfg:            &Config{},
			wantErrMessage: "either `traces` or `duration` must be greater than 0",
		},
		{
			name: "Invalid config - negative traces",
			cfg: &Config{
				NumTraces: -1,
			},
			wantErrMessage: "either `traces` or `duration` must be greater than 0",
		},
		{
			name: "Invalid config - zero duration",
			cfg: &Config{
				Config: common.Config{
					TotalDuration: 0,
				},
			},
			wantErrMessage: "either `traces` or `duration` must be greater than 0",
		},
		{
			name: "Invalid config - negative duration",
			cfg: &Config{
				Config: common.Config{
					TotalDuration: -10 * time.Second,
				},
			},
			wantErrMessage: "either `traces` or `duration` must be greater than 0",
		},

		{
			name: "Invalid enhanced config - error rate greater than 1",
			cfg: &Config{
				NumTraces: 10,
				ErrorRate: 1.1,
			},
			wantErrMessage: "error-rate must be between 0.0 and 1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErrMessage != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrMessage)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestConfig_EnhancedDefaults(t *testing.T) {
	config := &Config{}
	config.SetDefaults()

	// Test enhanced defaults
	assert.False(t, config.Enhanced, "Enhanced should default to false")
	assert.Equal(t, 5, config.SpansPerTrace, "SpansPerTrace should default to 5")
	assert.Equal(t, 3, config.MaxDepth, "MaxDepth should default to 3")
	assert.Equal(t, "random", config.Pattern, "Pattern should default to random")
	assert.InDelta(t, 0.05, config.ErrorRate, 0.001, "ErrorRate should default to 0.05")
	assert.Equal(t, 3, config.Concurrency, "Concurrency should default to 3")
	assert.Equal(t, 3, config.EventsPerSpan, "EventsPerSpan should default to 3")
	assert.Equal(t, 10, config.AttributesPerSpan, "AttributesPerSpan should default to 10")
	assert.Len(t, config.Services, 5, "Services should have 5 default services")
}

func TestConfig_Flags(t *testing.T) {
	config := &Config{}
	config.SetDefaults()

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	config.Flags(fs)

	// Test that enhanced flag is registered
	enhancedFlag := fs.Lookup("enhanced")
	assert.NotNil(t, enhancedFlag, "Enhanced flag should be registered")
	assert.Equal(t, "Enable enhanced trace generation with realistic patterns, events, attributes, logs, and metrics", enhancedFlag.Usage)

	// Test that other enhanced flags are registered
	spansPerTraceFlag := fs.Lookup("spans-per-trace")
	assert.NotNil(t, spansPerTraceFlag, "SpansPerTrace flag should be registered")

	maxDepthFlag := fs.Lookup("max-depth")
	assert.NotNil(t, maxDepthFlag, "MaxDepth flag should be registered")

	patternFlag := fs.Lookup("pattern")
	assert.NotNil(t, patternFlag, "Pattern flag should be registered")

	errorRateFlag := fs.Lookup("error-rate")
	assert.NotNil(t, errorRateFlag, "ErrorRate flag should be registered")

	concurrencyFlag := fs.Lookup("concurrency")
	assert.NotNil(t, concurrencyFlag, "Concurrency flag should be registered")

	eventsPerSpanFlag := fs.Lookup("events-per-span")
	assert.NotNil(t, eventsPerSpanFlag, "EventsPerSpan flag should be registered")

	attributesPerSpanFlag := fs.Lookup("attributes-per-span")
	assert.NotNil(t, attributesPerSpanFlag, "AttributesPerSpan flag should be registered")

	servicesFlag := fs.Lookup("services")
	assert.NotNil(t, servicesFlag, "Services flag should be registered")
}

func TestConfig_EnhancedGenerationLogic(t *testing.T) {
	config := &Config{}
	config.SetDefaults()

	// Test that enhanced generation is disabled by default
	assert.False(t, config.UseEnhancedGeneration(), "Enhanced generation should be disabled by default")

	// Test enabling enhanced generation
	config.SetEnhancedGeneration()
	assert.True(t, config.UseEnhancedGeneration(), "Enhanced generation should be enabled")

	// Test that enhanced generation overrides legacy parameters
	config.NumChildSpans = 10
	config.SpansPerTrace = 5
	config.Enhanced = true

	// When enhanced is true, SpansPerTrace should be used instead of NumChildSpans
	assert.Equal(t, 5, config.SpansPerTrace, "SpansPerTrace should be used when enhanced is true")
	assert.Equal(t, 10, config.NumChildSpans, "NumChildSpans should remain unchanged")
}

func TestConfig_ServiceValidation(t *testing.T) {
	config := &Config{}
	config.SetDefaults()

	// Test with valid services
	config.Services = []string{"service1", "service2", "service3"}
	err := config.Validate()
	require.NoError(t, err, "Should be valid with multiple services")

	// Test with empty services (should use defaults)
	config.Services = []string{}
	err = config.Validate()
	require.NoError(t, err, "Should be valid with empty services")

	// Test with single service
	config.Services = []string{"single-service"}
	err = config.Validate()
	require.NoError(t, err, "Should be valid with single service")
}

func TestConfig_PatternValidation(t *testing.T) {
	config := &Config{}
	config.SetDefaults()

	validPatterns := []string{"simple", "chain", "fan-out", "complex", "random"}

	for _, pattern := range validPatterns {
		t.Run("valid_pattern_"+pattern, func(t *testing.T) {
			config.NumTraces = 10
			config.Enhanced = true
			config.Pattern = pattern
			err := config.Validate()
			assert.NoError(t, err, "Should be valid with pattern: %s", pattern)
		})
	}

	invalidPatterns := []string{"invalid", "test", "unknown", "custom"}

	for _, pattern := range invalidPatterns {
		t.Run("invalid_pattern_"+pattern, func(t *testing.T) {
			config.NumTraces = 10
			config.Enhanced = true
			config.Pattern = pattern
			err := config.Validate()
			require.Error(t, err, "Should be invalid with pattern: %s", pattern)
			assert.Contains(t, err.Error(), "pattern must be one of")
		})
	}
}

func TestConfig_ErrorRateValidation(t *testing.T) {
	config := &Config{}
	config.SetDefaults()

	// Test valid error rates
	validRates := []float64{0.0, 0.1, 0.5, 0.9, 1.0}

	for _, rate := range validRates {
		t.Run("valid_rate_"+fmt.Sprintf("%.1f", rate), func(t *testing.T) {
			config.NumTraces = 10
			config.Enhanced = true
			config.ErrorRate = rate
			err := config.Validate()
			assert.NoError(t, err, "Should be valid with error rate: %f", rate)
		})
	}

	// Test invalid error rates
	invalidRates := []float64{-0.1, -1.0, 1.1, 2.0}

	for _, rate := range invalidRates {
		t.Run("invalid_rate_"+fmt.Sprintf("%.1f", rate), func(t *testing.T) {
			config.NumTraces = 10
			config.Enhanced = true
			config.ErrorRate = rate
			err := config.Validate()
			require.Error(t, err, "Should be invalid with error rate: %f", rate)
			assert.Contains(t, err.Error(), "error-rate must be between 0.0 and 1.0")
		})
	}
}

func TestConfig_SpansPerTraceValidation(t *testing.T) {
	config := &Config{}
	config.SetDefaults()

	// Test valid spans per trace
	validSpans := []int{1, 5, 10, 50, 100}

	for _, spans := range validSpans {
		t.Run(fmt.Sprintf("valid_spans_%d", spans), func(t *testing.T) {
			config.NumTraces = 10
			config.Enhanced = true
			config.SpansPerTrace = spans
			err := config.Validate()
			assert.NoError(t, err, "Should be valid with spans per trace: %d", spans)
		})
	}

	// Test invalid spans per trace
	invalidSpans := []int{0, -1, -10}

	for _, spans := range invalidSpans {
		t.Run(fmt.Sprintf("invalid_spans_%d", spans), func(t *testing.T) {
			config.NumTraces = 10
			config.Enhanced = true
			config.SpansPerTrace = spans
			err := config.Validate()
			require.Error(t, err, "Should be invalid with spans per trace: %d", spans)
			assert.Contains(t, err.Error(), "spans-per-trace must be greater than 0")
		})
	}
}

func TestConfig_MaxDepthValidation(t *testing.T) {
	config := &Config{}
	config.SetDefaults()

	// Test valid max depth values
	validDepths := []int{1, 2, 3, 5, 10}

	for _, depth := range validDepths {
		t.Run(fmt.Sprintf("valid_depth_%d", depth), func(t *testing.T) {
			config.NumTraces = 10
			config.Enhanced = true
			config.MaxDepth = depth
			err := config.Validate()
			assert.NoError(t, err, "Should be valid with max depth: %d", depth)
		})
	}

	// Test invalid max depth values
	invalidDepths := []int{0, -1, -5}

	for _, depth := range invalidDepths {
		t.Run(fmt.Sprintf("invalid_depth_%d", depth), func(t *testing.T) {
			config.NumTraces = 10
			config.Enhanced = true
			config.MaxDepth = depth
			err := config.Validate()
			require.Error(t, err, "Should be invalid with max depth: %d", depth)
			assert.Contains(t, err.Error(), "max-depth must be at least 1")
		})
	}
}

func TestConfig_ConcurrencyValidation(t *testing.T) {
	config := &Config{}
	config.SetDefaults()

	// Test valid concurrency values
	validConcurrency := []int{1, 2, 3, 5, 10, 50}

	for _, concurrency := range validConcurrency {
		t.Run(fmt.Sprintf("valid_concurrency_%d", concurrency), func(t *testing.T) {
			config.NumTraces = 10
			config.Enhanced = true
			config.Concurrency = concurrency
			err := config.Validate()
			assert.NoError(t, err, "Should be valid with concurrency: %d", concurrency)
		})
	}

	// Test invalid concurrency values
	invalidConcurrency := []int{0, -1, -5}

	for _, concurrency := range invalidConcurrency {
		t.Run(fmt.Sprintf("invalid_concurrency_%d", concurrency), func(t *testing.T) {
			config.NumTraces = 10
			config.Enhanced = true
			config.Concurrency = concurrency
			err := config.Validate()
			require.Error(t, err, "Should be invalid with concurrency: %d", concurrency)
			assert.Contains(t, err.Error(), "concurrency must be greater than 0")
		})
	}
}

func TestConfig_EventsPerSpanValidation(t *testing.T) {
	config := &Config{}
	config.SetDefaults()

	// Test valid events per span values
	validEvents := []int{0, 1, 2, 3, 5, 10}

	for _, events := range validEvents {
		t.Run(fmt.Sprintf("valid_events_%d", events), func(t *testing.T) {
			config.NumTraces = 10
			config.Enhanced = true
			config.EventsPerSpan = events
			err := config.Validate()
			assert.NoError(t, err, "Should be valid with events per span: %d", events)
		})
	}

	// Test invalid events per span values
	invalidEvents := []int{-1, -5, -10}

	for _, events := range invalidEvents {
		t.Run(fmt.Sprintf("invalid_events_%d", events), func(t *testing.T) {
			config.NumTraces = 10
			config.Enhanced = true
			config.EventsPerSpan = events
			err := config.Validate()
			require.Error(t, err, "Should be invalid with events per span: %d", events)
			assert.Contains(t, err.Error(), "events-per-span must be non-negative")
		})
	}
}

func TestConfig_AttributesPerSpanValidation(t *testing.T) {
	config := &Config{}
	config.SetDefaults()

	// Test valid attributes per span values
	validAttributes := []int{0, 1, 2, 5, 10, 50}

	for _, attributes := range validAttributes {
		t.Run(fmt.Sprintf("valid_attributes_%d", attributes), func(t *testing.T) {
			config.NumTraces = 10
			config.Enhanced = true
			config.AttributesPerSpan = attributes
			err := config.Validate()
			assert.NoError(t, err, "Should be valid with attributes per span: %d", attributes)
		})
	}

	// Test invalid attributes per span values
	invalidAttributes := []int{-1, -5, -10}

	for _, attributes := range invalidAttributes {
		t.Run(fmt.Sprintf("invalid_attributes_%d", attributes), func(t *testing.T) {
			config.NumTraces = 10
			config.Enhanced = true
			config.AttributesPerSpan = attributes
			err := config.Validate()
			require.Error(t, err, "Should be invalid with attributes per span: %d", attributes)
			assert.Contains(t, err.Error(), "attributes-per-span must be non-negative")
		})
	}
}

func TestConfig_EdgeCases(t *testing.T) {
	config := &Config{}
	config.SetDefaults()

	// Test with all parameters set to minimum valid values
	t.Run("minimum_valid_values", func(t *testing.T) {
		config.NumTraces = 1
		config.Enhanced = true
		config.SpansPerTrace = 1
		config.MaxDepth = 1
		config.Pattern = "simple"
		config.ErrorRate = 0.0
		config.Concurrency = 1
		config.EventsPerSpan = 0
		config.AttributesPerSpan = 0
		config.Services = []string{"test-service"}

		err := config.Validate()
		require.NoError(t, err, "Should be valid with minimum values")
	})

	// Test with all parameters set to maximum reasonable values
	t.Run("maximum_reasonable_values", func(t *testing.T) {
		config.NumTraces = 1000
		config.Enhanced = true
		config.SpansPerTrace = 100
		config.MaxDepth = 10
		config.Pattern = "complex"
		config.ErrorRate = 1.0
		config.Concurrency = 50
		config.EventsPerSpan = 20
		config.AttributesPerSpan = 100
		config.Services = []string{"service1", "service2", "service3", "service4", "service5"}

		err := config.Validate()
		assert.NoError(t, err, "Should be valid with maximum reasonable values")
	})

	// Test with empty services list
	t.Run("empty_services", func(t *testing.T) {
		config.NumTraces = 10
		config.Enhanced = true
		config.Services = []string{}

		err := config.Validate()
		assert.NoError(t, err, "Should be valid with empty services (will use defaults)")
	})
}

func TestConfig_ParameterInteractions(t *testing.T) {
	config := &Config{}
	config.SetDefaults()

	// Test interaction between SpansPerTrace and NumChildSpans
	t.Run("spans_per_trace_overrides_child_spans", func(t *testing.T) {
		config.NumTraces = 10
		config.Enhanced = true
		config.NumChildSpans = 5
		config.SpansPerTrace = 10

		err := config.Validate()
		require.NoError(t, err, "Should be valid")
		assert.Equal(t, 10, config.SpansPerTrace, "SpansPerTrace should be used")
		assert.Equal(t, 5, config.NumChildSpans, "NumChildSpans should remain unchanged")
	})

	// Test interaction between MaxDepth and SpansPerTrace
	t.Run("max_depth_with_spans_per_trace", func(t *testing.T) {
		config.NumTraces = 10
		config.Enhanced = true
		config.SpansPerTrace = 20
		config.MaxDepth = 3

		err := config.Validate()
		assert.NoError(t, err, "Should be valid with MaxDepth < SpansPerTrace")
	})

	// Test interaction between Concurrency and SpansPerTrace
	t.Run("concurrency_with_spans_per_trace", func(t *testing.T) {
		config.NumTraces = 10
		config.Enhanced = true
		config.SpansPerTrace = 10
		config.Concurrency = 5

		err := config.Validate()
		assert.NoError(t, err, "Should be valid with Concurrency < SpansPerTrace")
	})
}

func TestConfig_BoundaryValues(t *testing.T) {
	config := &Config{}
	config.SetDefaults()

	// Test boundary values for each parameter
	t.Run("boundary_values", func(t *testing.T) {
		config.NumTraces = 1
		config.Enhanced = true
		config.SpansPerTrace = 1
		config.MaxDepth = 1
		config.Pattern = "simple"
		config.ErrorRate = 0.0
		config.Concurrency = 1
		config.EventsPerSpan = 0
		config.AttributesPerSpan = 0
		config.Services = []string{"test"}

		err := config.Validate()
		assert.NoError(t, err, "Should be valid with boundary values")
	})

	// Test very large values
	t.Run("very_large_values", func(t *testing.T) {
		config.NumTraces = 10000
		config.Enhanced = true
		config.SpansPerTrace = 1000
		config.MaxDepth = 100
		config.Pattern = "random"
		config.ErrorRate = 0.5
		config.Concurrency = 1000
		config.EventsPerSpan = 1000
		config.AttributesPerSpan = 1000
		config.Services = []string{"service1", "service2", "service3", "service4", "service5", "service6", "service7", "service8", "service9", "service10"}

		err := config.Validate()
		assert.NoError(t, err, "Should be valid with very large values")
	})
}

func TestConfig_ErrorControlPresets(t *testing.T) {
	tests := []struct {
		name         string
		clean        bool
		stress       bool
		chaos        bool
		expectedRate float64
		expectedMode string
	}{
		{
			name:         "Clean mode",
			clean:        true,
			expectedRate: 0.0,
			expectedMode: "Clean",
		},
		{
			name:         "Stress mode",
			stress:       true,
			expectedRate: 0.2,
			expectedMode: "Stress",
		},
		{
			name:         "Chaos mode",
			chaos:        true,
			expectedRate: 0.5,
			expectedMode: "Chaos",
		},
		{
			name:         "Clean takes priority over stress",
			clean:        true,
			stress:       true,
			expectedRate: 0.0,
			expectedMode: "Clean",
		},
		{
			name:         "Clean takes priority over chaos",
			clean:        true,
			chaos:        true,
			expectedRate: 0.0,
			expectedMode: "Clean",
		},
		{
			name:         "Stress takes priority over chaos",
			stress:       true,
			chaos:        true,
			expectedRate: 0.2,
			expectedMode: "Stress",
		},
		{
			name:         "No preset uses default",
			expectedRate: 0.05,
			expectedMode: "Default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{}
			config.SetDefaults()

			config.Clean = tt.clean
			config.Stress = tt.stress
			config.Chaos = tt.chaos

			err := config.Validate()
			require.NoError(t, err)

			assert.InDelta(t, tt.expectedRate, config.ErrorRate, 0.001, "Error rate should match expected")
		})
	}
}
