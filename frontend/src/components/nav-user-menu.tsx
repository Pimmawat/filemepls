"use client";

import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { Link, useRouter } from "@/i18n/navigation";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { api, ApiError, type User } from "@/lib/api";

function initials(name: string) {
  return name
    .trim()
    .split(/\s+/)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase())
    .join("");
}

export function NavUserMenu({ user }: { user: User }) {
  const t = useTranslations("Nav");
  const router = useRouter();

  async function handleLogout() {
    try {
      await api.logout();
      router.refresh();
    } catch (err) {
      toast.error(t("logoutFailed"), {
        description: err instanceof ApiError ? err.message : undefined,
      });
    }
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={<Button variant="ghost" size="sm" className="gap-2" />}
      >
        <Avatar className="size-6">
          <AvatarImage src={user.avatarUrl} alt={user.displayName} />
          <AvatarFallback>{initials(user.displayName)}</AvatarFallback>
        </Avatar>
        {user.displayName}
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem render={<Link href="/files" />}>{t("files")}</DropdownMenuItem>
        <DropdownMenuItem render={<Link href="/shared-with-me" />}>
          {t("sharedWithMe")}
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={handleLogout}>{t("logout")}</DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
