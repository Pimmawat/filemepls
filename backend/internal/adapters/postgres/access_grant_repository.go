package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"filemepls/internal/domain"
	"filemepls/internal/ports"
)

var _ ports.AccessGrantRepository = (*AccessGrantRepository)(nil)

type AccessGrantRepository struct {
	pool *pgxpool.Pool
}

func NewAccessGrantRepository(pool *pgxpool.Pool) *AccessGrantRepository {
	return &AccessGrantRepository{pool: pool}
}

const uniqueViolationCode = "23505"

func (r *AccessGrantRepository) Save(ctx context.Context, g *domain.AccessGrant) error {
	const q = `
		INSERT INTO access_grants (id, target_type, file_id, folder_id, grantee_id, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := r.pool.Exec(ctx, q, g.ID, string(g.TargetType), g.FileID, g.FolderID, g.GranteeID, g.CreatedBy, g.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode {
			return domain.ErrAlreadyGranted
		}
		return fmt.Errorf("postgres: save access grant: %w", err)
	}
	return nil
}

func (r *AccessGrantRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.AccessGrant, error) {
	const q = `
		SELECT id, target_type, file_id, folder_id, grantee_id, created_by, created_at
		FROM access_grants WHERE id = $1`
	return scanAccessGrant(r.pool.QueryRow(ctx, q, id))
}

func (r *AccessGrantRepository) ListByFile(ctx context.Context, fileID uuid.UUID) ([]*domain.AccessGrant, error) {
	const q = `
		SELECT id, target_type, file_id, folder_id, grantee_id, created_by, created_at
		FROM access_grants WHERE file_id = $1 ORDER BY created_at DESC`
	return scanAccessGrants(ctx, r.pool, q, fileID)
}

func (r *AccessGrantRepository) ListByFolder(ctx context.Context, folderID uuid.UUID) ([]*domain.AccessGrant, error) {
	const q = `
		SELECT id, target_type, file_id, folder_id, grantee_id, created_by, created_at
		FROM access_grants WHERE folder_id = $1 ORDER BY created_at DESC`
	return scanAccessGrants(ctx, r.pool, q, folderID)
}

func scanAccessGrants(ctx context.Context, pool *pgxpool.Pool, q string, arg uuid.UUID) ([]*domain.AccessGrant, error) {
	rows, err := pool.Query(ctx, q, arg)
	if err != nil {
		return nil, fmt.Errorf("postgres: list access grants: %w", err)
	}
	defer rows.Close()

	var grants []*domain.AccessGrant
	for rows.Next() {
		g, err := scanAccessGrant(rows)
		if err != nil {
			return nil, err
		}
		grants = append(grants, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list access grants: %w", err)
	}
	return grants, nil
}

func scanAccessGrant(row rowScanner) (*domain.AccessGrant, error) {
	var g domain.AccessGrant
	var targetType string
	if err := row.Scan(&g.ID, &targetType, &g.FileID, &g.FolderID, &g.GranteeID, &g.CreatedBy, &g.CreatedAt); err != nil {
		return nil, mapErr(err)
	}
	g.TargetType = domain.AccessTargetType(targetType)
	return &g, nil
}

func (r *AccessGrantRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const q = `DELETE FROM access_grants WHERE id = $1`
	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("postgres: delete access grant: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *AccessGrantRepository) HasFileAccess(ctx context.Context, fileID, granteeID uuid.UUID) (bool, error) {
	const q = `SELECT EXISTS(SELECT 1 FROM access_grants WHERE file_id = $1 AND grantee_id = $2)`
	var ok bool
	if err := r.pool.QueryRow(ctx, q, fileID, granteeID).Scan(&ok); err != nil {
		return false, fmt.Errorf("postgres: check file access: %w", err)
	}
	return ok, nil
}

func (r *AccessGrantRepository) HasFolderAccess(ctx context.Context, folderID, granteeID uuid.UUID) (bool, error) {
	const q = `SELECT EXISTS(SELECT 1 FROM access_grants WHERE folder_id = $1 AND grantee_id = $2)`
	var ok bool
	if err := r.pool.QueryRow(ctx, q, folderID, granteeID).Scan(&ok); err != nil {
		return false, fmt.Errorf("postgres: check folder access: %w", err)
	}
	return ok, nil
}

func (r *AccessGrantRepository) ListSharedFiles(ctx context.Context, granteeID uuid.UUID) ([]*domain.File, error) {
	const q = `
		SELECT f.id, f.hash, f.size, f.mime, f.name, f.owner_id, f.parent_id, f.created_at
		FROM files f
		JOIN access_grants g ON g.file_id = f.id
		WHERE g.grantee_id = $1
		ORDER BY g.created_at DESC`
	rows, err := r.pool.Query(ctx, q, granteeID)
	if err != nil {
		return nil, fmt.Errorf("postgres: list shared files: %w", err)
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
		return nil, fmt.Errorf("postgres: list shared files: %w", err)
	}
	return files, nil
}

func (r *AccessGrantRepository) ListSharedFolders(ctx context.Context, granteeID uuid.UUID) ([]*domain.Folder, error) {
	const q = `
		SELECT f.id, f.name, f.parent_id, f.owner_id, f.created_at
		FROM folders f
		JOIN access_grants g ON g.folder_id = f.id
		WHERE g.grantee_id = $1
		ORDER BY g.created_at DESC`
	rows, err := r.pool.Query(ctx, q, granteeID)
	if err != nil {
		return nil, fmt.Errorf("postgres: list shared folders: %w", err)
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
		return nil, fmt.Errorf("postgres: list shared folders: %w", err)
	}
	return folders, nil
}
