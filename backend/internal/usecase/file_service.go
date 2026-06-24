package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"

	"filemepls/internal/domain"
	"filemepls/internal/ports"
)

type FileService struct {
	files        ports.FileRepository
	blobs        ports.BlobRepository
	folders      ports.FolderRepository
	storage      ports.StoragePort
	maxSize      int64
	allowedMimes []string
}

func NewFileService(files ports.FileRepository, blobs ports.BlobRepository, folders ports.FolderRepository, storage ports.StoragePort, maxSize int64, allowedMimes []string) *FileService {
	return &FileService{files: files, blobs: blobs, folders: folders, storage: storage, maxSize: maxSize, allowedMimes: allowedMimes}
}

// Upload streams body through a SHA-256 hash into a staging key (never
// buffering the whole file in memory), then either dedups against an
// existing Blob or promotes the staged bytes to their final
// content-addressed key. A new owned File record is always created,
// regardless of whether the bytes were already on disk. parentID nil
// means the file is uploaded at root level.
func (s *FileService) Upload(ctx context.Context, ownerID uuid.UUID, declaredMime, declaredName string, parentID *uuid.UUID, body io.Reader) (*domain.File, error) {
	if !mimeAllowed(declaredMime, s.allowedMimes) {
		return nil, domain.ErrUnsupportedMime
	}
	if parentID != nil {
		parent, err := s.folders.FindByID(ctx, *parentID)
		if err != nil {
			return nil, err
		}
		if err := parent.EnsureOwnedBy(ownerID); err != nil {
			return nil, err
		}
	}

	stagingKey := domain.StorageKey("_staging/" + uuid.New().String())
	hasher := sha256.New()
	limited := io.Reader(body)
	if s.maxSize > 0 {
		limited = io.LimitReader(body, s.maxSize+1)
	}
	tee := io.TeeReader(limited, hasher)

	size, err := s.storage.Save(ctx, stagingKey, tee)
	if err != nil {
		return nil, fmt.Errorf("usecase: stage upload: %w", err)
	}
	if s.maxSize > 0 && size > s.maxSize {
		_ = s.storage.Delete(ctx, stagingKey)
		return nil, domain.ErrFileTooLarge
	}

	hash := hex.EncodeToString(hasher.Sum(nil))

	if err := s.promoteOrDiscard(ctx, stagingKey, hash, size, declaredMime); err != nil {
		return nil, err
	}

	file, err := domain.NewFile(hash, size, declaredMime, declaredName, ownerID, parentID, s.maxSize, s.allowedMimes)
	if err != nil {
		return nil, err
	}
	if err := s.files.Save(ctx, file); err != nil {
		return nil, fmt.Errorf("usecase: save file: %w", err)
	}
	return file, nil
}

// promoteOrDiscard resolves the staged upload against the Blob table: if a
// Blob with this hash already exists, the staged bytes are redundant and
// discarded; otherwise they're renamed into their final content-addressed
// key and a new Blob record is created.
func (s *FileService) promoteOrDiscard(ctx context.Context, stagingKey domain.StorageKey, hash string, size int64, mime string) error {
	_, err := s.blobs.FindByHash(ctx, hash)
	if err == nil {
		if err := s.storage.Delete(ctx, stagingKey); err != nil {
			return fmt.Errorf("usecase: discard staged duplicate: %w", err)
		}
		return nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return fmt.Errorf("usecase: lookup blob: %w", err)
	}

	blob, err := domain.NewBlob(hash, size, mime)
	if err != nil {
		_ = s.storage.Delete(ctx, stagingKey)
		return err
	}
	finalKey, err := blob.StorageKey()
	if err != nil {
		_ = s.storage.Delete(ctx, stagingKey)
		return err
	}
	if err := s.storage.Rename(ctx, stagingKey, finalKey); err != nil {
		return fmt.Errorf("usecase: promote staged upload: %w", err)
	}
	if err := s.blobs.Save(ctx, blob); err != nil {
		return fmt.Errorf("usecase: save blob: %w", err)
	}
	return nil
}

func (s *FileService) List(ctx context.Context, ownerID uuid.UUID) ([]*domain.File, error) {
	return s.files.ListByOwner(ctx, ownerID)
}

func (s *FileService) GetMetadata(ctx context.Context, ownerID, fileID uuid.UUID) (*domain.File, error) {
	f, err := s.files.FindByID(ctx, fileID)
	if err != nil {
		return nil, err
	}
	if err := f.EnsureOwnedBy(ownerID); err != nil {
		return nil, err
	}
	return f, nil
}

// Delete removes the caller's File record, then deletes the underlying
// Blob (DB record + on-disk bytes) only if no other File row still
// references the same hash.
func (s *FileService) Delete(ctx context.Context, ownerID, fileID uuid.UUID) error {
	f, err := s.files.FindByID(ctx, fileID)
	if err != nil {
		return err
	}
	if err := f.EnsureOwnedBy(ownerID); err != nil {
		return err
	}
	if err := s.files.Delete(ctx, fileID); err != nil {
		return err
	}

	count, err := s.files.CountByHash(ctx, f.Hash)
	if err != nil {
		return fmt.Errorf("usecase: count files by hash: %w", err)
	}
	if count > 0 {
		return nil
	}

	key, err := f.StorageKey()
	if err != nil {
		return err
	}
	if err := s.storage.Delete(ctx, key); err != nil {
		return fmt.Errorf("usecase: delete blob bytes: %w", err)
	}
	if err := s.blobs.Delete(ctx, f.Hash); err != nil {
		return fmt.Errorf("usecase: delete blob record: %w", err)
	}
	return nil
}

// DownloadRange resolves ownership and streams the requested byte range
// (or the whole file if rangeHeader is empty) from storage.
func (s *FileService) DownloadRange(ctx context.Context, ownerID, fileID uuid.UUID, rangeHeader string) (stream io.ReadCloser, offset, contentLength, totalSize int64, partial bool, mime, name string, createdAt time.Time, err error) {
	f, err := s.files.FindByID(ctx, fileID)
	if err != nil {
		return nil, 0, 0, 0, false, "", "", time.Time{}, err
	}
	if err := f.EnsureOwnedBy(ownerID); err != nil {
		return nil, 0, 0, 0, false, "", "", time.Time{}, err
	}

	off, length, isPartial, err := parseRange(rangeHeader, f.Size)
	if err != nil {
		return nil, 0, 0, 0, false, "", "", time.Time{}, err
	}

	key, err := f.StorageKey()
	if err != nil {
		return nil, 0, 0, 0, false, "", "", time.Time{}, err
	}

	rc, total, err := s.storage.GetRange(ctx, key, off, length)
	if err != nil {
		return nil, 0, 0, 0, false, "", "", time.Time{}, err
	}

	cl := length
	if cl <= 0 {
		cl = total - off
	}
	return rc, off, cl, total, isPartial, f.Mime, f.Name, f.CreatedAt, nil
}

// mimeAllowed checks mime against the allowlist. An allowlist containing
// "*" (or "*/*") matches any mime type.
func mimeAllowed(mime string, allowed []string) bool {
	for _, m := range allowed {
		if m == "*" || m == "*/*" || strings.EqualFold(m, mime) {
			return true
		}
	}
	return false
}
