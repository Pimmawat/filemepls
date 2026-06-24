package ports

// PasswordHasher abstracts the password hashing algorithm (bcrypt/argon2)
// so it can change without touching business logic. Domain stays
// stdlib-only and never imports this package directly; the usecase layer
// calls Verify and combines the result with domain.ShareLink's pure checks
// (IsExpired, IsDownloadLimitReached, RequiresPassword).
type PasswordHasher interface {
	Hash(plain string) (string, error)
	Verify(hash, plain string) error // returns domain.ErrInvalidPassword on mismatch
}
