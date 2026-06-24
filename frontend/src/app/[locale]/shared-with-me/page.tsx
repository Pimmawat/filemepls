import { cookies } from "next/headers";
import { getLocale } from "next-intl/server";

import { redirect } from "@/i18n/navigation";
import { api } from "@/lib/api";
import { getCurrentUser } from "@/lib/session";
import { SharedWithMeList } from "./shared-with-me-list";

export default async function SharedWithMePage() {
  const locale = await getLocale();
  const user = await getCurrentUser();
  if (!user) {
    redirect({ href: "/login", locale });
    return;
  }

  const store = await cookies();
  const sessionCookie = store.get("filemepls_session")?.value;
  const cookieHeader = sessionCookie ? `filemepls_session=${sessionCookie}` : undefined;

  const shared = await api.sharedWithMe(cookieHeader);

  return (
    <main className="mx-auto w-full max-w-5xl flex-1 px-4 py-10">
      <SharedWithMeList initial={shared} />
    </main>
  );
}
