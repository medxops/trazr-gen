package logs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestGrpcExporterOptions_Insecure(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	cfg.Insecure = true
	cfg.CustomEndpoint = "localhost:4317"
	opts, err := grpcExporterOptions(cfg)
	require.NoError(t, err)
	require.NotEmpty(t, opts)
}

func TestGrpcExporterOptions_TLS_Error(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	cfg.Insecure = false
	cfg.CaFile = "bad.pem"
	cfg.ClientAuth.Enabled = false
	cfg.CustomEndpoint = "localhost:4317"
	// This should fail because the CA file does not exist
	_, err := grpcExporterOptions(cfg)
	require.Error(t, err)
}

func TestHttpExporterOptions_Insecure(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	cfg.Insecure = true
	cfg.CustomEndpoint = "localhost:4318"
	opts, err := httpExporterOptions(cfg)
	require.NoError(t, err)
	require.NotEmpty(t, opts)
}

func TestHttpExporterOptions_TLS_Error(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	cfg.Insecure = false
	cfg.CaFile = "bad.pem"
	cfg.ClientAuth.Enabled = false
	cfg.CustomEndpoint = "localhost:4318"
	// This should fail because the CA file does not exist
	_, err := httpExporterOptions(cfg)
	require.Error(t, err)
}

func TestCreateExporter_HTTP(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	cfg.UseHTTP = true
	cfg.Insecure = true
	cfg.CustomEndpoint = "localhost:4318"
	logger := zaptest.NewLogger(t)

	exp, err := createExporter(cfg, logger)
	require.NoError(t, err)
	require.NotNil(t, exp)
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
	logger := zaptest.NewLogger(t)

	exp, err := createExporter(cfg, logger)
	require.NoError(t, err)
	require.NotNil(t, exp)
	if exp != nil {
		t.Cleanup(func() { _ = exp.Shutdown(context.Background()) })
	}
}

func TestCreateExporter_HTTP_TLS_Error(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	cfg.UseHTTP = true
	cfg.Insecure = false
	cfg.CaFile = "bad.pem"
	cfg.CustomEndpoint = "localhost:4318"
	logger := zaptest.NewLogger(t)

	_, err := createExporter(cfg, logger)
	require.Error(t, err)
}

func TestCreateExporter_gRPC_TLS_Error(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	cfg.UseHTTP = false
	cfg.Insecure = false
	cfg.CaFile = "bad.pem"
	cfg.CustomEndpoint = "localhost:4317"
	logger := zaptest.NewLogger(t)

	_, err := createExporter(cfg, logger)
	require.Error(t, err)
}
