import { getTranslations } from "next-intl/server";
import Image from "next/image";

import { Link } from "@/i18n/navigation";
import { LanguageSwitcher } from "@/components/language-switcher";
import { NavUserMenu } from "@/components/nav-user-menu";
import { Button } from "@/components/ui/button";
import { Wordmark } from "@/components/wordmark";
import { getCurrentUser } from "@/lib/session";
import icon from "@/app/[locale]/icon.png";

export async function Nav() {
  const t = await getTranslations("Nav");
  const user = await getCurrentUser();

  return (
    <header className="border-b">
      <div className="mx-auto flex h-14 max-w-5xl items-center justify-between px-4">
        <Link href="/" className="flex items-center gap-2">
          <Image src={icon} alt="" className="size-7" />
          <Wordmark />
        </Link>
        <nav className="flex items-center gap-4 text-sm">
          <Link href="/send" className="text-muted-foreground hover:text-foreground">
            {t("send")}
          </Link>
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
