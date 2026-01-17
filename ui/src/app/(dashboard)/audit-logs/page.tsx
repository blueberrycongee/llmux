"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Download } from "lucide-react";
import { useI18n } from "@/i18n/locale-provider";

export default function AuditLogsPage() {
    const { t } = useI18n();
    return (
        <div className="space-y-6">
            <div className="flex items-center justify-between">
                <div>
                    <h1 className="text-3xl font-bold tracking-tight">{t("dashboard.auditLogs.title")}</h1>
                    <p className="text-muted-foreground">
                        {t("dashboard.auditLogs.description")}
                    </p>
                </div>
                <Button variant="outline" className="gap-2">
                    <Download className="w-4 h-4" />
                    {t("dashboard.auditLogs.action.export")}
                </Button>
            </div>

            <Card className="glass-card">
                <CardHeader>
                    <CardTitle>{t("dashboard.auditLogs.section.recent")}</CardTitle>
                </CardHeader>
                <CardContent>
                    <div className="text-sm text-muted-foreground py-8 text-center">
                        {t("dashboard.auditLogs.empty")}
                    </div>
                </CardContent>
            </Card>
        </div>
    );
}
