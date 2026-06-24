package domain

import (
	"errors"
	"testing"
)

func TestNewUser(t *testing.T) {
	u, err := NewUser("alice@example.com", "Alice", "github", "https://avatars.githubusercontent.com/u/1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Email != "alice@example.com" || u.DisplayName != "Alice" || u.Provider != "github" || u.AvatarURL != "https://avatars.githubusercontent.com/u/1" {
		t.Errorf("unexpected user fields: %+v", u)
	}
}

func TestNewUser_EmptyEmail(t *testing.T) {
	_, err := NewUser("", "Alice", "github", "")
	if !errors.Is(err, ErrEmptyEmail) {
		t.Fatalf("got err %v, want %v", err, ErrEmptyEmail)
	}
}
