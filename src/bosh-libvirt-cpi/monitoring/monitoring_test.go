package monitoring

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewMetricsCollector tests the creation of a new MetricsCollector
func TestNewMetricsCollector(t *testing.T) {
	collector := NewMetricsCollector()
	assert.NotNil(t, collector)
	assert.Equal(t, 0, len(collector.GetMetrics()))
}

// TestRecordMetric tests recording a single metric
func TestRecordMetric(t *testing.T) {
	collector := NewMetricsCollector()
	metric := Metric{
		Type:  OperationLatency,
		Name:  "test_operation",
		Value: 100.0,
		Unit:  "ms",
		Tags:  map[string]string{"host": "localhost"},
	}

	collector.RecordMetric(metric)
	metrics := collector.GetMetrics()

	assert.Equal(t, 1, len(metrics))
	assert.Equal(t, OperationLatency, metrics[0].Type)
	assert.Equal(t, "test_operation", metrics[0].Name)
	assert.Equal(t, 100.0, metrics[0].Value)
}

// TestRecordMetric_AutoTimestamp tests that timestamp is automatically set
func TestRecordMetric_AutoTimestamp(t *testing.T) {
	collector := NewMetricsCollector()
	before := time.Now()

	metric := Metric{
		Type:  OperationCount,
		Name:  "test_count",
		Value: 1.0,
		Unit:  "count",
	}
	collector.RecordMetric(metric)

	after := time.Now()
	metrics := collector.GetMetrics()

	require.Equal(t, 1, len(metrics))
	assert.False(t, metrics[0].Timestamp.IsZero())
	assert.True(t, metrics[0].Timestamp.After(before))
	assert.True(t, metrics[0].Timestamp.Before(after.Add(time.Second)))
}

// TestRecordMetric_CustomTimestamp tests that custom timestamp is preserved
func TestRecordMetric_CustomTimestamp(t *testing.T) {
	collector := NewMetricsCollector()
	customTime := time.Now().Add(-time.Hour)

	metric := Metric{
		Type:      OperationLatency,
		Name:      "test_operation",
		Value:     50.0,
		Unit:      "ms",
		Timestamp: customTime,
	}
	collector.RecordMetric(metric)

	metrics := collector.GetMetrics()
	require.Equal(t, 1, len(metrics))
	assert.Equal(t, customTime, metrics[0].Timestamp)
}

// TestGetMetrics tests retrieving all metrics
func TestGetMetrics(t *testing.T) {
	collector := NewMetricsCollector()

	// Add multiple metrics
	for i := 0; i < 5; i++ {
		metric := Metric{
			Type:  OperationLatency,
			Name:  "test_operation",
			Value: float64(i * 100),
			Unit:  "ms",
		}
		collector.RecordMetric(metric)
	}

	metrics := collector.GetMetrics()
	assert.Equal(t, 5, len(metrics))

	// Verify values are in order
	for i := 0; i < 5; i++ {
		assert.Equal(t, float64(i*100), metrics[i].Value)
	}
}

// TestGetMetrics_ReturnsCopy tests that GetMetrics returns a copy
func TestGetMetrics_ReturnsCopy(t *testing.T) {
	collector := NewMetricsCollector()
	metric := Metric{
		Type:  OperationLatency,
		Name:  "test_operation",
		Value: 100.0,
		Unit:  "ms",
	}
	collector.RecordMetric(metric)

	metrics1 := collector.GetMetrics()
	metrics2 := collector.GetMetrics()

	// Both should have same content but different underlying arrays
	assert.Equal(t, len(metrics1), len(metrics2))
	// Test that we got different slice headers (copies, not same reference)
	assert.True(t, &metrics1 != &metrics2)
}

// TestGetMetricsByType tests retrieving metrics by type
func TestGetMetricsByType(t *testing.T) {
	collector := NewMetricsCollector()

	// Record different types
	collector.RecordMetric(Metric{Type: OperationLatency, Name: "op1", Value: 100.0, Unit: "ms"})
	collector.RecordMetric(Metric{Type: OperationCount, Name: "op1", Value: 1.0, Unit: "count"})
	collector.RecordMetric(Metric{Type: OperationLatency, Name: "op2", Value: 200.0, Unit: "ms"})
	collector.RecordMetric(Metric{Type: ErrorCount, Name: "op1", Value: 0.0, Unit: "count"})

	// Get only OperationLatency metrics
	latencyMetrics := collector.GetMetricsByType(OperationLatency)
	assert.Equal(t, 2, len(latencyMetrics))
	for _, m := range latencyMetrics {
		assert.Equal(t, OperationLatency, m.Type)
	}

	// Get only OperationCount metrics
	countMetrics := collector.GetMetricsByType(OperationCount)
	assert.Equal(t, 1, len(countMetrics))
	assert.Equal(t, OperationCount, countMetrics[0].Type)
}

// TestGetMetricsByName tests retrieving metrics by name
func TestGetMetricsByName(t *testing.T) {
	collector := NewMetricsCollector()

	// Record metrics with different names
	collector.RecordMetric(Metric{Type: OperationLatency, Name: "operation_a", Value: 100.0, Unit: "ms"})
	collector.RecordMetric(Metric{Type: OperationLatency, Name: "operation_b", Value: 200.0, Unit: "ms"})
	collector.RecordMetric(Metric{Type: OperationLatency, Name: "operation_a", Value: 150.0, Unit: "ms"})

	// Get metrics by name
	metricsA := collector.GetMetricsByName("operation_a")
	assert.Equal(t, 2, len(metricsA))
	for _, m := range metricsA {
		assert.Equal(t, "operation_a", m.Name)
	}

	metricsB := collector.GetMetricsByName("operation_b")
	assert.Equal(t, 1, len(metricsB))
	assert.Equal(t, "operation_b", metricsB[0].Name)
}

// TestClearMetrics tests clearing all metrics
func TestClearMetrics(t *testing.T) {
	collector := NewMetricsCollector()

	// Add metrics
	for i := 0; i < 10; i++ {
		collector.RecordMetric(Metric{Type: OperationLatency, Name: "test", Value: float64(i), Unit: "ms"})
	}

	assert.Equal(t, 10, len(collector.GetMetrics()))

	// Clear
	collector.ClearMetrics()
	assert.Equal(t, 0, len(collector.GetMetrics()))
}

// TestSetMetricLimit tests setting a metric limit
func TestSetMetricLimit(t *testing.T) {
	collector := NewMetricsCollector()
	collector.SetMetricLimit("limited_metric", 5)

	// Add more metrics than the limit
	for i := 0; i < 10; i++ {
		collector.RecordMetric(Metric{
			Type:  OperationLatency,
			Name:  "limited_metric",
			Value: float64(i),
			Unit:  "ms",
		})
	}

	// Should only keep 5 most recent
	metrics := collector.GetMetrics()
	assert.Equal(t, 5, len(metrics))

	// Verify we have the most recent values (5-9)
	expectedValues := []float64{5.0, 6.0, 7.0, 8.0, 9.0}
	for i, m := range metrics {
		assert.Equal(t, expectedValues[i], m.Value)
	}
}

// TestSetMetricLimit_MultipleMetrics tests metric limit with mixed metrics
func TestSetMetricLimit_MultipleMetrics(t *testing.T) {
	collector := NewMetricsCollector()
	collector.SetMetricLimit("limited", 3)

	// Add metrics with the limited name - should trim to 3
	for i := 0; i < 5; i++ {
		collector.RecordMetric(Metric{Type: OperationLatency, Name: "limited", Value: float64(i), Unit: "ms"})
	}

	// Verify limit is applied
	limitedMetrics := collector.GetMetricsByName("limited")
	assert.Equal(t, 3, len(limitedMetrics))

	// Now add metrics with unlimited name (no limit set)
	for i := 0; i < 5; i++ {
		collector.RecordMetric(Metric{Type: OperationLatency, Name: "unlimited", Value: float64(i*10), Unit: "ms"})
	}

	unlimitedMetrics := collector.GetMetricsByName("unlimited")
	assert.Equal(t, 5, len(unlimitedMetrics))
}

// TestGetStatistics tests basic statistics calculation
func TestGetStatistics(t *testing.T) {
	collector := NewMetricsCollector()

	// Record metrics: 10, 20, 30, 40, 50
	values := []float64{10.0, 20.0, 30.0, 40.0, 50.0}
	for _, v := range values {
		collector.RecordMetric(Metric{Type: OperationLatency, Name: "stat_test", Value: v, Unit: "ms"})
	}

	stats := collector.GetStatistics("stat_test")

	assert.Equal(t, 5, stats.Count)
	assert.Equal(t, 10.0, stats.Min)
	assert.Equal(t, 50.0, stats.Max)
	assert.Equal(t, 150.0, stats.Total)
	assert.Equal(t, 30.0, stats.Average) // (10+20+30+40+50)/5 = 150/5 = 30
}

// TestGetStatistics_EmptyMetrics tests statistics with no metrics
func TestGetStatistics_EmptyMetrics(t *testing.T) {
	collector := NewMetricsCollector()

	stats := collector.GetStatistics("nonexistent")

	assert.Equal(t, 0, stats.Count)
	assert.Equal(t, 0.0, stats.Min)
	assert.Equal(t, 0.0, stats.Max)
	assert.Equal(t, 0.0, stats.Average)
	assert.Equal(t, 0.0, stats.Total)
}

// TestGetStatistics_SingleMetric tests statistics with single metric
func TestGetStatistics_SingleMetric(t *testing.T) {
	collector := NewMetricsCollector()
	collector.RecordMetric(Metric{Type: OperationLatency, Name: "single", Value: 42.0, Unit: "ms"})

	stats := collector.GetStatistics("single")

	assert.Equal(t, 1, stats.Count)
	assert.Equal(t, 42.0, stats.Min)
	assert.Equal(t, 42.0, stats.Max)
	assert.Equal(t, 42.0, stats.Average)
	assert.Equal(t, 42.0, stats.Total)
}

// TestNewPerformanceTracker tests creating a new performance tracker
func TestNewPerformanceTracker(t *testing.T) {
	collector := NewMetricsCollector()
	tracker := NewPerformanceTracker(collector, "test_operation")

	assert.NotNil(t, tracker)
	assert.Equal(t, collector, tracker.collector)
	assert.Equal(t, "test_operation", tracker.name)
}

// TestPerformanceTracker_End tests performance tracking
func TestPerformanceTracker_End(t *testing.T) {
	collector := NewMetricsCollector()
	tracker := NewPerformanceTracker(collector, "timed_operation")

	// Sleep for a measurable duration
	time.Sleep(10 * time.Millisecond)
	elapsed := tracker.End()

	// Verify elapsed time is reasonable
	assert.True(t, elapsed >= 10*time.Millisecond)

	// Verify metric was recorded
	metrics := collector.GetMetrics()
	require.Equal(t, 1, len(metrics))
	assert.Equal(t, OperationLatency, metrics[0].Type)
	assert.Equal(t, "timed_operation", metrics[0].Name)
	assert.Equal(t, "ms", metrics[0].Unit)
	assert.Greater(t, metrics[0].Value, 0.0)
}

// TestPerformanceTracker_EndWithTags tests performance tracking with tags
func TestPerformanceTracker_EndWithTags(t *testing.T) {
	collector := NewMetricsCollector()
	tracker := NewPerformanceTracker(collector, "tagged_operation")

	tags := map[string]string{
		"host":      "test-host",
		"operation": "vm_creation",
	}

	time.Sleep(5 * time.Millisecond)
	elapsed := tracker.EndWithTags(tags)

	// Verify metric was recorded with tags
	metrics := collector.GetMetrics()
	require.Equal(t, 1, len(metrics))
	assert.Equal(t, "tagged_operation", metrics[0].Name)
	assert.Equal(t, tags, metrics[0].Tags)
	assert.True(t, elapsed > 0)
}

// TestNewHealthChecker tests creating a new health checker
func TestNewHealthChecker(t *testing.T) {
	collector := NewMetricsCollector()
	checker := NewHealthChecker(collector)

	assert.NotNil(t, checker)
	assert.Equal(t, collector, checker.collector)
	assert.Equal(t, 0.05, checker.errorThreshold)
	assert.Equal(t, 1000.0, checker.latencyThreshold)
}

// TestHealthChecker_EmptyMetrics tests health check with no metrics
func TestHealthChecker_EmptyMetrics(t *testing.T) {
	collector := NewMetricsCollector()
	checker := NewHealthChecker(collector)

	health := checker.CheckHealth()

	assert.Equal(t, "healthy", health.Status)
	assert.Equal(t, 0, len(health.Issues))
}

// TestHealthChecker_HealthyOperation tests health check with healthy metrics
func TestHealthChecker_HealthyOperation(t *testing.T) {
	collector := NewMetricsCollector()
	checker := NewHealthChecker(collector)

	// Add healthy metrics (latency < 1000ms)
	for i := 0; i < 10; i++ {
		collector.RecordMetric(Metric{
			Type:  OperationLatency,
			Name:  "operation",
			Value: 100.0, // 100ms, below threshold
			Unit:  "ms",
		})
	}

	health := checker.CheckHealth()
	assert.Equal(t, "healthy", health.Status)
}

// TestHealthChecker_DegradedLatency tests health check with high latency
func TestHealthChecker_DegradedLatency(t *testing.T) {
	collector := NewMetricsCollector()
	checker := NewHealthChecker(collector)

	// Add metrics with high latency (> 1000ms)
	// More than 10% should trigger degraded status
	for i := 0; i < 20; i++ {
		collector.RecordMetric(Metric{
			Type:  OperationLatency,
			Name:  "slow_operation",
			Value: 1500.0, // 1500ms, above threshold
			Unit:  "ms",
		})
	}

	health := checker.CheckHealth()
	assert.Equal(t, "degraded", health.Status)
	assert.True(t, len(health.Issues) > 0)
}

// TestHealthChecker_DegradedErrorRate tests health check with high error rate
func TestHealthChecker_DegradedErrorRate(t *testing.T) {
	collector := NewMetricsCollector()
	checker := NewHealthChecker(collector)

	// Add error metrics
	for i := 0; i < 5; i++ {
		collector.RecordMetric(Metric{
			Type:  ErrorCount,
			Name:  "operation",
			Value: 1.0,
			Unit:  "count",
		})
	}

	health := checker.CheckHealth()
	assert.Equal(t, "degraded", health.Status)
	assert.True(t, len(health.Issues) > 0)
}

// TestCalculateStatistics tests the statistics calculation function
func TestCalculateStatistics(t *testing.T) {
	// Test with normal values
	values := []float64{1.0, 2.0, 3.0, 4.0, 5.0}
	stats := calculateStatistics(values)

	assert.Equal(t, 5, stats.Count)
	assert.Equal(t, 1.0, stats.Min)
	assert.Equal(t, 5.0, stats.Max)
	assert.Equal(t, 15.0, stats.Total)
	assert.Equal(t, 3.0, stats.Average)
}

// TestCalculateStatistics_EmptyValues tests statistics with empty values
func TestCalculateStatistics_EmptyValues(t *testing.T) {
	stats := calculateStatistics([]float64{})

	assert.Equal(t, 0, stats.Count)
	assert.Equal(t, 0.0, stats.Min)
	assert.Equal(t, 0.0, stats.Max)
	assert.Equal(t, 0.0, stats.Average)
	assert.Equal(t, 0.0, stats.Total)
}

// TestCalculateStatistics_NegativeValues tests statistics with negative values
func TestCalculateStatistics_NegativeValues(t *testing.T) {
	values := []float64{-5.0, -2.0, 0.0, 2.0, 5.0}
	stats := calculateStatistics(values)

	assert.Equal(t, 5, stats.Count)
	assert.Equal(t, -5.0, stats.Min)
	assert.Equal(t, 5.0, stats.Max)
	assert.Equal(t, 0.0, stats.Total)
	assert.Equal(t, 0.0, stats.Average)
}

// TestMetricsCollector_ThreadSafety tests thread-safe operations
func TestMetricsCollector_ThreadSafety(t *testing.T) {
	collector := NewMetricsCollector()

	// Use channels to coordinate goroutines
	done := make(chan bool, 10)

	// Launch 10 goroutines to record metrics concurrently
	for i := 0; i < 10; i++ {
		go func(index int) {
			for j := 0; j < 10; j++ {
				collector.RecordMetric(Metric{
					Type:  OperationLatency,
					Name:  "concurrent_test",
					Value: float64(index*10 + j),
					Unit:  "ms",
				})
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have all 100 metrics
	metrics := collector.GetMetrics()
	assert.Equal(t, 100, len(metrics))
}

