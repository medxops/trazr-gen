// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKeyValueSet(t *testing.T) {
	tests := []struct {
		flag     string
		expected KeyValue
		err      error
	}{
		{
			flag:     "key=\"value\"",
			expected: KeyValue(map[string]any{"key": "value"}),
		},
		{
			flag:     "key=\"\"",
			expected: KeyValue(map[string]any{"key": ""}),
		},
		{
			flag:     "key=\"",
			expected: KeyValue(map[string]any{"key": "\""}),
		},
		{
			flag:     "key=value",
			expected: KeyValue(map[string]any{"key": "value"}),
		},
		{
			flag: "key",
			err:  errFormatOTLPAttributes,
		},
		{
			flag:     "key=true",
			expected: KeyValue(map[string]any{"key": true}),
		},
		{
			flag:     "key=false",
			expected: KeyValue(map[string]any{"key": false}),
		},
		{
			flag:     "key=123",
			expected: KeyValue(map[string]any{"key": 123}),
		},
		{
			flag:     "key=-456",
			expected: KeyValue(map[string]any{"key": -456}),
		},
		{
			flag:     "key=0",
			expected: KeyValue(map[string]any{"key": 0}),
		},
		{
			flag:     "key=12.34",
			expected: KeyValue(map[string]any{"key": 12.34}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.flag, func(t *testing.T) {
			kv := KeyValue(make(map[string]any))
			err := kv.Set(tt.flag)
			if err != nil || tt.err != nil {
				assert.Equal(t, err, tt.err)
			} else {
				assert.Equal(t, tt.expected, kv)
			}
		})
	}
}

func TestEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		http     bool
		expected string
	}{
		{
			"default-no-http",
			"",
			false,
			defaultGRPCEndpoint,
		},
		{
			"default-with-http",
			"",
			true,
			defaultHTTPEndpoint,
		},
		{
			"custom-endpoint-no-http",
			"collector:4317",
			false,
			"collector:4317",
		},
		{
			"custom-endpoint-with-http",
			"collector:4317",
			true,
			"collector:4317",
		},
		{
			"wrong-custom-endpoint-with-http",
			defaultGRPCEndpoint,
			true,
			defaultGRPCEndpoint,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				CustomEndpoint: tc.endpoint,
				UseHTTP:        tc.http,
			}

			assert.Equal(t, tc.expected, cfg.Endpoint())
		})
	}
}

func TestSensitiveDataConfigAndFlag(t *testing.T) {
	t.Run("config default is empty", func(t *testing.T) {
		cfg := &Config{}
		cfg.SetDefaults()
		assert.Empty(t, cfg.SensitiveData)
	})

	t.Run("set via config struct", func(t *testing.T) {
		cfg := &Config{SensitiveData: []string{"foo", "bar"}}
		assert.Equal(t, []string{"foo", "bar"}, cfg.SensitiveData)
	})

	t.Run("set via CLI flag", func(t *testing.T) {
		cfg := &Config{}
		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		cfg.CommonFlags(fs)
		_ = fs.Parse([]string{"--sensitive-data=foo", "--sensitive-data=bar"})
		assert.Equal(t, []string{"foo", "bar"}, cfg.SensitiveData)
	})

	t.Run("set via CLI flag comma-separated", func(t *testing.T) {
		cfg := &Config{}
		fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
		cfg.CommonFlags(fs)
		_ = fs.Parse([]string{"--sensitive-data=foo,bar"})
		assert.Equal(t, []string{"foo", "bar"}, cfg.SensitiveData)
	})
}

func TestFlattenMap(t *testing.T) {
	in := map[string]any{
		"a": 1,
		"b": map[string]any{"c": 2, "d": map[string]any{"e": 3}},
	}
	out := make(map[string]any)
	if err := FlattenMap("", in, out); err != nil {
		t.Fatalf("FlattenMap error: %v", err)
	}
	if out["a"] != 1 || out["b.c"] != 2 || out["b.d.e"] != 3 {
		t.Errorf("FlattenMap output incorrect: %v", out)
	}

	// Test unsupported type
	in2 := map[string]any{"x": make(chan int)}
	out2 := make(map[string]any)
	if err := FlattenMap("", in2, out2); err == nil {
		t.Error("FlattenMap should error on unsupported type")
	}
}

func TestKeyValueSet_ErrorPaths(t *testing.T) {
	kv := KeyValue{}
	// Invalid JSON
	if err := kv.Set("{"); err == nil {
		t.Error("Set should error on invalid JSON")
	}
	// Invalid key-value
	if err := kv.Set("foo"); err == nil {
		t.Error("Set should error on missing '='")
	}
	// parseKeyValue error
	bad := KeyValue{}
	if err := parseKeyValue("foo", &bad); err == nil {
		t.Error("parseKeyValue should error on missing '='")
	}
}

func TestKeyValue_Type(t *testing.T) {
	kv := &KeyValue{}
	assert.Equal(t, "map[string]any", kv.Type())
}

func TestConfig_GetHeaders(t *testing.T) {
	tests := []struct {
		name    string
		headers KeyValue
		expect  map[string]string
	}{
		{
			name:    "string values",
			headers: KeyValue{"foo": "bar", "baz": "qux"},
			expect:  map[string]string{"foo": "bar", "baz": "qux"},
		},
		{
			name:    "bool values",
			headers: KeyValue{"a": true, "b": false},
			expect:  map[string]string{"a": "true", "b": "false"},
		},
		{
			name:    "mixed values",
			headers: KeyValue{"x": "y", "z": true},
			expect:  map[string]string{"x": "y", "z": "true"},
		},
		{
			name:    "empty",
			headers: KeyValue{},
			expect:  map[string]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Headers: tt.headers}
			assert.Equal(t, tt.expect, cfg.GetHeaders())
		})
	}
}

func TestConfig_InitAttributes(t *testing.T) {
	t.Run("flattens and injects sensitive marker", func(t *testing.T) {
		cfg := &Config{
			ResourceAttributes:  KeyValue{"foo": map[string]any{"bar": "baz"}, "secret": "val"},
			TelemetryAttributes: KeyValue{"a": map[string]any{"b": "c"}, "secret": "val"},
			Headers:             KeyValue{"h": map[string]any{"i": "j"}},
			SensitiveData:       []string{"secret"},
		}
		err := cfg.InitAttributes()
		require.NoError(t, err)
		// Check flattening
		assert.Equal(t, "baz", cfg.ResourceAttributes["foo.bar"])
		assert.Equal(t, "c", cfg.TelemetryAttributes["a.b"])
		assert.Equal(t, "j", cfg.Headers["h.i"])
		// Sensitive marker injected
		_, ok := cfg.ResourceAttributes["trazrgen.sensitive.data"]
		assert.True(t, ok)
		_, ok = cfg.TelemetryAttributes["trazrgen.sensitive.data"]
		assert.True(t, ok)
	})

	t.Run("flatten error propagates", func(t *testing.T) {
		cfg := &Config{
			ResourceAttributes: KeyValue{"bad": make(chan int)},
		}
		err := cfg.InitAttributes()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported attribute value type")
	})
}

func TestSplitCommaSeparated(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"a,b,c", []string{"a", "b", "c"}},
		{"a,\"b,c\",d", []string{"a", "\"b,c\"", "d"}},
		{"a,\"b,c,d\",e", []string{"a", "\"b,c,d\"", "e"}},
		{"a", []string{"a"}},
		{"", []string{}},
		{"\"a,b\",c", []string{"\"a,b\"", "c"}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitCommaSeparated(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestParseKeyValue(t *testing.T) {
	tests := []struct {
		input    string
		expected KeyValue
		hasError bool
	}{
		{"foo=bar", KeyValue{"foo": "bar"}, false},
		{"foo=123", KeyValue{"foo": 123}, false},
		{"foo=true", KeyValue{"foo": true}, false},
		{"foo=false", KeyValue{"foo": false}, false},
		{"foo=12.34", KeyValue{"foo": 12.34}, false},
		{"foo=\"quoted\"", KeyValue{"foo": "quoted"}, false},
		{"foo=", KeyValue{"foo": ""}, false},
		{"foo", KeyValue{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			kv := KeyValue{}
			err := parseKeyValue(tt.input, &kv)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, kv)
			}
		})
	}
}

func TestKeyValue_String(t *testing.T) {
	kv := &KeyValue{}
	assert.Empty(t, kv.String())
}

func TestConfig_SetDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	assert.Equal(t, 1, cfg.WorkerCount)
	assert.InEpsilon(t, 1.0, cfg.Rate, 1e-9)
	assert.Equal(t, time.Duration(0), cfg.TotalDuration)
	assert.Equal(t, 1*time.Second, cfg.ReportingInterval)
	assert.Equal(t, "localhost:4318", cfg.CustomEndpoint)
	assert.True(t, cfg.Insecure)
	assert.True(t, cfg.InsecureSkipVerify)
	assert.True(t, cfg.UseHTTP)
	assert.Equal(t, KeyValue{}, cfg.Headers)
	assert.Equal(t, KeyValue{}, cfg.ResourceAttributes)
	assert.Equal(t, "trazr-gen", cfg.ServiceName)
	assert.Equal(t, KeyValue{}, cfg.TelemetryAttributes)
	assert.Equal(t, []string{}, cfg.SensitiveData)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.True(t, cfg.MockData)
	assert.Equal(t, int64(0), cfg.MockSeed)
	assert.True(t, cfg.TerminalOutput)
	assert.False(t, cfg.ClientAuth.Enabled)
	assert.Empty(t, cfg.ClientAuth.ClientCertFile)
	assert.Empty(t, cfg.ClientAuth.ClientKeyFile)
}

func TestConfig_CommonFlags(t *testing.T) {
	cfg := &Config{}
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	cfg.CommonFlags(fs)
	// Set some flags and parse
	args := []string{
		"--workers=5",
		"--rate=2.5",
		"--duration=10s",
		"--interval=3s",
		"--otlp-endpoint=custom:1234",
		"--otlp-insecure=false",
		"--otlp-insecure-skip-verify=false",
		"--otlp-http=false",
		"--service=myservice",
		"--ca-cert=ca.pem",
		"--mtls=true",
		"--client-cert=cert.pem",
		"--client-key=key.pem",
		"--sensitive-data=foo,bar",
		"--mock-data=false",
		"--mock-seed=99",
		"--terminal-output=false",
	}
	err := fs.Parse(args)
	require.NoError(t, err)
	assert.Equal(t, 5, cfg.WorkerCount)
	assert.InEpsilon(t, 2.5, cfg.Rate, 1e-9)
	assert.Equal(t, 10*time.Second, cfg.TotalDuration)
	assert.Equal(t, 3*time.Second, cfg.ReportingInterval)
	assert.Equal(t, "custom:1234", cfg.CustomEndpoint)
	assert.False(t, cfg.Insecure)
	assert.False(t, cfg.InsecureSkipVerify)
	assert.False(t, cfg.UseHTTP)
	assert.Equal(t, "myservice", cfg.ServiceName)
	assert.Equal(t, "ca.pem", cfg.CaFile)
	assert.True(t, cfg.ClientAuth.Enabled)
	assert.Equal(t, "cert.pem", cfg.ClientAuth.ClientCertFile)
	assert.Equal(t, "key.pem", cfg.ClientAuth.ClientKeyFile)
	assert.ElementsMatch(t, []string{"foo", "bar"}, cfg.SensitiveData)
	assert.False(t, cfg.MockData)
	assert.Equal(t, int64(99), cfg.MockSeed)
	assert.False(t, cfg.TerminalOutput)
}

func TestClientAuthStruct(t *testing.T) {
	c := ClientAuth{}
	c.Enabled = true
	c.ClientCertFile = "cert.pem"
	c.ClientKeyFile = "key.pem"
	assert.True(t, c.Enabled)
	assert.Equal(t, "cert.pem", c.ClientCertFile)
	assert.Equal(t, "key.pem", c.ClientKeyFile)
}
