import "server-only";
import { cookies } from "next/headers";

import { api, type User } from "@/lib/api";

const SESSION_COOKIE = "filemepls_session";

// Next.js server-side fetch does not automatically forward the incoming
// request's cookies to a different origin, so Server Components must read
// the cookie themselves and pass it through explicitly.
async function getSessionCookieHeader(): Promise<string | undefined> {
  const store = await cookies();
  const value = store.get(SESSION_COOKIE)?.value;
  return value ? `${SESSION_COOKIE}=${value}` : undefined;
}

export async function getCurrentUser(): Promise<User | null> {
  const cookie = await getSessionCookieHeader();
  if (!cookie) return null;
  try {
    return await api.me(cookie);
  } catch {
    return null;
  }
}
