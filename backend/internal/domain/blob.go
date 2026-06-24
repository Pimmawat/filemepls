package domain

import "time"

// Blob is the deduped, content-addressed record of bytes actually stored
// on disk, keyed by hash. Unlike File, it has no owner: multiple owned File
// records may reference the same Blob, but each Blob is written to storage
// at most once.
type Blob struct {
	Hash      string // 64-char lowercase hex sha256
	Size      int64
	Mime      string
	CreatedAt time.Time
}

func NewBlob(hash string, size int64, mime string) (*Blob, error) {
	if err := validateHash(hash); err != nil {
		return nil, err
	}
	if size <= 0 {
		return nil, ErrInvalidSize
	}

	return &Blob{
		Hash:      hash,
		Size:      size,
		Mime:      mime,
		CreatedAt: time.Now().UTC(),
	}, nil
}

// StorageKey derives the on-disk location for this blob from its hash,
// e.g. "ab/ab3f9c...".
func (b *Blob) StorageKey() (StorageKey, error) {
	return storageKeyFromHash(b.Hash)
}
