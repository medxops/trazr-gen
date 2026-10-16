package traces

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	collecttracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

func TestStartExportsLegacyAndEnhancedTraces(t *testing.T) {
	for _, tc := range []struct {
		name     string
		enhanced bool
	}{
		{name: "legacy"},
		{name: "enhanced", enhanced: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			var spans []*tracepb.Span
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/traces" {
					http.Error(w, "wrong path", http.StatusNotFound)
					return
				}
				payload, err := io.ReadAll(r.Body)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				var request collecttracepb.ExportTraceServiceRequest
				if err := proto.Unmarshal(payload, &request); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				mu.Lock()
				for _, resourceSpans := range request.ResourceSpans {
					for _, scopeSpans := range resourceSpans.ScopeSpans {
						spans = append(spans, scopeSpans.Spans...)
					}
				}
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
			cfg.Batch = false
			cfg.Rate = 0
			cfg.NumTraces = 1
			cfg.Enhanced = tc.enhanced
			cfg.Pattern = "chain"
			cfg.SpansPerTrace = 3
			cfg.TerminalOutput = false
			cfg.PrintDetails = false
			cfg.ShowProgressCounter = false

			previousProvider := otel.GetTracerProvider()
			defer otel.SetTracerProvider(previousProvider)
			require.NoError(t, Start(cfg, zap.NewNop()))

			mu.Lock()
			defer mu.Unlock()
			require.GreaterOrEqual(t, len(spans), 2)
			traceID := spans[0].TraceId
			var hasRoot, hasChild bool
			for _, span := range spans {
				require.Equal(t, traceID, span.TraceId)
				hasRoot = hasRoot || len(span.ParentSpanId) == 0
				hasChild = hasChild || len(span.ParentSpanId) > 0
			}
			require.True(t, hasRoot, "expected a root span")
			require.True(t, hasChild, "expected a child span")
		})
	}
}
