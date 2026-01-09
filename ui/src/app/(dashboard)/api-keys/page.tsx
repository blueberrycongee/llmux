"use client";

import { useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from "@/components/ui/dialog";
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
} from "lucide-react";
import { useApiKeys } from "@/hooks/use-api-keys";
import type { APIKey, GenerateKeyRequest } from "@/types/api";

// Skeleton component for loading state
function KeyRowSkeleton() {
    return (
        <div className="flex items-center justify-between p-4 border-b border-border/50 last:border-0">
            <div className="flex items-center gap-4 flex-1">
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

// Status badge component
function StatusBadge({ isActive, blocked }: { isActive: boolean; blocked: boolean }) {
    if (blocked) {
        return (
            <span className="inline-flex items-center gap-1 px-2 py-1 text-xs font-medium rounded-full bg-red-500/10 text-red-400">
                <ShieldOff className="w-3 h-3" />
                Blocked
            </span>
        );
    }
    if (isActive) {
        return (
            <span className="inline-flex items-center gap-1 px-2 py-1 text-xs font-medium rounded-full bg-green-500/10 text-green-400">
                <Shield className="w-3 h-3" />
                Active
            </span>
        );
    }
    return (
        <span className="inline-flex items-center gap-1 px-2 py-1 text-xs font-medium rounded-full bg-gray-500/10 text-gray-400">
            Inactive
        </span>
    );
}

// Budget progress component
function BudgetProgress({ spent, max }: { spent: number; max?: number }) {
    if (!max) {
        return <span className="text-sm text-muted-foreground">No limit</span>;
    }
    const percentage = Math.min((spent / max) * 100, 100);
    const isNearLimit = percentage >= 80;
    const isOverLimit = percentage >= 100;

    return (
        <div className="w-32">
            <div className="flex items-center justify-between text-xs mb-1">
                <span className={isOverLimit ? "text-red-400" : isNearLimit ? "text-yellow-400" : "text-muted-foreground"}>
                    ${spent.toFixed(2)}
                </span>
                <span className="text-muted-foreground">${max.toFixed(2)}</span>
            </div>
            <div className="h-1.5 bg-secondary rounded-full overflow-hidden">
                <div
                    className={`h-full rounded-full transition-all ${isOverLimit ? "bg-red-500" : isNearLimit ? "bg-yellow-500" : "bg-primary"
                        }`}
                    style={{ width: `${percentage}%` }}
                />
            </div>
        </div>
    );
}

// Key row component
function KeyRow({
    apiKey,
    onBlock,
    onUnblock,
    onDelete,
    onRegenerate,
}: {
    apiKey: APIKey;
    onBlock: (key: string) => void;
    onUnblock: (key: string) => void;
    onDelete: (key: string) => void;
    onRegenerate: (key: string) => void;
}) {
    const [showMenu, setShowMenu] = useState(false);
    const [copied, setCopied] = useState(false);

    const copyPrefix = async () => {
        await navigator.clipboard.writeText(apiKey.key_prefix);
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
    };

    const formatDate = (dateString: string) => {
        const date = new Date(dateString);
        const now = new Date();
        const diffMs = now.getTime() - date.getTime();
        const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

        if (diffDays === 0) return "Today";
        if (diffDays === 1) return "Yesterday";
        if (diffDays < 7) return `${diffDays} days ago`;
        if (diffDays < 30) return `${Math.floor(diffDays / 7)} weeks ago`;
        return date.toLocaleDateString();
    };

    return (
        <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -10 }}
            className="flex items-center justify-between p-4 border-b border-border/50 last:border-0 hover:bg-secondary/30 transition-colors group"
            data-testid={`key-row-${apiKey.id}`}
        >
            <div className="flex items-center gap-4 flex-1">
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
                            Created {formatDate(apiKey.created_at)}
                        </span>
                        {apiKey.last_used_at && (
                            <span className="text-xs text-muted-foreground">
                                Last used {formatDate(apiKey.last_used_at)}
                            </span>
                        )}
                    </div>
                </div>
            </div>

            <div className="flex items-center gap-6">
                <BudgetProgress spent={apiKey.spent_budget} max={apiKey.max_budget} />

                <div className="relative">
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
                                            Unblock Key
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
                                            Block Key
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
                                        Regenerate Key
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
                                        Delete Key
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

    const handleCreate = async () => {
        if (!name.trim()) {
            setError("Name is required");
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
            setError(err instanceof Error ? err.message : "Failed to create key");
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
                        {createdKey ? "API Key Created!" : "Create New API Key"}
                    </DialogTitle>
                    <DialogDescription>
                        {createdKey
                            ? "Make sure to copy your API key now. You won't be able to see it again!"
                            : "Create a new API key to access LLM models."}
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
                                This is the only time you will see this key. Store it securely.
                            </p>
                        </div>
                    </div>
                ) : (
                    <div className="space-y-4">
                        <div className="space-y-2">
                            <Label htmlFor="name">Key Name</Label>
                            <Input
                                id="name"
                                placeholder="e.g., Production API Key"
                                value={name}
                                onChange={(e) => setName(e.target.value)}
                                data-testid="key-name-input"
                            />
                        </div>
                        <div className="space-y-2">
                            <Label htmlFor="budget">Max Budget (Optional)</Label>
                            <div className="relative">
                                <span className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground">
                                    $
                                </span>
                                <Input
                                    id="budget"
                                    type="number"
                                    placeholder="100.00"
                                    value={maxBudget}
                                    onChange={(e) => setMaxBudget(e.target.value)}
                                    className="pl-7"
                                    data-testid="key-budget-input"
                                />
                            </div>
                            <p className="text-xs text-muted-foreground">
                                Leave empty for unlimited budget
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
                            Done
                        </Button>
                    ) : (
                        <>
                            <Button variant="ghost" onClick={handleClose}>
                                Cancel
                            </Button>
                            <Button
                                onClick={handleCreate}
                                disabled={isCreating}
                                data-testid="create-key-submit"
                            >
                                {isCreating ? (
                                    <>
                                        <RefreshCw className="w-4 h-4 mr-2 animate-spin" />
                                        Creating...
                                    </>
                                ) : (
                                    "Create Key"
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
    const [createDialogOpen, setCreateDialogOpen] = useState(false);
    const [searchQuery, setSearchQuery] = useState("");

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

    // Filter keys by search query
    const filteredKeys = keys.filter(
        (key) =>
            key.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
            key.key_prefix.toLowerCase().includes(searchQuery.toLowerCase())
    );

    const handleCreateKey = async (data: GenerateKeyRequest) => {
        const result = await createKey(data);
        return { key: result.key };
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
                    <h1 className="text-3xl font-bold tracking-tight">API Keys</h1>
                    <p className="text-muted-foreground">
                        Manage your API keys for accessing LLM models.
                    </p>
                </div>
                <Button
                    className="gap-2"
                    onClick={() => setCreateDialogOpen(true)}
                    data-testid="create-key-button"
                >
                    <Plus className="w-4 h-4" />
                    Create New Key
                </Button>
            </motion.div>

            {/* Search and Filters */}
            <motion.div
                initial={{ opacity: 0, y: -10 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ delay: 0.1 }}
                className="flex items-center gap-4"
            >
                <div className="relative flex-1 max-w-sm">
                    <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
                    <Input
                        placeholder="Search keys..."
                        value={searchQuery}
                        onChange={(e) => setSearchQuery(e.target.value)}
                        className="pl-9"
                        data-testid="search-input"
                    />
                </div>
                <Button variant="ghost" size="icon" onClick={refresh} title="Refresh">
                    <RefreshCw className="w-4 h-4" />
                </Button>
            </motion.div>

            {/* Error State */}
            {error && (
                <motion.div
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    className="flex items-center gap-2 p-4 bg-red-500/10 border border-red-500/20 rounded-lg"
                    data-testid="error-message"
                >
                    <AlertCircle className="w-5 h-5 text-red-400" />
                    <p className="text-red-400">Failed to load API keys: {error.message}</p>
                    <Button variant="ghost" size="sm" onClick={refresh} className="ml-auto">
                        Retry
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
                    <CardHeader className="flex flex-row items-center justify-between pb-2">
                        <CardTitle>
                            Active Keys{" "}
                            {!isLoading && (
                                <span className="text-muted-foreground font-normal">({total})</span>
                            )}
                        </CardTitle>
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
                                    <>No keys matching "{searchQuery}"</>
                                ) : (
                                    <>No API keys found. Create one to get started.</>
                                )}
                            </div>
                        ) : (
                            <AnimatePresence mode="popLayout">
                                {filteredKeys.map((key) => (
                                    <KeyRow
                                        key={key.id}
                                        apiKey={key}
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
        </div>
    );
}
