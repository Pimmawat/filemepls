package ports

import (
	"context"

	"github.com/google/uuid"

	"filemepls/internal/domain"
)

// FolderRepository persists folders. Lookups return domain.ErrNotFound
// instead of nil, nil when no record matches.
type FolderRepository interface {
	Save(ctx context.Context, f *domain.Folder) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Folder, error)
	// ListChildren returns the direct subfolders of parentID, owner-scoped.
	// parentID nil lists root-level folders.
	ListChildren(ctx context.Context, ownerID uuid.UUID, parentID *uuid.UUID) ([]*domain.Folder, error)
	UpdateParent(ctx context.Context, folderID uuid.UUID, parentID *uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
}
