package common

import (
	"fmt"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel/attribute"
)

// ProcessMockMarkers expands gofakeit/mock templates in attrs (if mockData is true), type-detects values, and appends a trazr.mock.data marker if any mock keys.
// It does NOT perform key injection; it operates on the raw attribute values.
func ProcessMockMarkers(attrs map[string]any) ([]attribute.KeyValue, error) {
	var result []attribute.KeyValue
	var mockKeys []string
	for k, t := range attrs {
		switch v := t.(type) {
		case string:
			if strings.Contains(v, "{{") && strings.Contains(v, "}}") {
				parsed, err := ProcessMockTemplate(v, nil)
				if err != nil {
					return nil, err
				}
				// Try to parse as int
				if intVal, err := strconv.Atoi(parsed); err == nil {
					result = append(result, attribute.Int(k, intVal))
					mockKeys = append(mockKeys, k)
					continue
				}
				// Try to parse as bool
				if boolVal, err := strconv.ParseBool(parsed); err == nil {
					result = append(result, attribute.Bool(k, boolVal))
					mockKeys = append(mockKeys, k)
					continue
				}
				// Try to parse as float64
				if floatVal, err := strconv.ParseFloat(parsed, 64); err == nil {
					result = append(result, attribute.Float64(k, floatVal))
					mockKeys = append(mockKeys, k)
					continue
				}
				// If not a number/bool/float, treat as string
				result = append(result, attribute.String(k, parsed))
				mockKeys = append(mockKeys, k)
				continue
			}
			result = append(result, attribute.String(k, v))
		case bool:
			result = append(result, attribute.Bool(k, v))
		case int:
			result = append(result, attribute.Int(k, v))
		}
	}
	fmt.Println("RESULT:", result)
	if len(mockKeys) > 0 {
		result = append(result, attribute.String("trazr.mock.data", strings.Join(mockKeys, ",")))
	}
	return result, nil
}

// attributesFromMap converts a map[string]any to a slice of attribute.KeyValue.
func attributesFromMap(attrs map[string]any) []attribute.KeyValue {
	var result []attribute.KeyValue
	for k, v := range attrs {
		switch val := v.(type) {
		case string:
			result = append(result, attribute.String(k, val))
		case bool:
			result = append(result, attribute.Bool(k, val))
		case int:
			result = append(result, attribute.Int(k, val))
		}
	}
	return result
}

// GetResourceAttrWithMockMarker returns resource attributes as OpenTelemetry KeyValue pairs, including:
// - service.name
// - all resource attributes
// - trazr.mock.data (keys with mock data templates)
// Note: logBody is not relevant for resource attributes, so pass "".
func (c *Config) GetResourceAttrWithMockMarker() ([]attribute.KeyValue, error) {
	var attrs []attribute.KeyValue
	var err error
	if c.MockData {
		attrs, err = ProcessMockMarkers(c.ResourceAttributes)
		if err != nil {
			return nil, err
		}
	} else {
		attrs = attributesFromMap(c.ResourceAttributes)
	}
	// Ensure service.name is always present as a resource attribute
	found := false
	for _, attr := range attrs {
		if string(attr.Key) == "service.name" {
			found = true
			break
		}
	}
	if !found && c.ServiceName != "" {
		attrs = append(attrs, attribute.String("service.name", c.ServiceName))
	}
	return attrs, nil
}

// GetTelemetryAttrWithMockMarker returns telemetry attributes as OpenTelemetry KeyValue pairs, including:
// - all telemetry attributes
// - trazr.mock.data (keys with mock data templates)
// Note: logBody is not relevant for telemetry attributes, so pass "".
func (c *Config) GetTelemetryAttrWithMockMarker() ([]attribute.KeyValue, error) {
	if c.MockData {
		return ProcessMockMarkers(c.TelemetryAttributes)
	}
	return attributesFromMap(c.TelemetryAttributes), nil
}

// GetHeadersWithMockMarker processes headers for mock templates and adds an 'X-trazr.mock.data' header listing all header keys that used mock data.
func (c *Config) GetHeadersWithMockMarker() (map[string]string, error) {
	result := make(map[string]string, len(c.Headers))
	var mockKeys []string
	for k, v := range c.Headers {
		switch val := v.(type) {
		case string:
			if c.MockData && strings.Contains(val, "{{") && strings.Contains(val, "}}") {
				parsed, err := ProcessMockTemplate(val, nil)
				if err != nil {
					return nil, fmt.Errorf("mock template processing failed for header %q: %w", k, err)
				}
				result[k] = parsed
				mockKeys = append(mockKeys, k)
			} else {
				result[k] = val
			}
		case bool:
			result[k] = strconv.FormatBool(val)
		case int:
			result[k] = strconv.Itoa(val)
		}
	}
	if len(mockKeys) > 0 {
		result["X-trazr.mock.data"] = strings.Join(mockKeys, ",")
	}
	return result, nil
}

// InjectSensitiveDataMarker adds the 'trazr.sensitive.data' key to attrs if any sensitive keys are present.
// Call this once at startup after config and attributes are loaded.
func InjectSensitiveDataMarker(attrs map[string]any, sensitiveKeys []string) {
	var present []string
	for _, k := range sensitiveKeys {
		if _, ok := attrs[k]; ok {
			present = append(present, k)
		}
	}
	if len(present) > 0 {
		attrs["trazr.sensitive.data"] = strings.Join(present, ",")
	}
}

// Minimal tests for documentation
// Example: TestProcessAttributesMap_KeyInjection
// Example: TestProcessMockMarkers_MockExpansion
