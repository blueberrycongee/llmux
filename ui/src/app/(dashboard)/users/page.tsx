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
import { Skeleton, TableRowSkeleton } from "@/components/ui/skeleton";

// User role options
const roleOptions: { value: UserRole; label: string }[] = [
    { value: "proxy_admin", label: "Proxy Admin" },
    { value: "proxy_admin_viewer", label: "Admin Viewer" },
    { value: "org_admin", label: "Org Admin" },
    { value: "internal_user", label: "Internal User" },
    { value: "internal_user_viewer", label: "Internal Viewer" },
    { value: "team", label: "Team" },
    { value: "customer", label: "Customer" },
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
            setError(err instanceof Error ? err.message : "Failed to create user");
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
                    <DialogTitle>Create New User</DialogTitle>
                    <DialogDescription>
                        Add a new user to the system.
                    </DialogDescription>
                </DialogHeader>

                <div className="space-y-4">
                    <div className="space-y-2">
                        <Label htmlFor="alias">Display Name</Label>
                        <Input
                            id="alias"
                            placeholder="e.g., John Doe"
                            value={alias}
                            onChange={(e) => setAlias(e.target.value)}
                        />
                    </div>

                    <div className="space-y-2">
                        <Label htmlFor="email">Email Address</Label>
                        <Input
                            id="email"
                            type="email"
                            placeholder="john@example.com"
                            value={email}
                            onChange={(e) => setEmail(e.target.value)}
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
                                    placeholder="1000"
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
                        Cancel
                    </Button>
                    <Button onClick={handleCreate} disabled={isCreating}>
                        {isCreating ? (
                            <>
                                <RefreshCw className="w-4 h-4 mr-2 animate-spin" />
                                Creating...
                            </>
                        ) : (
                            "Create User"
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
                        <div className="font-medium">{user.user_alias || "Unnamed User"}</div>
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
                            {user.teams.length} team{user.teams.length !== 1 ? "s" : ""}
                        </Badge>
                    ) : (
                        <span className="text-muted-foreground text-sm">No teams</span>
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
                                            Delete
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
    return (
        <Card className="glass-card">
            <CardContent className="p-0">
                <Table>
                    <TableHeader>
                        <TableRow className="hover:bg-transparent">
                            <TableHead className="text-xs uppercase text-muted-foreground">User</TableHead>
                            <TableHead className="text-xs uppercase text-muted-foreground">Email</TableHead>
                            <TableHead className="text-xs uppercase text-muted-foreground">Role</TableHead>
                            <TableHead className="text-xs uppercase text-muted-foreground">Teams</TableHead>
                            <TableHead className="text-xs uppercase text-muted-foreground">Budget</TableHead>
                            <TableHead className="text-xs uppercase text-muted-foreground">Status</TableHead>
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
                title="Users"
                description="Manage system users and their permissions."
                action={
                    <Button
                        className="gap-2"
                        onClick={() => setCreateDialogOpen(true)}
                        data-testid="create-user-button"
                    >
                        <UserPlus className="w-4 h-4" />
                        Create User
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
                        placeholder="Search users..."
                        value={searchQuery}
                        onChange={(e) => setSearchQuery(e.target.value)}
                        className="pl-9"
                    />
                </div>
                <div className="flex items-center gap-2">
                    <Select value={roleFilter} onValueChange={setRoleFilter}>
                        <SelectTrigger className="w-40">
                            <Filter className="w-4 h-4 mr-2 text-muted-foreground" />
                            <SelectValue placeholder="All Roles" />
                        </SelectTrigger>
                        <SelectContent>
                            <SelectItem value="all">All Roles</SelectItem>
                            {roleOptions.map((opt) => (
                                <SelectItem key={opt.value} value={opt.value}>
                                    {opt.label}
                                </SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                    <Button variant="ghost" size="icon" onClick={refresh} title="Refresh">
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
                        title={debouncedSearch || roleFilter !== "all" ? "No matching users" : "No users yet"}
                        description={
                            debouncedSearch || roleFilter !== "all"
                                ? "Try adjusting your search or filter"
                                : "Create your first user to get started"
                        }
                        action={
                            !debouncedSearch && roleFilter === "all" && (
                                <Button onClick={() => setCreateDialogOpen(true)}>
                                    <UserPlus className="w-4 h-4 mr-2" />
                                    Create User
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
                                        <TableHead className="text-xs uppercase text-muted-foreground font-medium">User</TableHead>
                                        <TableHead className="text-xs uppercase text-muted-foreground font-medium">Email</TableHead>
                                        <TableHead className="text-xs uppercase text-muted-foreground font-medium">Role</TableHead>
                                        <TableHead className="text-xs uppercase text-muted-foreground font-medium">Teams</TableHead>
                                        <TableHead className="text-xs uppercase text-muted-foreground font-medium">Budget</TableHead>
                                        <TableHead className="text-xs uppercase text-muted-foreground font-medium">Status</TableHead>
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
                            Showing {users.length} of {total} user{total !== 1 ? "s" : ""}
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
