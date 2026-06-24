import type { NextRequest } from "next/server";
import createMiddleware from "next-intl/middleware";
import { routing } from "./i18n/routing";

const handleI18n = createMiddleware(routing);

// Next.js 16 renamed the `middleware` file convention to `proxy`; next-intl's
// `createMiddleware` still returns a plain NextRequest -> NextResponse
// function, which is exactly what `proxy` expects.
export function proxy(request: NextRequest) {
  return handleI18n(request);
}

export const config = {
  matcher: ["/((?!api|trpc|_next|_vercel|.*\\..*).*)"],
};
