"use client";

import { useTheme } from "next-themes";
import { useTranslations } from "next-intl";
import { Moon, Sun } from "lucide-react";

import { Button } from "@/components/ui/button";

// Toggle between light and dark. The visible icon is driven purely by the
// `.dark` class on <html> (which next-themes sets before first paint), so
// there's no hydration mismatch and no client-only mount state — the CSS
// shows the moon in light mode and the sun in dark mode.
export function ThemeToggle() {
  const t = useTranslations("ThemeToggle");
  const { resolvedTheme, setTheme } = useTheme();

  return (
    <Button
      variant="ghost"
      size="icon-sm"
      aria-label={t("label")}
      title={t("label")}
      onClick={() => setTheme(resolvedTheme === "dark" ? "light" : "dark")}
    >
      <Moon className="size-4 dark:hidden" />
      <Sun className="hidden size-4 dark:block" />
    </Button>
  );
}
