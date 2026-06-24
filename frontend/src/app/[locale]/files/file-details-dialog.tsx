"use client";

import { useState } from "react";
import { useFormatter, useTranslations } from "next-intl";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { api, ApiError, type FileMeta } from "@/lib/api";

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

// FileDetailsDialog fetches the file's metadata fresh when opened — this
// doubles as the fix for the "download errors show a blank JSON page" bug:
// since we already know the file is accessible by the time this succeeds,
// a Download control is only ever shown once that's confirmed. If the
// fetch fails (deleted/forbidden), we show a toast and never render a
// download control at all, instead of letting the user hit a broken link.
export function FileDetailsDialog({
  fileId,
  open,
  onOpenChange,
}: {
  fileId: string | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const t = useTranslations("FileDetails");
  const format = useFormatter();
  const [file, setFile] = useState<FileMeta | null>(null);
  const [loading, setLoading] = useState(false);
  const [failed, setFailed] = useState(false);

  async function load(id: string) {
    setLoading(true);
    setFailed(false);
    setFile(null);
    try {
      setFile(await api.getFile(id));
    } catch (err) {
      setFailed(true);
      toast.error(t("loadFailed"), {
        description: err instanceof ApiError ? err.message : undefined,
      });
    } finally {
      setLoading(false);
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        onOpenChange(next);
        if (next && fileId) load(fileId);
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("title")}</DialogTitle>
        </DialogHeader>

        {loading ? (
          <p className="text-sm text-muted-foreground">…</p>
        ) : failed || !file ? (
          <p className="text-sm text-muted-foreground">{t("loadFailed")}</p>
        ) : (
          <dl className="flex flex-col gap-2 text-sm">
            <div className="flex justify-between gap-4">
              <dt className="text-muted-foreground">{t("name")}</dt>
              <dd className="truncate text-right">{file.name}</dd>
            </div>
            <div className="flex justify-between gap-4">
              <dt className="text-muted-foreground">{t("size")}</dt>
              <dd>{formatSize(file.size)}</dd>
            </div>
            <div className="flex justify-between gap-4">
              <dt className="text-muted-foreground">{t("type")}</dt>
              <dd>{file.mime}</dd>
            </div>
            <div className="flex justify-between gap-4">
              <dt className="text-muted-foreground">{t("created")}</dt>
              <dd>{format.dateTime(new Date(file.createdAt), { dateStyle: "short", timeStyle: "medium" })}</dd>
            </div>
          </dl>
        )}

        <DialogFooter>
          <DialogClose render={<Button variant="ghost" />}>{t("close")}</DialogClose>
          {file && (
            <Button nativeButton={false} render={<a href={api.downloadUrl(file.id)} />}>
              {t("download")}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
