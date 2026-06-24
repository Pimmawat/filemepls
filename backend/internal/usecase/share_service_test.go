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

func newTestShareService(t *testing.T) (*ShareService, *FileService, uuid.UUID, *domain.File) {
	t.Helper()
	ctx := context.Background()
	fileSvc, _, _, _ := newTestFileService()
	owner := uuid.New()
	f, err := fileSvc.Upload(ctx, owner, "text/plain", "file.txt", nil, strings.NewReader("share me"))
	if err != nil {
		t.Fatalf("Upload() error: %v", err)
	}

	files := newFakeFileRepository()
	files.byID[f.ID] = f
	folders := newFakeFolderRepository()
	shares := newFakeShareRepository()
	storage := newFakeStorage()
	key, _ := f.StorageKey()
	storage.objects[key] = []byte("share me")

	shareSvc := NewShareService(files, folders, shares, storage, fakePasswordHasher{})
	return shareSvc, fileSvc, owner, f
}

func TestShareService_CreateShareLink_EnforcesOwnership(t *testing.T) {
	svc, _, owner, f := newTestShareService(t)
	other := uuid.New()

	if _, err := svc.CreateShareLink(context.Background(), owner, f.ID, domain.VisibilityPublic, nil, nil, ""); err != nil {
		t.Fatalf("CreateShareLink() by owner: unexpected error: %v", err)
	}
	if _, err := svc.CreateShareLink(context.Background(), other, f.ID, domain.VisibilityPublic, nil, nil, ""); !errors.Is(err, domain.ErrNotOwner) {
		t.Errorf("got %v, want %v", err, domain.ErrNotOwner)
	}
}

func TestShareService_CreateShareLink_HashesPassword(t *testing.T) {
	svc, _, owner, f := newTestShareService(t)

	share, err := svc.CreateShareLink(context.Background(), owner, f.ID, domain.VisibilityUnlisted, nil, nil, "secret")
	if err != nil {
		t.Fatalf("CreateShareLink() error: %v", err)
	}
	if share.PasswordHash == nil || *share.PasswordHash == "secret" {
		t.Errorf("expected password to be hashed, got %v", share.PasswordHash)
	}
	if !share.RequiresPassword() {
		t.Error("expected RequiresPassword() to be true")
	}
}

func TestShareService_GetPublicShare_States(t *testing.T) {
	ctx := context.Background()
	svc, _, owner, f := newTestShareService(t)

	t.Run("ok", func(t *testing.T) {
		share, err := svc.CreateShareLink(ctx, owner, f.ID, domain.VisibilityPublic, nil, nil, "")
		if err != nil {
			t.Fatalf("CreateShareLink() error: %v", err)
		}
		_, gotFile, _, err := svc.GetPublicShare(ctx, share.Token)
		if err != nil {
			t.Fatalf("GetPublicShare() error: %v", err)
		}
		if gotFile.ID != f.ID {
			t.Errorf("got file %v, want %v", gotFile.ID, f.ID)
		}
	})

	t.Run("expired", func(t *testing.T) {
		past := time.Now().Add(-time.Hour)
		share, err := svc.CreateShareLink(ctx, owner, f.ID, domain.VisibilityPublic, &past, nil, "")
		if err != nil {
			t.Fatalf("CreateShareLink() error: %v", err)
		}
		_, _, _, err = svc.GetPublicShare(ctx, share.Token)
		if !errors.Is(err, domain.ErrShareExpired) {
			t.Errorf("got %v, want %v", err, domain.ErrShareExpired)
		}
	})

	t.Run("limit reached", func(t *testing.T) {
		zero := 0
		share, err := svc.CreateShareLink(ctx, owner, f.ID, domain.VisibilityPublic, nil, &zero, "")
		if err != nil {
			t.Fatalf("CreateShareLink() error: %v", err)
		}
		_, _, _, err = svc.GetPublicShare(ctx, share.Token)
		if !errors.Is(err, domain.ErrDownloadLimitHit) {
			t.Errorf("got %v, want %v", err, domain.ErrDownloadLimitHit)
		}
	})

	t.Run("unknown token", func(t *testing.T) {
		_, _, _, err := svc.GetPublicShare(ctx, "no-such-token")
		if !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("got %v, want %v", err, domain.ErrNotFound)
		}
	})
}

func TestShareService_RedeemShareDownload(t *testing.T) {
	ctx := context.Background()
	svc, _, owner, f := newTestShareService(t)

	t.Run("no password required", func(t *testing.T) {
		share, err := svc.CreateShareLink(ctx, owner, f.ID, domain.VisibilityPublic, nil, nil, "")
		if err != nil {
			t.Fatalf("CreateShareLink() error: %v", err)
		}
		stream, _, cl, total, _, _, gotFile, err := svc.RedeemShareDownload(ctx, share.Token, "", "")
		if err != nil {
			t.Fatalf("RedeemShareDownload() error: %v", err)
		}
		defer func() { _ = stream.Close() }()
		if cl != total || gotFile.ID != f.ID {
			t.Errorf("cl=%d total=%d gotFile=%v", cl, total, gotFile.ID)
		}
		data, _ := io.ReadAll(stream)
		if string(data) != "share me" {
			t.Errorf("data = %q", data)
		}
	})

	t.Run("wrong password rejected", func(t *testing.T) {
		share, err := svc.CreateShareLink(ctx, owner, f.ID, domain.VisibilityPublic, nil, nil, "secret")
		if err != nil {
			t.Fatalf("CreateShareLink() error: %v", err)
		}
		_, _, _, _, _, _, _, err = svc.RedeemShareDownload(ctx, share.Token, "wrong", "")
		if !errors.Is(err, domain.ErrInvalidPassword) {
			t.Errorf("got %v, want %v", err, domain.ErrInvalidPassword)
		}
	})

	t.Run("missing password rejected", func(t *testing.T) {
		share, err := svc.CreateShareLink(ctx, owner, f.ID, domain.VisibilityPublic, nil, nil, "secret")
		if err != nil {
			t.Fatalf("CreateShareLink() error: %v", err)
		}
		_, _, _, _, _, _, _, err = svc.RedeemShareDownload(ctx, share.Token, "", "")
		if !errors.Is(err, domain.ErrPasswordRequired) {
			t.Errorf("got %v, want %v", err, domain.ErrPasswordRequired)
		}
	})

	t.Run("correct password accepted and increments count", func(t *testing.T) {
		share, err := svc.CreateShareLink(ctx, owner, f.ID, domain.VisibilityPublic, nil, nil, "secret")
		if err != nil {
			t.Fatalf("CreateShareLink() error: %v", err)
		}
		stream, _, _, _, _, _, _, err := svc.RedeemShareDownload(ctx, share.Token, "secret", "")
		if err != nil {
			t.Fatalf("RedeemShareDownload() error: %v", err)
		}
		_ = stream.Close()

		_, _, _, err = svc.GetPublicShare(ctx, share.Token)
		if err != nil {
			t.Fatalf("GetPublicShare() after one download: unexpected error: %v", err)
		}
	})

	t.Run("download limit enforced across redemptions", func(t *testing.T) {
		one := 1
		share, err := svc.CreateShareLink(ctx, owner, f.ID, domain.VisibilityPublic, nil, &one, "")
		if err != nil {
			t.Fatalf("CreateShareLink() error: %v", err)
		}
		stream, _, _, _, _, _, _, err := svc.RedeemShareDownload(ctx, share.Token, "", "")
		if err != nil {
			t.Fatalf("first RedeemShareDownload() error: %v", err)
		}
		_ = stream.Close()

		_, _, _, _, _, _, _, err = svc.RedeemShareDownload(ctx, share.Token, "", "")
		if !errors.Is(err, domain.ErrDownloadLimitHit) {
			t.Errorf("second download: got %v, want %v", err, domain.ErrDownloadLimitHit)
		}
	})
}

func TestShareService_RevokeShareLink(t *testing.T) {
	ctx := context.Background()
	svc, _, owner, f := newTestShareService(t)
	other := uuid.New()

	share, err := svc.CreateShareLink(ctx, owner, f.ID, domain.VisibilityPublic, nil, nil, "")
	if err != nil {
		t.Fatalf("CreateShareLink() error: %v", err)
	}

	if err := svc.RevokeShareLink(ctx, other, share.ID); !errors.Is(err, domain.ErrNotOwner) {
		t.Errorf("got %v, want %v", err, domain.ErrNotOwner)
	}
	if err := svc.RevokeShareLink(ctx, owner, share.ID); err != nil {
		t.Fatalf("RevokeShareLink() by owner: unexpected error: %v", err)
	}
	if _, _, _, err := svc.GetPublicShare(ctx, share.Token); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected revoked share to be gone, got %v", err)
	}
}

func TestShareService_ListSharesForFile(t *testing.T) {
	ctx := context.Background()
	svc, _, owner, f := newTestShareService(t)
	other := uuid.New()

	if _, err := svc.CreateShareLink(ctx, owner, f.ID, domain.VisibilityPublic, nil, nil, ""); err != nil {
		t.Fatalf("CreateShareLink() error: %v", err)
	}
	if _, err := svc.CreateShareLink(ctx, owner, f.ID, domain.VisibilityUnlisted, nil, nil, "secret"); err != nil {
		t.Fatalf("CreateShareLink() error: %v", err)
	}

	list, err := svc.ListSharesForFile(ctx, owner, f.ID)
	if err != nil {
		t.Fatalf("ListSharesForFile() error: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("got %d shares, want 2", len(list))
	}

	if _, err := svc.ListSharesForFile(ctx, other, f.ID); !errors.Is(err, domain.ErrNotOwner) {
		t.Errorf("got %v, want %v", err, domain.ErrNotOwner)
	}
}
