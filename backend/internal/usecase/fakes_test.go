package usecase

import (
	"bytes"
	"context"
	"errors"
	"io"
	"time"

	"github.com/google/uuid"

	"filemepls/internal/domain"
	"filemepls/internal/ports"
)

// sameUUIDPtr reports whether two optional UUIDs are equal, treating two
// nils as equal (used by fakes to filter "direct children of parentID",
// where parentID nil means root).
func sameUUIDPtr(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// --- fakeFileRepository ---

type fakeFileRepository struct {
	byID map[uuid.UUID]*domain.File
}

func newFakeFileRepository() *fakeFileRepository {
	return &fakeFileRepository{byID: make(map[uuid.UUID]*domain.File)}
}

func (r *fakeFileRepository) Save(_ context.Context, f *domain.File) error {
	clone := *f
	r.byID[f.ID] = &clone
	return nil
}

func (r *fakeFileRepository) FindByID(_ context.Context, id uuid.UUID) (*domain.File, error) {
	f, ok := r.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	clone := *f
	return &clone, nil
}

func (r *fakeFileRepository) ListByOwner(_ context.Context, ownerID uuid.UUID) ([]*domain.File, error) {
	var out []*domain.File
	for _, f := range r.byID {
		if f.OwnerID == ownerID {
			clone := *f
			out = append(out, &clone)
		}
	}
	return out, nil
}

func (r *fakeFileRepository) ListByParent(_ context.Context, ownerID uuid.UUID, parentID *uuid.UUID) ([]*domain.File, error) {
	var out []*domain.File
	for _, f := range r.byID {
		if f.OwnerID == ownerID && sameUUIDPtr(f.ParentID, parentID) {
			clone := *f
			out = append(out, &clone)
		}
	}
	return out, nil
}

func (r *fakeFileRepository) UpdateParent(_ context.Context, fileID uuid.UUID, parentID *uuid.UUID) error {
	f, ok := r.byID[fileID]
	if !ok {
		return domain.ErrNotFound
	}
	f.ParentID = parentID
	return nil
}

func (r *fakeFileRepository) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := r.byID[id]; !ok {
		return domain.ErrNotFound
	}
	delete(r.byID, id)
	return nil
}

func (r *fakeFileRepository) CountByHash(_ context.Context, hash string) (int64, error) {
	var count int64
	for _, f := range r.byID {
		if f.Hash == hash {
			count++
		}
	}
	return count, nil
}

var _ ports.FileRepository = (*fakeFileRepository)(nil)

// --- fakeBlobRepository ---

type fakeBlobRepository struct {
	byHash map[string]*domain.Blob
}

func newFakeBlobRepository() *fakeBlobRepository {
	return &fakeBlobRepository{byHash: make(map[string]*domain.Blob)}
}

func (r *fakeBlobRepository) Save(_ context.Context, b *domain.Blob) error {
	clone := *b
	r.byHash[b.Hash] = &clone
	return nil
}

func (r *fakeBlobRepository) FindByHash(_ context.Context, hash string) (*domain.Blob, error) {
	b, ok := r.byHash[hash]
	if !ok {
		return nil, domain.ErrNotFound
	}
	clone := *b
	return &clone, nil
}

func (r *fakeBlobRepository) Delete(_ context.Context, hash string) error {
	delete(r.byHash, hash)
	return nil
}

var _ ports.BlobRepository = (*fakeBlobRepository)(nil)

// --- fakeFolderRepository ---

type fakeFolderRepository struct {
	byID map[uuid.UUID]*domain.Folder
}

func newFakeFolderRepository() *fakeFolderRepository {
	return &fakeFolderRepository{byID: make(map[uuid.UUID]*domain.Folder)}
}

func (r *fakeFolderRepository) Save(_ context.Context, f *domain.Folder) error {
	clone := *f
	r.byID[f.ID] = &clone
	return nil
}

func (r *fakeFolderRepository) FindByID(_ context.Context, id uuid.UUID) (*domain.Folder, error) {
	f, ok := r.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	clone := *f
	return &clone, nil
}

func (r *fakeFolderRepository) ListChildren(_ context.Context, ownerID uuid.UUID, parentID *uuid.UUID) ([]*domain.Folder, error) {
	var out []*domain.Folder
	for _, f := range r.byID {
		if f.OwnerID == ownerID && sameUUIDPtr(f.ParentID, parentID) {
			clone := *f
			out = append(out, &clone)
		}
	}
	return out, nil
}

func (r *fakeFolderRepository) UpdateParent(_ context.Context, folderID uuid.UUID, parentID *uuid.UUID) error {
	f, ok := r.byID[folderID]
	if !ok {
		return domain.ErrNotFound
	}
	f.ParentID = parentID
	return nil
}

func (r *fakeFolderRepository) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := r.byID[id]; !ok {
		return domain.ErrNotFound
	}
	delete(r.byID, id)
	return nil
}

var _ ports.FolderRepository = (*fakeFolderRepository)(nil)

// --- fakeStorage ---

type fakeStorage struct {
	objects map[domain.StorageKey][]byte
}

func newFakeStorage() *fakeStorage {
	return &fakeStorage{objects: make(map[domain.StorageKey][]byte)}
}

func (s *fakeStorage) Save(_ context.Context, key domain.StorageKey, r io.Reader) (int64, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return 0, err
	}
	s.objects[key] = data
	return int64(len(data)), nil
}

func (s *fakeStorage) Get(_ context.Context, key domain.StorageKey) (io.ReadCloser, error) {
	data, ok := s.objects[key]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (s *fakeStorage) GetRange(_ context.Context, key domain.StorageKey, offset, length int64) (io.ReadCloser, int64, error) {
	data, ok := s.objects[key]
	if !ok {
		return nil, 0, domain.ErrNotFound
	}
	total := int64(len(data))
	if offset < 0 || offset > total {
		return nil, 0, errors.New("fake storage: offset out of range")
	}
	end := total
	if length > 0 && offset+length < total {
		end = offset + length
	}
	return io.NopCloser(bytes.NewReader(data[offset:end])), total, nil
}

func (s *fakeStorage) Delete(_ context.Context, key domain.StorageKey) error {
	delete(s.objects, key)
	return nil
}

func (s *fakeStorage) Exists(_ context.Context, key domain.StorageKey) (bool, error) {
	_, ok := s.objects[key]
	return ok, nil
}

func (s *fakeStorage) Rename(_ context.Context, oldKey, newKey domain.StorageKey) error {
	data, ok := s.objects[oldKey]
	if !ok {
		return domain.ErrNotFound
	}
	s.objects[newKey] = data
	delete(s.objects, oldKey)
	return nil
}

func (s *fakeStorage) PresignedURL(_ context.Context, _ domain.StorageKey, _ time.Duration) (string, error) {
	return "", errors.New("fake storage: presigned URLs not supported")
}

var _ ports.StoragePort = (*fakeStorage)(nil)

// --- fakeShareRepository ---

type fakeShareRepository struct {
	byID map[uuid.UUID]*domain.ShareLink
}

func newFakeShareRepository() *fakeShareRepository {
	return &fakeShareRepository{byID: make(map[uuid.UUID]*domain.ShareLink)}
}

func (r *fakeShareRepository) Save(_ context.Context, s *domain.ShareLink) error {
	clone := *s
	r.byID[s.ID] = &clone
	return nil
}

func (r *fakeShareRepository) FindByID(_ context.Context, id uuid.UUID) (*domain.ShareLink, error) {
	s, ok := r.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	clone := *s
	return &clone, nil
}

func (r *fakeShareRepository) FindByToken(_ context.Context, token string) (*domain.ShareLink, error) {
	for _, s := range r.byID {
		if s.Token == token {
			clone := *s
			return &clone, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *fakeShareRepository) ListByFile(_ context.Context, fileID uuid.UUID) ([]*domain.ShareLink, error) {
	var out []*domain.ShareLink
	for _, s := range r.byID {
		if s.FileID != nil && *s.FileID == fileID {
			clone := *s
			out = append(out, &clone)
		}
	}
	return out, nil
}

func (r *fakeShareRepository) ListByFolder(_ context.Context, folderID uuid.UUID) ([]*domain.ShareLink, error) {
	var out []*domain.ShareLink
	for _, s := range r.byID {
		if s.FolderID != nil && *s.FolderID == folderID {
			clone := *s
			out = append(out, &clone)
		}
	}
	return out, nil
}

func (r *fakeShareRepository) IncrementDownloadCount(_ context.Context, id uuid.UUID) error {
	s, ok := r.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	s.DownloadCount++
	return nil
}

func (r *fakeShareRepository) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := r.byID[id]; !ok {
		return domain.ErrNotFound
	}
	delete(r.byID, id)
	return nil
}

var _ ports.ShareRepository = (*fakeShareRepository)(nil)

// --- fakeUserRepository ---

type fakeUserRepository struct {
	byID map[uuid.UUID]*domain.User
}

func newFakeUserRepository() *fakeUserRepository {
	return &fakeUserRepository{byID: make(map[uuid.UUID]*domain.User)}
}

func (r *fakeUserRepository) Save(_ context.Context, u *domain.User) error {
	clone := *u
	r.byID[u.ID] = &clone
	return nil
}

func (r *fakeUserRepository) FindByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	u, ok := r.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	clone := *u
	return &clone, nil
}

func (r *fakeUserRepository) FindByEmail(_ context.Context, email string) (*domain.User, error) {
	for _, u := range r.byID {
		if u.Email == email {
			clone := *u
			return &clone, nil
		}
	}
	return nil, domain.ErrNotFound
}

var _ ports.UserRepository = (*fakeUserRepository)(nil)

// --- fakePasswordHasher ---

// fakePasswordHasher is a deterministic, non-cryptographic stand-in: the
// "hash" is just the plaintext prefixed with a marker, fast for tests.
type fakePasswordHasher struct{}

func (fakePasswordHasher) Hash(plain string) (string, error) {
	return "hashed:" + plain, nil
}

func (fakePasswordHasher) Verify(hash, plain string) error {
	if hash != "hashed:"+plain {
		return domain.ErrInvalidPassword
	}
	return nil
}

var _ ports.PasswordHasher = fakePasswordHasher{}

// --- fakeAuthProvider ---

type fakeAuthProvider struct {
	name    string
	info    ports.ProviderUserInfo
	exchErr error
}

func (p *fakeAuthProvider) Name() string { return p.name }

func (p *fakeAuthProvider) AuthorizeURL(state string) string {
	return "https://example.com/" + p.name + "/authorize?state=" + state
}

func (p *fakeAuthProvider) ExchangeCode(_ context.Context, _ string) (ports.ProviderUserInfo, error) {
	if p.exchErr != nil {
		return ports.ProviderUserInfo{}, p.exchErr
	}
	return p.info, nil
}

var _ ports.AuthProvider = (*fakeAuthProvider)(nil)
