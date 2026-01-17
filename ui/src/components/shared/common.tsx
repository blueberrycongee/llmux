"use client";

import { motion } from "framer-motion";
import { Shield, ShieldOff, AlertTriangle, Clock, CheckCircle2, XCircle } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { useI18n } from "@/i18n/locale-provider";

interface StatusBadgeProps {
    isActive: boolean;
    blocked?: boolean;
    className?: string;
    size?: "sm" | "md";
}

export function StatusBadge({ isActive, blocked = false, className, size = "md" }: StatusBadgeProps) {
    const { t } = useI18n();
    const iconSize = size === "sm" ? "w-3 h-3" : "w-3.5 h-3.5";

    if (blocked) {
        return (
            <Badge variant="error" className={cn("gap-1", className)}>
                <ShieldOff className={iconSize} />
                <span>{t("status.blocked")}</span>
            </Badge>
        );
    }

    if (isActive) {
        return (
            <Badge variant="success" className={cn("gap-1", className)}>
                <Shield className={iconSize} />
                <span>{t("status.active")}</span>
            </Badge>
        );
    }

    return (
        <Badge variant="secondary" className={cn("gap-1", className)}>
            <Clock className={iconSize} />
            <span>{t("status.inactive")}</span>
        </Badge>
    );
}

interface RoleBadgeProps {
    role: string;
    className?: string;
}

const roleConfig: Record<string, { labelKey: string; variant: "default" | "info" | "warning" | "success" }> = {
    proxy_admin: { labelKey: "role.admin", variant: "warning" },
    proxy_admin_viewer: { labelKey: "role.adminViewer", variant: "info" },
    org_admin: { labelKey: "role.orgAdmin", variant: "warning" },
    internal_user: { labelKey: "role.internalUser", variant: "default" },
    internal_user_viewer: { labelKey: "role.viewer", variant: "info" },
    team: { labelKey: "role.team", variant: "success" },
    customer: { labelKey: "role.customer", variant: "default" },
};

export function RoleBadge({ role, className }: RoleBadgeProps) {
    const { t } = useI18n();
    const config = roleConfig[role] || { labelKey: role, variant: "default" as const };

    return (
        <Badge variant={config.variant} className={className}>
            {t(config.labelKey)}
        </Badge>
    );
}

interface BudgetProgressProps {
    spent: number;
    max?: number;
    showLabel?: boolean;
    size?: "sm" | "md";
    className?: string;
}

export function BudgetProgress({
    spent,
    max,
    showLabel = true,
    size = "md",
    className
}: BudgetProgressProps) {
    const { t } = useI18n();
    if (!max) {
        return (
            <div className={cn("text-sm", className)}>
                <span className="text-muted-foreground">{t("budget.spent")}: </span>
                <span className="font-medium">${spent.toFixed(2)}</span>
                <span className="text-muted-foreground"> / {t("budget.noLimit")}</span>
            </div>
        );
    }

    const percentage = Math.min((spent / max) * 100, 100);
    const isNearLimit = percentage >= 80;
    const isOverLimit = percentage >= 100;
    const barHeight = size === "sm" ? "h-1.5" : "h-2";

    return (
        <div className={cn("space-y-1", className)}>
            {showLabel && (
                <div className="flex items-center justify-between text-sm">
                    <span className="text-muted-foreground">{t("budget.budget")}</span>
                    <span className={cn(
                        isOverLimit && "text-red-400",
                        isNearLimit && !isOverLimit && "text-yellow-400"
                    )}>
                        ${spent.toFixed(2)} / ${max.toFixed(2)}
                    </span>
                </div>
            )}
            <div className={cn("bg-secondary rounded-full overflow-hidden", barHeight)}>
                <motion.div
                    className={cn(
                        "h-full rounded-full",
                        isOverLimit ? "bg-red-500" : isNearLimit ? "bg-yellow-500" : "bg-primary"
                    )}
                    initial={{ width: 0 }}
                    animate={{ width: `${percentage}%` }}
                    transition={{ duration: 0.5, ease: "easeOut" }}
                />
            </div>
        </div>
    );
}

interface EmptyStateProps {
    icon: React.ReactNode;
    title: string;
    description: string;
    action?: React.ReactNode;
    className?: string;
}

export function EmptyState({ icon, title, description, action, className }: EmptyStateProps) {
    return (
        <motion.div
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            className={cn("text-center py-12", className)}
        >
            <div className="text-muted-foreground mb-4 flex justify-center">
                {icon}
            </div>
            <h3 className="text-lg font-medium mb-2">{title}</h3>
            <p className="text-muted-foreground mb-4 max-w-md mx-auto">{description}</p>
            {action}
        </motion.div>
    );
}

interface ErrorStateProps {
    message: string;
    onRetry?: () => void;
    className?: string;
}

export function ErrorState({ message, onRetry, className }: ErrorStateProps) {
    const { t } = useI18n();
    return (
        <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            className={cn(
                "flex items-center gap-3 p-4 bg-red-500/10 border border-red-500/20 rounded-lg",
                className
            )}
        >
            <AlertTriangle className="w-5 h-5 text-red-400 shrink-0" />
            <p className="text-red-400 flex-1">{message}</p>
            {onRetry && (
                <button
                    onClick={onRetry}
                    className="text-sm text-red-400 hover:text-red-300 underline underline-offset-2"
                >
                    {t("common.retry")}
                </button>
            )}
        </motion.div>
    );
}

interface PageHeaderProps {
    title: string;
    description?: string;
    action?: React.ReactNode;
    backHref?: string;
    className?: string;
}

export function PageHeader({ title, description, action, className }: PageHeaderProps) {
    return (
        <motion.div
            initial={{ opacity: 0, y: -20 }}
            animate={{ opacity: 1, y: 0 }}
            className={cn("flex items-center justify-between", className)}
        >
            <div>
                <h1 className="text-3xl font-bold tracking-tight">{title}</h1>
                {description && (
                    <p className="text-muted-foreground mt-1">{description}</p>
                )}
            </div>
            {action}
        </motion.div>
    );
}
