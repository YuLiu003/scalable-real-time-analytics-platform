package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Global metrics that are exposed by the main package
var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Number of HTTP requests processed",
		},
		[]string{"endpoint", "method", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"endpoint", "method"},
	)

	// Note: Processing metrics are defined in processor/processor.go
	// Do not duplicate them here to avoid conflicts
)

// RecordHTTPRequest records metrics for an HTTP request
func RecordHTTPRequest(endpoint, method, status string, duration float64) {
	httpRequestsTotal.WithLabelValues(endpoint, method, status).Inc()
	httpRequestDuration.WithLabelValues(endpoint, method).Observe(duration)
}
