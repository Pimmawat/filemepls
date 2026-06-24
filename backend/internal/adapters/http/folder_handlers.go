package http

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"filemepls/internal/usecase"
)

func CreateFolderHandler(folders *usecase.FolderService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req createFolderRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		folder, err := folders.Create(c.Request.Context(), userIDFromContext(c), req.Name, req.ParentID)
		if err != nil {
			respondErr(c, err)
			return
		}
		c.JSON(http.StatusCreated, toFolderDTO(folder))
	}
}

// BrowseFoldersHandler lists a folder's contents (or root, if id is
// omitted): its breadcrumb, subfolders, and files.
func BrowseFoldersHandler(folders *usecase.FolderService) gin.HandlerFunc {
	return func(c *gin.Context) {
		folderID, err := parseOptionalUUID(c.Query("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
			return
		}

		result, err := folders.Browse(c.Request.Context(), userIDFromContext(c), folderID)
		if err != nil {
			respondErr(c, err)
			return
		}
		c.JSON(http.StatusOK, toBrowseResultDTO(result))
	}
}

func DeleteFolderHandler(folders *usecase.FolderService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid folder id"})
			return
		}

		if err := folders.Delete(c.Request.Context(), userIDFromContext(c), id); err != nil {
			respondErr(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func MoveFolderHandler(folders *usecase.FolderService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid folder id"})
			return
		}

		var req moveRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := folders.MoveFolder(c.Request.Context(), userIDFromContext(c), id, req.ParentID); err != nil {
			respondErr(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// FolderZipHandler streams a ZIP of the owner's own folder. Validation
// (PrepareZip) happens entirely before any header/byte is written, so an
// unauthorized request still gets a normal JSON error response.
func FolderZipHandler(folders *usecase.FolderService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid folder id"})
			return
		}

		userID := userIDFromContext(c)
		folder, err := folders.PrepareZip(c.Request.Context(), userID, id)
		if err != nil {
			respondErr(c, err)
			return
		}

		c.Header("Content-Disposition", contentDisposition(folder.Name+".zip", time.Now()))
		c.Header("Content-Type", "application/zip")
		if err := folders.StreamZip(c.Request.Context(), folder.OwnerID, folder.ID, c.Writer); err != nil {
			log.Printf("folder zip stream error: %v", err)
		}
	}
}
