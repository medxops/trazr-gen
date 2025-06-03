// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces

import (
	"context"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/medxops/trazr-gen/internal/common"
)

func TestHTTPExporterOptions_TLS(t *testing.T) {
	// TODO add test cases for mTLS
	for name, tc := range map[string]struct {
		tls         bool
		tlsServerCA bool // use the httptest.Server's TLS cert as the CA
		cfg         Config

		expectTransportError bool
	}{
		"Insecure": {
			tls: false,
			cfg: Config{Config: common.Config{Insecure: true}},
		},
		"InsecureSkipVerify": {
			tls: true,
			cfg: Config{Config: common.Config{InsecureSkipVerify: true}},
		},
		"InsecureSkipVerifyDisabled": {
			tls:                  true,
			expectTransportError: true,
		},
		"CaFile": {
			tls:         true,
			tlsServerCA: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			var called bool
			var h http.HandlerFunc = func(http.ResponseWriter, *http.Request) {
				called = true
			}
			var srv *httptest.Server
			if tc.tls {
				srv = httptest.NewTLSServer(h)
			} else {
				srv = httptest.NewServer(h)
			}
			defer srv.Close()
			srvURL, _ := url.Parse(srv.URL)

			cfg := tc.cfg
			cfg.CustomEndpoint = srvURL.Host
			if tc.tlsServerCA {
				caFile := filepath.Join(t.TempDir(), "cert.pem")
				err := os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{
					Type:  "CERTIFICATE",
					Bytes: srv.TLS.Certificates[0].Certificate[0],
				}), 0o600)
				require.NoError(t, err)
				cfg.CaFile = caFile
			}

			opts, err := httpExporterOptions(&cfg)
			require.NoError(t, err)
			client := otlptracehttp.NewClient(opts...)

			err = client.UploadTraces(context.Background(), []*tracepb.ResourceSpans{})
			if tc.expectTransportError {
				require.Error(t, err)
				assert.False(t, called)
			} else {
				require.NoError(t, err)
				assert.True(t, called)
			}
		})
	}
}

func TestHTTPExporterOptions_HTTP(t *testing.T) {
	for name, tc := range map[string]struct {
		cfg Config

		expectedHTTPPath string
		expectedHeader   http.Header
	}{
		"HTTPPath": {
			cfg:              Config{Config: common.Config{HTTPPath: "/foo"}},
			expectedHTTPPath: "/foo",
		},
		"Headers": {
			cfg: Config{
				Config: common.Config{Headers: map[string]any{"a": "b"}},
			},
			expectedHTTPPath: "/v1/traces",
			expectedHeader:   http.Header{"a": []string{"b"}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			var httpPath string
			var header http.Header
			var h http.HandlerFunc = func(_ http.ResponseWriter, r *http.Request) {
				httpPath = r.URL.Path
				header = r.Header
			}
			srv := httptest.NewServer(h)
			defer srv.Close()
			srvURL, _ := url.Parse(srv.URL)

			cfg := tc.cfg
			cfg.Insecure = true
			cfg.CustomEndpoint = srvURL.Host
			opts, err := httpExporterOptions(&cfg)
			require.NoError(t, err)
			client := otlptracehttp.NewClient(opts...)

			err = client.UploadTraces(context.Background(), []*tracepb.ResourceSpans{})
			require.NoError(t, err)
			assert.Equal(t, tc.expectedHTTPPath, httpPath)
			for k, expected := range tc.expectedHeader {
				assert.Equal(t, expected, []string{header.Get(k)})
			}
		})
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

func TestCreateExporter_HTTP(t *testing.T) {
	cfg := &Config{}
	cfg.SetDefaults()
	cfg.UseHTTP = true
	cfg.Insecure = true
	cfg.CustomEndpoint = "localhost:4318"

	exp, err := otlptracehttp.New(context.Background())
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

	exp, err := otlptracegrpc.New(context.Background())
	require.NoError(t, err)
	if exp != nil {
		t.Cleanup(func() { _ = exp.Shutdown(context.Background()) })
	}
}
