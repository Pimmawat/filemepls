package ports

import (
	"context"

	"github.com/google/uuid"

	"filemepls/internal/domain"
)

// UserRepository persists users. Lookups return domain.ErrNotFound instead
// of nil, nil when no record matches.
type UserRepository interface {
	Save(ctx context.Context, u *domain.User) error
	Update(ctx context.Context, u *domain.User) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	// SearchByEmail returns up to limit users whose email contains query
	// (case-insensitive), excluding excludeID, ordered by email.
	SearchByEmail(ctx context.Context, query string, excludeID uuid.UUID, limit int) ([]*domain.User, error)
}
