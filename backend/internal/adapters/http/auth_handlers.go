package http

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"filemepls/internal/usecase"
)

const oauthStateCookieName = "oauth_state"

// AuthorizeHandler redirects the browser to the provider's consent screen,
// stashing a CSRF state value in a short-lived cookie to verify on
// callback.
func AuthorizeHandler(auth *usecase.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		provider := c.Param("provider")

		url, state, err := auth.AuthorizeURL(provider)
		if err != nil {
			respondErr(c, err)
			return
		}

		c.SetCookie(oauthStateCookieName, state, 5*60, "/api/auth", "", isSecureRequest(c), true)
		c.Redirect(http.StatusFound, url)
	}
}

// CallbackHandler verifies the CSRF state, exchanges the authorization
// code, sets the session cookie, and redirects the browser back to the
// frontend. The OAuth provider's redirect_uri must point here (only the
// backend holds the client secret).
func CallbackHandler(auth *usecase.AuthService, frontendBaseURL, defaultLocale string, jwtTTL time.Duration, cookieDomain string) gin.HandlerFunc {
	return func(c *gin.Context) {
		provider := c.Param("provider")
		code := c.Query("code")
		state := c.Query("state")

		stateCookie, err := c.Cookie(oauthStateCookieName)
		if err != nil || state == "" || state != stateCookie {
			respondErr(c, usecase.ErrInvalidState)
			return
		}
		c.SetCookie(oauthStateCookieName, "", -1, "/api/auth", "", isSecureRequest(c), true)

		token, _, err := auth.HandleCallback(c.Request.Context(), provider, code)
		if err != nil {
			respondErr(c, err)
			return
		}

		maxAge := int(jwtTTL.Seconds())
		c.SetCookie(sessionCookieName, token, maxAge, "/", cookieDomain, isSecureRequest(c), true)
		c.Redirect(http.StatusFound, frontendBaseURL+"/"+defaultLocale+"/files")
	}
}

func LogoutHandler(cookieDomain string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.SetCookie(sessionCookieName, "", -1, "/", cookieDomain, isSecureRequest(c), true)
		c.Status(http.StatusNoContent)
	}
}

func MeHandler(auth *usecase.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := auth.Me(c.Request.Context(), userIDFromContext(c))
		if err != nil {
			respondErr(c, err)
			return
		}
		c.JSON(http.StatusOK, toUserDTO(user))
	}
}

// isSecureRequest reports whether the request arrived over HTTPS, directly
// or via a reverse proxy setting X-Forwarded-Proto (e.g. a future Nginx
// layer), so the session/state cookies can set Secure only when it won't
// break local HTTP development.
func isSecureRequest(c *gin.Context) bool {
	return c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
}
