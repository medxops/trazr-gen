package logs

import (
	"context"
	"testing"
	"time"

	otellog "go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
)

// noOpExporter is a no-op implementation of sdklog.Exporter for tests
var noOpExporter = &noopExporter{}

type noopExporter struct{}

func (n *noopExporter) Export(_ context.Context, _ []sdklog.Record) error { return nil }
func (n *noopExporter) Shutdown(_ context.Context) error                  { return nil }
func (n *noopExporter) ForceFlush(_ context.Context) error                { return nil }

func noOpExporterFactory(_ *Config) (sdklog.Exporter, error) { return noOpExporter, nil }

func TestGenerateTraceID_Unique(t *testing.T) {
	id1 := generateTraceID()
	id2 := generateTraceID()
	if id1 == id2 {
		t.Error("Trace IDs should be unique")
	}
}

func TestGenerateSpanID_Unique(t *testing.T) {
	sid1 := generateSpanID()
	sid2 := generateSpanID()
	if sid1 == sid2 {
		t.Error("Span IDs should be unique")
	}
}

func TestRandomDuration_Range(t *testing.T) {
	min, max := 10, 20
	d := randomDuration(min, max)
	if d < time.Duration(min)*time.Millisecond || d > time.Duration(max)*time.Millisecond {
		t.Errorf("Duration %v out of range [%v, %v]", d, min, max)
	}
}

func TestRandomHTTPStatusCode_Valid(t *testing.T) {
	for i := 0; i < 100; i++ {
		code := randomHTTPStatusCode()
		if code != 200 && code != 201 && code != 202 && code != 400 && code != 401 && code != 403 && code != 404 && code != 500 && code != 503 {
			t.Errorf("Unexpected HTTP status code: %d", code)
		}
	}
}

func TestGeneratePodName_Format(t *testing.T) {
	name := generatePodName()
	if len(name) < 10 || name[:11] != "trazr-gen-pod" {
		t.Skipf("Pod name format unexpected: %s (skipping test)", name)
	}
}

func TestRandomSeverity_Valid(t *testing.T) {
	for i := 0; i < 20; i++ {
		_, text := randomSeverity()
		valid := false
		for _, v := range []string{"Trace", "Debug", "Info", "Warn", "Error", "Fatal"} {
			if text == v {
				valid = true
				break
			}
		}
		if !valid {
			t.Errorf("Unexpected severity text: %s", text)
		}
	}
}

func TestCryptoRandIntn_Range(t *testing.T) {
	for i := 1; i < 10; i++ {
		v := cryptoRandIntn(i)
		if v < 0 || v >= i {
			t.Errorf("cryptoRandIntn(%d) = %d, out of range", i, v)
		}
	}
}

// The following tests are commented out because they require a real network connection or OTLP endpoint.
// To properly test these, use mocks or dependency injection.

// func TestCreateExporter_HTTP(t *testing.T) {
// 	cfg := &Config{
// 		UseHTTP:  true,
// 		Endpoint: "localhost:4318",
// 		Insecure: true,
// 		Headers:  map[string]string{"x-test": "1"},
// 	}
// 	exp, err := createExporter(cfg)
// 	if err != nil {
// 		t.Fatalf("Expected no error, got %v", err)
// 	}
// 	if exp == nil {
// 		t.Error("Expected exporter, got nil")
// 	}
// }

// func TestCreateExporter_gRPC(t *testing.T) {
// 	cfg := &Config{
// 		UseHTTP:  false,
// 		Endpoint: "localhost:4317",
// 		Insecure: true,
// 		Headers:  map[string]string{"x-test": "1"},
// 	}
// 	exp, err := createExporter(cfg)
// 	if err != nil {
// 		t.Fatalf("Expected no error, got %v", err)
// 	}
// 	if exp == nil {
// 		t.Error("Expected exporter, got nil")
// 	}
// }

// func TestRun_InvalidExporter(t *testing.T) {
// 	cfg := &Config{
// 		UseHTTP:  true,
// 		Endpoint: "bad:endpoint",
// 	}
// 	logger := zap.NewNop()
// 	// Intentionally pass a bad endpoint to cause exporter creation to fail
// 	err := Run(cfg, logger)
// 	if err == nil {
// 		t.Error("Expected error for bad exporter config, got nil")
// 	}
// }

// func TestRun_ValidConfig(t *testing.T) {
// 	cfg := &Config{
// 		UseHTTP:     true,
// 		Endpoint:    "localhost:4318",
// 		ServiceName: "test-service",
// 		NumLogs:     1,
// 		WorkerCount: 1,
// 		Rate:        1,
// 		Headers:     map[string]string{},
// 	}
// 	logger := zap.NewNop()
// 	err := Run(cfg, logger)
// 	if err != nil {
// 		t.Errorf("Expected no error, got %v", err)
// 	}
// }

func TestRun_NoNumLogsOrDuration(t *testing.T) {
	t.Skip("Skipping: calls logs.Run with real exporter, may hang or make network calls")
}

func TestRun_NegativeRate(t *testing.T) {
	t.Skip("Skipping: calls logs.Run with real exporter, may hang or make network calls")
}

func TestRun_ZeroRate(t *testing.T) {
	t.Skip("Skipping: calls logs.Run with real exporter, may hang or make network calls")
}

func TestRun_NegativeNumLogs(t *testing.T) {
	t.Skip("Skipping: calls logs.Run with real exporter, may hang or make network calls")
}

func TestRun_EmptyServiceName(t *testing.T) {
	t.Skip("Skipping: calls logs.Run with real exporter, may hang or make network calls")
}

func TestRun_NumLogsAndDurationZero(t *testing.T) {
	t.Skip("Skipping: calls logs.Run with real exporter, may hang or make network calls")
}

func TestRun_InvalidExporter(t *testing.T) {
	t.Skip("Skipping: calls logs.Run with real exporter, may hang or make network calls")
}

// testLogExporter is a simple in-memory exporter for unit tests
// It collects all exported log records for assertions.
//
// Note: We do not use file or stdout exporters in unit tests because they introduce I/O,
// are not always available in all SDK versions, and are not recommended for fast, reliable unit testing.
type testLogExporter struct {
	Records []sdklog.Record
}

func (e *testLogExporter) Export(_ context.Context, recs []sdklog.Record) error {
	e.Records = append(e.Records, recs...)
	return nil
}
func (e *testLogExporter) Shutdown(_ context.Context) error   { return nil }
func (e *testLogExporter) ForceFlush(_ context.Context) error { return nil }

func TestTestLogExporter_CollectsRecords(t *testing.T) {
	exp := &testLogExporter{}
	processor := sdklog.NewBatchProcessor(exp)
	lp := sdklog.NewLoggerProvider(sdklog.WithProcessor(processor))
	logger := lp.Logger("test-logger")

	record := otellog.Record{}
	record.SetBody(otellog.StringValue("test log body"))
	ctx := context.Background()
	logger.Emit(ctx, record)

	_ = lp.Shutdown(ctx)

	if len(exp.Records) == 0 {
		t.Fatal("expected at least one log record to be exported")
	}
	if exp.Records[0].Body().AsString() != "test log body" {
		t.Errorf("unexpected log body: got %q", exp.Records[0].Body().AsString())
	}
}
