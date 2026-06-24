package http

import (
	"time"

	"github.com/google/uuid"

	"filemepls/internal/domain"
	"filemepls/internal/usecase"
)

type fileDTO struct {
	ID        uuid.UUID  `json:"id"`
	Hash      string     `json:"hash"`
	Size      int64      `json:"size"`
	Mime      string     `json:"mime"`
	Name      string     `json:"name"`
	OwnerID   uuid.UUID  `json:"ownerId"`
	ParentID  *uuid.UUID `json:"parentId"`
	CreatedAt time.Time  `json:"createdAt"`
}

func toFileDTO(f *domain.File) fileDTO {
	return fileDTO{ID: f.ID, Hash: f.Hash, Size: f.Size, Mime: f.Mime, Name: f.Name, OwnerID: f.OwnerID, ParentID: f.ParentID, CreatedAt: f.CreatedAt}
}

func toFileDTOs(files []*domain.File) []fileDTO {
	out := make([]fileDTO, len(files))
	for i, f := range files {
		out[i] = toFileDTO(f)
	}
	return out
}

type folderDTO struct {
	ID        uuid.UUID  `json:"id"`
	Name      string     `json:"name"`
	OwnerID   uuid.UUID  `json:"ownerId"`
	ParentID  *uuid.UUID `json:"parentId"`
	CreatedAt time.Time  `json:"createdAt"`
}

func toFolderDTO(f *domain.Folder) folderDTO {
	return folderDTO{ID: f.ID, Name: f.Name, OwnerID: f.OwnerID, ParentID: f.ParentID, CreatedAt: f.CreatedAt}
}

func toFolderDTOs(folders []*domain.Folder) []folderDTO {
	out := make([]folderDTO, len(folders))
	for i, f := range folders {
		out[i] = toFolderDTO(f)
	}
	return out
}

type browseResultDTO struct {
	Folder     *folderDTO  `json:"folder"`
	Breadcrumb []folderDTO `json:"breadcrumb"`
	Subfolders []folderDTO `json:"subfolders"`
	Files      []fileDTO   `json:"files"`
}

func toBrowseResultDTO(b *usecase.BrowseResult) browseResultDTO {
	var folder *folderDTO
	if b.Folder != nil {
		dto := toFolderDTO(b.Folder)
		folder = &dto
	}
	return browseResultDTO{
		Folder:     folder,
		Breadcrumb: toFolderDTOs(b.Breadcrumb),
		Subfolders: toFolderDTOs(b.Subfolders),
		Files:      toFileDTOs(b.Files),
	}
}

type userDTO struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"displayName"`
	Provider    string    `json:"provider"`
	AvatarURL   string    `json:"avatarUrl"`
	CreatedAt   time.Time `json:"createdAt"`
}

func toUserDTO(u *domain.User) userDTO {
	return userDTO{ID: u.ID, Email: u.Email, DisplayName: u.DisplayName, Provider: u.Provider, AvatarURL: u.AvatarURL, CreatedAt: u.CreatedAt}
}

type shareLinkDTO struct {
	ID            uuid.UUID  `json:"id"`
	Token         string     `json:"token"`
	TargetType    string     `json:"targetType"`
	FileID        *uuid.UUID `json:"fileId"`
	FolderID      *uuid.UUID `json:"folderId"`
	ExpiresAt     *time.Time `json:"expiresAt"`
	MaxDownloads  *int       `json:"maxDownloads"`
	DownloadCount int        `json:"downloadCount"`
	Visibility    string     `json:"visibility"`
	CreatedAt     time.Time  `json:"createdAt"`
}

func toShareLinkDTO(s *domain.ShareLink) shareLinkDTO {
	return shareLinkDTO{
		ID:            s.ID,
		Token:         s.Token,
		TargetType:    string(s.TargetType),
		FileID:        s.FileID,
		FolderID:      s.FolderID,
		ExpiresAt:     s.ExpiresAt,
		MaxDownloads:  s.MaxDownloads,
		DownloadCount: s.DownloadCount,
		Visibility:    string(s.Visibility),
		CreatedAt:     s.CreatedAt,
	}
}

func toShareLinkDTOs(shares []*domain.ShareLink) []shareLinkDTO {
	out := make([]shareLinkDTO, len(shares))
	for i, s := range shares {
		out[i] = toShareLinkDTO(s)
	}
	return out
}

type createShareRequest struct {
	Visibility   string     `json:"visibility" binding:"required"`
	ExpiresAt    *time.Time `json:"expiresAt"`
	MaxDownloads *int       `json:"maxDownloads"`
	Password     string     `json:"password"`
}

// redeemShareRequest binds either a JSON body (programmatic API use) or a
// form-urlencoded body (the real browser <form method="POST"> submission
// the frontend uses for password-protected downloads, which streams the
// response as a native download rather than buffering it in JS).
type redeemShareRequest struct {
	Password string `json:"password" form:"password"`
}

// verifyPasswordRequest is JSON-only: the password pre-verify endpoint is
// always called via fetch from the frontend, never a real form submission.
type verifyPasswordRequest struct {
	Password string `json:"password"`
}

type publicShareStateResponse struct {
	Status     string           `json:"status"` // "ok" | "expired" | "limit_reached" | "needs_password" | "not_found"
	TargetType string           `json:"targetType,omitempty"`
	File       *fileDTO         `json:"file,omitempty"`
	Folder     *browseResultDTO `json:"folder,omitempty"`
}

type createFolderRequest struct {
	Name     string     `json:"name" binding:"required"`
	ParentID *uuid.UUID `json:"parentId"`
}

type moveRequest struct {
	ParentID *uuid.UUID `json:"parentId"`
}

type userSummaryDTO struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"displayName"`
	AvatarURL   string    `json:"avatarUrl"`
}

func toUserSummaryDTO(u *domain.User) userSummaryDTO {
	return userSummaryDTO{ID: u.ID, Email: u.Email, DisplayName: u.DisplayName, AvatarURL: u.AvatarURL}
}

func toUserSummaryDTOs(users []*domain.User) []userSummaryDTO {
	out := make([]userSummaryDTO, len(users))
	for i, u := range users {
		out[i] = toUserSummaryDTO(u)
	}
	return out
}

type accessGrantDTO struct {
	ID               uuid.UUID  `json:"id"`
	TargetType       string     `json:"targetType"`
	FileID           *uuid.UUID `json:"fileId"`
	FolderID         *uuid.UUID `json:"folderId"`
	GranteeID        uuid.UUID  `json:"granteeId"`
	GranteeEmail     string     `json:"granteeEmail"`
	GranteeName      string     `json:"granteeName"`
	GranteeAvatarURL string     `json:"granteeAvatarUrl"`
	CreatedAt        time.Time  `json:"createdAt"`
}

func toAccessGrantDTO(v usecase.AccessGrantView) accessGrantDTO {
	return accessGrantDTO{
		ID:               v.Grant.ID,
		TargetType:       string(v.Grant.TargetType),
		FileID:           v.Grant.FileID,
		FolderID:         v.Grant.FolderID,
		GranteeID:        v.Grant.GranteeID,
		GranteeEmail:     v.Grantee.Email,
		GranteeName:      v.Grantee.DisplayName,
		GranteeAvatarURL: v.Grantee.AvatarURL,
		CreatedAt:        v.Grant.CreatedAt,
	}
}

func toAccessGrantDTOs(views []usecase.AccessGrantView) []accessGrantDTO {
	out := make([]accessGrantDTO, len(views))
	for i, v := range views {
		out[i] = toAccessGrantDTO(v)
	}
	return out
}

type grantAccessRequest struct {
	Email string `json:"email" binding:"required"`
}

type sharedWithMeDTO struct {
	Files   []fileDTO   `json:"files"`
	Folders []folderDTO `json:"folders"`
}
