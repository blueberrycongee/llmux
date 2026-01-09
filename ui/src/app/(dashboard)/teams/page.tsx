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
} from "@/components/ui/dialog";
import {
    Plus,
    Users,
    MoreVertical,
    Shield,
    ShieldOff,
    Trash2,
    DollarSign,
    Key,
    RefreshCw,
    AlertCircle,
    Search,
    ChevronRight,
} from "lucide-react";
import { useTeams } from "@/hooks/use-teams";
import type { Team, CreateTeamRequest } from "@/types/api";
import Link from "next/link";

// Skeleton component for loading state
function TeamCardSkeleton() {
    return (
        <Card className="glass-card">
            <CardContent className="p-6">
                <div className="flex items-start justify-between mb-4">
                    <div className="flex items-center gap-3">
                        <div className="w-12 h-12 bg-muted animate-pulse rounded-lg" />
                        <div>
                            <div className="h-5 w-32 bg-muted animate-pulse rounded mb-2" />
                            <div className="h-3 w-20 bg-muted animate-pulse rounded" />
                        </div>
                    </div>
                    <div className="h-6 w-16 bg-muted animate-pulse rounded-full" />
                </div>
                <div className="space-y-3">
                    <div className="h-4 w-full bg-muted animate-pulse rounded" />
                    <div className="h-4 w-3/4 bg-muted animate-pulse rounded" />
                </div>
            </CardContent>
        </Card>
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
        return (
            <div className="text-sm">
                <span className="text-muted-foreground">Spent: </span>
                <span className="font-medium">${spent.toFixed(2)}</span>
                <span className="text-muted-foreground"> / No limit</span>
            </div>
        );
    }

    const percentage = Math.min((spent / max) * 100, 100);
    const isNearLimit = percentage >= 80;
    const isOverLimit = percentage >= 100;

    return (
        <div className="space-y-1">
            <div className="flex items-center justify-between text-sm">
                <span className="text-muted-foreground">Budget</span>
                <span className={isOverLimit ? "text-red-400" : isNearLimit ? "text-yellow-400" : ""}>
                    ${spent.toFixed(2)} / ${max.toFixed(2)}
                </span>
            </div>
            <div className="h-2 bg-secondary rounded-full overflow-hidden">
                <div
                    className={`h-full rounded-full transition-all ${isOverLimit ? "bg-red-500" : isNearLimit ? "bg-yellow-500" : "bg-primary"
                        }`}
                    style={{ width: `${percentage}%` }}
                />
            </div>
        </div>
    );
}

// Team card component
function TeamCard({
    team,
    onBlock,
    onUnblock,
    onDelete,
}: {
    team: Team;
    onBlock: (teamId: string) => void;
    onUnblock: (teamId: string) => void;
    onDelete: (teamId: string) => void;
}) {
    const [showMenu, setShowMenu] = useState(false);

    return (
        <motion.div
            initial={{ opacity: 0, y: 20, scale: 0.95 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: -20, scale: 0.95 }}
            data-testid={`team-card-${team.team_id}`}
        >
            <Card className="glass-card group hover:shadow-lg transition-all duration-300">
                <CardContent className="p-6">
                    {/* Header */}
                    <div className="flex items-start justify-between mb-4">
                        <div className="flex items-center gap-3">
                            <div className="w-12 h-12 rounded-lg bg-primary/10 flex items-center justify-center">
                                <Users className="w-6 h-6 text-primary" />
                            </div>
                            <div>
                                <h3 className="font-semibold text-lg" data-testid={`team-name-${team.team_id}`}>
                                    {team.team_alias || team.team_id}
                                </h3>
                                <p className="text-xs text-muted-foreground font-mono">
                                    {team.team_id}
                                </p>
                            </div>
                        </div>

                        <div className="flex items-center gap-2">
                            <StatusBadge isActive={team.is_active} blocked={team.blocked} />

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
                                                {team.blocked ? (
                                                    <button
                                                        onClick={() => {
                                                            onUnblock(team.team_id);
                                                            setShowMenu(false);
                                                        }}
                                                        className="w-full flex items-center gap-2 px-3 py-2 text-sm hover:bg-secondary transition-colors text-green-400"
                                                    >
                                                        <Shield className="w-4 h-4" />
                                                        Unblock
                                                    </button>
                                                ) : (
                                                    <button
                                                        onClick={() => {
                                                            onBlock(team.team_id);
                                                            setShowMenu(false);
                                                        }}
                                                        className="w-full flex items-center gap-2 px-3 py-2 text-sm hover:bg-secondary transition-colors text-yellow-400"
                                                    >
                                                        <ShieldOff className="w-4 h-4" />
                                                        Block
                                                    </button>
                                                )}
                                                <div className="my-1 border-t border-border" />
                                                <button
                                                    onClick={() => {
                                                        onDelete(team.team_id);
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
                    </div>

                    {/* Stats */}
                    <div className="grid grid-cols-2 gap-4 mb-4">
                        <div className="flex items-center gap-2 text-sm">
                            <Users className="w-4 h-4 text-muted-foreground" />
                            <span className="text-muted-foreground">Members:</span>
                            <span className="font-medium">{team.members?.length || 0}</span>
                        </div>
                        <div className="flex items-center gap-2 text-sm">
                            <Key className="w-4 h-4 text-muted-foreground" />
                            <span className="text-muted-foreground">Rate:</span>
                            <span className="font-medium">
                                {team.rpm_limit ? `${team.rpm_limit} RPM` : "No limit"}
                            </span>
                        </div>
                    </div>

                    {/* Budget */}
                    <BudgetProgress spent={team.spend} max={team.max_budget} />

                    {/* View Details Link */}
                    <Link
                        href={`/teams/${team.team_id}`}
                        className="mt-4 flex items-center justify-between p-3 -mx-3 rounded-lg hover:bg-secondary/50 transition-colors group/link"
                    >
                        <span className="text-sm font-medium text-muted-foreground group-hover/link:text-foreground">
                            View Details
                        </span>
                        <ChevronRight className="w-4 h-4 text-muted-foreground group-hover/link:text-foreground group-hover/link:translate-x-1 transition-all" />
                    </Link>
                </CardContent>
            </Card>
        </motion.div>
    );
}

// Create team dialog component
function CreateTeamDialog({
    open,
    onOpenChange,
    onCreate,
}: {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    onCreate: (data: CreateTeamRequest) => Promise<Team>;
}) {
    const [name, setName] = useState("");
    const [maxBudget, setMaxBudget] = useState("");
    const [rpmLimit, setRpmLimit] = useState("");
    const [isCreating, setIsCreating] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const handleCreate = async () => {
        if (!name.trim()) {
            setError("Team name is required");
            return;
        }

        setIsCreating(true);
        setError(null);

        try {
            await onCreate({
                team_alias: name.trim(),
                max_budget: maxBudget ? parseFloat(maxBudget) : undefined,
                rpm_limit: rpmLimit ? parseInt(rpmLimit) : undefined,
            });
            handleClose();
        } catch (err) {
            setError(err instanceof Error ? err.message : "Failed to create team");
        } finally {
            setIsCreating(false);
        }
    };

    const handleClose = () => {
        setName("");
        setMaxBudget("");
        setRpmLimit("");
        setError(null);
        onOpenChange(false);
    };

    return (
        <Dialog open={open} onOpenChange={handleClose}>
            <DialogContent className="sm:max-w-md">
                <DialogHeader>
                    <DialogTitle>Create New Team</DialogTitle>
                    <DialogDescription>
                        Create a team to organize users and manage access.
                    </DialogDescription>
                </DialogHeader>

                <div className="space-y-4">
                    <div className="space-y-2">
                        <Label htmlFor="name">Team Name</Label>
                        <Input
                            id="name"
                            placeholder="e.g., Engineering"
                            value={name}
                            onChange={(e) => setName(e.target.value)}
                            data-testid="team-name-input"
                        />
                    </div>

                    <div className="grid grid-cols-2 gap-4">
                        <div className="space-y-2">
                            <Label htmlFor="budget">Max Budget</Label>
                            <div className="relative">
                                <span className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground">
                                    $
                                </span>
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

                        <div className="space-y-2">
                            <Label htmlFor="rpm">Rate Limit (RPM)</Label>
                            <Input
                                id="rpm"
                                type="number"
                                placeholder="100"
                                value={rpmLimit}
                                onChange={(e) => setRpmLimit(e.target.value)}
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
                        Cancel
                    </Button>
                    <Button
                        onClick={handleCreate}
                        disabled={isCreating}
                        data-testid="create-team-submit"
                    >
                        {isCreating ? (
                            <>
                                <RefreshCw className="w-4 h-4 mr-2 animate-spin" />
                                Creating...
                            </>
                        ) : (
                            "Create Team"
                        )}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}

export default function TeamsPage() {
    const [createDialogOpen, setCreateDialogOpen] = useState(false);
    const [searchQuery, setSearchQuery] = useState("");

    const {
        teams,
        total,
        isLoading,
        error,
        refresh,
        createTeam,
        deleteTeam,
        blockTeam,
        unblockTeam,
    } = useTeams();

    // Filter teams by search query
    const filteredTeams = teams.filter(
        (team) =>
            (team.team_alias?.toLowerCase() || "").includes(searchQuery.toLowerCase()) ||
            team.team_id.toLowerCase().includes(searchQuery.toLowerCase())
    );

    return (
        <div className="space-y-6">
            {/* Header */}
            <motion.div
                initial={{ opacity: 0, y: -20 }}
                animate={{ opacity: 1, y: 0 }}
                className="flex items-center justify-between"
            >
                <div>
                    <h1 className="text-3xl font-bold tracking-tight">Teams</h1>
                    <p className="text-muted-foreground">
                        Manage teams and member access.
                    </p>
                </div>
                <Button
                    className="gap-2"
                    onClick={() => setCreateDialogOpen(true)}
                    data-testid="create-team-button"
                >
                    <Plus className="w-4 h-4" />
                    Create Team
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
                        placeholder="Search teams..."
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
                    <p className="text-red-400">Failed to load teams: {error.message}</p>
                    <Button variant="ghost" size="sm" onClick={refresh} className="ml-auto">
                        Retry
                    </Button>
                </motion.div>
            )}

            {/* Teams Grid */}
            {isLoading ? (
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4" data-testid="loading-skeleton">
                    {[1, 2, 3, 4, 5, 6].map((i) => (
                        <TeamCardSkeleton key={i} />
                    ))}
                </div>
            ) : filteredTeams.length === 0 ? (
                <motion.div
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    className="text-center py-12"
                    data-testid="empty-state"
                >
                    <Users className="w-12 h-12 text-muted-foreground mx-auto mb-4" />
                    <h3 className="text-lg font-medium mb-2">
                        {searchQuery ? `No teams matching "${searchQuery}"` : "No teams yet"}
                    </h3>
                    <p className="text-muted-foreground mb-4">
                        {searchQuery
                            ? "Try adjusting your search query"
                            : "Create your first team to get started"}
                    </p>
                    {!searchQuery && (
                        <Button onClick={() => setCreateDialogOpen(true)}>
                            <Plus className="w-4 h-4 mr-2" />
                            Create Team
                        </Button>
                    )}
                </motion.div>
            ) : (
                <motion.div
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4"
                >
                    <AnimatePresence mode="popLayout">
                        {filteredTeams.map((team) => (
                            <TeamCard
                                key={team.team_id}
                                team={team}
                                onBlock={blockTeam}
                                onUnblock={unblockTeam}
                                onDelete={deleteTeam}
                            />
                        ))}
                    </AnimatePresence>
                </motion.div>
            )}

            {/* Create Team Dialog */}
            <CreateTeamDialog
                open={createDialogOpen}
                onOpenChange={setCreateDialogOpen}
                onCreate={createTeam}
            />
        </div>
    );
}
