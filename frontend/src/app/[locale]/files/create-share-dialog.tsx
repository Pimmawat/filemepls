"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { Link2 } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { api, ApiError, type ShareLink, type Visibility } from "@/lib/api";

type Target = { type: "file"; id: string } | { type: "folder"; id: string };

export function CreateShareDialog({ target }: { target: Target }) {
  const t = useTranslations("Share");
  const [open, setOpen] = useState(false);
  const [shares, setShares] = useState<ShareLink[]>([]);
  const [loadingShares, setLoadingShares] = useState(false);
  const [visibility, setVisibility] = useState<Visibility>("public");
  const [expiresAt, setExpiresAt] = useState("");
  const [maxDownloads, setMaxDownloads] = useState("");
  const [password, setPassword] = useState("");
  const [alias, setAlias] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [revokingId, setRevokingId] = useState<string | null>(null);
  const [copiedId, setCopiedId] = useState<string | null>(null);

  function resetForm() {
    setVisibility("public");
    setExpiresAt("");
    setMaxDownloads("");
    setPassword("");
    setAlias("");
  }

  async function loadShares() {
    setLoadingShares(true);
    try {
      const list =
        target.type === "file"
          ? await api.listShareLinks(target.id)
          : await api.listFolderShareLinks(target.id);
      setShares(list);
    } catch (err) {
      toast.error(t("loadFailed"), {
        description: err instanceof ApiError ? err.message : undefined,
      });
    } finally {
      setLoadingShares(false);
    }
  }

  async function handleCreate() {
    setSubmitting(true);
    try {
      const input = {
        visibility,
        expiresAt: expiresAt ? new Date(expiresAt).toISOString() : undefined,
        maxDownloads: maxDownloads ? Number(maxDownloads) : undefined,
        password: password || undefined,
        alias: alias.trim() || undefined,
      };
      const created =
        target.type === "file"
          ? await api.createShareLink(target.id, input)
          : await api.createFolderShareLink(target.id, input);
      // The backend is idempotent: an identical request (or an existing alias
      // for this item) returns the current link instead of a duplicate, so
      // de-dupe by id before prepending.
      setShares((prev) => [created, ...prev.filter((s) => s.id !== created.id)]);
      resetForm();
    } catch (err) {
      if (err instanceof ApiError && alias.trim()) {
        if (err.status === 409) {
          toast.error(t("aliasTaken"));
          return;
        }
        if (err.status === 400) {
          toast.error(t("aliasInvalid"));
          return;
        }
      }
      toast.error(t("createFailed"), {
        description: err instanceof ApiError ? err.message : undefined,
      });
    } finally {
      setSubmitting(false);
    }
  }

  function shareUrl(token: string) {
    return `${window.location.origin}/share/${token}`;
  }

  async function handleCopy(share: ShareLink) {
    try {
      await navigator.clipboard.writeText(shareUrl(share.token));
      setCopiedId(share.id);
      setTimeout(() => setCopiedId((id) => (id === share.id ? null : id)), 2000);
    } catch {
      toast.error(t("copyFailed"));
    }
  }

  async function handleRevoke(share: ShareLink) {
    setRevokingId(share.id);
    try {
      await api.revokeShareLink(share.id);
      setShares((prev) => prev.filter((s) => s.id !== share.id));
      toast.success(t("revoked"));
    } catch (err) {
      toast.error(t("revokeFailed"), {
        description: err instanceof ApiError ? err.message : undefined,
      });
    } finally {
      setRevokingId(null);
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        setOpen(next);
        if (next) {
          loadShares();
        } else {
          setShares([]);
          resetForm();
        }
      }}
    >
      <DialogTrigger
        render={<Button variant="ghost" size="icon-sm" title={t("title")} aria-label={t("title")} />}
      >
        <Link2 />
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("title")}</DialogTitle>
        </DialogHeader>

        <div className="flex flex-col gap-4">
          {loadingShares ? (
            <p className="text-sm text-muted-foreground">{t("loadingLinks")}</p>
          ) : shares.length > 0 ? (
            <div className="flex flex-col gap-2">
              {shares.map((s) => (
                <div key={s.id} className="flex items-center gap-2">
                  <Input readOnly value={shareUrl(s.token)} className="flex-1" />
                  <Button size="sm" onClick={() => handleCopy(s)}>
                    {copiedId === s.id ? t("copied") : t("copy")}
                  </Button>
                  <Button
                    size="sm"
                    variant="destructive"
                    onClick={() => handleRevoke(s)}
                    disabled={revokingId === s.id}
                  >
                    {t("revoke")}
                  </Button>
                </div>
              ))}
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">{t("noLinks")}</p>
          )}

          <Separator />

          <div className="flex flex-col gap-4">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="visibility">{t("visibility")}</Label>
              <select
                id="visibility"
                className="h-8 rounded-lg border border-border bg-background px-2 text-sm"
                value={visibility}
                onChange={(e) => setVisibility(e.target.value as Visibility)}
              >
                <option value="public">{t("visibilityPublic")}</option>
                <option value="unlisted">{t("visibilityUnlisted")}</option>
                <option value="private">{t("visibilityPrivate")}</option>
              </select>
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="alias">{t("aliasLabel")}</Label>
              <div className="flex items-center gap-1.5">
                <span className="shrink-0 text-sm text-muted-foreground">/share/</span>
                <Input
                  id="alias"
                  value={alias}
                  onChange={(e) => setAlias(e.target.value)}
                  placeholder={t("aliasPlaceholder")}
                  className="flex-1"
                />
              </div>
              <p className="text-xs text-muted-foreground">{t("aliasHint")}</p>
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="expiresAt">{t("expiresAt")}</Label>
              <Input
                id="expiresAt"
                type="datetime-local"
                value={expiresAt}
                onChange={(e) => setExpiresAt(e.target.value)}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="maxDownloads">{t("maxDownloads")}</Label>
              <Input
                id="maxDownloads"
                type="number"
                min={1}
                value={maxDownloads}
                onChange={(e) => setMaxDownloads(e.target.value)}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="password">{t("password")}</Label>
              <Input
                id="password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            </div>
          </div>
        </div>

        <DialogFooter>
          <DialogClose render={<Button variant="ghost" />}>{t("cancel")}</DialogClose>
          <Button onClick={handleCreate} disabled={submitting}>
            {t("create")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
