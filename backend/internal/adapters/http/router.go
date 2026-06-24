package http

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"filemepls/internal/usecase"
)

type Deps struct {
	Files           *usecase.FileService
	Folders         *usecase.FolderService
	Shares          *usecase.ShareService
	Auth            *usecase.AuthService
	AllowedOrigins  []string
	FrontendBaseURL string
	DefaultLocale   string
	JWTTTL          time.Duration
}

func NewRouter(deps Deps) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(CORS(deps.AllowedOrigins))
	r.MaxMultipartMemory = 1 << 20 // 1MB; the usecase's own io.LimitReader is the real ceiling

	r.GET("/healthz", func(c *gin.Context) { c.Status(http.StatusOK) })

	authGroup := r.Group("/api/auth")
	authGroup.GET("/:provider/authorize", AuthorizeHandler(deps.Auth))
	authGroup.GET("/:provider/callback", CallbackHandler(deps.Auth, deps.FrontendBaseURL, deps.DefaultLocale, deps.JWTTTL))
	authGroup.POST("/logout", LogoutHandler())
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

	foldersGroup := r.Group("/api/folders")
	foldersGroup.Use(RequireAuth(deps.Auth))
	foldersGroup.POST("", CreateFolderHandler(deps.Folders))
	foldersGroup.GET("/browse", BrowseFoldersHandler(deps.Folders))
	foldersGroup.DELETE("/:id", DeleteFolderHandler(deps.Folders))
	foldersGroup.PATCH("/:id/move", MoveFolderHandler(deps.Folders))
	foldersGroup.GET("/:id/download", FolderZipHandler(deps.Folders))
	foldersGroup.POST("/:id/shares", CreateFolderShareHandler(deps.Shares))
	foldersGroup.GET("/:id/shares", ListFolderSharesHandler(deps.Shares))

	r.DELETE("/api/shares/:id", RequireAuth(deps.Auth), RevokeShareHandler(deps.Shares))

	r.GET("/api/share/:token", PublicShareInfoHandler(deps.Shares))
	r.POST("/api/share/:token/browse", BrowsePublicFolderShareHandler(deps.Shares))
	r.POST("/api/share/:token/verify-password", VerifySharePasswordHandler(deps.Shares))
	r.POST("/api/share/:token/download", PublicShareDownloadHandler(deps.Shares))
	r.POST("/api/share/:token/zip", PublicFolderZipHandler(deps.Shares))
	r.POST("/api/share/:token/files/:fileId/download", PublicFolderFileDownloadHandler(deps.Shares))

	return r
}
