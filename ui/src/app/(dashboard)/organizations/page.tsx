"use client";

import { useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import Link from "next/link";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog";
import {
    Building2,
    Plus,
    Search,
    RefreshCw,
    AlertCircle,
    ChevronRight,
    MoreVertical,
    Trash2,
    DollarSign,
    Users,
    Key,
    Shield,
    ShieldOff,
} from "lucide-react";
import { useOrganizations } from "@/hooks";
import type { Organization, CreateOrganizationRequest } from "@/types/api";
import { BudgetProgress, PageHeader, EmptyState, ErrorState } from "@/components/shared/common";
import { CardSkeleton } from "@/components/ui/skeleton";
import { useI18n } from "@/i18n/locale-provider";

// Organization Card Skeleton
function OrgCardSkeleton() {
    return (
        <Card className="glass-card">
            <CardContent className="p-6">
                <div className="flex items-start justify-between mb-4">
                    <div className="flex items-center gap-3">
                        <div className="w-12 h-12 bg-muted animate-pulse rounded-xl" />
                        <div>
                            <div className="h-5 w-32 bg-muted animate-pulse rounded mb-2" />
                            <div className="h-3 w-20 bg-muted animate-pulse rounded" />
                        </div>
                    </div>
                    <div className="h-6 w-16 bg-muted animate-pulse rounded-full" />
                </div>
                <div className="space-y-3">
                    <div className="grid grid-cols-2 gap-4">
                        <div className="h-16 bg-muted animate-pulse rounded-lg" />
                        <div className="h-16 bg-muted animate-pulse rounded-lg" />
                    </div>
                    <div className="h-4 w-full bg-muted animate-pulse rounded" />
                </div>
            </CardContent>
        </Card>
    );
}

// Create Organization Dialog
function CreateOrganizationDialog({
    open,
    onOpenChange,
    onCreate,
}: {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    onCreate: (data: CreateOrganizationRequest) => Promise<Organization>;
}) {
    const [name, setName] = useState("");
    const [maxBudget, setMaxBudget] = useState("");
    const [isCreating, setIsCreating] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const { t } = useI18n();

    const handleCreate = async () => {
        if (!name.trim()) {
            setError(t("dashboard.organizations.form.name.required"));
            return;
        }

        setIsCreating(true);
        setError(null);

        try {
            await onCreate({
                organization_alias: name.trim(),
                max_budget: maxBudget ? parseFloat(maxBudget) : undefined,
            });
            handleClose();
        } catch (err) {
            setError(err instanceof Error ? err.message : t("dashboard.organizations.form.error.createFailed"));
        } finally {
            setIsCreating(false);
        }
    };

    const handleClose = () => {
        setName("");
        setMaxBudget("");
        setError(null);
        onOpenChange(false);
    };

    return (
        <Dialog open={open} onOpenChange={handleClose}>
            <DialogContent className="sm:max-w-md">
                <DialogHeader>
                    <DialogTitle>{t("dashboard.organizations.dialog.create.title")}</DialogTitle>
                    <DialogDescription>
                        {t("dashboard.organizations.dialog.create.description")}
                    </DialogDescription>
                </DialogHeader>

                <div className="space-y-4">
                    <div className="space-y-2">
                        <Label htmlFor="name">{t("dashboard.organizations.form.name.label")}</Label>
                        <Input
                            id="name"
                            placeholder={t("dashboard.organizations.form.name.placeholder")}
                            value={name}
                            onChange={(e) => setName(e.target.value)}
                            data-testid="org-name-input"
                        />
                    </div>

                    <div className="space-y-2">
                        <Label htmlFor="budget">{t("dashboard.organizations.form.maxBudget.label")}</Label>
                        <div className="relative">
                            <span className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground">$</span>
                            <Input
                                id="budget"
                                type="number"
                                placeholder={t("dashboard.organizations.form.maxBudget.placeholder")}
                                value={maxBudget}
                                onChange={(e) => setMaxBudget(e.target.value)}
                                className="pl-7"
                            />
                        </div>
                    </div>

                    {error && (
                        <div className="flex items-center gap-2 p-3 bg-red-500/10 border border-red-500/20 rounded-lg">
                            <AlertCircle className="w-4 h-4 text-red-400" />
                            <p className="text-sm text-red-400">{error}</p>
                        </div>
                    )}
                </div>

                <DialogFooter>
                    <Button variant="ghost" onClick={handleClose}>
                        {t("common.cancel")}
                    </Button>
                    <Button onClick={handleCreate} disabled={isCreating} data-testid="create-org-submit">
                        {isCreating ? (
                            <>
                                <RefreshCw className="w-4 h-4 mr-2 animate-spin" />
                                {t("dashboard.organizations.form.submit.creating")}
                            </>
                        ) : (
                            t("dashboard.organizations.form.submit.create")
                        )}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}

// Organization Card
function OrganizationCard({
    org,
    onDelete,
}: {
    org: Organization;
    onDelete: (orgId: string) => void;
}) {
    const [showMenu, setShowMenu] = useState(false);
    const { t } = useI18n();

    return (
        <motion.div
            initial={{ opacity: 0, y: 20, scale: 0.95 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: -20, scale: 0.95 }}
            data-testid={`org-card-${org.organization_id}`}
        >
            <Card className="glass-card group hover:shadow-lg transition-all duration-300">
                <CardContent className="p-6">
                    {/* Header */}
                    <div className="flex items-start justify-between mb-4">
                        <div className="flex items-center gap-3">
                            <div className="w-12 h-12 rounded-xl bg-gradient-to-br from-blue-500/20 to-purple-500/20 flex items-center justify-center border border-blue-500/20">
                                <Building2 className="w-6 h-6 text-blue-400" />
                            </div>
                            <div>
                                <h3 className="font-semibold text-lg" data-testid={`org-name-${org.organization_id}`}>
                                    {org.organization_alias}
                                </h3>
                                <p className="text-xs text-muted-foreground font-mono">
                                    {org.organization_id.slice(0, 16)}...
                                </p>
                            </div>
                        </div>

                        <div className="flex items-center gap-2">
                            <Badge variant="success" className="gap-1">
                                <Shield className="w-3 h-3" />
                                {t("dashboard.organizations.card.active")}
                            </Badge>

                            <div className="relative">
                                <Button
                                    variant="ghost"
                                    size="icon"
                                    onClick={() => setShowMenu(!showMenu)}
                                    className="h-8 w-8 opacity-0 group-hover:opacity-100 transition-opacity"
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
                                                className="absolute right-0 top-full mt-1 w-40 bg-popover border border-border rounded-lg shadow-lg z-50 py-1"
                                            >
                                                <button
                                                    onClick={() => {
                                                        onDelete(org.organization_id);
                                                        setShowMenu(false);
                                                    }}
                                                    className="w-full flex items-center gap-2 px-3 py-2 text-sm hover:bg-secondary transition-colors text-red-400"
                                                >
                                                    <Trash2 className="w-4 h-4" />
                                                    {t("common.delete")}
                                                </button>
                                            </motion.div>
                                        </>
                                    )}
                                </AnimatePresence>
                            </div>
                        </div>
                    </div>

                    {/* Stats Grid */}
                    <div className="grid grid-cols-2 gap-3 mb-4">
                        <div className="flex items-center gap-2 p-3 rounded-lg bg-secondary/50">
                            <DollarSign className="w-4 h-4 text-green-400" />
                            <div>
                                <div className="text-xs text-muted-foreground">{t("dashboard.organizations.card.spend")}</div>
                                <div className="font-semibold">${org.spend.toFixed(2)}</div>
                            </div>
                        </div>
                        <div className="flex items-center gap-2 p-3 rounded-lg bg-secondary/50">
                            <Key className="w-4 h-4 text-purple-400" />
                            <div>
                                <div className="text-xs text-muted-foreground">{t("dashboard.organizations.card.models")}</div>
                                <div className="font-semibold">{org.models?.length || t("dashboard.organizations.card.modelsAll")}</div>
                            </div>
                        </div>
                    </div>

                    {/* Budget Progress */}
                    <BudgetProgress spent={org.spend} max={org.max_budget} />

                    {/* View Details Link */}
                    <Link
                        href={`/organizations/${org.organization_id}`}
                        className="mt-4 flex items-center justify-between p-3 -mx-3 rounded-lg hover:bg-secondary/50 transition-colors group/link"
                    >
                        <span className="text-sm font-medium text-muted-foreground group-hover/link:text-foreground">
                            {t("common.viewDetails")}
                        </span>
                        <ChevronRight className="w-4 h-4 text-muted-foreground group-hover/link:text-foreground group-hover/link:translate-x-1 transition-all" />
                    </Link>
                </CardContent>
            </Card>
        </motion.div>
    );
}

export default function OrganizationsPage() {
    const { t } = useI18n();
    const [createDialogOpen, setCreateDialogOpen] = useState(false);
    const [searchQuery, setSearchQuery] = useState("");

    const {
        organizations,
        total,
        isLoading,
        error,
        refresh,
        createOrganization,
        deleteOrganization,
    } = useOrganizations();

    // Filter organizations by search query
    const filteredOrgs = organizations.filter(
        (org) =>
            org.organization_alias.toLowerCase().includes(searchQuery.toLowerCase()) ||
            org.organization_id.toLowerCase().includes(searchQuery.toLowerCase())
    );

    return (
        <div className="space-y-6">
            {/* Header */}
            <PageHeader
                title={t("dashboard.organizations.title")}
                description={t("dashboard.organizations.description")}
                action={
                    <Button
                        className="gap-2"
                        onClick={() => setCreateDialogOpen(true)}
                        data-testid="create-org-button"
                    >
                        <Plus className="w-4 h-4" />
                        {t("dashboard.organizations.action.new")}
                    </Button>
                }
            />

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
                        placeholder={t("dashboard.organizations.search.placeholder")}
                        value={searchQuery}
                        onChange={(e) => setSearchQuery(e.target.value)}
                        className="pl-9"
                        data-testid="search-input"
                    />
                </div>
                <Button variant="ghost" size="icon" onClick={refresh} title={t("common.refresh")}>
                    <RefreshCw className="w-4 h-4" />
                </Button>
            </motion.div>

            {/* Error State */}
            {error && (
                <ErrorState message={error.message} onRetry={refresh} />
            )}

            {/* Organizations Grid */}
            {isLoading ? (
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4" data-testid="loading-skeleton">
                    {[1, 2, 3, 4, 5, 6].map((i) => (
                        <OrgCardSkeleton key={i} />
                    ))}
                </div>
            ) : filteredOrgs.length === 0 ? (
                <motion.div
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    data-testid="empty-state"
                >
                    <Card className="glass-card">
                        <EmptyState
                            icon={<Building2 className="w-12 h-12" />}
                            title={
                                searchQuery
                                    ? t("dashboard.organizations.empty.noMatch", { query: searchQuery })
                                    : t("dashboard.organizations.empty.none")
                            }
                            description={
                                searchQuery
                                    ? t("dashboard.organizations.empty.tryAdjust")
                                    : t("dashboard.organizations.empty.createFirst")
                            }
                            action={
                                !searchQuery && (
                                    <Button onClick={() => setCreateDialogOpen(true)}>
                                        <Plus className="w-4 h-4 mr-2" />
                                        {t("dashboard.organizations.action.new")}
                                    </Button>
                                )
                            }
                        />
                    </Card>
                </motion.div>
            ) : (
                <motion.div
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4"
                >
                    <AnimatePresence mode="popLayout">
                        {filteredOrgs.map((org) => (
                            <OrganizationCard
                                key={org.organization_id}
                                org={org}
                                onDelete={deleteOrganization}
                            />
                        ))}
                    </AnimatePresence>
                </motion.div>
            )}

            {/* Pagination Info */}
            {!isLoading && filteredOrgs.length > 0 && (
                <div className="text-sm text-muted-foreground">
                    {t("pagination.showingCountOfTotal", {
                        count: filteredOrgs.length,
                        total,
                        item: t("dashboard.organizations.title"),
                    })}
                </div>
            )}

            {/* Create Organization Dialog */}
            <CreateOrganizationDialog
                open={createDialogOpen}
                onOpenChange={setCreateDialogOpen}
                onCreate={createOrganization}
            />
        </div>
    );
}
