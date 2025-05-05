package middleware

import (
	"strconv"
	"time"

	"github.com/YuLiu003/real-time-analytics-platform/metrics"

	"github.com/gin-gonic/gin"
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
		metrics.RequestCount.WithLabelValues(path, method, status).Inc()
		metrics.ResponseTime.WithLabelValues(path, method, status).Observe(time.Since(start).Seconds())
	}
}
