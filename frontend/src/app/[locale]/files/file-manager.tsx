"use client";

import { useRef, useState } from "react";
import { useFormatter, useTranslations } from "next-intl";
import { Download } from "lucide-react";
import { toast } from "sonner";

import { useRouter } from "@/i18n/navigation";
import { Breadcrumb } from "@/components/breadcrumb";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Progress } from "@/components/ui/progress";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  api,
  ApiError,
  UploadCancelledError,
  type BrowseResult,
  type FileMeta,
  type FolderMeta,
} from "@/lib/api";
import { CreateShareDialog } from "./create-share-dialog";
import { FileDetailsDialog } from "./file-details-dialog";

const DRAG_MIME = "application/x-filemepls-item";

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let value = bytes / 1024;
  let i = 0;
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024;
    i++;
  }
  return `${value.toFixed(1)} ${units[i]}`;
}

export function FileManager({
  initialBrowse,
  folderId,
}: {
  initialBrowse: BrowseResult;
  folderId: string | null;
}) {
  const t = useTranslations("Files");
  const format = useFormatter();
  const router = useRouter();

  const [browse, setBrowse] = useState(initialBrowse);
  const [uploading, setUploading] = useState(false);
  const [uploadProgress, setUploadProgress] = useState(0);
  const [dragOver, setDragOver] = useState(false);
  const [detailsFileId, setDetailsFileId] = useState<string | null>(null);
  const [newFolderOpen, setNewFolderOpen] = useState(false);
  const [newFolderName, setNewFolderName] = useState("");
  const [creatingFolder, setCreatingFolder] = useState(false);

  const fileInputRef = useRef<HTMLInputElement>(null);
  const abortControllerRef = useRef<AbortController | null>(null);
  // Tracks the item currently being dragged (set on a row's onDragStart,
  // read by whichever drop target receives the drop) — simpler than
  // plumbing it through dataTransfer for every intermediate component.
  const draggedItemRef = useRef<{ type: "file" | "folder"; id: string } | null>(null);

  async function uploadInto(file: File, parentId: string | null) {
    const controller = new AbortController();
    abortControllerRef.current = controller;
    setUploading(true);
    setUploadProgress(0);
    try {
      const created = await api.uploadFile(
        file,
        parentId,
        (loaded, total) => setUploadProgress(Math.round((loaded / total) * 100)),
        controller.signal,
      );
      if (parentId === folderId) {
        setBrowse((prev) => ({ ...prev, files: [created, ...prev.files] }));
      }
    } catch (err) {
      if (err instanceof UploadCancelledError) {
        toast(t("uploadCancelled"));
      } else {
        toast.error(t("uploadFailed"), {
          description: err instanceof ApiError ? err.message : undefined,
        });
      }
    } finally {
      setUploading(false);
      abortControllerRef.current = null;
    }
  }

  async function handleFileSelected(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    await uploadInto(file, folderId);
    e.target.value = "";
  }

  function handleCancelUpload() {
    abortControllerRef.current?.abort();
  }

  async function handleDeleteFile(id: string) {
    try {
      await api.deleteFile(id);
      setBrowse((prev) => ({ ...prev, files: prev.files.filter((f) => f.id !== id) }));
      toast.success(t("deleted"));
    } catch (err) {
      toast.error(t("deleteFailed"), {
        description: err instanceof ApiError ? err.message : undefined,
      });
    }
  }

  async function handleDeleteFolder(id: string) {
    if (!window.confirm(t("deleteFolderConfirm"))) return;
    try {
      await api.deleteFolder(id);
      setBrowse((prev) => ({ ...prev, subfolders: prev.subfolders.filter((f) => f.id !== id) }));
      toast.success(t("folderDeleted"));
    } catch (err) {
      toast.error(t("folderDeleteFailed"), {
        description: err instanceof ApiError ? err.message : undefined,
      });
    }
  }

  async function handleCreateFolder() {
    if (!newFolderName.trim()) return;
    setCreatingFolder(true);
    try {
      const created = await api.createFolder(newFolderName.trim(), folderId);
      setBrowse((prev) => ({ ...prev, subfolders: [created, ...prev.subfolders] }));
      setNewFolderOpen(false);
      setNewFolderName("");
    } catch (err) {
      toast.error(t("folderCreateFailed"), {
        description: err instanceof ApiError ? err.message : undefined,
      });
    } finally {
      setCreatingFolder(false);
    }
  }

  async function moveDraggedItemTo(destFolderId: string | null) {
    const item = draggedItemRef.current;
    draggedItemRef.current = null;
    if (!item) return;
    try {
      if (item.type === "file") {
        await api.moveFile(item.id, destFolderId);
        setBrowse((prev) => ({ ...prev, files: prev.files.filter((f) => f.id !== item.id) }));
      } else {
        await api.moveFolder(item.id, destFolderId);
        setBrowse((prev) => ({
          ...prev,
          subfolders: prev.subfolders.filter((f) => f.id !== item.id),
        }));
      }
      toast.success(t("moved"));
    } catch (err) {
      toast.error(t("moveFailed"), {
        description: err instanceof ApiError ? err.message : undefined,
      });
    }
  }

  function handleDropOnFolderRow(e: React.DragEvent, folder: FolderMeta) {
    e.preventDefault();
    e.stopPropagation();
    if (e.dataTransfer.types.includes("Files")) {
      const file = e.dataTransfer.files[0];
      if (file) uploadInto(file, folder.id);
      return;
    }
    moveDraggedItemTo(folder.id);
  }

  function handlePageDrop(e: React.DragEvent) {
    e.preventDefault();
    setDragOver(false);
    if (e.dataTransfer.types.includes("Files")) {
      const file = e.dataTransfer.files[0];
      if (file) uploadInto(file, folderId);
    }
  }

  function navigateTo(id: string) {
    router.push(id ? `/files/${id}` : "/files");
  }

  return (
    <div
      className="flex flex-col gap-6"
      onDragOver={(e) => {
        if (e.dataTransfer.types.includes("Files")) {
          e.preventDefault();
          setDragOver(true);
        }
      }}
      onDragLeave={() => setDragOver(false)}
      onDrop={handlePageDrop}
    >
      <div className="flex items-center justify-between">
        <h1 className="text-2xl">{t("title")}</h1>
        <div className="flex items-center gap-2">
          <Button variant="outline" onClick={() => setNewFolderOpen(true)}>
            {t("createFolder")}
          </Button>
          <input
            ref={fileInputRef}
            type="file"
            className="hidden"
            onChange={handleFileSelected}
            disabled={uploading}
          />
          <Button disabled={uploading} onClick={() => fileInputRef.current?.click()}>
            {uploading ? t("uploading") : t("upload")}
          </Button>
        </div>
      </div>

      <Breadcrumb items={browse.breadcrumb} basePath="/files" onDropOnSegment={moveDraggedItemTo} />

      {dragOver && (
        <div className="flex items-center justify-center rounded-lg border-2 border-dashed border-primary/50 bg-primary/5 py-8 text-sm text-muted-foreground">
          {t("dropHereToUpload")}
        </div>
      )}

      {uploading && (
        <div className="flex items-center gap-3">
          <Progress value={uploadProgress} className="flex-1" />
          <span className="w-10 text-right text-sm text-muted-foreground tabular-nums">
            {uploadProgress}%
          </span>
          <Button size="sm" variant="ghost" onClick={handleCancelUpload}>
            {t("cancelUpload")}
          </Button>
        </div>
      )}

      {browse.subfolders.length === 0 && browse.files.length === 0 ? (
        <p className="text-muted-foreground">{folderId ? t("emptyFolder") : t("empty")}</p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("colName")}</TableHead>
              <TableHead>{t("colSize")}</TableHead>
              <TableHead>{t("colType")}</TableHead>
              <TableHead>{t("colCreated")}</TableHead>
              <TableHead className="text-right">{t("colActions")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {browse.subfolders.map((folder) => (
              <TableRow
                key={folder.id}
                draggable
                onDragStart={(e) => {
                  draggedItemRef.current = { type: "folder", id: folder.id };
                  e.dataTransfer.setData(DRAG_MIME, folder.id);
                }}
                onDragOver={(e) => e.preventDefault()}
                onDrop={(e) => handleDropOnFolderRow(e, folder)}
                className="cursor-pointer"
                onClick={() => navigateTo(folder.id)}
              >
                <TableCell className="max-w-xs truncate font-medium">📁 {folder.name}</TableCell>
                <TableCell>—</TableCell>
                <TableCell>—</TableCell>
                <TableCell>
                  {format.dateTime(new Date(folder.createdAt), { dateStyle: "short", timeStyle: "medium" })}
                </TableCell>
                <TableCell>
                  <div className="flex justify-end gap-2" onClick={(e) => e.stopPropagation()}>
                    <Button
                      size="icon-sm"
                      variant="ghost"
                      title={t("downloadZip")}
                      aria-label={t("downloadZip")}
                      nativeButton={false}
                      render={<a href={api.folderDownloadZipUrl(folder.id)} download />}
                    >
                      <Download />
                    </Button>
                    <CreateShareDialog target={{ type: "folder", id: folder.id }} />
                    <Button size="sm" variant="destructive" onClick={() => handleDeleteFolder(folder.id)}>
                      {t("delete")}
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            ))}
            {browse.files.map((f) => (
              <FileRow
                key={f.id}
                file={f}
                onOpenDetails={() => setDetailsFileId(f.id)}
                onDelete={() => handleDeleteFile(f.id)}
                onDragStart={() => {
                  draggedItemRef.current = { type: "file", id: f.id };
                }}
              />
            ))}
          </TableBody>
        </Table>
      )}

      <FileDetailsDialog
        fileId={detailsFileId}
        open={detailsFileId !== null}
        onOpenChange={(open) => {
          if (!open) setDetailsFileId(null);
        }}
      />

      <Dialog open={newFolderOpen} onOpenChange={setNewFolderOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("newFolderTitle")}</DialogTitle>
          </DialogHeader>
          <Input
            value={newFolderName}
            onChange={(e) => setNewFolderName(e.target.value)}
            placeholder={t("folderNamePlaceholder")}
            onKeyDown={(e) => {
              if (e.key === "Enter") handleCreateFolder();
            }}
          />
          <DialogFooter>
            <DialogClose render={<Button variant="ghost" />}>{t("cancel")}</DialogClose>
            <Button onClick={handleCreateFolder} disabled={creatingFolder || !newFolderName.trim()}>
              {t("create")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function FileRow({
  file,
  onOpenDetails,
  onDelete,
  onDragStart,
}: {
  file: FileMeta;
  onOpenDetails: () => void;
  onDelete: () => void;
  onDragStart: () => void;
}) {
  const t = useTranslations("Files");
  const format = useFormatter();

  return (
    <TableRow draggable onDragStart={onDragStart}>
      <TableCell className="max-w-xs cursor-pointer truncate" title={file.name} onClick={onOpenDetails}>
        {file.name || <span className="font-mono text-xs text-muted-foreground">{file.hash.slice(0, 12)}</span>}
      </TableCell>
      <TableCell>{formatSize(file.size)}</TableCell>
      <TableCell>{file.mime}</TableCell>
      <TableCell>
        {format.dateTime(new Date(file.createdAt), { dateStyle: "short", timeStyle: "medium" })}
      </TableCell>
      <TableCell>
        <div className="flex justify-end gap-2">
          <Button
            size="icon-sm"
            variant="ghost"
            title={t("download")}
            aria-label={t("download")}
            nativeButton={false}
            render={<a href={api.downloadUrl(file.id)} download={file.name || undefined} />}
          >
            <Download />
          </Button>
          <CreateShareDialog target={{ type: "file", id: file.id }} />
          <Button size="sm" variant="destructive" onClick={onDelete}>
            {t("delete")}
          </Button>
        </div>
      </TableCell>
    </TableRow>
  );
}
