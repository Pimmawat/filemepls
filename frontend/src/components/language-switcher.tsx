"use client";

import { useLocale, useTranslations } from "next-intl";
import { Globe } from "lucide-react";

import { usePathname, useRouter } from "@/i18n/navigation";
import { routing } from "@/i18n/routing";
import { Button } from "@/components/ui/button";

// Two locales, so this is a direct toggle rather than a dropdown: one click
// switches straight to the other language on the current page. The label is
// the current locale's short code (TH / EN).
export function LanguageSwitcher() {
  const t = useTranslations("LanguageSwitcher");
  const locale = useLocale();
  const router = useRouter();
  const pathname = usePathname();

  const nextLocale = routing.locales.find((l) => l !== locale) ?? locale;

  return (
    <Button
      variant="ghost"
      size="sm"
      aria-label={t("label")}
      title={t("label")}
      onClick={() => router.replace(pathname, { locale: nextLocale })}
    >
      <Globe className="size-4" />
      <span className="font-medium">{locale.toUpperCase()}</span>
    </Button>
  );
}
