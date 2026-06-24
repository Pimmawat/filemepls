package ports

import (
	"context"

	"filemepls/internal/domain"
)

// BlobRepository persists deduped, content-addressed blob records (the
// actual bytes on disk, keyed by hash). FindByHash returns
// domain.ErrNotFound instead of nil, nil when no record matches.
type BlobRepository interface {
	Save(ctx context.Context, b *domain.Blob) error
	FindByHash(ctx context.Context, hash string) (*domain.Blob, error)
	Delete(ctx context.Context, hash string) error
}
