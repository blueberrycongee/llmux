"use client";

import React, { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import {
  DEFAULT_LOCALE,
  LOCALE_COOKIE,
  LOCALE_STORAGE_KEY,
  localeToHtmlLang,
  normalizeLocale,
  translate,
  type AppLocale,
  type MessageVars,
} from "@/i18n/i18n";

type I18nContextValue = {
  locale: AppLocale;
  setLocale: (next: AppLocale) => void;
  t: (key: string, vars?: MessageVars) => string;
};

const I18nContext = createContext<I18nContextValue | null>(null);

function readCookieLocale(): AppLocale | null {
  if (typeof document === "undefined") return null;
  const match = document.cookie.match(new RegExp(`(?:^|;\\s*)${LOCALE_COOKIE}=([^;]+)`));
  if (!match) return null;
  return normalizeLocale(decodeURIComponent(match[1]));
}

function writeLocaleCookie(locale: AppLocale) {
  if (typeof document === "undefined") return;
  const oneYear = 60 * 60 * 24 * 365;
  document.cookie = `${LOCALE_COOKIE}=${encodeURIComponent(locale)}; path=/; max-age=${oneYear}`;
}

function readStoredLocale(): AppLocale | null {
  if (typeof window === "undefined") return null;
  try {
    const v = window.localStorage.getItem(LOCALE_STORAGE_KEY);
    return v ? normalizeLocale(v) : null;
  } catch {
    return null;
  }
}

function writeStoredLocale(locale: AppLocale) {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(LOCALE_STORAGE_KEY, locale);
  } catch {
    // ignore
  }
}

export function LocaleProvider({
  initialLocale,
  children,
}: {
  initialLocale?: AppLocale;
  children: React.ReactNode;
}) {
  const [locale, setLocaleState] = useState<AppLocale>(initialLocale ?? DEFAULT_LOCALE);

  // On mount, prefer persisted locale (cookie > localStorage) if different.
  useEffect(() => {
    const persisted = readCookieLocale() ?? readStoredLocale();
    if (persisted && persisted !== locale) {
      setLocaleState(persisted);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (typeof document !== "undefined") {
      document.documentElement.lang = localeToHtmlLang(locale);
    }
  }, [locale]);

  const setLocale = useCallback((next: AppLocale) => {
    const normalized = normalizeLocale(next);
    setLocaleState(normalized);
    writeLocaleCookie(normalized);
    writeStoredLocale(normalized);
  }, []);

  const t = useCallback(
    (key: string, vars?: MessageVars) => translate(locale, key, vars),
    [locale]
  );

  const value = useMemo<I18nContextValue>(() => ({ locale, setLocale, t }), [locale, setLocale, t]);

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n(): I18nContextValue {
  const ctx = useContext(I18nContext);
  if (!ctx) {
    throw new Error("useI18n must be used within LocaleProvider");
  }
  return ctx;
}

