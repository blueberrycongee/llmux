"use client";

import { useState } from "react";
import { motion } from "framer-motion";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { LogIn, ShieldCheck, Sparkles, Zap } from "lucide-react";
import { useI18n } from "@/i18n/locale-provider";

export default function LoginPage() {
    const [isLoading, setIsLoading] = useState(false);
    const [email, setEmail] = useState("");
    const [password, setPassword] = useState("");
    const { t } = useI18n();

    const handleLogin = async (e: React.FormEvent) => {
        e.preventDefault();
        setIsLoading(true);
        // Simulate API call
        await new Promise(resolve => setTimeout(resolve, 1500));
        setIsLoading(false);
    };

    const handleSSO = () => {
        window.location.href = "/api/auth/oidc/login";
    };

    return (
        <div className="relative min-h-screen flex items-center justify-center overflow-hidden bg-gradient-to-br from-slate-950 via-slate-900 to-slate-950">
            {/* Animated background elements */}
            <div className="absolute inset-0 overflow-hidden">
                <motion.div
                    className="absolute top-1/4 left-1/4 w-96 h-96 bg-blue-500/10 rounded-full blur-3xl"
                    animate={{
                        scale: [1, 1.2, 1],
                        opacity: [0.3, 0.5, 0.3],
                    }}
                    transition={{
                        duration: 8,
                        repeat: Infinity,
                        ease: "easeInOut",
                    }}
                />
                <motion.div
                    className="absolute bottom-1/4 right-1/4 w-96 h-96 bg-purple-500/10 rounded-full blur-3xl"
                    animate={{
                        scale: [1.2, 1, 1.2],
                        opacity: [0.5, 0.3, 0.5],
                    }}
                    transition={{
                        duration: 8,
                        repeat: Infinity,
                        ease: "easeInOut",
                    }}
                />
            </div>

            {/* Main content */}
            <div className="relative z-10 w-full max-w-md px-4">
                <motion.div
                    initial={{ opacity: 0, y: 20 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ duration: 0.5 }}
                >
                    {/* Logo and branding */}
                    <div className="text-center mb-8">
                        <motion.div
                            className="inline-flex items-center justify-center w-16 h-16 mb-4 rounded-2xl bg-gradient-to-br from-blue-500 to-purple-600 shadow-lg shadow-blue-500/20"
                            whileHover={{ scale: 1.05, rotate: 5 }}
                            transition={{ type: "spring", stiffness: 400, damping: 10 }}
                        >
                            <Zap className="w-8 h-8 text-white" />
                        </motion.div>
                        <h1 className="text-4xl font-bold tracking-tight mb-2 bg-gradient-to-r from-white to-slate-400 bg-clip-text text-transparent">
                            LLMux
                        </h1>
                        <p className="text-sm text-slate-400">
                            {t("login.subtitle")}
                        </p>
                    </div>

                    {/* Login card */}
                    <Card className="glass-card">
                        <CardHeader className="space-y-1">
                            <CardTitle className="text-2xl font-bold tracking-tight">
                                {t("login.welcomeBack")}
                            </CardTitle>
                            <CardDescription>
                                {t("login.signInToContinue")}
                            </CardDescription>
                        </CardHeader>
                        <CardContent>
                            <form onSubmit={handleLogin} className="space-y-4">
                                <div className="space-y-2">
                                    <Label htmlFor="email">{t("login.email")}</Label>
                                    <Input
                                        id="email"
                                        type="email"
                                        placeholder={t("login.emailPlaceholder")}
                                        value={email}
                                        onChange={(e) => setEmail(e.target.value)}
                                        required
                                        disabled={isLoading}
                                    />
                                </div>
                                <div className="space-y-2">
                                    <Label htmlFor="password">{t("login.password")}</Label>
                                    <Input
                                        id="password"
                                        type="password"
                                        placeholder="••••••••"
                                        value={password}
                                        onChange={(e) => setPassword(e.target.value)}
                                        required
                                        disabled={isLoading}
                                    />
                                </div>

                                <Button
                                    type="submit"
                                    className="w-full group relative overflow-hidden"
                                    disabled={isLoading}
                                >
                                    {isLoading ? (
                                        <motion.div
                                            className="flex items-center gap-2"
                                            initial={{ opacity: 0 }}
                                            animate={{ opacity: 1 }}
                                        >
                                            <motion.div
                                                className="w-4 h-4 border-2 border-white border-t-transparent rounded-full"
                                                animate={{ rotate: 360 }}
                                                transition={{ duration: 1, repeat: Infinity, ease: "linear" }}
                                            />
                                            {t("login.signingIn")}
                                        </motion.div>
                                    ) : (
                                        <>
                                            <LogIn className="w-4 h-4 mr-2" />
                                            {t("login.signIn")}
                                        </>
                                    )}
                                </Button>

                                <div className="relative my-6">
                                    <div className="absolute inset-0 flex items-center">
                                        <div className="w-full border-t border-border/50" />
                                    </div>
                                    <div className="relative flex justify-center text-xs uppercase">
                                        <span className="bg-card/50 px-2 text-muted-foreground">
                                            {t("login.orContinueWith")}
                                        </span>
                                    </div>
                                </div>

                                <Button
                                    type="button"
                                    variant="outline"
                                    className="w-full group"
                                    onClick={handleSSO}
                                    disabled={isLoading}
                                >
                                    <ShieldCheck className="w-4 h-4 mr-2 transition-transform group-hover:scale-110" />
                                    {t("login.signInWithSSO")}
                                </Button>
                            </form>

                            {/* Features highlight */}
                            <div className="mt-8 pt-6 border-t border-border/50">
                                <div className="grid grid-cols-3 gap-4 text-center">
                                    <motion.div
                                        className="space-y-1"
                                        whileHover={{ y: -2 }}
                                        transition={{ type: "spring", stiffness: 400, damping: 10 }}
                                    >
                                        <div className="flex justify-center">
                                            <div className="w-8 h-8 rounded-lg bg-blue-500/10 flex items-center justify-center">
                                                <ShieldCheck className="w-4 h-4 text-blue-400" />
                                            </div>
                                        </div>
                                        <p className="text-xs text-muted-foreground">{t("login.feature.secure")}</p>
                                    </motion.div>
                                    <motion.div
                                        className="space-y-1"
                                        whileHover={{ y: -2 }}
                                        transition={{ type: "spring", stiffness: 400, damping: 10 }}
                                    >
                                        <div className="flex justify-center">
                                            <div className="w-8 h-8 rounded-lg bg-purple-500/10 flex items-center justify-center">
                                                <Zap className="w-4 h-4 text-purple-400" />
                                            </div>
                                        </div>
                                        <p className="text-xs text-muted-foreground">{t("login.feature.fast")}</p>
                                    </motion.div>
                                    <motion.div
                                        className="space-y-1"
                                        whileHover={{ y: -2 }}
                                        transition={{ type: "spring", stiffness: 400, damping: 10 }}
                                    >
                                        <div className="flex justify-center">
                                            <div className="w-8 h-8 rounded-lg bg-green-500/10 flex items-center justify-center">
                                                <Sparkles className="w-4 h-4 text-green-400" />
                                            </div>
                                        </div>
                                        <p className="text-xs text-muted-foreground">{t("login.feature.smart")}</p>
                                    </motion.div>
                                </div>
                            </div>
                        </CardContent>
                    </Card>

                    {/* Footer */}
                    <p className="mt-6 text-center text-xs text-slate-500">
                        {t("login.footer.prefix")} {" "}
                        <a href="#" className="underline hover:text-slate-400 transition-colors">
                            {t("login.footer.terms")}
                        </a>{" "}
                        {t("login.footer.and")} {" "}
                        <a href="#" className="underline hover:text-slate-400 transition-colors">
                            {t("login.footer.privacy")}
                        </a>
                    </p>
                </motion.div>
            </div>
        </div>
    );
}
