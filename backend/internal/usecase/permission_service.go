package usecase

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"filemepls/internal/domain"
	"filemepls/internal/ports"
)

// AccessGrantView pairs a stored AccessGrant with the grantee's profile —
// the repository only stores IDs, so the service joins in the grantee's
// display info for the UI.
type AccessGrantView struct {
	Grant   *domain.AccessGrant
	Grantee *domain.User
}

type PermissionService struct {
	files   ports.FileRepository
	folders ports.FolderRepository
	grants  ports.AccessGrantRepository
	users   ports.UserRepository
}

func NewPermissionService(files ports.FileRepository, folders ports.FolderRepository, grants ports.AccessGrantRepository, users ports.UserRepository) *PermissionService {
	return &PermissionService{files: files, folders: folders, grants: grants, users: users}
}

// SearchUsers looks up users by email for the "assign permission" picker,
// excluding the caller themselves.
func (s *PermissionService) SearchUsers(ctx context.Context, callerID uuid.UUID, query string) ([]*domain.User, error) {
	return s.users.SearchByEmail(ctx, query, callerID, 10)
}

// GrantFileAccess requires ownerID to own fileID, then grants the user with
// granteeEmail view-only access to it.
func (s *PermissionService) GrantFileAccess(ctx context.Context, ownerID, fileID uuid.UUID, granteeEmail string) (*domain.AccessGrant, *domain.User, error) {
	f, err := s.files.FindByID(ctx, fileID)
	if err != nil {
		return nil, nil, err
	}
	if err := f.EnsureOwnedBy(ownerID); err != nil {
		return nil, nil, err
	}
	grantee, err := s.users.FindByEmail(ctx, granteeEmail)
	if err != nil {
		return nil, nil, err
	}
	grant := domain.NewFileAccessGrant(fileID, grantee.ID, ownerID)
	if err := s.grants.Save(ctx, grant); err != nil {
		return nil, nil, err
	}
	return grant, grantee, nil
}

// GrantFolderAccess requires ownerID to own folderID, then grants the user
// with granteeEmail view-only access to it (and, since Browse/Download
// honor grants cascading down the tree, everything inside it).
func (s *PermissionService) GrantFolderAccess(ctx context.Context, ownerID, folderID uuid.UUID, granteeEmail string) (*domain.AccessGrant, *domain.User, error) {
	f, err := s.folders.FindByID(ctx, folderID)
	if err != nil {
		return nil, nil, err
	}
	if err := f.EnsureOwnedBy(ownerID); err != nil {
		return nil, nil, err
	}
	grantee, err := s.users.FindByEmail(ctx, granteeEmail)
	if err != nil {
		return nil, nil, err
	}
	grant := domain.NewFolderAccessGrant(folderID, grantee.ID, ownerID)
	if err := s.grants.Save(ctx, grant); err != nil {
		return nil, nil, err
	}
	return grant, grantee, nil
}

// ListFileGrants requires ownerID to own fileID.
func (s *PermissionService) ListFileGrants(ctx context.Context, ownerID, fileID uuid.UUID) ([]AccessGrantView, error) {
	f, err := s.files.FindByID(ctx, fileID)
	if err != nil {
		return nil, err
	}
	if err := f.EnsureOwnedBy(ownerID); err != nil {
		return nil, err
	}
	grants, err := s.grants.ListByFile(ctx, fileID)
	if err != nil {
		return nil, err
	}
	return s.withGrantees(ctx, grants)
}

// ListFolderGrants requires ownerID to own folderID.
func (s *PermissionService) ListFolderGrants(ctx context.Context, ownerID, folderID uuid.UUID) ([]AccessGrantView, error) {
	f, err := s.folders.FindByID(ctx, folderID)
	if err != nil {
		return nil, err
	}
	if err := f.EnsureOwnedBy(ownerID); err != nil {
		return nil, err
	}
	grants, err := s.grants.ListByFolder(ctx, folderID)
	if err != nil {
		return nil, err
	}
	return s.withGrantees(ctx, grants)
}

func (s *PermissionService) withGrantees(ctx context.Context, grants []*domain.AccessGrant) ([]AccessGrantView, error) {
	out := make([]AccessGrantView, 0, len(grants))
	for _, g := range grants {
		grantee, err := s.users.FindByID(ctx, g.GranteeID)
		if err != nil {
			return nil, err
		}
		out = append(out, AccessGrantView{Grant: g, Grantee: grantee})
	}
	return out, nil
}

// RevokeGrant requires ownerID to own the grant's underlying file or
// folder.
func (s *PermissionService) RevokeGrant(ctx context.Context, ownerID, grantID uuid.UUID) error {
	grant, err := s.grants.FindByID(ctx, grantID)
	if err != nil {
		return err
	}

	switch grant.TargetType {
	case domain.AccessTargetFile:
		f, err := s.files.FindByID(ctx, *grant.FileID)
		if err != nil {
			return err
		}
		if err := f.EnsureOwnedBy(ownerID); err != nil {
			return err
		}
	case domain.AccessTargetFolder:
		f, err := s.folders.FindByID(ctx, *grant.FolderID)
		if err != nil {
			return err
		}
		if err := f.EnsureOwnedBy(ownerID); err != nil {
			return err
		}
	default:
		return fmt.Errorf("usecase: unknown access grant target type %q", grant.TargetType)
	}

	return s.grants.Delete(ctx, grantID)
}

// ListSharedWithMe returns the top-level files and folders directly granted
// to userID — not their descendants, since browsing into a shared folder
// already reveals its contents via FolderService.Browse, which honors
// grants cascading down the tree.
func (s *PermissionService) ListSharedWithMe(ctx context.Context, userID uuid.UUID) ([]*domain.File, []*domain.Folder, error) {
	files, err := s.grants.ListSharedFiles(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	folders, err := s.grants.ListSharedFolders(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	return files, folders, nil
}
