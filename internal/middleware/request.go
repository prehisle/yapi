package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/prehisle/yapi/pkg/metrics"
)

// RequestIDKey is the gin context key storing current request id.
const RequestIDKey = "request_id"

// RequestID ensures each incoming request has a traceable identifier.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.NewString()
		}
		c.Set(RequestIDKey, requestID)
		c.Writer.Header().Set("X-Request-ID", requestID)
		c.Next()
	}
}

// AccessLogger prints structured logs for each request.
func AccessLogger(logger *slog.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start)
		status := c.Writer.Status()
		requestID := RequestIDFromContext(c)
		route := c.FullPath()
		if route == "" {
			route = "<unmatched>"
		}

		logger.Info("http request",
			"request_id", requestID,
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", status,
			"latency_ms", duration.Milliseconds(),
			"client_ip", c.ClientIP(),
			"user_agent", c.Request.UserAgent(),
		)

		metrics.ObserveHTTPRequest(c.Request.Method, route, status, duration)
	}
}

// RequestIDFromContext returns the request id stored in gin context.
func RequestIDFromContext(c *gin.Context) string {
	if value, exists := c.Get(RequestIDKey); exists {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return ""
}

// WithRequestID adds the request id header to outgoing upstream requests.
func WithRequestID(req *http.Request, requestID string) {
	if requestID == "" {
		return
	}
	req.Header.Set("X-Request-ID", requestID)
}
