package domain

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestNewFolder(t *testing.T) {
	owner := uuid.New()
	parent := uuid.New()

	f, err := NewFolder("Photos", &parent, owner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Name != "Photos" || f.OwnerID != owner || f.ParentID == nil || *f.ParentID != parent {
		t.Errorf("unexpected folder: %+v", f)
	}
}

func TestNewFolder_Root(t *testing.T) {
	owner := uuid.New()
	f, err := NewFolder("Photos", nil, owner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.ParentID != nil {
		t.Errorf("expected root folder to have nil ParentID, got %v", f.ParentID)
	}
}

func TestNewFolder_TrimsWhitespace(t *testing.T) {
	f, err := NewFolder("  Photos  ", nil, uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Name != "Photos" {
		t.Errorf("name = %q, want %q", f.Name, "Photos")
	}
}

func TestNewFolder_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{"empty", "", ErrEmptyFolderName},
		{"whitespace only", "   ", ErrEmptyFolderName},
		{"forward slash", "a/b", ErrInvalidFolderName},
		{"backslash", "a\\b", ErrInvalidFolderName},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewFolder(tt.input, nil, uuid.New()); !errors.Is(err, tt.wantErr) {
				t.Errorf("got %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestFolder_EnsureOwnedBy(t *testing.T) {
	owner := uuid.New()
	other := uuid.New()
	f := &Folder{OwnerID: owner}

	if err := f.EnsureOwnedBy(owner); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if err := f.EnsureOwnedBy(other); !errors.Is(err, ErrNotOwner) {
		t.Errorf("got %v, want %v", err, ErrNotOwner)
	}
}
