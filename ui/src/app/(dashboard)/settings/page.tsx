"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Save } from "lucide-react";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
import { useI18n } from "@/i18n/locale-provider";
import type { AppLocale } from "@/i18n/i18n";

export default function SettingsPage() {
    const { t, locale, setLocale } = useI18n();

    return (
        <div className="space-y-6">
            <div className="flex items-center justify-between">
                <div>
                    <h1 className="text-3xl font-bold tracking-tight">{t("settings.title")}</h1>
                    <p className="text-muted-foreground">
                        {t("settings.description")}
                    </p>
                </div>
                <Button className="gap-2">
                    <Save className="w-4 h-4" />
                    {t("common.saveChanges")}
                </Button>
            </div>

            <Card className="glass-card">
                <CardHeader>
                    <CardTitle>{t("settings.general.title")}</CardTitle>
                </CardHeader>
                <CardContent>
                    <div className="space-y-6">
                        <div>
                            <div className="font-medium">{t("settings.language.title")}</div>
                            <div className="text-sm text-muted-foreground">{t("settings.language.description")}</div>
                            <div className="mt-3 max-w-xs">
                                <Select value={locale} onValueChange={(v) => setLocale(v as AppLocale)}>
                                    <SelectTrigger>
                                        <SelectValue />
                                    </SelectTrigger>
                                    <SelectContent>
                                        <SelectItem value="cn">{t("settings.language.cn")}</SelectItem>
                                        <SelectItem value="i18n">{t("settings.language.i18n")}</SelectItem>
                                    </SelectContent>
                                </Select>
                            </div>
                        </div>

                        <div className="text-sm text-muted-foreground py-4 text-center">
                            {t("settings.general.comingSoon")}
                        </div>
                    </div>
                </CardContent>
            </Card>
        </div>
    );
}
