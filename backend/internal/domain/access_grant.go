package domain

import (
	"time"

	"github.com/google/uuid"
)

// AccessTargetType discriminates which of FileID/FolderID an AccessGrant
// targets — exactly one is ever set.
type AccessTargetType string

const (
	AccessTargetFile   AccessTargetType = "file"
	AccessTargetFolder AccessTargetType = "folder"
)

// AccessGrant is view-only access to a file or folder, given by its owner
// to another user (the grantee). A grant on a folder cascades to every
// file and subfolder inside it.
type AccessGrant struct {
	ID         uuid.UUID
	TargetType AccessTargetType
	FileID     *uuid.UUID // set iff TargetType == AccessTargetFile
	FolderID   *uuid.UUID // set iff TargetType == AccessTargetFolder
	GranteeID  uuid.UUID
	CreatedBy  uuid.UUID
	CreatedAt  time.Time
}

func NewFileAccessGrant(fileID, granteeID, createdBy uuid.UUID) *AccessGrant {
	return &AccessGrant{
		ID:         uuid.New(),
		TargetType: AccessTargetFile,
		FileID:     &fileID,
		GranteeID:  granteeID,
		CreatedBy:  createdBy,
		CreatedAt:  time.Now().UTC(),
	}
}

func NewFolderAccessGrant(folderID, granteeID, createdBy uuid.UUID) *AccessGrant {
	return &AccessGrant{
		ID:         uuid.New(),
		TargetType: AccessTargetFolder,
		FolderID:   &folderID,
		GranteeID:  granteeID,
		CreatedBy:  createdBy,
		CreatedAt:  time.Now().UTC(),
	}
}
