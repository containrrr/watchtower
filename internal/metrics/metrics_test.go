package metrics

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dto "github.com/prometheus/client_model/go"

	"github.com/nicholas-fedor/watchtower/pkg/types"
	mockTypes "github.com/nicholas-fedor/watchtower/pkg/types/mocks"
)

func TestNewMetric(t *testing.T) {
	type args struct {
		report types.Report
	}

	tests := []struct {
		name string
		args args
		want *Metric
	}{
		{
			name: "empty report",
			args: args{
				report: func() types.Report {
					mock := mockTypes.NewMockReport(t)
					mock.EXPECT().Scanned().Return([]types.ContainerReport{})
					mock.EXPECT().Updated().Return([]types.ContainerReport{})
					mock.EXPECT().Failed().Return([]types.ContainerReport{})
					mock.EXPECT().Restarted().Return([]types.ContainerReport{})
					mock.EXPECT().Skipped().Return([]types.ContainerReport{})

					return mock
				}(),
			},
			want: &Metric{
				Scanned:   0,
				Updated:   0,
				Failed:    0,
				Restarted: 0,
				Skipped:   0,
			},
		},
		{
			name: "mixed report",
			args: args{
				report: func() types.Report {
					mock := mockTypes.NewMockReport(t)
					mock.EXPECT().Scanned().Return(make([]types.ContainerReport, 3))
					mock.EXPECT().Updated().Return(make([]types.ContainerReport, 1))
					mock.EXPECT().Failed().Return(make([]types.ContainerReport, 1))
					mock.EXPECT().Restarted().Return(make([]types.ContainerReport, 2))
					mock.EXPECT().Skipped().Return(make([]types.ContainerReport, 1))

					return mock
				}(),
			},
			want: &Metric{
				Scanned:   3,
				Updated:   1, // Only count actually updated containers
				Failed:    1,
				Restarted: 2,
				Skipped:   1,
			},
		},
		{
			name: "only restarted containers",
			args: args{
				report: func() types.Report {
					mock := mockTypes.NewMockReport(t)
					mock.EXPECT().Scanned().Return(make([]types.ContainerReport, 5))
					mock.EXPECT().Updated().Return([]types.ContainerReport{})
					mock.EXPECT().Failed().Return([]types.ContainerReport{})
					mock.EXPECT().Restarted().Return(make([]types.ContainerReport, 5))
					mock.EXPECT().Skipped().Return([]types.ContainerReport{})

					return mock
				}(),
			},
			want: &Metric{
				Scanned:   5,
				Updated:   0,
				Failed:    0,
				Restarted: 5,
				Skipped:   0,
			},
		},
		{
			name: "restarted with failures",
			args: args{
				report: func() types.Report {
					mock := mockTypes.NewMockReport(t)
					mock.EXPECT().Scanned().Return(make([]types.ContainerReport, 10))
					mock.EXPECT().Updated().Return(make([]types.ContainerReport, 3))
					mock.EXPECT().Failed().Return(make([]types.ContainerReport, 2))
					mock.EXPECT().Restarted().Return(make([]types.ContainerReport, 4))
					mock.EXPECT().Skipped().Return(make([]types.ContainerReport, 2))

					return mock
				}(),
			},
			want: &Metric{
				Scanned:   10,
				Updated:   3,
				Failed:    2,
				Restarted: 4,
				Skipped:   2,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewMetric(tt.args.report); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewMetric() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewMetric_ErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		report      types.Report
		want        *Metric
		shouldPanic bool
	}{
		{
			name: "nil report",
			report: func() types.Report {
				return nil
			}(),
			want:        nil,
			shouldPanic: true,
		},
		{
			name: "report with nil slices",
			report: func() types.Report {
				mock := mockTypes.NewMockReport(t)
				mock.EXPECT().Scanned().Return(nil)
				mock.EXPECT().Updated().Return(nil)
				mock.EXPECT().Failed().Return(nil)
				mock.EXPECT().Restarted().Return(nil)
				mock.EXPECT().Skipped().Return(nil)

				return mock
			}(),
			want: &Metric{
				Scanned:   0,
				Updated:   0,
				Failed:    0,
				Restarted: 0,
				Skipped:   0,
			},
			shouldPanic: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.shouldPanic {
				defer func() {
					if r := recover(); r == nil {
						t.Errorf("NewMetric() expected panic but didn't get one")
					}
				}()

				NewMetric(tt.report)
			} else {
				got := NewMetric(tt.report)
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("NewMetric() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestMetrics_StateTransitions(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		registry := prometheus.NewRegistry()

		m, err := NewWithRegistry(registry)
		if err != nil {
			t.Fatalf("Failed to create metrics: %v", err)
		}

		t.Cleanup(func() { m.Shutdown() })

		tests := []struct {
			name           string
			initialMetric  *Metric
			expectedValues map[string]float64
		}{
			{
				name: "initial state",
				initialMetric: &Metric{
					Scanned:   0,
					Updated:   0,
					Failed:    0,
					Restarted: 0,
				},
				expectedValues: map[string]float64{
					"watchtower_containers_scanned":   0,
					"watchtower_containers_updated":   0,
					"watchtower_containers_failed":    0,
					"watchtower_containers_restarted": 0,
				},
			},
			{
				name: "after first scan with restarted",
				initialMetric: &Metric{
					Scanned:   5,
					Updated:   2,
					Failed:    1,
					Restarted: 3,
				},
				expectedValues: map[string]float64{
					"watchtower_containers_scanned":   5,
					"watchtower_containers_updated":   2,
					"watchtower_containers_failed":    1,
					"watchtower_containers_restarted": 3,
				},
			},
			{
				name: "after second scan with more restarted",
				initialMetric: &Metric{
					Scanned:   8,
					Updated:   3,
					Failed:    0,
					Restarted: 5,
				},
				expectedValues: map[string]float64{
					"watchtower_containers_scanned":   8,
					"watchtower_containers_updated":   3,
					"watchtower_containers_failed":    0,
					"watchtower_containers_restarted": 5,
				},
			},
		}

		for _, tt := range tests {
			m.Register(tt.initialMetric)

			// Wait for the metric to be processed
			synctest.Wait()

			// Gather metrics and verify
			metricFamilies, err := registry.Gather()
			if err != nil {
				t.Fatalf("Failed to gather metrics: %v", err)
			}

			for _, mf := range metricFamilies {
				if expectedValue, exists := tt.expectedValues[mf.GetName()]; exists {
					if len(mf.GetMetric()) == 0 {
						t.Errorf("No metrics found for %s", mf.GetName())

						continue
					}

					actualValue := mf.GetMetric()[0].GetGauge().GetValue()
					if actualValue != expectedValue {
						t.Errorf(
							"Metric %s = %v, want %v",
							mf.GetName(),
							actualValue,
							expectedValue,
						)
					}
				}
			}
		}

		// Shutdown the handler
		m.Shutdown()

		// Wait for shutdown
		synctest.Wait()

		// Assert metrics stopped: register a metric after shutdown and verify gauges don't change
		// The last registered metric was {Scanned: 8, Updated: 3, Failed: 0, Restarted: 5}
		expectedAfterShutdown := map[string]float64{
			"watchtower_containers_scanned":   8,
			"watchtower_containers_updated":   3,
			"watchtower_containers_failed":    0,
			"watchtower_containers_restarted": 5,
		}
		assertMetricsStoppedAfterShutdown(t, m, registry, expectedAfterShutdown)
	})
}

func TestMetrics_Register(t *testing.T) {
	type args struct {
		metric *Metric
	}

	tests := []struct {
		name string
		m    *Metrics
		args args
	}{
		{
			name: "register metric",
			m:    &Metrics{channel: make(chan *Metric, 10)},
			args: args{
				metric: &Metric{Scanned: 1, Updated: 2, Failed: 0, Restarted: 0},
			},
		},
		{
			name: "register nil metric",
			m:    &Metrics{channel: make(chan *Metric, 10)},
			args: args{
				metric: nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.m.Register(tt.args.metric)

			select {
			case got := <-tt.m.channel:
				if !reflect.DeepEqual(got, tt.args.metric) {
					t.Errorf("Metrics.Register() enqueued %v, want %v", got, tt.args.metric)
				}
			default:
				t.Errorf("Metrics.Register() did not enqueue metric")
			}
		})
	}
}

func TestMetrics_PriorityOrdering(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		registry := prometheus.NewRegistry()

		m, err := NewWithRegistry(registry)
		if err != nil {
			t.Fatalf("Failed to create metrics: %v", err)
		}

		t.Cleanup(func() { m.Shutdown() })

		// Test that restarted metrics are aggregated correctly with other metrics
		metrics := []*Metric{
			{Scanned: 2, Updated: 1, Failed: 0, Restarted: 1},
			{Scanned: 3, Updated: 0, Failed: 1, Restarted: 2},
			{Scanned: 1, Updated: 1, Failed: 0, Restarted: 0},
		}

		// Register metrics in sequence
		for _, metric := range metrics {
			m.Register(metric)
			synctest.Wait() // Wait for processing
		}

		// Wait for all processing
		synctest.Wait()

		// Gather final metrics
		metricFamilies, err := registry.Gather()
		if err != nil {
			t.Fatalf("Failed to gather metrics: %v", err)
		}

		// Verify aggregated values
		expectedTotals := map[string]float64{
			"watchtower_containers_scanned":   1, // Most recent gauge value
			"watchtower_containers_updated":   1, // Most recent gauge value
			"watchtower_containers_failed":    0, // Most recent gauge value
			"watchtower_containers_restarted": 0, // Most recent gauge value
		}

		for _, mf := range metricFamilies {
			if expectedValue, exists := expectedTotals[mf.GetName()]; exists {
				if len(mf.GetMetric()) == 0 {
					t.Errorf("No metrics found for %s", mf.GetName())

					continue
				}

				actualValue := mf.GetMetric()[0].GetGauge().GetValue()
				if actualValue != expectedValue {
					t.Errorf(
						"Final aggregated metric %s = %v, want %v",
						mf.GetName(),
						actualValue,
						expectedValue,
					)
				}
			}
		}

		// Shutdown
		m.Shutdown()

		// Wait for shutdown
		synctest.Wait()

		// Assert metrics stopped: register a metric after shutdown and verify gauges don't change
		// The last registered metric was {Scanned: 1, Updated: 1, Failed: 0, Restarted: 0}
		expectedAfterShutdown := map[string]float64{
			"watchtower_containers_scanned":   1,
			"watchtower_containers_updated":   1,
			"watchtower_containers_failed":    0,
			"watchtower_containers_restarted": 0,
		}
		assertMetricsStoppedAfterShutdown(t, m, registry, expectedAfterShutdown)
	})
}

func TestDefault(t *testing.T) {
	// Default registers its collectors with the process-wide Prometheus
	// registry, so it initializes once per process. The test neither resets nor
	// shuts it down, which keeps it valid when the test runs more than once.
	got := Default()

	if again := Default(); again != got {
		t.Errorf("Default() returned %p, then %p; want the same instance", got, again)
	}

	tests := []struct {
		name string
	}{
		{name: "new metrics instance"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Default() returned: %+v", got)

			if got == nil {
				t.Fatalf("Default() returned nil pointer")
			}

			if got.channel == nil {
				t.Errorf("Default().channel is nil")
			} else if cap(got.channel) != 10 {
				t.Errorf("Default() channel capacity = %d, want 10", cap(got.channel))
			}

			if got.scanned == nil {
				t.Errorf("Default().scanned is nil")
			}

			if got.updated == nil {
				t.Errorf("Default().updated is nil")
			}

			if got.failed == nil {
				t.Errorf("Default().failed is nil")
			}

			if got.restarted == nil {
				t.Errorf("Default().restarted is nil")
			}

			if got.skipped == nil {
				t.Errorf("Default().skipped is nil")
			}

			if got.total == nil {
				t.Errorf("Default().total is nil")
			}

			if got.skippedScans == nil {
				t.Errorf("Default().skippedScans is nil")
			}

			if got.dropped == nil {
				t.Errorf("Default().dropped is nil")
			}

			if got.imageBytes == nil {
				t.Errorf("Default().imageBytes is nil")
			}

			if got.imageReclaimable == nil {
				t.Errorf("Default().imageReclaimable is nil")
			}

			if got.diskSpaceMaxBytes == nil {
				t.Errorf("Default().diskSpaceMaxBytes is nil")
			}

			if got.diskSpaceWarnBytes == nil {
				t.Errorf("Default().diskSpaceWarnBytes is nil")
			}

			gotAgain := Default()
			if got != gotAgain {
				t.Errorf("Default() did not return singleton: got %p, gotAgain %p", got, gotAgain)
			}
		})
	}
}

// verifyMetricValue is a helper function that checks for empty mf.Metric,
// reads either Gauge or Counter value and compares to expectedValue.
func verifyMetricValue(t *testing.T, mf *dto.MetricFamily, expectedValue float64) {
	t.Helper()

	if len(mf.GetMetric()) == 0 {
		t.Errorf("No metrics found for %s", mf.GetName())

		return
	}

	var actualValue float64
	if mf.GetMetric()[0].GetGauge() != nil {
		actualValue = mf.GetMetric()[0].GetGauge().GetValue()
	} else if mf.GetMetric()[0].GetCounter() != nil {
		actualValue = mf.GetMetric()[0].GetCounter().GetValue()
	}

	if actualValue != expectedValue {
		t.Errorf("Metric %s = %v, want %v", mf.GetName(), actualValue, expectedValue)
	}
}

func TestMetrics_IntegrationWithOtherTypes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		registry := prometheus.NewRegistry()

		m, err := NewWithRegistry(registry)
		if err != nil {
			t.Fatalf("Failed to create metrics: %v", err)
		}

		t.Cleanup(func() { m.Shutdown() })

		// Test comprehensive integration of all metric types including restarted
		testMetric := &Metric{
			Scanned:   10,
			Updated:   4,
			Failed:    2,
			Restarted: 3,
		}

		// Register the comprehensive metric
		m.Register(testMetric)
		synctest.Wait()

		// Gather and verify all metrics are set correctly
		metricFamilies, err := registry.Gather()
		if err != nil {
			t.Fatalf("Failed to gather metrics: %v", err)
		}

		expectedValues := map[string]float64{
			"watchtower_containers_scanned":   10,
			"watchtower_containers_updated":   4,
			"watchtower_containers_failed":    2,
			"watchtower_containers_restarted": 3,
			"watchtower_scans_total":          1, // Counter incremented
			"watchtower_scans_skipped_total":  0, // No skips
		}

		for _, mf := range metricFamilies {
			if expectedValue, exists := expectedValues[mf.GetName()]; exists {
				verifyMetricValue(t, mf, expectedValue)
			}
		}

		// Test that restarted metrics don't interfere with other counters
		m.Register(&Metric{Scanned: 5, Updated: 1, Failed: 0, Restarted: 2})
		synctest.Wait()

		// Re-gather and verify counters incremented correctly
		metricFamilies, err = registry.Gather()
		if err != nil {
			t.Fatalf("Failed to gather metrics: %v", err)
		}

		// Verify final state
		finalExpectedValues := map[string]float64{
			"watchtower_containers_scanned":   5, // Latest gauge value
			"watchtower_containers_updated":   1, // Latest gauge value
			"watchtower_containers_failed":    0, // Latest gauge value
			"watchtower_containers_restarted": 2, // Latest gauge value
			"watchtower_scans_total":          2, // Counter incremented again
			"watchtower_scans_skipped_total":  0, // Still no skips
		}

		for _, mf := range metricFamilies {
			if expectedValue, exists := finalExpectedValues[mf.GetName()]; exists {
				verifyMetricValue(t, mf, expectedValue)
			}
		}

		// Shutdown
		m.Shutdown()

		// Wait for shutdown
		synctest.Wait()

		// Assert metrics stopped: register a metric after shutdown and verify gauges don't change
		// The last registered metric was {Scanned: 5, Updated: 1, Failed: 0, Restarted: 2}
		expectedAfterShutdown := map[string]float64{
			"watchtower_containers_scanned":   5,
			"watchtower_containers_updated":   1,
			"watchtower_containers_failed":    0,
			"watchtower_containers_restarted": 2,
		}
		assertMetricsStoppedAfterShutdown(t, m, registry, expectedAfterShutdown)
	})
}

func TestNewWithRegistry(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "new metrics with registry"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := prometheus.NewRegistry()

			got, err := NewWithRegistry(registry)
			if err != nil {
				t.Fatalf("NewWithRegistry() returned error: %v", err)
			}

			t.Cleanup(func() { got.Shutdown() })

			if got == nil {
				t.Fatalf("NewWithRegistry() returned nil pointer")
			}

			if got.channel == nil {
				t.Errorf("NewWithRegistry().channel is nil")
			} else if cap(got.channel) != 10 {
				t.Errorf("NewWithRegistry() channel capacity = %d, want 10", cap(got.channel))
			}

			if got.scanned == nil {
				t.Errorf("NewWithRegistry().scanned is nil")
			}

			if got.updated == nil {
				t.Errorf("NewWithRegistry().updated is nil")
			}

			if got.failed == nil {
				t.Errorf("NewWithRegistry().failed is nil")
			}

			if got.total == nil {
				t.Errorf("NewWithRegistry().total is nil")
			}

			if got.skippedScans == nil {
				t.Errorf("NewWithRegistry().skippedScans is nil")
			}

			// Gather metrics from the registry and verify registration
			metricFamilies, err := registry.Gather()
			if err != nil {
				t.Fatalf("Failed to gather metrics: %v", err)
			}

			if len(metricFamilies) != 14 {
				t.Errorf("Expected 14 metric families registered, got %d", len(metricFamilies))
			}

			expectedNames := map[string]bool{
				"watchtower_containers_scanned":              true,
				"watchtower_containers_updated":              true,
				"watchtower_containers_failed":               true,
				"watchtower_containers_restarted":            true,
				"watchtower_containers_skipped":              true,
				"watchtower_containers_restarted_total":      true,
				"watchtower_scans_total":                     true,
				"watchtower_scans_skipped_total":             true,
				"watchtower_scans_failed_total":              true,
				"watchtower_metrics_dropped_total":           true,
				"watchtower_docker_images_bytes":             true,
				"watchtower_docker_images_reclaimable_bytes": true,
				"watchtower_disk_space_max_bytes":            true,
				"watchtower_disk_space_warn_bytes":           true,
			}

			for _, mf := range metricFamilies {
				if !expectedNames[mf.GetName()] {
					t.Errorf("Unexpected metric family registered: %s", mf.GetName())
				}
			}
		})
	}
}

func TestMetrics_RaceConditions(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		registry := prometheus.NewRegistry()

		m, err := NewWithRegistry(registry)
		if err != nil {
			t.Fatalf("Failed to create metrics: %v", err)
		}

		t.Cleanup(func() { m.Shutdown() })

		// Test concurrent registration of metrics with restarted containers
		const (
			numGoroutines       = 10
			metricsPerGoroutine = 5
		)

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := range numGoroutines {
			go func(id int) {
				defer wg.Done()

				for j := range metricsPerGoroutine {
					metric := &Metric{
						Scanned:   1 + id + j,
						Updated:   id % 2,
						Failed:    (id + j) % 3,
						Restarted: id + j, // Include restarted in race condition test
					}
					m.Register(metric)
				}
			}(i)
		}

		wg.Wait()

		// Wait for all metrics to be processed
		synctest.Wait()

		// Verify that the system didn't crash and metrics were processed
		metricFamilies, err := registry.Gather()
		if err != nil {
			t.Fatalf("Failed to gather metrics after concurrent operations: %v", err)
		}

		// Check that scans_total counter was incremented (at least once per registered metric, accounting for drops)
		var (
			totalScans   float64
			totalDropped float64
		)

		for _, mf := range metricFamilies {
			switch mf.GetName() {
			case "watchtower_scans_total":
				if len(mf.GetMetric()) > 0 && mf.GetMetric()[0].GetCounter() != nil {
					totalScans = mf.GetMetric()[0].GetCounter().GetValue()
				}
			case "watchtower_metrics_dropped_total":
				if len(mf.GetMetric()) > 0 && mf.GetMetric()[0].GetCounter() != nil {
					totalDropped = mf.GetMetric()[0].GetCounter().GetValue()
				}
			}
		}

		expectedMinProcessed := float64(numGoroutines * metricsPerGoroutine)

		totalProcessed := totalScans + totalDropped
		if totalProcessed < expectedMinProcessed {
			t.Errorf(
				"Expected at least %v total metrics processed (scans + dropped) after concurrent operations, got %v (scans: %v, dropped: %v)",
				expectedMinProcessed,
				totalProcessed,
				totalScans,
				totalDropped,
			)
		}

		// Shutdown
		m.Shutdown()

		// Wait for shutdown
		synctest.Wait()

		// Assert metrics stopped: register a metric after shutdown and verify scans_total doesn't increase
		testMetricAfterShutdown := &Metric{Scanned: 10, Updated: 5, Failed: 2, Restarted: 7}
		m.Register(testMetricAfterShutdown)
		synctest.Wait()

		// Gather metrics again
		metricFamiliesAfter, err := registry.Gather()
		if err != nil {
			t.Fatalf("Failed to gather metrics after shutdown: %v", err)
		}

		// Check that scans_total hasn't increased (meaning the metric wasn't processed)
		var totalScansAfter float64

		for _, mf := range metricFamiliesAfter {
			if mf.GetName() == "watchtower_scans_total" {
				if len(mf.GetMetric()) > 0 && mf.GetMetric()[0].GetCounter() != nil {
					totalScansAfter = mf.GetMetric()[0].GetCounter().GetValue()
				}

				break
			}
		}

		if totalScansAfter != totalScans {
			t.Errorf(
				"scans_total changed after shutdown: got %v, want %v",
				totalScansAfter,
				totalScans,
			)
		}
	})
}

func TestRegisterScan(t *testing.T) {
	type args struct {
		metric *Metric
	}

	tests := []struct {
		name string
		args args
	}{
		{
			name: "register scan metric",
			args: args{
				metric: &Metric{Scanned: 2, Updated: 1, Failed: 0, Restarted: 0},
			},
		},
		{
			name: "register nil metric",
			args: args{
				metric: nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Metrics{channel: make(chan *Metric, 10)}
			m.RegisterScan(tt.args.metric)

			select {
			case got := <-m.channel:
				if !reflect.DeepEqual(got, tt.args.metric) {
					t.Errorf("RegisterScan() enqueued %v, want %v", got, tt.args.metric)
				}
			default:
				t.Errorf("RegisterScan() did not enqueue metric")
			}
		})
	}
}

func TestMetrics_RestartedWithPartialFailures(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		registry := prometheus.NewRegistry()

		m, err := NewWithRegistry(registry)
		if err != nil {
			t.Fatalf("Failed to create metrics: %v", err)
		}

		t.Cleanup(func() { m.Shutdown() })

		// Test scenario where restarted containers have partial failures
		// This simulates a report where some containers were restarted successfully
		// while others failed during the restart process
		testCases := []struct {
			name     string
			metric   *Metric
			expected map[string]float64
		}{
			{
				name: "partial restart success",
				metric: &Metric{
					Scanned:   8,
					Updated:   3,
					Failed:    2,
					Restarted: 3, // 3 out of potentially more were restarted
				},
				expected: map[string]float64{
					"watchtower_containers_scanned":   8,
					"watchtower_containers_updated":   3,
					"watchtower_containers_failed":    2,
					"watchtower_containers_restarted": 3,
				},
			},
			{
				name: "all restarted with failures",
				metric: &Metric{
					Scanned:   5,
					Updated:   0,
					Failed:    3,
					Restarted: 2, // Only 2 restarted despite failures
				},
				expected: map[string]float64{
					"watchtower_containers_scanned":   5,
					"watchtower_containers_updated":   0,
					"watchtower_containers_failed":    3,
					"watchtower_containers_restarted": 2,
				},
			},
			{
				name: "successful restarts only",
				metric: &Metric{
					Scanned:   6,
					Updated:   4,
					Failed:    0,
					Restarted: 2, // Clean restarts without failures
				},
				expected: map[string]float64{
					"watchtower_containers_scanned":   6,
					"watchtower_containers_updated":   4,
					"watchtower_containers_failed":    0,
					"watchtower_containers_restarted": 2,
				},
			},
		}

		for _, tc := range testCases {
			m.Register(tc.metric)
			synctest.Wait()

			// Gather and verify metrics
			metricFamilies, err := registry.Gather()
			if err != nil {
				t.Fatalf("Failed to gather metrics: %v", err)
			}

			for _, mf := range metricFamilies {
				if expectedValue, exists := tc.expected[mf.GetName()]; exists {
					if len(mf.GetMetric()) == 0 {
						t.Errorf("No metrics found for %s", mf.GetName())

						continue
					}

					var actualValue float64
					if mf.GetMetric()[0].GetGauge() != nil {
						actualValue = mf.GetMetric()[0].GetGauge().GetValue()
					}

					if actualValue != expectedValue {
						t.Errorf(
							"Metric %s = %v, want %v",
							mf.GetName(),
							actualValue,
							expectedValue,
						)
					}
				}
			}
		}

		// Shutdown
		m.Shutdown()

		// Wait for shutdown
		synctest.Wait()

		// Assert metrics stopped: register a metric after shutdown and verify gauges don't change
		// The last registered metric was {Scanned: 6, Updated: 4, Failed: 0, Restarted: 2}
		expectedAfterShutdown := map[string]float64{
			"watchtower_containers_scanned":   6,
			"watchtower_containers_updated":   4,
			"watchtower_containers_failed":    0,
			"watchtower_containers_restarted": 2,
		}
		assertMetricsStoppedAfterShutdown(t, m, registry, expectedAfterShutdown)
	})
}

func TestMetrics_HandleUpdate(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tests := []struct {
			name          string
			metric        *Metric // nil for the "skipped scan" path
			expectGauge   float64
			expectCounter float64
		}{
			{
				name:          "handle valid metric",
				metric:        &Metric{Scanned: 3, Updated: 2, Failed: 1, Restarted: 1, Skipped: 1},
				expectGauge:   1, // Skipped gauge set from metric
				expectCounter: 0, // skippedScans not incremented for valid metrics
			},
			{
				name:          "handle nil metric (skipped scan)",
				metric:        nil,
				expectGauge:   0, // Skipped gauge reset to 0
				expectCounter: 1, // skippedScans incremented for skipped scans
			},
		}

		for _, tt := range tests {
			reg := prometheus.NewRegistry()
			ctx, cancel := context.WithCancel(context.Background())

			m := &Metrics{
				channel: make(chan *Metric, 1),
				stopCh:  make(chan struct{}),
				ctx:     ctx,
				cancel:  cancel,
				scanned: promauto.With(reg).
					NewGauge(prometheus.GaugeOpts{Name: "test_scanned"}),
				updated: promauto.With(reg).
					NewGauge(prometheus.GaugeOpts{Name: "test_updated"}),
				failed: promauto.With(reg).
					NewGauge(prometheus.GaugeOpts{Name: "test_failed"}),
				restarted: promauto.With(reg).
					NewGauge(prometheus.GaugeOpts{Name: "test_restarted"}),
				skipped: promauto.With(reg).
					NewGauge(prometheus.GaugeOpts{Name: "test_skipped"}),
				restartedTotal: promauto.With(reg).
					NewCounter(prometheus.CounterOpts{Name: "test_restarted_total"}),
				total: promauto.With(reg).
					NewCounter(prometheus.CounterOpts{Name: "test_total"}),
				skippedScans: promauto.With(reg).
					NewCounter(prometheus.CounterOpts{Name: "test_skipped_scans"}),
			}

			// Start HandleUpdate in a goroutine
			done := make(chan struct{})

			go func() {
				m.HandleUpdate()
				close(done)
			}()

			// Send metric to channel
			m.channel <- tt.metric

			synctest.Wait() // Allow metric to be processed

			// Close stopCh and wait for HandleUpdate to exit
			close(m.stopCh)
			synctest.Wait()

			select {
			case <-done:
				// processed to completion
			default:
				t.Fatal("HandleUpdate timed out")
			}

			// Verify skipped gauge and skippedScans counter values from the registry
			metricFamilies, err := reg.Gather()
			if err != nil {
				t.Fatalf("Failed to gather metrics: %v", err)
			}

			foundSkippedGauge := false
			foundSkippedScans := false

			for _, mf := range metricFamilies {
				switch mf.GetName() {
				case "test_skipped":
					foundSkippedGauge = true

					if len(mf.GetMetric()) == 0 {
						t.Fatalf("%s: no samples in %s metric family", tt.name, mf.GetName())
					}

					if mf.GetMetric()[0].GetGauge() == nil {
						t.Fatalf("%s: %s metric is not a gauge", tt.name, mf.GetName())
					}

					val := mf.GetMetric()[0].GetGauge().GetValue()
					if val != tt.expectGauge {
						t.Errorf("%s: skipped gauge = %v, want %v", tt.name, val, tt.expectGauge)
					}
				case "test_skipped_scans":
					foundSkippedScans = true

					if len(mf.GetMetric()) == 0 {
						t.Fatalf("%s: no samples in %s metric family", tt.name, mf.GetName())
					}

					if mf.GetMetric()[0].GetCounter() == nil {
						t.Fatalf("%s: %s metric is not a counter", tt.name, mf.GetName())
					}

					val := mf.GetMetric()[0].GetCounter().GetValue()
					if val != tt.expectCounter {
						t.Errorf("%s: skippedScans counter = %v, want %v", tt.name, val, tt.expectCounter)
					}
				}
			}

			if !foundSkippedGauge {
				t.Errorf("%s: skipped gauge metric not found in registry", tt.name)
			}

			if !foundSkippedScans {
				t.Errorf("%s: skippedScans counter metric not found in registry", tt.name)
			}
		}
	})
}

func TestMetrics_HandleUpdate_ContextCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tests := []struct {
			name  string
			setup func(*Metrics) // Function to set up the test scenario
		}{
			{
				name: "context cancellation without pending metrics",
				setup: func(m *Metrics) {
					// No metrics sent, just cancel context
					m.cancel()
				},
			},
			{
				name: "context cancellation during metrics processing",
				setup: func(m *Metrics) {
					// Send a metric and then cancel context
					m.channel <- &Metric{Scanned: 5, Updated: 3, Failed: 1, Restarted: 2}

					synctest.Wait() // Allow processing to start
					m.cancel()
				},
			},
			{
				name: "context cancellation with pending metrics in channel",
				setup: func(m *Metrics) {
					// Fill channel with multiple metrics
					m.channel <- &Metric{Scanned: 2, Updated: 1, Failed: 0, Restarted: 1}

					m.channel <- &Metric{Scanned: 4, Updated: 2, Failed: 1, Restarted: 1}

					m.channel <- &Metric{Scanned: 6, Updated: 3, Failed: 0, Restarted: 2}

					synctest.Wait() // Brief delay
					m.cancel()
				},
			},
		}

		for _, tt := range tests {
			// Create metrics instance with registry
			reg := prometheus.NewRegistry()
			ctx, cancel := context.WithCancel(context.Background())

			m := &Metrics{
				channel: make(chan *Metric, 10), // Larger buffer for pending metrics test
				stopCh:  make(chan struct{}),
				ctx:     ctx,
				cancel:  cancel,
				scanned: promauto.With(reg).
					NewGauge(prometheus.GaugeOpts{Name: "test_scanned_ctx"}),
				updated: promauto.With(reg).
					NewGauge(prometheus.GaugeOpts{Name: "test_updated_ctx"}),
				failed: promauto.With(reg).
					NewGauge(prometheus.GaugeOpts{Name: "test_failed_ctx"}),
				restarted: promauto.With(reg).
					NewGauge(prometheus.GaugeOpts{Name: "test_restarted_ctx"}),
				skipped: promauto.With(reg).
					NewGauge(prometheus.GaugeOpts{Name: "test_skipped_ctx"}),
				restartedTotal: promauto.With(reg).
					NewCounter(prometheus.CounterOpts{Name: "test_restarted_total_ctx"}),
				total: promauto.With(reg).
					NewCounter(prometheus.CounterOpts{Name: "test_total_ctx"}),
				skippedScans: promauto.With(reg).
					NewCounter(prometheus.CounterOpts{Name: "test_skipped_scans_ctx"}),
				dropped: promauto.With(reg).
					NewCounter(prometheus.CounterOpts{Name: "test_dropped_ctx"}),
			}

			// Start HandleUpdate in goroutine
			done := make(chan struct{})

			go func() {
				m.HandleUpdate()
				close(done)
			}()

			// Execute test setup (sends metrics and/or cancels context)
			tt.setup(m)

			// Wait for HandleUpdate to exit due to context cancellation
			synctest.Wait()

			// Check if done is closed
			select {
			case <-done:
				// HandleUpdate exited cleanly due to context cancellation
			default:
				t.Fatal(
					"HandleUpdate did not shutdown within timeout after context cancellation",
				)
			}

			// Verify that the context is indeed canceled
			select {
			case <-m.ctx.Done():
				// Context is canceled as expected
			default:
				t.Error("Context was not canceled")
			}

			// Verify that stopCh is still open (not closed by Shutdown method)
			select {
			case <-m.stopCh:
				t.Error("stopCh was closed, but Shutdown() was not called")
			default:
				// stopCh is still open, which is correct for context-only cancellation
			}
		}
	})
}

// assertMetricsStoppedAfterShutdown verifies that metrics processing has stopped after shutdown
// by registering a test metric and ensuring gauge values don't change.
func assertMetricsStoppedAfterShutdown(
	t *testing.T,
	m *Metrics,
	registry *prometheus.Registry,
	expectedValues map[string]float64,
) {
	t.Helper()

	testMetric := &Metric{Scanned: 10, Updated: 5, Failed: 2, Restarted: 7}
	m.Register(testMetric)
	synctest.Wait()

	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics after shutdown: %v", err)
	}

	for _, mf := range metricFamilies {
		if expectedValue, exists := expectedValues[mf.GetName()]; exists {
			if len(mf.GetMetric()) == 0 {
				continue
			}

			actualValue := mf.GetMetric()[0].GetGauge().GetValue()
			if actualValue != expectedValue {
				t.Errorf(
					"Metric %s changed after shutdown: got %v, want %v",
					mf.GetName(),
					actualValue,
					expectedValue,
				)
			}
		}
	}
}

func TestSetImageDiskUsage(t *testing.T) {
	registry := prometheus.NewRegistry()

	m, err := NewWithRegistry(registry)
	if err != nil {
		t.Fatalf("Failed to create metrics: %v", err)
	}

	t.Cleanup(m.Shutdown)

	m.SetImageDiskUsage(4096, 1024, 40_000, 32_000)

	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Failed to gather metrics: %v", err)
	}

	want := map[string]float64{
		"watchtower_docker_images_bytes":             4096,
		"watchtower_docker_images_reclaimable_bytes": 1024,
		"watchtower_disk_space_max_bytes":            40_000,
		"watchtower_disk_space_warn_bytes":           32_000,
	}

	for _, mf := range metricFamilies {
		expected, ok := want[mf.GetName()]
		if !ok {
			continue
		}

		if len(mf.GetMetric()) == 0 {
			t.Errorf("No samples for %s", mf.GetName())

			continue
		}

		got := mf.GetMetric()[0].GetGauge().GetValue()
		if got != expected {
			t.Errorf("%s = %v, want %v", mf.GetName(), got, expected)
		}

		delete(want, mf.GetName())
	}

	if len(want) != 0 {
		t.Errorf("missing metrics: %v", want)
	}
}

// TestMetrics_FailedScans verifies that a failed scan increments the failed
// scans counter as well as the scans counter, sets the gauges from its partial
// counts, and is marked failed in the last scan and the history, while a
// successful scan is not.
func TestMetrics_FailedScans(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		m, err := NewWithRegistry(prometheus.NewRegistry())
		require.NoError(t, err)

		defer m.Shutdown()

		m.Register(&Metric{Scanned: 4, Updated: 1})
		m.Register(&Metric{Scanned: 2, Failed: 1, ScanFailed: true})
		synctest.Wait()

		assert.InDelta(t, 2, collectorValue(t, m.total), 0)
		assert.InDelta(t, 1, collectorValue(t, m.failedScans), 0)
		assert.InDelta(t, 2, collectorValue(t, m.scanned), 0)
		assert.InDelta(t, 1, collectorValue(t, m.failed), 0)
		assert.True(t, m.GetLastScan().ScanFailed)

		history := m.GetHistory(nil, nil, 0)
		require.Len(t, history, 2)
		assert.False(t, history[0].ScanFailed)
		assert.True(t, history[1].ScanFailed)
	})
}

// collectorValue returns the current value of a counter or gauge.
func collectorValue(t *testing.T, metric prometheus.Metric) float64 {
	t.Helper()

	var out dto.Metric

	require.NoError(t, metric.Write(&out))

	if out.GetCounter() != nil {
		return out.GetCounter().GetValue()
	}

	return out.GetGauge().GetValue()
}
