package domain

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID
	Email        string
	DisplayName  string
	Provider     string // "github" | "google" | "password"
	AvatarURL    string
	PasswordHash *string // nil for OAuth-only users; set for ProviderPassword users
	CreatedAt    time.Time
}

// ProviderPassword identifies a user that signed up with email+password
// rather than via an OAuth provider.
const ProviderPassword = "password"

func NewUser(email, displayName, provider, avatarURL string) (*User, error) {
	if email == "" {
		return nil, ErrEmptyEmail
	}

	return &User{
		ID:          uuid.New(),
		Email:       email,
		DisplayName: displayName,
		Provider:    provider,
		AvatarURL:   avatarURL,
		CreatedAt:   time.Now().UTC(),
	}, nil
}

// NewLocalUser creates a User authenticated by an email+password pair.
// passwordHash must already be hashed by the caller (the usecase layer,
// via ports.PasswordHasher) — domain stays stdlib-only and never hashes
// passwords itself.
func NewLocalUser(email, displayName, passwordHash string) (*User, error) {
	if email == "" {
		return nil, ErrEmptyEmail
	}
	if passwordHash == "" {
		return nil, ErrEmptyPasswordHash
	}

	return &User{
		ID:           uuid.New(),
		Email:        email,
		DisplayName:  displayName,
		Provider:     ProviderPassword,
		PasswordHash: &passwordHash,
		CreatedAt:    time.Now().UTC(),
	}, nil
}
