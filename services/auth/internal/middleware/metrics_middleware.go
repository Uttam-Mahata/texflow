package middleware

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"auth/pkg/metrics"
)

// MetricsMiddleware records HTTP metrics
func MetricsMiddleware(m *metrics.Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Process request
		c.Next()

		// Record metrics
		duration := time.Since(start).Seconds()
		status := c.Writer.Status()
		method := c.Request.Method
		path := c.FullPath()

		m.HTTPRequestsTotal.WithLabelValues(
			method,
			path,
			fmt.Sprintf("%d", status),
		).Inc()

		m.HTTPRequestDuration.WithLabelValues(
			method,
			path,
		).Observe(duration)
	}
}
