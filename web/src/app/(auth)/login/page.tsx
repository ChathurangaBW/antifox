"use client";

import { useEffect, useState } from "react";

import { useRouter } from "next/navigation";

import { useTranslation } from "react-i18next";

import { LanguageSwitcher } from "@/app/(main)/_components/sidebar/language-switcher";
import { AntifoxLogo } from "@/components/antifox-logo";
import { BrandBackdrop, BrandWordmark, brand } from "@/components/brand-backdrop";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { api } from "@/lib/api";
import { auth } from "@/lib/auth";

export default function LoginPage() {
  const router = useRouter();
  const { t } = useTranslation();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [checking, setChecking] = useState(true);

  useEffect(() => {
    // Login directly into main interface (static export no) middleware The floor jumps on the line.
    if (auth.getToken()) {
      router.replace("/function/tasks");
      return;
    }
    api.authStatus()
      .then(({ initialized }) => {
        if (!initialized) router.replace("/setup");
      })
      .catch(() => setError(t("login.errBackend")))
      .finally(() => setChecking(false));
  }, [router, t]);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError("");
    try {
      const { token } = await api.login(username.trim(), password);
      auth.setToken(token);
      router.replace("/function/tasks");
    } catch (e) {
      setError((e as Error)?.message || t("login.errFailed"));
    } finally {
      setLoading(false);
    }
  }

  if (checking) return null;

  return (
    <div className="relative flex h-dvh">
      <div className="absolute top-4 right-4 z-20">
        <LanguageSwitcher />
      </div>
      {/* Left brand panel: branded backdrop in the logo's colours + logo and name */}
      <div className="relative hidden flex-col items-center justify-center overflow-hidden p-12 text-center lg:flex lg:w-1/2">
        <BrandBackdrop />
        <div className="relative z-10 flex flex-col items-center gap-5">
          <AntifoxLogo className="size-28 drop-shadow-[0_10px_30px_rgba(224,96,26,0.35)]" />
          <div className="space-y-1.5">
            <BrandWordmark className="text-4xl" />
            <div className="text-xs font-medium uppercase tracking-[0.25em]" style={{ color: brand.cream, opacity: 0.7 }}>
              AI Autonomous Pentest Platform
            </div>
          </div>
        </div>
      </div>

      {/* Right panel */}
      <div className="flex w-full items-center justify-center bg-background p-8 lg:w-1/2">
        <div className="w-full max-w-md space-y-10 py-24 lg:py-32">
          <div className="space-y-4 text-center">
            <h2 className="text-2xl font-medium tracking-tight">{t("login.title")}</h2>
            <p className="mx-auto max-w-xl text-muted-foreground">{t("login.welcome")}</p>
          </div>
          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            <div className="space-y-1.5">
              <Label htmlFor="username">{t("login.username")}</Label>
              <Input
                id="username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                placeholder={t("login.usernamePlaceholder")}
                autoFocus
                autoComplete="username"
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="password">{t("login.password")}</Label>
              <Input
                id="password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder={t("login.passwordPlaceholder")}
                autoComplete="current-password"
              />
            </div>
            {error && <p className="text-sm text-destructive">{error}</p>}
            <Button type="submit" className="w-full" disabled={loading || !password}>
              {loading ? t("login.submitting") : t("login.submit")}
            </Button>
          </form>
        </div>
      </div>
    </div>
  );
}
