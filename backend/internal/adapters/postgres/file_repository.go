package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"filemepls/internal/domain"
	"filemepls/internal/ports"
)

var _ ports.FileRepository = (*FileRepository)(nil)

type FileRepository struct {
	pool *pgxpool.Pool
}

func NewFileRepository(pool *pgxpool.Pool) *FileRepository {
	return &FileRepository{pool: pool}
}

func (r *FileRepository) Save(ctx context.Context, f *domain.File) error {
	const q = `INSERT INTO files (id, hash, size, mime, name, owner_id, parent_id, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	if _, err := r.pool.Exec(ctx, q, f.ID, f.Hash, f.Size, f.Mime, f.Name, f.OwnerID, f.ParentID, f.CreatedAt); err != nil {
		return fmt.Errorf("postgres: save file: %w", err)
	}
	return nil
}

func (r *FileRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.File, error) {
	const q = `SELECT id, hash, size, mime, name, owner_id, parent_id, created_at FROM files WHERE id = $1`
	row := r.pool.QueryRow(ctx, q, id)
	return scanFile(row)
}

func (r *FileRepository) ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]*domain.File, error) {
	const q = `SELECT id, hash, size, mime, name, owner_id, parent_id, created_at FROM files WHERE owner_id = $1 ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, q, ownerID)
	if err != nil {
		return nil, fmt.Errorf("postgres: list files: %w", err)
	}
	defer rows.Close()

	var files []*domain.File
	for rows.Next() {
		f, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list files: %w", err)
	}
	return files, nil
}

func (r *FileRepository) ListByParent(ctx context.Context, ownerID uuid.UUID, parentID *uuid.UUID) ([]*domain.File, error) {
	const q = `
		SELECT id, hash, size, mime, name, owner_id, parent_id, created_at
		FROM files WHERE owner_id = $1 AND parent_id IS NOT DISTINCT FROM $2
		ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, q, ownerID, parentID)
	if err != nil {
		return nil, fmt.Errorf("postgres: list files by parent: %w", err)
	}
	defer rows.Close()

	var files []*domain.File
	for rows.Next() {
		f, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list files by parent: %w", err)
	}
	return files, nil
}

func (r *FileRepository) UpdateParent(ctx context.Context, fileID uuid.UUID, parentID *uuid.UUID) error {
	const q = `UPDATE files SET parent_id = $1 WHERE id = $2`
	tag, err := r.pool.Exec(ctx, q, parentID, fileID)
	if err != nil {
		return fmt.Errorf("postgres: update file parent: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *FileRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const q = `DELETE FROM files WHERE id = $1`
	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("postgres: delete file: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *FileRepository) CountByHash(ctx context.Context, hash string) (int64, error) {
	const q = `SELECT count(*) FROM files WHERE hash = $1`
	var count int64
	if err := r.pool.QueryRow(ctx, q, hash).Scan(&count); err != nil {
		return 0, fmt.Errorf("postgres: count files by hash: %w", err)
	}
	return count, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanFile(row rowScanner) (*domain.File, error) {
	var f domain.File
	if err := row.Scan(&f.ID, &f.Hash, &f.Size, &f.Mime, &f.Name, &f.OwnerID, &f.ParentID, &f.CreatedAt); err != nil {
		return nil, mapErr(err)
	}
	return &f, nil
}
