import type { Metadata } from "next";
import localFont from "next/font/local";
import "./globals.css";
import { Providers } from "@/components/providers";
import { cookies } from "next/headers";
import { LOCALE_COOKIE, localeToHtmlLang, normalizeLocale, type AppLocale } from "@/i18n/i18n";

const geistSans = localFont({
  src: "./fonts/GeistVF.woff",
  variable: "--font-geist-sans",
  weight: "100 900",
});
const geistMono = localFont({
  src: "./fonts/GeistMonoVF.woff",
  variable: "--font-geist-mono",
  weight: "100 900",
});

export const metadata: Metadata = {
  title: "LLMux - Enterprise LLM Gateway",
  description: "Multi-tenant LLM Gateway with SSO, RBAC, and Audit Logging",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  const cookieStore = cookies();
  const locale = normalizeLocale(cookieStore.get(LOCALE_COOKIE)?.value) as AppLocale;

  return (
    <html lang={localeToHtmlLang(locale)} suppressHydrationWarning>
      <body
        className={`${geistSans.variable} ${geistMono.variable} antialiased`}
      >
        <Providers initialLocale={locale}>
          {children}
        </Providers>
      </body>
    </html>
  );
}
