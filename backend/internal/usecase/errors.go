package usecase

import "errors"

// Orchestration-level errors: about how the application is wired (unknown
// OAuth provider, CSRF mismatch), not business invariants — domain sentinel
// errors cover those instead.
var (
	ErrUnknownProvider = errors.New("usecase: unknown oauth provider")
	ErrInvalidState    = errors.New("usecase: oauth state mismatch")
)
