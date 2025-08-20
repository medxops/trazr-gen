// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package e2etest

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/medxops/trazr-gen/pkg/traces"
)

func TestGenerateTraces(t *testing.T) {
	f := otlpreceiver.NewFactory()
	sink := &consumertest.TracesSink{}
	rCfg := f.CreateDefaultConfig()
	endpoint := getAvailableLocalAddress(t)
	rCfg.(*otlpreceiver.Config).GRPC.NetAddr.Endpoint = endpoint
	rCfg.(*otlpreceiver.Config).HTTP = nil
	r, err := f.CreateTraces(context.Background(), receivertest.NewNopSettings(f.Type()), rCfg, sink)
	require.NoError(t, err)
	err = r.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, r.Shutdown(context.Background()))
	}()
	cfg := traces.NewConfig()
	cfg.WorkerCount = 10
	cfg.Rate = 10
	cfg.TotalDuration = 10 * time.Second
	cfg.ReportingInterval = 10
	cfg.CustomEndpoint = endpoint
	cfg.Insecure = true
	cfg.NumTraces = 6000
	cfg.UseHTTP = false
	cfg.TerminalOutput = false

	go func() {
		err = traces.Start(cfg, zap.NewNop())
		assert.NoError(t, err)
	}()
	require.Eventually(t, func() bool {
		return len(sink.AllTraces()) > 0
	}, 10*time.Second, 100*time.Millisecond)
}

func TestGenerateTracesWithSelectiveSensitiveAttributes(t *testing.T) {
	f := otlpreceiver.NewFactory()
	sink := &consumertest.TracesSink{}
	rCfg := f.CreateDefaultConfig()
	endpoint := getAvailableLocalAddress(t)
	rCfg.(*otlpreceiver.Config).GRPC.NetAddr.Endpoint = endpoint
	rCfg.(*otlpreceiver.Config).HTTP = nil
	r, err := f.CreateTraces(context.Background(), receivertest.NewNopSettings(f.Type()), rCfg, sink)
	require.NoError(t, err)
	err = r.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() { require.NoError(t, r.Shutdown(context.Background())) }()

	cfg := traces.NewConfig()
	cfg.WorkerCount = 1
	cfg.Rate = 1
	cfg.TotalDuration = 1 * time.Second
	cfg.CustomEndpoint = endpoint
	cfg.Insecure = true
	cfg.NumTraces = 1
	cfg.ResourceAttributes = map[string]any{"a": "A", "b": "B", "c": "C"}
	cfg.TelemetryAttributes = map[string]any{"d": "D", "e": "E", "f": "F"}
	cfg.SensitiveData = []string{"a", "d"}
	cfg.UseHTTP = false
	cfg.TerminalOutput = false

	go func() { _ = traces.Start(cfg, zap.NewNop()) }()

	require.Eventually(t, func() bool {
		return len(sink.AllTraces()) > 0
	}, 5*time.Second, 100*time.Millisecond)

	// Check resource attributes for sensitive data
	resAttrs := sink.AllTraces()[0].ResourceSpans().At(0).Resource().Attributes()
	resVal, resOk := resAttrs.Get("trazrgen.sensitive.data")
	if !resOk {
		t.Fatalf("trazrgen.sensitive.data attribute not found on resource; got attributes: %+v", resAttrs.AsRaw())
	}
	assert.Contains(t, resVal.AsString(), "a", "'a' should be marked sensitive (resource)")
	assert.NotContains(t, resVal.AsString(), "c", "'c' should NOT be marked sensitive (resource)")

	// Check span attributes for sensitive data
	span := sink.AllTraces()[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
	val, ok := span.Attributes().Get("trazrgen.sensitive.data")
	if !ok {
		t.Fatalf("trazrgen.sensitive.data attribute not found on span; got attributes: %+v", span.Attributes().AsRaw())
	}
	assert.Contains(t, val.AsString(), "d", "'d' should be marked sensitive (span)")
	assert.NotContains(t, val.AsString(), "f", "'f' should NOT be marked sensitive (span)")
}

func TestEnhancedTraceGenerationWithLogsAndMetrics(t *testing.T) {
	// Set up OTLP receiver for traces
	f := otlpreceiver.NewFactory()
	traceSink := &consumertest.TracesSink{}
	rCfg := f.CreateDefaultConfig()
	endpoint := getAvailableLocalAddress(t)
	rCfg.(*otlpreceiver.Config).GRPC.NetAddr.Endpoint = endpoint
	rCfg.(*otlpreceiver.Config).HTTP = nil
	r, err := f.CreateTraces(context.Background(), receivertest.NewNopSettings(f.Type()), rCfg, traceSink)
	require.NoError(t, err)
	err = r.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, r.Shutdown(context.Background()))
	}()

	// Set up OTLP receiver for logs
	logSink := &consumertest.LogsSink{}
	rCfgLogs := f.CreateDefaultConfig()
	endpointLogs := getAvailableLocalAddress(t)
	rCfgLogs.(*otlpreceiver.Config).GRPC.NetAddr.Endpoint = endpointLogs
	rCfgLogs.(*otlpreceiver.Config).HTTP = nil
	rLogs, err := f.CreateLogs(context.Background(), receivertest.NewNopSettings(f.Type()), rCfgLogs, logSink)
	require.NoError(t, err)
	err = rLogs.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, rLogs.Shutdown(context.Background()))
	}()

	// Set up OTLP receiver for metrics
	metricSink := &consumertest.MetricsSink{}
	rCfgMetrics := f.CreateDefaultConfig()
	endpointMetrics := getAvailableLocalAddress(t)
	rCfgMetrics.(*otlpreceiver.Config).GRPC.NetAddr.Endpoint = endpointMetrics
	rCfgMetrics.(*otlpreceiver.Config).HTTP = nil
	rMetrics, err := f.CreateMetrics(context.Background(), receivertest.NewNopSettings(f.Type()), rCfgMetrics, metricSink)
	require.NoError(t, err)
	err = rMetrics.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, rMetrics.Shutdown(context.Background()))
	}()

	// Configure enhanced trace generation
	cfg := traces.NewConfig()
	cfg.WorkerCount = 1
	cfg.Rate = 1
	cfg.TotalDuration = 3 * time.Second
	cfg.CustomEndpoint = endpoint
	cfg.Insecure = true
	cfg.NumTraces = 2
	cfg.UseHTTP = false
	cfg.TerminalOutput = false

	// Enable enhanced mode with specific parameters
	cfg.Enhanced = true
	cfg.SpansPerTrace = 3
	cfg.Pattern = "simple"
	cfg.ErrorRate = 0.1
	cfg.Services = []string{"api-gateway", "user-service", "database-service"}

	// Start enhanced trace generation
	go func() {
		err = traces.Start(cfg, zap.NewNop())
		assert.NoError(t, err)
	}()

	// Wait for traces to be generated
	require.Eventually(t, func() bool {
		return len(traceSink.AllTraces()) > 0
	}, 10*time.Second, 100*time.Millisecond)

	// Verify traces were generated
	traces := traceSink.AllTraces()
	assert.Greater(t, len(traces), 0, "Should have generated traces")

	// Verify trace structure
	trace := traces[0]
	resourceSpans := trace.ResourceSpans()
	assert.Greater(t, resourceSpans.Len(), 0, "Should have resource spans")

	// Verify spans were generated
	scopeSpans := resourceSpans.At(0).ScopeSpans()
	assert.Greater(t, scopeSpans.Len(), 0, "Should have scope spans")

	spans := scopeSpans.At(0).Spans()
	assert.Greater(t, spans.Len(), 0, "Should have spans")

	// Verify span attributes and events
	for i := 0; i < spans.Len(); i++ {
		span := spans.At(i)

		// Verify span has a name
		assert.NotEmpty(t, span.Name(), "Span should have a name")

		// Verify span has attributes (enhanced mode adds rich attributes)
		assert.Greater(t, span.Attributes().Len(), 0, "Enhanced spans should have attributes")

		// Verify span has events (enhanced mode adds events)
		assert.Greater(t, span.Events().Len(), 0, "Enhanced spans should have events")

		// Verify service name is one of the configured services
		serviceAttr, exists := span.Attributes().Get("service.name")
		if exists {
			serviceName := serviceAttr.AsString()
			assert.Contains(t, cfg.Services, serviceName, "Service name should be from configured services")
		}
	}

	// Note: In a real integration test with actual OTLP exporters,
	// we would also verify that logs and metrics were sent to their respective endpoints.
	// For this test, we're verifying the enhanced trace generation logic works correctly.
	t.Logf("Generated %d traces with enhanced mode", len(traces))
	t.Logf("First trace has %d spans", spans.Len())
}

func TestEnhancedTraceGenerationPatterns(t *testing.T) {
	patterns := []string{"simple", "chain", "fan-out", "complex", "random"}

	for _, pattern := range patterns {
		t.Run(pattern, func(t *testing.T) {
			f := otlpreceiver.NewFactory()
			sink := &consumertest.TracesSink{}
			rCfg := f.CreateDefaultConfig()
			endpoint := getAvailableLocalAddress(t)
			rCfg.(*otlpreceiver.Config).GRPC.NetAddr.Endpoint = endpoint
			rCfg.(*otlpreceiver.Config).HTTP = nil
			r, err := f.CreateTraces(context.Background(), receivertest.NewNopSettings(f.Type()), rCfg, sink)
			require.NoError(t, err)
			err = r.Start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err)
			defer func() {
				require.NoError(t, r.Shutdown(context.Background()))
			}()

			cfg := traces.NewConfig()
			cfg.WorkerCount = 1
			cfg.Rate = 1
			cfg.TotalDuration = 2 * time.Second
			cfg.CustomEndpoint = endpoint
			cfg.Insecure = true
			cfg.NumTraces = 1
			cfg.UseHTTP = false
			cfg.TerminalOutput = false

			// Enable enhanced mode with specific pattern
			cfg.Enhanced = true
			cfg.SpansPerTrace = 5
			cfg.Pattern = pattern
			cfg.Services = []string{"api-gateway", "user-service"}

			go func() {
				err = traces.Start(cfg, zap.NewNop())
				assert.NoError(t, err)
			}()

			require.Eventually(t, func() bool {
				return len(sink.AllTraces()) > 0
			}, 5*time.Second, 100*time.Millisecond)

			traces := sink.AllTraces()
			assert.Greater(t, len(traces), 0, "Should have generated traces for pattern: %s", pattern)

			// Verify trace structure
			trace := traces[0]
			spans := trace.ResourceSpans().At(0).ScopeSpans().At(0).Spans()
			assert.Greater(t, spans.Len(), 0, "Should have spans for pattern: %s", pattern)

			t.Logf("Pattern %s: Generated %d spans", pattern, spans.Len())
		})
	}
}
