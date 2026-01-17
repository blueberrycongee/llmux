"use client";

import * as React from "react";
import { motion, AnimatePresence, type HTMLMotionProps } from "framer-motion";
import { X } from "lucide-react";
import { cn } from "@/lib/utils";
import { useI18n } from "@/i18n/locale-provider";

const SheetContext = React.createContext<{
    open: boolean;
    onOpenChange: (open: boolean) => void;
} | null>(null);

export function Sheet({
    children,
    open,
    onOpenChange,
}: {
    children: React.ReactNode;
    open: boolean;
    onOpenChange: (open: boolean) => void;
}) {
    return (
        <SheetContext.Provider value={{ open, onOpenChange }}>
            {children}
        </SheetContext.Provider>
    );
}

interface SheetContentProps extends HTMLMotionProps<"div"> {
    side?: "right" | "left";
}

export function SheetContent({
    children,
    side = "right",
    className,
    ...props
}: SheetContentProps) {
    const { t } = useI18n();
    const context = React.useContext(SheetContext);
    if (!context) throw new Error("SheetContent must be used within a Sheet");
    const { open, onOpenChange } = context;

    return (
        <AnimatePresence>
            {open && (
                <>
                    {/* Backdrop */}
                    <motion.div
                        initial={{ opacity: 0 }}
                        animate={{ opacity: 1 }}
                        exit={{ opacity: 0 }}
                        onClick={() => onOpenChange(false)}
                        className="fixed inset-0 z-50 bg-background/80 backdrop-blur-sm"
                    />
                    {/* Panel */}
                    <motion.div
                        initial={{ x: side === "right" ? "100%" : "-100%" }}
                        animate={{ x: 0 }}
                        exit={{ x: side === "right" ? "100%" : "-100%" }}
                        transition={{ type: "spring", damping: 20, stiffness: 300 }}
                        className={cn(
                            "fixed z-50 gap-4 bg-background p-6 shadow-lg transition ease-in-out inset-y-0 right-0 h-full w-3/4 border-l sm:max-w-sm",
                            side === "left" && "left-0 border-r border-l-0",
                            className
                        )}
                        {...props}
                    >
                        <div className="flex flex-col h-full">
                            {children}
                        </div>
                        <button
                            onClick={() => onOpenChange(false)}
                            className="absolute right-4 top-4 rounded-sm opacity-70 ring-offset-background transition-opacity hover:opacity-100 focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 disabled:pointer-events-none data-[state=open]:bg-secondary"
                        >
                            <X className="h-4 w-4" />
                            <span className="sr-only">{t("a11y.close")}</span>
                        </button>
                    </motion.div>
                </>
            )}
        </AnimatePresence>
    );
}

export function SheetHeader({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
    return (
        <div
            className={cn("flex flex-col space-y-2 text-center sm:text-left", className)}
            {...props}
        />
    );
}

export function SheetFooter({ className, ...props }: React.HTMLAttributes<HTMLDivElement>) {
    return (
        <div
            className={cn("flex flex-col-reverse sm:flex-row sm:justify-end sm:space-x-2", className)}
            {...props}
        />
    );
}

export function SheetTitle({ className, ...props }: React.HTMLAttributes<HTMLHeadingElement>) {
    return (
        <h2 className={cn("text-lg font-semibold text-foreground", className)} {...props} />
    );
}

export function SheetDescription({ className, ...props }: React.HTMLAttributes<HTMLParagraphElement>) {
    return (
        <p className={cn("text-sm text-muted-foreground", className)} {...props} />
    );
}
