import { cookies } from "next/headers";
import { getLocale } from "next-intl/server";

import { redirect } from "@/i18n/navigation";
import { api, ApiError } from "@/lib/api";
import { getCurrentUser } from "@/lib/session";
import { FileManager } from "../file-manager";

type Props = {
  params: Promise<{ locale: string; folderId: string }>;
};

export default async function FolderPage({ params }: Props) {
  const { folderId } = await params;
  const locale = await getLocale();
  const user = await getCurrentUser();
  if (!user) {
    redirect({ href: "/login", locale });
  }

  const store = await cookies();
  const sessionCookie = store.get("filemepls_session")?.value;
  const cookieHeader = sessionCookie ? `filemepls_session=${sessionCookie}` : undefined;

  let browse;
  try {
    browse = await api.browse(folderId, cookieHeader);
  } catch (err) {
    if (err instanceof ApiError && (err.status === 404 || err.status === 403)) {
      redirect({ href: "/files", locale });
    }
    throw err;
  }

  return (
    <main className="mx-auto w-full max-w-5xl flex-1 px-4 py-10">
      <FileManager initialBrowse={browse} folderId={folderId} />
    </main>
  );
}
