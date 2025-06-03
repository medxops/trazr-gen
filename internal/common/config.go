// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/pflag"
)

var errFormatOTLPAttributes = errors.New("value should be in one of the following formats: key=\"value\", key=true, key=false, or key=<integer>")

const (
	defaultGRPCEndpoint = "localhost:4317"
	defaultHTTPEndpoint = "localhost:4318"
)

type KeyValue map[string]any

var _ pflag.Value = (*KeyValue)(nil)

func (v *KeyValue) String() string {
	return ""
}

func (v *KeyValue) Set(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	// Try JSON object
	if strings.HasPrefix(s, "{") {
		var m map[string]any
		if err := json.Unmarshal([]byte(s), &m); err != nil {
			return fmt.Errorf("invalid JSON for attributes: %w", err)
		}
		for k, val := range m {
			(*v)[k] = val
		}
		return nil
	}
	// Try comma-separated key-value pairs
	pairs := splitCommaSeparated(s)
	for _, pair := range pairs {
		if err := parseKeyValue(pair, v); err != nil {
			return err
		}
	}
	return nil
}

// splitCommaSeparated splits on commas, but ignores commas inside quotes
func splitCommaSeparated(s string) []string {
	if s == "" {
		return []string{}
	}
	var result []string
	var current strings.Builder
	inQuotes := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '"' {
			inQuotes = !inQuotes
		}
		if c == ',' && !inQuotes {
			result = append(result, strings.TrimSpace(current.String()))
			current.Reset()
		} else {
			current.WriteByte(c)
		}
	}
	if current.Len() > 0 {
		result = append(result, strings.TrimSpace(current.String()))
	}
	return result
}

// parseKeyValue parses a single key-value pair and adds it to the map
func parseKeyValue(s string, v *KeyValue) error {
	kv := strings.SplitN(s, "=", 2)
	if len(kv) != 2 {
		return errFormatOTLPAttributes
	}
	key := strings.TrimSpace(kv[0])
	val := strings.TrimSpace(kv[1])
	// Try bool
	if val == "true" {
		(*v)[key] = true
		return nil
	}
	if val == "false" {
		(*v)[key] = false
		return nil
	}
	// Try int
	if intVal, err := strconv.Atoi(val); err == nil {
		(*v)[key] = intVal
		return nil
	}
	// Try float
	if floatVal, err := strconv.ParseFloat(val, 64); err == nil {
		(*v)[key] = floatVal
		return nil
	}
	// If quoted string, remove quotes
	if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
		(*v)[key] = val[1 : len(val)-1]
		return nil
	}
	// Accept unquoted string for convenience
	(*v)[key] = val
	return nil
}

func (v *KeyValue) Type() string {
	return "map[string]any"
}

// Config holds all global/common configuration for CLI and config file.
// All fields must have a `mapstructure` tag matching the CLI/config key (dashed, lower-case).
type Config struct {
	WorkerCount       int           `mapstructure:"workers"`
	Rate              float64       `mapstructure:"rate"`
	TotalDuration     time.Duration `mapstructure:"duration"`
	ReportingInterval time.Duration `mapstructure:"interval"`

	// OTLP config
	CustomEndpoint      string   `mapstructure:"otlp-endpoint"`
	Insecure            bool     `mapstructure:"otlp-insecure"`
	InsecureSkipVerify  bool     `mapstructure:"otlp-insecure-skip-verify"`
	UseHTTP             bool     `mapstructure:"otlp-http"`
	HTTPPath            string   `mapstructure:"otlp-http-url-path"`
	Headers             KeyValue `mapstructure:"otlp-header"`
	ResourceAttributes  KeyValue `mapstructure:"otlp-attributes"`
	ServiceName         string   `mapstructure:"service"`
	TelemetryAttributes KeyValue `mapstructure:"telemetry-attributes"`

	// Sensitive data keys (attributes or headers)
	SensitiveData []string `mapstructure:"sensitive-data"`

	// OTLP TLS configuration
	CaFile string `mapstructure:"ca-cert"`

	// OTLP mTLS configuration
	ClientAuth ClientAuth `mapstructure:"client-auth"`

	LogLevel string `mapstructure:"log-level"`

	MockData       bool  `mapstructure:"mock-data"` // Enable mock data generation for templated fields
	MockSeed       int64 `mapstructure:"mock-seed"` // Seed for mock data generation (used only at startup)
	TerminalOutput bool  `mapstructure:"terminal-output"`
}

type ClientAuth struct {
	Enabled        bool   `mapstructure:"mtls"`
	ClientCertFile string `mapstructure:"client-cert"`
	ClientKeyFile  string `mapstructure:"client-key"`
}

// Endpoint returns the appropriate endpoint URL based on the selected communication mode (gRPC or HTTP)
// or custom endpoint provided in the configuration.
func (c *Config) Endpoint() string {
	if c.CustomEndpoint != "" {
		return c.CustomEndpoint
	}
	if c.UseHTTP {
		return defaultHTTPEndpoint
	}
	return defaultGRPCEndpoint
}

// CommonFlags registers common config flags.
func (c *Config) CommonFlags(fs *pflag.FlagSet) {
	fs.IntVar(&c.WorkerCount, "workers", c.WorkerCount, "Number of workers (goroutines) to run")
	fs.Float64Var(&c.Rate, "rate", c.Rate, "# of metrics/spans/logs per second each worker should generate. 0 means no throttling.")
	fs.DurationVar(&c.TotalDuration, "duration", c.TotalDuration, "For how long to run the test")
	fs.DurationVar(&c.ReportingInterval, "interval", c.ReportingInterval, "Reporting interval")

	fs.StringVar(&c.CustomEndpoint, "otlp-endpoint", c.CustomEndpoint, "Destination endpoint for exporting logs, metrics and traces")
	fs.BoolVar(&c.Insecure, "otlp-insecure", c.Insecure, "Whether to enable client transport security for the exporter's grpc or http connection")
	fs.BoolVar(&c.InsecureSkipVerify, "otlp-insecure-skip-verify", c.InsecureSkipVerify, "Whether a client verifies the server's certificate chain and host name")
	fs.BoolVar(&c.UseHTTP, "otlp-http", c.UseHTTP, "Whether to use HTTP exporter rather than a gRPC one")

	fs.StringVar(&c.ServiceName, "service", c.ServiceName, "Service name to use")

	// custom headers
	fs.Var(&c.Headers, "otlp-header", "Custom OTLP header (key=\"value\"). Repeat for multiple headers.")

	// custom resource attributes
	fs.Var(&c.ResourceAttributes, "otlp-attributes", "Custom telemetry attribute (key=\"value\"). Repeat for multiple attributes.")

	fs.Var(&c.TelemetryAttributes, "telemetry-attributes", "Custom telemetry attribute (key=\"value\"). Repeat for multiple attributes.")

	// TLS CA configuration
	fs.StringVar(&c.CaFile, "ca-cert", c.CaFile, "Trusted Certificate Authority to verify server certificate")

	// mTLS configuration
	fs.BoolVar(&c.ClientAuth.Enabled, "mtls", c.ClientAuth.Enabled, "Whether to require client authentication for mTLS")
	fs.StringVar(&c.ClientAuth.ClientCertFile, "client-cert", c.ClientAuth.ClientCertFile, "Client certificate file")
	fs.StringVar(&c.ClientAuth.ClientKeyFile, "client-key", c.ClientAuth.ClientKeyFile, "Client private key file")

	fs.StringSliceVar(&c.SensitiveData, "sensitive-data", c.SensitiveData, "Sensitive attribute or header keys (comma-separated or repeatable)")

	fs.BoolVar(&c.MockData, "mock-data", c.MockData, "Enable mock data generation for templated fields")
	fs.Int64Var(&c.MockSeed, "mock-seed", c.MockSeed, "Seed for mock data generation (used only at startup)")
	fs.BoolVar(&c.TerminalOutput, "terminal-output", c.TerminalOutput, "Enable terminal output for logs (default: true)")
}

// SetDefaults is here to mirror the defaults for flags above,
// This allows for us to have a single place to change the defaults
// while exposing the API for use.
func (c *Config) SetDefaults() {
	c.WorkerCount = 1
	c.Rate = 1
	c.TotalDuration = 0
	c.ReportingInterval = 1 * time.Second
	c.CustomEndpoint = "localhost:4318"
	c.Insecure = true
	c.InsecureSkipVerify = true
	c.UseHTTP = true
	c.HTTPPath = ""
	c.Headers = make(KeyValue)
	c.ResourceAttributes = make(KeyValue)
	c.ServiceName = "trazr-gen"
	c.TelemetryAttributes = make(KeyValue)
	c.CaFile = ""
	c.ClientAuth.Enabled = false
	c.ClientAuth.ClientCertFile = ""
	c.ClientAuth.ClientKeyFile = ""
	c.SensitiveData = []string{}
	c.LogLevel = "info"
	c.MockData = true
	c.MockSeed = 0
	c.TerminalOutput = true
}

func (c *Config) GetHeaders() map[string]string {
	m := make(map[string]string, len(c.Headers))
	for k, t := range c.Headers {
		switch v := t.(type) {
		case bool:
			m[k] = strconv.FormatBool(v)
		case string:
			m[k] = v
		}
	}
	return m
}

// FlattenMap recursively flattens a nested map into a flat map with dot-separated keys. Returns error on unsupported types.
func FlattenMap(prefix string, in map[string]any, out map[string]any) error {
	for k, v := range in {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch child := v.(type) {
		case map[string]any:
			if err := FlattenMap(key, child, out); err != nil {
				return err
			}
		case map[any]any: // handle YAML decoding to map[any]any
			childMap := make(map[string]any)
			for ck, cv := range child {
				cs, ok := ck.(string)
				if !ok {
					return fmt.Errorf("unsupported non-string map key: %v (type %T)", ck, ck)
				}
				childMap[cs] = cv
			}
			if err := FlattenMap(key, childMap, out); err != nil {
				return err
			}
		case nil, string, bool, int, float64:
			out[key] = v
		default:
			return fmt.Errorf("unsupported attribute value type for key %q: %T", key, v)
		}
	}
	return nil
}

// InitAttributes performs one-time initialization of attribute maps, including adding the 'trazr.sensitive.data' key to both ResourceAttributes and TelemetryAttributes if any sensitive keys are present.
// Call this once after config and attributes are loaded.
func (c *Config) InitAttributes() error {
	flatRes := make(map[string]any)
	if err := FlattenMap("", c.ResourceAttributes, flatRes); err != nil {
		return fmt.Errorf("failed to flatten resource attributes: %w", err)
	}
	c.ResourceAttributes = flatRes

	flatTel := make(map[string]any)
	if err := FlattenMap("", c.TelemetryAttributes, flatTel); err != nil {
		return fmt.Errorf("failed to flatten telemetry attributes: %w", err)
	}
	c.TelemetryAttributes = flatTel

	flatHeaders := make(map[string]any)
	if err := FlattenMap("", c.Headers, flatHeaders); err != nil {
		return fmt.Errorf("failed to flatten headers: %w", err)
	}
	c.Headers = flatHeaders

	InjectSensitiveDataMarker(c.ResourceAttributes, c.SensitiveData)
	InjectSensitiveDataMarker(c.TelemetryAttributes, c.SensitiveData)
	return nil
}

// ShowNonDefaultConfig prints all config fields that differ from their default values.
// It works for any config struct (logs, metrics, traces, or common) that has a SetDefaults() method.
// Fields that are not set to their default values are printed as: <field path>: <value> (default: <default value>)
// Output is sent to the provided UserOutput.
// Returns true if at least one value differs from default, false otherwise.
func ShowNonDefaultConfig(cfg any) {
	cfgVal := reflect.ValueOf(cfg)
	if cfgVal.Kind() == reflect.Ptr {
		cfgVal = cfgVal.Elem()
	}
	cfgType := cfgVal.Type()

	// Create a new zero value and set defaults
	defaultPtr := reflect.New(cfgType)
	setDefaults := defaultPtr.MethodByName("SetDefaults")
	if setDefaults.IsValid() {
		setDefaults.Call(nil)
	}
	defaultVal := defaultPtr.Elem()

	var found bool
	var walk func(path string, v, def reflect.Value)
	walk = func(path string, v, def reflect.Value) {
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			f := t.Field(i)
			fname := f.Name
			if f.Anonymous && v.Field(i).Kind() == reflect.Struct {
				// Embedded struct: recurse with path
				walk(path, v.Field(i), def.Field(i))
				continue
			}
			fieldPath := fname
			if path != "" {
				fieldPath = path + "." + fname
			}
			val := v.Field(i)
			defVal := def.Field(i)
			if !valuesEqual(val, defVal) {
				if !found {
					fmt.Println("----------------- Overridden Config Values -----------------")
					found = true
				}
				fmt.Printf("%s: %v (default: %v)\n", fieldPath, printable(val), printable(defVal))
			}
		}
	}
	walk("", cfgVal, defaultVal)
	if found {
		fmt.Println("------------------------------------------------------------")
	}
}

// valuesEqual compares two reflect.Values for equality, handling slices, maps, and basic types.
func valuesEqual(a, b reflect.Value) bool {
	if !a.IsValid() || !b.IsValid() {
		return a.IsValid() == b.IsValid()
	}
	if a.Type() != b.Type() {
		return false
	}
	switch a.Kind() {
	case reflect.Slice:
		if a.IsNil() && b.IsNil() {
			return true
		}
		if a.Len() != b.Len() {
			return false
		}
		for i := 0; i < a.Len(); i++ {
			if !valuesEqual(a.Index(i), b.Index(i)) {
				return false
			}
		}
		return true
	case reflect.Map:
		if a.IsNil() && b.IsNil() {
			return true
		}
		if a.Len() != b.Len() {
			return false
		}
		for _, k := range a.MapKeys() {
			av := a.MapIndex(k)
			bv := b.MapIndex(k)
			if !bv.IsValid() || !valuesEqual(av, bv) {
				return false
			}
		}
		return true
	case reflect.Struct:
		for i := 0; i < a.NumField(); i++ {
			if !valuesEqual(a.Field(i), b.Field(i)) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(a.Interface(), b.Interface())
	}
}

// printable returns a value suitable for printing (dereferencing pointers, formatting slices/maps).
func printable(v reflect.Value) any {
	if !v.IsValid() {
		return nil
	}
	switch v.Kind() {
	case reflect.Ptr:
		if v.IsNil() {
			return nil
		}
		return printable(v.Elem())
	case reflect.Slice:
		var out []any
		for i := 0; i < v.Len(); i++ {
			out = append(out, printable(v.Index(i)))
		}
		return out
	case reflect.Map:
		m := make(map[any]any, v.Len())
		for _, k := range v.MapKeys() {
			m[printable(k)] = printable(v.MapIndex(k))
		}
		return m
	case reflect.Struct:
		return fmt.Sprintf("%v", v.Interface())
	default:
		return v.Interface()
	}
}
