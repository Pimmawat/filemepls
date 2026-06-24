package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"filemepls/internal/usecase"
)

// SearchUsersHandler looks up users by email substring for the "assign
// permission" picker. An empty query returns an empty list rather than
// every user in the system.
func SearchUsersHandler(perms *usecase.PermissionService) gin.HandlerFunc {
	return func(c *gin.Context) {
		query := c.Query("q")
		if query == "" {
			c.JSON(http.StatusOK, []userSummaryDTO{})
			return
		}

		users, err := perms.SearchUsers(c.Request.Context(), userIDFromContext(c), query)
		if err != nil {
			respondErr(c, err)
			return
		}
		c.JSON(http.StatusOK, toUserSummaryDTOs(users))
	}
}

func GrantFileAccessHandler(perms *usecase.PermissionService) gin.HandlerFunc {
	return func(c *gin.Context) {
		fileID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file id"})
			return
		}

		var req grantAccessRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		grant, grantee, err := perms.GrantFileAccess(c.Request.Context(), userIDFromContext(c), fileID, req.Email)
		if err != nil {
			respondErr(c, err)
			return
		}
		c.JSON(http.StatusCreated, toAccessGrantDTO(usecase.AccessGrantView{Grant: grant, Grantee: grantee}))
	}
}

func ListFileGrantsHandler(perms *usecase.PermissionService) gin.HandlerFunc {
	return func(c *gin.Context) {
		fileID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file id"})
			return
		}

		list, err := perms.ListFileGrants(c.Request.Context(), userIDFromContext(c), fileID)
		if err != nil {
			respondErr(c, err)
			return
		}
		c.JSON(http.StatusOK, toAccessGrantDTOs(list))
	}
}

func GrantFolderAccessHandler(perms *usecase.PermissionService) gin.HandlerFunc {
	return func(c *gin.Context) {
		folderID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid folder id"})
			return
		}

		var req grantAccessRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		grant, grantee, err := perms.GrantFolderAccess(c.Request.Context(), userIDFromContext(c), folderID, req.Email)
		if err != nil {
			respondErr(c, err)
			return
		}
		c.JSON(http.StatusCreated, toAccessGrantDTO(usecase.AccessGrantView{Grant: grant, Grantee: grantee}))
	}
}

func ListFolderGrantsHandler(perms *usecase.PermissionService) gin.HandlerFunc {
	return func(c *gin.Context) {
		folderID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid folder id"})
			return
		}

		list, err := perms.ListFolderGrants(c.Request.Context(), userIDFromContext(c), folderID)
		if err != nil {
			respondErr(c, err)
			return
		}
		c.JSON(http.StatusOK, toAccessGrantDTOs(list))
	}
}

func RevokeGrantHandler(perms *usecase.PermissionService) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid grant id"})
			return
		}

		if err := perms.RevokeGrant(c.Request.Context(), userIDFromContext(c), id); err != nil {
			respondErr(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

// SharedWithMeHandler lists the top-level files and folders directly
// granted to the caller.
func SharedWithMeHandler(perms *usecase.PermissionService) gin.HandlerFunc {
	return func(c *gin.Context) {
		files, folders, err := perms.ListSharedWithMe(c.Request.Context(), userIDFromContext(c))
		if err != nil {
			respondErr(c, err)
			return
		}
		c.JSON(http.StatusOK, sharedWithMeDTO{Files: toFileDTOs(files), Folders: toFolderDTOs(folders)})
	}
}
