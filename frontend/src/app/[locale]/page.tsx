import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";

export default function Home() {
  const t = useTranslations("Home");

  return (
    <main className="flex flex-1 flex-col items-center justify-center gap-6 px-4 text-center">
      <h1 className="max-w-xl text-4xl tracking-tight">
        {t("title")}
      </h1>
      <p className="max-w-md text-lg text-muted-foreground">{t("subtitle")}</p>
      <Button size="lg">{t("cta")}</Button>
    </main>
  );
}
