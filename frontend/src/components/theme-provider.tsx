"use client";

import { ThemeProvider as NextThemesProvider } from "next-themes";
import type { ThemeProviderProps } from "next-themes";

// Client boundary around next-themes so the server-rendered layout can wrap
// the app in a theme context. attribute="class" toggles the `.dark` class on
// <html>, which is what globals.css keys its dark-mode tokens off of.
export function ThemeProvider({ children, ...props }: ThemeProviderProps) {
  return <NextThemesProvider {...props}>{children}</NextThemesProvider>;
}
