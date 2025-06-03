package logs

import (
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigFileMapping(t *testing.T) {
	yaml := `
logs: 10
otlp-endpoint: localhost:4318
service: trazr-gen
body: test-body
severity-text: Debug
severity-number: 5
trace-id: 1234567890abcdef1234567890abcdef
span-id: 1234567890abcdef
`
	v := viper.New()
	v.SetConfigType("yaml")
	require.NoError(t, v.ReadConfig(strings.NewReader(yaml)))

	var cfg Config
	require.NoError(t, v.Unmarshal(&cfg))

	assert.Equal(t, 10, cfg.NumLogs)
	assert.Equal(t, "localhost:4318", cfg.CustomEndpoint)
	assert.Equal(t, "trazr-gen", cfg.ServiceName)
	assert.Equal(t, "test-body", cfg.Body)
	assert.Equal(t, "Debug", cfg.SeverityText)
	assert.Equal(t, "5", cfg.SeverityNumber)
	assert.Equal(t, "1234567890abcdef1234567890abcdef", cfg.TraceID)
	assert.Equal(t, "1234567890abcdef", cfg.SpanID)
}

func TestSetDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	if cfg.HTTPPath != "/v1/logs" || cfg.NumLogs != 1 || cfg.Body != "Log message" || cfg.SeverityText != "Info" || cfg.SeverityNumber != "9" {
		t.Error("SetDefaults did not set expected values")
	}
}

func TestFlags(t *testing.T) {
	cfg := &Config{}
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	cfg.Flags(fs)
	// Just check that flags are registered and can be parsed
	err := fs.Parse([]string{"--logs=5", "--body=foo", "--severity-text=Warn", "--severity-number=2", "--trace-id=123", "--span-id=456"})
	if err != nil {
		t.Errorf("Flags parsing failed: %v", err)
	}
}

func TestGetHeaders(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	cfg.Headers["foo"] = "bar"
	h := cfg.GetHeaders()
	if h["foo"] != "bar" {
		t.Error("GetHeaders did not return expected header")
	}
}

func TestIsMockDataEnabled(t *testing.T) {
	cfg := &Config{}
	cfg.MockData = true
	if !cfg.IsMockDataEnabled() {
		t.Error("IsMockDataEnabled should return true when MockData is true")
	}
}

func TestInitAttributes(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	if err := cfg.InitAttributes(); err != nil {
		t.Errorf("InitAttributes returned error: %v", err)
	}
}

func TestNewConfig(t *testing.T) {
	cfg := NewConfig()
	require.NotNil(t, cfg)
	assert.Equal(t, "/v1/logs", cfg.HTTPPath)
	assert.Equal(t, 1, cfg.NumLogs)
	assert.Equal(t, "Log message", cfg.Body)
	assert.Equal(t, "Info", cfg.SeverityText)
	assert.Equal(t, "9", cfg.SeverityNumber)
	assert.Empty(t, cfg.TraceID)
	assert.Empty(t, cfg.SpanID)
}
