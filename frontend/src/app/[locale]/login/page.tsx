import { getTranslations } from "next-intl/server";

import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";

export default async function LoginPage() {
  const t = await getTranslations("Login");

  return (
    <main className="flex flex-1 flex-col items-center justify-center gap-6 px-4 text-center">
      <div className="space-y-2">
        <h1 className="text-2xl">{t("title")}</h1>
        <p className="text-muted-foreground">{t("subtitle")}</p>
      </div>
      <div className="flex w-full max-w-xs flex-col gap-3">
        {/* Real <a> navigation, not fetch: this must be a top-level browser
            redirect to the backend, which itself redirects to the OAuth
            provider's consent screen. */}
        <Button size="lg" nativeButton={false} render={<a href={api.authorizeUrl("github")} />}>
          {t("continueWithGithub")}
        </Button>
        <Button disabled={true} size="lg" variant="outline" nativeButton={false} render={<a href={api.authorizeUrl("google")} />}>
          {t("continueWithGoogle")}
        </Button>
      </div>
    </main>
  );
}
