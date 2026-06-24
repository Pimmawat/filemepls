package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Folder is a purely organizational grouping of files/folders. Storage
// stays content-addressed by hash regardless of folder structure (see
// File.StorageKey) — folder names never touch the filesystem, so the
// validation below is about display/DB safety only, not path safety.
type Folder struct {
	ID        uuid.UUID
	Name      string
	ParentID  *uuid.UUID // nil = root level
	OwnerID   uuid.UUID
	CreatedAt time.Time
}

func NewFolder(name string, parentID *uuid.UUID, ownerID uuid.UUID) (*Folder, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrEmptyFolderName
	}
	if strings.ContainsAny(name, "/\\") {
		return nil, ErrInvalidFolderName
	}

	return &Folder{
		ID:        uuid.New(),
		Name:      name,
		ParentID:  parentID,
		OwnerID:   ownerID,
		CreatedAt: time.Now().UTC(),
	}, nil
}

func (f *Folder) EnsureOwnedBy(userID uuid.UUID) error {
	if f.OwnerID != userID {
		return ErrNotOwner
	}
	return nil
}
