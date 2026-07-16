"use client";

import { useTranslations } from "next-intl";
import { Moon, Sun } from "lucide-react";

import { useTheme } from "@/components/theme-provider";
import { Button } from "@/components/ui/button";

// Toggle light/dark. The visible icon is driven purely by the `.dark` class on
// <html> via CSS, so there's no hydration mismatch and no client-only state:
// a moon in light mode, a sun in dark mode.
export function ThemeToggle() {
  const t = useTranslations("ThemeToggle");
  const { toggle } = useTheme();

  return (
    <Button
      variant="ghost"
      size="icon-sm"
      aria-label={t("label")}
      title={t("label")}
      onClick={toggle}
    >
      <Moon className="size-4 dark:hidden" />
      <Sun className="hidden size-4 dark:block" />
    </Button>
  );
}
