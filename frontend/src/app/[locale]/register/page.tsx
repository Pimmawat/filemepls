import { getLocale, getTranslations } from "next-intl/server";

import { Link, redirect } from "@/i18n/navigation";
import { getCurrentUser } from "@/lib/session";
import { RegisterForm } from "./register-form";

export default async function RegisterPage() {
  const locale = await getLocale();
  const user = await getCurrentUser();
  if (user) {
    redirect({ href: "/files", locale });
    return;
  }

  const t = await getTranslations("Register");

  return (
    <main className="flex flex-1 flex-col items-center justify-center gap-6 px-4 text-center">
      <div className="space-y-2">
        <h1 className="text-2xl">{t("title")}</h1>
        <p className="text-muted-foreground">{t("subtitle")}</p>
      </div>

      <RegisterForm />

      <p className="text-sm text-muted-foreground">
        {t("haveAccount")}{" "}
        <Link href="/login" className="underline">
          {t("loginLink")}
        </Link>
      </p>
    </main>
  );
}
