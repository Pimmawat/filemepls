package usecase

import (
	"context"

	"github.com/google/uuid"

	"filemepls/internal/domain"
	"filemepls/internal/ports"
)

// folderAccess reports whether userID may view folder (already fetched by
// the caller), either as its owner or via a view-only access grant on the
// folder itself or one of its ancestors (grants cascade to descendants).
// When access comes from a grant (not ownership), grantRoot is the
// ancestor folder where that grant was found — callers use it to stop a
// breadcrumb walk there, so a grantee can never see folder names above the
// folder they were actually granted (mirrors how a public folder share's
// browse stays scoped to the share's own root).
func folderAccess(ctx context.Context, folders ports.FolderRepository, grants ports.AccessGrantRepository, folder *domain.Folder, userID uuid.UUID) (ok bool, grantRoot *uuid.UUID, err error) {
	if folder.OwnerID == userID {
		return true, nil, nil
	}
	cur := folder
	for {
		hasGrant, err := grants.HasFolderAccess(ctx, cur.ID, userID)
		if err != nil {
			return false, nil, err
		}
		if hasGrant {
			id := cur.ID
			return true, &id, nil
		}
		if cur.ParentID == nil {
			return false, nil, nil
		}
		next, err := folders.FindByID(ctx, *cur.ParentID)
		if err != nil {
			return false, nil, err
		}
		cur = next
	}
}

// fileAccess reports whether userID may view f (already fetched), either as
// its owner, via a direct grant on the file, or via a folderAccess grant on
// one of its ancestor folders.
func fileAccess(ctx context.Context, folders ports.FolderRepository, grants ports.AccessGrantRepository, f *domain.File, userID uuid.UUID) (bool, error) {
	if f.OwnerID == userID {
		return true, nil
	}
	hasGrant, err := grants.HasFileAccess(ctx, f.ID, userID)
	if err != nil {
		return false, err
	}
	if hasGrant {
		return true, nil
	}
	if f.ParentID == nil {
		return false, nil
	}
	parent, err := folders.FindByID(ctx, *f.ParentID)
	if err != nil {
		return false, err
	}
	ok, _, err := folderAccess(ctx, folders, grants, parent, userID)
	return ok, err
}
