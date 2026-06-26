package http

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"filemepls/internal/domain"
	"filemepls/internal/usecase"
)

func CreateShareHandler(shares *usecase.ShareService) gin.HandlerFunc {
	return func(c *gin.Context) {
		fileID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file id"})
			return
		}

		var req createShareRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		visibility := domain.Visibility(req.Visibility)
		share, err := shares.CreateShareLink(c.Request.Context(), userIDFromContext(c), fileID, visibility, req.ExpiresAt, req.MaxDownloads, req.Password)
		if err != nil {
			respondErr(c, err)
			return
		}
		c.JSON(http.StatusCreated, toShareLinkDTO(share))
	}
}

// ListSharesHandler lists every share link created for a file, so a link
// created in an earlier session can be found again and revoked later.
func ListSharesHandler(shares *usecase.ShareService) gin.HandlerFunc {
	return func(c *gin.Context) {
		fileID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file id"})
			return
		}

		list, err := shares.ListSharesForFile(c.Request.Context(), userIDFromContext(c), fileID)
		if err != nil {
			respondErr(c, err)
			return
		}
		c.JSON(http.StatusOK, toShareLinkDTOs(list))
	}
}

func CreateFolderShareHandler(shares *usecase.ShareService) gin.HandlerFunc {
	return func(c *gin.Context) {
		folderID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid folder id"})
			return
		}

		var req createShareRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		visibility := domain.Visibility(req.Visibility)
		share, err := shares.CreateFolderShareLink(c.Request.Context(), userIDFromContext(c), folderID, visibility, req.ExpiresAt, req.MaxDownloads, req.Password)
		if err != nil {
			respondErr(c, err)
			return
		}
		c.JSON(http.StatusCreated, toShareLinkDTO(share))
	}
}

func ListFolderSharesHandler(shares *usecase.ShareService) gin.HandlerFunc {
	return func(c *gin.Context) {
		folderID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid folder id"})
			return
		}

		list, err := shares.ListSharesForFolder(c.Request.Context(), userIDFromContext(c), folderID)
		if err != nil {
			respondErr(c, err)
			return
		}
		c.JSON(http.StatusOK, toShareLinkDTOs(list))
	}
}

func RevokeShareHandler(shares *usecase.ShareService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid share id"})
			return
		}

		if err := shares.RevokeShareLink(c.Request.Context(), userIDFromContext(c), id); err != nil {
			respondErr(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// PublicShareInfoHandler reports a share link's redemption state without
// granting access or requiring auth, so the frontend can render the right
// UI (expired/limit-reached/needs-password/ready) before any download
// attempt. For an "ok" folder share, embeds the share's own root browse
// data directly (subfolders/files) — reaching "ok" already means no
// password is required.
func PublicShareInfoHandler(shares *usecase.ShareService) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Param("token")
		requesterID := optionalUserIDFromContext(c)

		share, f, folder, err := shares.GetPublicShare(c.Request.Context(), token, requesterID)
		switch {
		case err == nil:
			if share.RequiresPassword() {
				c.JSON(http.StatusOK, publicShareStateResponse{Status: "needs_password", TargetType: string(share.TargetType)})
				return
			}
			resp := publicShareStateResponse{Status: "ok", TargetType: string(share.TargetType)}
			if share.TargetType == domain.ShareTargetFile {
				dto := toFileDTO(f)
				resp.File = &dto
			} else {
				browse, err := shares.BrowsePublicFolder(c.Request.Context(), token, nil, requesterID)
				if err != nil {
					respondErr(c, err)
					return
				}
				dto := toBrowseResultDTO(browse)
				resp.Folder = &dto
			}
			_ = folder // folder identity already embedded via resp.Folder for the folder case
			c.JSON(http.StatusOK, resp)
		case errors.Is(err, domain.ErrShareExpired):
			c.JSON(http.StatusOK, publicShareStateResponse{Status: "expired"})
		case errors.Is(err, domain.ErrDownloadLimitHit):
			c.JSON(http.StatusOK, publicShareStateResponse{Status: "limit_reached"})
		case errors.Is(err, domain.ErrAuthRequired):
			c.JSON(http.StatusOK, publicShareStateResponse{Status: "auth_required"})
		case errors.Is(err, domain.ErrNotFound):
			c.JSON(http.StatusNotFound, publicShareStateResponse{Status: "not_found"})
		default:
			respondErr(c, err)
		}
	}
}

// VerifySharePasswordHandler checks a candidate password with no download
// side effects, so the frontend can pre-flight-check it via fetch before
// ever submitting/navigating to the real download form — a wrong password
// then surfaces as a normal in-app error instead of the browser navigating
// to and rendering a raw JSON error response in place of the whole app.
func VerifySharePasswordHandler(shares *usecase.ShareService) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Param("token")

		var req verifyPasswordRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := shares.VerifySharePassword(c.Request.Context(), token, req.Password, optionalUserIDFromContext(c)); err != nil {
			respondErr(c, err)
			return
		}
		c.Status(http.StatusOK)
	}
}

type browseFolderShareRequest struct {
	FolderID *uuid.UUID `json:"folderId"`
	Password string     `json:"password"`
}

// BrowsePublicFolderShareHandler browses a folder-target share, optionally
// descending into one of its subfolders (folderId in the body). A POST
// (not GET+query string) so a required password never appears in a URL,
// server log, or browser history. Re-verifies the password on every call —
// there's no session concept for anonymous share access, consistent with
// how the existing single-file share download already re-checks the
// password on every request rather than issuing any kind of session token.
func BrowsePublicFolderShareHandler(shares *usecase.ShareService) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Param("token")

		var req browseFolderShareRequest
		_ = c.ShouldBindJSON(&req) // an empty body is valid (root, no password)

		requesterID := optionalUserIDFromContext(c)
		if err := shares.VerifySharePassword(c.Request.Context(), token, req.Password, requesterID); err != nil {
			respondErr(c, err)
			return
		}

		browse, err := shares.BrowsePublicFolder(c.Request.Context(), token, req.FolderID, requesterID)
		if err != nil {
			respondErr(c, err)
			return
		}
		c.JSON(http.StatusOK, toBrowseResultDTO(browse))
	}
}

// PublicShareDownloadHandler redeems a file-target share token, no auth
// required. Accepts either a JSON body or a form-urlencoded body (the
// latter for the real browser <form method="POST"> the frontend submits,
// which streams the response as a native download).
func PublicShareDownloadHandler(shares *usecase.ShareService) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Param("token")

		var req redeemShareRequest
		_ = c.ShouldBind(&req) // an empty/missing password field is valid (no-password shares)

		stream, offset, contentLength, totalSize, partial, mime, file, err := shares.RedeemShareDownload(
			c.Request.Context(), token, req.Password, c.GetHeader("Range"), optionalUserIDFromContext(c))
		if err != nil {
			respondErr(c, err)
			return
		}

		c.Header("Content-Disposition", contentDisposition(file.Name, file.CreatedAt))
		writeDownloadResponse(c, stream, offset, contentLength, totalSize, partial, mime)
	}
}

// PublicFolderFileDownloadHandler downloads a single file living inside a
// publicly shared folder, no auth required — the per-file counterpart to
// PublicFolderZipHandler. Same form-POST convention (password never in a
// URL) and the same validate-before-respond ordering.
func PublicFolderFileDownloadHandler(shares *usecase.ShareService) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Param("token")
		fileID, err := uuid.Parse(c.Param("fileId"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file id"})
			return
		}

		var req redeemShareRequest
		_ = c.ShouldBind(&req)

		stream, offset, contentLength, totalSize, partial, mime, file, err := shares.RedeemFolderFileDownload(
			c.Request.Context(), token, fileID, req.Password, c.GetHeader("Range"), optionalUserIDFromContext(c))
		if err != nil {
			respondErr(c, err)
			return
		}

		c.Header("Content-Disposition", contentDisposition(file.Name, file.CreatedAt))
		writeDownloadResponse(c, stream, offset, contentLength, totalSize, partial, mime)
	}
}

type zipFolderShareRequest struct {
	FolderID *uuid.UUID `json:"folderId" form:"folderId"`
	Password string     `json:"password" form:"password"`
}

// PublicFolderZipHandler redeems a folder-target share token as a ZIP
// download, no auth required. Same form-POST convention as
// PublicShareDownloadHandler (password never in a URL). Validation happens
// entirely before any header/byte is written — see
// ShareService.PrepareFolderShareZip — so a wrong password or expired/
// limit-hit share still gets a normal JSON error response, not a 200
// committed ahead of the check.
func PublicFolderZipHandler(shares *usecase.ShareService) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Param("token")

		var req zipFolderShareRequest
		_ = c.ShouldBind(&req)

		ownerID, folderID, folderName, err := shares.PrepareFolderShareZip(c.Request.Context(), token, req.Password, req.FolderID, optionalUserIDFromContext(c))
		if err != nil {
			respondErr(c, err)
			return
		}

		c.Header("Content-Disposition", contentDisposition(folderName+".zip", time.Now()))
		c.Header("Content-Type", "application/zip")
		if err := shares.StreamZip(c.Request.Context(), ownerID, folderID, c.Writer); err != nil {
			log.Printf("folder zip stream error: %v", err)
		}
	}
}
