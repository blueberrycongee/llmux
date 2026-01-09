"use client";

import { useState, useEffect } from "react";
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription, SheetFooter } from "@/components/ui/sheet";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator"; // We might need to create this or use hr
import {
    Key,
    Shield,
    ShieldOff,
    RefreshCw,
    Trash2,
    Copy,
    Check,
    Calendar,
    DollarSign,
    Activity,
    AlertCircle,
    Save,
} from "lucide-react";
import { StatusBadge, BudgetProgress } from "@/components/shared/common";
import { apiClient } from "@/lib/api";
import type { APIKey, GenerateKeyRequest } from "@/types/api";

interface ApiKeySheetProps {
    apiKey: APIKey | null;
    open: boolean;
    onOpenChange: (open: boolean) => void;
    onUpdate: () => void;
}

export function ApiKeySheet({ apiKey, open, onOpenChange, onUpdate }: ApiKeySheetProps) {
    const [isEditing, setIsEditing] = useState(false);
    const [name, setName] = useState("");
    const [maxBudget, setMaxBudget] = useState("");
    const [isSaving, setIsSaving] = useState(false);
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        if (apiKey) {
            setName(apiKey.name);
            setMaxBudget(apiKey.max_budget?.toString() || "");
        }
    }, [apiKey]);

    if (!apiKey) return null;

    const handleSave = async () => {
        setIsSaving(true);
        setError(null);

        try {
            await apiClient.updateKey(apiKey.id, {
                name: name.trim() || undefined,
                max_budget: maxBudget ? parseFloat(maxBudget) : undefined,
            });
            onUpdate();
            setIsEditing(false);
        } catch (err) {
            setError(err instanceof Error ? err.message : "Failed to update key");
        } finally {
            setIsSaving(false);
        }
    };

    const handleBlock = async () => {
        try {
            await apiClient.blockKey(apiKey.id);
            onUpdate();
        } catch (err) {
            console.error(err);
        }
    };

    const handleUnblock = async () => {
        try {
            await apiClient.unblockKey(apiKey.id);
            onUpdate();
        } catch (err) {
            console.error(err);
        }
    };

    const handleDelete = async () => {
        if (confirm("Are you sure you want to delete this API key? This action cannot be undone.")) {
            try {
                await apiClient.deleteKeys([apiKey.id]);
                onUpdate();
                onOpenChange(false);
            } catch (err) {
                console.error(err);
            }
        }
    };

    return (
        <Sheet open={open} onOpenChange={onOpenChange}>
            <div className="space-y-6">
                <SheetHeader>
                    <div className="flex items-center justify-between">
                        <SheetTitle>API Key Details</SheetTitle>
                        <StatusBadge isActive={apiKey.is_active} blocked={apiKey.blocked} />
                    </div>
                    <SheetDescription>
                        View and manage configuration for this API key.
                    </SheetDescription>
                </SheetHeader>

                <div className="space-y-6">
                    {/* Key Info */}
                    <div className="p-4 rounded-lg bg-secondary/50 space-y-3">
                        <div className="flex items-center gap-3">
                            <div className="w-10 h-10 rounded-lg bg-primary/10 flex items-center justify-center">
                                <Key className="w-5 h-5 text-primary" />
                            </div>
                            <div>
                                <div className="font-medium">{apiKey.name}</div>
                                <div className="text-xs text-muted-foreground font-mono">
                                    {apiKey.key_prefix}...
                                </div>
                            </div>
                        </div>
                        <div className="flex items-center gap-2 text-xs text-muted-foreground">
                            <Calendar className="w-3 h-3" />
                            Created {new Date(apiKey.created_at).toLocaleDateString()}
                        </div>
                    </div>

                    {/* Edit Form */}
                    <div className="space-y-4">
                        <div className="flex items-center justify-between">
                            <h4 className="text-sm font-medium">Configuration</h4>
                            {!isEditing && (
                                <Button variant="ghost" size="sm" onClick={() => setIsEditing(true)}>
                                    Edit
                                </Button>
                            )}
                        </div>

                        {isEditing ? (
                            <div className="space-y-4 p-4 border rounded-lg animate-in fade-in slide-in-from-top-2">
                                <div className="space-y-2">
                                    <Label htmlFor="edit-name">Name</Label>
                                    <Input
                                        id="edit-name"
                                        value={name}
                                        onChange={(e) => setName(e.target.value)}
                                    />
                                </div>
                                <div className="space-y-2">
                                    <Label htmlFor="edit-budget">Max Budget</Label>
                                    <div className="relative">
                                        <span className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground">$</span>
                                        <Input
                                            id="edit-budget"
                                            type="number"
                                            value={maxBudget}
                                            onChange={(e) => setMaxBudget(e.target.value)}
                                            className="pl-7"
                                        />
                                    </div>
                                </div>
                                {error && (
                                    <div className="text-sm text-red-400 flex items-center gap-2">
                                        <AlertCircle className="w-4 h-4" />
                                        {error}
                                    </div>
                                )}
                                <div className="flex gap-2 justify-end">
                                    <Button variant="ghost" size="sm" onClick={() => setIsEditing(false)}>
                                        Cancel
                                    </Button>
                                    <Button size="sm" onClick={handleSave} disabled={isSaving}>
                                        {isSaving ? "Saving..." : "Save Changes"}
                                    </Button>
                                </div>
                            </div>
                        ) : (
                            <div className="space-y-4">
                                <div className="grid grid-cols-2 gap-4">
                                    <div className="p-3 rounded-lg border bg-card">
                                        <div className="text-xs text-muted-foreground mb-1">Max Budget</div>
                                        <div className="font-medium">
                                            {apiKey.max_budget ? `$${apiKey.max_budget.toFixed(2)}` : "Unlimited"}
                                        </div>
                                    </div>
                                    <div className="p-3 rounded-lg border bg-card">
                                        <div className="text-xs text-muted-foreground mb-1">Spend</div>
                                        <div className="font-medium">${apiKey.spent_budget.toFixed(2)}</div>
                                    </div>
                                </div>
                                {apiKey.max_budget && (
                                    <BudgetProgress spent={apiKey.spent_budget} max={apiKey.max_budget} />
                                )}
                            </div>
                        )}
                    </div>

                    {/* Actions */}
                    <div className="space-y-3 pt-6 border-t">
                        <h4 className="text-sm font-medium text-muted-foreground">Danger Zone</h4>
                        <div className="flex flex-col gap-2">
                            {apiKey.blocked ? (
                                <Button variant="outline" className="justify-start text-green-400 hover:text-green-500" onClick={handleUnblock}>
                                    <Shield className="w-4 h-4 mr-2" />
                                    Unblock API Key
                                </Button>
                            ) : (
                                <Button variant="outline" className="justify-start text-yellow-400 hover:text-yellow-500" onClick={handleBlock}>
                                    <ShieldOff className="w-4 h-4 mr-2" />
                                    Block API Key
                                </Button>
                            )}
                            <Button variant="outline" className="justify-start text-red-400 hover:text-red-500 hover:bg-red-500/10" onClick={handleDelete}>
                                <Trash2 className="w-4 h-4 mr-2" />
                                Delete API Key
                            </Button>
                        </div>
                    </div>
                </div>
            </div>
        </Sheet>
    );
}
