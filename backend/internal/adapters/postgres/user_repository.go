package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"filemepls/internal/domain"
	"filemepls/internal/ports"
)

var _ ports.UserRepository = (*UserRepository)(nil)

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) Save(ctx context.Context, u *domain.User) error {
	const q = `INSERT INTO users (id, email, display_name, provider, avatar_url, password_hash, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7)`
	if _, err := r.pool.Exec(ctx, q, u.ID, u.Email, u.DisplayName, u.Provider, u.AvatarURL, u.PasswordHash, u.CreatedAt); err != nil {
		return fmt.Errorf("postgres: save user: %w", err)
	}
	return nil
}

func (r *UserRepository) Update(ctx context.Context, u *domain.User) error {
	const q = `UPDATE users SET display_name = $2, avatar_url = $3 WHERE id = $1`
	if _, err := r.pool.Exec(ctx, q, u.ID, u.DisplayName, u.AvatarURL); err != nil {
		return fmt.Errorf("postgres: update user: %w", err)
	}
	return nil
}

func (r *UserRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	const q = `SELECT id, email, display_name, provider, avatar_url, password_hash, created_at FROM users WHERE id = $1`
	return scanUser(r.pool.QueryRow(ctx, q, id))
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	const q = `SELECT id, email, display_name, provider, avatar_url, password_hash, created_at FROM users WHERE email = $1`
	return scanUser(r.pool.QueryRow(ctx, q, email))
}

func (r *UserRepository) SearchByEmail(ctx context.Context, query string, excludeID uuid.UUID, limit int) ([]*domain.User, error) {
	const q = `
		SELECT id, email, display_name, provider, avatar_url, password_hash, created_at
		FROM users
		WHERE email ILIKE '%' || $1 || '%' AND id != $2
		ORDER BY email
		LIMIT $3`
	rows, err := r.pool.Query(ctx, q, query, excludeID, limit)
	if err != nil {
		return nil, fmt.Errorf("postgres: search users: %w", err)
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: search users: %w", err)
	}
	return users, nil
}

func scanUser(row rowScanner) (*domain.User, error) {
	var u domain.User
	if err := row.Scan(&u.ID, &u.Email, &u.DisplayName, &u.Provider, &u.AvatarURL, &u.PasswordHash, &u.CreatedAt); err != nil {
		return nil, mapErr(err)
	}
	return &u, nil
}
