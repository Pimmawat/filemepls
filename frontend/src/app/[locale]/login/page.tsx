import { getLocale, getTranslations } from "next-intl/server";

import { Link, redirect } from "@/i18n/navigation";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { api } from "@/lib/api";
import { getCurrentUser } from "@/lib/session";
import { LoginForm } from "./login-form";

export default async function LoginPage() {
  const locale = await getLocale();
  const user = await getCurrentUser();
  if (user) {
    redirect({ href: "/files", locale });
    return;
  }

  const t = await getTranslations("Login");

  return (
    <main className="flex flex-1 flex-col items-center justify-center gap-6 px-4 text-center">
      <div className="space-y-2">
        <h1 className="text-2xl">{t("title")}</h1>
        <p className="text-muted-foreground">{t("subtitle")}</p>
      </div>

      <LoginForm />

      <p className="text-sm text-muted-foreground">
        {t("noAccount")}{" "}
        <Link href="/register" className="underline">
          {t("registerLink")}
        </Link>
      </p>

      <div className="flex w-full max-w-xs items-center gap-2">
        <Separator className="flex-1" />
        <span className="text-xs text-muted-foreground">{t("or")}</span>
        <Separator className="flex-1" />
      </div>

      <div className="flex w-full max-w-xs flex-col gap-3">
        {/* Real <a> navigation, not fetch: this must be a top-level browser
            redirect to the backend, which itself redirects to the OAuth
            provider's consent screen. */}
        <Button size="lg" variant="outline" nativeButton={false} render={<a href={api.authorizeUrl("github")} />}>
          {t("continueWithGithub")}
        </Button>
        <Button size="lg" variant="outline" nativeButton={false} render={<a href={api.authorizeUrl("google")} />}>
          {t("continueWithGoogle")}
        </Button>
      </div>
    </main>
  );
}
