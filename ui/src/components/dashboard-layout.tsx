"use client";

import { useState, useEffect } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { usePathname } from "next/navigation";
import Link from "next/link";
import {
    LayoutDashboard,
    Key,
    Users,
    Building2,
    Shield,
    FileText,
    Settings,
    GitBranch,
    Presentation,
    Network,
    BrainCircuit,
    ChevronLeft,
    Moon,
    Sun,
    LogOut,
} from "lucide-react";
import { useTheme } from "next-themes";
import { Button } from "@/components/ui/button";
import { useI18n } from "@/i18n/locale-provider";

const navigation = [
    { nameKey: "nav.overview", fallback: "Overview", href: "/", icon: LayoutDashboard },
    { nameKey: "nav.demoCenter", fallback: "Demo Center", href: "/demo-center", icon: Presentation },
    { nameKey: "nav.gatewayVisualizer", fallback: "Gateway Visualizer", href: "/gateway-visualizer", icon: Network },
    { nameKey: "nav.agentRouter", fallback: "Agent Router", href: "/agent-router", icon: BrainCircuit },
    { nameKey: "nav.routeMemory", fallback: "Route Memory", href: "/route-memory", icon: BrainCircuit },
    { nameKey: "nav.agentManagement", fallback: "Agent Management", href: "/agent-management", icon: Users },
    { nameKey: "nav.agentTeams", fallback: "Agent Teams", href: "/agent-teams", icon: GitBranch },
    { nameKey: "nav.toolMarketplace", fallback: "Tool Marketplace", href: "/tool-marketplace", icon: Network },
    { nameKey: "nav.agentTown", fallback: "Agent Town", href: "/agent-town", icon: GitBranch },
    { nameKey: "nav.apiKeys", fallback: "API Keys", href: "/api-keys", icon: Key },
    { nameKey: "nav.teams", fallback: "Teams", href: "/teams", icon: Users },
    { nameKey: "nav.organizations", fallback: "Organizations", href: "/organizations", icon: Building2 },
    { nameKey: "nav.users", fallback: "Users", href: "/users", icon: Shield },
    { nameKey: "nav.auditLogs", fallback: "Audit Logs", href: "/audit-logs", icon: FileText },
    { nameKey: "nav.settings", fallback: "Settings", href: "/settings", icon: Settings },
];

export function DashboardLayout({ children }: { children: React.ReactNode }) {
    const [collapsed, setCollapsed] = useState(false);
    const [mounted, setMounted] = useState(false);
    const pathname = usePathname();
    const { theme, setTheme } = useTheme();
    const { t } = useI18n();

    useEffect(() => {
        setMounted(true);
    }, []);

    if (!mounted) {
        return null;
    }

    return (
        <div className="flex h-screen bg-background overflow-hidden">
            <motion.aside
                className="glass border-r flex flex-col"
                initial={false}
                animate={{
                    width: collapsed ? 64 : 240,
                }}
                transition={{ type: "spring", stiffness: 300, damping: 30 }}
            >
                <div className="h-14 flex items-center justify-between px-4 border-b border-border/50">
                    <AnimatePresence mode="wait">
                        {!collapsed && (
                            <motion.h1
                                key="expanded"
                                initial={{ opacity: 0 }}
                                animate={{ opacity: 1 }}
                                exit={{ opacity: 0 }}
                                className="text-lg font-bold tracking-tight"
                            >
                                LLMux
                            </motion.h1>
                        )}
                    </AnimatePresence>
                    <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => setCollapsed(!collapsed)}
                        className="h-8 w-8"
                    >
                        <motion.div
                            animate={{ rotate: collapsed ? 180 : 0 }}
                            transition={{ type: "spring", stiffness: 400, damping: 30 }}
                        >
                            <ChevronLeft className="w-4 h-4" />
                        </motion.div>
                    </Button>
                </div>

                <nav className="flex-1 px-2 py-4 space-y-1 overflow-y-auto">
                    {navigation.map((item) => {
                        const isActive = pathname === item.href;
                        const Icon = item.icon;
                        const label = t(item.nameKey) === item.nameKey ? item.fallback : t(item.nameKey);

                        return (
                            <Link key={item.nameKey} href={item.href}>
                                <motion.div
                                    className={`
                    relative flex items-center gap-3 px-3 py-2 rounded-lg
                    text-sm font-medium transition-colors
                    ${isActive
                                            ? "text-foreground bg-secondary"
                                            : "text-muted-foreground hover:text-foreground hover:bg-secondary/50"
                                        }
                  `}
                                    whileHover={{ x: 2 }}
                                    transition={{ type: "spring", stiffness: 400, damping: 30 }}
                                >
                                    {isActive && (
                                        <motion.div
                                            className="absolute left-0 top-1/2 -translate-y-1/2 w-1 h-8 bg-primary rounded-r-full"
                                            layoutId="activeIndicator"
                                            transition={{ type: "spring", stiffness: 400, damping: 30 }}
                                        />
                                    )}
                                    <Icon className="w-5 h-5 shrink-0" />
                                    <AnimatePresence mode="wait">
                                        {!collapsed && (
                                            <motion.span
                                                key="label"
                                                initial={{ opacity: 0, width: 0 }}
                                                animate={{ opacity: 1, width: "auto" }}
                                                exit={{ opacity: 0, width: 0 }}
                                                className="overflow-hidden whitespace-nowrap"
                                            >
                                                {label}
                                            </motion.span>
                                        )}
                                    </AnimatePresence>
                                </motion.div>
                            </Link>
                        );
                    })}
                </nav>

                <div className="p-2 border-t border-border/50 space-y-1">
                    <Button
                        variant="ghost"
                        size={collapsed ? "icon" : "default"}
                        className="w-full justify-start"
                        onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
                    >
                        {theme === "dark" ? (
                            <Sun className="w-4 h-4" />
                        ) : (
                            <Moon className="w-4 h-4" />
                        )}
                        {!collapsed && <span className="ml-3">{t("nav.toggleTheme")}</span>}
                    </Button>
                    <Button
                        variant="ghost"
                        size={collapsed ? "icon" : "default"}
                        className="w-full justify-start text-destructive hover:text-destructive"
                    >
                        <LogOut className="w-4 h-4" />
                        {!collapsed && <span className="ml-3">{t("nav.signOut")}</span>}
                    </Button>
                </div>
            </motion.aside>

            <main className="flex-1 overflow-auto">
                <motion.div
                    initial={{ opacity: 0, y: 10 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ duration: 0.3 }}
                    className="p-8"
                >
                    {children}
                </motion.div>
            </main>
        </div>
    );
}
