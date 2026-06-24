package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"filemepls/internal/domain"
	"filemepls/internal/ports"
)

var _ ports.FolderRepository = (*FolderRepository)(nil)

type FolderRepository struct {
	pool *pgxpool.Pool
}

func NewFolderRepository(pool *pgxpool.Pool) *FolderRepository {
	return &FolderRepository{pool: pool}
}

func (r *FolderRepository) Save(ctx context.Context, f *domain.Folder) error {
	const q = `INSERT INTO folders (id, name, parent_id, owner_id, created_at) VALUES ($1, $2, $3, $4, $5)`
	if _, err := r.pool.Exec(ctx, q, f.ID, f.Name, f.ParentID, f.OwnerID, f.CreatedAt); err != nil {
		return fmt.Errorf("postgres: save folder: %w", err)
	}
	return nil
}

func (r *FolderRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Folder, error) {
	const q = `SELECT id, name, parent_id, owner_id, created_at FROM folders WHERE id = $1`
	return scanFolder(r.pool.QueryRow(ctx, q, id))
}

func (r *FolderRepository) ListChildren(ctx context.Context, ownerID uuid.UUID, parentID *uuid.UUID) ([]*domain.Folder, error) {
	const q = `
		SELECT id, name, parent_id, owner_id, created_at
		FROM folders WHERE owner_id = $1 AND parent_id IS NOT DISTINCT FROM $2
		ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, q, ownerID, parentID)
	if err != nil {
		return nil, fmt.Errorf("postgres: list folders: %w", err)
	}
	defer rows.Close()

	var folders []*domain.Folder
	for rows.Next() {
		f, err := scanFolder(rows)
		if err != nil {
			return nil, err
		}
		folders = append(folders, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list folders: %w", err)
	}
	return folders, nil
}

func (r *FolderRepository) UpdateParent(ctx context.Context, folderID uuid.UUID, parentID *uuid.UUID) error {
	const q = `UPDATE folders SET parent_id = $1 WHERE id = $2`
	tag, err := r.pool.Exec(ctx, q, parentID, folderID)
	if err != nil {
		return fmt.Errorf("postgres: update folder parent: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *FolderRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const q = `DELETE FROM folders WHERE id = $1`
	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("postgres: delete folder: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func scanFolder(row rowScanner) (*domain.Folder, error) {
	var f domain.Folder
	if err := row.Scan(&f.ID, &f.Name, &f.ParentID, &f.OwnerID, &f.CreatedAt); err != nil {
		return nil, mapErr(err)
	}
	return &f, nil
}
