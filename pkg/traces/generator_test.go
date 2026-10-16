// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/medxops/trazr-gen/pkg/logs"
	"github.com/medxops/trazr-gen/pkg/metrics"
)

func TestTraceGenerator_GenerateTrace(t *testing.T) {
	config := &Config{}
	config.SetDefaults()
	config.Enhanced = true // Enable enhanced generation for this test

	logger := zap.NewNop()
	generator := NewTraceGenerator(config, logger)

	// Test basic trace generation
	trace, err := generator.GenerateTrace(context.TODO())
	if err != nil {
		t.Fatalf("Failed to generate trace: %v", err)
	}
	require.NotNil(t, trace, "Trace should not be nil")
	if len(trace.Spans) == 0 {
		t.Fatal("Trace should have at least one span")
	}
	if len(trace.Spans) == 0 {
		t.Fatal("Trace should have spans")
	}

	// Verify trace structure - should have at least one span and not exceed max
	if len(trace.Spans) == 0 {
		t.Errorf("Expected at least 1 span, got %d", len(trace.Spans))
	}
	if len(trace.Spans) > config.SpansPerTrace {
		t.Errorf("Expected at most %d spans, got %d", config.SpansPerTrace, len(trace.Spans))
	}

	// Verify root span
	rootSpan := trace.Spans[0]
	if rootSpan.Depth != 0 {
		t.Errorf("Expected root span depth 0, got %d", rootSpan.Depth)
	}

	if rootSpan.ParentID != "" {
		t.Errorf("Expected root span to have no parent, got %s", rootSpan.ParentID)
	}
}

func TestPatterns_GenerateSpans(t *testing.T) {
	config := &Config{}
	config.SetDefaults()
	config.SpansPerTrace = 5
	config.MaxDepth = 3

	services := []string{"service1", "service2", "service3"}

	tests := []struct {
		name    string
		pattern Pattern
	}{
		{"SimplePattern", &SimplePattern{}},
		{"ChainPattern", &ChainPattern{}},
		{"FanOutPattern", &FanOutPattern{}},
		{"ComplexPattern", &ComplexPattern{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spans := tt.pattern.GenerateSpans(config, [16]byte{}, services)

			if len(spans) == 0 {
				t.Fatal("Generated spans slice is empty")
			}

			if len(spans) > config.SpansPerTrace {
				t.Errorf("Generated too many spans: %d > %d", len(spans), config.SpansPerTrace)
			}

			// Verify root span
			rootSpan := spans[0]
			if rootSpan.Depth != 0 {
				t.Errorf("Expected root span depth 0, got %d", rootSpan.Depth)
			}

			// Verify all spans have valid depth
			for i, span := range spans {
				if span.Depth < 0 || span.Depth >= config.MaxDepth {
					t.Errorf("Span %d has invalid depth: %d", i, span.Depth)
				}
			}
		})
	}
}

func TestEventGenerator_GenerateSpanEvents(t *testing.T) {
	logConfig := &logs.Config{}
	logConfig.SetDefaults()

	logger := zap.NewNop()
	generator := NewEventGenerator(logConfig, logger)

	span := &Span{
		StartTime: time.Now(),
		EndTime:   time.Now().Add(time.Second),
	}

	events := generator.GenerateSpanEvents(span, 3)

	if len(events) != 3 {
		t.Errorf("Expected 3 events, got %d", len(events))
	}

	for i, event := range events {
		if event.Name == "" {
			t.Errorf("Event %d has empty name", i)
		}

		if event.Timestamp.Before(span.StartTime) || event.Timestamp.After(span.EndTime) {
			t.Errorf("Event %d timestamp %v is outside span duration %v to %v",
				i, event.Timestamp, span.StartTime, span.EndTime)
		}
	}
}

func TestAttributeGenerator_GenerateSpanAttributes_Generator(t *testing.T) {
	metricConfig := &metrics.Config{}
	metricConfig.SetDefaults()

	logger := zap.NewNop()
	config := &Config{}
	generator := NewAttributeGenerator(config, metricConfig, logger)

	span := &Span{
		Service: "test-service",
		Depth:   1,
	}

	attributes := generator.GenerateSpanAttributes(span, 5)

	if len(attributes) == 0 {
		t.Fatal("Generated attributes map is empty")
	}

	// Verify service name is set
	if serviceName, ok := attributes["service.name"]; !ok {
		t.Error("service.name attribute not found")
	} else if serviceName != "test-service" {
		t.Errorf("Expected service.name to be 'test-service', got %v", serviceName)
	}
}
