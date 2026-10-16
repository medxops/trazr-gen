package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	collectmetricpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

func TestStartExportsRequestedMetrics(t *testing.T) {
	for _, tc := range []struct {
		name       string
		metricType MetricType
	}{
		{name: "gauge", metricType: MetricTypeGauge},
		{name: "sum", metricType: MetricTypeSum},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			var received int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/metrics" {
					http.Error(w, "wrong path", http.StatusNotFound)
					return
				}
				payload, err := io.ReadAll(r.Body)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				var request collectmetricpb.ExportMetricsServiceRequest
				if err := proto.Unmarshal(payload, &request); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				count := 0
				for _, resourceMetrics := range request.ResourceMetrics {
					for _, scopeMetrics := range resourceMetrics.ScopeMetrics {
						for _, metric := range scopeMetrics.Metrics {
							if metric.Name != "test_metric" {
								http.Error(w, "wrong metric name", http.StatusBadRequest)
								return
							}
							switch tc.metricType {
							case MetricTypeGauge:
								if gauge := metric.GetGauge(); gauge != nil {
									count += len(gauge.DataPoints)
								}
							case MetricTypeSum:
								if sum := metric.GetSum(); sum != nil {
									count += len(sum.DataPoints)
								}
							}
						}
					}
				}
				mu.Lock()
				received += count
				mu.Unlock()
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			endpoint, err := url.Parse(server.URL)
			require.NoError(t, err)
			cfg := NewConfig()
			cfg.CustomEndpoint = endpoint.Host
			cfg.UseHTTP = true
			cfg.Insecure = true
			cfg.Rate = 0
			cfg.NumMetrics = 3
			cfg.MetricName = "test_metric"
			cfg.MetricType = tc.metricType
			cfg.TerminalOutput = false
			cfg.PrintDetails = false
			cfg.ShowProgressCounter = false
			require.NoError(t, Start(cfg, zap.NewNop()))

			mu.Lock()
			defer mu.Unlock()
			require.Equal(t, 3, received)
		})
	}
}
