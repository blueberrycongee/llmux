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
    Building2,
    Users,
    Settings,
    DollarSign,
    Key,
    Plus,
    Trash2,
    Edit,
    RefreshCw,
    AlertCircle,
    UserPlus,
    Shield,
    Clock,
} from "lucide-react";
import { useOrganizationInfo, useOrganizationMembers, useTeams, useUsers } from "@/hooks";
import { apiClient } from "@/lib/api";
import { StatusBadge, BudgetProgress, EmptyState, ErrorState, RoleBadge } from "@/components/shared/common";
import { Skeleton, CardSkeleton, TableRowSkeleton } from "@/components/ui/skeleton";
import type { Organization, CreateOrganizationRequest, OrganizationMembership } from "@/types/api";
import { useI18n } from "@/i18n/locale-provider";

// Organization Detail Header
function OrgDetailHeader({
    org,
    onEdit,
}: {
    org: Organization;
    onEdit: () => void;
}) {
    const { t } = useI18n();
    return (
        <div className="flex items-start justify-between">
            <div className="flex items-center gap-4">
                <Link href="/organizations">
                    <Button variant="ghost" size="icon" className="h-10 w-10">
                        <ArrowLeft className="w-5 h-5" />
                    </Button>
                </Link>
                <div className="w-16 h-16 rounded-xl bg-gradient-to-br from-blue-500/20 to-purple-500/20 flex items-center justify-center border border-blue-500/20">
                    <Building2 className="w-8 h-8 text-blue-400" />
                </div>
                <div>
                    <div className="flex items-center gap-3">
                        <h1 className="text-2xl font-bold tracking-tight">
                            {org.organization_alias}
                        </h1>
                        <Badge variant="success" className="gap-1">
                            <Shield className="w-3 h-3" />
                            {t("dashboard.organizationDetail.badge.active")}
                        </Badge>
                    </div>
                    <p className="text-sm text-muted-foreground font-mono">{org.organization_id}</p>
                </div>
            </div>
            <Button onClick={onEdit} className="gap-2">
                <Edit className="w-4 h-4" />
                {t("dashboard.organizationDetail.action.edit")}
            </Button>
        </div>
    );
}

// Stats Cards
function OrgStatsCards({ org, memberCount, teamCount }: { org: Organization; memberCount: number; teamCount: number }) {
    const { t } = useI18n();
    const stats = [
        {
            label: t("dashboard.organizationDetail.stats.teams"),
            value: teamCount,
            icon: Users,
            color: "text-blue-400",
            bgColor: "bg-blue-500/10",
        },
        {
            label: t("dashboard.organizationDetail.stats.members"),
            value: memberCount,
            icon: Users,
            color: "text-green-400",
            bgColor: "bg-green-500/10",
        },
        {
            label: t("dashboard.organizationDetail.stats.budgetUsed"),
            value: `$${org.spend.toFixed(2)}`,
            subtitle: org.max_budget
                ? t("dashboard.organizationDetail.stats.ofBudget", { amount: org.max_budget.toFixed(2) })
                : t("dashboard.organizationDetail.stats.noLimit"),
            icon: DollarSign,
            color: "text-yellow-400",
            bgColor: "bg-yellow-500/10",
        },
        {
            label: t("dashboard.organizationDetail.stats.models"),
            value: org.models?.length || t("dashboard.organizationDetail.stats.modelsAll"),
            subtitle: org.models?.length
                ? `${org.models.slice(0, 2).join(", ")}...`
                : t("dashboard.organizationDetail.stats.modelsAllAllowed"),
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

// Teams Section
function TeamsSection({ organizationId }: { organizationId: string }) {
    const { teams, isLoading } = useTeams({ organizationId });
    const { t } = useI18n();

    if (isLoading) {
        return (
            <Card className="glass-card">
                <CardHeader>
                    <CardTitle className="text-lg">{t("dashboard.organizationDetail.teams.title")}</CardTitle>
                </CardHeader>
                <CardContent>
                    <div className="space-y-3">
                        {[1, 2, 3].map((i) => (
                            <Skeleton key={i} className="h-16 rounded-lg" />
                        ))}
                    </div>
                </CardContent>
            </Card>
        );
    }

    if (teams.length === 0) {
        return (
            <Card className="glass-card">
                <CardHeader>
                    <CardTitle className="text-lg">{t("dashboard.organizationDetail.teams.title")}</CardTitle>
                    <CardDescription>{t("dashboard.organizationDetail.teams.subtitle")}</CardDescription>
                </CardHeader>
                <CardContent>
                    <EmptyState
                        icon={<Users className="w-12 h-12" />}
                        title={t("dashboard.organizationDetail.teams.empty.title")}
                        description={t("dashboard.organizationDetail.teams.empty.desc")}
                        action={
                            <Link href="/teams">
                                <Button variant="outline" size="sm">
                                    <Plus className="w-4 h-4 mr-2" />
                                    {t("dashboard.organizationDetail.teams.empty.action")}
                                </Button>
                            </Link>
                        }
                        className="py-6"
                    />
                </CardContent>
            </Card>
        );
    }

    return (
        <Card className="glass-card">
            <CardHeader className="flex flex-row items-center justify-between py-4">
                <div>
                    <CardTitle className="text-lg">{t("dashboard.organizationDetail.teams.title")}</CardTitle>
                    <CardDescription>
                        {t("dashboard.organizationDetail.teams.count", {
                            count: teams.length,
                            item: t("dashboard.organizationDetail.stats.teams"),
                        })}
                    </CardDescription>
                </div>
                <Link href="/teams">
                    <Button variant="outline" size="sm">{t("common.viewAll")}</Button>
                </Link>
            </CardHeader>
            <CardContent className="space-y-3">
                <AnimatePresence>
                    {teams.map((team, i) => (
                        <motion.div
                            key={team.team_id}
                            initial={{ opacity: 0, y: 10 }}
                            animate={{ opacity: 1, y: 0 }}
                            transition={{ delay: i * 0.05 }}
                        >
                            <Link href={`/teams/${team.team_id}`}>
                                <div className="flex items-center justify-between p-4 rounded-lg bg-secondary/50 hover:bg-secondary transition-colors group">
                                    <div className="flex items-center gap-3">
                                        <div className="w-10 h-10 rounded-lg bg-primary/10 flex items-center justify-center">
                                            <Users className="w-5 h-5 text-primary" />
                                        </div>
                                        <div>
                                            <div className="font-medium">{team.team_alias || team.team_id}</div>
                                            <div className="text-xs text-muted-foreground">
                                                {t("dashboard.organizationDetail.stats.members")}: {team.members?.length || 0} • ${team.spend.toFixed(2)}
                                            </div>
                                        </div>
                                    </div>
                                    <StatusBadge isActive={team.is_active} blocked={team.blocked} size="sm" />
                                </div>
                            </Link>
                        </motion.div>
                    ))}
                </AnimatePresence>
            </CardContent>
        </Card>
    );
}

// Add Member Dialog
function AddMemberDialog({
    open,
    onOpenChange,
    onAdd,
    existingMemberIds,
}: {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    onAdd: (userId: string, role?: string) => Promise<void>;
    existingMemberIds: string[];
}) {
    const [selectedUserId, setSelectedUserId] = useState("");
    const [role, setRole] = useState("member");
    const [isAdding, setIsAdding] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [searchQuery, setSearchQuery] = useState("");
    const { users, isLoading } = useUsers({});
    const { t } = useI18n();

    // Filter out existing members
    const availableUsers = users.filter(
        (u) => !existingMemberIds.includes(u.user_id) &&
            (u.user_id.toLowerCase().includes(searchQuery.toLowerCase()) ||
                (u.user_alias?.toLowerCase() || "").includes(searchQuery.toLowerCase()) ||
                (u.user_email?.toLowerCase() || "").includes(searchQuery.toLowerCase()))
    );

    const handleAdd = async () => {
        if (!selectedUserId) {
            setError(t("dashboard.organizationDetail.dialog.addMember.validation.selectUser"));
            return;
        }

        setIsAdding(true);
        setError(null);

        try {
            await onAdd(selectedUserId, role);
            handleClose();
        } catch (err) {
            setError(err instanceof Error ? err.message : t("dashboard.organizationDetail.dialog.addMember.error.addFailed"));
        } finally {
            setIsAdding(false);
        }
    };

    const handleClose = () => {
        setSelectedUserId("");
        setRole("member");
        setSearchQuery("");
        setError(null);
        onOpenChange(false);
    };

    return (
        <Dialog open={open} onOpenChange={handleClose}>
            <DialogContent className="sm:max-w-lg">
                <DialogHeader>
                    <DialogTitle>{t("dashboard.organizationDetail.dialog.addMember.title")}</DialogTitle>
                    <DialogDescription>
                        {t("dashboard.organizationDetail.dialog.addMember.description")}
                    </DialogDescription>
                </DialogHeader>

                <div className="space-y-4">
                    <div className="space-y-2">
                        <Label>{t("dashboard.organizationDetail.dialog.addMember.searchLabel")}</Label>
                        <Input
                            placeholder={t("dashboard.organizationDetail.dialog.addMember.searchPlaceholder")}
                            value={searchQuery}
                            onChange={(e) => setSearchQuery(e.target.value)}
                        />
                    </div>

                    <div className="max-h-48 overflow-y-auto border rounded-lg divide-y">
                        {isLoading ? (
                            <div className="p-4 text-center text-muted-foreground">{t("dashboard.organizationDetail.dialog.addMember.loading")}</div>
                        ) : availableUsers.length === 0 ? (
                            <div className="p-4 text-center text-muted-foreground">
                                {searchQuery ? t("dashboard.organizationDetail.dialog.addMember.noMatch") : t("dashboard.organizationDetail.dialog.addMember.none")}
                            </div>
                        ) : (
                            availableUsers.map((user) => (
                                <button
                                    key={user.user_id}
                                    onClick={() => setSelectedUserId(user.user_id)}
                                    className={`w-full flex items-center gap-3 p-3 text-left hover:bg-secondary transition-colors ${selectedUserId === user.user_id ? "bg-secondary" : ""
                                        }`}
                                >
                                    <div className="w-9 h-9 rounded-full bg-primary/10 flex items-center justify-center shrink-0">
                                        <Users className="w-4 h-4 text-primary" />
                                    </div>
                                    <div className="flex-1 min-w-0">
                                        <div className="font-medium truncate text-sm">
                                            {user.user_alias || user.user_id}
                                        </div>
                                        <div className="text-xs text-muted-foreground truncate">
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

                    <div className="space-y-2">
                        <Label>{t("dashboard.organizationDetail.dialog.addMember.role")}</Label>
                        <div className="flex gap-2">
                            <Button
                                type="button"
                                variant={role === "member" ? "default" : "outline"}
                                size="sm"
                                onClick={() => setRole("member")}
                            >
                                {t("dashboard.organizationDetail.dialog.addMember.role.member")}
                            </Button>
                            <Button
                                type="button"
                                variant={role === "org_admin" ? "default" : "outline"}
                                size="sm"
                                onClick={() => setRole("org_admin")}
                            >
                                {t("dashboard.organizationDetail.dialog.addMember.role.admin")}
                            </Button>
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
                    <Button onClick={handleAdd} disabled={isAdding || !selectedUserId}>
                        {isAdding ? (
                            <>
                                <RefreshCw className="w-4 h-4 mr-2 animate-spin" />
                                {t("dashboard.organizationDetail.dialog.addMember.submit.adding")}
                            </>
                        ) : (
                            t("dashboard.organizationDetail.dialog.addMember.submit.add")
                        )}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}

// Members Section
function MembersSection({
    members,
    isLoading,
    onAddMember,
    onRemoveMember,
}: {
    members: OrganizationMembership[];
    isLoading: boolean;
    onAddMember: () => void;
    onRemoveMember: (userId: string) => void;
}) {
    const { users } = useUsers({});
    const userMap = new Map(users.map((u) => [u.user_id, u]));
    const { t } = useI18n();

    if (isLoading) {
        return (
            <Card className="glass-card">
                <CardHeader>
                    <CardTitle className="text-lg">{t("dashboard.organizationDetail.members.title")}</CardTitle>
                </CardHeader>
                <CardContent className="p-0">
                    <Table>
                        <TableHeader>
                            <TableRow className="hover:bg-transparent">
                                <TableHead>{t("dashboard.organizationDetail.members.table.user")}</TableHead>
                                <TableHead>{t("dashboard.organizationDetail.members.table.role")}</TableHead>
                                <TableHead>{t("dashboard.organizationDetail.members.table.spend")}</TableHead>
                                <TableHead className="w-16"></TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {[1, 2, 3].map((i) => (
                                <TableRowSkeleton key={i} columns={4} />
                            ))}
                        </TableBody>
                    </Table>
                </CardContent>
            </Card>
        );
    }

    return (
        <Card className="glass-card">
            <CardHeader className="flex flex-row items-center justify-between py-4">
                <div>
                    <CardTitle className="text-lg">{t("dashboard.organizationDetail.members.title")}</CardTitle>
                    <CardDescription>
                        {t("dashboard.organizationDetail.teams.count", {
                            count: members.length,
                            item: t("dashboard.organizationDetail.stats.members"),
                        })}
                    </CardDescription>
                </div>
                <Button onClick={onAddMember} size="sm" className="gap-2">
                    <UserPlus className="w-4 h-4" />
                    {t("dashboard.organizationDetail.members.action.add")}
                </Button>
            </CardHeader>
            <CardContent className="p-0">
                {members.length === 0 ? (
                    <EmptyState
                        icon={<Users className="w-12 h-12" />}
                        title={t("dashboard.organizationDetail.members.empty.title")}
                        description={t("dashboard.organizationDetail.members.empty.desc")}
                        action={
                            <Button onClick={onAddMember} variant="outline" size="sm">
                                <Plus className="w-4 h-4 mr-2" />
                                {t("dashboard.organizationDetail.members.empty.action")}
                            </Button>
                        }
                        className="py-8"
                    />
                ) : (
                    <Table>
                        <TableHeader>
                            <TableRow className="hover:bg-transparent">
                                <TableHead className="text-xs uppercase text-muted-foreground">{t("dashboard.organizationDetail.members.table.user")}</TableHead>
                                <TableHead className="text-xs uppercase text-muted-foreground">{t("dashboard.organizationDetail.members.table.role")}</TableHead>
                                <TableHead className="text-xs uppercase text-muted-foreground">{t("dashboard.organizationDetail.members.table.spend")}</TableHead>
                                <TableHead className="text-xs uppercase text-muted-foreground">{t("dashboard.organizationDetail.members.table.joined")}</TableHead>
                                <TableHead className="w-16"></TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            <AnimatePresence mode="popLayout">
                                {members.map((member, index) => {
                                    const user = userMap.get(member.user_id);
                                    return (
                                        <motion.tr
                                            key={member.user_id}
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
                                                            {user?.user_alias || member.user_id}
                                                        </div>
                                                        <div className="text-xs text-muted-foreground">
                                                            {user?.user_email || member.user_id.slice(0, 12) + "..."}
                                                        </div>
                                                    </div>
                                                </div>
                                            </TableCell>
                                            <TableCell>
                                                <Badge variant={member.user_role === "org_admin" ? "warning" : "secondary"}>
                                                    {member.user_role || "member"}
                                                </Badge>
                                            </TableCell>
                                            <TableCell className="font-medium">
                                                ${member.spend.toFixed(2)}
                                            </TableCell>
                                            <TableCell className="text-muted-foreground text-sm">
                                                {member.joined_at
                                                    ? new Date(member.joined_at).toLocaleDateString()
                                                    : t("dashboard.organizationDetail.members.table.joinedEmpty")}
                                            </TableCell>
                                            <TableCell>
                                                <Button
                                                    variant="ghost"
                                                    size="icon"
                                                    onClick={() => onRemoveMember(member.user_id)}
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

// Settings Section
function SettingsSection({ org }: { org: Organization }) {
    const { t } = useI18n();
    return (
        <Card className="glass-card">
            <CardHeader>
                <CardTitle className="text-lg">{t("dashboard.organizationDetail.settings.title")}</CardTitle>
                <CardDescription>{t("dashboard.organizationDetail.settings.subtitle")}</CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
                {/* Budget Settings */}
                <div className="space-y-3">
                    <h4 className="text-sm font-medium flex items-center gap-2">
                        <DollarSign className="w-4 h-4 text-green-400" />
                        {t("dashboard.organizationDetail.settings.budget.title")}
                    </h4>
                    <div className="grid grid-cols-2 gap-4">
                        <div className="p-4 rounded-lg bg-secondary/50">
                            <div className="text-sm text-muted-foreground">{t("dashboard.organizationDetail.settings.budget.max")}</div>
                            <div className="text-lg font-semibold">
                                {org.max_budget ? `$${org.max_budget.toFixed(2)}` : t("budget.noLimit")}
                            </div>
                        </div>
                        <div className="p-4 rounded-lg bg-secondary/50">
                            <div className="text-sm text-muted-foreground">{t("dashboard.organizationDetail.settings.budget.current")}</div>
                            <div className="text-lg font-semibold">${org.spend.toFixed(2)}</div>
                        </div>
                    </div>
                    {org.max_budget && (
                        <BudgetProgress spent={org.spend} max={org.max_budget} />
                    )}
                </div>

                {/* Allowed Models */}
                <div className="space-y-3">
                    <h4 className="text-sm font-medium flex items-center gap-2">
                        <Key className="w-4 h-4 text-purple-400" />
                        {t("dashboard.organizationDetail.settings.models.title")}
                    </h4>
                    <div className="flex flex-wrap gap-2">
                        {org.models && org.models.length > 0 ? (
                            org.models.map((model) => (
                                <Badge key={model} variant="secondary">
                                    {model}
                                </Badge>
                            ))
                        ) : (
                            <span className="text-muted-foreground text-sm">{t("dashboard.organizationDetail.settings.models.allAllowed")}</span>
                        )}
                    </div>
                </div>

                {/* Model Spend */}
                {org.model_spend && Object.keys(org.model_spend).length > 0 && (
                    <div className="space-y-3">
                        <h4 className="text-sm font-medium">{t("dashboard.organizationDetail.settings.spendByModel")}</h4>
                        <div className="space-y-2">
                            {Object.entries(org.model_spend).map(([model, spend]) => (
                                <div key={model} className="flex items-center justify-between p-3 rounded-lg bg-secondary/50">
                                    <span className="text-sm font-medium">{model}</span>
                                    <span className="text-sm">${(spend as number).toFixed(2)}</span>
                                </div>
                            ))}
                        </div>
                    </div>
                )}

                {/* Metadata */}
                <div className="space-y-3">
                    <h4 className="text-sm font-medium flex items-center gap-2">
                        <Clock className="w-4 h-4 text-blue-400" />
                        {t("dashboard.organizationDetail.settings.meta.title")}
                    </h4>
                    <div className="grid grid-cols-2 gap-4 text-sm">
                        <div>
                            <span className="text-muted-foreground">{t("dashboard.organizationDetail.settings.meta.created")}</span>
                            <span className="ml-2">{new Date(org.created_at).toLocaleDateString()}</span>
                        </div>
                        <div>
                            <span className="text-muted-foreground">{t("dashboard.organizationDetail.settings.meta.updated")}</span>
                            <span className="ml-2">{new Date(org.updated_at).toLocaleDateString()}</span>
                        </div>
                    </div>
                </div>
            </CardContent>
        </Card>
    );
}

// Edit Organization Dialog
function EditOrgDialog({
    open,
    onOpenChange,
    org,
    onSave,
}: {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    org: Organization;
    onSave: (updates: Partial<CreateOrganizationRequest>) => Promise<void>;
}) {
    const { t } = useI18n();
    const [name, setName] = useState(org.organization_alias);
    const [maxBudget, setMaxBudget] = useState(org.max_budget?.toString() || "");
    const [isSaving, setIsSaving] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const handleSave = async () => {
        if (!name.trim()) {
            setError(t("dashboard.organizations.form.name.required"));
            return;
        }

        setIsSaving(true);
        setError(null);

        try {
            await onSave({
                organization_alias: name.trim(),
                max_budget: maxBudget ? parseFloat(maxBudget) : undefined,
            });
            onOpenChange(false);
        } catch (err) {
            setError(err instanceof Error ? err.message : t("dashboard.organizationDetail.dialog.edit.error.updateFailed"));
        } finally {
            setIsSaving(false);
        }
    };

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="sm:max-w-md">
                <DialogHeader>
                    <DialogTitle>{t("dashboard.organizationDetail.dialog.edit.title")}</DialogTitle>
                    <DialogDescription>
                        {t("dashboard.organizationDetail.dialog.edit.description")}
                    </DialogDescription>
                </DialogHeader>

                <div className="space-y-4">
                    <div className="space-y-2">
                        <Label htmlFor="name">{t("dashboard.organizations.form.name.label")}</Label>
                        <Input
                            id="name"
                            value={name}
                            onChange={(e) => setName(e.target.value)}
                            placeholder={t("dashboard.organizations.form.name.placeholder")}
                        />
                    </div>

                    <div className="space-y-2">
                        <Label htmlFor="budget">{t("dashboard.organizationDetail.settings.budget.max")}</Label>
                        <div className="relative">
                            <span className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground">$</span>
                            <Input
                                id="budget"
                                type="number"
                                value={maxBudget}
                                onChange={(e) => setMaxBudget(e.target.value)}
                                placeholder={t("dashboard.organizations.form.maxBudget.placeholder")}
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
                    <Button variant="ghost" onClick={() => onOpenChange(false)}>
                        {t("common.cancel")}
                    </Button>
                    <Button onClick={handleSave} disabled={isSaving}>
                        {isSaving ? (
                            <>
                                <RefreshCw className="w-4 h-4 mr-2 animate-spin" />
                                {t("dashboard.organizationDetail.dialog.edit.submit.saving")}
                            </>
                        ) : (
                            t("dashboard.organizationDetail.dialog.edit.submit.save")
                        )}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}

// Loading Skeleton
function OrgDetailSkeleton() {
    return (
        <div className="space-y-6">
            <div className="flex items-start justify-between">
                <div className="flex items-center gap-4">
                    <Skeleton className="h-10 w-10 rounded-lg" />
                    <Skeleton className="w-16 h-16 rounded-xl" />
                    <div className="space-y-2">
                        <Skeleton className="h-8 w-48" />
                        <Skeleton className="h-4 w-32" />
                    </div>
                </div>
                <Skeleton className="h-10 w-20 rounded-lg" />
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
                {[1, 2, 3, 4].map((i) => (
                    <CardSkeleton key={i} />
                ))}
            </div>

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
export default function OrganizationDetailPage() {
    const { t } = useI18n();
    const params = useParams();
    const orgId = params.id as string;

    const { organization, isLoading, error, refresh, updateOrganization } = useOrganizationInfo(orgId);
    const { members, isLoading: membersLoading, addMember, removeMember } = useOrganizationMembers(orgId);
    const { teams } = useTeams({ organizationId: orgId });

    const [editDialogOpen, setEditDialogOpen] = useState(false);
    const [addMemberDialogOpen, setAddMemberDialogOpen] = useState(false);
    const [activeTab, setActiveTab] = useState("teams");

    const handleUpdateOrg = async (updates: Partial<CreateOrganizationRequest>) => {
        await updateOrganization(updates);
    };

    const handleAddMember = async (userId: string, role?: string) => {
        await addMember(userId, role);
    };

    const handleRemoveMember = async (userId: string) => {
        await removeMember(userId);
    };

    if (isLoading) {
        return <OrgDetailSkeleton />;
    }

    if (error) {
        return (
            <div className="space-y-6">
                <div className="flex items-center gap-4">
                    <Link href="/organizations">
                        <Button variant="ghost" size="icon">
                            <ArrowLeft className="w-5 h-5" />
                        </Button>
                    </Link>
                    <h1 className="text-2xl font-bold">{t("dashboard.organizationDetail.error.title")}</h1>
                </div>
                <ErrorState message={error.message} onRetry={refresh} />
            </div>
        );
    }

    if (!organization) {
        return (
            <div className="space-y-6">
                <div className="flex items-center gap-4">
                    <Link href="/organizations">
                        <Button variant="ghost" size="icon">
                            <ArrowLeft className="w-5 h-5" />
                        </Button>
                    </Link>
                    <h1 className="text-2xl font-bold">{t("dashboard.organizationDetail.notFound.title")}</h1>
                </div>
                <EmptyState
                    icon={<Building2 className="w-12 h-12" />}
                    title={t("dashboard.organizationDetail.notFound.emptyTitle")}
                    description={t("dashboard.organizationDetail.notFound.emptyDesc")}
                    action={
                        <Link href="/organizations">
                            <Button>{t("dashboard.organizationDetail.notFound.action.back")}</Button>
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
            <OrgDetailHeader
                org={organization}
                onEdit={() => setEditDialogOpen(true)}
            />

            {/* Stats */}
            <OrgStatsCards
                org={organization}
                memberCount={members.length}
                teamCount={teams.length}
            />

            {/* Tabs Content */}
            <Tabs value={activeTab} onValueChange={setActiveTab}>
                <TabsList className="w-full md:w-auto">
                    <TabsTrigger value="teams" className="gap-2">
                        <Users className="w-4 h-4" />
                        {t("dashboard.organizationDetail.teams.title")}
                    </TabsTrigger>
                    <TabsTrigger value="members" className="gap-2">
                        <Users className="w-4 h-4" />
                        {t("dashboard.organizationDetail.members.title")}
                    </TabsTrigger>
                    <TabsTrigger value="settings" className="gap-2">
                        <Settings className="w-4 h-4" />
                        {t("nav.settings")}
                    </TabsTrigger>
                </TabsList>

                <TabsContent value="teams" className="mt-6">
                    <TeamsSection organizationId={orgId} />
                </TabsContent>

                <TabsContent value="members" className="mt-6">
                    <MembersSection
                        members={members}
                        isLoading={membersLoading}
                        onAddMember={() => setAddMemberDialogOpen(true)}
                        onRemoveMember={handleRemoveMember}
                    />
                </TabsContent>

                <TabsContent value="settings" className="mt-6">
                    <SettingsSection org={organization} />
                </TabsContent>
            </Tabs>

            {/* Dialogs */}
            <EditOrgDialog
                open={editDialogOpen}
                onOpenChange={setEditDialogOpen}
                org={organization}
                onSave={handleUpdateOrg}
            />

            <AddMemberDialog
                open={addMemberDialogOpen}
                onOpenChange={setAddMemberDialogOpen}
                onAdd={handleAddMember}
                existingMemberIds={members.map((m) => m.user_id)}
            />
        </motion.div>
    );
}
