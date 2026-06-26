package http

import (
	"net/http"
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
}

func NewRouter(deps Deps) *gin.Engine {
	//gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(CORS(deps.AllowedOrigins))
	r.MaxMultipartMemory = 1 << 20 // 1MB; the usecase's own io.LimitReader is the real ceiling

	r.GET("/healthz", func(c *gin.Context) { c.Status(http.StatusOK) })

	authGroup := r.Group("/api/auth")
	authGroup.GET("/:provider/authorize", AuthorizeHandler(deps.Auth))
	authGroup.GET("/:provider/callback", CallbackHandler(deps.Auth, deps.FrontendBaseURL, deps.DefaultLocale, deps.JWTTTL, deps.CookieDomain))
	authGroup.POST("/register", RegisterHandler(deps.Auth, deps.JWTTTL, deps.CookieDomain))
	authGroup.POST("/login", LoginHandler(deps.Auth, deps.JWTTTL, deps.CookieDomain))
	authGroup.POST("/logout", LogoutHandler(deps.CookieDomain))
	authGroup.GET("/me", RequireAuth(deps.Auth), MeHandler(deps.Auth))

	filesGroup := r.Group("/api/files")
	filesGroup.Use(RequireAuth(deps.Auth))
	filesGroup.POST("", UploadHandler(deps.Files))
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
	r.POST("/api/share/:token/download", OptionalAuth(deps.Auth), PublicShareDownloadHandler(deps.Shares))
	r.POST("/api/share/:token/zip", OptionalAuth(deps.Auth), PublicFolderZipHandler(deps.Shares))
	r.POST("/api/share/:token/files/:fileId/download", OptionalAuth(deps.Auth), PublicFolderFileDownloadHandler(deps.Shares))

	r.GET("/api/send/ws", SendWSHandler(deps.SendHub, deps.AllowedOrigins))

	return r
}
