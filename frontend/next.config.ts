import type { NextConfig } from "next";
import createNextIntlPlugin from "next-intl/plugin";

const withNextIntl = createNextIntlPlugin("./src/i18n/request.ts");

// Baseline security headers applied to every route Next serves (the API lives
// on the separate Go backend, which sets its own — see middleware_security.go).
// A full script-src CSP is intentionally omitted: a strict one needs
// per-request nonces + a Proxy and forces dynamic rendering (see the Next CSP
// guide). frame-ancestors here still blocks framing without touching scripts.
const securityHeaders = [
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "X-Frame-Options", value: "DENY" },
  { key: "Referrer-Policy", value: "no-referrer" },
  { key: "Strict-Transport-Security", value: "max-age=31536000; includeSubDomains" },
  { key: "Permissions-Policy", value: "camera=(), microphone=(), geolocation=()" },
  { key: "Content-Security-Policy", value: "frame-ancestors 'none'" },
];

const nextConfig: NextConfig = {
  // Self-contained server build (.next/standalone) for deploying outside
  // Vercel — only the files actually needed at runtime, not the full
  // node_modules tree. `postbuild` copies public/ and .next/static into it;
  // server.js does not do that itself.
  output: "standalone",
  // Don't advertise the framework/version in the X-Powered-By header.
  poweredByHeader: false,
  async headers() {
    return [
      {
        source: "/:path*",
        headers: securityHeaders,
      },
    ];
  },
};

export default withNextIntl(nextConfig);
