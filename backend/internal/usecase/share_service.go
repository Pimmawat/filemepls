package usecase

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"

	"filemepls/internal/domain"
	"filemepls/internal/ports"
)

type ShareService struct {
	files   ports.FileRepository
	folders ports.FolderRepository
	shares  ports.ShareRepository
	storage ports.StoragePort
	hasher  ports.PasswordHasher
}

func NewShareService(files ports.FileRepository, folders ports.FolderRepository, shares ports.ShareRepository, storage ports.StoragePort, hasher ports.PasswordHasher) *ShareService {
	return &ShareService{files: files, folders: folders, shares: shares, storage: storage, hasher: hasher}
}

func newToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashPasswordIfSet(hasher ports.PasswordHasher, plainPassword string) (*string, error) {
	if plainPassword == "" {
		return nil, nil
	}
	hash, err := hasher.Hash(plainPassword)
	if err != nil {
		return nil, fmt.Errorf("usecase: hash password: %w", err)
	}
	return &hash, nil
}

// shareParams bundles the user-configurable settings of a share link, used
// both to create one and to detect an existing duplicate.
type shareParams struct {
	visibility    domain.Visibility
	expiresAt     *time.Time
	maxDownloads  *int
	plainPassword string
}

// CreateShareLink requires ownerID to own fileID. If alias is non-empty it is
// used as the (guessable) custom token, rejected as taken if already in use by
// a different target. With no alias, a repeated request with identical
// settings returns the existing link instead of creating a duplicate.
func (s *ShareService) CreateShareLink(ctx context.Context, ownerID, fileID uuid.UUID, visibility domain.Visibility, expiresAt *time.Time, maxDownloads *int, plainPassword, alias string) (*domain.ShareLink, error) {
	f, err := s.files.FindByID(ctx, fileID)
	if err != nil {
		return nil, err
	}
	if err := f.EnsureOwnedBy(ownerID); err != nil {
		return nil, err
	}

	p := shareParams{visibility: visibility, expiresAt: expiresAt, maxDownloads: maxDownloads, plainPassword: plainPassword}
	build := func(token string) (*domain.ShareLink, error) {
		return domain.NewShareLinkForFile(token, fileID, visibility, expiresAt, maxDownloads)
	}

	if alias != "" {
		existing, err := s.resolveAlias(ctx, alias, func(sh *domain.ShareLink) bool {
			return sh.TargetType == domain.ShareTargetFile && sh.FileID != nil && *sh.FileID == fileID
		})
		if err != nil || existing != nil {
			return existing, err
		}
		return s.persistShare(ctx, build, alias, p)
	}

	list, err := s.shares.ListByFile(ctx, fileID)
	if err != nil {
		return nil, fmt.Errorf("usecase: list shares: %w", err)
	}
	if dup := s.findDuplicate(list, p); dup != nil {
		return dup, nil
	}
	token, err := newToken()
	if err != nil {
		return nil, fmt.Errorf("usecase: generate token: %w", err)
	}
	return s.persistShare(ctx, build, token, p)
}

// CreateFolderShareLink is the folder counterpart of CreateShareLink, with the
// same alias / de-duplication semantics.
func (s *ShareService) CreateFolderShareLink(ctx context.Context, ownerID, folderID uuid.UUID, visibility domain.Visibility, expiresAt *time.Time, maxDownloads *int, plainPassword, alias string) (*domain.ShareLink, error) {
	f, err := s.folders.FindByID(ctx, folderID)
	if err != nil {
		return nil, err
	}
	if err := f.EnsureOwnedBy(ownerID); err != nil {
		return nil, err
	}

	p := shareParams{visibility: visibility, expiresAt: expiresAt, maxDownloads: maxDownloads, plainPassword: plainPassword}
	build := func(token string) (*domain.ShareLink, error) {
		return domain.NewShareLinkForFolder(token, folderID, visibility, expiresAt, maxDownloads)
	}

	if alias != "" {
		existing, err := s.resolveAlias(ctx, alias, func(sh *domain.ShareLink) bool {
			return sh.TargetType == domain.ShareTargetFolder && sh.FolderID != nil && *sh.FolderID == folderID
		})
		if err != nil || existing != nil {
			return existing, err
		}
		return s.persistShare(ctx, build, alias, p)
	}

	list, err := s.shares.ListByFolder(ctx, folderID)
	if err != nil {
		return nil, fmt.Errorf("usecase: list shares: %w", err)
	}
	if dup := s.findDuplicate(list, p); dup != nil {
		return dup, nil
	}
	token, err := newToken()
	if err != nil {
		return nil, fmt.Errorf("usecase: generate token: %w", err)
	}
	return s.persistShare(ctx, build, token, p)
}

// resolveAlias validates a custom alias and checks whether it is already in
// use. Returns (existing, nil) when the alias already points at this same
// target (idempotent re-submit), (nil, ErrShareAliasTaken) when it belongs to
// something else, and (nil, nil) when it's free to claim.
func (s *ShareService) resolveAlias(ctx context.Context, alias string, ownedByTarget func(*domain.ShareLink) bool) (*domain.ShareLink, error) {
	if err := domain.ValidateShareAlias(alias); err != nil {
		return nil, err
	}
	existing, err := s.shares.FindByToken(ctx, alias)
	switch {
	case err == nil:
		if ownedByTarget(existing) {
			return existing, nil
		}
		return nil, domain.ErrShareAliasTaken
	case errors.Is(err, domain.ErrNotFound):
		return nil, nil
	default:
		return nil, fmt.Errorf("usecase: look up alias: %w", err)
	}
}

// persistShare builds a share link with the given token, sets its password
// hash, and saves it. A unique-violation on the token surfaces as
// ErrShareAliasTaken (covers a race between resolveAlias and the insert).
func (s *ShareService) persistShare(ctx context.Context, build func(token string) (*domain.ShareLink, error), token string, p shareParams) (*domain.ShareLink, error) {
	share, err := build(token)
	if err != nil {
		return nil, err
	}
	passwordHash, err := hashPasswordIfSet(s.hasher, p.plainPassword)
	if err != nil {
		return nil, err
	}
	share.PasswordHash = passwordHash

	if err := s.shares.Save(ctx, share); err != nil {
		if errors.Is(err, domain.ErrShareAliasTaken) {
			return nil, domain.ErrShareAliasTaken
		}
		return nil, fmt.Errorf("usecase: save share link: %w", err)
	}
	return share, nil
}

// findDuplicate returns an existing, still-usable share link whose settings
// exactly match p, or nil. This makes creating a link idempotent: clicking
// "create" again with the same options hands back the same link instead of
// piling up duplicates.
func (s *ShareService) findDuplicate(list []*domain.ShareLink, p shareParams) *domain.ShareLink {
	now := time.Now()
	for _, sh := range list {
		if sh.IsExpired(now) || sh.IsDownloadLimitReached() {
			continue
		}
		if sh.Visibility != p.visibility {
			continue
		}
		if !timePtrEqual(sh.ExpiresAt, p.expiresAt) || !intPtrEqual(sh.MaxDownloads, p.maxDownloads) {
			continue
		}
		if p.plainPassword == "" {
			if sh.PasswordHash != nil {
				continue
			}
		} else {
			if sh.PasswordHash == nil || s.hasher.Verify(*sh.PasswordHash, p.plainPassword) != nil {
				continue
			}
		}
		return sh
	}
	return nil
}

func timePtrEqual(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}

func intPtrEqual(a, b *int) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// ListSharesForFile requires ownerID to own fileID, then returns all share
// links created for it (so a previously created link can be found again
// and revoked later, not just immediately after creation).
func (s *ShareService) ListSharesForFile(ctx context.Context, ownerID, fileID uuid.UUID) ([]*domain.ShareLink, error) {
	f, err := s.files.FindByID(ctx, fileID)
	if err != nil {
		return nil, err
	}
	if err := f.EnsureOwnedBy(ownerID); err != nil {
		return nil, err
	}
	return s.shares.ListByFile(ctx, fileID)
}

// ListSharesForFolder requires ownerID to own folderID.
func (s *ShareService) ListSharesForFolder(ctx context.Context, ownerID, folderID uuid.UUID) ([]*domain.ShareLink, error) {
	f, err := s.folders.FindByID(ctx, folderID)
	if err != nil {
		return nil, err
	}
	if err := f.EnsureOwnedBy(ownerID); err != nil {
		return nil, err
	}
	return s.shares.ListByFolder(ctx, folderID)
}

// GetPublicShare resolves a token to its ShareLink plus its target (File
// XOR Folder, matching share.TargetType). requesterID is the caller's
// authenticated user ID, or uuid.Nil for an anonymous visitor — a
// VisibilityPrivate share refuses anonymous requesters with
// domain.ErrAuthRequired before any other state is revealed. Otherwise
// surfaces domain.ErrShareExpired / domain.ErrDownloadLimitHit from the pure
// domain checks so callers can render the right state before a password
// attempt.
func (s *ShareService) GetPublicShare(ctx context.Context, token string, requesterID uuid.UUID) (share *domain.ShareLink, file *domain.File, folder *domain.Folder, err error) {
	share, err = s.shares.FindByToken(ctx, token)
	if err != nil {
		return nil, nil, nil, err
	}

	if share.Visibility == domain.VisibilityPrivate && requesterID == uuid.Nil {
		return share, nil, nil, domain.ErrAuthRequired
	}

	if share.IsExpired(time.Now()) {
		return share, nil, nil, domain.ErrShareExpired
	}
	if share.IsDownloadLimitReached() {
		return share, nil, nil, domain.ErrDownloadLimitHit
	}

	switch share.TargetType {
	case domain.ShareTargetFile:
		file, err = s.files.FindByID(ctx, *share.FileID)
		if err != nil {
			return share, nil, nil, err
		}
	case domain.ShareTargetFolder:
		folder, err = s.folders.FindByID(ctx, *share.FolderID)
		if err != nil {
			return share, nil, nil, err
		}
	}
	return share, file, folder, nil
}

func (s *ShareService) verifySharePassword(share *domain.ShareLink, plainPassword string) error {
	if !share.RequiresPassword() {
		return nil
	}
	if plainPassword == "" {
		return domain.ErrPasswordRequired
	}
	return s.hasher.Verify(*share.PasswordHash, plainPassword)
}

// VerifySharePassword checks a candidate password against token's share
// link without any download side effects (no IncrementDownloadCount) — used
// by the frontend to pre-flight-check a password before triggering the real
// download navigation, so a wrong password surfaces as an in-app error
// instead of a raw-JSON page replacing the app.
func (s *ShareService) VerifySharePassword(ctx context.Context, token, plainPassword string, requesterID uuid.UUID) error {
	share, _, _, err := s.GetPublicShare(ctx, token, requesterID)
	if err != nil {
		return err
	}
	return s.verifySharePassword(share, plainPassword)
}

// RedeemShareDownload re-checks expiry/limit (defense in depth against a
// race between GetPublicShare and the actual download), verifies the
// password if required, records the download, and streams the requested
// byte range from storage. Only valid for file-target shares.
func (s *ShareService) RedeemShareDownload(ctx context.Context, token, plainPassword, rangeHeader string, requesterID uuid.UUID) (stream io.ReadCloser, offset, contentLength, totalSize int64, partial bool, mime string, file *domain.File, err error) {
	share, f, _, err := s.GetPublicShare(ctx, token, requesterID)
	if err != nil {
		return nil, 0, 0, 0, false, "", nil, err
	}
	if share.TargetType != domain.ShareTargetFile {
		return nil, 0, 0, 0, false, "", nil, domain.ErrShareTargetMismatch
	}

	if err := s.verifySharePassword(share, plainPassword); err != nil {
		return nil, 0, 0, 0, false, "", nil, err
	}

	if err := share.RecordDownload(); err != nil {
		return nil, 0, 0, 0, false, "", nil, err
	}
	if err := s.shares.IncrementDownloadCount(ctx, share.ID); err != nil {
		return nil, 0, 0, 0, false, "", nil, fmt.Errorf("usecase: record download: %w", err)
	}

	off, length, isPartial, err := parseRange(rangeHeader, f.Size)
	if err != nil {
		return nil, 0, 0, 0, false, "", nil, err
	}

	key, err := f.StorageKey()
	if err != nil {
		return nil, 0, 0, 0, false, "", nil, err
	}

	rc, total, err := s.storage.GetRange(ctx, key, off, length)
	if err != nil {
		return nil, 0, 0, 0, false, "", nil, err
	}

	cl := length
	if cl <= 0 {
		cl = total - off
	}
	return rc, off, cl, total, isPartial, f.Mime, f, nil
}

// RedeemFolderFileDownload downloads a single file living inside a
// publicly shared folder (or one of its descendant subfolders) — the
// per-file counterpart to PrepareFolderShareZip's whole-folder download.
// Like RedeemShareDownload, validation (password, containment) happens
// before the download is recorded or any bytes are read.
func (s *ShareService) RedeemFolderFileDownload(ctx context.Context, token string, fileID uuid.UUID, plainPassword, rangeHeader string, requesterID uuid.UUID) (stream io.ReadCloser, offset, contentLength, totalSize int64, partial bool, mime string, file *domain.File, err error) {
	share, _, folder, err := s.GetPublicShare(ctx, token, requesterID)
	if err != nil {
		return nil, 0, 0, 0, false, "", nil, err
	}
	if share.TargetType != domain.ShareTargetFolder {
		return nil, 0, 0, 0, false, "", nil, domain.ErrShareTargetMismatch
	}
	if err := s.verifySharePassword(share, plainPassword); err != nil {
		return nil, 0, 0, 0, false, "", nil, err
	}

	f, err := s.files.FindByID(ctx, fileID)
	if err != nil {
		return nil, 0, 0, 0, false, "", nil, err
	}
	if f.ParentID == nil {
		return nil, 0, 0, 0, false, "", nil, domain.ErrNotFound
	}
	within, err := isWithinShare(ctx, s.folders, *f.ParentID, folder.ID)
	if err != nil {
		return nil, 0, 0, 0, false, "", nil, err
	}
	if !within {
		return nil, 0, 0, 0, false, "", nil, domain.ErrNotFound
	}

	if err := share.RecordDownload(); err != nil {
		return nil, 0, 0, 0, false, "", nil, err
	}
	if err := s.shares.IncrementDownloadCount(ctx, share.ID); err != nil {
		return nil, 0, 0, 0, false, "", nil, fmt.Errorf("usecase: record download: %w", err)
	}

	off, length, isPartial, err := parseRange(rangeHeader, f.Size)
	if err != nil {
		return nil, 0, 0, 0, false, "", nil, err
	}

	key, err := f.StorageKey()
	if err != nil {
		return nil, 0, 0, 0, false, "", nil, err
	}

	rc, total, err := s.storage.GetRange(ctx, key, off, length)
	if err != nil {
		return nil, 0, 0, 0, false, "", nil, err
	}

	cl := length
	if cl <= 0 {
		cl = total - off
	}
	return rc, off, cl, total, isPartial, f.Mime, f, nil
}

// isWithinShare reports whether candidateID is shareRootID itself or one of
// its descendants, by walking candidateID's ancestor chain upward. This is
// the security boundary for browsing/zipping a publicly shared folder's
// subtree: a visitor must never be able to reach a folder outside the
// shared subtree by guessing a different folder UUID.
func isWithinShare(ctx context.Context, folders ports.FolderRepository, candidateID, shareRootID uuid.UUID) (bool, error) {
	cur := candidateID
	for {
		if cur == shareRootID {
			return true, nil
		}
		f, err := folders.FindByID(ctx, cur)
		if err != nil {
			return false, err
		}
		if f.ParentID == nil {
			return false, nil
		}
		cur = *f.ParentID
	}
}

// BrowsePublicFolder browses a publicly shared folder or one of its
// descendant subfolders (subFolderID nil = the share's own root). The
// breadcrumb is scoped to stop at the share's root, never revealing
// anything above it. Returns domain.ErrNotFound (not a distinguishing
// error) if subFolderID isn't actually within the shared subtree, so a
// visitor can't tell the difference between "doesn't exist" and "exists
// but isn't shared".
func (s *ShareService) BrowsePublicFolder(ctx context.Context, token string, subFolderID *uuid.UUID, requesterID uuid.UUID) (*BrowseResult, error) {
	share, _, folder, err := s.GetPublicShare(ctx, token, requesterID)
	if err != nil {
		return nil, err
	}
	if share.TargetType != domain.ShareTargetFolder {
		return nil, domain.ErrShareTargetMismatch
	}

	browseID := folder.ID
	if subFolderID != nil {
		within, err := isWithinShare(ctx, s.folders, *subFolderID, folder.ID)
		if err != nil {
			return nil, err
		}
		if !within {
			return nil, domain.ErrNotFound
		}
		browseID = *subFolderID
	}

	return browseFolder(ctx, s.folders, s.files, folder.OwnerID, &browseID, &folder.ID)
}

// PrepareFolderShareZip verifies the password (if required), validates
// subFolderID is within the shared subtree (if provided), and records
// exactly one download against the share link — all BEFORE any bytes are
// streamed, so the HTTP handler can still respond with a normal JSON error
// if anything fails rather than committing to a 200 response first. Only
// valid for folder-target shares. Returns the resolved folder's owner, the
// target folder ID to stream, and its display name (for Content-Disposition).
func (s *ShareService) PrepareFolderShareZip(ctx context.Context, token, plainPassword string, subFolderID *uuid.UUID, requesterID uuid.UUID) (ownerID, folderID uuid.UUID, folderName string, err error) {
	share, _, folder, err := s.GetPublicShare(ctx, token, requesterID)
	if err != nil {
		return uuid.Nil, uuid.Nil, "", err
	}
	if share.TargetType != domain.ShareTargetFolder {
		return uuid.Nil, uuid.Nil, "", domain.ErrShareTargetMismatch
	}
	if err := s.verifySharePassword(share, plainPassword); err != nil {
		return uuid.Nil, uuid.Nil, "", err
	}

	targetFolderID, targetFolderName := folder.ID, folder.Name
	if subFolderID != nil {
		within, err := isWithinShare(ctx, s.folders, *subFolderID, folder.ID)
		if err != nil {
			return uuid.Nil, uuid.Nil, "", err
		}
		if !within {
			return uuid.Nil, uuid.Nil, "", domain.ErrNotFound
		}
		sub, err := s.folders.FindByID(ctx, *subFolderID)
		if err != nil {
			return uuid.Nil, uuid.Nil, "", err
		}
		targetFolderID, targetFolderName = sub.ID, sub.Name
	}

	if err := share.RecordDownload(); err != nil {
		return uuid.Nil, uuid.Nil, "", err
	}
	if err := s.shares.IncrementDownloadCount(ctx, share.ID); err != nil {
		return uuid.Nil, uuid.Nil, "", fmt.Errorf("usecase: record download: %w", err)
	}

	return folder.OwnerID, targetFolderID, targetFolderName, nil
}

// StreamZip streams folderID's (owned by ownerID) entire recursive
// contents to w as a ZIP archive. Call only after PrepareFolderShareZip
// (or FolderService.PrepareZip, for the owner-authenticated path) has
// confirmed access — this method does no access control itself.
func (s *ShareService) StreamZip(ctx context.Context, ownerID, folderID uuid.UUID, w io.Writer) error {
	return streamFolderZip(ctx, w, s.storage, s.files, s.folders, ownerID, folderID)
}

// RevokeShareLink requires ownerID to own the share link's underlying file
// or folder.
func (s *ShareService) RevokeShareLink(ctx context.Context, ownerID, shareID uuid.UUID) error {
	share, err := s.shares.FindByID(ctx, shareID)
	if err != nil {
		return err
	}

	switch share.TargetType {
	case domain.ShareTargetFile:
		f, err := s.files.FindByID(ctx, *share.FileID)
		if err != nil {
			return err
		}
		if err := f.EnsureOwnedBy(ownerID); err != nil {
			return err
		}
	case domain.ShareTargetFolder:
		f, err := s.folders.FindByID(ctx, *share.FolderID)
		if err != nil {
			return err
		}
		if err := f.EnsureOwnedBy(ownerID); err != nil {
			return err
		}
	}

	return s.shares.Delete(ctx, shareID)
}
