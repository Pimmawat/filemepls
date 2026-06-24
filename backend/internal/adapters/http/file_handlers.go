package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"filemepls/internal/usecase"
)

func UploadHandler(files *usecase.FileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		fileHeader, err := c.FormFile("file")
		if err != nil {
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

		stream, offset, contentLength, totalSize, partial, mime, name, createdAt, err := files.DownloadRange(
			c.Request.Context(), userIDFromContext(c), id, c.GetHeader("Range"))
		if err != nil {
			respondErr(c, err)
			return
		}

		c.Header("Content-Disposition", contentDisposition(name, createdAt))
		writeDownloadResponse(c, stream, offset, contentLength, totalSize, partial, mime)
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
