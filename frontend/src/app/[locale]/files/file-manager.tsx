"use client";

import { useCallback, useRef, useState } from "react";
import { useFormatter, useTranslations } from "next-intl";
import { Download, Folder, FolderPlus, Trash2, Upload, X } from "lucide-react";
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
import { AssignPermissionDialog } from "./assign-permission-dialog";
import { CreateShareDialog } from "./create-share-dialog";
import { FileDetailsDialog } from "./file-details-dialog";

const DRAG_MIME = "application/x-filemepls-item";
const UPLOAD_TOAST_ID = "upload-progress";

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

function formatSpeed(bytesPerSec: number): string {
  if (bytesPerSec < 1024) return `${Math.round(bytesPerSec)} B/s`;
  const units = ["KB/s", "MB/s", "GB/s"];
  let value = bytesPerSec / 1024;
  let i = 0;
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024;
    i++;
  }
  return `${value.toFixed(1)} ${units[i]}`;
}

type UploadItem = { file: File; parentId: string | null };

// A directory's entries can come back across multiple readEntries() calls
// (the spec caps each batch, ~100 in Chrome) — keep calling until it
// returns an empty array.
function readAllEntries(reader: FileSystemDirectoryReader): Promise<FileSystemEntry[]> {
  return new Promise((resolve, reject) => {
    const all: FileSystemEntry[] = [];
    const readBatch = () => {
      reader.readEntries((batch) => {
        if (batch.length === 0) {
          resolve(all);
        } else {
          all.push(...batch);
          readBatch();
        }
      }, reject);
    };
    readBatch();
  });
}

function readFileEntry(entry: FileSystemFileEntry): Promise<File> {
  return new Promise((resolve, reject) => entry.file(resolve, reject));
}

export function FileManager({
  initialBrowse,
  folderId,
  viewerId,
}: {
  initialBrowse: BrowseResult;
  folderId: string | null;
  viewerId: string;
}) {
  const t = useTranslations("Files");
  const format = useFormatter();
  const router = useRouter();

  const [browse, setBrowse] = useState(initialBrowse);
  // Browsing a folder shared by someone else (view-only): root is always
  // the viewer's own, so it's only ever false once inside such a folder.
  const isOwner = !browse.folder || browse.folder.ownerId === viewerId;
  const [uploading, setUploading] = useState(false);
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
  const lastProgressRef = useRef<{ time: number; loaded: number }>({ time: 0, loaded: 0 });

  const renderUploadToast = useCallback(
    (fileName: string, progress: number, speed: number, current: number, total: number) => {
      toast.custom(
        (toastId) => (
          <div className="flex w-[356px] flex-col gap-2 rounded-lg border bg-popover p-4 text-popover-foreground shadow-lg">
            <div className="flex items-center justify-between">
              <span className="truncate text-sm font-medium">
                {total > 1 ? `(${current}/${total}) ${fileName}` : fileName}
              </span>
              <button
                type="button"
                className="ml-2 shrink-0 rounded-sm p-0.5 text-muted-foreground hover:text-foreground"
                onClick={() => {
                  abortControllerRef.current?.abort();
                  toast.dismiss(toastId);
                }}
              >
                <X className="size-4" />
              </button>
            </div>
            <Progress value={progress} />
            <div className="flex items-center justify-between text-xs text-muted-foreground tabular-nums">
              <span>{progress}%</span>
              <span>{formatSpeed(speed)}</span>
            </div>
          </div>
        ),
        { id: UPLOAD_TOAST_ID, duration: Infinity },
      );
    },
    [],
  );

  // Recursively walks dropped directory entries, recreating the folder
  // structure server-side as it goes (so a nested file's parentId points at
  // the right newly created folder) and flattens everything into a single
  // upload queue.
  async function collectEntries(
    entries: FileSystemEntry[],
    parentId: string | null,
  ): Promise<UploadItem[]> {
    const items: UploadItem[] = [];
    for (const entry of entries) {
      if (entry.isFile) {
        const file = await readFileEntry(entry as FileSystemFileEntry);
        items.push({ file, parentId });
      } else if (entry.isDirectory) {
        let created;
        try {
          created = await api.createFolder(entry.name, parentId);
        } catch (err) {
          toast.error(t("folderCreateFailed"), {
            description: err instanceof ApiError ? err.message : undefined,
          });
          continue;
        }
        if (parentId === folderId) {
          setBrowse((prev) => ({ ...prev, subfolders: [created, ...prev.subfolders] }));
        }
        const children = await readAllEntries((entry as FileSystemDirectoryEntry).createReader());
        items.push(...(await collectEntries(children, created.id)));
      }
    }
    return items;
  }

  // dataTransfer.items must be read synchronously (before any await) since
  // some browsers invalidate it once the drop event handler returns; the
  // FileSystemEntry objects it yields stay valid afterwards.
  async function collectDroppedItems(
    dataTransfer: DataTransfer,
    parentId: string | null,
  ): Promise<UploadItem[]> {
    const entries = Array.from(dataTransfer.items)
      .map((item) => item.webkitGetAsEntry?.())
      .filter((entry): entry is FileSystemEntry => entry !== null);
    if (entries.length === 0) {
      // Fallback for browsers without the (de facto standard) entries API.
      return Array.from(dataTransfer.files).map((file) => ({ file, parentId }));
    }
    return collectEntries(entries, parentId);
  }

  async function uploadItemsInto(items: UploadItem[]) {
    if (items.length === 0) return;
    const controller = new AbortController();
    abortControllerRef.current = controller;
    setUploading(true);
    const matchingCurrentFolder: FileMeta[] = [];
    const failed: string[] = [];
    let successCount = 0;
    let cancelled = false;

    // One progress bar for the whole batch instead of one per file: track
    // bytes already finished plus the bytes loaded for whichever file is
    // currently in flight, so the bar advances smoothly across files
    // instead of resetting to 0% each time the next one starts.
    const totalBytes = items.reduce((sum, item) => sum + item.file.size, 0);
    let completedBytes = 0;
    let currentSpeed = 0;
    lastProgressRef.current = { time: Date.now(), loaded: 0 };

    for (let i = 0; i < items.length; i++) {
      const { file, parentId } = items[i];
      renderUploadToast(
        file.name,
        totalBytes > 0 ? Math.round((completedBytes / totalBytes) * 100) : 0,
        currentSpeed,
        i + 1,
        items.length,
      );
      try {
        const created = await api.uploadFile(
          file,
          parentId,
          (loaded) => {
            const overallLoaded = completedBytes + loaded;
            const pct = totalBytes > 0 ? Math.round((overallLoaded / totalBytes) * 100) : 0;
            const now = Date.now();
            const prev = lastProgressRef.current;
            const elapsed = (now - prev.time) / 1000;
            if (elapsed >= 0.3) {
              currentSpeed = (overallLoaded - prev.loaded) / elapsed;
              lastProgressRef.current = { time: now, loaded: overallLoaded };
            }
            renderUploadToast(file.name, pct, currentSpeed, i + 1, items.length);
          },
          controller.signal,
        );
        successCount++;
        completedBytes += file.size;
        if (parentId === folderId) matchingCurrentFolder.push(created);
      } catch (err) {
        if (err instanceof UploadCancelledError) {
          cancelled = true;
          break;
        }
        failed.push(file.name);
        completedBytes += file.size;
      }
    }

    toast.dismiss(UPLOAD_TOAST_ID);
    if (matchingCurrentFolder.length > 0) {
      setBrowse((prev) => ({ ...prev, files: [...matchingCurrentFolder, ...prev.files] }));
    }
    if (cancelled) {
      toast(t("uploadCancelled"));
    } else {
      if (successCount > 0) {
        toast.success(
          successCount === 1
            ? t("uploadComplete")
            : t("uploadCompleteMultiple", { count: successCount }),
        );
      }
      if (failed.length > 0) {
        toast.error(t("uploadFailed"), { description: failed.join(", ") });
      }
    }

    setUploading(false);
    abortControllerRef.current = null;
  }

  async function handleFileSelected(e: React.ChangeEvent<HTMLInputElement>) {
    const files = Array.from(e.target.files ?? []);
    if (files.length === 0) return;
    await uploadItemsInto(files.map((file) => ({ file, parentId: folderId })));
    e.target.value = "";
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

  async function handleDropOnFolderRow(e: React.DragEvent, folder: FolderMeta) {
    e.preventDefault();
    e.stopPropagation();
    if (e.dataTransfer.types.includes("Files")) {
      const items = await collectDroppedItems(e.dataTransfer, folder.id);
      if (items.length > 0) uploadItemsInto(items);
      return;
    }
    moveDraggedItemTo(folder.id);
  }

  async function handlePageDrop(e: React.DragEvent) {
    e.preventDefault();
    setDragOver(false);
    if (e.dataTransfer.types.includes("Files")) {
      const items = await collectDroppedItems(e.dataTransfer, folderId);
      if (items.length > 0) uploadItemsInto(items);
    }
  }

  function navigateTo(id: string) {
    router.push(id ? `/files/${id}` : "/files");
  }

  return (
    <div
      className="flex flex-col gap-6"
      onDragOver={(e) => {
        if (isOwner && e.dataTransfer.types.includes("Files")) {
          e.preventDefault();
          setDragOver(true);
        }
      }}
      onDragLeave={() => setDragOver(false)}
      onDrop={isOwner ? handlePageDrop : undefined}
    >
      <div className="flex items-center justify-between">
        <h1 className="text-2xl">{t("title")}</h1>
        {isOwner && (
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="icon"
              title={t("createFolder")}
              aria-label={t("createFolder")}
              onClick={() => setNewFolderOpen(true)}
            >
              <FolderPlus />
            </Button>
            <input
              ref={fileInputRef}
              type="file"
              multiple
              className="hidden"
              onChange={handleFileSelected}
              disabled={uploading}
            />
            <Button
              size="icon"
              disabled={uploading}
              title={uploading ? t("uploading") : t("upload")}
              aria-label={uploading ? t("uploading") : t("upload")}
              onClick={() => fileInputRef.current?.click()}
            >
              <Upload />
            </Button>
          </div>
        )}
      </div>

      <Breadcrumb
        items={browse.breadcrumb}
        basePath="/files"
        onDropOnSegment={isOwner ? moveDraggedItemTo : undefined}
      />

      {dragOver && (
        <div className="flex items-center justify-center rounded-lg border-2 border-dashed border-primary/50 bg-primary/5 py-8 text-sm text-muted-foreground">
          {t("dropHereToUpload")}
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
                draggable={isOwner}
                onDragStart={
                  isOwner
                    ? (e) => {
                        draggedItemRef.current = { type: "folder", id: folder.id };
                        e.dataTransfer.setData(DRAG_MIME, folder.id);
                      }
                    : undefined
                }
                onDragOver={isOwner ? (e) => e.preventDefault() : undefined}
                onDrop={isOwner ? (e) => handleDropOnFolderRow(e, folder) : undefined}
                className="cursor-pointer"
                onClick={() => navigateTo(folder.id)}
              >
                <TableCell className="max-w-xs truncate font-medium"><Folder className="inline-block size-4 mr-1.5 -mt-0.5 text-muted-foreground" /> {folder.name}</TableCell>
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
                    {isOwner && (
                      <>
                        <CreateShareDialog target={{ type: "folder", id: folder.id }} />
                        <AssignPermissionDialog target={{ type: "folder", id: folder.id }} />
                        <Button
                          size="icon-sm"
                          variant="destructive"
                          title={t("delete")}
                          aria-label={t("delete")}
                          onClick={() => handleDeleteFolder(folder.id)}
                        >
                          <Trash2 />
                        </Button>
                      </>
                    )}
                  </div>
                </TableCell>
              </TableRow>
            ))}
            {browse.files.map((f) => (
              <FileRow
                key={f.id}
                file={f}
                isOwner={isOwner}
                onOpenDetails={() => setDetailsFileId(f.id)}
                onDelete={() => handleDeleteFile(f.id)}
                onDragStart={
                  isOwner
                    ? () => {
                        draggedItemRef.current = { type: "file", id: f.id };
                      }
                    : undefined
                }
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
  isOwner,
  onOpenDetails,
  onDelete,
  onDragStart,
}: {
  file: FileMeta;
  isOwner: boolean;
  onOpenDetails: () => void;
  onDelete: () => void;
  onDragStart?: () => void;
}) {
  const t = useTranslations("Files");
  const format = useFormatter();

  return (
    <TableRow draggable={isOwner} onDragStart={onDragStart}>
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
          {isOwner && (
            <>
              <CreateShareDialog target={{ type: "file", id: file.id }} />
              <AssignPermissionDialog target={{ type: "file", id: file.id }} />
              <Button
                size="icon-sm"
                variant="destructive"
                title={t("delete")}
                aria-label={t("delete")}
                onClick={onDelete}
              >
                <Trash2 />
              </Button>
            </>
          )}
        </div>
      </TableCell>
    </TableRow>
  );
}
