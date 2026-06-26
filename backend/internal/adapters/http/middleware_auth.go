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

// OptionalAuth behaves like RequireAuth when a valid session cookie is
// present, but never aborts the request when it's missing or invalid —
// for routes that serve both anonymous and logged-in visitors (public share
// links), where login status only changes what's allowed, not whether the
// route exists.
func OptionalAuth(auth *usecase.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Cookie(sessionCookieName)
		if err != nil || cookie == "" {
			c.Next()
			return
		}

		userID, err := auth.VerifyToken(cookie)
		if err != nil {
			c.Next()
			return
		}

		c.Set("userID", userID)
		c.Next()
	}
}

// optionalUserIDFromContext returns the authenticated user's ID if
// OptionalAuth found a valid session, or uuid.Nil for an anonymous visitor.
func optionalUserIDFromContext(c *gin.Context) uuid.UUID {
	v, ok := c.Get("userID")
	if !ok {
		return uuid.Nil
	}
	return v.(uuid.UUID)
}
