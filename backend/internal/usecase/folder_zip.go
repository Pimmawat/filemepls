package usecase

import (
	"archive/zip"
	"context"
	"fmt"
	"io"

	"github.com/google/uuid"

	"filemepls/internal/ports"
)

// streamFolderZip writes a ZIP archive of folderID's entire recursive
// contents to w, streaming each file directly from storage. archive/zip
// writes each entry's compressed bytes as soon as they're produced (using
// a trailing data descriptor since totalSize isn't known up front), so
// neither a whole file nor the whole archive is ever buffered in memory —
// only the small, entry-count-proportional central directory is held
// until the final Close.
func streamFolderZip(ctx context.Context, w io.Writer, storage ports.StoragePort, fileRepo ports.FileRepository, folderRepo ports.FolderRepository, ownerID, folderID uuid.UUID) error {
	zw := zip.NewWriter(w)
	if err := walkAndZip(ctx, zw, storage, fileRepo, folderRepo, ownerID, folderID, ""); err != nil {
		_ = zw.Close()
		return err
	}
	return zw.Close()
}

func walkAndZip(ctx context.Context, zw *zip.Writer, storage ports.StoragePort, fileRepo ports.FileRepository, folderRepo ports.FolderRepository, ownerID, folderID uuid.UUID, prefix string) error {
	files, err := fileRepo.ListByParent(ctx, ownerID, &folderID)
	if err != nil {
		return fmt.Errorf("usecase: list files for zip: %w", err)
	}
	for _, f := range files {
		key, err := f.StorageKey()
		if err != nil {
			return err
		}
		rc, err := storage.Get(ctx, key)
		if err != nil {
			return fmt.Errorf("usecase: open file for zip: %w", err)
		}
		entry, err := zw.Create(prefix + f.Name)
		if err != nil {
			_ = rc.Close()
			return fmt.Errorf("usecase: create zip entry: %w", err)
		}
		_, copyErr := io.Copy(entry, rc)
		_ = rc.Close()
		if copyErr != nil {
			return fmt.Errorf("usecase: write zip entry: %w", copyErr)
		}
	}

	subfolders, err := folderRepo.ListChildren(ctx, ownerID, &folderID)
	if err != nil {
		return fmt.Errorf("usecase: list subfolders for zip: %w", err)
	}
	for _, sub := range subfolders {
		if err := walkAndZip(ctx, zw, storage, fileRepo, folderRepo, ownerID, sub.ID, prefix+sub.Name+"/"); err != nil {
			return err
		}
	}
	return nil
}
