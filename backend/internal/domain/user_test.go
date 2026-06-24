package domain

import (
	"errors"
	"testing"
)

func TestNewUser(t *testing.T) {
	u, err := NewUser("alice@example.com", "Alice", "github")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Email != "alice@example.com" || u.DisplayName != "Alice" || u.Provider != "github" {
		t.Errorf("unexpected user fields: %+v", u)
	}
}

func TestNewUser_EmptyEmail(t *testing.T) {
	_, err := NewUser("", "Alice", "github")
	if !errors.Is(err, ErrEmptyEmail) {
		t.Fatalf("got err %v, want %v", err, ErrEmptyEmail)
	}
}
