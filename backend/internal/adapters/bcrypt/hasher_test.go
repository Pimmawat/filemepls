package bcrypt

import (
	"errors"
	"testing"

	"filemepls/internal/domain"
)

func TestHasher_HashAndVerify(t *testing.T) {
	h := NewWithCost(4) // low cost for fast tests

	hash, err := h.Hash("correct-password")
	if err != nil {
		t.Fatalf("Hash() error: %v", err)
	}
	if hash == "correct-password" {
		t.Fatal("Hash() returned the plaintext unchanged")
	}

	if err := h.Verify(hash, "correct-password"); err != nil {
		t.Errorf("Verify() with correct password error: %v", err)
	}

	if err := h.Verify(hash, "wrong-password"); !errors.Is(err, domain.ErrInvalidPassword) {
		t.Errorf("Verify() with wrong password got %v, want %v", err, domain.ErrInvalidPassword)
	}
}
