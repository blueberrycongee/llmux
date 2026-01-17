"use client";

import { useState } from "react";
import { useParams } from "next/navigation";
import { motion, AnimatePresence } from "framer-motion";
import Link from "next/link";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog";
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table";
import {
    ArrowLeft,
    Users,
    Settings,
    DollarSign,
    Zap,
    Key,
    Plus,
    Trash2,
    Edit,
    RefreshCw,
    AlertCircle,
    UserPlus,
    Shield,
    ShieldOff,
    Clock,
} from "lucide-react";
import { useTeamInfo, useTeamMembers, useUsers } from "@/hooks";
import { apiClient } from "@/lib/api";
import { StatusBadge, BudgetProgress, EmptyState, ErrorState } from "@/components/shared/common";
import { Skeleton, CardSkeleton } from "@/components/ui/skeleton";
import type { Team, CreateTeamRequest } from "@/types/api";
import { useI18n } from "@/i18n/locale-provider";

// Team Detail Header Component
function TeamDetailHeader({
    team,
    onEdit,
    onBlock,
    onUnblock,
}: {
    team: Team;
    onEdit: () => void;
    onBlock: () => void;
    onUnblock: () => void;
}) {
    const { t } = useI18n();
    return (
        <div className="flex items-start justify-between">
            <div className="flex items-center gap-4">
                <Link href="/teams">
                    <Button variant="ghost" size="icon" className="h-10 w-10">
                        <ArrowLeft className="w-5 h-5" />
                    </Button>
                </Link>
                <div className="w-16 h-16 rounded-xl bg-gradient-to-br from-primary/20 to-primary/5 flex items-center justify-center border border-primary/20">
                    <Users className="w-8 h-8 text-primary" />
                </div>
                <div>
                    <div className="flex items-center gap-3">
                        <h1 className="text-2xl font-bold tracking-tight">
                            {team.team_alias || team.team_id}
                        </h1>
                        <StatusBadge isActive={team.is_active} blocked={team.blocked} />
                    </div>
                    <p className="text-sm text-muted-foreground font-mono">{team.team_id}</p>
                    {team.organization_id && (
                        <p className="text-xs text-muted-foreground mt-1">
                            {t("dashboard.teamDetail.header.organization", { orgId: team.organization_id })}
                        </p>
                    )}
                </div>
            </div>
            <div className="flex items-center gap-2">
                {team.blocked ? (
                    <Button variant="outline" onClick={onUnblock} className="gap-2">
                        <Shield className="w-4 h-4" />
                        {t("dashboard.teamDetail.action.unblock")}
                    </Button>
                ) : (
                    <Button variant="outline" onClick={onBlock} className="gap-2 text-yellow-500 hover:text-yellow-400">
                        <ShieldOff className="w-4 h-4" />
                        {t("dashboard.teamDetail.action.block")}
                    </Button>
                )}
                <Button onClick={onEdit} className="gap-2">
                    <Edit className="w-4 h-4" />
                    {t("dashboard.teamDetail.action.edit")}
                </Button>
            </div>
        </div>
    );
}

// Stats Cards Component
function TeamStatsCards({ team }: { team: Team }) {
    const { t } = useI18n();
    const stats = [
        {
            label: t("dashboard.teamDetail.stats.members"),
            value: team.members?.length || 0,
            icon: Users,
            color: "text-blue-400",
            bgColor: "bg-blue-500/10",
        },
        {
            label: t("dashboard.teamDetail.stats.budgetUsed"),
            value: `$${team.spend.toFixed(2)}`,
            subtitle: team.max_budget
                ? t("dashboard.teamDetail.stats.ofBudget", { amount: team.max_budget.toFixed(2) })
                : t("dashboard.teamDetail.stats.noLimit"),
            icon: DollarSign,
            color: "text-green-400",
            bgColor: "bg-green-500/10",
        },
        {
            label: t("dashboard.teamDetail.stats.rateLimit"),
            value: team.rpm_limit ? `${team.rpm_limit} RPM` : t("dashboard.teamDetail.stats.rateUnlimited"),
            subtitle: team.tpm_limit ? `${team.tpm_limit.toLocaleString()} TPM` : undefined,
            icon: Zap,
            color: "text-yellow-400",
            bgColor: "bg-yellow-500/10",
        },
        {
            label: t("dashboard.teamDetail.stats.models"),
            value: team.models?.length || t("dashboard.teamDetail.stats.modelsAll"),
            subtitle: team.models?.length
                ? `${team.models.slice(0, 2).join(", ")}...`
                : t("dashboard.teamDetail.stats.modelsAllAllowed"),
            icon: Key,
            color: "text-purple-400",
            bgColor: "bg-purple-500/10",
        },
    ];

    return (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
            {stats.map((stat, i) => (
                <motion.div
                    key={stat.label}
                    initial={{ opacity: 0, y: 20 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ delay: i * 0.1 }}
                >
                    <Card className="glass-card hover:shadow-lg transition-all duration-300">
                        <CardContent className="p-5">
                            <div className="flex items-center justify-between mb-3">
                                <span className="text-sm text-muted-foreground">{stat.label}</span>
                                <div className={`w-9 h-9 rounded-lg ${stat.bgColor} flex items-center justify-center`}>
                                    <stat.icon className={`w-5 h-5 ${stat.color}`} />
                                </div>
                            </div>
                            <div className="text-2xl font-bold tracking-tight">{stat.value}</div>
                            {stat.subtitle && (
                                <p className="text-xs text-muted-foreground mt-1 truncate">{stat.subtitle}</p>
                            )}
                        </CardContent>
                    </Card>
                </motion.div>
            ))}
        </div>
    );
}

// Add Member Dialog
function AddMemberDialog({
    open,
    onOpenChange,
    onAdd,
    existingMembers,
}: {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    onAdd: (userId: string) => Promise<void>;
    existingMembers: string[];
}) {
    const [selectedUserId, setSelectedUserId] = useState("");
    const [isAdding, setIsAdding] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [searchQuery, setSearchQuery] = useState("");
    const { users, isLoading } = useUsers({});
    const { t } = useI18n();

    // Filter out existing members
    const availableUsers = users.filter(
        (u) => !existingMembers.includes(u.user_id) &&
            (u.user_id.toLowerCase().includes(searchQuery.toLowerCase()) ||
                (u.user_alias?.toLowerCase() || "").includes(searchQuery.toLowerCase()) ||
                (u.user_email?.toLowerCase() || "").includes(searchQuery.toLowerCase()))
    );

    const handleAdd = async () => {
        if (!selectedUserId) {
            setError(t("dashboard.teamDetail.dialog.addMember.validation.selectUser"));
            return;
        }

        setIsAdding(true);
        setError(null);

        try {
            await onAdd(selectedUserId);
            handleClose();
        } catch (err) {
            setError(err instanceof Error ? err.message : t("dashboard.teamDetail.dialog.addMember.error.addFailed"));
        } finally {
            setIsAdding(false);
        }
    };

    const handleClose = () => {
        setSelectedUserId("");
        setSearchQuery("");
        setError(null);
        onOpenChange(false);
    };

    return (
        <Dialog open={open} onOpenChange={handleClose}>
            <DialogContent className="sm:max-w-lg">
                <DialogHeader>
                    <DialogTitle>{t("dashboard.teamDetail.dialog.addMember.title")}</DialogTitle>
                    <DialogDescription>
                        {t("dashboard.teamDetail.dialog.addMember.description")}
                    </DialogDescription>
                </DialogHeader>

                <div className="space-y-4">
                    <div className="space-y-2">
                        <Label>{t("dashboard.teamDetail.dialog.addMember.searchLabel")}</Label>
                        <Input
                            placeholder={t("dashboard.teamDetail.dialog.addMember.searchPlaceholder")}
                            value={searchQuery}
                            onChange={(e) => setSearchQuery(e.target.value)}
                        />
                    </div>

                    <div className="max-h-64 overflow-y-auto border rounded-lg divide-y">
                        {isLoading ? (
                            <div className="p-4 text-center text-muted-foreground">{t("dashboard.teamDetail.dialog.addMember.loading")}</div>
                        ) : availableUsers.length === 0 ? (
                            <div className="p-4 text-center text-muted-foreground">
                                {searchQuery ? t("dashboard.teamDetail.dialog.addMember.noMatch") : t("dashboard.teamDetail.dialog.addMember.none")}
                            </div>
                        ) : (
                            availableUsers.map((user) => (
                                <button
                                    key={user.user_id}
                                    onClick={() => setSelectedUserId(user.user_id)}
                                    className={`w-full flex items-center gap-3 p-3 text-left hover:bg-secondary transition-colors ${selectedUserId === user.user_id ? "bg-secondary" : ""
                                        }`}
                                >
                                    <div className="w-10 h-10 rounded-full bg-primary/10 flex items-center justify-center shrink-0">
                                        <Users className="w-5 h-5 text-primary" />
                                    </div>
                                    <div className="flex-1 min-w-0">
                                        <div className="font-medium truncate">
                                            {user.user_alias || user.user_id}
                                        </div>
                                        <div className="text-sm text-muted-foreground truncate">
                                            {user.user_email || user.user_id}
                                        </div>
                                    </div>
                                    {selectedUserId === user.user_id && (
                                        <div className="w-2 h-2 rounded-full bg-primary" />
                                    )}
                                </button>
                            ))
                        )}
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
                    <Button
                        onClick={handleAdd}
                        disabled={isAdding || !selectedUserId}
                    >
                        {isAdding ? (
                            <>
                                <RefreshCw className="w-4 h-4 mr-2 animate-spin" />
                                {t("dashboard.teamDetail.dialog.addMember.submit.adding")}
                            </>
                        ) : (
                            t("dashboard.teamDetail.dialog.addMember.submit.add")
                        )}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}

// Members Table Component
function MembersSection({
    team,
    onAddMember,
    onRemoveMember,
}: {
    team: Team;
    onAddMember: () => void;
    onRemoveMember: (userId: string) => void;
}) {
    const members = team.members || [];
    const { users } = useUsers({});
    const { t } = useI18n();

    // Create a map of user details
    const userMap = new Map(users.map((u) => [u.user_id, u]));

    return (
        <Card className="glass-card">
            <CardHeader className="flex flex-row items-center justify-between py-4">
                <div>
                    <CardTitle className="text-lg">{t("dashboard.teamDetail.members.title")}</CardTitle>
                    <CardDescription>
                        {t("dashboard.organizationDetail.teams.count", {
                            count: members.length,
                            item: t("dashboard.teamDetail.stats.members"),
                        })}
                    </CardDescription>
                </div>
                <Button onClick={onAddMember} size="sm" className="gap-2">
                    <UserPlus className="w-4 h-4" />
                    {t("dashboard.teamDetail.members.action.add")}
                </Button>
            </CardHeader>
            <CardContent className="p-0">
                {members.length === 0 ? (
                    <EmptyState
                        icon={<Users className="w-12 h-12" />}
                        title={t("dashboard.teamDetail.members.empty.title")}
                        description={t("dashboard.teamDetail.members.empty.desc")}
                        action={
                            <Button onClick={onAddMember} variant="outline" size="sm">
                                <Plus className="w-4 h-4 mr-2" />
                                {t("dashboard.teamDetail.members.empty.action")}
                            </Button>
                        }
                        className="py-8"
                    />
                ) : (
                    <Table>
                        <TableHeader>
                            <TableRow className="hover:bg-transparent">
                                <TableHead className="text-xs uppercase text-muted-foreground">{t("dashboard.teamDetail.members.table.user")}</TableHead>
                                <TableHead className="text-xs uppercase text-muted-foreground">{t("dashboard.teamDetail.members.table.email")}</TableHead>
                                <TableHead className="text-xs uppercase text-muted-foreground">{t("dashboard.teamDetail.members.table.role")}</TableHead>
                                <TableHead className="text-xs uppercase text-muted-foreground">{t("dashboard.teamDetail.members.table.status")}</TableHead>
                                <TableHead className="text-xs uppercase text-muted-foreground w-16"></TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            <AnimatePresence mode="popLayout">
                                {members.map((userId, index) => {
                                    const user = userMap.get(userId);
                                    return (
                                        <motion.tr
                                            key={userId}
                                            initial={{ opacity: 0, y: 10 }}
                                            animate={{ opacity: 1, y: 0 }}
                                            exit={{ opacity: 0, y: -10 }}
                                            transition={{ delay: index * 0.05 }}
                                            className="group hover:bg-secondary/50 transition-colors"
                                        >
                                            <TableCell className="py-4">
                                                <div className="flex items-center gap-3">
                                                    <div className="w-8 h-8 rounded-full bg-primary/10 flex items-center justify-center">
                                                        <Users className="w-4 h-4 text-primary" />
                                                    </div>
                                                    <div>
                                                        <div className="font-medium">
                                                            {user?.user_alias || userId}
                                                        </div>
                                                        <div className="text-xs text-muted-foreground font-mono">
                                                            {userId.slice(0, 12)}...
                                                        </div>
                                                    </div>
                                                </div>
                                            </TableCell>
                                            <TableCell className="text-muted-foreground">
                                                {user?.user_email || "—"}
                                            </TableCell>
                                            <TableCell>
                                                <Badge variant="secondary">
                                                    {user?.user_role || "member"}
                                                </Badge>
                                            </TableCell>
                                            <TableCell>
                                                {user ? (
                                                    <StatusBadge isActive={user.is_active} size="sm" />
                                                ) : (
                                                    <Badge variant="secondary">{t("dashboard.teamDetail.members.table.unknown")}</Badge>
                                                )}
                                            </TableCell>
                                            <TableCell>
                                                <Button
                                                    variant="ghost"
                                                    size="icon"
                                                    onClick={() => onRemoveMember(userId)}
                                                    className="h-8 w-8 opacity-0 group-hover:opacity-100 transition-opacity text-red-400 hover:text-red-400 hover:bg-red-500/10"
                                                >
                                                    <Trash2 className="w-4 h-4" />
                                                </Button>
                                            </TableCell>
                                        </motion.tr>
                                    );
                                })}
                            </AnimatePresence>
                        </TableBody>
                    </Table>
                )}
            </CardContent>
        </Card>
    );
}

// Edit Team Dialog
function EditTeamDialog({
    open,
    onOpenChange,
    team,
    onSave,
}: {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    team: Team;
    onSave: (updates: Partial<CreateTeamRequest>) => Promise<void>;
}) {
    const { t } = useI18n();
    const [name, setName] = useState(team.team_alias || "");
    const [maxBudget, setMaxBudget] = useState(team.max_budget?.toString() || "");
    const [rpmLimit, setRpmLimit] = useState(team.rpm_limit?.toString() || "");
    const [tpmLimit, setTpmLimit] = useState(team.tpm_limit?.toString() || "");
    const [isSaving, setIsSaving] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const handleSave = async () => {
        setIsSaving(true);
        setError(null);

        try {
            await onSave({
                team_alias: name.trim() || undefined,
                max_budget: maxBudget ? parseFloat(maxBudget) : undefined,
                rpm_limit: rpmLimit ? parseInt(rpmLimit) : undefined,
                tpm_limit: tpmLimit ? parseInt(tpmLimit) : undefined,
            });
            onOpenChange(false);
        } catch (err) {
            setError(err instanceof Error ? err.message : t("dashboard.teamDetail.dialog.edit.error.updateFailed"));
        } finally {
            setIsSaving(false);
        }
    };

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="sm:max-w-md">
                <DialogHeader>
                    <DialogTitle>{t("dashboard.teamDetail.dialog.edit.title")}</DialogTitle>
                    <DialogDescription>
                        {t("dashboard.teamDetail.dialog.edit.description")}
                    </DialogDescription>
                </DialogHeader>

                <div className="space-y-4">
                    <div className="space-y-2">
                        <Label htmlFor="name">{t("dashboard.teamDetail.dialog.edit.field.name")}</Label>
                        <Input
                            id="name"
                            value={name}
                            onChange={(e) => setName(e.target.value)}
                            placeholder="e.g., Engineering"
                        />
                    </div>

                    <div className="grid grid-cols-2 gap-4">
                        <div className="space-y-2">
                            <Label htmlFor="budget">{t("dashboard.teamDetail.dialog.edit.field.maxBudget")}</Label>
                            <div className="relative">
                                <span className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground">$</span>
                                <Input
                                    id="budget"
                                    type="number"
                                    value={maxBudget}
                                    onChange={(e) => setMaxBudget(e.target.value)}
                                    placeholder="1000"
                                    className="pl-7"
                                />
                            </div>
                        </div>
                        <div className="space-y-2">
                            <Label htmlFor="rpm">{t("dashboard.teamDetail.dialog.edit.field.rpm")}</Label>
                            <Input
                                id="rpm"
                                type="number"
                                value={rpmLimit}
                                onChange={(e) => setRpmLimit(e.target.value)}
                                placeholder="100"
                            />
                        </div>
                    </div>

                    <div className="space-y-2">
                        <Label htmlFor="tpm">{t("dashboard.teamDetail.dialog.edit.field.tpm")}</Label>
                        <Input
                            id="tpm"
                            type="number"
                            value={tpmLimit}
                            onChange={(e) => setTpmLimit(e.target.value)}
                            placeholder="100000"
                        />
                    </div>

                    {error && (
                        <div className="flex items-center gap-2 p-3 bg-red-500/10 border border-red-500/20 rounded-lg">
                            <AlertCircle className="w-4 h-4 text-red-400" />
                            <p className="text-sm text-red-400">{error}</p>
                        </div>
                    )}
                </div>

                <DialogFooter>
                    <Button variant="ghost" onClick={() => onOpenChange(false)}>
                        {t("common.cancel")}
                    </Button>
                    <Button onClick={handleSave} disabled={isSaving}>
                        {isSaving ? (
                            <>
                                <RefreshCw className="w-4 h-4 mr-2 animate-spin" />
                                {t("dashboard.teamDetail.dialog.edit.submit.saving")}
                            </>
                        ) : (
                            t("dashboard.teamDetail.dialog.edit.submit.save")
                        )}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}

// Settings Tab Component
function SettingsSection({ team }: { team: Team }) {
    const { t } = useI18n();
    return (
        <Card className="glass-card">
            <CardHeader>
                <CardTitle className="text-lg">{t("dashboard.teamDetail.settings.title")}</CardTitle>
                <CardDescription>{t("dashboard.teamDetail.settings.subtitle")}</CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
                {/* Budget Settings */}
                <div className="space-y-3">
                    <h4 className="text-sm font-medium flex items-center gap-2">
                        <DollarSign className="w-4 h-4 text-green-400" />
                        {t("dashboard.teamDetail.settings.budget.title")}
                    </h4>
                    <div className="grid grid-cols-2 gap-4">
                        <div className="p-4 rounded-lg bg-secondary/50">
                            <div className="text-sm text-muted-foreground">{t("dashboard.teamDetail.settings.budget.max")}</div>
                            <div className="text-lg font-semibold">
                                {team.max_budget ? `$${team.max_budget.toFixed(2)}` : t("budget.noLimit")}
                            </div>
                        </div>
                        <div className="p-4 rounded-lg bg-secondary/50">
                            <div className="text-sm text-muted-foreground">{t("dashboard.teamDetail.settings.budget.current")}</div>
                            <div className="text-lg font-semibold">${team.spend.toFixed(2)}</div>
                        </div>
                    </div>
                    {team.max_budget && (
                        <BudgetProgress spent={team.spend} max={team.max_budget} />
                    )}
                </div>

                {/* Rate Limits */}
                <div className="space-y-3">
                    <h4 className="text-sm font-medium flex items-center gap-2">
                        <Zap className="w-4 h-4 text-yellow-400" />
                        {t("dashboard.teamDetail.settings.rate.title")}
                    </h4>
                    <div className="grid grid-cols-3 gap-4">
                        <div className="p-4 rounded-lg bg-secondary/50">
                            <div className="text-sm text-muted-foreground">RPM</div>
                            <div className="text-lg font-semibold">
                                {team.rpm_limit?.toLocaleString() || "∞"}
                            </div>
                        </div>
                        <div className="p-4 rounded-lg bg-secondary/50">
                            <div className="text-sm text-muted-foreground">TPM</div>
                            <div className="text-lg font-semibold">
                                {team.tpm_limit?.toLocaleString() || "∞"}
                            </div>
                        </div>
                        <div className="p-4 rounded-lg bg-secondary/50">
                            <div className="text-sm text-muted-foreground">{t("dashboard.teamDetail.settings.rate.maxParallel")}</div>
                            <div className="text-lg font-semibold">
                                {team.max_parallel_requests?.toLocaleString() || "∞"}
                            </div>
                        </div>
                    </div>
                </div>

                {/* Allowed Models */}
                <div className="space-y-3">
                    <h4 className="text-sm font-medium flex items-center gap-2">
                        <Key className="w-4 h-4 text-purple-400" />
                        {t("dashboard.teamDetail.settings.models.title")}
                    </h4>
                    <div className="flex flex-wrap gap-2">
                        {team.models && team.models.length > 0 ? (
                            team.models.map((model) => (
                                <Badge key={model} variant="secondary">
                                    {model}
                                </Badge>
                            ))
                        ) : (
                            <span className="text-muted-foreground text-sm">{t("dashboard.teamDetail.settings.models.allAllowed")}</span>
                        )}
                    </div>
                </div>

                {/* Metadata */}
                <div className="space-y-3">
                    <h4 className="text-sm font-medium flex items-center gap-2">
                        <Clock className="w-4 h-4 text-blue-400" />
                        {t("dashboard.teamDetail.settings.meta.title")}
                    </h4>
                    <div className="grid grid-cols-2 gap-4 text-sm">
                        <div>
                            <span className="text-muted-foreground">{t("dashboard.teamDetail.settings.meta.created")}</span>
                            <span className="ml-2">{new Date(team.created_at).toLocaleDateString()}</span>
                        </div>
                        <div>
                            <span className="text-muted-foreground">{t("dashboard.teamDetail.settings.meta.updated")}</span>
                            <span className="ml-2">{new Date(team.updated_at).toLocaleDateString()}</span>
                        </div>
                    </div>
                </div>
            </CardContent>
        </Card>
    );
}

// Loading skeleton for the page
function TeamDetailSkeleton() {
    return (
        <div className="space-y-6">
            {/* Header */}
            <div className="flex items-start justify-between">
                <div className="flex items-center gap-4">
                    <Skeleton className="h-10 w-10 rounded-lg" />
                    <Skeleton className="w-16 h-16 rounded-xl" />
                    <div className="space-y-2">
                        <Skeleton className="h-8 w-48" />
                        <Skeleton className="h-4 w-32" />
                    </div>
                </div>
                <div className="flex gap-2">
                    <Skeleton className="h-10 w-24 rounded-lg" />
                    <Skeleton className="h-10 w-20 rounded-lg" />
                </div>
            </div>

            {/* Stats */}
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
                {[1, 2, 3, 4].map((i) => (
                    <CardSkeleton key={i} />
                ))}
            </div>

            {/* Content */}
            <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
                <div className="lg:col-span-2">
                    <Skeleton className="h-96 rounded-xl" />
                </div>
                <Skeleton className="h-96 rounded-xl" />
            </div>
        </div>
    );
}

// Main Component
export default function TeamDetailPage() {
    const { t } = useI18n();
    const params = useParams();
    const teamId = params.id as string;

    const { team, isLoading, error, refresh } = useTeamInfo(teamId);
    const { addMember, removeMember } = useTeamMembers({ teamId });

    const [editDialogOpen, setEditDialogOpen] = useState(false);
    const [addMemberDialogOpen, setAddMemberDialogOpen] = useState(false);
    const [activeTab, setActiveTab] = useState("members");

    const handleBlock = async () => {
        try {
            await apiClient.blockTeam(teamId);
            refresh();
        } catch (err) {
            console.error("Failed to block team:", err);
        }
    };

    const handleUnblock = async () => {
        try {
            await apiClient.unblockTeam(teamId);
            refresh();
        } catch (err) {
            console.error("Failed to unblock team:", err);
        }
    };

    const handleUpdateTeam = async (updates: Partial<CreateTeamRequest>) => {
        await apiClient.updateTeam(teamId, updates);
        refresh();
    };

    const handleAddMember = async (userId: string) => {
        await addMember(userId);
        refresh();
    };

    const handleRemoveMember = async (userId: string) => {
        await removeMember(userId);
        refresh();
    };

    if (isLoading) {
        return <TeamDetailSkeleton />;
    }

    if (error) {
        return (
            <div className="space-y-6">
                <div className="flex items-center gap-4">
                    <Link href="/teams">
                        <Button variant="ghost" size="icon">
                            <ArrowLeft className="w-5 h-5" />
                        </Button>
                    </Link>
                    <h1 className="text-2xl font-bold">{t("dashboard.teamDetail.error.title")}</h1>
                </div>
                <ErrorState message={error.message} onRetry={refresh} />
            </div>
        );
    }

    if (!team) {
        return (
            <div className="space-y-6">
                <div className="flex items-center gap-4">
                    <Link href="/teams">
                        <Button variant="ghost" size="icon">
                            <ArrowLeft className="w-5 h-5" />
                        </Button>
                    </Link>
                    <h1 className="text-2xl font-bold">{t("dashboard.teamDetail.notFound.title")}</h1>
                </div>
                <EmptyState
                    icon={<Users className="w-12 h-12" />}
                    title={t("dashboard.teamDetail.notFound.emptyTitle")}
                    description={t("dashboard.teamDetail.notFound.emptyDesc")}
                    action={
                        <Link href="/teams">
                            <Button>{t("dashboard.teamDetail.notFound.action.back")}</Button>
                        </Link>
                    }
                />
            </div>
        );
    }

    return (
        <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            className="space-y-6"
        >
            {/* Header */}
            <TeamDetailHeader
                team={team}
                onEdit={() => setEditDialogOpen(true)}
                onBlock={handleBlock}
                onUnblock={handleUnblock}
            />

            {/* Stats */}
            <TeamStatsCards team={team} />

            {/* Tabs Content */}
            <Tabs value={activeTab} onValueChange={setActiveTab}>
                <TabsList className="w-full md:w-auto">
                    <TabsTrigger value="members" className="gap-2">
                        <Users className="w-4 h-4" />
                        {t("dashboard.teamDetail.tabs.members")}
                    </TabsTrigger>
                    <TabsTrigger value="settings" className="gap-2">
                        <Settings className="w-4 h-4" />
                        {t("dashboard.teamDetail.tabs.settings")}
                    </TabsTrigger>
                </TabsList>

                <TabsContent value="members" className="mt-6">
                    <MembersSection
                        team={team}
                        onAddMember={() => setAddMemberDialogOpen(true)}
                        onRemoveMember={handleRemoveMember}
                    />
                </TabsContent>

                <TabsContent value="settings" className="mt-6">
                    <SettingsSection team={team} />
                </TabsContent>
            </Tabs>

            {/* Dialogs */}
            <EditTeamDialog
                open={editDialogOpen}
                onOpenChange={setEditDialogOpen}
                team={team}
                onSave={handleUpdateTeam}
            />

            <AddMemberDialog
                open={addMemberDialogOpen}
                onOpenChange={setAddMemberDialogOpen}
                onAdd={handleAddMember}
                existingMembers={team.members || []}
            />
        </motion.div>
    );
}
