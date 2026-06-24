package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5"

	"filemepls/internal/domain"
)

// mapErr maps pgx's "no rows" sentinel to the shared domain.ErrNotFound,
// passing every other error through unchanged.
func mapErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}
