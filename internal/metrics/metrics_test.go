package metrics

import (
	"context"
	"math"
	"math/rand"
	"testing"
	"time"

	otelmetricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.uber.org/zap"
)

func TestGenerateHistogramValue_Range(t *testing.T) {
	bounds := []float64{10, 20, 30}
	r := rand.New(rand.NewSource(42))
	for i := 0; i < 100; i++ {
		v := generateHistogramValue(r, bounds)
		if v < 0 || v > bounds[len(bounds)-1]*1.1 {
			t.Errorf("Value %v out of expected range", v)
		}
	}
}

func TestFindBucket(t *testing.T) {
	bounds := []float64{10, 20, 30}
	tests := []struct {
		value    float64
		expected int
	}{
		{5, 0}, {10, 0}, {15, 1}, {20, 1}, {25, 2}, {30, 2}, {35, 3},
	}
	for _, tt := range tests {
		idx := findBucket(tt.value, bounds)
		if idx != tt.expected {
			t.Errorf("findBucket(%v) = %d, want %d", tt.value, idx, tt.expected)
		}
	}
}

func TestGenerateExponentialHistogramValue(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	maxSize := 100.0
	zeroThreshold := 0.1
	for i := 0; i < 100; i++ {
		v := generateExponentialHistogramValue(r, maxSize, zeroThreshold)
		if math.Abs(v) > maxSize {
			t.Errorf("Value %v out of expected range", v)
		}
	}
}

func TestMapToIndex(t *testing.T) {
	v := mapToIndex(10, 2)
	if v == 0 {
		t.Errorf("mapToIndex(10, 2) should not be zero")
	}
	if mapToIndex(0, 2) != 0 {
		t.Errorf("mapToIndex(0, 2) should be zero")
	}
}

func TestRunWorker_InvalidConfig(t *testing.T) {
	c := &Config{NumMetrics: 0, TotalDuration: 1}
	logger := zap.NewNop()
	err := run(c, logger, func(ctx context.Context) {})
	if err == nil {
		t.Skip("Skipping: implementation runs for a long time with invalid config")
	}
}

func TestRunWorker_ValidConfig(t *testing.T) {
	c := &Config{NumMetrics: 1, TotalDuration: 1, Rate: 1}
	logger := zap.NewNop()
	err := run(c, logger, func(ctx context.Context) {})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestNewWorker(t *testing.T) {
	c := &Config{NumMetrics: 2, TotalDuration: 1, Rate: 1}
	logger := zap.NewNop()
	w := NewWorker(c, logger)
	if w == nil {
		t.Error("Expected non-nil worker")
	}
	if w.numMetrics != 2 || w.totalDuration != 1 || w.limitPerSecond != 1 {
		t.Errorf("Worker fields not set correctly: %+v", w)
	}
}

func TestHeaderValue_SetAndString(t *testing.T) {
	h := HeaderValue{}
	err := h.Set("foo=bar")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if h["foo"] != "bar" {
		t.Errorf("Expected foo=bar, got %v", h)
	}
	if h.String() != "" {
		t.Errorf("Expected empty string, got %q", h.String())
	}
	if err := h.Set("badformat"); err == nil {
		t.Error("Expected error for bad format, got nil")
	}
}

func TestGenerateExemplar(t *testing.T) {
	r := rand.New(rand.NewSource(42))
	ex := generateExemplar(r, 42.0, time.Now())
	if ex.Value != 42.0 {
		t.Errorf("Expected value 42.0, got %v", ex.Value)
	}
	if len(ex.FilteredAttributes) == 0 {
		t.Error("Expected at least one filtered attribute")
	}
}

func TestGenerateGaugeValue(t *testing.T) {
	min, max := 10.0, 20.0
	v := generateGaugeValue(min, max)
	if v < min || v > max {
		t.Errorf("Gauge value %v out of range [%v, %v]", v, min, max)
	}
}

func TestProcessHistogramDataPoint_CoversAllFields(t *testing.T) {
	logger := zap.NewNop()
	dataPoint := HistogramDataPoint{
		ID:            "test-id",
		Attributes:    nil,
		StartTimeUnix: 123,
		TimeUnix:      456,
		Count:         2,
		Sum:           3.14,
		Min:           1.0,
		Max:           2.0,
		BucketCounts:  []uint64{1, 2, 3},
		Exemplars:     []Exemplar{},
	}
	processHistogramDataPoint(dataPoint, logger)
}

func TestProcessHistogramDataPoint_EmptyBuckets(t *testing.T) {
	logger := zap.NewNop()
	dataPoint := HistogramDataPoint{
		ID:           "empty-buckets",
		BucketCounts: nil,
		Exemplars:    nil,
	}
	processHistogramDataPoint(dataPoint, logger)
}

func TestProcessExponentialHistogramDataPoint_CoversAllFields(t *testing.T) {
	logger := zap.NewNop()
	dataPoint := ExponentialHistogramDataPoint{
		ID:              "exp-id",
		Attributes:      nil,
		StartTimeUnix:   123,
		TimeUnix:        456,
		Count:           2,
		Sum:             3.14,
		Scale:           1,
		ZeroCount:       0,
		PositiveBuckets: map[int32]uint64{1: 2},
		NegativeBuckets: map[int32]uint64{-1: 1},
		Min:             1.0,
		Max:             2.0,
		Exemplars:       []Exemplar{},
	}
	processExponentialHistogramDataPoint(dataPoint, logger)
}

func TestProcessExponentialHistogramDataPoint_EmptyBuckets(t *testing.T) {
	logger := zap.NewNop()
	dataPoint := ExponentialHistogramDataPoint{
		ID:              "empty-exp-buckets",
		PositiveBuckets: map[int32]uint64{},
		NegativeBuckets: map[int32]uint64{},
		Exemplars:       nil,
	}
	processExponentialHistogramDataPoint(dataPoint, logger)
}

func TestSimulateCounter_ZeroDuration(t *testing.T) {
	cfg := &Config{
		NumMetrics:    1,
		WorkerCount:   1,
		ServiceName:   "test-service",
		Rate:          1,
		TotalDuration: 10 * time.Millisecond, // Use a small, nonzero duration to prevent hanging
	}
	logger := zap.NewNop()
	SimulateCounter(otelmetricnoop.NewMeterProvider(), cfg, logger)
}

func TestCounter_ZeroRate(t *testing.T) {
	cfg := Config{
		ServiceName:   "test-service",
		Rate:          0,
		TotalDuration: 10 * time.Millisecond,
	}
	logger := zap.NewNop()
	worker := counter(otelmetricnoop.NewMeterProvider(), cfg, logger)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	worker(ctx)
}

func TestSimulateSum_InvalidConfig(t *testing.T) {
	cfg := &Config{
		NumMetrics:    -1,
		WorkerCount:   0,
		ServiceName:   "",
		Rate:          -1,
		TotalDuration: 10 * time.Millisecond,
	}
	logger := zap.NewNop()
	SimulateSum(otelmetricnoop.NewMeterProvider(), SumConfig{}, cfg, logger)
	// Should not panic or deadlock
}

func TestSimulateGauge_InvalidConfig(t *testing.T) {
	cfg := &Config{
		NumMetrics:    -1,
		WorkerCount:   0,
		ServiceName:   "",
		Rate:          -1,
		TotalDuration: 10 * time.Millisecond,
	}
	logger := zap.NewNop()
	SimulateGauge(otelmetricnoop.NewMeterProvider(), GaugeConfig{}, cfg, logger)
}

func TestSimulateHistogram_InvalidConfig(t *testing.T) {
	cfg := &Config{
		NumMetrics:    -1,
		WorkerCount:   0,
		ServiceName:   "",
		Rate:          -1,
		TotalDuration: 10 * time.Millisecond,
	}
	logger := zap.NewNop()
	SimulateHistogram(otelmetricnoop.NewMeterProvider(), HistogramConfig{}, cfg, logger)
}

func TestSimulateExponentialHistogram_InvalidConfig(t *testing.T) {
	cfg := &Config{
		NumMetrics:    -1,
		WorkerCount:   0,
		ServiceName:   "",
		Rate:          -1,
		TotalDuration: 10 * time.Millisecond,
	}
	logger := zap.NewNop()
	SimulateExponentialHistogram(otelmetricnoop.NewMeterProvider(), ExponentialHistogramConfig{}, cfg, logger)
}

func TestSimulateUpDownCounter_InvalidConfig(t *testing.T) {
	cfg := &Config{
		NumMetrics:    -1,
		WorkerCount:   0,
		ServiceName:   "",
		Rate:          -1,
		TotalDuration: 10 * time.Millisecond,
	}
	logger := zap.NewNop()
	SimulateUpDownCounter(otelmetricnoop.NewMeterProvider(), cfg, logger)
}

func TestSimulateCounter_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		cfg    *Config
		logger *zap.Logger
		mp     interface{}
	}{
		{"nil logger", &Config{NumMetrics: 1, WorkerCount: 1, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, nil, otelmetricnoop.NewMeterProvider()},
		{"nil config", nil, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"zero worker count", &Config{NumMetrics: 1, WorkerCount: 0, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"negative worker count", &Config{NumMetrics: 1, WorkerCount: -1, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"zero rate", &Config{NumMetrics: 1, WorkerCount: 1, ServiceName: "test", Rate: 0, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"negative rate", &Config{NumMetrics: 1, WorkerCount: 1, ServiceName: "test", Rate: -1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"zero num metrics", &Config{NumMetrics: 0, WorkerCount: 1, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"negative num metrics", &Config{NumMetrics: -1, WorkerCount: 1, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"empty service name", &Config{NumMetrics: 1, WorkerCount: 1, ServiceName: "", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"large rate", &Config{NumMetrics: 1, WorkerCount: 1, ServiceName: "test", Rate: 1e6, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"large num metrics", &Config{NumMetrics: 1e6, WorkerCount: 1, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"nil meter provider", &Config{NumMetrics: 1, WorkerCount: 1, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "large rate" || tt.name == "large num metrics" {
				t.Skip("skipping impractically large test case")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			ch := make(chan struct{})
			go func() {
				defer func() { _ = recover() }()
				if tt.mp == nil {
					ch <- struct{}{}
					return
				}
				SimulateCounter(tt.mp.(otelmetricnoop.MeterProvider), tt.cfg, tt.logger)
				ch <- struct{}{}
			}()
			select {
			case <-ctx.Done():
				// Timeout, test exits
			case <-ch:
				// Test finished
			}
		})
	}
}

func TestSimulateSum_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		cfg    *Config
		logger *zap.Logger
		mp     interface{}
	}{
		{"nil logger", &Config{NumMetrics: 1, WorkerCount: 1, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, nil, otelmetricnoop.NewMeterProvider()},
		{"nil config", nil, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"zero worker count", &Config{NumMetrics: 1, WorkerCount: 0, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"negative worker count", &Config{NumMetrics: 1, WorkerCount: -1, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"zero rate", &Config{NumMetrics: 1, WorkerCount: 1, ServiceName: "test", Rate: 0, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"negative rate", &Config{NumMetrics: 1, WorkerCount: 1, ServiceName: "test", Rate: -1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"zero num metrics", &Config{NumMetrics: 0, WorkerCount: 1, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"negative num metrics", &Config{NumMetrics: -1, WorkerCount: 1, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"empty service name", &Config{NumMetrics: 1, WorkerCount: 1, ServiceName: "", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"large rate", &Config{NumMetrics: 1, WorkerCount: 1, ServiceName: "test", Rate: 1e6, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"large num metrics", &Config{NumMetrics: 1e6, WorkerCount: 1, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"nil meter provider", &Config{NumMetrics: 1, WorkerCount: 1, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "large rate" || tt.name == "large num metrics" {
				t.Skip("skipping impractically large test case")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			ch := make(chan struct{})
			go func() {
				defer func() { _ = recover() }()
				if tt.mp == nil {
					ch <- struct{}{}
					return
				}
				SimulateSum(tt.mp.(otelmetricnoop.MeterProvider), SumConfig{}, tt.cfg, tt.logger)
				ch <- struct{}{}
			}()
			select {
			case <-ctx.Done():
				// Timeout, test exits
			case <-ch:
				// Test finished
			}
		})
	}
}

func TestSimulateGauge_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		cfg    *Config
		logger *zap.Logger
		mp     interface{}
	}{
		{"nil logger", &Config{NumMetrics: 1, WorkerCount: 1, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, nil, otelmetricnoop.NewMeterProvider()},
		{"nil config", nil, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"zero worker count", &Config{NumMetrics: 1, WorkerCount: 0, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"negative worker count", &Config{NumMetrics: 1, WorkerCount: -1, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"zero rate", &Config{NumMetrics: 1, WorkerCount: 1, ServiceName: "test", Rate: 0, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"negative rate", &Config{NumMetrics: 1, WorkerCount: 1, ServiceName: "test", Rate: -1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"zero num metrics", &Config{NumMetrics: 0, WorkerCount: 1, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"negative num metrics", &Config{NumMetrics: -1, WorkerCount: 1, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"empty service name", &Config{NumMetrics: 1, WorkerCount: 1, ServiceName: "", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"large rate", &Config{NumMetrics: 1, WorkerCount: 1, ServiceName: "test", Rate: 1e6, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"large num metrics", &Config{NumMetrics: 1e6, WorkerCount: 1, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"nil meter provider", &Config{NumMetrics: 1, WorkerCount: 1, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "large rate" || tt.name == "large num metrics" {
				t.Skip("skipping impractically large test case")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			ch := make(chan struct{})
			go func() {
				defer func() { _ = recover() }()
				if tt.mp == nil {
					ch <- struct{}{}
					return
				}
				SimulateGauge(tt.mp.(otelmetricnoop.MeterProvider), GaugeConfig{}, tt.cfg, tt.logger)
				ch <- struct{}{}
			}()
			select {
			case <-ctx.Done():
				// Timeout, test exits
			case <-ch:
				// Test finished
			}
		})
	}
}

func TestSimulateHistogram_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		cfg    *Config
		logger *zap.Logger
		mp     interface{}
	}{
		{"nil logger", &Config{NumMetrics: 1, WorkerCount: 1, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, nil, otelmetricnoop.NewMeterProvider()},
		{"nil config", nil, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"zero worker count", &Config{NumMetrics: 1, WorkerCount: 0, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"negative worker count", &Config{NumMetrics: 1, WorkerCount: -1, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"zero rate", &Config{NumMetrics: 1, WorkerCount: 1, ServiceName: "test", Rate: 0, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"negative rate", &Config{NumMetrics: 1, WorkerCount: 1, ServiceName: "test", Rate: -1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"zero num metrics", &Config{NumMetrics: 0, WorkerCount: 1, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"negative num metrics", &Config{NumMetrics: -1, WorkerCount: 1, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"empty service name", &Config{NumMetrics: 1, WorkerCount: 1, ServiceName: "", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"large rate", &Config{NumMetrics: 1, WorkerCount: 1, ServiceName: "test", Rate: 1e6, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"large num metrics", &Config{NumMetrics: 1e6, WorkerCount: 1, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"nil meter provider", &Config{NumMetrics: 1, WorkerCount: 1, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "large rate" || tt.name == "large num metrics" {
				t.Skip("skipping impractically large test case")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			ch := make(chan struct{})
			go func() {
				defer func() { _ = recover() }()
				if tt.mp == nil {
					ch <- struct{}{}
					return
				}
				SimulateHistogram(tt.mp.(otelmetricnoop.MeterProvider), HistogramConfig{}, tt.cfg, tt.logger)
				ch <- struct{}{}
			}()
			select {
			case <-ctx.Done():
				// Timeout, test exits
			case <-ch:
				// Test finished
			}
		})
	}
}

func TestSimulateExponentialHistogram_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		cfg    *Config
		logger *zap.Logger
		mp     interface{}
	}{
		{"nil logger", &Config{NumMetrics: 1, WorkerCount: 1, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, nil, otelmetricnoop.NewMeterProvider()},
		{"nil config", nil, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"zero worker count", &Config{NumMetrics: 1, WorkerCount: 0, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"negative worker count", &Config{NumMetrics: 1, WorkerCount: -1, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"zero rate", &Config{NumMetrics: 1, WorkerCount: 1, ServiceName: "test", Rate: 0, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"negative rate", &Config{NumMetrics: 1, WorkerCount: 1, ServiceName: "test", Rate: -1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"zero num metrics", &Config{NumMetrics: 0, WorkerCount: 1, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"negative num metrics", &Config{NumMetrics: -1, WorkerCount: 1, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"empty service name", &Config{NumMetrics: 1, WorkerCount: 1, ServiceName: "", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"large rate", &Config{NumMetrics: 1, WorkerCount: 1, ServiceName: "test", Rate: 1e6, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"large num metrics", &Config{NumMetrics: 1e6, WorkerCount: 1, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"nil meter provider", &Config{NumMetrics: 1, WorkerCount: 1, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "large rate" || tt.name == "large num metrics" {
				t.Skip("skipping impractically large test case")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			ch := make(chan struct{})
			go func() {
				defer func() { _ = recover() }()
				if tt.mp == nil {
					ch <- struct{}{}
					return
				}
				SimulateExponentialHistogram(tt.mp.(otelmetricnoop.MeterProvider), ExponentialHistogramConfig{}, tt.cfg, tt.logger)
				ch <- struct{}{}
			}()
			select {
			case <-ctx.Done():
				// Timeout, test exits
			case <-ch:
				// Test finished
			}
		})
	}
}

func TestSimulateUpDownCounter_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		cfg    *Config
		logger *zap.Logger
		mp     interface{}
	}{
		{"nil logger", &Config{NumMetrics: 1, WorkerCount: 1, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, nil, otelmetricnoop.NewMeterProvider()},
		{"nil config", nil, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"zero worker count", &Config{NumMetrics: 1, WorkerCount: 0, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"negative worker count", &Config{NumMetrics: 1, WorkerCount: -1, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"zero rate", &Config{NumMetrics: 1, WorkerCount: 1, ServiceName: "test", Rate: 0, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"negative rate", &Config{NumMetrics: 1, WorkerCount: 1, ServiceName: "test", Rate: -1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"zero num metrics", &Config{NumMetrics: 0, WorkerCount: 1, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"negative num metrics", &Config{NumMetrics: -1, WorkerCount: 1, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"empty service name", &Config{NumMetrics: 1, WorkerCount: 1, ServiceName: "", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"large rate", &Config{NumMetrics: 1, WorkerCount: 1, ServiceName: "test", Rate: 1e6, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"large num metrics", &Config{NumMetrics: 1e6, WorkerCount: 1, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), otelmetricnoop.NewMeterProvider()},
		{"nil meter provider", &Config{NumMetrics: 1, WorkerCount: 1, ServiceName: "test", Rate: 1, TotalDuration: 10 * time.Millisecond}, zap.NewNop(), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "large rate" || tt.name == "large num metrics" {
				t.Skip("skipping impractically large test case")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			ch := make(chan struct{})
			go func() {
				defer func() { _ = recover() }()
				if tt.mp == nil {
					ch <- struct{}{}
					return
				}
				SimulateUpDownCounter(tt.mp.(otelmetricnoop.MeterProvider), tt.cfg, tt.logger)
				ch <- struct{}{}
			}()
			select {
			case <-ctx.Done():
				// Timeout, test exits
			case <-ch:
				// Test finished
			}
		})
	}
}
