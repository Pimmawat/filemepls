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
	const q = `INSERT INTO users (id, email, display_name, provider, created_at) VALUES ($1, $2, $3, $4, $5)`
	if _, err := r.pool.Exec(ctx, q, u.ID, u.Email, u.DisplayName, u.Provider, u.CreatedAt); err != nil {
		return fmt.Errorf("postgres: save user: %w", err)
	}
	return nil
}

func (r *UserRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	const q = `SELECT id, email, display_name, provider, created_at FROM users WHERE id = $1`
	return scanUser(r.pool.QueryRow(ctx, q, id))
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	const q = `SELECT id, email, display_name, provider, created_at FROM users WHERE email = $1`
	return scanUser(r.pool.QueryRow(ctx, q, email))
}

func scanUser(row rowScanner) (*domain.User, error) {
	var u domain.User
	if err := row.Scan(&u.ID, &u.Email, &u.DisplayName, &u.Provider, &u.CreatedAt); err != nil {
		return nil, mapErr(err)
	}
	return &u, nil
}
