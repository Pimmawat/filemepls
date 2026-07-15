package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"filemepls/internal/domain"
	"filemepls/internal/ports"
)

var _ ports.BlobRepository = (*BlobRepository)(nil)

type BlobRepository struct {
	pool *pgxpool.Pool
}

func NewBlobRepository(pool *pgxpool.Pool) *BlobRepository {
	return &BlobRepository{pool: pool}
}

// Save is idempotent: two concurrent first-uploads of identical content both
// resolve their FindByHash lookups to "not found" and race to insert the same
// (content-addressed) hash. ON CONFLICT DO NOTHING lets the loser treat the
// row as already-present instead of failing the whole upload with a spurious
// primary-key violation.
func (r *BlobRepository) Save(ctx context.Context, b *domain.Blob) error {
	const q = `INSERT INTO blobs (hash, size, mime, created_at) VALUES ($1, $2, $3, $4) ON CONFLICT (hash) DO NOTHING`
	if _, err := r.pool.Exec(ctx, q, b.Hash, b.Size, b.Mime, b.CreatedAt); err != nil {
		return fmt.Errorf("postgres: save blob: %w", err)
	}
	return nil
}

func (r *BlobRepository) FindByHash(ctx context.Context, hash string) (*domain.Blob, error) {
	const q = `SELECT hash, size, mime, created_at FROM blobs WHERE hash = $1`
	row := r.pool.QueryRow(ctx, q, hash)

	var b domain.Blob
	if err := row.Scan(&b.Hash, &b.Size, &b.Mime, &b.CreatedAt); err != nil {
		return nil, mapErr(err)
	}
	return &b, nil
}

func (r *BlobRepository) Delete(ctx context.Context, hash string) error {
	const q = `DELETE FROM blobs WHERE hash = $1`
	if _, err := r.pool.Exec(ctx, q, hash); err != nil {
		return fmt.Errorf("postgres: delete blob: %w", err)
	}
	return nil
}

func (r *BlobRepository) ListOrphanHashes(ctx context.Context, olderThan time.Time) ([]string, error) {
	const q = `
		SELECT b.hash
		FROM blobs b
		WHERE b.created_at < $1
		  AND NOT EXISTS (SELECT 1 FROM files f WHERE f.hash = b.hash)`
	rows, err := r.pool.Query(ctx, q, olderThan)
	if err != nil {
		return nil, fmt.Errorf("postgres: list orphan blobs: %w", err)
	}
	defer rows.Close()

	var hashes []string
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			return nil, fmt.Errorf("postgres: scan orphan blob: %w", err)
		}
		hashes = append(hashes, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list orphan blobs: %w", err)
	}
	return hashes, nil
}
