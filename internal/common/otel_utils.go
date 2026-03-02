package common

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
)

// LogSeverity constants for OpenTelemetry log severity levels
const (
	LogSeverityTrace int32 = 1
	LogSeverityDebug int32 = 5
	LogSeverityInfo  int32 = 9
	LogSeverityWarn  int32 = 13
	LogSeverityError int32 = 17
	LogSeverityFatal int32 = 21
)

// AttrToLogKeyValue converts []attribute.KeyValue to []log.KeyValue
// This is a shared utility used by both logs and traces packages
func AttrToLogKeyValue(attrs []attribute.KeyValue) []log.KeyValue {
	result := make([]log.KeyValue, len(attrs))
	for i, attr := range attrs {
		var v log.Value
		switch attr.Value.Type() {
		case attribute.BOOL:
			v = log.BoolValue(attr.Value.AsBool())
		case attribute.INT64:
			v = log.Int64Value(attr.Value.AsInt64())
		case attribute.FLOAT64:
			v = log.Float64Value(attr.Value.AsFloat64())
		case attribute.STRING:
			v = log.StringValue(attr.Value.AsString())
		default:
			v = log.StringValue(attr.Value.Emit())
		}
		result[i] = log.KeyValue{
			Key:   string(attr.Key),
			Value: v,
		}
	}
	return result
}

// sensitiveKeys holds attribute keys whose values should be highlighted in
// terminal output. Populated at startup from the config's sensitive-data list.
var sensitiveKeys = map[string]bool{}

// SetSensitiveKeys replaces the sensitive keys set with the provided list.
// Call this once during config initialization.
func SetSensitiveKeys(keys []string) {
	m := make(map[string]bool, len(keys))
	for _, k := range keys {
		m[k] = true
	}
	sensitiveKeys = m
}

// ANSI escape codes for yellow text
const (
	ansiYellow = "\033[33m"
	ansiReset  = "\033[0m"
)

// FormatAttrValue formats an attribute value for terminal display, trimming
// trailing whitespace that YAML multi-line blocks may introduce.
func FormatAttrValue(v interface{}) string {
	return strings.TrimSpace(fmt.Sprintf("%v", v))
}

// IsSensitiveKey returns true if the given attribute key is considered sensitive.
func IsSensitiveKey(key string) bool {
	return sensitiveKeys[key]
}

// Yellow wraps text in ANSI yellow escape codes for terminal display.
func Yellow(s string) string {
	return ansiYellow + s + ansiReset
}

// FormatSensitiveValue returns the value highlighted in yellow if the key
// is in the sensitive keys list, otherwise returns it plain.
func FormatSensitiveValue(key string, value string) string {
	if IsSensitiveKey(key) {
		return Yellow(value)
	}
	return value
}

// FormatSensitiveBody wraps the body text in yellow for terminal display.
func FormatSensitiveBody(body string) string {
	return Yellow(body)
}

// SeverityTextFromNumber returns the canonical OpenTelemetry severity text for a given number
// This is a shared utility used by both logs and traces packages
func SeverityTextFromNumber(severityNumber int32) string {
	severityNumberToText := map[int32]string{
		1: "Trace", 2: "Trace2", 3: "Trace3", 4: "Trace4",
		5: "Debug", 6: "Debug2", 7: "Debug3", 8: "Debug4",
		9: "Info", 10: "Info2", 11: "Info3", 12: "Info4",
		13: "Warn", 14: "Warn2", 15: "Warn3", 16: "Warn4",
		17: "Error", 18: "Error2", 19: "Error3", 20: "Error4",
		21: "Fatal", 22: "Fatal2", 23: "Fatal3", 24: "Fatal4",
	}
	return severityNumberToText[severityNumber]
}

// SeverityTextToNumber converts severity text to OpenTelemetry numeric severity
// This is a shared utility used by both logs and traces packages
func SeverityTextToNumber(severityText string) int32 {
	switch severityText {
	case "trace":
		return LogSeverityTrace
	case "debug":
		return LogSeverityDebug
	case "info":
		return LogSeverityInfo
	case "warn":
		return LogSeverityWarn
	case "error":
		return LogSeverityError
	case "fatal":
		return LogSeverityFatal
	default:
		return LogSeverityInfo // Default to info
	}
}
