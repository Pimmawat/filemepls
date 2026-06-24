package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"filemepls/internal/domain"
	"filemepls/internal/ports"
	"filemepls/internal/usecase"
)

// --- minimal in-memory port fakes, mirroring internal/usecase's test
// fakes but kept local since Go test fakes aren't exported across packages.

// sameUUIDPtrHTTP reports whether two optional UUIDs are equal, treating
// two nils as equal (parentID nil means root).
func sameUUIDPtrHTTP(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

type memFileRepo struct{ byID map[uuid.UUID]*domain.File }

func newMemFileRepo() *memFileRepo { return &memFileRepo{byID: map[uuid.UUID]*domain.File{}} }
func (r *memFileRepo) Save(_ context.Context, f *domain.File) error {
	c := *f
	r.byID[f.ID] = &c
	return nil
}
func (r *memFileRepo) FindByID(_ context.Context, id uuid.UUID) (*domain.File, error) {
	f, ok := r.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	c := *f
	return &c, nil
}
func (r *memFileRepo) ListByOwner(_ context.Context, ownerID uuid.UUID) ([]*domain.File, error) {
	var out []*domain.File
	for _, f := range r.byID {
		if f.OwnerID == ownerID {
			c := *f
			out = append(out, &c)
		}
	}
	return out, nil
}
func (r *memFileRepo) ListByParent(_ context.Context, ownerID uuid.UUID, parentID *uuid.UUID) ([]*domain.File, error) {
	var out []*domain.File
	for _, f := range r.byID {
		if f.OwnerID == ownerID && sameUUIDPtrHTTP(f.ParentID, parentID) {
			c := *f
			out = append(out, &c)
		}
	}
	return out, nil
}
func (r *memFileRepo) UpdateParent(_ context.Context, fileID uuid.UUID, parentID *uuid.UUID) error {
	f, ok := r.byID[fileID]
	if !ok {
		return domain.ErrNotFound
	}
	f.ParentID = parentID
	return nil
}
func (r *memFileRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := r.byID[id]; !ok {
		return domain.ErrNotFound
	}
	delete(r.byID, id)
	return nil
}
func (r *memFileRepo) CountByHash(_ context.Context, hash string) (int64, error) {
	var n int64
	for _, f := range r.byID {
		if f.Hash == hash {
			n++
		}
	}
	return n, nil
}

type memBlobRepo struct{ byHash map[string]*domain.Blob }

func newMemBlobRepo() *memBlobRepo { return &memBlobRepo{byHash: map[string]*domain.Blob{}} }
func (r *memBlobRepo) Save(_ context.Context, b *domain.Blob) error {
	c := *b
	r.byHash[b.Hash] = &c
	return nil
}
func (r *memBlobRepo) FindByHash(_ context.Context, hash string) (*domain.Blob, error) {
	b, ok := r.byHash[hash]
	if !ok {
		return nil, domain.ErrNotFound
	}
	c := *b
	return &c, nil
}
func (r *memBlobRepo) Delete(_ context.Context, hash string) error {
	delete(r.byHash, hash)
	return nil
}

type memFolderRepo struct{ byID map[uuid.UUID]*domain.Folder }

func newMemFolderRepo() *memFolderRepo { return &memFolderRepo{byID: map[uuid.UUID]*domain.Folder{}} }
func (r *memFolderRepo) Save(_ context.Context, f *domain.Folder) error {
	c := *f
	r.byID[f.ID] = &c
	return nil
}
func (r *memFolderRepo) FindByID(_ context.Context, id uuid.UUID) (*domain.Folder, error) {
	f, ok := r.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	c := *f
	return &c, nil
}
func (r *memFolderRepo) ListChildren(_ context.Context, ownerID uuid.UUID, parentID *uuid.UUID) ([]*domain.Folder, error) {
	var out []*domain.Folder
	for _, f := range r.byID {
		if f.OwnerID == ownerID && sameUUIDPtrHTTP(f.ParentID, parentID) {
			c := *f
			out = append(out, &c)
		}
	}
	return out, nil
}
func (r *memFolderRepo) UpdateParent(_ context.Context, folderID uuid.UUID, parentID *uuid.UUID) error {
	f, ok := r.byID[folderID]
	if !ok {
		return domain.ErrNotFound
	}
	f.ParentID = parentID
	return nil
}
func (r *memFolderRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := r.byID[id]; !ok {
		return domain.ErrNotFound
	}
	delete(r.byID, id)
	return nil
}

type memStorage struct{ objects map[domain.StorageKey][]byte }

func newMemStorage() *memStorage { return &memStorage{objects: map[domain.StorageKey][]byte{}} }
func (s *memStorage) Save(_ context.Context, key domain.StorageKey, r io.Reader) (int64, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return 0, err
	}
	s.objects[key] = data
	return int64(len(data)), nil
}
func (s *memStorage) Get(_ context.Context, key domain.StorageKey) (io.ReadCloser, error) {
	data, ok := s.objects[key]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}
func (s *memStorage) GetRange(_ context.Context, key domain.StorageKey, offset, length int64) (io.ReadCloser, int64, error) {
	data, ok := s.objects[key]
	if !ok {
		return nil, 0, domain.ErrNotFound
	}
	total := int64(len(data))
	end := total
	if length > 0 && offset+length < total {
		end = offset + length
	}
	return io.NopCloser(bytes.NewReader(data[offset:end])), total, nil
}
func (s *memStorage) Delete(_ context.Context, key domain.StorageKey) error {
	delete(s.objects, key)
	return nil
}
func (s *memStorage) Exists(_ context.Context, key domain.StorageKey) (bool, error) {
	_, ok := s.objects[key]
	return ok, nil
}
func (s *memStorage) Rename(_ context.Context, oldKey, newKey domain.StorageKey) error {
	data, ok := s.objects[oldKey]
	if !ok {
		return domain.ErrNotFound
	}
	s.objects[newKey] = data
	delete(s.objects, oldKey)
	return nil
}
func (s *memStorage) PresignedURL(_ context.Context, _ domain.StorageKey, _ time.Duration) (string, error) {
	return "", nil
}

type memUserRepo struct{ byID map[uuid.UUID]*domain.User }

func newMemUserRepo() *memUserRepo { return &memUserRepo{byID: map[uuid.UUID]*domain.User{}} }
func (r *memUserRepo) Save(_ context.Context, u *domain.User) error {
	c := *u
	r.byID[u.ID] = &c
	return nil
}
func (r *memUserRepo) Update(_ context.Context, u *domain.User) error {
	c := *u
	r.byID[u.ID] = &c
	return nil
}
func (r *memUserRepo) FindByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	u, ok := r.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	c := *u
	return &c, nil
}
func (r *memUserRepo) FindByEmail(_ context.Context, email string) (*domain.User, error) {
	for _, u := range r.byID {
		if u.Email == email {
			c := *u
			return &c, nil
		}
	}
	return nil, domain.ErrNotFound
}
func (r *memUserRepo) SearchByEmail(_ context.Context, query string, excludeID uuid.UUID, limit int) ([]*domain.User, error) {
	var out []*domain.User
	for _, u := range r.byID {
		if u.ID == excludeID {
			continue
		}
		if strings.Contains(strings.ToLower(u.Email), strings.ToLower(query)) {
			c := *u
			out = append(out, &c)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

type memAccessGrantRepo struct {
	byID    map[uuid.UUID]*domain.AccessGrant
	files   *memFileRepo
	folders *memFolderRepo
}

func newMemAccessGrantRepo(files *memFileRepo, folders *memFolderRepo) *memAccessGrantRepo {
	return &memAccessGrantRepo{byID: map[uuid.UUID]*domain.AccessGrant{}, files: files, folders: folders}
}
func (r *memAccessGrantRepo) Save(_ context.Context, g *domain.AccessGrant) error {
	c := *g
	r.byID[g.ID] = &c
	return nil
}
func (r *memAccessGrantRepo) FindByID(_ context.Context, id uuid.UUID) (*domain.AccessGrant, error) {
	g, ok := r.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	c := *g
	return &c, nil
}
func (r *memAccessGrantRepo) ListByFile(_ context.Context, fileID uuid.UUID) ([]*domain.AccessGrant, error) {
	var out []*domain.AccessGrant
	for _, g := range r.byID {
		if g.FileID != nil && *g.FileID == fileID {
			c := *g
			out = append(out, &c)
		}
	}
	return out, nil
}
func (r *memAccessGrantRepo) ListByFolder(_ context.Context, folderID uuid.UUID) ([]*domain.AccessGrant, error) {
	var out []*domain.AccessGrant
	for _, g := range r.byID {
		if g.FolderID != nil && *g.FolderID == folderID {
			c := *g
			out = append(out, &c)
		}
	}
	return out, nil
}
func (r *memAccessGrantRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := r.byID[id]; !ok {
		return domain.ErrNotFound
	}
	delete(r.byID, id)
	return nil
}
func (r *memAccessGrantRepo) HasFileAccess(_ context.Context, fileID, granteeID uuid.UUID) (bool, error) {
	for _, g := range r.byID {
		if g.FileID != nil && *g.FileID == fileID && g.GranteeID == granteeID {
			return true, nil
		}
	}
	return false, nil
}
func (r *memAccessGrantRepo) HasFolderAccess(_ context.Context, folderID, granteeID uuid.UUID) (bool, error) {
	for _, g := range r.byID {
		if g.FolderID != nil && *g.FolderID == folderID && g.GranteeID == granteeID {
			return true, nil
		}
	}
	return false, nil
}
func (r *memAccessGrantRepo) ListSharedFiles(_ context.Context, granteeID uuid.UUID) ([]*domain.File, error) {
	var out []*domain.File
	for _, g := range r.byID {
		if g.FileID == nil || g.GranteeID != granteeID {
			continue
		}
		if f, ok := r.files.byID[*g.FileID]; ok {
			c := *f
			out = append(out, &c)
		}
	}
	return out, nil
}
func (r *memAccessGrantRepo) ListSharedFolders(_ context.Context, granteeID uuid.UUID) ([]*domain.Folder, error) {
	var out []*domain.Folder
	for _, g := range r.byID {
		if g.FolderID == nil || g.GranteeID != granteeID {
			continue
		}
		if f, ok := r.folders.byID[*g.FolderID]; ok {
			c := *f
			out = append(out, &c)
		}
	}
	return out, nil
}

type memShareRepo struct {
	byID map[uuid.UUID]*domain.ShareLink
}

func newMemShareRepo() *memShareRepo { return &memShareRepo{byID: map[uuid.UUID]*domain.ShareLink{}} }
func (r *memShareRepo) Save(_ context.Context, s *domain.ShareLink) error {
	c := *s
	r.byID[s.ID] = &c
	return nil
}
func (r *memShareRepo) FindByID(_ context.Context, id uuid.UUID) (*domain.ShareLink, error) {
	s, ok := r.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	c := *s
	return &c, nil
}
func (r *memShareRepo) FindByToken(_ context.Context, token string) (*domain.ShareLink, error) {
	for _, s := range r.byID {
		if s.Token == token {
			c := *s
			return &c, nil
		}
	}
	return nil, domain.ErrNotFound
}
func (r *memShareRepo) ListByFile(_ context.Context, fileID uuid.UUID) ([]*domain.ShareLink, error) {
	var out []*domain.ShareLink
	for _, s := range r.byID {
		if s.FileID != nil && *s.FileID == fileID {
			c := *s
			out = append(out, &c)
		}
	}
	return out, nil
}
func (r *memShareRepo) ListByFolder(_ context.Context, folderID uuid.UUID) ([]*domain.ShareLink, error) {
	var out []*domain.ShareLink
	for _, s := range r.byID {
		if s.FolderID != nil && *s.FolderID == folderID {
			c := *s
			out = append(out, &c)
		}
	}
	return out, nil
}
func (r *memShareRepo) IncrementDownloadCount(_ context.Context, id uuid.UUID) error {
	s, ok := r.byID[id]
	if !ok {
		return domain.ErrNotFound
	}
	s.DownloadCount++
	return nil
}
func (r *memShareRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := r.byID[id]; !ok {
		return domain.ErrNotFound
	}
	delete(r.byID, id)
	return nil
}

type memHasher struct{}

func (memHasher) Hash(plain string) (string, error) { return "hashed:" + plain, nil }
func (memHasher) Verify(hash, plain string) error {
	if hash != "hashed:"+plain {
		return domain.ErrInvalidPassword
	}
	return nil
}

// memAuthProvider is a fake AuthProvider that always "exchanges" to the
// same fixed identity, so tests can mint a real session JWT for a known
// user without an actual OAuth round trip.
type memAuthProvider struct {
	name string
	info ports.ProviderUserInfo
}

func (p memAuthProvider) Name() string { return p.name }
func (p memAuthProvider) AuthorizeURL(state string) string {
	return "https://example.com?state=" + state
}
func (p memAuthProvider) ExchangeCode(context.Context, string) (ports.ProviderUserInfo, error) {
	return p.info, nil
}

// testServer wires a full router against in-memory fakes and returns the
// engine plus a ready-to-use session cookie for an authenticated test user.
type testServer struct {
	router        http.Handler
	sessionCookie string
	userID        uuid.UUID
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()

	files := newMemFileRepo()
	blobs := newMemBlobRepo()
	folders := newMemFolderRepo()
	storage := newMemStorage()
	users := newMemUserRepo()
	shares := newMemShareRepo()
	grants := newMemAccessGrantRepo(files, folders)

	fileSvc := usecase.NewFileService(files, blobs, folders, grants, storage, 1000, []string{"text/plain"})
	folderSvc := usecase.NewFolderService(folders, files, fileSvc, grants, storage)
	shareSvc := usecase.NewShareService(files, folders, shares, storage, memHasher{})
	permissionSvc := usecase.NewPermissionService(files, folders, grants, users)

	// The fake provider always "exchanges" to this fixed identity, so
	// HandleCallback mints a real session JWT for a known user without an
	// actual OAuth round trip (per the plan's testing approach: the rest of
	// the API is fully exercisable by minting a JWT this way).
	provider := memAuthProvider{name: "github", info: ports.ProviderUserInfo{Email: "test@example.com", DisplayName: "Test User"}}
	authSvc := usecase.NewAuthService(users, []ports.AuthProvider{provider}, []byte("test-secret"), time.Hour)

	router := NewRouter(Deps{
		Files:           fileSvc,
		Folders:         folderSvc,
		Shares:          shareSvc,
		Permissions:     permissionSvc,
		Auth:            authSvc,
		AllowedOrigins:  []string{"http://localhost:3000"},
		FrontendBaseURL: "http://localhost:3000",
		DefaultLocale:   "en",
		JWTTTL:          time.Hour,
	})

	token, user, err := authSvc.HandleCallback(context.Background(), "github", "irrelevant-code-since-fake-provider-ignores-it")
	if err != nil {
		t.Fatalf("HandleCallback() error: %v", err)
	}

	return &testServer{router: router, sessionCookie: token, userID: user.ID}
}

func (ts *testServer) do(t *testing.T, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	if ts.sessionCookie != "" {
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: ts.sessionCookie})
	}
	rec := httptest.NewRecorder()
	ts.router.ServeHTTP(rec, req)
	return rec
}

func TestHealthz(t *testing.T) {
	ts := newTestServer(t)
	rec := ts.do(t, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func multipartUpload(t *testing.T, content, mime string) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, err := w.CreatePart(map[string][]string{
		"Content-Disposition": {`form-data; name="file"; filename="test.txt"`},
		"Content-Type":        {mime},
	})
	if err != nil {
		t.Fatalf("CreatePart() error: %v", err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}
	return body, w.FormDataContentType()
}

func TestUploadListDownloadDelete(t *testing.T) {
	ts := newTestServer(t)

	body, contentType := multipartUpload(t, "hello world", "text/plain")
	req := httptest.NewRequest(http.MethodPost, "/api/files", body)
	req.Header.Set("Content-Type", contentType)
	rec := ts.do(t, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var created fileDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}
	if created.Size != 11 {
		t.Errorf("created.Size = %d, want 11", created.Size)
	}
	if created.Name != "test.txt" {
		t.Errorf("created.Name = %q, want %q", created.Name, "test.txt")
	}

	rec = ts.do(t, httptest.NewRequest(http.MethodGet, "/api/files", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d", rec.Code)
	}
	var list []fileDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}
	if len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("list = %+v, want exactly the created file", list)
	}

	rec = ts.do(t, httptest.NewRequest(http.MethodGet, "/api/files/"+created.ID.String()+"/download", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "hello world" {
		t.Fatalf("download status=%d body=%q", rec.Code, rec.Body.String())
	}
	// Original filename preserved as-is in Content-Disposition.
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, `filename="test.txt"`) {
		t.Errorf("Content-Disposition = %q, want original filename test.txt", cd)
	}

	rangeReq := httptest.NewRequest(http.MethodGet, "/api/files/"+created.ID.String()+"/download", nil)
	rangeReq.Header.Set("Range", "bytes=0-4")
	rec = ts.do(t, rangeReq)
	if rec.Code != http.StatusPartialContent || rec.Body.String() != "hello" {
		t.Fatalf("range download status=%d body=%q", rec.Code, rec.Body.String())
	}
	if cr := rec.Header().Get("Content-Range"); cr != "bytes 0-4/11" {
		t.Errorf("Content-Range = %q", cr)
	}

	rec = ts.do(t, httptest.NewRequest(http.MethodDelete, "/api/files/"+created.ID.String(), nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d", rec.Code)
	}

	rec = ts.do(t, httptest.NewRequest(http.MethodGet, "/api/files/"+created.ID.String(), nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("metadata after delete status = %d, want 404", rec.Code)
	}
}

func TestUpload_RequiresAuth(t *testing.T) {
	ts := newTestServer(t)
	ts.sessionCookie = ""

	body, contentType := multipartUpload(t, "hello", "text/plain")
	req := httptest.NewRequest(http.MethodPost, "/api/files", body)
	req.Header.Set("Content-Type", contentType)
	rec := ts.do(t, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestShareCreateAndPublicRedeem(t *testing.T) {
	ts := newTestServer(t)

	body, contentType := multipartUpload(t, "share this", "text/plain")
	req := httptest.NewRequest(http.MethodPost, "/api/files", body)
	req.Header.Set("Content-Type", contentType)
	rec := ts.do(t, req)
	var created fileDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	shareReq := bytes.NewBufferString(`{"visibility":"public"}`)
	req2 := httptest.NewRequest(http.MethodPost, "/api/files/"+created.ID.String()+"/shares", shareReq)
	req2.Header.Set("Content-Type", "application/json")
	rec = ts.do(t, req2)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create share status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var share shareLinkDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &share); err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}

	// Public info, no auth cookie.
	ts.sessionCookie = ""
	rec = ts.do(t, httptest.NewRequest(http.MethodGet, "/api/share/"+share.Token, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("public info status = %d", rec.Code)
	}
	var state publicShareStateResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &state)
	if state.Status != "ok" {
		t.Fatalf("state.Status = %q, want ok", state.Status)
	}

	rec = ts.do(t, httptest.NewRequest(http.MethodPost, "/api/share/"+share.Token+"/download", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "share this" {
		t.Fatalf("redeem status=%d body=%q", rec.Code, rec.Body.String())
	}
}

// TestListSharesThenRevoke covers finding a share link created in an
// earlier request (not just immediately after creation) via GET
// /api/files/:id/shares, then revoking it by ID.
func TestListSharesThenRevoke(t *testing.T) {
	ts := newTestServer(t)

	body, contentType := multipartUpload(t, "share this too", "text/plain")
	req := httptest.NewRequest(http.MethodPost, "/api/files", body)
	req.Header.Set("Content-Type", contentType)
	rec := ts.do(t, req)
	var created fileDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	shareReq := bytes.NewBufferString(`{"visibility":"unlisted"}`)
	req2 := httptest.NewRequest(http.MethodPost, "/api/files/"+created.ID.String()+"/shares", shareReq)
	req2.Header.Set("Content-Type", "application/json")
	rec = ts.do(t, req2)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create share status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var share shareLinkDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &share)

	// Simulate re-opening the share dialog in a later session: list shares
	// for the file and find the one created earlier.
	rec = ts.do(t, httptest.NewRequest(http.MethodGet, "/api/files/"+created.ID.String()+"/shares", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("list shares status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var list []shareLinkDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}
	if len(list) != 1 || list[0].ID != share.ID {
		t.Fatalf("list shares = %+v, want exactly the one created share", list)
	}

	rec = ts.do(t, httptest.NewRequest(http.MethodDelete, "/api/shares/"+share.ID.String(), nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("revoke status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = ts.do(t, httptest.NewRequest(http.MethodGet, "/api/files/"+created.ID.String()+"/shares", nil))
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 0 {
		t.Fatalf("list shares after revoke = %+v, want empty", list)
	}
}

func TestCORSHeaders(t *testing.T) {
	ts := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := ts.do(t, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("Access-Control-Allow-Origin = %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Access-Control-Allow-Credentials = %q", got)
	}
}
