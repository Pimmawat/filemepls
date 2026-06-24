package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// StorageKey is a safe-by-construction storage location derived from a
// validated file hash. The only way to obtain one is File.StorageKey, so a
// malformed or attacker-influenced string can never become a storage key.
type StorageKey string

const hashLen = 64

type File struct {
	ID        uuid.UUID
	Hash      string // 64-char lowercase hex sha256
	Size      int64
	Mime      string
	Name      string // original uploaded filename, as given by the client
	OwnerID   uuid.UUID
	ParentID  *uuid.UUID // nil = root level
	CreatedAt time.Time
}

func NewFile(hash string, size int64, mime, name string, ownerID uuid.UUID, parentID *uuid.UUID, maxSize int64, allowedMimes []string) (*File, error) {
	if err := validateHash(hash); err != nil {
		return nil, err
	}
	if err := validateSize(size, maxSize); err != nil {
		return nil, err
	}
	if err := validateMime(mime, allowedMimes); err != nil {
		return nil, err
	}

	return &File{
		ID:        uuid.New(),
		Hash:      hash,
		Size:      size,
		Mime:      mime,
		Name:      name,
		OwnerID:   ownerID,
		ParentID:  parentID,
		CreatedAt: time.Now().UTC(),
	}, nil
}

// StorageKey derives the on-disk location for this file from its validated
// hash, e.g. "ab/ab3f9c...". It never incorporates user-supplied filenames.
func (f *File) StorageKey() (StorageKey, error) {
	return storageKeyFromHash(f.Hash)
}

func storageKeyFromHash(hash string) (StorageKey, error) {
	if err := validateHash(hash); err != nil {
		return "", err
	}
	return StorageKey(fmt.Sprintf("%s/%s", hash[:2], hash)), nil
}

func (f *File) EnsureOwnedBy(userID uuid.UUID) error {
	if f.OwnerID != userID {
		return ErrNotOwner
	}
	return nil
}

func validateHash(hash string) error {
	if hash == "" {
		return ErrEmptyHash
	}
	if len(hash) != hashLen {
		return ErrInvalidHash
	}
	for _, c := range hash {
		isLowerHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')
		if !isLowerHex {
			return ErrInvalidHash
		}
	}
	return nil
}

// validateSize rejects empty files. max <= 0 means unlimited (no upper
// bound check).
func validateSize(size, max int64) error {
	if size <= 0 {
		return ErrInvalidSize
	}
	if max > 0 && size > max {
		return ErrFileTooLarge
	}
	return nil
}

// validateMime checks mime against the allowlist. An allowlist containing
// "*" (or "*/*") matches any mime type.
func validateMime(mime string, allowed []string) error {
	for _, m := range allowed {
		if m == "*" || m == "*/*" || strings.EqualFold(m, mime) {
			return nil
		}
	}
	return ErrUnsupportedMime
}
