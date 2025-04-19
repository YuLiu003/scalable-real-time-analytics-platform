package main

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const appName = "gin_api" // To distinguish from other potential services

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "api_requests_total",
			Help: "Total number of HTTP requests processed, partitioned by status code, method and path.",
		},
		[]string{"app", "code", "method", "path"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "api_request_duration_seconds",
			Help: "HTTP request latency distributions.",
			// Define suitable buckets, e.g., from 1ms to 10s
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 14), // 1ms, 2ms, 4ms ... ~8s
		},
		[]string{"app", "path", "method"},
	)

	// Add tenant-specific request count (mirroring Flask app)
	tenantRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "tenant_request_count",
			Help: "Requests by tenant ID and endpoint.",
		},
		[]string{"tenant_id", "endpoint"},
	)

	kafkaMessagesProduced = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "kafka_messages_produced_total",
			Help: "Total number of messages successfully produced to Kafka, partitioned by topic.",
		},
		[]string{"topic"},
	)
)

// prometheusMiddleware intercepts HTTP requests to record metrics.
func prometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath() // Get the matched route path (e.g., /api/data)
		if path == "" {
			path = "unmatched" // Handle unmatched routes
		}

		// Process request
		c.Next()

		// After request
		duration := time.Since(start)
		statusCode := c.Writer.Status()
		method := c.Request.Method

		// Record metrics
		httpRequestsTotal.WithLabelValues(appName, strconv.Itoa(statusCode), method, path).Inc()
		httpRequestDuration.WithLabelValues(appName, path, method).Observe(duration.Seconds())

		// Record tenant-specific metric if tenant ID is available
		// We need to extract the tenant ID potentially added in the handler
		// Let's add it to the Gin context within the handlers where applicable
		if tenantID, exists := c.Get("tenantID"); exists {
			if tenantIDStr, ok := tenantID.(string); ok && tenantIDStr != "" && tenantIDStr != defaultTenantKey {
				tenantRequestsTotal.WithLabelValues(tenantIDStr, path).Inc()
			}
		}
	}
}

// prometheusHandler returns a Gin handler for the /metrics endpoint.
func prometheusHandler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}
