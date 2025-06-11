// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.uber.org/zap"

	"github.com/medxops/trazr-gen/internal/common"
)

const (
	telemetryAttrKeyOne   = "k1"
	telemetryAttrKeyTwo   = "k2"
	telemetryAttrValueOne = "v1"
	telemetryAttrValueTwo = "v2"
)

type mockExporter struct {
	logs []sdklog.Record
}

func (m *mockExporter) Export(_ context.Context, records []sdklog.Record) error {
	m.logs = append(m.logs, records...)
	return nil
}

func (m *mockExporter) Shutdown(_ context.Context) error {
	return nil
}

func (m *mockExporter) ForceFlush(_ context.Context) error {
	return nil
}

func TestFixedNumberOfLogs(t *testing.T) {
	cfg := &Config{
		Config: common.Config{
			WorkerCount: 1,
		},
		NumLogs:        5,
		SeverityText:   "Info",
		SeverityNumber: "9",
	}

	m := &mockExporter{}

	// test
	logger, _ := zap.NewDevelopment()
	require.NoError(t, run(cfg, m, logger))

	time.Sleep(1 * time.Second)

	// verify
	require.Len(t, m.logs, 5)
}

func TestRateOfLogs(t *testing.T) {
	cfg := &Config{
		Config: common.Config{
			Rate:          10,
			TotalDuration: time.Second / 2,
			WorkerCount:   1,
		},
		SeverityText:   "Info",
		SeverityNumber: "9",
	}
	m := &mockExporter{}

	// test
	require.NoError(t, run(cfg, m, zap.NewNop()))

	// verify
	// the minimum acceptable number of logs for the rate of 10/sec for half a second
	assert.GreaterOrEqual(t, len(m.logs), 5, "there should have been 5 or more logs, had %d", len(m.logs))
	// the maximum acceptable number of logs for the rate of 10/sec for half a second
	assert.LessOrEqual(t, len(m.logs), 20, "there should have been less than 20 logs, had %d", len(m.logs))
}

func TestUnthrottled(t *testing.T) {
	cfg := &Config{
		Config: common.Config{
			TotalDuration: 1 * time.Second,
			WorkerCount:   1,
		},
		SeverityText:   "Info",
		SeverityNumber: "9",
	}
	m := &mockExporter{}

	// test
	logger, _ := zap.NewDevelopment()
	require.NoError(t, run(cfg, m, logger))

	assert.Greater(t, len(m.logs), 100, "there should have been more than 100 logs, had %d", len(m.logs))
}

func TestCustomBody(t *testing.T) {
	cfg := &Config{
		Body:    "custom body",
		NumLogs: 1,
		Config: common.Config{
			WorkerCount: 1,
		},
		SeverityText:   "Info",
		SeverityNumber: "9",
	}
	m := &mockExporter{}

	// test
	logger, _ := zap.NewDevelopment()
	require.NoError(t, run(cfg, m, logger))

	assert.Equal(t, "custom body", m.logs[0].Body().AsString())
}

func TestLogsWithNoTelemetryAttributes(t *testing.T) {
	cfg := configWithNoAttributes(2, "custom body")

	m := &mockExporter{}

	// test
	logger, _ := zap.NewDevelopment()
	require.NoError(t, run(cfg, m, logger))

	time.Sleep(1 * time.Second)

	// verify
	require.Len(t, m.logs, 2)
	for _, log := range m.logs {
		// Should have 0 telemetry attributes
		assert.Equal(t, 0, log.AttributesLen(), "shouldn't have more than 0 telemetry attributes")
		// Check service.name in resource attributes
		res := log.Resource()
		attrs := (&res).Attributes()
		found := false
		for _, attr := range attrs {
			if string(attr.Key) == "service.name" {
				found = true
				assert.Equal(t, cfg.ServiceName, attr.Value.AsString())
				break
			}
		}
		assert.True(t, found, "service.name should be present in resource attributes")
	}
}

func TestLogsWithOneTelemetryAttributes(t *testing.T) {
	qty := 1
	cfg := configWithOneAttribute(qty, "custom body")

	m := &mockExporter{}

	// test
	logger, _ := zap.NewDevelopment()
	require.NoError(t, run(cfg, m, logger))

	time.Sleep(1 * time.Second)

	// verify
	require.Len(t, m.logs, qty)
	for _, l := range m.logs {
		// Should have 1 telemetry attribute
		assert.Equal(t, 1, l.AttributesLen(), "should have exactly 1 telemetry attribute")
		l.WalkAttributes(func(attr log.KeyValue) bool {
			if attr.Key == telemetryAttrKeyOne {
				assert.Equal(t, telemetryAttrValueOne, attr.Value.AsString())
			}
			return true
		})
		// Check service.name in resource attributes
		res := l.Resource()
		attrs := (&res).Attributes()
		found := false
		for _, attr := range attrs {
			if string(attr.Key) == "service.name" {
				found = true
				assert.Equal(t, cfg.ServiceName, attr.Value.AsString())
				break
			}
		}
		assert.True(t, found, "service.name should be present in resource attributes")
	}
}

func TestLogsWithMultipleTelemetryAttributes(t *testing.T) {
	qty := 1
	cfg := configWithMultipleAttributes(qty, "custom body")

	m := &mockExporter{}

	// test
	logger, _ := zap.NewDevelopment()
	require.NoError(t, run(cfg, m, logger))

	time.Sleep(1 * time.Second)

	// verify
	require.Len(t, m.logs, qty)
	for _, l := range m.logs {
		// Should have 2 telemetry attributes
		assert.Equal(t, 2, l.AttributesLen(), "should have exactly 2 telemetry attributes")
		// Check service.name in resource attributes
		res := l.Resource()
		attrs := (&res).Attributes()
		found := false
		for _, attr := range attrs {
			if string(attr.Key) == "service.name" {
				found = true
				assert.Equal(t, cfg.ServiceName, attr.Value.AsString())
				break
			}
		}
		assert.True(t, found, "service.name should be present in resource attributes")
	}
}

func TestLogsWithTraceIDAndSpanID(t *testing.T) {
	qty := 1
	cfg := configWithOneAttribute(qty, "custom body")
	cfg.TraceID = "ae87dadd90e9935a4bc9660628efd569"
	cfg.SpanID = "5828fa4960140870"

	m := &mockExporter{}

	// test
	logger, _ := zap.NewDevelopment()
	require.NoError(t, run(cfg, m, logger))

	// verify
	require.Len(t, m.logs, qty)
	for _, l := range m.logs {
		assert.Equal(t, "ae87dadd90e9935a4bc9660628efd569", l.TraceID().String())
		assert.Equal(t, "5828fa4960140870", l.SpanID().String())
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name           string
		cfg            *Config
		wantErrMessage string
	}{
		{
			name: "No duration or NumLogs",
			cfg: &Config{
				Config: common.Config{
					WorkerCount: 1,
				},
				TraceID: "123",
			},
			wantErrMessage: "either `logs` or `duration` must be greater than 0",
		},
		{
			name: "TraceID invalid",
			cfg: &Config{
				Config: common.Config{
					WorkerCount: 1,
				},
				NumLogs: 5,
				TraceID: "123",
			},
			wantErrMessage: "TraceID must be a 32 character hex string, like: 'ae87dadd90e9935a4bc9660628efd569'",
		},
		{
			name: "SpanID invalid",
			cfg: &Config{
				Config: common.Config{
					WorkerCount: 1,
				},
				NumLogs: 5,
				TraceID: "ae87dadd90e9935a4bc9660628efd569",
				SpanID:  "123",
			},
			wantErrMessage: "SpanID must be a 16 character hex string, like: '5828fa4960140870'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &mockExporter{}
			logger, _ := zap.NewDevelopment()
			require.EqualError(t, run(tt.cfg, m, logger), tt.wantErrMessage)
		})
	}
}

func configWithNoAttributes(qty int, body string) *Config {
	return &Config{
		Body:    body,
		NumLogs: qty,
		Config: common.Config{
			WorkerCount:         1,
			TelemetryAttributes: nil,
			ServiceName:         "test-service",
		},
		SeverityText:   "Info",
		SeverityNumber: "9",
	}
}

func configWithOneAttribute(qty int, body string) *Config {
	return &Config{
		Body:    body,
		NumLogs: qty,
		Config: common.Config{
			WorkerCount:         1,
			TelemetryAttributes: common.KeyValue{telemetryAttrKeyOne: telemetryAttrValueOne},
			ServiceName:         "test-service",
		},
		SeverityText:   "Info",
		SeverityNumber: "9",
	}
}

func configWithMultipleAttributes(qty int, body string) *Config {
	kvs := common.KeyValue{telemetryAttrKeyOne: telemetryAttrValueOne, telemetryAttrKeyTwo: telemetryAttrValueTwo}
	return &Config{
		Body:    body,
		NumLogs: qty,
		Config: common.Config{
			WorkerCount:         1,
			TelemetryAttributes: kvs,
			ServiceName:         "test-service",
		},
		SeverityText:   "Info",
		SeverityNumber: "9",
	}
}

func TestSeverityNumberParsing(t *testing.T) {
	common.InitMockData(42) // deterministic output
	tests := []struct {
		name           string
		mockData       bool
		severityNumber string
		wantInt        int
		wantErr        bool
	}{
		{
			name:           "Static number",
			mockData:       false,
			severityNumber: "9",
			wantInt:        9,
			wantErr:        false,
		},
		{
			name:           "Templated number (mockData on)",
			mockData:       true,
			severityNumber: "{{Number 1 24}}",
			wantInt:        0, // not used
			wantErr:        false,
		},
		{
			name:           "Invalid number",
			mockData:       false,
			severityNumber: "notanumber",
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				SeverityNumber: tt.severityNumber,
				Config: common.Config{
					MockData: tt.mockData,
				},
			}
			severityNumberStr := cfg.SeverityNumber
			if cfg.MockData && len(severityNumberStr) > 0 && (severityNumberStr[0] == '{' || severityNumberStr[0] == '$') {
				parsed, err := common.ProcessMockTemplate(severityNumberStr, nil)
				if err != nil {
					if !tt.wantErr {
						t.Fatalf("unexpected error: %v", err)
					}
					return
				}
				severityNumberStr = parsed
			}
			intVal, err := strconv.Atoi(severityNumberStr)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.name == "Templated number (mockData on)" {
				assert.GreaterOrEqual(t, intVal, 1)
				assert.LessOrEqual(t, intVal, 24)
			} else {
				assert.Equal(t, tt.wantInt, intVal)
			}
		})
	}
}

func TestAttrToLogKeyValue(t *testing.T) {
	attrs := []attribute.KeyValue{
		attribute.String("str", "val"),
		attribute.Bool("bool", true),
		attribute.Int("int", 42),
		attribute.Float64("float", 3.14),
	}
	result := attrToLogKeyValue(attrs)
	if len(result) != len(attrs) {
		t.Fatalf("expected %d, got %d", len(attrs), len(result))
	}
	for i, attr := range attrs {
		if result[i].Key != string(attr.Key) {
			t.Errorf("expected key %q, got %q", attr.Key, result[i].Key)
		}
	}
	// Check value types
	if result[0].Value.AsString() != "val" {
		t.Errorf("expected string value 'val', got %v", result[0].Value.AsString())
	}
	if !result[1].Value.AsBool() {
		t.Errorf("expected bool value true, got %v", result[1].Value.AsBool())
	}
	if result[2].Value.AsInt64() != 42 {
		t.Errorf("expected int value 42, got %v", result[2].Value.AsInt64())
	}
	if result[3].Value.AsFloat64() != 3.14 {
		t.Errorf("expected float value 3.14, got %v", result[3].Value.AsFloat64())
	}
}

func TestWorker_ReportProgressf(t *testing.T) {
	var called bool
	var got string
	w := worker{
		progressCb: func(msg string) {
			called = true
			got = msg
		},
	}
	w.reportProgressf("hello %s", "world")
	if !called {
		t.Fatal("progressCb was not called")
	}
	if got != "hello world" {
		t.Fatalf("expected 'hello world', got %q", got)
	}
}
