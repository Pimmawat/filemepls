package usecase

import (
	"context"
	"fmt"
	"io"

	"github.com/google/uuid"

	"filemepls/internal/domain"
	"filemepls/internal/ports"
)

// BrowseResult is the contents of a folder (or root) ready for display:
// the folder itself (nil at root), its ancestor chain root-first including
// itself (empty at root), its direct subfolders, and its direct files.
type BrowseResult struct {
	Folder     *domain.Folder
	Breadcrumb []*domain.Folder
	Subfolders []*domain.Folder
	Files      []*domain.File
}

type FolderService struct {
	folders  ports.FolderRepository
	fileRepo ports.FileRepository
	files    *FileService // reused for blob-refcount-safe single-file deletion
	storage  ports.StoragePort
}

func NewFolderService(folders ports.FolderRepository, fileRepo ports.FileRepository, files *FileService, storage ports.StoragePort) *FolderService {
	return &FolderService{folders: folders, fileRepo: fileRepo, files: files, storage: storage}
}

// Create requires ownerID to own parentID (if non-nil).
func (s *FolderService) Create(ctx context.Context, ownerID uuid.UUID, name string, parentID *uuid.UUID) (*domain.Folder, error) {
	if parentID != nil {
		parent, err := s.folders.FindByID(ctx, *parentID)
		if err != nil {
			return nil, err
		}
		if err := parent.EnsureOwnedBy(ownerID); err != nil {
			return nil, err
		}
	}

	folder, err := domain.NewFolder(name, parentID, ownerID)
	if err != nil {
		return nil, err
	}
	if err := s.folders.Save(ctx, folder); err != nil {
		return nil, fmt.Errorf("usecase: save folder: %w", err)
	}
	return folder, nil
}

// Browse requires ownerID to own folderID (if non-nil; root is always
// browsable by its owner).
func (s *FolderService) Browse(ctx context.Context, ownerID uuid.UUID, folderID *uuid.UUID) (*BrowseResult, error) {
	if folderID != nil {
		folder, err := s.folders.FindByID(ctx, *folderID)
		if err != nil {
			return nil, err
		}
		if err := folder.EnsureOwnedBy(ownerID); err != nil {
			return nil, err
		}
	}
	return browseFolder(ctx, s.folders, s.fileRepo, ownerID, folderID, nil)
}

// buildBreadcrumb walks the ancestor chain of folderID via repeated
// FindByID, returning it root-first and including folderID itself. If
// stopAt is non-nil, the walk stops once stopAt is reached instead of
// continuing to the true root — used when browsing a publicly shared
// folder's subtree, which must never reveal anything above the share's
// own root folder.
func buildBreadcrumb(ctx context.Context, folders ports.FolderRepository, folderID uuid.UUID, stopAt *uuid.UUID) ([]*domain.Folder, error) {
	var chain []*domain.Folder
	cur := folderID
	for {
		f, err := folders.FindByID(ctx, cur)
		if err != nil {
			return nil, err
		}
		chain = append(chain, f)
		if stopAt != nil && f.ID == *stopAt {
			break
		}
		if f.ParentID == nil {
			break
		}
		cur = *f.ParentID
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain, nil
}

// browseFolder builds a BrowseResult for an already-authorized request
// (ownership or share-grant checked by the caller). breadcrumbStopAt is
// forwarded to buildBreadcrumb (nil = walk to the true root).
func browseFolder(ctx context.Context, folders ports.FolderRepository, fileRepo ports.FileRepository, ownerID uuid.UUID, folderID, breadcrumbStopAt *uuid.UUID) (*BrowseResult, error) {
	var folder *domain.Folder
	var breadcrumb []*domain.Folder
	if folderID != nil {
		f, err := folders.FindByID(ctx, *folderID)
		if err != nil {
			return nil, err
		}
		folder = f
		bc, err := buildBreadcrumb(ctx, folders, *folderID, breadcrumbStopAt)
		if err != nil {
			return nil, err
		}
		breadcrumb = bc
	}

	subfolders, err := folders.ListChildren(ctx, ownerID, folderID)
	if err != nil {
		return nil, fmt.Errorf("usecase: list subfolders: %w", err)
	}
	files, err := fileRepo.ListByParent(ctx, ownerID, folderID)
	if err != nil {
		return nil, fmt.Errorf("usecase: list files: %w", err)
	}
	return &BrowseResult{Folder: folder, Breadcrumb: breadcrumb, Subfolders: subfolders, Files: files}, nil
}

// Delete requires ownerID to own folderID, then recursively deletes every
// file and subfolder inside it. Each file is deleted via FileService.Delete
// (the exact same blob-refcount-safe logic as a single-file delete), so
// cascading delete can never leak an orphaned blob. Folder rows are deleted
// bottom-up (children before parents); the DB has no ON DELETE CASCADE on
// parent_id, so a bug that deletes a folder before it's empty fails loudly
// with a foreign-key violation instead of silently orphaning blobs. This is
// crash-safe but not transactional: re-invoking Delete on the same folderID
// after a partial run simply continues (already-removed children are
// skipped via ErrNotFound).
func (s *FolderService) Delete(ctx context.Context, ownerID, folderID uuid.UUID) error {
	folder, err := s.folders.FindByID(ctx, folderID)
	if err != nil {
		return err
	}
	if err := folder.EnsureOwnedBy(ownerID); err != nil {
		return err
	}
	if err := s.deleteContents(ctx, ownerID, folderID); err != nil {
		return err
	}
	return s.folders.Delete(ctx, folderID)
}

func (s *FolderService) deleteContents(ctx context.Context, ownerID, folderID uuid.UUID) error {
	files, err := s.fileRepo.ListByParent(ctx, ownerID, &folderID)
	if err != nil {
		return fmt.Errorf("usecase: list files for delete: %w", err)
	}
	for _, f := range files {
		if err := s.files.Delete(ctx, ownerID, f.ID); err != nil {
			return fmt.Errorf("usecase: delete file %s: %w", f.ID, err)
		}
	}

	children, err := s.folders.ListChildren(ctx, ownerID, &folderID)
	if err != nil {
		return fmt.Errorf("usecase: list subfolders for delete: %w", err)
	}
	for _, child := range children {
		if err := s.deleteContents(ctx, ownerID, child.ID); err != nil {
			return err
		}
		if err := s.folders.Delete(ctx, child.ID); err != nil {
			return fmt.Errorf("usecase: delete subfolder %s: %w", child.ID, err)
		}
	}
	return nil
}

// MoveFile requires ownerID to own fileID and (if non-nil) newParentID.
func (s *FolderService) MoveFile(ctx context.Context, ownerID, fileID uuid.UUID, newParentID *uuid.UUID) error {
	f, err := s.fileRepo.FindByID(ctx, fileID)
	if err != nil {
		return err
	}
	if err := f.EnsureOwnedBy(ownerID); err != nil {
		return err
	}
	if newParentID != nil {
		parent, err := s.folders.FindByID(ctx, *newParentID)
		if err != nil {
			return err
		}
		if err := parent.EnsureOwnedBy(ownerID); err != nil {
			return err
		}
	}
	return s.fileRepo.UpdateParent(ctx, fileID, newParentID)
}

// MoveFolder requires ownerID to own folderID and (if non-nil) newParentID,
// and rejects moving a folder into itself or into one of its own
// descendants (which would create a cycle).
func (s *FolderService) MoveFolder(ctx context.Context, ownerID, folderID uuid.UUID, newParentID *uuid.UUID) error {
	target, err := s.folders.FindByID(ctx, folderID)
	if err != nil {
		return err
	}
	if err := target.EnsureOwnedBy(ownerID); err != nil {
		return err
	}
	if newParentID == nil {
		return s.folders.UpdateParent(ctx, folderID, nil)
	}
	if *newParentID == folderID {
		return domain.ErrCyclicMove
	}

	parent, err := s.folders.FindByID(ctx, *newParentID)
	if err != nil {
		return err
	}
	if err := parent.EnsureOwnedBy(ownerID); err != nil {
		return err
	}

	cur := *newParentID
	for {
		f, err := s.folders.FindByID(ctx, cur)
		if err != nil {
			return err
		}
		if f.ID == folderID {
			return domain.ErrCyclicMove
		}
		if f.ParentID == nil {
			break
		}
		cur = *f.ParentID
	}

	return s.folders.UpdateParent(ctx, folderID, newParentID)
}

// PrepareZip requires ownerID to own folderID and returns it, without
// streaming anything yet — split from StreamZip so the HTTP handler can
// validate (and respond with a normal JSON error on failure) before
// committing to a 200 response and writing any bytes.
func (s *FolderService) PrepareZip(ctx context.Context, ownerID, folderID uuid.UUID) (*domain.Folder, error) {
	folder, err := s.folders.FindByID(ctx, folderID)
	if err != nil {
		return nil, err
	}
	if err := folder.EnsureOwnedBy(ownerID); err != nil {
		return nil, err
	}
	return folder, nil
}

// StreamZip streams folderID's entire recursive contents to w as a ZIP
// archive. Call only after PrepareZip has confirmed access.
func (s *FolderService) StreamZip(ctx context.Context, ownerID, folderID uuid.UUID, w io.Writer) error {
	return streamFolderZip(ctx, w, s.storage, s.fileRepo, s.folders, ownerID, folderID)
}
