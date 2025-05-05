package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RequestCount tracks the number of HTTP requests
	RequestCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "admin_ui_request_count",
			Help: "Total number of requests to the Admin UI service",
		},
		[]string{"path", "method", "status"},
	)

	// ResponseTime tracks request response time
	ResponseTime = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "admin_ui_response_time_seconds",
			Help:    "Response time of the Admin UI service",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"path", "method", "status"},
	)

	// TenantServiceLatency tracks latency of tenant service requests
	TenantServiceLatency = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "tenant_service_request_latency_seconds",
			Help:    "Latency of requests to the tenant service",
			Buckets: prometheus.DefBuckets,
		},
	)

	// TenantServiceErrors tracks errors from tenant service
	TenantServiceErrors = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "tenant_service_request_errors",
			Help: "Total number of errors from the tenant service",
		},
	)
)

// PrometheusMetrics middleware records HTTP request metrics
func PrometheusMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		// Process request
		c.Next()

		// After request
		status := strconv.Itoa(c.Writer.Status())
		RequestCount.WithLabelValues(path, method, status).Inc()
		ResponseTime.WithLabelValues(path, method, status).Observe(time.Since(start).Seconds())
	}
}
