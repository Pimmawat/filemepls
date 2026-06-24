"use client";

import { useTranslations } from "next-intl";

import { Link } from "@/i18n/navigation";
import { cn } from "@/lib/utils";
import type { FolderMeta } from "@/lib/api";

type Props = {
  items: FolderMeta[];
  // basePath renders each segment as a link to `${basePath}/${folder.id}`
  // (or basePath itself for Home) — used for the owner's file manager,
  // where each folder is a real route. Mutually exclusive with onNavigate.
  basePath?: string;
  // onNavigate renders each segment as a click target instead of a link —
  // used for public share browsing, which is kept as client-side state
  // rather than real routes (the backend has no session for anonymous
  // visitors, so a real per-folder URL would mean re-entering any required
  // password on every navigation).
  onNavigate?: (folderId: string | null) => void;
  // onDropOnSegment lets a drag-and-drop move target a breadcrumb segment
  // (folderId null = Home/root).
  onDropOnSegment?: (folderId: string | null) => void;
};

export function Breadcrumb({ items, basePath, onNavigate, onDropOnSegment }: Props) {
  const t = useTranslations("Breadcrumb");

  function segment(label: string, href: string | undefined, folderId: string | null, isLast: boolean) {
    const content = (
      <span
        className={cn(
          "rounded px-1.5 py-0.5 text-sm",
          isLast ? "font-medium text-foreground" : "text-muted-foreground hover:text-foreground",
        )}
        onDragOver={onDropOnSegment ? (e) => e.preventDefault() : undefined}
        onDrop={
          onDropOnSegment
            ? (e) => {
                e.preventDefault();
                onDropOnSegment(folderId);
              }
            : undefined
        }
      >
        {label}
      </span>
    );
    if (isLast) return content;
    if (onNavigate) {
      return (
        <button type="button" onClick={() => onNavigate(folderId)}>
          {content}
        </button>
      );
    }
    if (href) return <Link href={href}>{content}</Link>;
    return content;
  }

  return (
    <nav className="flex items-center gap-1 text-sm">
      {segment(t("home"), basePath, null, items.length === 0)}
      {items.map((folder, i) => (
        <span key={folder.id} className="flex items-center gap-1">
          <span className="text-muted-foreground">/</span>
          {segment(
            folder.name,
            basePath ? `${basePath}/${folder.id}` : undefined,
            folder.id,
            i === items.length - 1,
          )}
        </span>
      ))}
    </nav>
  );
}
