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

// newTestShareServiceWithFolders wires a ShareService and FolderService
// against the same fakes, for tests covering folder-target shares.
func newTestShareServiceWithFolders() (*ShareService, *FolderService, *fakeFolderRepository, *fakeShareRepository) {
	files := newFakeFileRepository()
	blobs := newFakeBlobRepository()
	folders := newFakeFolderRepository()
	storage := newFakeStorage()
	fileSvc := NewFileService(files, blobs, folders, storage, 0, []string{"*"})
	folderSvc := NewFolderService(folders, files, fileSvc, storage)
	shares := newFakeShareRepository()
	shareSvc := NewShareService(files, folders, shares, storage, fakePasswordHasher{})
	return shareSvc, folderSvc, folders, shares
}

func TestShareService_CreateFolderShareLink_EnforcesOwnership(t *testing.T) {
	ctx := context.Background()
	shareSvc, folderSvc, _, _ := newTestShareServiceWithFolders()
	owner, other := uuid.New(), uuid.New()

	folder, err := folderSvc.Create(ctx, owner, "Photos", nil)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if _, err := shareSvc.CreateFolderShareLink(ctx, other, folder.ID, domain.VisibilityPublic, nil, nil, ""); !errors.Is(err, domain.ErrNotOwner) {
		t.Errorf("got %v, want %v", err, domain.ErrNotOwner)
	}
	share, err := shareSvc.CreateFolderShareLink(ctx, owner, folder.ID, domain.VisibilityPublic, nil, nil, "")
	if err != nil {
		t.Fatalf("CreateFolderShareLink() by owner: unexpected error: %v", err)
	}
	if share.TargetType != domain.ShareTargetFolder || share.FolderID == nil || *share.FolderID != folder.ID {
		t.Errorf("unexpected share: %+v", share)
	}
}

func TestShareService_BrowsePublicFolder(t *testing.T) {
	ctx := context.Background()
	shareSvc, folderSvc, _, _ := newTestShareServiceWithFolders()
	owner := uuid.New()

	root, err := folderSvc.Create(ctx, owner, "Photos", nil)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	sub, err := folderSvc.Create(ctx, owner, "Vacation", &root.ID)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	share, err := shareSvc.CreateFolderShareLink(ctx, owner, root.ID, domain.VisibilityPublic, nil, nil, "")
	if err != nil {
		t.Fatalf("CreateFolderShareLink() error: %v", err)
	}

	t.Run("share root, empty breadcrumb (scoped to share)", func(t *testing.T) {
		result, err := shareSvc.BrowsePublicFolder(ctx, share.Token, nil)
		if err != nil {
			t.Fatalf("BrowsePublicFolder() error: %v", err)
		}
		if result.Folder == nil || result.Folder.ID != root.ID {
			t.Fatalf("expected folder = root, got %+v", result.Folder)
		}
		if len(result.Breadcrumb) != 1 || result.Breadcrumb[0].ID != root.ID {
			t.Errorf("expected breadcrumb to start at the share's own root, got %+v", result.Breadcrumb)
		}
		if len(result.Subfolders) != 1 || result.Subfolders[0].ID != sub.ID {
			t.Errorf("expected sub as a subfolder, got %+v", result.Subfolders)
		}
	})

	t.Run("descend into subfolder within share", func(t *testing.T) {
		result, err := shareSvc.BrowsePublicFolder(ctx, share.Token, &sub.ID)
		if err != nil {
			t.Fatalf("BrowsePublicFolder() error: %v", err)
		}
		if result.Folder == nil || result.Folder.ID != sub.ID {
			t.Fatalf("expected folder = sub, got %+v", result.Folder)
		}
		if len(result.Breadcrumb) != 2 {
			t.Errorf("expected breadcrumb [root, sub], got %+v", result.Breadcrumb)
		}
	})
}

// TestShareService_BrowsePublicFolder_ContainmentCheck is the critical
// security test: a visitor must not be able to browse a folder outside
// the shared subtree by guessing a different folder UUID, even one
// belonging to the same owner.
func TestShareService_BrowsePublicFolder_ContainmentCheck(t *testing.T) {
	ctx := context.Background()
	shareSvc, folderSvc, _, _ := newTestShareServiceWithFolders()
	owner := uuid.New()

	sharedRoot, err := folderSvc.Create(ctx, owner, "Shared", nil)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	// A sibling folder, owned by the same user, that is NOT part of the
	// share — must remain unreachable via the share token.
	secret, err := folderSvc.Create(ctx, owner, "Secret", nil)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	share, err := shareSvc.CreateFolderShareLink(ctx, owner, sharedRoot.ID, domain.VisibilityPublic, nil, nil, "")
	if err != nil {
		t.Fatalf("CreateFolderShareLink() error: %v", err)
	}

	if _, err := shareSvc.BrowsePublicFolder(ctx, share.Token, &secret.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("got %v, want %v (must not reveal that secret exists, just like a missing folder)", err, domain.ErrNotFound)
	}

	// Also must not allow zip-downloading the unrelated folder.
	if _, _, _, err := shareSvc.PrepareFolderShareZip(ctx, share.Token, "", &secret.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("zip: got %v, want %v", err, domain.ErrNotFound)
	}
}

func TestShareService_BrowsePublicFolder_PasswordGated(t *testing.T) {
	ctx := context.Background()
	shareSvc, folderSvc, _, _ := newTestShareServiceWithFolders()
	owner := uuid.New()

	root, err := folderSvc.Create(ctx, owner, "Photos", nil)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	share, err := shareSvc.CreateFolderShareLink(ctx, owner, root.ID, domain.VisibilityPublic, nil, nil, "secret")
	if err != nil {
		t.Fatalf("CreateFolderShareLink() error: %v", err)
	}

	if err := shareSvc.VerifySharePassword(ctx, share.Token, "wrong"); !errors.Is(err, domain.ErrInvalidPassword) {
		t.Errorf("got %v, want %v", err, domain.ErrInvalidPassword)
	}
	if err := shareSvc.VerifySharePassword(ctx, share.Token, "secret"); err != nil {
		t.Errorf("unexpected error verifying correct password: %v", err)
	}
}

func TestShareService_RedeemFolderFileDownload(t *testing.T) {
	ctx := context.Background()
	shareSvc, folderSvc, _, _ := newTestShareServiceWithFolders()
	owner := uuid.New()

	root, err := folderSvc.Create(ctx, owner, "Shared", nil)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	sub, err := folderSvc.Create(ctx, owner, "Sub", &root.ID)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	outside, err := folderSvc.Create(ctx, owner, "Outside", nil)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	fileSvc := folderSvc.files
	inside, err := fileSvc.Upload(ctx, owner, "text/plain", "nested.txt", &sub.ID, strings.NewReader("nested content"))
	if err != nil {
		t.Fatalf("Upload() error: %v", err)
	}
	outsideFile, err := fileSvc.Upload(ctx, owner, "text/plain", "secret.txt", &outside.ID, strings.NewReader("secret"))
	if err != nil {
		t.Fatalf("Upload() error: %v", err)
	}

	share, err := shareSvc.CreateFolderShareLink(ctx, owner, root.ID, domain.VisibilityPublic, nil, nil, "")
	if err != nil {
		t.Fatalf("CreateFolderShareLink() error: %v", err)
	}

	rc, _, _, _, _, mime, f, err := shareSvc.RedeemFolderFileDownload(ctx, share.Token, inside.ID, "", "")
	if err != nil {
		t.Fatalf("RedeemFolderFileDownload() error: %v", err)
	}
	data, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(data) != "nested content" || mime != "text/plain" || f.ID != inside.ID {
		t.Errorf("unexpected download: data=%q mime=%q fileID=%v", data, mime, f.ID)
	}

	// A file outside the shared subtree must be unreachable, even though
	// it belongs to the same owner.
	if _, _, _, _, _, _, _, err := shareSvc.RedeemFolderFileDownload(ctx, share.Token, outsideFile.ID, "", ""); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("got %v, want %v", err, domain.ErrNotFound)
	}
}

func TestShareService_RedeemFolderShareZip(t *testing.T) {
	ctx := context.Background()
	shareSvc, folderSvc, _, _ := newTestShareServiceWithFolders()
	owner := uuid.New()

	root, err := folderSvc.Create(ctx, owner, "Archive", nil)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	fileSvc := folderSvc.files
	if _, err := fileSvc.Upload(ctx, owner, "text/plain", "a.txt", &root.ID, strings.NewReader("alpha")); err != nil {
		t.Fatalf("Upload() error: %v", err)
	}

	one := 1
	share, err := shareSvc.CreateFolderShareLink(ctx, owner, root.ID, domain.VisibilityPublic, nil, &one, "")
	if err != nil {
		t.Fatalf("CreateFolderShareLink() error: %v", err)
	}

	ownerID, folderID, name, err := shareSvc.PrepareFolderShareZip(ctx, share.Token, "", nil)
	if err != nil {
		t.Fatalf("PrepareFolderShareZip() error: %v", err)
	}
	if name != "Archive" {
		t.Errorf("name = %q, want %q", name, "Archive")
	}

	var buf bytes.Buffer
	if err := shareSvc.StreamZip(ctx, ownerID, folderID, &buf); err != nil {
		t.Fatalf("StreamZip() error: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("zip.NewReader() error: %v", err)
	}
	if len(zr.File) != 1 || zr.File[0].Name != "a.txt" {
		t.Fatalf("unexpected zip contents: %+v", zr.File)
	}
	rc, _ := zr.File[0].Open()
	data, _ := io.ReadAll(rc)
	_ = rc.Close()
	if string(data) != "alpha" {
		t.Errorf("zip entry content = %q, want %q", data, "alpha")
	}

	// The download limit (1) must now be hit — exactly one download was
	// recorded for the whole zip, not once per file inside it.
	if _, _, _, err := shareSvc.PrepareFolderShareZip(ctx, share.Token, "", nil); !errors.Is(err, domain.ErrDownloadLimitHit) {
		t.Errorf("second zip attempt: got %v, want %v", err, domain.ErrDownloadLimitHit)
	}
}

func TestShareService_RedeemFolderShareZip_WrongPasswordRejectedBeforeStreaming(t *testing.T) {
	ctx := context.Background()
	shareSvc, folderSvc, _, _ := newTestShareServiceWithFolders()
	owner := uuid.New()

	root, err := folderSvc.Create(ctx, owner, "Archive", nil)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	share, err := shareSvc.CreateFolderShareLink(ctx, owner, root.ID, domain.VisibilityPublic, nil, nil, "secret")
	if err != nil {
		t.Fatalf("CreateFolderShareLink() error: %v", err)
	}

	_, _, _, err = shareSvc.PrepareFolderShareZip(ctx, share.Token, "wrong", nil)
	if !errors.Is(err, domain.ErrInvalidPassword) {
		t.Errorf("got %v, want %v", err, domain.ErrInvalidPassword)
	}
}

func TestShareService_ListSharesForFolder(t *testing.T) {
	ctx := context.Background()
	shareSvc, folderSvc, _, _ := newTestShareServiceWithFolders()
	owner, other := uuid.New(), uuid.New()

	root, err := folderSvc.Create(ctx, owner, "Photos", nil)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if _, err := shareSvc.CreateFolderShareLink(ctx, owner, root.ID, domain.VisibilityPublic, nil, nil, ""); err != nil {
		t.Fatalf("CreateFolderShareLink() error: %v", err)
	}

	list, err := shareSvc.ListSharesForFolder(ctx, owner, root.ID)
	if err != nil {
		t.Fatalf("ListSharesForFolder() error: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("got %d shares, want 1", len(list))
	}

	if _, err := shareSvc.ListSharesForFolder(ctx, other, root.ID); !errors.Is(err, domain.ErrNotOwner) {
		t.Errorf("got %v, want %v", err, domain.ErrNotOwner)
	}
}
