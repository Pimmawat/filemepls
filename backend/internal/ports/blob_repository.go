package ports

import (
	"context"
	"time"

	"filemepls/internal/domain"
)

// BlobRepository persists deduped, content-addressed blob records (the
// actual bytes on disk, keyed by hash). FindByHash returns
// domain.ErrNotFound instead of nil, nil when no record matches.
type BlobRepository interface {
	Save(ctx context.Context, b *domain.Blob) error
	FindByHash(ctx context.Context, hash string) (*domain.Blob, error)
	Delete(ctx context.Context, hash string) error
	// ListOrphanHashes returns hashes of blobs that no File row references and
	// that were created before olderThan. The age filter is a grace period: a
	// blob is written and its File row saved as two steps, so a just-promoted
	// blob would momentarily look orphaned — only sweeping older ones avoids
	// deleting bytes an in-flight upload is about to reference.
	ListOrphanHashes(ctx context.Context, olderThan time.Time) ([]string, error)
}
