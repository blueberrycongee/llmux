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
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
import {
    ArrowLeft,
    Users,
    Settings,
    DollarSign,
    Key,
    Edit,
    RefreshCw,
    AlertCircle,
    Mail,
    Shield,
    ShieldOff,
    Clock,
    Building2,
    Activity,
    Zap,
} from "lucide-react";
import { useUserInfo, useApiKeys, useTeams } from "@/hooks";
import { apiClient } from "@/lib/api";
import { StatusBadge, RoleBadge, BudgetProgress, EmptyState, ErrorState } from "@/components/shared/common";
import { Skeleton, CardSkeleton } from "@/components/ui/skeleton";
import type { User, CreateUserRequest, UserRole } from "@/types/api";

// Role options
const roleOptions: { value: UserRole; label: string }[] = [
    { value: "proxy_admin", label: "Proxy Admin" },
    { value: "proxy_admin_viewer", label: "Admin Viewer" },
    { value: "org_admin", label: "Org Admin" },
    { value: "internal_user", label: "Internal User" },
    { value: "internal_user_viewer", label: "Internal Viewer" },
    { value: "team", label: "Team" },
    { value: "customer", label: "Customer" },
];

// User Detail Header
function UserDetailHeader({
    user,
    onEdit,
}: {
    user: User;
    onEdit: () => void;
}) {
    return (
        <div className="flex items-start justify-between">
            <div className="flex items-center gap-4">
                <Link href="/users">
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
                            {user.user_alias || "Unnamed User"}
                        </h1>
                        <StatusBadge isActive={user.is_active} />
                        <RoleBadge role={user.user_role} />
                    </div>
                    <p className="text-sm text-muted-foreground font-mono">{user.user_id}</p>
                    {user.user_email && (
                        <p className="text-sm text-muted-foreground flex items-center gap-1.5 mt-1">
                            <Mail className="w-3.5 h-3.5" />
                            {user.user_email}
                        </p>
                    )}
                </div>
            </div>
            <Button onClick={onEdit} className="gap-2">
                <Edit className="w-4 h-4" />
                Edit User
            </Button>
        </div>
    );
}

// Stats Cards
function UserStatsCards({ user }: { user: User }) {
    const stats = [
        {
            label: "Teams",
            value: user.teams?.length || 0,
            icon: Users,
            color: "text-blue-400",
            bgColor: "bg-blue-500/10",
        },
        {
            label: "Budget Used",
            value: `$${user.spend.toFixed(2)}`,
            subtitle: user.max_budget ? `of $${user.max_budget.toFixed(2)}` : "No limit",
            icon: DollarSign,
            color: "text-green-400",
            bgColor: "bg-green-500/10",
        },
        {
            label: "Rate Limit",
            value: user.rpm_limit ? `${user.rpm_limit} RPM` : "Unlimited",
            subtitle: user.tpm_limit ? `${user.tpm_limit.toLocaleString()} TPM` : undefined,
            icon: Zap,
            color: "text-yellow-400",
            bgColor: "bg-yellow-500/10",
        },
        {
            label: "Models",
            value: user.models?.length || "All",
            subtitle: user.models?.length ? `${user.models.slice(0, 2).join(", ")}...` : "All models allowed",
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
function TeamsSection({ user }: { user: User }) {
    const { teams, isLoading } = useTeams({});
    const userTeamIds = user.teams || [];

    // Filter to get user's teams
    const userTeams = teams.filter((t) => userTeamIds.includes(t.team_id));

    if (userTeamIds.length === 0) {
        return (
            <Card className="glass-card">
                <CardHeader>
                    <CardTitle className="text-lg">Teams</CardTitle>
                    <CardDescription>Teams this user belongs to</CardDescription>
                </CardHeader>
                <CardContent>
                    <EmptyState
                        icon={<Users className="w-12 h-12" />}
                        title="No teams"
                        description="This user is not a member of any teams"
                        className="py-6"
                    />
                </CardContent>
            </Card>
        );
    }

    return (
        <Card className="glass-card">
            <CardHeader>
                <CardTitle className="text-lg">Teams</CardTitle>
                <CardDescription>{userTeams.length} team{userTeams.length !== 1 ? "s" : ""}</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
                {isLoading ? (
                    <div className="space-y-3">
                        {[1, 2].map((i) => (
                            <Skeleton key={i} className="h-16 rounded-lg" />
                        ))}
                    </div>
                ) : (
                    <AnimatePresence>
                        {userTeams.map((team, i) => (
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
                                                    {team.members?.length || 0} members
                                                </div>
                                            </div>
                                        </div>
                                        <StatusBadge isActive={team.is_active} blocked={team.blocked} size="sm" />
                                    </div>
                                </Link>
                            </motion.div>
                        ))}
                    </AnimatePresence>
                )}
            </CardContent>
        </Card>
    );
}

// API Keys Section
function ApiKeysSection({ userId }: { userId: string }) {
    const { keys, isLoading } = useApiKeys({ userId });

    if (isLoading) {
        return (
            <Card className="glass-card">
                <CardHeader>
                    <CardTitle className="text-lg">API Keys</CardTitle>
                </CardHeader>
                <CardContent>
                    <div className="space-y-3">
                        {[1, 2, 3].map((i) => (
                            <Skeleton key={i} className="h-14 rounded-lg" />
                        ))}
                    </div>
                </CardContent>
            </Card>
        );
    }

    if (keys.length === 0) {
        return (
            <Card className="glass-card">
                <CardHeader>
                    <CardTitle className="text-lg">API Keys</CardTitle>
                    <CardDescription>Keys associated with this user</CardDescription>
                </CardHeader>
                <CardContent>
                    <EmptyState
                        icon={<Key className="w-12 h-12" />}
                        title="No API keys"
                        description="This user has no associated API keys"
                        action={
                            <Link href="/api-keys">
                                <Button variant="outline" size="sm">
                                    Go to API Keys
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
            <CardHeader className="flex flex-row items-center justify-between">
                <div>
                    <CardTitle className="text-lg">API Keys</CardTitle>
                    <CardDescription>{keys.length} key{keys.length !== 1 ? "s" : ""}</CardDescription>
                </div>
                <Link href="/api-keys">
                    <Button variant="outline" size="sm">View All</Button>
                </Link>
            </CardHeader>
            <CardContent className="space-y-3">
                <AnimatePresence>
                    {keys.slice(0, 5).map((key, i) => (
                        <motion.div
                            key={key.id}
                            initial={{ opacity: 0, y: 10 }}
                            animate={{ opacity: 1, y: 0 }}
                            transition={{ delay: i * 0.05 }}
                            className="flex items-center justify-between p-3 rounded-lg bg-secondary/50"
                        >
                            <div className="flex items-center gap-3">
                                <div className="w-8 h-8 rounded-lg bg-yellow-500/10 flex items-center justify-center">
                                    <Key className="w-4 h-4 text-yellow-400" />
                                </div>
                                <div>
                                    <div className="font-medium text-sm">{key.name}</div>
                                    <div className="text-xs text-muted-foreground font-mono">
                                        {key.key_prefix}...
                                    </div>
                                </div>
                            </div>
                            <StatusBadge isActive={key.is_active} blocked={key.blocked} size="sm" />
                        </motion.div>
                    ))}
                </AnimatePresence>
            </CardContent>
        </Card>
    );
}

// Settings Section
function SettingsSection({ user }: { user: User }) {
    return (
        <Card className="glass-card">
            <CardHeader>
                <CardTitle className="text-lg">User Settings</CardTitle>
                <CardDescription>Configuration and limits</CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
                {/* Budget Settings */}
                <div className="space-y-3">
                    <h4 className="text-sm font-medium flex items-center gap-2">
                        <DollarSign className="w-4 h-4 text-green-400" />
                        Budget Configuration
                    </h4>
                    <div className="grid grid-cols-2 gap-4">
                        <div className="p-4 rounded-lg bg-secondary/50">
                            <div className="text-sm text-muted-foreground">Max Budget</div>
                            <div className="text-lg font-semibold">
                                {user.max_budget ? `$${user.max_budget.toFixed(2)}` : "Unlimited"}
                            </div>
                        </div>
                        <div className="p-4 rounded-lg bg-secondary/50">
                            <div className="text-sm text-muted-foreground">Current Spend</div>
                            <div className="text-lg font-semibold">${user.spend.toFixed(2)}</div>
                        </div>
                    </div>
                    {user.max_budget && (
                        <BudgetProgress spent={user.spend} max={user.max_budget} />
                    )}
                </div>

                {/* Rate Limits */}
                <div className="space-y-3">
                    <h4 className="text-sm font-medium flex items-center gap-2">
                        <Zap className="w-4 h-4 text-yellow-400" />
                        Rate Limits
                    </h4>
                    <div className="grid grid-cols-3 gap-4">
                        <div className="p-4 rounded-lg bg-secondary/50">
                            <div className="text-sm text-muted-foreground">RPM</div>
                            <div className="text-lg font-semibold">
                                {user.rpm_limit?.toLocaleString() || "∞"}
                            </div>
                        </div>
                        <div className="p-4 rounded-lg bg-secondary/50">
                            <div className="text-sm text-muted-foreground">TPM</div>
                            <div className="text-lg font-semibold">
                                {user.tpm_limit?.toLocaleString() || "∞"}
                            </div>
                        </div>
                        <div className="p-4 rounded-lg bg-secondary/50">
                            <div className="text-sm text-muted-foreground">Max Parallel</div>
                            <div className="text-lg font-semibold">
                                {user.max_parallel_requests?.toLocaleString() || "∞"}
                            </div>
                        </div>
                    </div>
                </div>

                {/* Allowed Models */}
                <div className="space-y-3">
                    <h4 className="text-sm font-medium flex items-center gap-2">
                        <Key className="w-4 h-4 text-purple-400" />
                        Allowed Models
                    </h4>
                    <div className="flex flex-wrap gap-2">
                        {user.models && user.models.length > 0 ? (
                            user.models.map((model) => (
                                <Badge key={model} variant="secondary">
                                    {model}
                                </Badge>
                            ))
                        ) : (
                            <span className="text-muted-foreground text-sm">All models allowed</span>
                        )}
                    </div>
                </div>

                {/* Metadata */}
                <div className="space-y-3">
                    <h4 className="text-sm font-medium flex items-center gap-2">
                        <Clock className="w-4 h-4 text-blue-400" />
                        Metadata
                    </h4>
                    <div className="grid grid-cols-2 gap-4 text-sm">
                        {user.created_at && (
                            <div>
                                <span className="text-muted-foreground">Created:</span>
                                <span className="ml-2">{new Date(user.created_at).toLocaleDateString()}</span>
                            </div>
                        )}
                        {user.updated_at && (
                            <div>
                                <span className="text-muted-foreground">Updated:</span>
                                <span className="ml-2">{new Date(user.updated_at).toLocaleDateString()}</span>
                            </div>
                        )}
                        {user.organization_id && (
                            <div className="col-span-2">
                                <span className="text-muted-foreground flex items-center gap-1">
                                    <Building2 className="w-3.5 h-3.5" />
                                    Organization:
                                </span>
                                <span className="ml-2 font-mono text-xs">{user.organization_id}</span>
                            </div>
                        )}
                    </div>
                </div>
            </CardContent>
        </Card>
    );
}

// Edit User Dialog
function EditUserDialog({
    open,
    onOpenChange,
    user,
    onSave,
}: {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    user: User;
    onSave: (updates: Partial<CreateUserRequest>) => Promise<void>;
}) {
    const [alias, setAlias] = useState(user.user_alias || "");
    const [email, setEmail] = useState(user.user_email || "");
    const [role, setRole] = useState<UserRole>(user.user_role);
    const [maxBudget, setMaxBudget] = useState(user.max_budget?.toString() || "");
    const [isSaving, setIsSaving] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const handleSave = async () => {
        setIsSaving(true);
        setError(null);

        try {
            await onSave({
                user_alias: alias.trim() || undefined,
                user_email: email.trim() || undefined,
                user_role: role,
                max_budget: maxBudget ? parseFloat(maxBudget) : undefined,
            });
            onOpenChange(false);
        } catch (err) {
            setError(err instanceof Error ? err.message : "Failed to update user");
        } finally {
            setIsSaving(false);
        }
    };

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="sm:max-w-md">
                <DialogHeader>
                    <DialogTitle>Edit User</DialogTitle>
                    <DialogDescription>
                        Update user information and settings.
                    </DialogDescription>
                </DialogHeader>

                <div className="space-y-4">
                    <div className="space-y-2">
                        <Label htmlFor="alias">Display Name</Label>
                        <Input
                            id="alias"
                            value={alias}
                            onChange={(e) => setAlias(e.target.value)}
                            placeholder="e.g., John Doe"
                        />
                    </div>

                    <div className="space-y-2">
                        <Label htmlFor="email">Email Address</Label>
                        <Input
                            id="email"
                            type="email"
                            value={email}
                            onChange={(e) => setEmail(e.target.value)}
                            placeholder="john@example.com"
                        />
                    </div>

                    <div className="grid grid-cols-2 gap-4">
                        <div className="space-y-2">
                            <Label htmlFor="role">Role</Label>
                            <Select value={role} onValueChange={(v) => setRole(v as UserRole)}>
                                <SelectTrigger>
                                    <SelectValue placeholder="Select role" />
                                </SelectTrigger>
                                <SelectContent>
                                    {roleOptions.map((opt) => (
                                        <SelectItem key={opt.value} value={opt.value}>
                                            {opt.label}
                                        </SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                        </div>
                        <div className="space-y-2">
                            <Label htmlFor="budget">Max Budget</Label>
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
                        Cancel
                    </Button>
                    <Button onClick={handleSave} disabled={isSaving}>
                        {isSaving ? (
                            <>
                                <RefreshCw className="w-4 h-4 mr-2 animate-spin" />
                                Saving...
                            </>
                        ) : (
                            "Save Changes"
                        )}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}

// Loading skeleton
function UserDetailSkeleton() {
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
                <Skeleton className="h-10 w-24 rounded-lg" />
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
export default function UserDetailPage() {
    const params = useParams();
    const userId = params.id as string;

    const { user, isLoading, error, refresh, updateUser } = useUserInfo(userId);
    const [editDialogOpen, setEditDialogOpen] = useState(false);
    const [activeTab, setActiveTab] = useState("overview");

    const handleUpdateUser = async (updates: Partial<CreateUserRequest>) => {
        await updateUser(updates);
    };

    if (isLoading) {
        return <UserDetailSkeleton />;
    }

    if (error) {
        return (
            <div className="space-y-6">
                <div className="flex items-center gap-4">
                    <Link href="/users">
                        <Button variant="ghost" size="icon">
                            <ArrowLeft className="w-5 h-5" />
                        </Button>
                    </Link>
                    <h1 className="text-2xl font-bold">User Details</h1>
                </div>
                <ErrorState message={error.message} onRetry={refresh} />
            </div>
        );
    }

    if (!user) {
        return (
            <div className="space-y-6">
                <div className="flex items-center gap-4">
                    <Link href="/users">
                        <Button variant="ghost" size="icon">
                            <ArrowLeft className="w-5 h-5" />
                        </Button>
                    </Link>
                    <h1 className="text-2xl font-bold">User Not Found</h1>
                </div>
                <EmptyState
                    icon={<Users className="w-12 h-12" />}
                    title="User not found"
                    description="The user you're looking for doesn't exist or has been deleted."
                    action={
                        <Link href="/users">
                            <Button>Back to Users</Button>
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
            <UserDetailHeader
                user={user}
                onEdit={() => setEditDialogOpen(true)}
            />

            {/* Stats */}
            <UserStatsCards user={user} />

            {/* Tabs Content */}
            <Tabs value={activeTab} onValueChange={setActiveTab}>
                <TabsList className="w-full md:w-auto">
                    <TabsTrigger value="overview" className="gap-2">
                        <Activity className="w-4 h-4" />
                        Overview
                    </TabsTrigger>
                    <TabsTrigger value="settings" className="gap-2">
                        <Settings className="w-4 h-4" />
                        Settings
                    </TabsTrigger>
                </TabsList>

                <TabsContent value="overview" className="mt-6">
                    <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                        <TeamsSection user={user} />
                        <ApiKeysSection userId={userId} />
                    </div>
                </TabsContent>

                <TabsContent value="settings" className="mt-6">
                    <SettingsSection user={user} />
                </TabsContent>
            </Tabs>

            {/* Edit Dialog */}
            <EditUserDialog
                open={editDialogOpen}
                onOpenChange={setEditDialogOpen}
                user={user}
                onSave={handleUpdateUser}
            />
        </motion.div>
    );
}
