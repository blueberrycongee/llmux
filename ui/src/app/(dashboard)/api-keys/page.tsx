"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Plus } from "lucide-react";

export default function ApiKeysPage() {
    return (
        <div className="space-y-6">
            <div className="flex items-center justify-between">
                <div>
                    <h1 className="text-3xl font-bold tracking-tight">API Keys</h1>
                    <p className="text-muted-foreground">
                        Manage your API keys for accessing LLM models.
                    </p>
                </div>
                <Button className="gap-2">
                    <Plus className="w-4 h-4" />
                    Create New Key
                </Button>
            </div>

            <Card className="glass-card">
                <CardHeader>
                    <CardTitle>Active Keys</CardTitle>
                </CardHeader>
                <CardContent>
                    <div className="text-sm text-muted-foreground py-8 text-center">
                        No API keys found. Create one to get started.
                    </div>
                </CardContent>
            </Card>
        </div>
    );
}
