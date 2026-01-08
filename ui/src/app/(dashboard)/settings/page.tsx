"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Save } from "lucide-react";

export default function SettingsPage() {
    return (
        <div className="space-y-6">
            <div className="flex items-center justify-between">
                <div>
                    <h1 className="text-3xl font-bold tracking-tight">Settings</h1>
                    <p className="text-muted-foreground">
                        Configure system preferences and defaults.
                    </p>
                </div>
                <Button className="gap-2">
                    <Save className="w-4 h-4" />
                    Save Changes
                </Button>
            </div>

            <Card className="glass-card">
                <CardHeader>
                    <CardTitle>General Settings</CardTitle>
                </CardHeader>
                <CardContent>
                    <div className="text-sm text-muted-foreground py-8 text-center">
                        Settings configuration coming soon.
                    </div>
                </CardContent>
            </Card>
        </div>
    );
}
