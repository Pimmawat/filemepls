import { cookies } from "next/headers";
import { getLocale } from "next-intl/server";

import { redirect } from "@/i18n/navigation";
import { api } from "@/lib/api";
import { getCurrentUser } from "@/lib/session";
import { FileManager } from "./file-manager";

export default async function FilesPage() {
  const locale = await getLocale();
  const user = await getCurrentUser();
  if (!user) {
    redirect({ href: "/login", locale });
  }

  const store = await cookies();
  const sessionCookie = store.get("filemepls_session")?.value;
  const cookieHeader = sessionCookie ? `filemepls_session=${sessionCookie}` : undefined;

  const browse = await api.browse(undefined, cookieHeader);

  return (
    <main className="mx-auto w-full max-w-5xl flex-1 px-4 py-10">
      <FileManager initialBrowse={browse} folderId={null} />
    </main>
  );
}
