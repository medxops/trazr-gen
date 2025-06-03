// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"

	"github.com/medxops/trazr-gen/internal/common"
)

func injectSensitiveHeaderMarker(cfg *Config) {
	sensitiveKeys := []string{}
	for _, k := range cfg.SensitiveData {
		if _, ok := cfg.Headers[k]; ok {
			sensitiveKeys = append(sensitiveKeys, k)
		}
	}
	if len(sensitiveKeys) > 0 {
		cfg.Headers["X-Trazr-Sensitive-Keys"] = strings.Join(sensitiveKeys, ",")
	} else {
		delete(cfg.Headers, "X-Trazr-Sensitive-Keys")
	}
}

// grpcExporterOptions creates the configuration options for a gRPC-based OTLP metric exporter.
// It configures the exporter with the provided endpoint, connection security settings, and headers.
func grpcExporterOptions(cfg *Config) ([]otlpmetricgrpc.Option, error) {
	grpcExpOpt := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(cfg.Endpoint()),
	}

	if cfg.Insecure {
		grpcExpOpt = append(grpcExpOpt, otlpmetricgrpc.WithInsecure())
	} else {
		credentials, err := common.GetTLSCredentialsForGRPCExporter(
			cfg.CaFile, cfg.ClientAuth, cfg.InsecureSkipVerify,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to get TLS credentials: %w", err)
		}
		grpcExpOpt = append(grpcExpOpt, otlpmetricgrpc.WithTLSCredentials(credentials))
	}

	injectSensitiveHeaderMarker(cfg)

	headers, err := cfg.GetHeadersWithMockMarker()
	if err != nil {
		return nil, err
	}
	if len(headers) > 0 {
		grpcExpOpt = append(grpcExpOpt, otlpmetricgrpc.WithHeaders(headers))
	}

	return grpcExpOpt, nil
}

// httpExporterOptions creates the configuration options for an HTTP-based OTLP metric exporter.
// It configures the exporter with the provided endpoint, URL path, connection security settings, and headers.
func httpExporterOptions(cfg *Config) ([]otlpmetrichttp.Option, error) {
	httpExpOpt := []otlpmetrichttp.Option{
		otlpmetrichttp.WithEndpoint(cfg.Endpoint()),
		otlpmetrichttp.WithURLPath(cfg.HTTPPath),
	}

	if cfg.Insecure {
		httpExpOpt = append(httpExpOpt, otlpmetrichttp.WithInsecure())
	} else {
		tlsCfg, err := common.GetTLSCredentialsForHTTPExporter(
			cfg.CaFile, cfg.ClientAuth, cfg.InsecureSkipVerify,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to get TLS credentials: %w", err)
		}
		httpExpOpt = append(httpExpOpt, otlpmetrichttp.WithTLSClientConfig(tlsCfg))
	}

	injectSensitiveHeaderMarker(cfg)

	headers, err := cfg.GetHeadersWithMockMarker()
	if err != nil {
		return nil, err
	}
	if len(headers) > 0 {
		httpExpOpt = append(httpExpOpt, otlpmetrichttp.WithHeaders(headers))
	}

	return httpExpOpt, nil
}
