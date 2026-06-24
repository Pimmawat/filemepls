package domain

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID          uuid.UUID
	Email       string
	DisplayName string
	Provider    string // "github" | "google"
	AvatarURL   string
	CreatedAt   time.Time
}

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
