"use client";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ThemeProvider } from "@/components/theme-provider";
import { useState } from "react";
import { LocaleProvider } from "@/i18n/locale-provider";
import type { AppLocale } from "@/i18n/i18n";

export function Providers({
    children,
    initialLocale,
}: {
    children: React.ReactNode;
    initialLocale?: AppLocale;
}) {
    const [queryClient] = useState(() => new QueryClient({
        defaultOptions: {
            queries: {
                staleTime: 60 * 1000,
            },
        },
    }));

    return (
        <QueryClientProvider client={queryClient}>
            <LocaleProvider initialLocale={initialLocale}>
                <ThemeProvider
                    attribute="class"
                    defaultTheme="dark"
                    enableSystem
                    disableTransitionOnChange
                >
                    {children}
                </ThemeProvider>
            </LocaleProvider>
        </QueryClientProvider>
    );
}
