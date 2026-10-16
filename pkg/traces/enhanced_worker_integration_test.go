// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/time/rate"

	"github.com/medxops/trazr-gen/pkg/logs"
	"github.com/medxops/trazr-gen/pkg/metrics"
)

func TestEnhancedWorkerLogAndMetricGeneration(t *testing.T) {
	// Create test logger that captures log entries
	logger := zaptest.NewLogger(t)

	// Create configuration
	config := NewConfig()
	config.Enhanced = true
	config.SpansPerTrace = 3
	config.Pattern = "simple"
	config.Services = []string{"api-gateway", "user-service", "database-service"}

	// Create log and metric configs
	logConfig := logs.NewConfig()
	logConfig.CustomEndpoint = "localhost:9999" // Use non-existent endpoint for testing
	logConfig.TerminalOutput = true             // Enable terminal output instead of HTTP calls

	metricConfig := metrics.NewConfig()

	// Create worker state
	var running atomic.Bool
	running.Store(true)
	var wg sync.WaitGroup
	var tracesCounter atomic.Int64
	progressCh := make(chan struct{}, 100)

	// Add to WaitGroup before starting
	wg.Add(1)

	// Create enhanced worker
	worker := NewEnhancedWorker(
		config,
		logConfig,
		metricConfig,
		logger,
		1,             // numTraces
		2*time.Second, // totalDuration
		rate.Limit(1), // limitPerSecond
		&running,
		&wg,
		&tracesCounter,
		progressCh,
		func(msg string) { t.Logf("Progress: %s", msg) },
	)

	// Start the worker
	ctx := context.Background()
	err := worker.Start(ctx)
	require.NoError(t, err)

	// Wait for completion
	time.Sleep(3 * time.Second)
	running.Store(false)

	// Wait for worker to finish
	wg.Wait()

	// Verify that traces were generated
	traceCount := tracesCounter.Load()
	assert.Positive(t, traceCount, "Should have generated at least one trace")

	t.Logf("Generated %d traces with enhanced mode", traceCount)
}

func TestEnhancedWorkerServiceSpecificGeneration(t *testing.T) {
	logger := zaptest.NewLogger(t)

	config := NewConfig()
	config.Enhanced = true
	config.SpansPerTrace = 2
	config.Pattern = "simple"
	config.Services = []string{"database-service", "payment-service"}

	logConfig := logs.NewConfig()
	logConfig.CustomEndpoint = "localhost:9999" // Use non-existent endpoint for testing
	logConfig.TerminalOutput = true             // Enable terminal output instead of HTTP calls

	metricConfig := metrics.NewConfig()

	var running atomic.Bool
	running.Store(true)
	var wg sync.WaitGroup
	var tracesCounter atomic.Int64
	progressCh := make(chan struct{}, 100)

	// Add to WaitGroup before starting
	wg.Add(1)

	worker := NewEnhancedWorker(
		config,
		logConfig,
		metricConfig,
		logger,
		1,
		2*time.Second,
		rate.Limit(1),
		&running,
		&wg,
		&tracesCounter,
		progressCh,
		func(msg string) { t.Logf("Progress: %s", msg) },
	)

	ctx := context.Background()
	err := worker.Start(ctx)
	require.NoError(t, err)

	time.Sleep(3 * time.Second)
	running.Store(false)

	traceCount := tracesCounter.Load()
	assert.Positive(t, traceCount, "Should have generated traces")

	t.Logf("Generated %d traces with database and payment services", traceCount)
}

func TestEnhancedWorkerPatternVariations(t *testing.T) {
	patterns := []string{"simple", "chain", "fan-out", "complex", "random"}

	for _, pattern := range patterns {
		t.Run(pattern, func(t *testing.T) {
			logger := zaptest.NewLogger(t)

			config := NewConfig()
			config.Enhanced = true
			config.SpansPerTrace = 3
			config.Pattern = pattern
			config.Services = []string{"api-gateway"}

			logConfig := logs.NewConfig()
			logConfig.CustomEndpoint = "localhost:9999" // Use non-existent endpoint for testing
			logConfig.TerminalOutput = true             // Enable terminal output instead of HTTP calls

			metricConfig := metrics.NewConfig()

			var running atomic.Bool
			running.Store(true)
			var wg sync.WaitGroup
			var tracesCounter atomic.Int64
			progressCh := make(chan struct{}, 100)

			// Add to WaitGroup before starting
			wg.Add(1)

			worker := NewEnhancedWorker(
				config,
				logConfig,
				metricConfig,
				logger,
				1,
				2*time.Second,
				rate.Limit(1),
				&running,
				&wg,
				&tracesCounter,
				progressCh,
				func(msg string) { t.Logf("Progress: %s", msg) },
			)

			ctx := context.Background()
			err := worker.Start(ctx)
			require.NoError(t, err)

			time.Sleep(3 * time.Second)
			running.Store(false)

			// Wait for worker to finish
			wg.Wait()

			traceCount := tracesCounter.Load()
			assert.Positive(t, traceCount, "Should have generated traces for pattern: %s", pattern)

			t.Logf("Pattern %s: Generated %d traces", pattern, traceCount)
		})
	}
}

func TestEnhancedWorkerErrorSimulation(t *testing.T) {
	logger := zaptest.NewLogger(t)

	config := NewConfig()
	config.Enhanced = true
	config.SpansPerTrace = 5
	config.Pattern = "simple"
	config.ErrorRate = 0.5 // 50% error rate
	config.Services = []string{"api-gateway", "user-service"}

	logConfig := logs.NewConfig()
	logConfig.CustomEndpoint = "localhost:9999" // Use non-existent endpoint for testing
	logConfig.TerminalOutput = true             // Enable terminal output instead of HTTP calls

	metricConfig := metrics.NewConfig()

	var running atomic.Bool
	running.Store(true)
	var wg sync.WaitGroup
	var tracesCounter atomic.Int64
	progressCh := make(chan struct{}, 100)

	// Add to WaitGroup before starting
	wg.Add(1)

	worker := NewEnhancedWorker(
		config,
		logConfig,
		metricConfig,
		logger,
		1,
		2*time.Second,
		rate.Limit(1),
		&running,
		&wg,
		&tracesCounter,
		progressCh,
		func(msg string) { t.Logf("Progress: %s", msg) },
	)

	ctx := context.Background()
	err := worker.Start(ctx)
	require.NoError(t, err)

	time.Sleep(3 * time.Second)
	running.Store(false)

	// Wait for worker to finish
	wg.Wait()

	traceCount := tracesCounter.Load()
	assert.Positive(t, traceCount, "Should have generated traces with error simulation")

	t.Logf("Generated %d traces with 50%% error rate", traceCount)
}

func TestEnhancedWorkerErrorCorrelation(t *testing.T) {
	logger := zaptest.NewLogger(t)

	config := NewConfig()
	config.Enhanced = true
	config.SpansPerTrace = 2
	config.Pattern = "simple"
	config.ErrorRate = 0.8 // High error rate to test error correlation
	config.Services = []string{"api-gateway", "database-service"}

	logConfig := logs.NewConfig()
	metricConfig := metrics.NewConfig()

	var running atomic.Bool
	running.Store(true)
	var wg sync.WaitGroup
	var tracesCounter atomic.Int64
	progressCh := make(chan struct{}, 100)

	// Add to WaitGroup before starting
	wg.Add(1)

	worker := NewEnhancedWorker(
		config,
		logConfig,
		metricConfig,
		logger,
		1,
		2*time.Second,
		rate.Limit(1),
		&running,
		&wg,
		&tracesCounter,
		progressCh,
		func(msg string) { t.Logf("Progress: %s", msg) },
	)

	ctx := context.Background()
	err := worker.Start(ctx)
	require.NoError(t, err)

	time.Sleep(3 * time.Second)
	running.Store(false)

	// Wait for worker to finish
	wg.Wait()

	traceCount := tracesCounter.Load()
	assert.Positive(t, traceCount, "Should have generated traces with error correlation")

	t.Logf("Generated %d traces with error correlation", traceCount)
}
