package ports

import (
	"context"

	"github.com/google/uuid"

	"filemepls/internal/domain"
)

// FileRepository persists per-owner file records. Lookups return
// domain.ErrNotFound instead of nil, nil when no record matches. Dedup of
// the underlying bytes is handled separately by BlobRepository: many File
// rows (different owners) may share the same hash.
type FileRepository interface {
	Save(ctx context.Context, f *domain.File) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.File, error)
	ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]*domain.File, error)
	// ListByParent returns the direct file children of parentID, owner-
	// scoped. parentID nil lists root-level files.
	ListByParent(ctx context.Context, ownerID uuid.UUID, parentID *uuid.UUID) ([]*domain.File, error)
	UpdateParent(ctx context.Context, fileID uuid.UUID, parentID *uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
	// CountByHash reports how many File rows (across all owners) reference
	// hash, used to decide whether the underlying Blob can be deleted.
	CountByHash(ctx context.Context, hash string) (int64, error)
}
