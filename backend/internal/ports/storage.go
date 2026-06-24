package ports

import (
	"context"
	"io"
	"time"

	"filemepls/internal/domain"
)

// StoragePort abstracts file storage so the local filesystem adapter can
// later be swapped for MinIO/S3 without touching usecase code. Methods use
// io.Reader/io.ReadCloser (never []byte) so large uploads/downloads stream
// instead of being buffered fully in memory.
//
// Implementations must resolve domain.StorageKey to an on-disk path
// defensively (filepath.Clean + verify containment under the storage root)
// even though StorageKey is already safe-by-construction in the domain
// layer — defense in depth.
type StoragePort interface {
	Save(ctx context.Context, key domain.StorageKey, r io.Reader) (size int64, err error)
	Get(ctx context.Context, key domain.StorageKey) (io.ReadCloser, error)
	// GetRange returns a reader for the byte range [offset, offset+length) of
	// the object at key, plus the object's total size (for Content-Range
	// headers). If length <= 0, it returns from offset through EOF.
	GetRange(ctx context.Context, key domain.StorageKey, offset, length int64) (io.ReadCloser, int64, error)
	Delete(ctx context.Context, key domain.StorageKey) error
	Exists(ctx context.Context, key domain.StorageKey) (bool, error)
	// Rename atomically moves an object from oldKey to newKey, used to
	// promote a staged upload to its final content-addressed key.
	Rename(ctx context.Context, oldKey, newKey domain.StorageKey) error
	PresignedURL(ctx context.Context, key domain.StorageKey, expiresIn time.Duration) (string, error)
}
