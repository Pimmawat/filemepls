import type { Metadata } from "next";
import { Geist_Mono, Nunito, Quicksand } from "next/font/google";
import localFont from "next/font/local";
import { hasLocale, NextIntlClientProvider } from "next-intl";
import { notFound } from "next/navigation";

import { routing } from "@/i18n/routing";
import { Nav } from "@/components/nav";
import { Toaster } from "@/components/ui/sonner";
import "./globals.css";

// Wordmark + all headings.
const quicksand = Quicksand({
  variable: "--font-quicksand",
  weight: "700",
  subsets: ["latin"],
});

// Body text: taglines, labels, captions.
const nunito = Nunito({
  variable: "--font-nunito",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

// Thai fallback for both body and heading stacks (globals.css) — Quicksand
// and Nunito have no Thai glyphs. Google Sans itself does (it's OFL-
// licensed, same as the other fonts here — see fonts/LICENSE.txt), so it's
// vendored directly via next/font/local rather than next/font/google,
// since this Next.js version's bundled Google Fonts catalog predates
// Google Sans's public release and doesn't list it yet. Subsetted to
// Thai + basic Latin only (not the full ~450KB/weight multi-script file)
// since this is purely a Thai fallback — Latin text still renders in
// Nunito/Quicksand via CSS's per-glyph font-family fallback.
const googleSansThai = localFont({
  variable: "--font-google-sans-thai",
  src: [
    { path: "./fonts/GoogleSans-Regular.woff2", weight: "400", style: "normal" },
    { path: "./fonts/GoogleSans-Bold.woff2", weight: "700", style: "normal" },
  ],
});

export const metadata: Metadata = {
  title: "FileMePls",
  description: "Self-hosted file storage and sharing",
};

type Props = {
  children: React.ReactNode;
  params: Promise<{ locale: string }>;
};

export default async function LocaleLayout({ children, params }: Props) {
  const { locale } = await params;

  if (!hasLocale(routing.locales, locale)) {
    notFound();
  }

  return (
    <html
      lang={locale}
      className={`${quicksand.variable} ${nunito.variable} ${geistMono.variable} ${googleSansThai.variable} h-full antialiased`}
    >
      <body className="min-h-full flex flex-col">
        <NextIntlClientProvider locale={locale}>
          <Nav />
          <div className="flex flex-1 flex-col">{children}</div>
          <Toaster />
        </NextIntlClientProvider>
      </body>
    </html>
  );
}
