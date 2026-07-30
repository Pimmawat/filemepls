package http

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"filemepls/internal/domain"
)

// statusFor maps domain sentinel errors to HTTP status codes. Anything
// unrecognized is treated as an internal error.
func statusFor(err error) int {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, domain.ErrNotOwner):
		return http.StatusForbidden
	case errors.Is(err, domain.ErrFileTooLarge):
		return http.StatusRequestEntityTooLarge
	case errors.Is(err, domain.ErrUnsupportedMime):
		return http.StatusUnsupportedMediaType
	case errors.Is(err, domain.ErrShareExpired), errors.Is(err, domain.ErrDownloadLimitHit):
		return http.StatusGone
	case errors.Is(err, domain.ErrAlreadyGranted):
		return http.StatusConflict
	case errors.Is(err, domain.ErrInvalidPassword),
		errors.Is(err, domain.ErrPasswordRequired),
		errors.Is(err, domain.ErrAuthRequired),
		errors.Is(err, domain.ErrInvalidCredentials):
		return http.StatusUnauthorized
	case errors.Is(err, domain.ErrEmailAlreadyTaken):
		return http.StatusConflict
	case errors.Is(err, domain.ErrInvalidVisibility),
		errors.Is(err, domain.ErrInvalidSize),
		errors.Is(err, domain.ErrEmptyHash),
		errors.Is(err, domain.ErrInvalidHash),
		errors.Is(err, domain.ErrEmptyEmail),
		errors.Is(err, domain.ErrEmptyFolderName),
		errors.Is(err, domain.ErrInvalidFolderName),
		errors.Is(err, domain.ErrCyclicMove),
		errors.Is(err, domain.ErrShareTargetRequired),
		errors.Is(err, domain.ErrShareTargetMismatch),
		errors.Is(err, domain.ErrEmptyPasswordHash),
		errors.Is(err, domain.ErrWeakPassword):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// respondErr maps err to an HTTP status and writes a JSON error body. 500s
// never leak the real error text to the client; the real error is logged
// server-side instead.
func respondErr(c *gin.Context, err error) {
	status := statusFor(err)
	if status == http.StatusInternalServerError {
		log.Printf("internal error: %v", err)
		c.JSON(status, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(status, gin.H{"error": err.Error()})
}
