// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"encoding/hex"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
	"go.uber.org/zap"
)

func Test_exemplarsFromConfig(t *testing.T) {
	traceID, err := hex.DecodeString("ae87dadd90e9935a4bc9660628efd569")
	require.NoError(t, err)

	spanID, err := hex.DecodeString("5828fa4960140870")
	require.NoError(t, err)

	tests := []struct {
		name         string
		c            *Config
		validateFunc func(t *testing.T, got []metricdata.Exemplar[int64])
	}{
		{
			name: "no exemplars",
			c:    &Config{},
			validateFunc: func(t *testing.T, got []metricdata.Exemplar[int64]) {
				assert.Nil(t, got)
			},
		},
		{
			name: "both-traceID-and-spanID",
			c: &Config{
				TraceID: "ae87dadd90e9935a4bc9660628efd569",
				SpanID:  "5828fa4960140870",
			},
			validateFunc: func(t *testing.T, got []metricdata.Exemplar[int64]) {
				require.Len(t, got, 1)
				metricdatatest.AssertEqual[metricdata.Exemplar[int64]](t, got[0], metricdata.Exemplar[int64]{
					TraceID: traceID,
					SpanID:  spanID,
				}, metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreValue())
			},
		},
		{
			name: "only-traceID",
			c: &Config{
				TraceID: "ae87dadd90e9935a4bc9660628efd569",
			},
			validateFunc: func(t *testing.T, got []metricdata.Exemplar[int64]) {
				require.Len(t, got, 1)
				metricdatatest.AssertEqual[metricdata.Exemplar[int64]](t, got[0], metricdata.Exemplar[int64]{
					TraceID: traceID,
					SpanID:  nil,
				}, metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreValue())
			},
		},
		{
			name: "only-spanID",
			c: &Config{
				SpanID: "5828fa4960140870",
			},
			validateFunc: func(t *testing.T, got []metricdata.Exemplar[int64]) {
				require.Len(t, got, 1)
				metricdatatest.AssertEqual[metricdata.Exemplar[int64]](t, got[0], metricdata.Exemplar[int64]{
					TraceID: nil,
					SpanID:  spanID,
				}, metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreValue())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.validateFunc(t, exemplarsFromConfig(tt.c))
		})
	}
}

func TestCreateExporter_TLS_Error(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	cfg.Insecure = false
	cfg.CaFile = "bad.pem"
	cfg.ClientAuth.Enabled = false
	cfg.CustomEndpoint = "localhost:4317"
	logger := zap.NewNop()
	_, err := createExporter(cfg, logger)
	require.Error(t, err)
}

func TestAggregationTemporality_Set_String_Type(t *testing.T) {
	var temporality AggregationTemporality
	tempPtr := &temporality

	// Test Set with valid values
	require.NoError(t, tempPtr.Set("delta"))
	assert.Equal(t, metricdata.DeltaTemporality, temporality.AsTemporality())
	assert.Equal(t, string(metricdata.DeltaTemporality), tempPtr.String())
	assert.Equal(t, "temporality", tempPtr.Type())

	require.NoError(t, tempPtr.Set("cumulative"))
	assert.Equal(t, metricdata.CumulativeTemporality, temporality.AsTemporality())
	assert.Equal(t, string(metricdata.CumulativeTemporality), tempPtr.String())

	// Test Set with invalid value
	err := tempPtr.Set("invalid")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "temporality must be one of")
}

func TestMetricType_Set_String_Type(t *testing.T) {
	var mt MetricType

	// Test Set with valid values
	for _, v := range []string{"Gauge", "Sum", "Histogram"} {
		require.NoError(t, mt.Set(v))
		assert.Equal(t, v, mt.String())
		assert.Equal(t, "MetricType", mt.Type())
	}

	// Test Set with invalid value
	err := mt.Set("invalid")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be one of")
}

func TestConfigHelpers(t *testing.T) {
	// Test NewConfig sets defaults
	cfg := NewConfig()
	assert.NotNil(t, cfg)
	assert.Equal(t, 1, cfg.NumMetrics)
	assert.Equal(t, "gen", cfg.MetricName)
	assert.Equal(t, "/v1/metrics", cfg.HTTPPath)

	// Test Flags registers flags (smoke test)
	fs := newFlagSet()
	cfg.Flags(fs)

	// Test GetHeaders returns empty map by default
	h := cfg.GetHeaders()
	assert.NotNil(t, h)
	assert.Empty(t, h)

	// Test IsMockDataEnabled
	cfg.MockData = true
	assert.True(t, cfg.IsMockDataEnabled())
	cfg.MockData = false
	assert.False(t, cfg.IsMockDataEnabled())

	// Test InitAttributes (should not error on default config)
	err := cfg.InitAttributes()
	assert.NoError(t, err)
}

// newFlagSet is a helper to create a pflag.FlagSet for testing
func newFlagSet() *pflag.FlagSet {
	return &pflag.FlagSet{}
}

type fakeCmd struct {
	template string
}

func (f *fakeCmd) SetHelpTemplate(s string) { f.template = s }

func TestSetHelpTemplateForCmd(t *testing.T) {
	cmd := &fakeCmd{}
	SetHelpTemplateForCmd(cmd)
	if cmd.template == "" {
		t.Error("SetHelpTemplateForCmd did not set the help template")
	}
}
