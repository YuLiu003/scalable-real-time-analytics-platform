package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger middleware logs the request path, method, client IP, and response time
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start time
		start := time.Now()

		// Process request
		c.Next()

		// End time
		end := time.Now()
		latency := end.Sub(start)

		// Access the request method and path
		method := c.Request.Method
		path := c.Request.URL.Path
		status := c.Writer.Status()
		clientIP := c.ClientIP()

		// Log format
		log.Printf("[ADMIN-UI] %s | %3d | %12v | %s | %s",
			method,
			status,
			latency,
			clientIP,
			path,
		)
	}
}
