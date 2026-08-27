"use client";

import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { type FileMeta } from "@/lib/api";

// ponytail: a share download is POST-only (the password must never sit in a
// URL), so an <img src> can't stream it — the bytes get buffered into a blob
// instead. Hence the cap: anything bigger stays download-only.
const PREVIEW_MAX_BYTES = 100 * 1024 * 1024;

export function isPreviewable(file: FileMeta): boolean {
  return (
    (file.mime.startsWith("image/") || file.mime.startsWith("video/")) &&
    file.size <= PREVIEW_MAX_BYTES
  );
}

// The blob URL is fetched by the caller before opening (see the "preview"
// state in ShareFileDownload / ShareFolderBrowser): Base UI only fires
// onOpenChange for changes it makes itself, so a dialog opened from the
// outside would never get a "now load it" signal.
//
// Downloading from here re-uses that same blob rather than POSTing again,
// so previewing and then saving costs the share link one redemption, not
// two — it matters for links with a download limit.
export function SharePreviewDialog({
  preview,
  onOpenChange,
}: {
  preview: { file: FileMeta; url: string } | null;
  onOpenChange: (open: boolean) => void;
}) {
  const t = useTranslations("SharePage");

  return (
    <Dialog open={preview !== null} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle className="truncate text-left">{preview?.file.name}</DialogTitle>
        </DialogHeader>

        {preview &&
          (preview.file.mime.startsWith("image/") ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img
              src={preview.url}
              alt={preview.file.name}
              className="max-h-[60vh] w-full rounded-md bg-muted object-contain"
            />
          ) : (
            <video src={preview.url} controls className="max-h-[60vh] w-full rounded-md bg-muted" />
          ))}

        <DialogFooter>
          <DialogClose render={<Button variant="ghost" />}>{t("close")}</DialogClose>
          {preview && (
            <Button
              nativeButton={false}
              render={<a href={preview.url} download={preview.file.name || undefined} />}
            >
              {t("downloadButton")}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
