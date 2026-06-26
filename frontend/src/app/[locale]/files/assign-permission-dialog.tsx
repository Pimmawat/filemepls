"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { UserPlus } from "lucide-react";
import { toast } from "sonner";

import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
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
import { Separator } from "@/components/ui/separator";
import { api, ApiError, type AccessGrant, type UserSummary } from "@/lib/api";

type Target = { type: "file"; id: string } | { type: "folder"; id: string };

function initials(name: string) {
  return name
    .trim()
    .split(/\s+/)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase())
    .join("");
}

export function AssignPermissionDialog({ target }: { target: Target }) {
  const t = useTranslations("Permissions");
  const [open, setOpen] = useState(false);
  const [grants, setGrants] = useState<AccessGrant[]>([]);
  const [loadingGrants, setLoadingGrants] = useState(false);
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<UserSummary[]>([]);
  const [searching, setSearching] = useState(false);
  const [grantingID, setGrantingID] = useState<string | null>(null);
  const [revokingID, setRevokingID] = useState<string | null>(null);

  async function loadGrants() {
    setLoadingGrants(true);
    try {
      const list =
        target.type === "file"
          ? await api.listFileGrants(target.id)
          : await api.listFolderGrants(target.id);
      setGrants(list);
    } catch (err) {
      toast.error(t("loadFailed"), {
        description: err instanceof ApiError ? err.message : undefined,
      });
    } finally {
      setLoadingGrants(false);
    }
  }

  useEffect(() => {
    if (!open || !query.trim()) {
      return;
    }
    const handle = setTimeout(async () => {
      setSearching(true);
      try {
        setResults(await api.searchUsers(query.trim()));
      } catch {
        setResults([]);
      } finally {
        setSearching(false);
      }
    }, 300);
    return () => clearTimeout(handle);
  }, [open, query]);

  const visibleResults = query.trim() ? results : [];

  async function handleGrant(user: UserSummary) {
    setGrantingID(user.id);
    try {
      const created =
        target.type === "file"
          ? await api.grantFileAccess(target.id, user.email)
          : await api.grantFolderAccess(target.id, user.email);
      setGrants((prev) => [created, ...prev]);
      setQuery("");
      setResults([]);
      toast.success(t("granted"));
    } catch (err) {
      toast.error(t("grantFailed"), {
        description: err instanceof ApiError ? err.message : undefined,
      });
    } finally {
      setGrantingID(null);
    }
  }

  async function handleRevoke(grant: AccessGrant) {
    setRevokingID(grant.id);
    try {
      await api.revokeGrant(grant.id);
      setGrants((prev) => prev.filter((g) => g.id !== grant.id));
      toast.success(t("revoked"));
    } catch (err) {
      toast.error(t("revokeFailed"), {
        description: err instanceof ApiError ? err.message : undefined,
      });
    } finally {
      setRevokingID(null);
    }
  }

  const grantedIDs = new Set(grants.map((g) => g.granteeId));

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        setOpen(next);
        if (next) {
          loadGrants();
        } else {
          setGrants([]);
          setQuery("");
          setResults([]);
        }
      }}
    >
      <DialogTrigger
        render={<Button variant="ghost" size="icon-sm" title={t("title")} aria-label={t("title")} />}
      >
        <UserPlus />
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("title")}</DialogTitle>
        </DialogHeader>

        <div className="flex flex-col gap-4">
          <Input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={t("searchPlaceholder")}
          />

          {searching && <p className="text-sm text-muted-foreground">{t("searching")}</p>}

          {visibleResults.length > 0 && (
            <div className="flex flex-col gap-1">
              {visibleResults.map((u) => {
                const alreadyGranted = grantedIDs.has(u.id);
                return (
                  <button
                    key={u.id}
                    type="button"
                    disabled={alreadyGranted || grantingID === u.id}
                    onClick={() => handleGrant(u)}
                    className="flex items-center gap-2 rounded-md p-1.5 text-left text-sm hover:bg-accent disabled:opacity-50"
                  >
                    <Avatar className="size-6">
                      <AvatarImage src={u.avatarUrl} alt={u.displayName} />
                      <AvatarFallback>{initials(u.displayName)}</AvatarFallback>
                    </Avatar>
                    <span className="flex-1 truncate">
                      {u.displayName}{" "}
                      <span className="text-muted-foreground">({u.email})</span>
                    </span>
                    {alreadyGranted && (
                      <span className="text-xs text-muted-foreground">{t("alreadyShared")}</span>
                    )}
                  </button>
                );
              })}
            </div>
          )}

          <Separator />

          {loadingGrants ? (
            <p className="text-sm text-muted-foreground">{t("loadingPeople")}</p>
          ) : grants.length > 0 ? (
            <div className="flex flex-col gap-2">
              {grants.map((g) => (
                <div key={g.id} className="flex items-center gap-2">
                  <Avatar className="size-6">
                    <AvatarImage src={g.granteeAvatarUrl} alt={g.granteeName} />
                    <AvatarFallback>{initials(g.granteeName)}</AvatarFallback>
                  </Avatar>
                  <span className="flex-1 truncate text-sm">
                    {g.granteeName}{" "}
                    <span className="text-muted-foreground">({g.granteeEmail})</span>
                  </span>
                  <Button
                    size="sm"
                    variant="destructive"
                    onClick={() => handleRevoke(g)}
                    disabled={revokingID === g.id}
                  >
                    {t("revoke")}
                  </Button>
                </div>
              ))}
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">{t("noPeople")}</p>
          )}
        </div>

        <DialogFooter>
          <DialogClose render={<Button variant="ghost" />}>{t("close")}</DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
