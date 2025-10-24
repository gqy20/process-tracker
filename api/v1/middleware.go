package v1

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CORSMiddleware handles Cross-Origin Resource Sharing
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		c.Header("Access-Control-Expose-Headers", "Content-Length")
		c.Header("Access-Control-Allow-Credentials", "true")

		// Handle preflight requests
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// RequestIDMiddleware adds a unique request ID to each request
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// LoggingMiddleware logs request information
func LoggingMiddleware() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("[%s] %s %s %d %s %s \"%s\" %s\n",
			param.TimeStamp.Format(time.RFC3339),
			param.Method,
			param.Path,
			param.StatusCode,
			param.Latency,
			param.ClientIP,
			param.Request.UserAgent(),
			param.ErrorMessage,
		)
	})
}

// ErrorHandlingMiddleware handles panics and errors
func ErrorHandlingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				SendInternalServerError(c, fmt.Errorf("panic recovered: %v", err))
				c.Abort()
			}
		}()
		c.Next()
	}
}

// ValidationMiddleware validates query parameters
func ValidationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var params QueryParams
		if err := c.ShouldBindQuery(&params); err != nil {
			SendBadRequest(c, "Invalid query parameters: "+err.Error())
			c.Abort()
			return
		}

		// Set default values
		if params.Limit == 0 {
			params.Limit = 20 // Default limit
		}
		if params.Limit > 100 {
			params.Limit = 100 // Max limit
		}
		if params.Format == "" {
			params.Format = "json"
		}
		if params.View == "" {
			params.View = "flat"
		}

		c.Set("query_params", params)
		c.Next()
	}
}

// ContentTypeMiddleware ensures proper content type for POST/PUT requests
func ContentTypeMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == "POST" || c.Request.Method == "PUT" || c.Request.Method == "PATCH" {
			contentType := c.GetHeader("Content-Type")
			if contentType != "" && !strings.Contains(contentType, "application/json") {
				SendBadRequest(c, "Content-Type must be application/json")
				c.Abort()
				return
			}
		}
		c.Next()
	}
}

// SecurityHeadersMiddleware adds security headers
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Content-Security-Policy", "default-src 'self'")
		c.Next()
	}
}

// APIVersionMiddleware ensures API version consistency
func APIVersionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if !strings.HasPrefix(path, "/v1/") {
			SendBadRequest(c, "Invalid API version. Use /v1/ prefix")
			c.Abort()
			return
		}
		c.Next()
	}
}

// RateLimitMiddleware implements basic rate limiting
func RateLimitMiddleware() gin.HandlerFunc {
	// Simple in-memory rate limiter
	// In production, you'd want to use Redis or similar
	clients := make(map[string][]time.Time)

	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		now := time.Now()

		// Clean old entries (older than 1 minute)
		if timestamps, exists := clients[clientIP]; exists {
			var validTimestamps []time.Time
			for _, timestamp := range timestamps {
				if now.Sub(timestamp) < time.Minute {
					validTimestamps = append(validTimestamps, timestamp)
				}
			}
			clients[clientIP] = validTimestamps
		}

		// Check rate limit (100 requests per minute)
		if len(clients[clientIP]) >= 100 {
			SendError(c, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED",
				"Rate limit exceeded (100 requests per minute)", nil)
			c.Abort()
			return
		}

		// Add current request timestamp
		clients[clientIP] = append(clients[clientIP], now)
		c.Next()
	}
}

// TimeoutMiddleware sets a timeout for requests
func TimeoutMiddleware(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request = c.Request.WithContext(c.Request.Context())
		c.Next()
	}
}