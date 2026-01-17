"use client";

import { useState, useEffect } from "react";
import { motion, AnimatePresence } from "framer-motion";
import Link from "next/link";
import { Card, CardContent } from "@/components/ui/card";
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
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from "@/components/ui/table";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
import {
    Users,
    UserPlus,
    Search,
    RefreshCw,
    AlertCircle,
    ChevronRight,
    MoreVertical,
    Trash2,
    Mail,
    Filter,
} from "lucide-react";
import { useUsers } from "@/hooks";
import type { User, CreateUserRequest, UserRole } from "@/types/api";
import { StatusBadge, RoleBadge, BudgetProgress, PageHeader, EmptyState, ErrorState } from "@/components/shared/common";
import { TableRowSkeleton } from "@/components/ui/skeleton";
import { useI18n } from "@/i18n/locale-provider";

// User role options
const roleOptions: { value: UserRole; labelKey: string }[] = [
    { value: "proxy_admin", labelKey: "role.admin" },
    { value: "proxy_admin_viewer", labelKey: "role.adminViewer" },
    { value: "org_admin", labelKey: "role.orgAdmin" },
    { value: "internal_user", labelKey: "role.internalUser" },
    { value: "internal_user_viewer", labelKey: "role.viewer" },
    { value: "team", labelKey: "role.team" },
    { value: "customer", labelKey: "role.customer" },
];

// Create User Dialog
function CreateUserDialog({
    open,
    onOpenChange,
    onCreate,
}: {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    onCreate: (data: CreateUserRequest) => Promise<User>;
}) {
    const [alias, setAlias] = useState("");
    const [email, setEmail] = useState("");
    const [role, setRole] = useState<UserRole>("internal_user");
    const [maxBudget, setMaxBudget] = useState("");
    const [isCreating, setIsCreating] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const { t } = useI18n();

    const handleCreate = async () => {
        setIsCreating(true);
        setError(null);

        // Parse budget with NaN validation
        const parsedBudget = maxBudget ? parseFloat(maxBudget) : undefined;
        const validBudget = parsedBudget !== undefined && !isNaN(parsedBudget) ? parsedBudget : undefined;

        try {
            await onCreate({
                user_alias: alias.trim() || undefined,
                user_email: email.trim() || undefined,
                user_role: role,
                max_budget: validBudget,
            });
            handleClose();
        } catch (err) {
            setError(err instanceof Error ? err.message : t("dashboard.users.form.error.createFailed"));
        } finally {
            setIsCreating(false);
        }
    };

    const handleClose = () => {
        setAlias("");
        setEmail("");
        setRole("internal_user");
        setMaxBudget("");
        setError(null);
        onOpenChange(false);
    };

    return (
        <Dialog open={open} onOpenChange={handleClose}>
            <DialogContent className="sm:max-w-md">
                <DialogHeader>
                    <DialogTitle>{t("dashboard.users.dialog.create.title")}</DialogTitle>
                    <DialogDescription>
                        {t("dashboard.users.dialog.create.description")}
                    </DialogDescription>
                </DialogHeader>

                <div className="space-y-4">
                    <div className="space-y-2">
                        <Label htmlFor="alias">{t("dashboard.users.form.displayName")}</Label>
                        <Input
                            id="alias"
                            placeholder={t("dashboard.users.form.displayName.placeholder")}
                            value={alias}
                            onChange={(e) => setAlias(e.target.value)}
                        />
                    </div>

                    <div className="space-y-2">
                        <Label htmlFor="email">{t("dashboard.users.form.email")}</Label>
                        <Input
                            id="email"
                            type="email"
                            placeholder={t("dashboard.users.form.email.placeholder")}
                            value={email}
                            onChange={(e) => setEmail(e.target.value)}
                        />
                    </div>

                    <div className="grid grid-cols-2 gap-4">
                        <div className="space-y-2">
                            <Label htmlFor="role">{t("dashboard.users.form.role")}</Label>
                            <Select value={role} onValueChange={(v) => setRole(v as UserRole)}>
                                <SelectTrigger>
                                    <SelectValue placeholder={t("dashboard.users.form.role.placeholder")} />
                                </SelectTrigger>
                                <SelectContent>
                                    {roleOptions.map((opt) => (
                                        <SelectItem key={opt.value} value={opt.value}>
                                            {t(opt.labelKey)}
                                        </SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                        </div>
                        <div className="space-y-2">
                            <Label htmlFor="budget">{t("dashboard.users.form.maxBudget")}</Label>
                            <div className="relative">
                                <span className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground">$</span>
                                <Input
                                    id="budget"
                                    type="number"
                                    placeholder={t("dashboard.users.form.maxBudget.placeholder")}
                                    value={maxBudget}
                                    onChange={(e) => setMaxBudget(e.target.value)}
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
                    <Button variant="ghost" onClick={handleClose}>
                        {t("common.cancel")}
                    </Button>
                    <Button onClick={handleCreate} disabled={isCreating}>
                        {isCreating ? (
                            <>
                                <RefreshCw className="w-4 h-4 mr-2 animate-spin" />
                                {t("dashboard.users.form.submit.creating")}
                            </>
                        ) : (
                            t("dashboard.users.form.submit.create")
                        )}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}

// User Row Component
function UserRow({
    user,
    onDelete,
    index,
}: {
    user: User;
    onDelete: (userId: string) => void;
    index: number;
}) {
    const [showMenu, setShowMenu] = useState(false);
    const { t } = useI18n();

    return (
        <motion.tr
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -10 }}
            transition={{ delay: index * 0.03 }}
            className="group hover:bg-secondary/50 transition-colors"
        >
            <TableCell className="py-4">
                <div className="flex items-center gap-3">
                    <div className="w-10 h-10 rounded-full bg-gradient-to-br from-primary/20 to-primary/5 flex items-center justify-center border border-primary/20">
                        <Users className="w-5 h-5 text-primary" />
                    </div>
                    <div>
                        <div className="font-medium">{user.user_alias || t("dashboard.users.row.unnamed")}</div>
                        <div className="text-xs text-muted-foreground font-mono">
                            {user.user_id.slice(0, 16)}...
                        </div>
                    </div>
                </div>
            </TableCell>
            <TableCell>
                {user.user_email ? (
                    <div className="flex items-center gap-2 text-sm">
                        <Mail className="w-3.5 h-3.5 text-muted-foreground" />
                        <span className="text-muted-foreground">{user.user_email}</span>
                    </div>
                ) : (
                    <span className="text-muted-foreground">—</span>
                )}
            </TableCell>
            <TableCell>
                <RoleBadge role={user.user_role} />
            </TableCell>
            <TableCell>
                <div className="flex items-center gap-2">
                    {user.teams && user.teams.length > 0 ? (
                        <Badge variant="secondary" className="gap-1">
                            <Users className="w-3 h-3" />
                            {t("dashboard.organizationDetail.teams.count", {
                                count: user.teams.length,
                                item: t("dashboard.organizationDetail.stats.teams"),
                            })}
                        </Badge>
                    ) : (
                        <span className="text-muted-foreground text-sm">{t("dashboard.users.row.noTeams")}</span>
                    )}
                </div>
            </TableCell>
            <TableCell>
                <BudgetProgress spent={user.spend} max={user.max_budget} showLabel={false} size="sm" />
            </TableCell>
            <TableCell>
                <StatusBadge isActive={user.is_active} size="sm" />
            </TableCell>
            <TableCell>
                <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                    <Link href={`/users/${user.user_id}`}>
                        <Button variant="ghost" size="icon" className="h-8 w-8">
                            <ChevronRight className="w-4 h-4" />
                        </Button>
                    </Link>
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
                                        className="absolute right-0 top-full mt-1 w-36 bg-popover border border-border rounded-lg shadow-lg z-50 py-1"
                                    >
                                        <button
                                            onClick={() => {
                                                onDelete(user.user_id);
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
            </TableCell>
        </motion.tr>
    );
}

// Table Skeleton
function UsersTableSkeleton() {
    const { t } = useI18n();
    return (
        <Card className="glass-card">
            <CardContent className="p-0">
                <Table>
                    <TableHeader>
                        <TableRow className="hover:bg-transparent">
                            <TableHead className="text-xs uppercase text-muted-foreground">{t("dashboard.users.table.user")}</TableHead>
                            <TableHead className="text-xs uppercase text-muted-foreground">{t("dashboard.users.table.email")}</TableHead>
                            <TableHead className="text-xs uppercase text-muted-foreground">{t("dashboard.users.table.role")}</TableHead>
                            <TableHead className="text-xs uppercase text-muted-foreground">{t("dashboard.users.table.teams")}</TableHead>
                            <TableHead className="text-xs uppercase text-muted-foreground">{t("dashboard.users.table.budget")}</TableHead>
                            <TableHead className="text-xs uppercase text-muted-foreground">{t("dashboard.users.table.status")}</TableHead>
                            <TableHead className="w-24"></TableHead>
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {[1, 2, 3, 4, 5].map((i) => (
                            <TableRowSkeleton key={i} columns={7} />
                        ))}
                    </TableBody>
                </Table>
            </CardContent>
        </Card>
    );
}

export default function UsersPage() {
    const { t } = useI18n();
    const [createDialogOpen, setCreateDialogOpen] = useState(false);
    const [searchQuery, setSearchQuery] = useState("");
    const [debouncedSearch, setDebouncedSearch] = useState("");
    const [roleFilter, setRoleFilter] = useState<string>("all");

    // Debounce search query to reduce API calls
    useEffect(() => {
        const timer = setTimeout(() => {
            setDebouncedSearch(searchQuery);
        }, 300);
        return () => clearTimeout(timer);
    }, [searchQuery]);

    // Pass search and role filter to server for proper filtering across all pages
    const {
        users,
        total,
        isLoading,
        error,
        refresh,
        createUser,
        deleteUser,
    } = useUsers({
        search: debouncedSearch || undefined,
        role: roleFilter !== "all" ? roleFilter : undefined,
    });

    return (
        <div className="space-y-6">
            {/* Header */}
            <PageHeader
                title={t("dashboard.users.title")}
                description={t("dashboard.users.description")}
                action={
                    <Button
                        className="gap-2"
                        onClick={() => setCreateDialogOpen(true)}
                        data-testid="create-user-button"
                    >
                        <UserPlus className="w-4 h-4" />
                        {t("dashboard.users.action.create")}
                    </Button>
                }
            />

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
                        placeholder={t("dashboard.users.search.placeholder")}
                        value={searchQuery}
                        onChange={(e) => setSearchQuery(e.target.value)}
                        className="pl-9"
                        data-testid="search-input"
                    />
                </div>
                <div className="flex items-center gap-2">
                    <Select value={roleFilter} onValueChange={setRoleFilter}>
                        <SelectTrigger className="w-40">
                            <Filter className="w-4 h-4 mr-2 text-muted-foreground" />
                            <SelectValue placeholder={t("dashboard.users.filter.allRoles")} />
                        </SelectTrigger>
                        <SelectContent>
                            <SelectItem value="all">{t("dashboard.users.filter.allRoles")}</SelectItem>
                            {roleOptions.map((opt) => (
                                <SelectItem key={opt.value} value={opt.value}>
                                    {t(opt.labelKey)}
                                </SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                    <Button variant="ghost" size="icon" onClick={refresh} title={t("common.refresh")}>
                        <RefreshCw className="w-4 h-4" />
                    </Button>
                </div>
            </motion.div>

            {/* Error State */}
            {error && (
                <ErrorState message={error.message} onRetry={refresh} />
            )}

            {/* Users Table */}
            {isLoading ? (
                <UsersTableSkeleton />
            ) : users.length === 0 ? (
                <Card className="glass-card">
                    <EmptyState
                        icon={<Users className="w-12 h-12" />}
                        title={debouncedSearch || roleFilter !== "all" ? t("dashboard.users.empty.noMatch") : t("dashboard.users.empty.none")}
                        description={
                            debouncedSearch || roleFilter !== "all"
                                ? t("dashboard.users.empty.tryAdjust")
                                : t("dashboard.users.empty.createFirst")
                        }
                        action={
                            !debouncedSearch && roleFilter === "all" && (
                                <Button onClick={() => setCreateDialogOpen(true)}>
                                    <UserPlus className="w-4 h-4 mr-2" />
                                    {t("dashboard.users.action.create")}
                                </Button>
                            )
                        }
                    />
                </Card>
            ) : (
                <motion.div
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                >
                    <Card className="glass-card">
                        <CardContent className="p-0">
                            <Table>
                                <TableHeader>
                                    <TableRow className="hover:bg-transparent">
                                        <TableHead className="text-xs uppercase text-muted-foreground font-medium">{t("dashboard.users.table.user")}</TableHead>
                                        <TableHead className="text-xs uppercase text-muted-foreground font-medium">{t("dashboard.users.table.email")}</TableHead>
                                        <TableHead className="text-xs uppercase text-muted-foreground font-medium">{t("dashboard.users.table.role")}</TableHead>
                                        <TableHead className="text-xs uppercase text-muted-foreground font-medium">{t("dashboard.users.table.teams")}</TableHead>
                                        <TableHead className="text-xs uppercase text-muted-foreground font-medium">{t("dashboard.users.table.budget")}</TableHead>
                                        <TableHead className="text-xs uppercase text-muted-foreground font-medium">{t("dashboard.users.table.status")}</TableHead>
                                        <TableHead className="w-24"></TableHead>
                                    </TableRow>
                                </TableHeader>
                                <TableBody>
                                    <AnimatePresence mode="popLayout">
                                        {users.map((user, index) => (
                                            <UserRow
                                                key={user.user_id}
                                                user={user}
                                                onDelete={deleteUser}
                                                index={index}
                                            />
                                        ))}
                                    </AnimatePresence>
                                </TableBody>
                            </Table>
                        </CardContent>
                    </Card>

                    {/* Pagination info */}
                    <div className="flex items-center justify-between text-sm text-muted-foreground mt-4">
                        <span>
                            {t("pagination.showingCountOfTotal", { count: users.length, total, item: t("dashboard.users.pagination.item") })}
                        </span>
                    </div>
                </motion.div>
            )}

            {/* Create User Dialog */}
            <CreateUserDialog
                open={createDialogOpen}
                onOpenChange={setCreateDialogOpen}
                onCreate={createUser}
            />
        </div>
    );
}
