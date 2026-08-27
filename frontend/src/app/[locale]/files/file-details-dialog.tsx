"use client";

import { useFormatter, useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { api, type FileMeta } from "@/lib/api";

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

// The caller fetches the metadata before opening this (see
// FileManager.openDetails) — which doubles as the fix for the "download
// errors show a blank JSON page" bug: the dialog, and the Download control
// in it, only ever appear once the file is confirmed accessible. A deleted
// or forbidden file toasts instead of opening.
export function FileDetailsDialog({
  file,
  open,
  onOpenChange,
}: {
  file: FileMeta | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const t = useTranslations("FileDetails");
  const format = useFormatter();

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{t("title")}</DialogTitle>
        </DialogHeader>

        {!file ? (
          <p className="text-sm text-muted-foreground">{t("loadFailed")}</p>
        ) : (
          <>
            {file.mime.startsWith("image/") && (
              // ponytail: the download URL streams the bytes with the right
              // Content-Type; Content-Disposition: attachment only applies to
              // navigations, not to <img>/<video> subresource loads.
              // eslint-disable-next-line @next/next/no-img-element
              <img
                src={api.downloadUrl(file.id)}
                alt={file.name}
                className="max-h-[60vh] w-full rounded-md bg-muted object-contain"
              />
            )}
            {file.mime.startsWith("video/") && (
              <video
                src={api.downloadUrl(file.id)}
                controls
                preload="metadata"
                className="max-h-[60vh] w-full rounded-md bg-muted"
              />
            )}
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
          </>
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
