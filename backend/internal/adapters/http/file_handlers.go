package http

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"filemepls/internal/usecase"
)

// maxConcurrentUploads bounds how many uploads may be spooling to disk at once,
// so a burst of large concurrent uploads can't exhaust disk/memory.
const maxConcurrentUploads = 16

// multipartOverhead is slack added on top of MaxUploadSize for the multipart
// envelope (boundaries, the parentId field, headers) so a legitimately
// max-sized file isn't rejected by the body cap.
const multipartOverhead = 8 << 20 // 8MB

func UploadHandler(files *usecase.FileService, maxUploadSize int64) gin.HandlerFunc {
	sem := make(chan struct{}, maxConcurrentUploads)
	return func(c *gin.Context) {
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
		default:
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "server busy, retry shortly"})
			return
		}

		// Cap the request body before parsing so an oversized multipart form is
		// rejected as it streams in, rather than after the whole thing has
		// already been spooled to a temp file on disk.
		if maxUploadSize > 0 {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadSize+multipartOverhead)
		}

		fileHeader, err := c.FormFile("file")
		if err != nil {
			if strings.Contains(err.Error(), "too large") {
				c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file too large"})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing \"file\" field"})
			return
		}

		parentID, err := parseOptionalUUID(c.PostForm("parentId"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid parentId"})
			return
		}

		f, err := fileHeader.Open()
		if err != nil {
			respondErr(c, err)
			return
		}
		defer func() { _ = f.Close() }()

		declaredMime := fileHeader.Header.Get("Content-Type")
		created, err := files.Upload(c.Request.Context(), userIDFromContext(c), declaredMime, fileHeader.Filename, parentID, f)
		if err != nil {
			respondErr(c, err)
			return
		}
		c.JSON(http.StatusCreated, toFileDTO(created))
	}
}

func ListHandler(files *usecase.FileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		list, err := files.List(c.Request.Context(), userIDFromContext(c))
		if err != nil {
			respondErr(c, err)
			return
		}
		c.JSON(http.StatusOK, toFileDTOs(list))
	}
}

func MetadataHandler(files *usecase.FileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file id"})
			return
		}

		f, err := files.GetMetadata(c.Request.Context(), userIDFromContext(c), id)
		if err != nil {
			respondErr(c, err)
			return
		}
		c.JSON(http.StatusOK, toFileDTO(f))
	}
}

func DeleteHandler(files *usecase.FileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file id"})
			return
		}

		if err := files.Delete(c.Request.Context(), userIDFromContext(c), id); err != nil {
			respondErr(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func DownloadHandler(files *usecase.FileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file id"})
			return
		}

		stream, offset, contentLength, totalSize, partial, mime, name, _, err := files.DownloadRange(
			c.Request.Context(), userIDFromContext(c), id, c.GetHeader("Range"))
		if err != nil {
			respondErr(c, err)
			return
		}

		c.Header("Content-Disposition", contentDisposition(name))
		writeDownloadResponse(c, stream, offset, contentLength, totalSize, partial, mime, etagFor(id))
	}
}

func MoveFileHandler(folders *usecase.FolderService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file id"})
			return
		}

		var req moveRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := folders.MoveFile(c.Request.Context(), userIDFromContext(c), id, req.ParentID); err != nil {
			respondErr(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}
