// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
)

func TestGetResourceAttrWithMockMarker_SensitiveAndNonSensitive(t *testing.T) {
	cfg := &Config{
		ServiceName:        "test-service",
		ResourceAttributes: KeyValue{"a": "A", "b": "B", "c": "C"},
		SensitiveData:      []string{"a"},
	}
	InjectSensitiveDataMarker(cfg.ResourceAttributes, cfg.SensitiveData)
	attrs, err := cfg.GetResourceAttrWithMockMarker()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, attr := range attrs {
		if string(attr.Key) == "trazrgen.sensitive.data" {
			found = true
			assert.Contains(t, attr.Value.AsString(), "a")
			assert.NotContains(t, attr.Value.AsString(), "c")
		}
	}
	assert.True(t, found, "trazrgen.sensitive.data should be present for sensitive keys")
}

func TestGetResourceAttrWithMockMarker_NoSensitive(t *testing.T) {
	cfg := &Config{
		ServiceName:        "test-service",
		ResourceAttributes: KeyValue{"a": "A", "b": "B"},
		SensitiveData:      []string{"x"},
	}
	InjectSensitiveDataMarker(cfg.ResourceAttributes, cfg.SensitiveData)
	attrs, err := cfg.GetResourceAttrWithMockMarker()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, attr := range attrs {
		if string(attr.Key) == "trazrgen.sensitive.data" {
			t.Errorf("trazrgen.sensitive.data should NOT be present if no sensitive keys are in attributes")
		}
	}
}

func TestGetTelemetryAttrWithMockMarker_SensitiveAndNonSensitive(t *testing.T) {
	cfg := &Config{
		TelemetryAttributes: KeyValue{"d": "D", "e": "E", "f": "F"},
		SensitiveData:       []string{"d"},
	}
	InjectSensitiveDataMarker(cfg.TelemetryAttributes, cfg.SensitiveData)
	attrs, err := cfg.GetTelemetryAttrWithMockMarker()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, attr := range attrs {
		if string(attr.Key) == "trazrgen.sensitive.data" {
			found = true
			assert.Contains(t, attr.Value.AsString(), "d")
			assert.NotContains(t, attr.Value.AsString(), "f")
		}
	}
	assert.True(t, found, "trazrgen.sensitive.data should be present for sensitive keys")
}

func TestGetTelemetryAttrWithMockMarker_NoSensitive(t *testing.T) {
	cfg := &Config{
		TelemetryAttributes: KeyValue{"d": "D", "e": "E"},
		SensitiveData:       []string{"x"},
	}
	InjectSensitiveDataMarker(cfg.TelemetryAttributes, cfg.SensitiveData)
	attrs, err := cfg.GetTelemetryAttrWithMockMarker()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, attr := range attrs {
		if string(attr.Key) == "trazrgen.sensitive.data" {
			t.Errorf("trazrgen.sensitive.data should NOT be present if no sensitive keys are in attributes")
		}
	}
}

func TestGetResourceAttrWithMockMarker_MockAndSensitiveData(t *testing.T) {
	cfg := &Config{
		ServiceName: "svc",
		ResourceAttributes: KeyValue{
			"static": "value",
			"mock1":  "{{FirstName}}",
			"mock2":  "{{Number 1 100}}",
			"bool":   true,
		},
		SensitiveData: []string{"static", "mock2"},
	}
	InjectSensitiveDataMarker(cfg.ResourceAttributes, cfg.SensitiveData)
	attrs, err := cfg.GetResourceAttrWithMockMarker()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	attrMap := map[string]attribute.KeyValue{}
	for _, a := range attrs {
		attrMap[string(a.Key)] = a
	}
	// Sensitive marker
	if a, ok := attrMap["trazrgen.sensitive.data"]; ok {
		assert.Equal(t, "static,mock2", a.Value.AsString())
	} else {
		t.Error("trazrgen.sensitive.data should be present")
	}
	// Mock marker (if you re-add mock marker logic, update this test accordingly)
}

func TestGetTelemetryAttrWithMockMarker_MockAndSensitiveData(t *testing.T) {
	cfg := &Config{
		TelemetryAttributes: KeyValue{
			"static": "value",
			"mock1":  "{{FirstName}}",
			"mock2":  "{{Number 1 100}}",
			"bool":   true,
		},
		SensitiveData: []string{"static", "mock2"},
	}
	InjectSensitiveDataMarker(cfg.TelemetryAttributes, cfg.SensitiveData)
	attrs, err := cfg.GetTelemetryAttrWithMockMarker()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	attrMap := map[string]attribute.KeyValue{}
	for _, a := range attrs {
		attrMap[string(a.Key)] = a
	}
	// Sensitive marker
	if a, ok := attrMap["trazrgen.sensitive.data"]; ok {
		assert.Equal(t, "static,mock2", a.Value.AsString())
	} else {
		t.Error("trazrgen.sensitive.data should be present")
	}
	// Mock marker (if you re-add mock marker logic, update this test accordingly)
}

func TestSensitiveDataOnlyPresentKeys(t *testing.T) {
	cfg := &Config{
		ServiceName: "svc",
		ResourceAttributes: KeyValue{
			"foo": "bar",
			"baz": "qux",
		},
		SensitiveData: []string{"foo", "missing", "baz"},
	}
	InjectSensitiveDataMarker(cfg.ResourceAttributes, cfg.SensitiveData)
	attrs, err := cfg.GetResourceAttrWithMockMarker()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	attrMap := map[string]attribute.KeyValue{}
	for _, a := range attrs {
		attrMap[string(a.Key)] = a
	}
	if a, ok := attrMap["trazrgen.sensitive.data"]; ok {
		// Should only include "foo" and "baz", not "missing"
		assert.ElementsMatch(t, []string{"foo", "baz"}, splitCommaList(a.Value.AsString()))
	} else {
		t.Error("trazrgen.sensitive.data should be present")
	}
}

// splitCommaList splits a comma-separated string into a slice, trimming whitespace.
func splitCommaList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		out = append(out, strings.TrimSpace(part))
	}
	return out
}

func TestProcessMockMarkers_MockExpansion(t *testing.T) {
	attrs := map[string]any{
		"user":   "{{FirstName}}",
		"static": "unchanged",
	}
	result, err := ProcessMockMarkers(attrs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	attrMap := map[string]attribute.KeyValue{}
	for _, a := range result {
		attrMap[string(a.Key)] = a
	}
	// user should be a string (expanded), static should be unchanged
	assert.Equal(t, attribute.STRING, attrMap["user"].Value.Type())
	assert.Equal(t, attribute.STRING, attrMap["static"].Value.Type())
	assert.Contains(t, attrMap, "trazrgen.mock.data")
}

func TestProcessMockMarkers_Error(t *testing.T) {
	attrs := map[string]any{
		"bad": "{{InvalidFunc}}",
	}
	_, err := ProcessMockMarkers(attrs)
	if err == nil {
		t.Error("expected error for invalid template in ProcessMockMarkers")
	}
}

func TestGetHeadersWithMockMarker_Error(t *testing.T) {
	cfg := &Config{
		Headers:  KeyValue{"bad": "{{InvalidFunc}}"},
		MockData: true,
	}
	_, err := cfg.GetHeadersWithMockMarker()
	if err == nil {
		t.Error("expected error for invalid template in GetHeadersWithMockMarker")
	}
}

func TestAttributesFromMap(t *testing.T) {
	attrs := map[string]any{
		"str":         "value",
		"bool":        true,
		"int":         42,
		"unsupported": 3.14, // should be ignored
	}
	result := attributesFromMap(attrs)
	attrMap := map[string]attribute.KeyValue{}
	for _, a := range result {
		attrMap[string(a.Key)] = a
	}
	assert.Equal(t, "value", attrMap["str"].Value.AsString())
	assert.Equal(t, attribute.BOOL, attrMap["bool"].Value.Type())
	assert.Equal(t, attribute.INT64, attrMap["int"].Value.Type())
	assert.NotContains(t, attrMap, "unsupported")
}

func TestInjectSensitiveDataMarker(t *testing.T) {
	attrs := map[string]any{"foo": 1, "bar": 2}
	InjectSensitiveDataMarker(attrs, []string{"foo", "baz"})
	val, ok := attrs["trazrgen.sensitive.data"]
	assert.True(t, ok)
	assert.Equal(t, "foo", val)

	attrs2 := map[string]any{"bar": 2}
	InjectSensitiveDataMarker(attrs2, []string{"foo"})
	_, ok = attrs2["trazrgen.sensitive.data"]
	assert.False(t, ok)
}

func TestGetResourceAttrWithMockMarker_IncludesRandomSeed(t *testing.T) {
	// Test with a fixed seed
	cfg := &Config{
		ServiceName: "test-service",
		MockSeed:    12345,
	}

	attrs, err := cfg.GetResourceAttrWithMockMarker()
	require.NoError(t, err)
	assert.NotNil(t, attrs)

	// Find the seed attribute
	var seedAttr attribute.KeyValue
	found := false
	for _, attr := range attrs {
		if string(attr.Key) == "trazrgen.random.seed" {
			seedAttr = attr
			found = true
			break
		}
	}

	assert.True(t, found, "trazrgen.random.seed should be present in resource attributes")
	assert.Equal(t, int64(12345), seedAttr.Value.AsInt64(), "Seed value should match the configured seed")
}

func TestGetResourceAttrWithMockMarker_TimestampBasedSeed(t *testing.T) {
	// Test with seed 0 (timestamp-based)
	cfg := &Config{
		ServiceName: "test-service",
		MockSeed:    0,
	}

	attrs, err := cfg.GetResourceAttrWithMockMarker()
	require.NoError(t, err)
	assert.NotNil(t, attrs)

	// Find the seed attribute
	var seedAttr attribute.KeyValue
	found := false
	for _, attr := range attrs {
		if string(attr.Key) == "trazrgen.random.seed" {
			seedAttr = attr
			found = true
			break
		}
	}

	assert.True(t, found, "trazrgen.random.seed should be present in resource attributes")
	assert.Equal(t, "timestamp-based", seedAttr.Value.AsString(), "Seed should indicate timestamp-based generation")
}

func TestGetResourceAttrWithMockMarker_SeedWithMockData(t *testing.T) {
	// Test with mock data enabled and a fixed seed
	cfg := &Config{
		ServiceName: "test-service",
		MockSeed:    99999,
		MockData:    true,
		ResourceAttributes: map[string]any{
			"host.ip": "{{IPv4Address}}",
		},
	}

	attrs, err := cfg.GetResourceAttrWithMockMarker()
	require.NoError(t, err)
	assert.NotNil(t, attrs)

	// Find the seed attribute
	var seedAttr attribute.KeyValue
	found := false
	for _, attr := range attrs {
		if string(attr.Key) == "trazrgen.random.seed" {
			seedAttr = attr
			found = true
			break
		}
	}

	assert.True(t, found, "trazrgen.random.seed should be present even with mock data")
	assert.Equal(t, int64(99999), seedAttr.Value.AsInt64(), "Seed value should match the configured seed")
}
