import type { AppLocale, Messages, MessageVars } from "./types";

export type { AppLocale, Messages, MessageVars };

export const LOCALE_COOKIE = "llmux_locale";
export const LOCALE_STORAGE_KEY = "llmux_locale";
export const DEFAULT_LOCALE: AppLocale = "i18n";

export function normalizeLocale(v?: string | null): AppLocale {
  if (!v) return DEFAULT_LOCALE;
  const raw = String(v).trim().toLowerCase();
  if (raw === "cn" || raw === "zh" || raw === "zh-cn" || raw === "zh-hans") return "cn";
  if (raw === "i18n" || raw === "en" || raw === "en-us" || raw === "en-gb") return "i18n";
  return DEFAULT_LOCALE;
}

export function localeToHtmlLang(locale: AppLocale): string {
  return locale === "cn" ? "zh-CN" : "en";
}

export function formatMessage(template: string, vars?: MessageVars): string {
  if (!vars) return template;
  return template.replace(/\{(\w+)\}/g, (_, key: string) => {
    const v = vars[key];
    if (v === null || v === undefined) return "";
    return String(v);
  });
}

import { messages as cn } from "@/i18n/messages/cn";
import { messages as i18n } from "@/i18n/messages/i18n";

export const messagesByLocale: Record<AppLocale, Messages> = {
  cn,
  i18n,
};

export function translate(locale: AppLocale, key: string, vars?: MessageVars): string {
  const dict = messagesByLocale[locale];
  const fallback = messagesByLocale[DEFAULT_LOCALE];

  const template = dict[key] ?? fallback[key] ?? key;
  return formatMessage(template, vars);
}

