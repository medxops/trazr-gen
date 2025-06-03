// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package e2etest

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/medxops/trazr-gen/pkg/logs"
)

func TestGenerateLogsWithSelectiveSensitiveAttributes(t *testing.T) {
	f := otlpreceiver.NewFactory()
	sink := &consumertest.LogsSink{}
	rCfg := f.CreateDefaultConfig()
	endpoint := getAvailableLocalAddress(t)
	rCfg.(*otlpreceiver.Config).GRPC.NetAddr.Endpoint = endpoint
	rCfg.(*otlpreceiver.Config).HTTP = nil
	r, err := f.CreateLogs(context.Background(), receivertest.NewNopSettings(f.Type()), rCfg, sink)
	require.NoError(t, err)
	err = r.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() { require.NoError(t, r.Shutdown(context.Background())) }()

	cfg := logs.NewConfig()
	cfg.WorkerCount = 1
	cfg.Rate = 1
	cfg.TotalDuration = 1 * time.Second
	cfg.CustomEndpoint = endpoint
	cfg.Insecure = true
	cfg.NumLogs = 1
	cfg.ResourceAttributes = map[string]any{"a": "A", "b": "B", "c": "C"}
	cfg.TelemetryAttributes = map[string]any{"d": "D", "e": "E", "f": "F"}
	cfg.SensitiveData = []string{"a", "d"}
	cfg.UseHTTP = false
	cfg.TerminalOutput = false

	go func() { _ = logs.Start(cfg, zap.NewNop()) }()

	require.Eventually(t, func() bool {
		return len(sink.AllLogs()) > 0
	}, 5*time.Second, 100*time.Millisecond)

	// Check resource attributes for sensitive data
	resAttrs := sink.AllLogs()[0].ResourceLogs().At(0).Resource().Attributes()
	resVal, resOk := resAttrs.Get("trazr.sensitive.data")
	if !resOk {
		t.Fatalf("trazr.sensitive.data attribute not found on resource; got attributes: %+v", resAttrs.AsRaw())
	}
	assert.Contains(t, resVal.AsString(), "a", "'a' should be marked sensitive (resource)")
	assert.NotContains(t, resVal.AsString(), "c", "'c' should NOT be marked sensitive (resource)")

	// Check log record attributes for sensitive data
	logRec := sink.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	val, ok := logRec.Attributes().Get("trazr.sensitive.data")
	if !ok {
		t.Fatalf("trazr.sensitive.data attribute not found on log record; got attributes: %+v", logRec.Attributes().AsRaw())
	}
	assert.Contains(t, val.AsString(), "d", "'d' should be marked sensitive (log)")
	assert.NotContains(t, val.AsString(), "f", "'f' should NOT be marked sensitive (log)")
}
