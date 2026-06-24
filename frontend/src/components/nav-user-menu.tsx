"use client";

import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { useRouter } from "@/i18n/navigation";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { api, ApiError, type User } from "@/lib/api";

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
      <DropdownMenuTrigger render={<Button variant="ghost" size="sm" />}>
        {user.displayName}
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem onClick={handleLogout}>{t("logout")}</DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
