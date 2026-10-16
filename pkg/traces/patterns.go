// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces

import (
	"fmt"
	"math/rand/v2"
	"strconv"

	"go.opentelemetry.io/otel/trace"
)

// SimplePattern generates a simple linear chain of spans
type SimplePattern struct{}

func (p *SimplePattern) GenerateSpans(config *Config, _ trace.TraceID, services []string) []*Span {
	spans := make([]*Span, 0, config.SpansPerTrace)

	// Use default service if services list is empty
	if len(services) == 0 {
		services = []string{"default-service"}
	}

	for i := 0; i < config.SpansPerTrace; i++ {
		service := services[i%len(services)]
		// Ensure depth is always less than MaxDepth
		depth := i % config.MaxDepth

		span := &Span{
			ID:       fmt.Sprintf("span-%d", i),
			Name:     service + "-operation",
			Service:  service,
			Depth:    depth,
			Children: make([]*Span, 0),
		}

		// Only set parent for non-root spans (depth > 0)
		if depth > 0 {
			span.ParentID = fmt.Sprintf("span-%d", i-1)
		}

		spans = append(spans, span)
	}

	return spans
}

// ChainPattern generates a chain of spans with some branching
type ChainPattern struct{}

func (p *ChainPattern) GenerateSpans(config *Config, _ trace.TraceID, services []string) []*Span {
	spans := make([]*Span, 0, config.SpansPerTrace)

	// Use default service if services list is empty
	if len(services) == 0 {
		services = []string{"default-service"}
	}

	// Create root span
	rootSpan := &Span{
		ID:       "span-0",
		Name:     services[0] + "-request",
		Service:  services[0],
		Depth:    0,
		Children: make([]*Span, 0),
	}
	spans = append(spans, rootSpan)

	// Create chain with some branching
	currentDepth := 1
	spanIndex := 1

	for spanIndex < config.SpansPerTrace {
		// Determine how many children for current level
		maxChildren := minInt(config.Concurrency, config.SpansPerTrace-spanIndex)
		if maxChildren <= 0 {
			break
		}

		// Find parent spans at current depth - 1
		parentSpans := findSpansAtDepth(spans, currentDepth-1)

		for _, parent := range parentSpans {
			if spanIndex >= config.SpansPerTrace {
				break
			}

			// Create 1-3 children per parent
			childrenCount := minInt(randInt(1, 4), maxChildren)
			for j := 0; j < childrenCount && spanIndex < config.SpansPerTrace; j++ {
				service := services[spanIndex%len(services)]
				childSpan := &Span{
					ID:       fmt.Sprintf("span-%d", spanIndex),
					Name:     service + "-operation",
					Service:  service,
					ParentID: parent.ID,
					Depth:    currentDepth,
					Children: make([]*Span, 0),
				}

				parent.Children = append(parent.Children, childSpan)
				spans = append(spans, childSpan)
				spanIndex++
			}
		}

		currentDepth++
		if currentDepth >= config.MaxDepth {
			break
		}
	}

	return spans
}

// FanOutPattern generates a fan-out pattern with parallel operations
type FanOutPattern struct{}

func (p *FanOutPattern) GenerateSpans(config *Config, _ trace.TraceID, services []string) []*Span {
	spans := make([]*Span, 0, config.SpansPerTrace)

	// Use default service if services list is empty
	if len(services) == 0 {
		services = []string{"default-service"}
	}

	// Create root span
	rootSpan := &Span{
		ID:       "span-0",
		Name:     services[0] + "-request",
		Service:  services[0],
		Depth:    0,
		Children: make([]*Span, 0),
	}
	spans = append(spans, rootSpan)

	// Create fan-out of parallel operations
	parallelSpans := minInt(config.Concurrency, config.SpansPerTrace-1)
	for i := 1; i <= parallelSpans; i++ {
		service := services[i%len(services)]
		childSpan := &Span{
			ID:       fmt.Sprintf("span-%d", i),
			Name:     service + "-parallel",
			Service:  service,
			ParentID: rootSpan.ID,
			Depth:    1,
			Children: make([]*Span, 0),
		}

		rootSpan.Children = append(rootSpan.Children, childSpan)
		spans = append(spans, childSpan)
	}

	// Add some nested spans if we have room
	spanIndex := parallelSpans + 1
	for spanIndex < config.SpansPerTrace {
		// Find a random parent at depth 1
		parentSpans := findSpansAtDepth(spans, 1)
		if len(parentSpans) == 0 {
			break
		}

		parent := parentSpans[spanIndex%len(parentSpans)]
		service := services[spanIndex%len(services)]
		childSpan := &Span{
			ID:       fmt.Sprintf("span-%d", spanIndex),
			Name:     service + "-nested",
			Service:  service,
			ParentID: parent.ID,
			Depth:    2,
			Children: make([]*Span, 0),
		}

		parent.Children = append(parent.Children, childSpan)
		spans = append(spans, childSpan)
		spanIndex++
	}

	return spans
}

// ComplexPattern generates a complex pattern with multiple levels and branching
type ComplexPattern struct{}

func (p *ComplexPattern) GenerateSpans(config *Config, _ trace.TraceID, services []string) []*Span {
	spans := make([]*Span, 0, config.SpansPerTrace)

	// Use default service if services list is empty
	if len(services) == 0 {
		services = []string{"default-service"}
	}

	// Create root span
	rootSpan := &Span{
		ID:       "span-0",
		Name:     services[0] + "-request",
		Service:  services[0],
		Depth:    0,
		Children: make([]*Span, 0),
	}
	spans = append(spans, rootSpan)

	spanIndex := 1

	// Create complex structure with multiple levels
	for depth := 1; depth < config.MaxDepth && spanIndex < config.SpansPerTrace; depth++ {
		parentSpans := findSpansAtDepth(spans, depth-1)

		for _, parent := range parentSpans {
			if spanIndex >= config.SpansPerTrace {
				break
			}

			// Create 1-2 children per parent for complex structure
			childrenCount := minInt(randInt(1, 3), config.SpansPerTrace-spanIndex)
			for j := 0; j < childrenCount && spanIndex < config.SpansPerTrace; j++ {
				service := services[spanIndex%len(services)]
				childSpan := &Span{
					ID:       fmt.Sprintf("span-%d", spanIndex),
					Name:     service + "-level-" + strconv.Itoa(depth),
					Service:  service,
					ParentID: parent.ID,
					Depth:    depth,
					Children: make([]*Span, 0),
				}

				parent.Children = append(parent.Children, childSpan)
				spans = append(spans, childSpan)
				spanIndex++
			}
		}
	}

	return spans
}

// RandomPattern generates a random pattern (delegates to other patterns)
type RandomPattern struct{}

func (p *RandomPattern) GenerateSpans(config *Config, traceID trace.TraceID, services []string) []*Span {
	patterns := []Pattern{
		&SimplePattern{},
		&ChainPattern{},
		&FanOutPattern{},
		&ComplexPattern{},
	}

	selectedPattern := patterns[randInt(0, len(patterns)-1)]
	return selectedPattern.GenerateSpans(config, traceID, services)
}

// Helper functions

// findSpansAtDepth returns all spans at a given depth
func findSpansAtDepth(spans []*Span, depth int) []*Span {
	var result []*Span
	for _, span := range spans {
		if span.Depth == depth {
			result = append(result, span)
		}
	}
	return result
}

// randInt returns a random integer between min and max (exclusive)
func randInt(minVal, maxVal int) int {
	return rand.IntN(maxVal-minVal+1) + minVal
}

// min returns the minimum of two integers
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
