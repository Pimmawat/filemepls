"use client";

import { useState } from "react";
import { useFormatter, useTranslations } from "next-intl";
import { Folder } from "lucide-react";
import { toast } from "sonner";

import { Breadcrumb } from "@/components/breadcrumb";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { api, ApiError, type BrowseResult, type FileMeta, type FolderMeta } from "@/lib/api";

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

// Folder shares are browsed without changing the URL: the backend has no
// session for anonymous visitors and re-checks the password on every
// single request, so a real per-folder route would mean re-typing the
// password on every navigation. Keeping the current folder + password in
// React state instead means it's only ever entered once per visit.
export function ShareFolderBrowser({
  token,
  initialBrowse,
  requiresPassword,
}: {
  token: string;
  initialBrowse: BrowseResult | null;
  requiresPassword: boolean;
}) {
  const t = useTranslations("SharePage");
  const tFiles = useTranslations("Files");
  const format = useFormatter();

  const [password, setPassword] = useState("");
  const [unlocked, setUnlocked] = useState(!requiresPassword);
  const [verifying, setVerifying] = useState(false);
  const [browse, setBrowse] = useState<BrowseResult | null>(initialBrowse);
  const [loading, setLoading] = useState(false);

  async function handleUnlock(e: React.FormEvent) {
    e.preventDefault();
    setVerifying(true);
    try {
      const result = await api.browsePublicFolderShare(token, undefined, password);
      setBrowse(result);
      setUnlocked(true);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        toast.error(t("wrongPassword"));
      } else {
        toast.error(t("wrongPassword"), {
          description: err instanceof ApiError ? err.message : undefined,
        });
      }
    } finally {
      setVerifying(false);
    }
  }

  async function navigate(folderId: string | null) {
    setLoading(true);
    try {
      const result = await api.browsePublicFolderShare(token, folderId ?? undefined, password);
      setBrowse(result);
    } catch (err) {
      toast.error(t("wrongPassword"), {
        description: err instanceof ApiError ? err.message : undefined,
      });
    } finally {
      setLoading(false);
    }
  }

  function downloadZip(folder: FolderMeta | null) {
    const form = document.createElement("form");
    form.method = "POST";
    form.action = api.shareZipUrl(token);
    form.style.display = "none";
    if (folder) {
      const folderInput = document.createElement("input");
      folderInput.name = "folderId";
      folderInput.value = folder.id;
      form.appendChild(folderInput);
    }
    if (password) {
      const passwordInput = document.createElement("input");
      passwordInput.name = "password";
      passwordInput.value = password;
      form.appendChild(passwordInput);
    }
    document.body.appendChild(form);
    form.submit();
    form.remove();
  }

  function downloadFile(file: FileMeta) {
    const form = document.createElement("form");
    form.method = "POST";
    form.action = api.shareFolderFileDownloadUrl(token, file.id);
    form.style.display = "none";
    if (password) {
      const passwordInput = document.createElement("input");
      passwordInput.name = "password";
      passwordInput.value = password;
      form.appendChild(passwordInput);
    }
    document.body.appendChild(form);
    form.submit();
    form.remove();
  }

  if (!unlocked || !browse) {
    return (
      <>
        <h1 className="text-xl">{t("needsPasswordTitle")}</h1>
        <form onSubmit={handleUnlock} className="flex w-full max-w-xs flex-col gap-3">
          <div className="flex flex-col gap-1.5 text-left">
            <Label htmlFor="password">{t("passwordLabel")}</Label>
            <Input
              id="password"
              type="password"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </div>
          <Button type="submit" disabled={verifying}>
            {verifying ? t("verifying") : t("enterPassword")}
          </Button>
        </form>
      </>
    );
  }

  return (
    <div className="flex w-full max-w-3xl flex-col gap-4 text-left">
      <div className="flex items-center justify-between">
        <Breadcrumb items={browse.breadcrumb} onNavigate={navigate} />
        <Button size="sm" variant="outline" onClick={() => downloadZip(browse.folder)}>
          {t("downloadZipButton")}
        </Button>
      </div>

      {browse.subfolders.length === 0 && browse.files.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t("emptySharedFolder")}</p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{tFiles("colName")}</TableHead>
              <TableHead>{tFiles("colSize")}</TableHead>
              <TableHead>{tFiles("colCreated")}</TableHead>
              <TableHead className="text-right">{tFiles("colActions")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {browse.subfolders.map((folder) => (
              <TableRow key={folder.id}>
                <TableCell className="max-w-xs truncate font-medium"><Folder className="inline-block size-4 mr-1.5 -mt-0.5 text-muted-foreground" /> {folder.name}</TableCell>
                <TableCell>—</TableCell>
                <TableCell>
                  {format.dateTime(new Date(folder.createdAt), { dateStyle: "short", timeStyle: "medium" })}
                </TableCell>
                <TableCell className="text-right">
                  <Button size="sm" variant="ghost" disabled={loading} onClick={() => navigate(folder.id)}>
                    {tFiles("open")}
                  </Button>
                </TableCell>
              </TableRow>
            ))}
            {browse.files.map((file) => (
              <TableRow key={file.id}>
                <TableCell className="max-w-xs truncate" title={file.name}>
                  {file.name}
                </TableCell>
                <TableCell>{formatSize(file.size)}</TableCell>
                <TableCell>
                  {format.dateTime(new Date(file.createdAt), { dateStyle: "short", timeStyle: "medium" })}
                </TableCell>
                <TableCell className="text-right">
                  <Button size="sm" variant="ghost" onClick={() => downloadFile(file)}>
                    {t("downloadButton")}
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  );
}
