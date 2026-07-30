package domain

import (
	"crypto/subtle"
	"time"

	"github.com/google/uuid"
)

// ShareTargetType discriminates which of FileID/FolderID a ShareLink
// targets — exactly one is ever set.
type ShareTargetType string

const (
	ShareTargetFile   ShareTargetType = "file"
	ShareTargetFolder ShareTargetType = "folder"
)

type ShareLink struct {
	ID            uuid.UUID
	Token         string // opaque; generated via crypto/rand by the usecase layer, not here
	TargetType    ShareTargetType
	FileID        *uuid.UUID // set iff TargetType == ShareTargetFile
	FolderID      *uuid.UUID // set iff TargetType == ShareTargetFolder
	ExpiresAt     *time.Time
	PasswordHash  *string // nil = no password
	MaxDownloads  *int    // nil = unlimited
	DownloadCount int
	Visibility    Visibility
	CreatedAt     time.Time
}

func NewShareLinkForFile(token string, fileID uuid.UUID, visibility Visibility, expiresAt *time.Time, maxDownloads *int) (*ShareLink, error) {
	return newShareLink(token, ShareTargetFile, &fileID, nil, visibility, expiresAt, maxDownloads)
}

func NewShareLinkForFolder(token string, folderID uuid.UUID, visibility Visibility, expiresAt *time.Time, maxDownloads *int) (*ShareLink, error) {
	return newShareLink(token, ShareTargetFolder, nil, &folderID, visibility, expiresAt, maxDownloads)
}

func newShareLink(token string, targetType ShareTargetType, fileID, folderID *uuid.UUID, visibility Visibility, expiresAt *time.Time, maxDownloads *int) (*ShareLink, error) {
	if (fileID == nil) == (folderID == nil) {
		return nil, ErrShareTargetRequired
	}
	if !visibility.Valid() {
		return nil, ErrInvalidVisibility
	}

	return &ShareLink{
		ID:           uuid.New(),
		Token:        token,
		TargetType:   targetType,
		FileID:       fileID,
		FolderID:     folderID,
		ExpiresAt:    expiresAt,
		MaxDownloads: maxDownloads,
		Visibility:   visibility,
		CreatedAt:    time.Now().UTC(),
	}, nil
}

func (s *ShareLink) IsExpired(now time.Time) bool {
	return s.ExpiresAt != nil && now.After(*s.ExpiresAt)
}

func (s *ShareLink) IsDownloadLimitReached() bool {
	return s.MaxDownloads != nil && s.DownloadCount >= *s.MaxDownloads
}

func (s *ShareLink) RequiresPassword() bool {
	return s.PasswordHash != nil
}

// RecordDownload increments the download count. Callers (usecase layer)
// should check IsDownloadLimitReached before granting access; this method
// still guards against recording past the limit.
func (s *ShareLink) RecordDownload() error {
	if s.IsDownloadLimitReached() {
		return ErrDownloadLimitHit
	}
	s.DownloadCount++
	return nil
}

// MatchesToken compares a candidate token against the stored one in
// constant time, to avoid leaking timing information about share tokens.
func MatchesToken(stored, candidate string) bool {
	return subtle.ConstantTimeCompare([]byte(stored), []byte(candidate)) == 1
}
