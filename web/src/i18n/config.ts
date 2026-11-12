import i18n from "i18next";
import { initReactI18next } from "react-i18next";

import enUS from "./locales/en-US.json";
import zhCN from "./locales/zh-CN.json";
import zhTW from "./locales/zh-TW.json";

// Keep legacy locale codes for stored preferences, but all variants now use English content.
export const LANGS = [
  { code: "en-US", label: "English (Global)" },
  { code: "zh-CN", label: "English (Legacy zh-CN)" },
  { code: "zh-TW", label: "English (Legacy zh-TW)" },
] as const;

export type LangCode = (typeof LANGS)[number]["code"];

export const DEFAULT_LANG: LangCode = "en-US";
export const LANG_STORAGE_KEY = "antifox-lang";

export function isLangCode(v: string | null | undefined): v is LangCode {
  return !!v && LANGS.some((l) => l.code === v);
}

// Resources are packed directly (non-staggered)——Static export friendly, no Suspense、SSR/First frame is the same.
if (!i18n.isInitialized) {
  void i18n.use(initReactI18next).init({
    resources: {
      "zh-CN": { translation: zhCN },
      "zh-TW": { translation: zhTW },
      "en-US": { translation: enUS },
    },
    lng: DEFAULT_LANG,
    fallbackLng: DEFAULT_LANG,
    interpolation: { escapeValue: false },
    returnNull: false,
  });
}

export default i18n;
