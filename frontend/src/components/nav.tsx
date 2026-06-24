import { getTranslations } from "next-intl/server";

import { Link } from "@/i18n/navigation";
import { LanguageSwitcher } from "@/components/language-switcher";
import { NavUserMenu } from "@/components/nav-user-menu";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { Wordmark } from "@/components/wordmark";
import { getCurrentUser } from "@/lib/session";

export async function Nav() {
  const t = await getTranslations("Nav");
  const user = await getCurrentUser();

  return (
    <header className="border-b">
      <div className="mx-auto flex h-14 max-w-5xl items-center justify-between px-4">
        <Link href="/">
          <Wordmark />
        </Link>
        <nav className="flex items-center gap-4 text-sm">
          <Link href="/files" className="text-muted-foreground hover:text-foreground">
            {t("files")}
          </Link>
          <Separator orientation="vertical" className="h-5" />
          <LanguageSwitcher />
          {user ? (
            <NavUserMenu user={user} />
          ) : (
            <Button size="sm" nativeButton={false} render={<Link href="/login" />}>
              {t("login")}
            </Button>
          )}
        </nav>
      </div>
    </header>
  );
}
