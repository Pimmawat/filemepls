package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"filemepls/internal/usecase"
)

const sessionCookieName = "filemepls_session"

// RequireAuth verifies the session cookie and stores the authenticated
// user's ID in the Gin context for downstream handlers.
func RequireAuth(auth *usecase.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Cookie(sessionCookieName)
		if err != nil || cookie == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
			return
		}

		userID, err := auth.VerifyToken(cookie)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid session"})
			return
		}

		c.Set("userID", userID)
		c.Next()
	}
}

func userIDFromContext(c *gin.Context) uuid.UUID {
	return c.MustGet("userID").(uuid.UUID)
}
