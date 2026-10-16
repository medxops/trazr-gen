// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"github.com/medxops/trazr-gen/internal/common"
)

type worker struct {
	running                *atomic.Bool                 // pointer to shared flag that indicates it's time to stop the test
	metricName             string                       // name of metric to generate
	metricType             MetricType                   // type of metric to generate
	aggregationTemporality AggregationTemporality       // Temporality type to use
	exemplars              []metricdata.Exemplar[int64] // exemplars to attach to the metric
	numMetrics             int                          // how many metrics the worker has to generate (only when duration==0)
	totalDuration          time.Duration                // how long to run the test for (overrides `numMetrics`)
	limitPerSecond         rate.Limit                   // how many metrics per second to generate
	wg                     *sync.WaitGroup              // notify when done
	logger                 *zap.Logger                  // logger
	index                  int                          // worker index
	clock                  Clock                        // clock
	metricsCounter         *int64                       // pointer to shared metrics counter
	progressCb             func(string)                 // optional callback for terminal output
	progressCh             chan struct{}                // channel for centralized progress reporting
}

// We use a 15-element bounds slice for histograms below, so there must be 16 buckets here.
// From metrics.proto:
// The number of elements in bucket_counts array must be by one greater than
// the number of elements in explicit_bounds array.
var histogramBucketSamples = []struct {
	bucketCounts []uint64
	sum          int64
}{
	{
		[]uint64{0, 0, 1, 0, 0, 0, 3, 4, 1, 1, 0, 0, 0, 0, 0, 0},
		3940,
	},
	{
		[]uint64{0, 0, 0, 0, 0, 0, 2, 4, 4, 0, 0, 0, 0, 0, 0, 0},
		4455,
	},
	{
		[]uint64{0, 0, 0, 0, 0, 0, 1, 4, 3, 2, 0, 0, 0, 0, 0, 0},
		5337,
	},
	{
		[]uint64{0, 0, 1, 0, 1, 0, 2, 2, 1, 3, 0, 0, 0, 0, 0, 0},
		4477,
	},
	{
		[]uint64{0, 0, 0, 0, 0, 1, 3, 2, 2, 2, 0, 0, 0, 0, 0, 0},
		4670,
	},
	{
		[]uint64{0, 0, 0, 1, 1, 0, 1, 1, 1, 5, 0, 0, 0, 0, 0, 0},
		5670,
	},
	{
		[]uint64{0, 0, 0, 0, 0, 2, 1, 1, 4, 2, 0, 0, 0, 0, 0, 0},
		5091,
	},
	{
		[]uint64{0, 0, 2, 0, 0, 0, 2, 4, 1, 1, 0, 0, 0, 0, 0, 0},
		3420,
	},
	{
		[]uint64{0, 0, 0, 0, 0, 0, 1, 3, 2, 4, 0, 0, 0, 0, 0, 0},
		5917,
	},
	{
		[]uint64{0, 0, 1, 0, 1, 0, 0, 4, 4, 0, 0, 0, 0, 0, 0, 0},
		3988,
	},
}

func (w worker) reportProgressf(format string, args ...any) {
	if w.progressCb != nil {
		w.progressCb(fmt.Sprintf(format, args...))
	}
}

func (w worker) simulateMetrics(ctx context.Context, res *resource.Resource, exporter sdkmetric.Exporter, cfg *Config) {
	limiter := rate.NewLimiter(w.limitPerSecond, 1)

	startTime := w.clock.Now()

	var i int64
	for w.running.Load() && ctx.Err() == nil {
		// Rate limit at the top of the loop so output pacing is visible
		if err := limiter.Wait(ctx); err != nil {
			if ctx.Err() != nil {
				w.logger.Info("context cancelled during rate limiting, stopping")
				break
			}
			w.reportProgressf("Limiter wait failed: %v", err)
			w.logger.Fatal("limiter wait failed, retry", zap.Error(err))
		}

		var metrics []metricdata.Metrics
		now := w.clock.Now()
		if w.aggregationTemporality.AsTemporality() == metricdata.DeltaTemporality {
			startTime = now.Add(-1 * time.Second)
		}

		// Build a fresh set of signal attributes for each metric data point
		signalAttrs, err := cfg.GetTelemetryAttrWithMockMarker()
		if err != nil {
			w.reportProgressf("Failed to process telemetry attributes: %v", err)
			w.logger.Fatal("failed to process telemetry attributes", zap.Error(err))
		}

		switch w.metricType {
		case MetricTypeGauge:
			metrics = append(metrics, metricdata.Metrics{
				Name: w.metricName,
				Data: metricdata.Gauge[int64]{
					DataPoints: []metricdata.DataPoint[int64]{
						{
							Time:       now,
							Value:      i,
							Attributes: attribute.NewSet(signalAttrs...),
							Exemplars:  w.exemplars,
						},
					},
				},
			})
		case MetricTypeSum:
			metrics = append(metrics, metricdata.Metrics{
				Name: w.metricName,
				Data: metricdata.Sum[int64]{
					IsMonotonic: true,
					Temporality: w.aggregationTemporality.AsTemporality(),
					DataPoints: []metricdata.DataPoint[int64]{
						{
							StartTime:  startTime,
							Time:       now,
							Value:      i,
							Attributes: attribute.NewSet(signalAttrs...),
							Exemplars:  w.exemplars,
						},
					},
				},
			})
		case MetricTypeHistogram:
			var totalCount uint64
			idx := int(i % 10)
			if idx < 0 {
				idx += 10
			}
			iteration := uint64(idx)
			sum := histogramBucketSamples[iteration].sum
			bucketCounts := histogramBucketSamples[iteration].bucketCounts
			for _, count := range bucketCounts {
				totalCount += count
			}
			metrics = append(metrics, metricdata.Metrics{
				Name: w.metricName,
				Data: metricdata.Histogram[int64]{
					Temporality: w.aggregationTemporality.AsTemporality(),
					DataPoints: []metricdata.HistogramDataPoint[int64]{
						{
							StartTime:  startTime,
							Time:       now,
							Attributes: attribute.NewSet(signalAttrs...),
							Exemplars:  w.exemplars,
							Count:      totalCount,
							Sum:        sum,
							// Bounds from https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/metrics/sdk.md#explicit-bucket-histogram-aggregation
							Bounds:       []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
							BucketCounts: bucketCounts,
						},
					},
				},
			})
		default:
			w.logger.Fatal("unknown metric type")
		}

		rm := metricdata.ResourceMetrics{
			Resource:     res,
			ScopeMetrics: []metricdata.ScopeMetrics{{Metrics: metrics}},
		}

		if err := exporter.Export(ctx, &rm); err != nil {
			if ctx.Err() != nil {
				w.logger.Info("context cancelled during export, stopping")
				break
			}
			w.reportProgressf("Exporter failed, will retry in 5s: %v", err)
			w.logger.Warn("exporter failed, will retry in 5s", zap.Error(err))
			// Use context-aware sleep
			select {
			case <-time.After(5 * time.Second):
			case <-ctx.Done():
				w.logger.Info("context cancelled during retry sleep, stopping")
				break
			}
			continue
		}

		if cfg.TerminalOutput && cfg.PrintDetails {
			for _, m := range metrics {
				fmt.Println("Generated Metric Record:")
				fmt.Printf("  Name: %s\n", m.Name)
				switch data := m.Data.(type) {
				case metricdata.Gauge[int64]:
					for _, dp := range data.DataPoints {
						fmt.Printf("  Type: Gauge\n")
						fmt.Printf("  Value: %v\n", dp.Value)
						fmt.Printf("  Time: %s\n", dp.Time.Format(time.RFC3339Nano))
						fmt.Println("  Attributes:")
						for i := 0; i < dp.Attributes.Len(); i++ {
							kv := dp.Attributes.ToSlice()[i]
							key := string(kv.Key)
							val := common.FormatAttrValue(kv.Value.AsInterface())
							fmt.Printf("    - %s: %s\n", key, common.FormatSensitiveValue(key, val))
						}
						if len(dp.Exemplars) > 0 {
							ex := dp.Exemplars[0]
							traceID := "-"
							if len(ex.TraceID) == 16 {
								traceID = hex.EncodeToString(ex.TraceID)
							}
							spanID := "-"
							if len(ex.SpanID) == 8 {
								spanID = hex.EncodeToString(ex.SpanID)
							}
							fmt.Printf("  TraceID: %s\n", traceID)
							fmt.Printf("  SpanID: %s\n", spanID)
						}
					}
				case metricdata.Sum[int64]:
					for _, dp := range data.DataPoints {
						fmt.Printf("  Type: Sum\n")
						fmt.Printf("  Value: %v\n", dp.Value)
						fmt.Printf("  StartTime: %s\n", dp.StartTime.Format(time.RFC3339Nano))
						fmt.Printf("  Time: %s\n", dp.Time.Format(time.RFC3339Nano))
						fmt.Println("  Attributes:")
						for i := 0; i < dp.Attributes.Len(); i++ {
							kv := dp.Attributes.ToSlice()[i]
							key := string(kv.Key)
							val := common.FormatAttrValue(kv.Value.AsInterface())
							fmt.Printf("    - %s: %s\n", key, common.FormatSensitiveValue(key, val))
						}
						if len(dp.Exemplars) > 0 {
							ex := dp.Exemplars[0]
							traceID := "-"
							if len(ex.TraceID) == 16 {
								traceID = hex.EncodeToString(ex.TraceID)
							}
							spanID := "-"
							if len(ex.SpanID) == 8 {
								spanID = hex.EncodeToString(ex.SpanID)
							}
							fmt.Printf("  TraceID: %s\n", traceID)
							fmt.Printf("  SpanID: %s\n", spanID)
						}
					}
				case metricdata.Histogram[int64]:
					for _, dp := range data.DataPoints {
						fmt.Printf("  Type: Histogram\n")
						fmt.Printf("  Count: %v\n", dp.Count)
						fmt.Printf("  Sum: %v\n", dp.Sum)
						fmt.Printf("  StartTime: %s\n", dp.StartTime.Format(time.RFC3339Nano))
						fmt.Printf("  Time: %s\n", dp.Time.Format(time.RFC3339Nano))
						fmt.Println("  Attributes:")
						for i := 0; i < dp.Attributes.Len(); i++ {
							kv := dp.Attributes.ToSlice()[i]
							key := string(kv.Key)
							val := common.FormatAttrValue(kv.Value.AsInterface())
							fmt.Printf("    - %s: %s\n", key, common.FormatSensitiveValue(key, val))
						}
						if len(dp.Exemplars) > 0 {
							ex := dp.Exemplars[0]
							traceID := "-"
							if len(ex.TraceID) == 16 {
								traceID = hex.EncodeToString(ex.TraceID)
							}
							spanID := "-"
							if len(ex.SpanID) == 8 {
								spanID = hex.EncodeToString(ex.SpanID)
							}
							fmt.Printf("  TraceID: %s\n", traceID)
							fmt.Printf("  SpanID: %s\n", spanID)
						}
					}
				}
				fmt.Println("  Resource Attributes:")
				resAttrs, _ := cfg.GetResourceAttrWithMockMarker()
				for _, attr := range resAttrs {
					key := string(attr.Key)
					val := common.FormatAttrValue(attr.Value.AsInterface())
					fmt.Printf("    - %s: %s\n", key, common.FormatSensitiveValue(key, val))
				}
				fmt.Println("--------------------------------------------------")
			}
		}

		i++
		if w.metricsCounter != nil {
			atomic.AddInt64(w.metricsCounter, 1)
		}
		if w.progressCh != nil {
			select {
			case w.progressCh <- struct{}{}:
			case <-ctx.Done():
				w.logger.Info("context cancelled during progress reporting, stopping")
				break
			}
		}
		if w.numMetrics != 0 && i >= int64(w.numMetrics) {
			break
		}
	}

	w.logger.Info("metrics generated", zap.Int64("metrics", i))
	w.wg.Done()
}
