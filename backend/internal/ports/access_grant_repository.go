package ports

import (
	"context"

	"github.com/google/uuid"

	"filemepls/internal/domain"
)

// AccessGrantRepository persists view-only access grants to other users.
// Lookups return domain.ErrNotFound instead of nil, nil when no record
// matches.
type AccessGrantRepository interface {
	Save(ctx context.Context, g *domain.AccessGrant) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.AccessGrant, error)
	// ListByFile returns all grants for fileID, most recent first.
	ListByFile(ctx context.Context, fileID uuid.UUID) ([]*domain.AccessGrant, error)
	// ListByFolder returns all grants for folderID, most recent first.
	ListByFolder(ctx context.Context, folderID uuid.UUID) ([]*domain.AccessGrant, error)
	Delete(ctx context.Context, id uuid.UUID) error
	// HasFileAccess reports whether granteeID has a direct grant on fileID.
	HasFileAccess(ctx context.Context, fileID, granteeID uuid.UUID) (bool, error)
	// HasFolderAccess reports whether granteeID has a direct grant on
	// folderID.
	HasFolderAccess(ctx context.Context, folderID, granteeID uuid.UUID) (bool, error)
	// ListSharedFiles returns files directly granted to granteeID.
	ListSharedFiles(ctx context.Context, granteeID uuid.UUID) ([]*domain.File, error)
	// ListSharedFolders returns folders directly granted to granteeID.
	ListSharedFolders(ctx context.Context, granteeID uuid.UUID) ([]*domain.Folder, error)
}
