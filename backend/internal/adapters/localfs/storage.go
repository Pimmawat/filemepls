package localfs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"filemepls/internal/domain"
	"filemepls/internal/ports"
)

var _ ports.StoragePort = (*Storage)(nil)

// Storage implements ports.StoragePort on the local filesystem.
type Storage struct {
	root string // absolute, cleaned
}

func New(root string) (*Storage, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("localfs: resolve root: %w", err)
	}
	if err := os.MkdirAll(abs, 0o750); err != nil {
		return nil, fmt.Errorf("localfs: create root: %w", err)
	}
	return &Storage{root: abs}, nil
}

// resolve maps a StorageKey to an absolute path, defensively re-verifying
// containment under root even though StorageKey is already safe-by
// construction upstream (defense in depth, per ports.StoragePort's doc).
func (s *Storage) resolve(key domain.StorageKey) (string, error) {
	cleanRel := filepath.Clean(string(key))
	if cleanRel == "." || cleanRel == ".." || strings.HasPrefix(cleanRel, "../") || filepath.IsAbs(cleanRel) {
		return "", fmt.Errorf("localfs: invalid key %q", key)
	}

	full := filepath.Clean(filepath.Join(s.root, cleanRel))
	if full != s.root && !strings.HasPrefix(full, s.root+string(filepath.Separator)) {
		return "", fmt.Errorf("localfs: key %q escapes storage root", key)
	}
	return full, nil
}

func (s *Storage) Save(ctx context.Context, key domain.StorageKey, r io.Reader) (int64, error) {
	path, err := s.resolve(key)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return 0, fmt.Errorf("localfs: mkdir: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".upload-*")
	if err != nil {
		return 0, fmt.Errorf("localfs: create temp file: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // no-op once the rename below succeeds

	n, err := io.Copy(tmp, r)
	if err != nil {
		_ = tmp.Close()
		return 0, fmt.Errorf("localfs: write: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return 0, fmt.Errorf("localfs: sync: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return 0, fmt.Errorf("localfs: close temp file: %w", err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return 0, fmt.Errorf("localfs: finalize: %w", err)
	}

	return n, nil
}

func (s *Storage) Get(ctx context.Context, key domain.StorageKey) (io.ReadCloser, error) {
	path, err := s.resolve(key)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("localfs: open: %w", err)
	}
	return f, nil
}

func (s *Storage) GetRange(ctx context.Context, key domain.StorageKey, offset, length int64) (io.ReadCloser, int64, error) {
	path, err := s.resolve(key)
	if err != nil {
		return nil, 0, err
	}
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, 0, domain.ErrNotFound
		}
		return nil, 0, fmt.Errorf("localfs: open: %w", err)
	}

	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, 0, fmt.Errorf("localfs: stat: %w", err)
	}
	totalSize := info.Size()

	if offset < 0 || offset > totalSize {
		_ = f.Close()
		return nil, 0, fmt.Errorf("localfs: offset %d out of range for size %d", offset, totalSize)
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		_ = f.Close()
		return nil, 0, fmt.Errorf("localfs: seek: %w", err)
	}

	var reader io.Reader = f
	if length > 0 {
		reader = io.LimitReader(f, length)
	}
	return &rangeReadCloser{Reader: reader, f: f}, totalSize, nil
}

type rangeReadCloser struct {
	io.Reader
	f *os.File
}

func (r *rangeReadCloser) Close() error { return r.f.Close() }

func (s *Storage) Delete(ctx context.Context, key domain.StorageKey) error {
	path, err := s.resolve(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("localfs: remove: %w", err)
	}
	return nil
}

func (s *Storage) Exists(ctx context.Context, key domain.StorageKey) (bool, error) {
	path, err := s.resolve(key)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("localfs: stat: %w", err)
}

func (s *Storage) Rename(ctx context.Context, oldKey, newKey domain.StorageKey) error {
	oldPath, err := s.resolve(oldKey)
	if err != nil {
		return err
	}
	newPath, err := s.resolve(newKey)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(newPath), 0o750); err != nil {
		return fmt.Errorf("localfs: mkdir: %w", err)
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		return fmt.Errorf("localfs: rename: %w", err)
	}
	return nil
}

// PresignedURL is an S3-ism with no local-filesystem equivalent. Local
// downloads stream directly through GetRange instead of redirecting to a
// presigned URL; this method only becomes meaningful for a future S3/MinIO
// adapter.
func (s *Storage) PresignedURL(ctx context.Context, key domain.StorageKey, expiresIn time.Duration) (string, error) {
	return "", errors.New("localfs: presigned URLs are not supported")
}
