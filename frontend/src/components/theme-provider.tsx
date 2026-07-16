"use client";

import { createContext, useContext, useEffect, useMemo } from "react";

type Theme = "light" | "dark";

type ThemeContextValue = {
  setTheme: (theme: Theme) => void;
  toggle: () => void;
};

const ThemeContext = createContext<ThemeContextValue | null>(null);

export const THEME_COOKIE = "theme";

function readThemeCookie(): Theme {
  if (typeof document === "undefined") return "light";
  return /(?:^|;\s*)theme=dark(?:;|$)/.test(document.cookie) ? "dark" : "light";
}

function applyTheme(theme: Theme) {
  const root = document.documentElement;
  root.classList.toggle("dark", theme === "dark");
  root.style.colorScheme = theme;
}

// Theme controller. The server sets the initial `.dark` class on <html> from
// the cookie (no flash, no <script>); this keeps it in sync afterwards:
//   - toggle/setTheme flip the class instantly and persist the choice in the
//     cookie so the next server render matches.
//   - the effect re-asserts the cookie's theme after every render, which
//     matters on a locale change: that re-renders this layout and can reset
//     <html>'s className to whatever the server produced, so we re-apply the
//     user's actual choice on top.
export function ThemeProvider({ children }: { children: React.ReactNode }) {
  useEffect(() => {
    applyTheme(readThemeCookie());
  });

  const value = useMemo<ThemeContextValue>(() => {
    const set = (theme: Theme) => {
      applyTheme(theme);
      document.cookie = `${THEME_COOKIE}=${theme}; path=/; max-age=31536000; SameSite=Lax`;
    };
    return {
      setTheme: set,
      toggle: () =>
        set(document.documentElement.classList.contains("dark") ? "light" : "dark"),
    };
  }, []);

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme() {
  const ctx = useContext(ThemeContext);
  if (!ctx) {
    throw new Error("useTheme must be used within a ThemeProvider");
  }
  return ctx;
}
