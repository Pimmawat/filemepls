package postgres

import (
	"context"
	"fmt"

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

func (r *BlobRepository) Save(ctx context.Context, b *domain.Blob) error {
	const q = `INSERT INTO blobs (hash, size, mime, created_at) VALUES ($1, $2, $3, $4)`
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
