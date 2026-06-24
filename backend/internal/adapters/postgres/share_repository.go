package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"filemepls/internal/domain"
	"filemepls/internal/ports"
)

var _ ports.ShareRepository = (*ShareRepository)(nil)

type ShareRepository struct {
	pool *pgxpool.Pool
}

func NewShareRepository(pool *pgxpool.Pool) *ShareRepository {
	return &ShareRepository{pool: pool}
}

func (r *ShareRepository) Save(ctx context.Context, s *domain.ShareLink) error {
	const q = `
		INSERT INTO share_links (id, token, target_type, file_id, folder_id, expires_at, password_hash, max_downloads, download_count, visibility, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`
	_, err := r.pool.Exec(ctx, q,
		s.ID, s.Token, string(s.TargetType), s.FileID, s.FolderID, s.ExpiresAt, s.PasswordHash, s.MaxDownloads, s.DownloadCount, s.Visibility, s.CreatedAt)
	if err != nil {
		return fmt.Errorf("postgres: save share link: %w", err)
	}
	return nil
}

func (r *ShareRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.ShareLink, error) {
	const q = `
		SELECT id, token, target_type, file_id, folder_id, expires_at, password_hash, max_downloads, download_count, visibility, created_at
		FROM share_links WHERE id = $1`
	return scanShareLink(r.pool.QueryRow(ctx, q, id))
}

func (r *ShareRepository) FindByToken(ctx context.Context, token string) (*domain.ShareLink, error) {
	const q = `
		SELECT id, token, target_type, file_id, folder_id, expires_at, password_hash, max_downloads, download_count, visibility, created_at
		FROM share_links WHERE token = $1`
	return scanShareLink(r.pool.QueryRow(ctx, q, token))
}

func (r *ShareRepository) ListByFile(ctx context.Context, fileID uuid.UUID) ([]*domain.ShareLink, error) {
	const q = `
		SELECT id, token, target_type, file_id, folder_id, expires_at, password_hash, max_downloads, download_count, visibility, created_at
		FROM share_links WHERE file_id = $1 ORDER BY created_at DESC`
	return scanShareLinks(ctx, r.pool, q, fileID)
}

func (r *ShareRepository) ListByFolder(ctx context.Context, folderID uuid.UUID) ([]*domain.ShareLink, error) {
	const q = `
		SELECT id, token, target_type, file_id, folder_id, expires_at, password_hash, max_downloads, download_count, visibility, created_at
		FROM share_links WHERE folder_id = $1 ORDER BY created_at DESC`
	return scanShareLinks(ctx, r.pool, q, folderID)
}

func scanShareLinks(ctx context.Context, pool *pgxpool.Pool, q string, arg uuid.UUID) ([]*domain.ShareLink, error) {
	rows, err := pool.Query(ctx, q, arg)
	if err != nil {
		return nil, fmt.Errorf("postgres: list share links: %w", err)
	}
	defer rows.Close()

	var shares []*domain.ShareLink
	for rows.Next() {
		s, err := scanShareLink(rows)
		if err != nil {
			return nil, err
		}
		shares = append(shares, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list share links: %w", err)
	}
	return shares, nil
}

func scanShareLink(row rowScanner) (*domain.ShareLink, error) {
	var s domain.ShareLink
	var visibility, targetType string
	if err := row.Scan(&s.ID, &s.Token, &targetType, &s.FileID, &s.FolderID, &s.ExpiresAt, &s.PasswordHash, &s.MaxDownloads, &s.DownloadCount, &visibility, &s.CreatedAt); err != nil {
		return nil, mapErr(err)
	}
	s.Visibility = domain.Visibility(visibility)
	s.TargetType = domain.ShareTargetType(targetType)
	return &s, nil
}

func (r *ShareRepository) IncrementDownloadCount(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE share_links SET download_count = download_count + 1 WHERE id = $1`
	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("postgres: increment download count: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *ShareRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const q = `DELETE FROM share_links WHERE id = $1`
	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("postgres: delete share link: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
