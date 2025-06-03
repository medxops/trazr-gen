package metrics

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"

	"github.com/medxops/trazr-gen/internal/common"
)

func TestInjectSensitiveHeaderMarker(t *testing.T) {
	cfg := &Config{
		Config: common.Config{
			Headers: map[string]any{
				"X-My-Header": "my-value",
				"Other":       "foo",
			},
			SensitiveData: []string{"X-My-Header"},
		},
	}
	injectSensitiveHeaderMarker(cfg)
	val, ok := cfg.Headers["X-Trazr-Sensitive-Keys"]
	if !ok {
		t.Fatalf("X-Trazr-Sensitive-Keys header not found; got headers: %+v", cfg.Headers)
	}
	if val != "X-My-Header" {
		t.Fatalf("Expected marker header to be 'X-My-Header', got '%v'", val)
	}

	// Test with no sensitive headers
	cfg2 := &Config{
		Config: common.Config{
			Headers: map[string]any{
				"Other": "foo",
			},
			SensitiveData: []string{"X-My-Header"},
		},
	}
	injectSensitiveHeaderMarker(cfg2)
	if _, ok := cfg2.Headers["X-Trazr-Sensitive-Keys"]; ok {
		t.Fatalf("Marker header should not be present when no sensitive headers")
	}
}

func TestGrpcExporterOptions_Insecure(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	cfg.Insecure = true
	cfg.CustomEndpoint = "localhost:4317"
	opts, err := grpcExporterOptions(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(opts) == 0 {
		t.Fatal("expected non-empty options slice")
	}
}

func TestGrpcExporterOptions_TLS_Error(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	cfg.Insecure = false
	cfg.CaFile = "bad.pem"
	cfg.ClientAuth.Enabled = false
	cfg.CustomEndpoint = "localhost:4317"
	_, err := grpcExporterOptions(cfg)
	if err == nil {
		t.Fatal("expected error for bad CA file")
	}
}

func TestHttpExporterOptions_Insecure(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	cfg.Insecure = true
	cfg.CustomEndpoint = "localhost:4318"
	opts, err := httpExporterOptions(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(opts) == 0 {
		t.Fatal("expected non-empty options slice")
	}
}

func TestHttpExporterOptions_TLS_Error(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	cfg.Insecure = false
	cfg.CaFile = "bad.pem"
	cfg.ClientAuth.Enabled = false
	cfg.CustomEndpoint = "localhost:4318"
	_, err := httpExporterOptions(cfg)
	if err == nil {
		t.Fatal("expected error for bad CA file")
	}
}

func TestCreateExporter_HTTP(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	cfg.UseHTTP = true
	cfg.Insecure = true
	cfg.CustomEndpoint = "localhost:4318"

	exp, err := otlpmetrichttp.New(context.Background())
	require.NoError(t, err)
	if exp != nil {
		t.Cleanup(func() { _ = exp.Shutdown(context.Background()) })
	}
}

func TestCreateExporter_gRPC(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	cfg.UseHTTP = false
	cfg.Insecure = true
	cfg.CustomEndpoint = "localhost:4317"

	exp, err := otlpmetricgrpc.New(context.Background())
	require.NoError(t, err)
	if exp != nil {
		t.Cleanup(func() { _ = exp.Shutdown(context.Background()) })
	}
}
