package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// SecurityHeaders adds baseline hardening headers to every response and pins
// the session/state cookies to SameSite=Lax.
//
//   - X-Content-Type-Options: nosniff — stops browsers from MIME-sniffing a
//     download into an executable/HTML type, which (together with the
//     attachment Content-Disposition already set on downloads) neutralizes
//     stored-XSS via uploaded file content.
//   - X-Frame-Options: DENY — no framing, so the app can't be clickjacked.
//   - Referrer-Policy: no-referrer — a share/download URL is never leaked to a
//     third-party site via the Referer header.
//   - Strict-Transport-Security — only emitted when the request is actually
//     HTTPS (directly or via a proxy's X-Forwarded-Proto), so it never breaks
//     plain-HTTP local development.
//   - SameSite=Lax — set here (gin exposes SameSite on the Context, not the
//     Engine) so every downstream c.SetCookie inherits it, giving CSRF
//     protection against cross-site POSTs without an explicit token.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.SetSameSite(http.SameSiteLaxMode)
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}
