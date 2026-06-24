import type { NextConfig } from "next";
import createNextIntlPlugin from "next-intl/plugin";

const withNextIntl = createNextIntlPlugin("./src/i18n/request.ts");

const nextConfig: NextConfig = {
  // Self-contained server build (.next/standalone) for deploying outside
  // Vercel — only the files actually needed at runtime, not the full
  // node_modules tree. Required for the native Windows service deployment.
  output: "standalone",
};

export default withNextIntl(nextConfig);
