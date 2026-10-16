package logs

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	collectlogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

func TestStartExportsRequestedLogs(t *testing.T) {
	var received int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/logs" {
			http.Error(w, "wrong path", http.StatusNotFound)
			return
		}
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var request collectlogspb.ExportLogsServiceRequest
		if err := proto.Unmarshal(payload, &request); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		for _, resourceLogs := range request.ResourceLogs {
			for _, scopeLogs := range resourceLogs.ScopeLogs {
				received += len(scopeLogs.LogRecords)
			}
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	endpoint, err := url.Parse(server.URL)
	require.NoError(t, err)
	cfg := NewConfig()
	cfg.CustomEndpoint = endpoint.Host
	cfg.Rate = 0
	cfg.NumLogs = 10
	cfg.TerminalOutput = false
	require.NoError(t, Start(cfg, zap.NewNop()))
	require.Equal(t, 10, received)
}

func TestStartRejectsUnreachableCollector(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	endpoint, err := url.Parse(server.URL)
	require.NoError(t, err)
	server.Close()

	cfg := NewConfig()
	cfg.CustomEndpoint = endpoint.Host
	cfg.TerminalOutput = false
	err = Start(cfg, zap.NewNop())
	require.ErrorContains(t, err, "cannot reach OTLP collector")
}

func TestStartReportsExportRejection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()
	endpoint, err := url.Parse(server.URL)
	require.NoError(t, err)

	cfg := NewConfig()
	cfg.CustomEndpoint = endpoint.Host
	cfg.TerminalOutput = false
	err = Start(cfg, zap.NewNop())
	require.ErrorContains(t, err, "export log")
	require.ErrorContains(t, err, "401")
}

func TestRunWaitsForFinalProgressCount(t *testing.T) {
	cfg := NewConfig()
	cfg.NumLogs = 10
	cfg.Rate = 0
	cfg.PrintDetails = false
	read, write, err := os.Pipe()
	require.NoError(t, err)
	original := os.Stdout
	os.Stdout = write
	t.Cleanup(func() { os.Stdout = original })

	err = run(context.Background(), cfg, &mockExporter{}, zap.NewNop())
	require.NoError(t, err)
	require.NoError(t, write.Close())
	output, err := io.ReadAll(read)
	require.NoError(t, err)
	require.NoError(t, read.Close())
	require.Contains(t, string(output), "Logs generated: 10\n")
	require.True(t, strings.HasSuffix(string(output), "Logs generated (final count): 10\n"))
}

func TestRunRejectsZeroWorkers(t *testing.T) {
	cfg := NewConfig()
	cfg.WorkerCount = 0
	require.ErrorContains(t, run(context.Background(), cfg, &mockExporter{}, zap.NewNop()), "workers must be greater than 0")
}
