package usecase

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"filemepls/internal/domain"
)

func newTestFileService() (*FileService, *fakeFileRepository, *fakeBlobRepository, *fakeStorage) {
	files := newFakeFileRepository()
	blobs := newFakeBlobRepository()
	folders := newFakeFolderRepository()
	storage := newFakeStorage()
	grants := newFakeAccessGrantRepository(files, folders)
	svc := NewFileService(files, blobs, folders, grants, storage, 1000, []string{"text/plain"})
	return svc, files, blobs, storage
}

func TestFileService_Upload(t *testing.T) {
	ctx := context.Background()
	svc, _, blobs, storage := newTestFileService()
	owner := uuid.New()

	f, err := svc.Upload(ctx, owner, "text/plain", "file.txt", nil, strings.NewReader("hello world"))
	if err != nil {
		t.Fatalf("Upload() error: %v", err)
	}
	if f.OwnerID != owner || f.Size != 11 || f.Mime != "text/plain" {
		t.Errorf("unexpected file: %+v", f)
	}

	blob, err := blobs.FindByHash(ctx, f.Hash)
	if err != nil {
		t.Fatalf("expected a blob record, got error: %v", err)
	}
	key, _ := blob.StorageKey()
	if exists, _ := storage.Exists(ctx, key); !exists {
		t.Error("expected blob bytes to exist in storage at the final key")
	}
}

func TestFileService_Upload_UnlimitedSizeAndWildcardMime(t *testing.T) {
	files := newFakeFileRepository()
	blobs := newFakeBlobRepository()
	folders := newFakeFolderRepository()
	storage := newFakeStorage()
	grants := newFakeAccessGrantRepository(files, folders)
	svc := NewFileService(files, blobs, folders, grants, storage, 0, []string{"*"}) // 0 = unlimited, "*" = any mime
	owner := uuid.New()

	big := strings.Repeat("a", 5000) // would exceed a maxSize of 1000
	f, err := svc.Upload(context.Background(), owner, "application/x-anything", "file.txt", nil, strings.NewReader(big))
	if err != nil {
		t.Fatalf("Upload() error: %v", err)
	}
	if f.Size != int64(len(big)) || f.Mime != "application/x-anything" {
		t.Errorf("unexpected file: %+v", f)
	}
}

func TestFileService_Upload_RejectsDisallowedMime(t *testing.T) {
	svc, _, _, _ := newTestFileService()
	_, err := svc.Upload(context.Background(), uuid.New(), "application/x-evil", "file.txt", nil, strings.NewReader("x"))
	if !errors.Is(err, domain.ErrUnsupportedMime) {
		t.Fatalf("got err %v, want %v", err, domain.ErrUnsupportedMime)
	}
}

// A client can't smuggle disallowed content past the allowlist by lying in the
// declared Content-Type: the real type is sniffed from the bytes and checked.
func TestFileService_Upload_RejectsMimeSpoofedByContent(t *testing.T) {
	files := newFakeFileRepository()
	blobs := newFakeBlobRepository()
	folders := newFakeFolderRepository()
	storage := newFakeStorage()
	grants := newFakeAccessGrantRepository(files, folders)
	svc := NewFileService(files, blobs, folders, grants, storage, 0, []string{"image/png"})

	html := "<html><body><script>alert(1)</script></body></html>"
	_, err := svc.Upload(context.Background(), uuid.New(), "image/png", "photo.png", nil, strings.NewReader(html))
	if !errors.Is(err, domain.ErrUnsupportedMime) {
		t.Fatalf("HTML declared as image/png: got %v, want %v", err, domain.ErrUnsupportedMime)
	}
	if len(storage.objects) != 0 {
		t.Errorf("expected nothing staged after a rejected spoofed upload, found %d objects", len(storage.objects))
	}
}

// A genuine PNG (magic bytes sniff to image/png) is accepted under an
// image/png-only allowlist — the sniff gate must not false-reject real content.
func TestFileService_Upload_AcceptsGenuinePNG(t *testing.T) {
	files := newFakeFileRepository()
	blobs := newFakeBlobRepository()
	folders := newFakeFolderRepository()
	storage := newFakeStorage()
	grants := newFakeAccessGrantRepository(files, folders)
	svc := NewFileService(files, blobs, folders, grants, storage, 0, []string{"image/png"})

	png := "\x89PNG\r\n\x1a\n" + strings.Repeat("\x00", 32) // header sniffs as image/png
	f, err := svc.Upload(context.Background(), uuid.New(), "image/png", "real.png", nil, strings.NewReader(png))
	if err != nil {
		t.Fatalf("genuine PNG upload: unexpected error: %v", err)
	}
	if f.Size != int64(len(png)) {
		t.Errorf("size = %d, want %d", f.Size, len(png))
	}
}

func TestFileService_Upload_RejectsOversized(t *testing.T) {
	svc, _, _, storage := newTestFileService()
	big := strings.Repeat("a", 2000) // maxSize is 1000

	_, err := svc.Upload(context.Background(), uuid.New(), "text/plain", "file.txt", nil, strings.NewReader(big))
	if !errors.Is(err, domain.ErrFileTooLarge) {
		t.Fatalf("got err %v, want %v", err, domain.ErrFileTooLarge)
	}
	if len(storage.objects) != 0 {
		t.Errorf("expected staged upload to be cleaned up, found %d objects", len(storage.objects))
	}
}

func TestFileService_Upload_DedupsAcrossOwners(t *testing.T) {
	ctx := context.Background()
	svc, files, blobs, storage := newTestFileService()
	ownerA, ownerB := uuid.New(), uuid.New()

	fileA, err := svc.Upload(ctx, ownerA, "text/plain", "file.txt", nil, strings.NewReader("same content"))
	if err != nil {
		t.Fatalf("Upload() error: %v", err)
	}
	fileB, err := svc.Upload(ctx, ownerB, "text/plain", "file.txt", nil, strings.NewReader("same content"))
	if err != nil {
		t.Fatalf("Upload() error: %v", err)
	}

	if fileA.ID == fileB.ID {
		t.Fatal("expected distinct File records per owner, got the same ID")
	}
	if fileA.Hash != fileB.Hash {
		t.Fatal("expected the same content hash for identical bytes")
	}

	// Owner B must see their own record in their list (the core requirement
	// driving the per-user-record design).
	listB, err := files.ListByOwner(ctx, ownerB)
	if err != nil || len(listB) != 1 || listB[0].ID != fileB.ID {
		t.Fatalf("ListByOwner(ownerB) = %+v, %v; want exactly fileB", listB, err)
	}

	// Only one blob/bytes-on-disk should exist despite two File records.
	blobCount := len(blobs.byHash)
	if blobCount != 1 {
		t.Errorf("expected exactly 1 deduped blob, got %d", blobCount)
	}
	objectCount := len(storage.objects)
	if objectCount != 1 {
		t.Errorf("expected exactly 1 stored object, got %d", objectCount)
	}
}

func TestFileService_PurgeOrphanBlobs(t *testing.T) {
	ctx := context.Background()
	svc, _, blobs, storage := newTestFileService()
	owner := uuid.New()

	// A live upload: its blob is referenced by a File and must survive.
	live, err := svc.Upload(ctx, owner, "text/plain", "keep.txt", nil, strings.NewReader("keep me"))
	if err != nil {
		t.Fatalf("Upload() error: %v", err)
	}

	// An orphan: a blob + bytes on disk with no File referencing it and an old
	// CreatedAt (past the grace period).
	orphan, err := domain.NewBlob(strings.Repeat("a", 64), 3, "text/plain")
	if err != nil {
		t.Fatalf("NewBlob() error: %v", err)
	}
	orphan.CreatedAt = time.Now().Add(-2 * time.Hour)
	if err := blobs.Save(ctx, orphan); err != nil {
		t.Fatalf("blobs.Save() error: %v", err)
	}
	orphanKey, _ := orphan.StorageKey()
	if _, err := storage.Save(ctx, orphanKey, strings.NewReader("abc")); err != nil {
		t.Fatalf("storage.Save() error: %v", err)
	}

	purged, err := svc.PurgeOrphanBlobs(ctx, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("PurgeOrphanBlobs() error: %v", err)
	}
	if purged != 1 {
		t.Errorf("purged = %d, want 1", purged)
	}

	// Orphan gone, both record and bytes.
	if _, err := blobs.FindByHash(ctx, orphan.Hash); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected orphan blob record deleted, got %v", err)
	}
	if exists, _ := storage.Exists(ctx, orphanKey); exists {
		t.Error("expected orphan blob bytes deleted from storage")
	}

	// Live blob untouched.
	if _, err := blobs.FindByHash(ctx, live.Hash); err != nil {
		t.Errorf("expected referenced blob to survive, got %v", err)
	}
}

func TestFileService_PurgeOrphanBlobs_RespectsGracePeriod(t *testing.T) {
	ctx := context.Background()
	svc, _, blobs, _ := newTestFileService()

	// A freshly-created orphan (recent CreatedAt) must be left alone so the
	// sweep can't race an upload mid-way between blob-save and file-save.
	fresh, _ := domain.NewBlob(strings.Repeat("b", 64), 3, "text/plain")
	_ = blobs.Save(ctx, fresh)

	purged, err := svc.PurgeOrphanBlobs(ctx, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("PurgeOrphanBlobs() error: %v", err)
	}
	if purged != 0 {
		t.Errorf("purged = %d, want 0 (within grace period)", purged)
	}
	if _, err := blobs.FindByHash(ctx, fresh.Hash); err != nil {
		t.Errorf("expected fresh blob to survive the grace period, got %v", err)
	}
}

func TestFileService_GetMetadata_EnforcesOwnership(t *testing.T) {
	ctx := context.Background()
	svc, _, _, _ := newTestFileService()
	owner := uuid.New()
	other := uuid.New()

	f, err := svc.Upload(ctx, owner, "text/plain", "file.txt", nil, strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("Upload() error: %v", err)
	}

	if _, err := svc.GetMetadata(ctx, owner, f.ID); err != nil {
		t.Errorf("GetMetadata() by owner: unexpected error: %v", err)
	}
	if _, err := svc.GetMetadata(ctx, other, f.ID); !errors.Is(err, domain.ErrNotOwner) {
		t.Errorf("GetMetadata() by non-owner: got %v, want %v", err, domain.ErrNotOwner)
	}
}

func TestFileService_Delete_KeepsBlobIfStillReferenced(t *testing.T) {
	ctx := context.Background()
	svc, files, blobs, storage := newTestFileService()
	ownerA, ownerB := uuid.New(), uuid.New()

	fileA, _ := svc.Upload(ctx, ownerA, "text/plain", "file.txt", nil, strings.NewReader("shared content"))
	fileB, err := svc.Upload(ctx, ownerB, "text/plain", "file.txt", nil, strings.NewReader("shared content"))
	if err != nil {
		t.Fatalf("Upload() error: %v", err)
	}

	if err := svc.Delete(ctx, ownerA, fileA.ID); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	if _, err := files.FindByID(ctx, fileA.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected fileA to be gone, got %v", err)
	}
	// fileB's owner should be unaffected; blob must still exist since fileB
	// still references it.
	if _, err := blobs.FindByHash(ctx, fileB.Hash); err != nil {
		t.Errorf("expected blob to survive (still referenced by fileB), got %v", err)
	}
	key, _ := fileB.StorageKey()
	if exists, _ := storage.Exists(ctx, key); !exists {
		t.Error("expected blob bytes to survive on disk (still referenced by fileB)")
	}
}

func TestFileService_Delete_RemovesBlobWhenLastReferenceGone(t *testing.T) {
	ctx := context.Background()
	svc, _, blobs, storage := newTestFileService()
	owner := uuid.New()

	f, err := svc.Upload(ctx, owner, "text/plain", "file.txt", nil, strings.NewReader("solo content"))
	if err != nil {
		t.Fatalf("Upload() error: %v", err)
	}
	key, _ := f.StorageKey()

	if err := svc.Delete(ctx, owner, f.ID); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	if _, err := blobs.FindByHash(ctx, f.Hash); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected blob to be deleted, got %v", err)
	}
	if exists, _ := storage.Exists(ctx, key); exists {
		t.Error("expected blob bytes to be deleted from storage")
	}
}

func TestFileService_Delete_EnforcesOwnership(t *testing.T) {
	ctx := context.Background()
	svc, _, _, _ := newTestFileService()
	owner, other := uuid.New(), uuid.New()

	f, _ := svc.Upload(ctx, owner, "text/plain", "file.txt", nil, strings.NewReader("hello"))
	if err := svc.Delete(ctx, other, f.ID); !errors.Is(err, domain.ErrNotOwner) {
		t.Errorf("got %v, want %v", err, domain.ErrNotOwner)
	}
}

func TestFileService_DownloadRange(t *testing.T) {
	ctx := context.Background()
	svc, _, _, _ := newTestFileService()
	owner := uuid.New()

	f, err := svc.Upload(ctx, owner, "text/plain", "file.txt", nil, strings.NewReader("0123456789"))
	if err != nil {
		t.Fatalf("Upload() error: %v", err)
	}

	t.Run("whole file", func(t *testing.T) {
		stream, offset, cl, total, partial, mime, name, _, err := svc.DownloadRange(ctx, owner, f.ID, "")
		if err != nil {
			t.Fatalf("DownloadRange() error: %v", err)
		}
		defer func() { _ = stream.Close() }()
		if partial || offset != 0 || cl != 10 || total != 10 || mime != "text/plain" || name != "file.txt" {
			t.Errorf("got offset=%d cl=%d total=%d partial=%v mime=%q name=%q", offset, cl, total, partial, mime, name)
		}
		data, _ := io.ReadAll(stream)
		if string(data) != "0123456789" {
			t.Errorf("data = %q", data)
		}
	})

	t.Run("partial range", func(t *testing.T) {
		stream, offset, cl, total, partial, _, _, _, err := svc.DownloadRange(ctx, owner, f.ID, "bytes=3-5")
		if err != nil {
			t.Fatalf("DownloadRange() error: %v", err)
		}
		defer func() { _ = stream.Close() }()
		if !partial || offset != 3 || cl != 3 || total != 10 {
			t.Errorf("got offset=%d cl=%d total=%d partial=%v", offset, cl, total, partial)
		}
		data, _ := io.ReadAll(stream)
		if string(data) != "345" {
			t.Errorf("data = %q, want %q", data, "345")
		}
	})

	t.Run("ownership enforced", func(t *testing.T) {
		_, _, _, _, _, _, _, _, err := svc.DownloadRange(ctx, uuid.New(), f.ID, "")
		if !errors.Is(err, domain.ErrNotOwner) {
			t.Errorf("got %v, want %v", err, domain.ErrNotOwner)
		}
	})
}
