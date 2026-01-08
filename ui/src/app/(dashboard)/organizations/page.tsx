"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Plus } from "lucide-react";

export default function OrganizationsPage() {
    return (
        <div className="space-y-6">
            <div className="flex items-center justify-between">
                <div>
                    <h1 className="text-3xl font-bold tracking-tight">Organizations</h1>
                    <p className="text-muted-foreground">
                        Manage organization settings and billing.
                    </p>
                </div>
                <Button className="gap-2">
                    <Plus className="w-4 h-4" />
                    New Organization
                </Button>
            </div>

            <Card className="glass-card">
                <CardHeader>
                    <CardTitle>Your Organizations</CardTitle>
                </CardHeader>
                <CardContent>
                    <div className="text-sm text-muted-foreground py-8 text-center">
                        No organizations found.
                    </div>
                </CardContent>
            </Card>
        </div>
    );
}
