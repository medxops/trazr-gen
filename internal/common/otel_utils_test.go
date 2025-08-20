package common

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
)

func TestAttrToLogKeyValue(t *testing.T) {
	attrs := []attribute.KeyValue{
		attribute.String("str", "val"),
		attribute.Bool("bool", true),
		attribute.Int("int", 42),
		attribute.Float64("float", 3.14),
	}
	result := AttrToLogKeyValue(attrs)

	assert.Len(t, result, len(attrs))

	// Check keys
	assert.Equal(t, "str", result[0].Key)
	assert.Equal(t, "bool", result[1].Key)
	assert.Equal(t, "int", result[2].Key)
	assert.Equal(t, "float", result[3].Key)

	// Check values
	assert.Equal(t, "val", result[0].Value.AsString())
	assert.True(t, result[1].Value.AsBool())
	assert.Equal(t, int64(42), result[2].Value.AsInt64())
	assert.InDelta(t, 3.14, result[3].Value.AsFloat64(), 0.001)
}

func TestSeverityTextFromNumber(t *testing.T) {
	tests := []struct {
		input int32
		want  string
	}{
		{1, "Trace"},
		{5, "Debug"},
		{9, "Info"},
		{13, "Warn"},
		{17, "Error"},
		{21, "Fatal"},
		{24, "Fatal4"},
		{99, ""}, // Unknown severity
	}

	for _, tt := range tests {
		t.Run(strconv.Itoa(int(tt.input)), func(t *testing.T) {
			got := SeverityTextFromNumber(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSeverityTextToNumber(t *testing.T) {
	tests := []struct {
		input string
		want  int32
	}{
		{"trace", LogSeverityTrace},
		{"debug", LogSeverityDebug},
		{"info", LogSeverityInfo},
		{"warn", LogSeverityWarn},
		{"error", LogSeverityError},
		{"fatal", LogSeverityFatal},
		{"unknown", LogSeverityInfo}, // Default to info
		{"", LogSeverityInfo},        // Empty string defaults to info
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := SeverityTextToNumber(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLogSeverityConstants(t *testing.T) {
	// Verify constants are correctly defined
	assert.Equal(t, int32(1), LogSeverityTrace)
	assert.Equal(t, int32(5), LogSeverityDebug)
	assert.Equal(t, int32(9), LogSeverityInfo)
	assert.Equal(t, int32(13), LogSeverityWarn)
	assert.Equal(t, int32(17), LogSeverityError)
	assert.Equal(t, int32(21), LogSeverityFatal)
}
