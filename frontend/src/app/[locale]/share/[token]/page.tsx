import { cookies } from "next/headers";
import { getTranslations } from "next-intl/server";

import { api, type PublicShareState } from "@/lib/api";
import { ShareFileDownload } from "./share-file-download";
import { ShareFolderBrowser } from "./share-folder-browser";

type Props = {
  params: Promise<{ locale: string; token: string }>;
};

export default async function SharePage({ params }: Props) {
  const { token } = await params;
  const t = await getTranslations("SharePage");

  const store = await cookies();
  const sessionCookie = store.get("filemepls_session")?.value;
  const cookieHeader = sessionCookie ? `filemepls_session=${sessionCookie}` : undefined;

  let state: PublicShareState;
  try {
    state = await api.getPublicShare(token, cookieHeader);
  } catch {
    state = { status: "not_found" };
  }

  return (
    <main className="flex flex-1 flex-col items-center justify-center gap-4 px-4 text-center">
      {state.status === "expired" && <h1 className="text-xl">{t("expiredTitle")}</h1>}
      {state.status === "limit_reached" && <h1 className="text-xl">{t("limitReachedTitle")}</h1>}
      {state.status === "auth_required" && <h1 className="text-xl">{t("authRequiredTitle")}</h1>}
      {state.status === "not_found" && <h1 className="text-xl">{t("notFoundTitle")}</h1>}

      {(state.status === "ok" || state.status === "needs_password") && state.targetType === "file" && (
        <ShareFileDownload
          token={token}
          file={state.file ?? null}
          requiresPassword={state.status === "needs_password"}
        />
      )}

      {(state.status === "ok" || state.status === "needs_password") && state.targetType === "folder" && (
        <ShareFolderBrowser
          token={token}
          initialBrowse={state.folder ?? null}
          requiresPassword={state.status === "needs_password"}
        />
      )}
    </main>
  );
}
