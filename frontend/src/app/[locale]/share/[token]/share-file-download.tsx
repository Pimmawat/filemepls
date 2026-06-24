"use client";

import { useRef, useState } from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { api, ApiError, type FileMeta } from "@/lib/api";

// The actual download is a real <form method="POST"> (native streaming, no
// JS buffering of the file). The risk that motivated this component: if
// the password is wrong, that raw POST navigation lands the browser on a
// bare JSON error response. So when a password is required, we first
// verify it via a side-effect-free fetch — only once that succeeds do we
// reveal/submit the real download form; a wrong password now surfaces as
// a normal toast instead of a blank JSON page.
export function ShareFileDownload({
  token,
  file,
  requiresPassword,
}: {
  token: string;
  // null when a password is required: the server withholds file metadata
  // until the password is verified, so the name is only known afterward.
  file: FileMeta | null;
  requiresPassword: boolean;
}) {
  const t = useTranslations("SharePage");
  const [password, setPassword] = useState("");
  const [verifying, setVerifying] = useState(false);
  const [verified, setVerified] = useState(!requiresPassword);
  const formRef = useRef<HTMLFormElement>(null);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (verified) {
      formRef.current?.submit();
      return;
    }
    setVerifying(true);
    try {
      await api.verifySharePassword(token, password);
      setVerified(true);
      // Submit on the next tick so the now-verified form (with the
      // password field) has re-rendered before .submit() is called.
      setTimeout(() => formRef.current?.submit(), 0);
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

  return (
    <>
      <h1 className="text-xl">{requiresPassword ? t("needsPasswordTitle") : t("readyTitle")}</h1>
      {file && <p className="text-sm text-muted-foreground">{file.name}</p>}
      <form
        ref={formRef}
        method="POST"
        action={api.shareDownloadUrl(token)}
        className="flex w-full max-w-xs flex-col gap-3"
        onSubmit={handleSubmit}
      >
        {requiresPassword && (
          <div className="flex flex-col gap-1.5 text-left">
            <Label htmlFor="password">{t("passwordLabel")}</Label>
            <Input
              id="password"
              name="password"
              type="password"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </div>
        )}
        <Button type="submit" disabled={verifying}>
          {verifying ? t("verifying") : t("downloadButton")}
        </Button>
      </form>
    </>
  );
}
