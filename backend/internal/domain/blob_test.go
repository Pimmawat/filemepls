package domain

import (
	"errors"
	"testing"
)

func TestNewBlob(t *testing.T) {
	tests := []struct {
		name    string
		hash    string
		size    int64
		mime    string
		wantErr error
	}{
		{"valid", validHash, 100, "image/png", nil},
		{"empty hash", "", 100, "image/png", ErrEmptyHash},
		{"invalid hash", "abc123", 100, "image/png", ErrInvalidHash},
		{"zero size", validHash, 0, "image/png", ErrInvalidSize},
		{"negative size", validHash, -1, "image/png", ErrInvalidSize},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := NewBlob(tt.hash, tt.size, tt.mime)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("got err %v, want %v", err, tt.wantErr)
				}
				if b != nil {
					t.Fatalf("expected nil blob on error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if b.Hash != tt.hash || b.Size != tt.size || b.Mime != tt.mime {
				t.Errorf("unexpected blob fields: %+v", b)
			}
		})
	}
}

func TestBlob_StorageKey(t *testing.T) {
	b, err := NewBlob(validHash, 10, "image/png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	key, err := b.StorageKey()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := StorageKey(validHash[:2] + "/" + validHash)
	if key != want {
		t.Errorf("key = %q, want %q", key, want)
	}
}
