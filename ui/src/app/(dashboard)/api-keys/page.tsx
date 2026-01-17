"use client";

import { useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Checkbox } from "@/components/ui/checkbox";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
import {
    Plus,
    Copy,
    Key,
    MoreVertical,
    Shield,
    ShieldOff,
    RefreshCw,
    Trash2,
    Check,
    AlertCircle,
    Search,
    Filter,
} from "lucide-react";
import { useApiKeys } from "@/hooks/use-api-keys";
import type { APIKey, GenerateKeyRequest } from "@/types/api";
import { ApiKeySheet } from "@/components/api-keys/api-key-sheet";
import { StatusBadge, BudgetProgress } from "@/components/shared/common";
import { useI18n } from "@/i18n/locale-provider";

// Skeleton component for loading state
function KeyRowSkeleton() {
    return (
        <div className="flex items-center justify-between p-4 border-b border-border/50 last:border-0">
            <div className="flex items-center gap-4 flex-1">
                <div className="w-4 h-4 bg-muted animate-pulse rounded" />
                <div className="w-10 h-10 bg-muted animate-pulse rounded-lg" />
                <div className="flex-1">
                    <div className="h-4 w-32 bg-muted animate-pulse rounded mb-2" />
                    <div className="h-3 w-48 bg-muted animate-pulse rounded" />
                </div>
            </div>
            <div className="flex items-center gap-4">
                <div className="h-6 w-16 bg-muted animate-pulse rounded-full" />
                <div className="h-8 w-8 bg-muted animate-pulse rounded" />
            </div>
        </div>
    );
}

// Key row component
function KeyRow({
    apiKey,
    selected,
    onSelect,
    onClick,
    onBlock,
    onUnblock,
    onDelete,
    onRegenerate,
}: {
    apiKey: APIKey;
    selected: boolean;
    onSelect: (checked: boolean) => void;
    onClick: () => void;
    onBlock: (key: string) => void;
    onUnblock: (key: string) => void;
    onDelete: (key: string) => void;
    onRegenerate: (key: string) => void;
}) {
    const [showMenu, setShowMenu] = useState(false);
    const [copied, setCopied] = useState(false);
    const { t } = useI18n();

    const copyPrefix = async (e: React.MouseEvent) => {
        e.stopPropagation();
        await navigator.clipboard.writeText(apiKey.key_prefix);
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
    };

    const formatDate = (dateString: string) => {
        const date = new Date(dateString);
        const now = new Date();
        const diffMs = now.getTime() - date.getTime();
        const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

        if (diffDays === 0) return t("time.today");
        if (diffDays === 1) return t("time.yesterday");
        if (diffDays < 7) return t("time.daysAgo", { days: diffDays });
        if (diffDays < 30) return t("time.weeksAgo", { weeks: Math.floor(diffDays / 7) });
        return date.toLocaleDateString();
    };

    return (
        <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -10 }}
            className={`flex items-center justify-between p-4 border-b border-border/50 last:border-0 hover:bg-secondary/30 transition-colors group cursor-pointer ${selected ? "bg-secondary/20" : ""}`}
            onClick={onClick}
            data-testid={`key-row-${apiKey.id}`}
        >
            <div className="flex items-center gap-4 flex-1">
                <div onClick={(e) => e.stopPropagation()}>
                    <Checkbox
                        checked={selected}
                        onCheckedChange={(checked) => onSelect(checked as boolean)}
                    />
                </div>
                <div className="w-10 h-10 rounded-lg bg-primary/10 flex items-center justify-center">
                    <Key className="w-5 h-5 text-primary" />
                </div>
                <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                        <h3 className="font-medium truncate" data-testid={`key-name-${apiKey.id}`}>
                            {apiKey.name}
                        </h3>
                        <StatusBadge isActive={apiKey.is_active} blocked={apiKey.blocked} />
                    </div>
                    <div className="flex items-center gap-4 mt-1">
                        <button
                            onClick={copyPrefix}
                            className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors font-mono"
                        >
                            {apiKey.key_prefix}...
                            {copied ? (
                                <Check className="w-3 h-3 text-green-400" />
                            ) : (
                                <Copy className="w-3 h-3 opacity-0 group-hover:opacity-100 transition-opacity" />
                            )}
                        </button>
                        <span className="text-xs text-muted-foreground">
                            {t("time.created", { time: formatDate(apiKey.created_at) })}
                        </span>
                        {apiKey.last_used_at && (
                            <span className="text-xs text-muted-foreground">
                                {t("time.lastUsed", { time: formatDate(apiKey.last_used_at) })}
                            </span>
                        )}
                    </div>
                </div>
            </div>

            <div className="flex items-center gap-6">
                <BudgetProgress spent={apiKey.spent_budget} max={apiKey.max_budget} />

                <div className="relative" onClick={(e) => e.stopPropagation()}>
                    <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => setShowMenu(!showMenu)}
                        className="h-8 w-8"
                    >
                        <MoreVertical className="w-4 h-4" />
                    </Button>

                    <AnimatePresence>
                        {showMenu && (
                            <>
                                <div
                                    className="fixed inset-0 z-40"
                                    onClick={() => setShowMenu(false)}
                                />
                                <motion.div
                                    initial={{ opacity: 0, scale: 0.95 }}
                                    animate={{ opacity: 1, scale: 1 }}
                                    exit={{ opacity: 0, scale: 0.95 }}
                                    className="absolute right-0 top-full mt-1 w-48 bg-popover border border-border rounded-lg shadow-lg z-50 py-1"
                                >
                                    {apiKey.blocked ? (
                                        <button
                                            onClick={() => {
                                                onUnblock(apiKey.id);
                                                setShowMenu(false);
                                            }}
                                            className="w-full flex items-center gap-2 px-3 py-2 text-sm hover:bg-secondary transition-colors text-green-400"
                                        >
                                            <Shield className="w-4 h-4" />
                                            {t("dashboard.apiKeys.menu.unblock")}
                                        </button>
                                    ) : (
                                        <button
                                            onClick={() => {
                                                onBlock(apiKey.id);
                                                setShowMenu(false);
                                            }}
                                            className="w-full flex items-center gap-2 px-3 py-2 text-sm hover:bg-secondary transition-colors text-yellow-400"
                                        >
                                            <ShieldOff className="w-4 h-4" />
                                            {t("dashboard.apiKeys.menu.block")}
                                        </button>
                                    )}
                                    <button
                                        onClick={() => {
                                            onRegenerate(apiKey.id);
                                            setShowMenu(false);
                                        }}
                                        className="w-full flex items-center gap-2 px-3 py-2 text-sm hover:bg-secondary transition-colors"
                                    >
                                        <RefreshCw className="w-4 h-4" />
                                        {t("dashboard.apiKeys.menu.regenerate")}
                                    </button>
                                    <div className="my-1 border-t border-border" />
                                    <button
                                        onClick={() => {
                                            onDelete(apiKey.id);
                                            setShowMenu(false);
                                        }}
                                        className="w-full flex items-center gap-2 px-3 py-2 text-sm hover:bg-secondary transition-colors text-red-400"
                                    >
                                        <Trash2 className="w-4 h-4" />
                                        {t("dashboard.apiKeys.menu.delete")}
                                    </button>
                                </motion.div>
                            </>
                        )}
                    </AnimatePresence>
                </div>
            </div>
        </motion.div>
    );
}

// Create key dialog component
function CreateKeyDialog({
    open,
    onOpenChange,
    onCreate,
}: {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    onCreate: (data: GenerateKeyRequest) => Promise<{ key: string }>;
}) {
    const [name, setName] = useState("");
    const [maxBudget, setMaxBudget] = useState("");
    const [isCreating, setIsCreating] = useState(false);
    const [createdKey, setCreatedKey] = useState<string | null>(null);
    const [copied, setCopied] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const { t } = useI18n();

    const handleCreate = async () => {
        if (!name.trim()) {
            setError(t("dashboard.apiKeys.form.validation.nameRequired"));
            return;
        }

        setIsCreating(true);
        setError(null);

        try {
            const result = await onCreate({
                name: name.trim(),
                max_budget: maxBudget ? parseFloat(maxBudget) : undefined,
            });
            setCreatedKey(result.key);
        } catch (err) {
            setError(err instanceof Error ? err.message : t("dashboard.apiKeys.form.error.createFailed"));
        } finally {
            setIsCreating(false);
        }
    };

    const copyKey = async () => {
        if (createdKey) {
            await navigator.clipboard.writeText(createdKey);
            setCopied(true);
            setTimeout(() => setCopied(false), 2000);
        }
    };

    const handleClose = () => {
        setName("");
        setMaxBudget("");
        setCreatedKey(null);
        setError(null);
        onOpenChange(false);
    };

    return (
        <Dialog open={open} onOpenChange={handleClose}>
            <DialogContent className="sm:max-w-md">
                <DialogHeader>
                    <DialogTitle>
                        {createdKey ? t("dashboard.apiKeys.dialog.created.title") : t("dashboard.apiKeys.dialog.create.title")}
                    </DialogTitle>
                    <DialogDescription>
                        {createdKey
                            ? t("dashboard.apiKeys.dialog.created.description")
                            : t("dashboard.apiKeys.dialog.create.description")}
                    </DialogDescription>
                </DialogHeader>

                {createdKey ? (
                    <div className="space-y-4">
                        <div className="p-4 bg-secondary/50 rounded-lg">
                            <div className="flex items-center justify-between gap-2">
                                <code className="flex-1 text-sm font-mono break-all text-green-400">
                                    {createdKey}
                                </code>
                                <Button variant="ghost" size="icon" onClick={copyKey}>
                                    {copied ? (
                                        <Check className="w-4 h-4 text-green-400" />
                                    ) : (
                                        <Copy className="w-4 h-4" />
                                    )}
                                </Button>
                            </div>
                        </div>
                        <div className="flex items-start gap-2 p-3 bg-yellow-500/10 border border-yellow-500/20 rounded-lg">
                            <AlertCircle className="w-5 h-5 text-yellow-500 flex-shrink-0 mt-0.5" />
                            <p className="text-sm text-yellow-200">
                                {t("dashboard.apiKeys.dialog.created.warning")}
                            </p>
                        </div>
                    </div>
                ) : (
                    <div className="space-y-4">
                        <div className="space-y-2">
                            <Label htmlFor="name">{t("dashboard.apiKeys.form.name.label")}</Label>
                            <Input
                                id="name"
                                placeholder={t("dashboard.apiKeys.form.name.placeholder")}
                                value={name}
                                onChange={(e) => setName(e.target.value)}
                                data-testid="key-name-input"
                            />
                        </div>
                        <div className="space-y-2">
                            <Label htmlFor="budget">{t("dashboard.apiKeys.form.maxBudget.label")}</Label>
                            <div className="relative">
                                <span className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground">
                                    $
                                </span>
                                <Input
                                    id="budget"
                                    type="number"
                                    placeholder={t("dashboard.apiKeys.form.maxBudget.placeholder")}
                                    value={maxBudget}
                                    onChange={(e) => setMaxBudget(e.target.value)}
                                    className="pl-7"
                                    data-testid="key-budget-input"
                                />
                            </div>
                            <p className="text-xs text-muted-foreground">
                                {t("dashboard.apiKeys.form.maxBudget.hint")}
                            </p>
                        </div>
                        {error && (
                            <div className="flex items-center gap-2 p-3 bg-red-500/10 border border-red-500/20 rounded-lg">
                                <AlertCircle className="w-4 h-4 text-red-400" />
                                <p className="text-sm text-red-400">{error}</p>
                            </div>
                        )}
                    </div>
                )}

                <DialogFooter>
                    {createdKey ? (
                        <Button onClick={handleClose} className="w-full">
                            {t("dashboard.apiKeys.form.submit.done")}
                        </Button>
                    ) : (
                        <>
                            <Button variant="ghost" onClick={handleClose}>
                                {t("dashboard.apiKeys.form.submit.cancel")}
                            </Button>
                            <Button
                                onClick={handleCreate}
                                disabled={isCreating}
                                data-testid="create-key-submit"
                            >
                                {isCreating ? (
                                    <>
                                        <RefreshCw className="w-4 h-4 mr-2 animate-spin" />
                                        {t("dashboard.apiKeys.form.submit.creating")}
                                    </>
                                ) : (
                                    t("dashboard.apiKeys.form.submit.create")
                                )}
                            </Button>
                        </>
                    )}
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}

export default function ApiKeysPage() {
    const { t } = useI18n();
    const [createDialogOpen, setCreateDialogOpen] = useState(false);
    const [searchQuery, setSearchQuery] = useState("");
    const [selectedKeys, setSelectedKeys] = useState<Set<string>>(new Set());
    const [selectedKeyForDetails, setSelectedKeyForDetails] = useState<APIKey | null>(null);
    const [filterType, setFilterType] = useState<"all" | "active" | "blocked">("all");

    const {
        keys,
        total,
        isLoading,
        error,
        refresh,
        createKey,
        deleteKey,
        blockKey,
        unblockKey,
        regenerateKey,
    } = useApiKeys();

    // Filter keys
    const filteredKeys = keys.filter((key) => {
        const matchesSearch =
            key.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
            key.key_prefix.toLowerCase().includes(searchQuery.toLowerCase());

        const matchesFilter =
            filterType === "all" ||
            (filterType === "active" && key.is_active && !key.blocked) ||
            (filterType === "blocked" && key.blocked);

        return matchesSearch && matchesFilter;
    });

    const handleCreateKey = async (data: GenerateKeyRequest) => {
        const result = await createKey(data);
        return { key: result.key };
    };

    const handleSelectAll = (checked: boolean) => {
        if (checked) {
            setSelectedKeys(new Set(filteredKeys.map((k) => k.id)));
        } else {
            setSelectedKeys(new Set());
        }
    };

    const handleSelectKey = (keyId: string, checked: boolean) => {
        const newSelected = new Set(selectedKeys);
        if (checked) {
            newSelected.add(keyId);
        } else {
            newSelected.delete(keyId);
        }
        setSelectedKeys(newSelected);
    };

    const handleBatchBlock = async () => {
        if (confirm(t("dashboard.apiKeys.confirm.blockKeys", { count: selectedKeys.size }))) {
            await Promise.all(Array.from(selectedKeys).map((id) => blockKey(id)));
            setSelectedKeys(new Set());
        }
    };

    const handleBatchDelete = async () => {
        if (confirm(t("dashboard.apiKeys.confirm.deleteKeys", { count: selectedKeys.size }))) {
            await Promise.all(Array.from(selectedKeys).map((id) => deleteKey(id)));
            setSelectedKeys(new Set());
        }
    };

    return (
        <div className="space-y-6">
            {/* Header */}
            <motion.div
                initial={{ opacity: 0, y: -20 }}
                animate={{ opacity: 1, y: 0 }}
                className="flex items-center justify-between"
            >
                <div>
                    <h1 className="text-3xl font-bold tracking-tight">{t("dashboard.apiKeys.title")}</h1>
                    <p className="text-muted-foreground">
                        {t("dashboard.apiKeys.description")}
                    </p>
                </div>
                <Button
                    className="gap-2"
                    onClick={() => setCreateDialogOpen(true)}
                    data-testid="create-key-button"
                >
                    <Plus className="w-4 h-4" />
                    {t("dashboard.apiKeys.actions.createNew")}
                </Button>
            </motion.div>

            {/* Search and Filters */}
            <motion.div
                initial={{ opacity: 0, y: -10 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ delay: 0.1 }}
                className="flex flex-col sm:flex-row items-start sm:items-center gap-4"
            >
                <div className="relative flex-1 max-w-sm">
                    <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
                    <Input
                        placeholder={t("dashboard.apiKeys.search.placeholder")}
                        value={searchQuery}
                        onChange={(e) => setSearchQuery(e.target.value)}
                        className="pl-9"
                        data-testid="search-input"
                    />
                </div>
                <div className="flex items-center gap-2">
                    <Select value={filterType} onValueChange={(v: any) => setFilterType(v)}>
                        <SelectTrigger className="w-32">
                            <Filter className="w-4 h-4 mr-2 text-muted-foreground" />
                            <SelectValue placeholder={t("dashboard.apiKeys.filter.placeholder")} />
                        </SelectTrigger>
                        <SelectContent>
                            <SelectItem value="all">{t("dashboard.apiKeys.filter.all")}</SelectItem>
                            <SelectItem value="active">{t("dashboard.apiKeys.filter.active")}</SelectItem>
                            <SelectItem value="blocked">{t("dashboard.apiKeys.filter.blocked")}</SelectItem>
                        </SelectContent>
                    </Select>
                    <Button variant="ghost" size="icon" onClick={refresh} title={t("common.refresh")}>
                        <RefreshCw className="w-4 h-4" />
                    </Button>
                </div>
            </motion.div>

            {/* Batch Actions */}
            <AnimatePresence>
                {selectedKeys.size > 0 && (
                    <motion.div
                        initial={{ opacity: 0, height: 0 }}
                        animate={{ opacity: 1, height: "auto" }}
                        exit={{ opacity: 0, height: 0 }}
                        className="flex items-center gap-4 p-2 bg-secondary/30 rounded-lg border border-border/50"
                    >
                        <span className="text-sm font-medium px-2">
                            {t("dashboard.apiKeys.actions.batchSelected", { count: selectedKeys.size })}
                        </span>
                        <div className="h-4 w-px bg-border" />
                        <Button
                            variant="ghost"
                            size="sm"
                            onClick={handleBatchBlock}
                            className="text-yellow-400 hover:text-yellow-500"
                        >
                            <ShieldOff className="w-4 h-4 mr-2" />
                            {t("dashboard.apiKeys.actions.blockSelected")}
                        </Button>
                        <Button
                            variant="ghost"
                            size="sm"
                            onClick={handleBatchDelete}
                            className="text-red-400 hover:text-red-500"
                        >
                            <Trash2 className="w-4 h-4 mr-2" />
                            {t("dashboard.apiKeys.actions.deleteSelected")}
                        </Button>
                    </motion.div>
                )}
            </AnimatePresence>

            {/* Error State */}
            {error && (
                <motion.div
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    className="flex items-center gap-2 p-4 bg-red-500/10 border border-red-500/20 rounded-lg"
                    data-testid="error-message"
                >
                    <AlertCircle className="w-5 h-5 text-red-400" />
                    <p className="text-red-400">{t("dashboard.apiKeys.error.loadFailed", { error: error.message })}</p>
                    <Button variant="ghost" size="sm" onClick={refresh} className="ml-auto">
                        {t("common.retry")}
                    </Button>
                </motion.div>
            )}

            {/* Keys Card */}
            <motion.div
                initial={{ opacity: 0, y: 20 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ delay: 0.2 }}
            >
                <Card className="glass-card">
                    <CardHeader className="flex flex-row items-center justify-between pb-2 border-b border-border/50">
                        <div className="flex items-center gap-4">
                            <Checkbox
                                checked={filteredKeys.length > 0 && selectedKeys.size === filteredKeys.length}
                                onCheckedChange={handleSelectAll}
                            />
                            <CardTitle>
                                {t("dashboard.apiKeys.list.title")}{" "}
                                {!isLoading && (
                                    <span className="text-muted-foreground font-normal">({total})</span>
                                )}
                            </CardTitle>
                        </div>
                    </CardHeader>
                    <CardContent className="p-0">
                        {isLoading ? (
                            <div data-testid="loading-skeleton">
                                {[1, 2, 3].map((i) => (
                                    <KeyRowSkeleton key={i} />
                                ))}
                            </div>
                        ) : filteredKeys.length === 0 ? (
                            <div
                                className="text-sm text-muted-foreground py-12 text-center"
                                data-testid="empty-state"
                            >
                                {searchQuery ? (
                                    <>{t("dashboard.apiKeys.empty.noMatch", { query: searchQuery })}</>
                                ) : (
                                    <>{t("dashboard.apiKeys.empty.noKeys")}</>
                                )}
                            </div>
                        ) : (
                            <AnimatePresence mode="popLayout">
                                {filteredKeys.map((key) => (
                                    <KeyRow
                                        key={key.id}
                                        apiKey={key}
                                        selected={selectedKeys.has(key.id)}
                                        onSelect={(checked) => handleSelectKey(key.id, checked)}
                                        onClick={() => setSelectedKeyForDetails(key)}
                                        onBlock={blockKey}
                                        onUnblock={unblockKey}
                                        onDelete={deleteKey}
                                        onRegenerate={regenerateKey}
                                    />
                                ))}
                            </AnimatePresence>
                        )}
                    </CardContent>
                </Card>
            </motion.div>

            {/* Create Key Dialog */}
            <CreateKeyDialog
                open={createDialogOpen}
                onOpenChange={setCreateDialogOpen}
                onCreate={handleCreateKey}
            />

            {/* Key Details Sheet */}
            <ApiKeySheet
                apiKey={selectedKeyForDetails}
                open={!!selectedKeyForDetails}
                onOpenChange={(open) => !open && setSelectedKeyForDetails(null)}
                onUpdate={refresh}
            />
        </div>
    );
}
