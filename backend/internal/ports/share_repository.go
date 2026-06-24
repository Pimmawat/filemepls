package ports

import (
	"context"

	"github.com/google/uuid"

	"filemepls/internal/domain"
)

// ShareRepository persists share links. Lookups return domain.ErrNotFound
// instead of nil, nil when no record matches.
type ShareRepository interface {
	Save(ctx context.Context, s *domain.ShareLink) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.ShareLink, error)
	FindByToken(ctx context.Context, token string) (*domain.ShareLink, error)
	// ListByFile returns all share links for fileID, most recent first.
	ListByFile(ctx context.Context, fileID uuid.UUID) ([]*domain.ShareLink, error)
	// ListByFolder returns all share links for folderID, most recent first.
	ListByFolder(ctx context.Context, folderID uuid.UUID) ([]*domain.ShareLink, error)
	IncrementDownloadCount(ctx context.Context, id uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
}
