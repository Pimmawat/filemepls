package usecase

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/google/uuid"

	"filemepls/internal/domain"
)

func newTestFolderService() (*FolderService, *fakeFolderRepository, *fakeFileRepository, *fakeBlobRepository, *fakeStorage) {
	folders := newFakeFolderRepository()
	files := newFakeFileRepository()
	blobs := newFakeBlobRepository()
	storage := newFakeStorage()
	fileSvc := NewFileService(files, blobs, folders, storage, 0, []string{"*"})
	folderSvc := NewFolderService(folders, files, fileSvc, storage)
	return folderSvc, folders, files, blobs, storage
}

func TestFolderService_Create(t *testing.T) {
	ctx := context.Background()
	svc, _, _, _, _ := newTestFolderService()
	owner := uuid.New()

	root, err := svc.Create(ctx, owner, "Photos", nil)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if root.Name != "Photos" || root.ParentID != nil {
		t.Errorf("unexpected root folder: %+v", root)
	}

	child, err := svc.Create(ctx, owner, "Vacation", &root.ID)
	if err != nil {
		t.Fatalf("Create() nested error: %v", err)
	}
	if child.ParentID == nil || *child.ParentID != root.ID {
		t.Errorf("unexpected child folder: %+v", child)
	}
}

func TestFolderService_Create_EnforcesParentOwnership(t *testing.T) {
	ctx := context.Background()
	svc, _, _, _, _ := newTestFolderService()
	owner, other := uuid.New(), uuid.New()

	root, err := svc.Create(ctx, owner, "Photos", nil)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if _, err := svc.Create(ctx, other, "Intrusion", &root.ID); !errors.Is(err, domain.ErrNotOwner) {
		t.Errorf("got %v, want %v", err, domain.ErrNotOwner)
	}
}

func TestFolderService_Browse(t *testing.T) {
	ctx := context.Background()
	svc, _, files, _, _ := newTestFolderService()
	owner := uuid.New()

	root, err := svc.Create(ctx, owner, "Photos", nil)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	child, err := svc.Create(ctx, owner, "Vacation", &root.ID)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	f, err := files.byIDFileUpload(t, owner, &child.ID)
	if err != nil {
		t.Fatalf("upload error: %v", err)
	}

	t.Run("root", func(t *testing.T) {
		result, err := svc.Browse(ctx, owner, nil)
		if err != nil {
			t.Fatalf("Browse() error: %v", err)
		}
		if result.Folder != nil || len(result.Breadcrumb) != 0 {
			t.Errorf("expected root browse with no folder/breadcrumb, got %+v", result)
		}
		if len(result.Subfolders) != 1 || result.Subfolders[0].ID != root.ID {
			t.Errorf("expected exactly root folder as subfolder, got %+v", result.Subfolders)
		}
	})

	t.Run("nested with breadcrumb", func(t *testing.T) {
		result, err := svc.Browse(ctx, owner, &child.ID)
		if err != nil {
			t.Fatalf("Browse() error: %v", err)
		}
		if result.Folder == nil || result.Folder.ID != child.ID {
			t.Fatalf("expected folder = child, got %+v", result.Folder)
		}
		if len(result.Breadcrumb) != 2 || result.Breadcrumb[0].ID != root.ID || result.Breadcrumb[1].ID != child.ID {
			t.Errorf("unexpected breadcrumb: %+v", result.Breadcrumb)
		}
		if len(result.Files) != 1 || result.Files[0].ID != f.ID {
			t.Errorf("expected exactly the uploaded file, got %+v", result.Files)
		}
	})

	t.Run("ownership enforced", func(t *testing.T) {
		other := uuid.New()
		if _, err := svc.Browse(ctx, other, &child.ID); !errors.Is(err, domain.ErrNotOwner) {
			t.Errorf("got %v, want %v", err, domain.ErrNotOwner)
		}
	})
}

// byIDFileUpload is a small test helper that inserts a file record
// directly (bypassing FileService.Upload, since these tests are about
// folder browsing/deletion, not upload itself).
func (r *fakeFileRepository) byIDFileUpload(t *testing.T, ownerID uuid.UUID, parentID *uuid.UUID) (*domain.File, error) {
	t.Helper()
	f := &domain.File{
		ID:       uuid.New(),
		Hash:     "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
		Size:     5,
		Mime:     "text/plain",
		Name:     "note.txt",
		OwnerID:  ownerID,
		ParentID: parentID,
	}
	if err := r.Save(context.Background(), f); err != nil {
		return nil, err
	}
	return f, nil
}

func TestFolderService_Delete_RecursiveAndBlobSafe(t *testing.T) {
	ctx := context.Background()
	svc, folders, _, blobs, storage := newTestFolderService()
	owner := uuid.New()

	root, err := svc.Create(ctx, owner, "Photos", nil)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	child, err := svc.Create(ctx, owner, "Vacation", &root.ID)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	// Upload a file into the nested child folder via the real Upload path,
	// so it's properly deduped/blob-tracked.
	fileSvc := svc.files
	f, err := fileSvc.Upload(ctx, owner, "text/plain", "note.txt", &child.ID, strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("Upload() error: %v", err)
	}
	key, _ := f.StorageKey()

	if err := svc.Delete(ctx, owner, root.ID); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	if _, err := folders.FindByID(ctx, root.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected root folder to be gone, got %v", err)
	}
	if _, err := folders.FindByID(ctx, child.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected child folder to be gone, got %v", err)
	}
	if _, err := blobs.FindByHash(ctx, f.Hash); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected blob to be deleted (no other reference), got %v", err)
	}
	if exists, _ := storage.Exists(ctx, key); exists {
		t.Error("expected blob bytes to be deleted from disk — recursive delete must not leak orphaned blobs")
	}
}

func TestFolderService_Delete_KeepsBlobStillReferencedElsewhere(t *testing.T) {
	ctx := context.Background()
	svc, _, _, blobs, storage := newTestFolderService()
	owner := uuid.New()

	root, err := svc.Create(ctx, owner, "Photos", nil)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	fileSvc := svc.files
	inFolder, err := fileSvc.Upload(ctx, owner, "text/plain", "note.txt", &root.ID, strings.NewReader("shared"))
	if err != nil {
		t.Fatalf("Upload() error: %v", err)
	}
	// Same bytes uploaded again at root level (outside the folder being deleted).
	atRoot, err := fileSvc.Upload(ctx, owner, "text/plain", "note.txt", nil, strings.NewReader("shared"))
	if err != nil {
		t.Fatalf("Upload() error: %v", err)
	}
	if inFolder.Hash != atRoot.Hash {
		t.Fatal("expected identical content to dedup to the same hash")
	}

	if err := svc.Delete(ctx, owner, root.ID); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	if _, err := blobs.FindByHash(ctx, atRoot.Hash); err != nil {
		t.Errorf("expected blob to survive (still referenced by the root-level file), got %v", err)
	}
	key, _ := atRoot.StorageKey()
	if exists, _ := storage.Exists(ctx, key); !exists {
		t.Error("expected blob bytes to survive on disk (still referenced)")
	}
}

func TestFolderService_Delete_EnforcesOwnership(t *testing.T) {
	ctx := context.Background()
	svc, _, _, _, _ := newTestFolderService()
	owner, other := uuid.New(), uuid.New()

	root, err := svc.Create(ctx, owner, "Photos", nil)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if err := svc.Delete(ctx, other, root.ID); !errors.Is(err, domain.ErrNotOwner) {
		t.Errorf("got %v, want %v", err, domain.ErrNotOwner)
	}
}

func TestFolderService_MoveFile(t *testing.T) {
	ctx := context.Background()
	svc, _, files, _, _ := newTestFolderService()
	owner, other := uuid.New(), uuid.New()

	dest, err := svc.Create(ctx, owner, "Dest", nil)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	fileSvc := svc.files
	f, err := fileSvc.Upload(ctx, owner, "text/plain", "note.txt", nil, strings.NewReader("hi"))
	if err != nil {
		t.Fatalf("Upload() error: %v", err)
	}

	if err := svc.MoveFile(ctx, other, f.ID, &dest.ID); !errors.Is(err, domain.ErrNotOwner) {
		t.Errorf("got %v, want %v", err, domain.ErrNotOwner)
	}

	if err := svc.MoveFile(ctx, owner, f.ID, &dest.ID); err != nil {
		t.Fatalf("MoveFile() error: %v", err)
	}
	moved, err := files.FindByID(ctx, f.ID)
	if err != nil {
		t.Fatalf("FindByID() error: %v", err)
	}
	if moved.ParentID == nil || *moved.ParentID != dest.ID {
		t.Errorf("expected file to be moved into dest, got parentID %v", moved.ParentID)
	}
}

func TestFolderService_MoveFolder_PreventsCycles(t *testing.T) {
	ctx := context.Background()
	svc, folders, _, _, _ := newTestFolderService()
	owner := uuid.New()

	a, err := svc.Create(ctx, owner, "A", nil)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	b, err := svc.Create(ctx, owner, "B", &a.ID)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	c, err := svc.Create(ctx, owner, "C", &b.ID)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if err := svc.MoveFolder(ctx, owner, a.ID, &a.ID); !errors.Is(err, domain.ErrCyclicMove) {
		t.Errorf("self-move: got %v, want %v", err, domain.ErrCyclicMove)
	}
	if err := svc.MoveFolder(ctx, owner, a.ID, &c.ID); !errors.Is(err, domain.ErrCyclicMove) {
		t.Errorf("move into own descendant: got %v, want %v", err, domain.ErrCyclicMove)
	}

	// A valid move (C up to root) must still work.
	if err := svc.MoveFolder(ctx, owner, c.ID, nil); err != nil {
		t.Fatalf("valid MoveFolder() error: %v", err)
	}
	moved, err := folders.FindByID(ctx, c.ID)
	if err != nil {
		t.Fatalf("FindByID() error: %v", err)
	}
	if moved.ParentID != nil {
		t.Errorf("expected C to be at root after move, got parentID %v", moved.ParentID)
	}
}

func TestFolderService_DownloadZip(t *testing.T) {
	ctx := context.Background()
	svc, _, _, _, _ := newTestFolderService()
	owner := uuid.New()

	root, err := svc.Create(ctx, owner, "Archive", nil)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	sub, err := svc.Create(ctx, owner, "Sub", &root.ID)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	fileSvc := svc.files
	if _, err := fileSvc.Upload(ctx, owner, "text/plain", "top.txt", &root.ID, strings.NewReader("top level")); err != nil {
		t.Fatalf("Upload() error: %v", err)
	}
	if _, err := fileSvc.Upload(ctx, owner, "text/plain", "nested.txt", &sub.ID, strings.NewReader("nested")); err != nil {
		t.Fatalf("Upload() error: %v", err)
	}

	folder, err := svc.PrepareZip(ctx, owner, root.ID)
	if err != nil {
		t.Fatalf("PrepareZip() error: %v", err)
	}

	var buf bytes.Buffer
	if err := svc.StreamZip(ctx, owner, folder.ID, &buf); err != nil {
		t.Fatalf("StreamZip() error: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("zip.NewReader() error: %v", err)
	}

	got := map[string]string{}
	for _, entry := range zr.File {
		rc, err := entry.Open()
		if err != nil {
			t.Fatalf("entry.Open() error: %v", err)
		}
		data, _ := io.ReadAll(rc)
		_ = rc.Close()
		got[entry.Name] = string(data)
	}

	want := map[string]string{
		"top.txt":        "top level",
		"Sub/nested.txt": "nested",
	}
	for name, content := range want {
		if got[name] != content {
			t.Errorf("zip entry %q = %q, want %q (all entries: %v)", name, got[name], content, got)
		}
	}
}

func TestFolderService_DownloadZip_EnforcesOwnership(t *testing.T) {
	ctx := context.Background()
	svc, _, _, _, _ := newTestFolderService()
	owner, other := uuid.New(), uuid.New()

	root, err := svc.Create(ctx, owner, "Archive", nil)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if _, err := svc.PrepareZip(ctx, other, root.ID); !errors.Is(err, domain.ErrNotOwner) {
		t.Errorf("got %v, want %v", err, domain.ErrNotOwner)
	}
}
