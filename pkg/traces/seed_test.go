package traces

import (
	"context"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/medxops/trazr-gen/internal/common"
)

func TestSeedMechanismBasic(t *testing.T) {
	// Test that the seed mechanism works for basic random generation
	seed := int64(12345)

	// Create seeded random sources
	rng1 := rand.New(rand.NewPCG(safeUint64FromInt64(seed), 0))
	rng2 := rand.New(rand.NewPCG(safeUint64FromInt64(seed), 0))

	// Generate some random numbers
	rand1 := make([]int, 5)
	for i := 0; i < 5; i++ {
		rand1[i] = rng1.IntN(1000)
	}

	// Generate again with the same seed
	rand2 := make([]int, 5)
	for i := 0; i < 5; i++ {
		rand2[i] = rng2.IntN(1000)
	}

	// Verify they are identical
	assert.Equal(t, rand1, rand2, "Random numbers should be identical with same seed")

	// Test with different seed
	rng3 := rand.New(rand.NewPCG(safeUint64FromInt64(seed+1), 0))
	rand3 := make([]int, 5)
	for i := 0; i < 5; i++ {
		rand3[i] = rng3.IntN(1000)
	}

	// Verify they are different
	assert.NotEqual(t, rand1, rand3, "Random numbers should be different with different seed")
}

func TestReproducibleTraceGeneration(t *testing.T) {
	// Test that using the same seed produces identical traces
	seed := int64(12345)

	// Create config for enhanced trace generation
	config := &Config{
		Config: common.Config{
			WorkerCount: 1,
		},
		Enhanced:          true,
		SpansPerTrace:     3,
		MaxDepth:          2,
		Pattern:           "simple", // Use specific pattern instead of random
		ErrorRate:         0.0,
		EventsPerSpan:     2,
		AttributesPerSpan: 5,
		Services:          []string{"test-service"},
	}

	// Generate first trace
	trace1, err := generateTraceWithSeed(config, seed)
	require.NoError(t, err)
	assert.NotNil(t, trace1)

	// Generate second trace with same seed
	trace2, err := generateTraceWithSeed(config, seed)
	require.NoError(t, err)
	assert.NotNil(t, trace2)

	// Verify traces are identical
	assert.Len(t, trace1.Spans, len(trace2.Spans), "Should have same number of spans")

	// Compare span details
	for i := 0; i < len(trace1.Spans); i++ {
		span1 := trace1.Spans[i]
		span2 := trace2.Spans[i]

		assert.Equal(t, span1.Name, span2.Name, "Span names should be identical")
		assert.Equal(t, span1.Service, span2.Service, "Span services should be identical")
		assert.Equal(t, span1.Depth, span2.Depth, "Span depths should be identical")
		assert.Equal(t, span1.ParentID, span2.ParentID, "Span parent IDs should be identical")
	}
}

func TestDifferentSeedsProduceDifferentTraces(t *testing.T) {
	// Test that different seeds produce different traces
	config := &Config{
		Config: common.Config{
			WorkerCount: 1,
		},
		Enhanced:          true,
		SpansPerTrace:     2,
		MaxDepth:          2,
		Pattern:           "simple",
		ErrorRate:         0.0,
		EventsPerSpan:     1,
		AttributesPerSpan: 3,
		Services:          []string{"test-service"},
	}

	// Generate first trace with seed 1
	trace1, err := generateTraceWithSeed(config, 1)
	require.NoError(t, err)

	// Generate second trace with seed 2
	trace2, err := generateTraceWithSeed(config, 2)
	require.NoError(t, err)

	// Debug output
	t.Logf("Trace 1 spans: %d", len(trace1.Spans))
	for i, span := range trace1.Spans {
		t.Logf("  Span %d: %s (%s) - Attributes: %d, Events: %d", i, span.Name, span.Service, len(span.Attributes), len(span.Events))
	}

	t.Logf("Trace 2 spans: %d", len(trace2.Spans))
	for i, span := range trace2.Spans {
		t.Logf("  Span %d: %s (%s) - Attributes: %d, Events: %d", i, span.Name, span.Service, len(span.Attributes), len(span.Events))
	}

	// Verify traces are different (check attributes and events)
	different := false
	for i := 0; i < len(trace1.Spans) && i < len(trace2.Spans); i++ {
		span1 := trace1.Spans[i]
		span2 := trace2.Spans[i]

		// Check if attributes are different
		if len(span1.Attributes) != len(span2.Attributes) {
			different = true
			break
		}

		// Check if events are different
		if len(span1.Events) != len(span2.Events) {
			different = true
			break
		}

		// Check if any attribute values are different
		for key, value1 := range span1.Attributes {
			if value2, exists := span2.Attributes[key]; !exists || value1 != value2 {
				different = true
				break
			}
		}

		if different {
			break
		}
	}

	assert.True(t, different, "Traces with different seeds should be different")
}

func TestMockDataWithSeed(t *testing.T) {
	// Test that mock data is also reproducible with the same seed
	seed := int64(54321)

	config := &Config{
		Config: common.Config{
			WorkerCount: 1,
			MockData:    true,
		},
		Enhanced:          true,
		SpansPerTrace:     2,
		MaxDepth:          2,
		Pattern:           "simple",
		ErrorRate:         0.0,
		EventsPerSpan:     1,
		AttributesPerSpan: 3,
		Services:          []string{"test-service"},
	}

	// Generate first trace with mock data
	trace1, err := generateTraceWithSeed(config, seed)
	require.NoError(t, err)

	// Generate second trace with same seed
	trace2, err := generateTraceWithSeed(config, seed)
	require.NoError(t, err)

	// Verify mock data attributes are identical
	for i := 0; i < len(trace1.Spans) && i < len(trace2.Spans); i++ {
		span1 := trace1.Spans[i]
		span2 := trace2.Spans[i]

		// Check that mock data attributes are present and identical
		for key, value1 := range span1.Attributes {
			if value2, exists := span2.Attributes[key]; exists {
				assert.Equal(t, value1, value2, "Mock data attributes should be identical")
			}
		}
	}
}

// Helper function to generate a trace with a specific seed
func generateTraceWithSeed(config *Config, seed int64) (*Trace, error) {
	// Set the seed in the config
	config.Seed = seed

	// Initialize generators
	logger := zap.NewNop()
	traceGenerator := NewTraceGenerator(config, logger)

	// Generate trace using the full process
	trace, err := traceGenerator.GenerateTrace(context.TODO())
	if err != nil {
		return nil, err
	}

	return trace, nil
}
