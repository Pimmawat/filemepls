package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CORS allow-lists exact origins (required for credentialed CORS — a
// wildcard origin can't be combined with Allow-Credentials) and echoes the
// request's Origin back when it matches.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = true
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if allowed[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Vary", "Origin")
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS, PATCH, PUT")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Range")
		c.Header("Access-Control-Expose-Headers", "Content-Range, Content-Length, Accept-Ranges")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
