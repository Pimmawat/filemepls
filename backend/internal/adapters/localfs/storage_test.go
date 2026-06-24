package localfs

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"filemepls/internal/domain"
)

func newTestStorage(t *testing.T) *Storage {
	t.Helper()
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	return s
}

func TestStorage_SaveGetDelete(t *testing.T) {
	ctx := context.Background()
	s := newTestStorage(t)
	key := domain.StorageKey("ab/abcdef")

	n, err := s.Save(ctx, key, bytes.NewReader([]byte("hello world")))
	if err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	if n != 11 {
		t.Errorf("Save() size = %d, want 11", n)
	}

	exists, err := s.Exists(ctx, key)
	if err != nil || !exists {
		t.Fatalf("Exists() = %v, %v, want true, nil", exists, err)
	}

	r, err := s.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	defer func() { _ = r.Close() }()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll() error: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("data = %q, want %q", data, "hello world")
	}

	if err := s.Delete(ctx, key); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}
	exists, err = s.Exists(ctx, key)
	if err != nil || exists {
		t.Fatalf("Exists() after delete = %v, %v, want false, nil", exists, err)
	}

	// Delete of a missing key is idempotent, not an error.
	if err := s.Delete(ctx, key); err != nil {
		t.Errorf("Delete() of missing key error: %v", err)
	}
}

func TestStorage_Get_NotFound(t *testing.T) {
	s := newTestStorage(t)
	_, err := s.Get(context.Background(), domain.StorageKey("ab/missing"))
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("got err %v, want %v", err, domain.ErrNotFound)
	}
}

func TestStorage_GetRange(t *testing.T) {
	ctx := context.Background()
	s := newTestStorage(t)
	key := domain.StorageKey("ab/abcdef")
	content := "0123456789"

	if _, err := s.Save(ctx, key, bytes.NewReader([]byte(content))); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	r, total, err := s.GetRange(ctx, key, 3, 4)
	if err != nil {
		t.Fatalf("GetRange() error: %v", err)
	}
	defer func() { _ = r.Close() }()
	if total != 10 {
		t.Errorf("total = %d, want 10", total)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll() error: %v", err)
	}
	if string(data) != "3456" {
		t.Errorf("data = %q, want %q", data, "3456")
	}
}

func TestStorage_GetRange_ThroughEOF(t *testing.T) {
	ctx := context.Background()
	s := newTestStorage(t)
	key := domain.StorageKey("ab/abcdef")
	content := "0123456789"

	if _, err := s.Save(ctx, key, bytes.NewReader([]byte(content))); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	r, total, err := s.GetRange(ctx, key, 7, 0)
	if err != nil {
		t.Fatalf("GetRange() error: %v", err)
	}
	defer func() { _ = r.Close() }()
	if total != 10 {
		t.Errorf("total = %d, want 10", total)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll() error: %v", err)
	}
	if string(data) != "789" {
		t.Errorf("data = %q, want %q", data, "789")
	}
}

func TestStorage_Rename(t *testing.T) {
	ctx := context.Background()
	s := newTestStorage(t)
	oldKey := domain.StorageKey("staging/xyz")
	newKey := domain.StorageKey("ab/abcdef")

	if _, err := s.Save(ctx, oldKey, bytes.NewReader([]byte("payload"))); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	if err := s.Rename(ctx, oldKey, newKey); err != nil {
		t.Fatalf("Rename() error: %v", err)
	}

	if exists, _ := s.Exists(ctx, oldKey); exists {
		t.Error("old key should no longer exist after rename")
	}
	if exists, _ := s.Exists(ctx, newKey); !exists {
		t.Error("new key should exist after rename")
	}
}

func TestStorage_Resolve_RejectsPathTraversal(t *testing.T) {
	s := newTestStorage(t)

	keys := []domain.StorageKey{
		"../../etc/passwd",
		"..",
		"ab/../../../etc/passwd",
	}
	for _, key := range keys {
		if _, err := s.resolve(key); err == nil {
			t.Errorf("resolve(%q) expected error, got nil", key)
		}
	}
}

func TestStorage_PresignedURL_NotSupported(t *testing.T) {
	s := newTestStorage(t)
	_, err := s.PresignedURL(context.Background(), domain.StorageKey("ab/abcdef"), 0)
	if err == nil {
		t.Error("expected an error, got nil")
	}
}

func TestNew_CreatesRootDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "storage")
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("precondition failed: %v", err)
	}
	if _, err := New(dir); err != nil {
		t.Fatalf("New() error: %v", err)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Errorf("expected root dir to be created: %v", err)
	}
}
