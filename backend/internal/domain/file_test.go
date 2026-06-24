package domain

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

const validHash = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"

func TestNewFile(t *testing.T) {
	owner := uuid.New()
	allowed := []string{"image/png", "text/plain"}

	tests := []struct {
		name    string
		hash    string
		size    int64
		mime    string
		maxSize int64
		wantErr error
	}{
		{"valid", validHash, 100, "image/png", 1000, nil},
		{"empty hash", "", 100, "image/png", 1000, ErrEmptyHash},
		{"short hash", "abc123", 100, "image/png", 1000, ErrInvalidHash},
		{"uppercase hash", strings.ToUpper(validHash), 100, "image/png", 1000, ErrInvalidHash},
		{"zero size", validHash, 0, "image/png", 1000, ErrInvalidSize},
		{"negative size", validHash, -1, "image/png", 1000, ErrInvalidSize},
		{"too large", validHash, 2000, "image/png", 1000, ErrFileTooLarge},
		{"disallowed mime", validHash, 100, "application/x-evil", 1000, ErrUnsupportedMime},
		{"case-insensitive mime", validHash, 100, "IMAGE/PNG", 1000, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := NewFile(tt.hash, tt.size, tt.mime, "photo.png", owner, nil, tt.maxSize, allowed)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("got err %v, want %v", err, tt.wantErr)
				}
				if f != nil {
					t.Fatalf("expected nil file on error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if f.OwnerID != owner {
				t.Errorf("owner = %v, want %v", f.OwnerID, owner)
			}
			if f.Name != "photo.png" {
				t.Errorf("name = %q, want %q", f.Name, "photo.png")
			}
		})
	}
}

func TestNewFile_UnlimitedSize(t *testing.T) {
	owner := uuid.New()
	// maxSize <= 0 means unlimited: a huge size must not trip ErrFileTooLarge.
	if _, err := NewFile(validHash, 10<<30, "image/png", "big.png", owner, nil, 0, []string{"image/png"}); err != nil {
		t.Errorf("unexpected error with maxSize=0: %v", err)
	}
	if _, err := NewFile(validHash, 10<<30, "image/png", "big.png", owner, nil, -1, []string{"image/png"}); err != nil {
		t.Errorf("unexpected error with maxSize=-1: %v", err)
	}
	// Zero/negative size is still always invalid, regardless of maxSize.
	if _, err := NewFile(validHash, 0, "image/png", "empty.png", owner, nil, 0, []string{"image/png"}); !errors.Is(err, ErrInvalidSize) {
		t.Errorf("got %v, want %v", err, ErrInvalidSize)
	}
}

func TestNewFile_WildcardMime(t *testing.T) {
	owner := uuid.New()
	for _, wildcard := range []string{"*", "*/*"} {
		if _, err := NewFile(validHash, 10, "application/x-anything", "data.bin", owner, nil, 1000, []string{wildcard}); err != nil {
			t.Errorf("wildcard %q: unexpected error: %v", wildcard, err)
		}
	}
}

func TestFile_StorageKey(t *testing.T) {
	owner := uuid.New()
	f, err := NewFile(validHash, 10, "image/png", "photo.png", owner, nil, 100, []string{"image/png"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	key, err := f.StorageKey()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := StorageKey(validHash[:2] + "/" + validHash)
	if key != want {
		t.Errorf("key = %q, want %q", key, want)
	}
}

func TestFile_StorageKey_RejectsMalformedHash(t *testing.T) {
	f := &File{Hash: "../../etc/passwd"}
	if _, err := f.StorageKey(); !errors.Is(err, ErrInvalidHash) {
		t.Fatalf("got err %v, want %v", err, ErrInvalidHash)
	}
}

func TestFile_EnsureOwnedBy(t *testing.T) {
	owner := uuid.New()
	other := uuid.New()
	f := &File{OwnerID: owner}

	if err := f.EnsureOwnedBy(owner); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := f.EnsureOwnedBy(other); !errors.Is(err, ErrNotOwner) {
		t.Errorf("got err %v, want %v", err, ErrNotOwner)
	}
}
