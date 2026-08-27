package http

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"filemepls/internal/sendhub"
	"filemepls/internal/usecase"
)

type Deps struct {
	Files           *usecase.FileService
	Folders         *usecase.FolderService
	Shares          *usecase.ShareService
	Permissions     *usecase.PermissionService
	Auth            *usecase.AuthService
	SendHub         *sendhub.Hub
	AllowedOrigins  []string
	FrontendBaseURL string
	DefaultLocale   string
	JWTTTL          time.Duration
	// CookieDomain sets the session cookie's Domain attribute; empty means
	// host-only (fine when frontend and backend share a host/port set).
	CookieDomain string
	// MaxUploadSize caps a single upload's body at the HTTP boundary (bytes);
	// 0 means unlimited. Enforcing it here (via http.MaxBytesReader) stops a
	// huge multipart body from being spooled to a temp file on disk before the
	// usecase's own size check ever runs.
	MaxUploadSize int64
	// TrustedProxies is forwarded to gin.SetTrustedProxies; empty trusts none
	// so ClientIP() (and thus the auth rate limiter) can't be fooled by a
	// spoofed X-Forwarded-For.
	TrustedProxies []string
	// SendRequireAuth, when true, requires a valid session to join the LAN-send
	// signaling hub (otherwise it's open, LocalSend-style).
	SendRequireAuth bool
	// Ready reports whether the service can serve traffic (e.g. the DB is
	// reachable). Backs the /readyz readiness probe; nil means "always ready".
	Ready func(context.Context) error
}

func NewRouter(deps Deps) *gin.Engine {
	// Default to release mode (quiet logs, no debug warnings) unless GIN_MODE
	// explicitly asks for debug/test — never run a production build in the
	// verbose debug mode gin.New() would otherwise imply.
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	// Trust only the configured proxies (none by default) so ClientIP() — the
	// key the auth rate limiter buckets on — reflects the real caller and
	// can't be spoofed via X-Forwarded-For.
	if err := r.SetTrustedProxies(deps.TrustedProxies); err != nil {
		log.Fatalf("invalid TRUSTED_PROXIES: %v", err)
	}
	r.Use(gin.Recovery())
	r.Use(SecurityHeaders()) // also pins SameSite=Lax for every SetCookie downstream
	r.Use(CORS(deps.AllowedOrigins))
	r.MaxMultipartMemory = 1 << 20 // 1MB; the usecase's own io.LimitReader is the real ceiling

	// Liveness: the process is up. Readiness: dependencies (the DB) are
	// reachable — load balancers/orchestrators should gate traffic on /readyz,
	// not /healthz, so a DB outage sheds load instead of returning errors.
	r.GET("/healthz", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/readyz", func(c *gin.Context) {
		if deps.Ready == nil {
			c.Status(http.StatusOK)
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := deps.Ready(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready"})
			return
		}
		c.Status(http.StatusOK)
	})

	// Throttle credential endpoints so passwords can't be brute-forced.
	authLimit := NewRateLimiter(10, time.Minute).Middleware()

	authGroup := r.Group("/api/auth")
	authGroup.GET("/:provider/authorize", AuthorizeHandler(deps.Auth))
	authGroup.GET("/:provider/callback", CallbackHandler(deps.Auth, deps.FrontendBaseURL, deps.DefaultLocale, deps.JWTTTL, deps.CookieDomain))
	authGroup.POST("/register", authLimit, RegisterHandler(deps.Auth, deps.JWTTTL, deps.CookieDomain))
	authGroup.POST("/login", authLimit, LoginHandler(deps.Auth, deps.JWTTTL, deps.CookieDomain))
	authGroup.POST("/logout", LogoutHandler(deps.CookieDomain))
	authGroup.GET("/me", RequireAuth(deps.Auth), MeHandler(deps.Auth))

	filesGroup := r.Group("/api/files")
	filesGroup.Use(RequireAuth(deps.Auth))
	filesGroup.POST("", UploadHandler(deps.Files, deps.MaxUploadSize))
	filesGroup.GET("", ListHandler(deps.Files))
	filesGroup.GET("/:id", MetadataHandler(deps.Files))
	filesGroup.GET("/:id/download", DownloadHandler(deps.Files))
	filesGroup.DELETE("/:id", DeleteHandler(deps.Files))
	filesGroup.PATCH("/:id/move", MoveFileHandler(deps.Folders))
	filesGroup.POST("/:id/shares", CreateShareHandler(deps.Shares))
	filesGroup.GET("/:id/shares", ListSharesHandler(deps.Shares))
	filesGroup.POST("/:id/permissions", GrantFileAccessHandler(deps.Permissions))
	filesGroup.GET("/:id/permissions", ListFileGrantsHandler(deps.Permissions))

	foldersGroup := r.Group("/api/folders")
	foldersGroup.Use(RequireAuth(deps.Auth))
	foldersGroup.POST("", CreateFolderHandler(deps.Folders))
	foldersGroup.GET("/browse", BrowseFoldersHandler(deps.Folders))
	foldersGroup.DELETE("/:id", DeleteFolderHandler(deps.Folders))
	foldersGroup.PATCH("/:id/move", MoveFolderHandler(deps.Folders))
	foldersGroup.GET("/:id/download", FolderZipHandler(deps.Folders))
	foldersGroup.POST("/:id/shares", CreateFolderShareHandler(deps.Shares))
	foldersGroup.GET("/:id/shares", ListFolderSharesHandler(deps.Shares))
	foldersGroup.POST("/:id/permissions", GrantFolderAccessHandler(deps.Permissions))
	foldersGroup.GET("/:id/permissions", ListFolderGrantsHandler(deps.Permissions))

	r.DELETE("/api/shares/:id", RequireAuth(deps.Auth), RevokeShareHandler(deps.Shares))
	r.DELETE("/api/permissions/:id", RequireAuth(deps.Auth), RevokeGrantHandler(deps.Permissions))
	r.GET("/api/users/search", RequireAuth(deps.Auth), SearchUsersHandler(deps.Permissions))
	r.GET("/api/shared-with-me", RequireAuth(deps.Auth), SharedWithMeHandler(deps.Permissions))

	r.GET("/api/share/:token", OptionalAuth(deps.Auth), PublicShareInfoHandler(deps.Shares))
	r.POST("/api/share/:token/browse", OptionalAuth(deps.Auth), BrowsePublicFolderShareHandler(deps.Shares))
	r.POST("/api/share/:token/verify-password", OptionalAuth(deps.Auth), VerifySharePasswordHandler(deps.Shares))
	r.GET("/api/share/:token/preview", OptionalAuth(deps.Auth), SharePreviewHandler(deps.Shares))
	r.POST("/api/share/:token/download", OptionalAuth(deps.Auth), PublicShareDownloadHandler(deps.Shares))
	r.POST("/api/share/:token/zip", OptionalAuth(deps.Auth), PublicFolderZipHandler(deps.Shares))
	r.POST("/api/share/:token/files/:fileId/download", OptionalAuth(deps.Auth), PublicFolderFileDownloadHandler(deps.Shares))

	// The signaling hub is open (LocalSend-style) by default; SEND_REQUIRE_AUTH
	// puts it behind a session so only logged-in users can join the roster.
	sendHandlers := []gin.HandlerFunc{SendWSHandler(deps.SendHub, deps.AllowedOrigins)}
	if deps.SendRequireAuth {
		sendHandlers = append([]gin.HandlerFunc{RequireAuth(deps.Auth)}, sendHandlers...)
	}
	r.GET("/api/send/ws", sendHandlers...)

	return r
}
