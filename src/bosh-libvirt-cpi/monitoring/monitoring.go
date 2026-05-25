package monitoring

import (
	"sync"
	"time"
)

// MetricType represents the type of metric being recorded
type MetricType string

const (
	OperationLatency MetricType = "operation_latency"
	OperationCount   MetricType = "operation_count"
	ErrorCount       MetricType = "error_count"
	ResourceUsage    MetricType = "resource_usage"
)

// Metric represents a single metric data point
type Metric struct {
	Type      MetricType
	Name      string
	Value     float64
	Unit      string
	Timestamp time.Time
	Tags      map[string]string
}

// MetricsCollector collects and stores metrics
type MetricsCollector struct {
	mu      sync.RWMutex
	metrics []Metric
	limits  map[string]int
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		metrics: make([]Metric, 0),
		limits:  make(map[string]int),
	}
}

// RecordMetric records a new metric
func (mc *MetricsCollector) RecordMetric(metric Metric) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if metric.Timestamp.IsZero() {
		metric.Timestamp = time.Now()
	}

	mc.metrics = append(mc.metrics, metric)

	// Limit metrics storage to prevent unbounded growth
	if limit, exists := mc.limits[metric.Name]; exists {
		if len(mc.metrics) > limit {
			// Keep only the most recent entries
			keep := len(mc.metrics) - limit
			if keep < 0 {
				keep = 0
			}
			mc.metrics = mc.metrics[keep:]
		}
	}
}

// GetMetrics returns all recorded metrics
func (mc *MetricsCollector) GetMetrics() []Metric {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make([]Metric, len(mc.metrics))
	copy(result, mc.metrics)
	return result
}

// GetMetricsByType returns metrics of a specific type
func (mc *MetricsCollector) GetMetricsByType(mType MetricType) []Metric {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	var result []Metric
	for _, m := range mc.metrics {
		if m.Type == mType {
			result = append(result, m)
		}
	}
	return result
}

// GetMetricsByName returns metrics with a specific name
func (mc *MetricsCollector) GetMetricsByName(name string) []Metric {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	var result []Metric
	for _, m := range mc.metrics {
		if m.Name == name {
			result = append(result, m)
		}
	}
	return result
}

// ClearMetrics clears all recorded metrics
func (mc *MetricsCollector) ClearMetrics() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.metrics = make([]Metric, 0)
}

// SetMetricLimit sets the maximum number of metrics to store for a given name
func (mc *MetricsCollector) SetMetricLimit(name string, limit int) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.limits[name] = limit
}

// GetStatistics returns basic statistics for metrics of a given name
func (mc *MetricsCollector) GetStatistics(name string) Statistics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	var values []float64
	for _, m := range mc.metrics {
		if m.Name == name {
			values = append(values, m.Value)
		}
	}

	return calculateStatistics(values)
}

// Statistics holds basic statistical information
type Statistics struct {
	Count   int
	Min     float64
	Max     float64
	Average float64
	Total   float64
}

// calculateStatistics calculates basic statistics from values
func calculateStatistics(values []float64) Statistics {
	stats := Statistics{
		Count: len(values),
	}

	if len(values) == 0 {
		return stats
	}

	stats.Min = values[0]
	stats.Max = values[0]

	for _, v := range values {
		stats.Total += v
		if v < stats.Min {
			stats.Min = v
		}
		if v > stats.Max {
			stats.Max = v
		}
	}

	stats.Average = stats.Total / float64(len(values))
	return stats
}

// PerformanceTracker tracks the performance of operations
type PerformanceTracker struct {
	collector *MetricsCollector
	startTime time.Time
	name      string
}

// NewPerformanceTracker creates a new performance tracker
func NewPerformanceTracker(collector *MetricsCollector, name string) *PerformanceTracker {
	return &PerformanceTracker{
		collector: collector,
		startTime: time.Now(),
		name:      name,
	}
}

// End records the end of an operation and returns the elapsed time
func (pt *PerformanceTracker) End() time.Duration {
	elapsed := time.Since(pt.startTime)

	pt.collector.RecordMetric(Metric{
		Type:  OperationLatency,
		Name:  pt.name,
		Value: float64(elapsed.Milliseconds()),
		Unit:  "ms",
	})

	return elapsed
}

// EndWithTags records the end of an operation with additional tags
func (pt *PerformanceTracker) EndWithTags(tags map[string]string) time.Duration {
	elapsed := time.Since(pt.startTime)

	pt.collector.RecordMetric(Metric{
		Type:  OperationLatency,
		Name:  pt.name,
		Value: float64(elapsed.Milliseconds()),
		Unit:  "ms",
		Tags:  tags,
	})

	return elapsed
}

// HealthChecker checks the health of operations
type HealthChecker struct {
	collector        *MetricsCollector
	errorThreshold   float64
	latencyThreshold float64
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(collector *MetricsCollector) *HealthChecker {
	return &HealthChecker{
		collector:        collector,
		errorThreshold:   0.05, // 5% error rate
		latencyThreshold: 1000, // 1 second in milliseconds
	}
}

// CheckHealth returns the overall health status
func (hc *HealthChecker) CheckHealth() HealthStatus {
	metrics := hc.collector.GetMetrics()

	if len(metrics) == 0 {
		return HealthStatus{Status: "healthy"}
	}

	errorCount := 0
	latencyIssues := 0

	for _, m := range metrics {
		switch m.Type {
		case OperationLatency:
			if m.Value > hc.latencyThreshold {
				latencyIssues++
			}
		case ErrorCount:
			errorCount++
		}
	}

	health := "healthy"
	issues := []string{}

	if errorCount > 0 {
		health = "degraded"
		issues = append(issues, "High error rate detected")
	}

	if latencyIssues > len(metrics)/10 { // More than 10% with high latency
		health = "degraded"
		issues = append(issues, "High latency detected")
	}

	return HealthStatus{
		Status: health,
		Issues: issues,
	}
}

// HealthStatus represents the health status of the system
type HealthStatus struct {
	Status string
	Issues []string
}
