"use client";

import { motion } from "framer-motion";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Activity,
  Users,
  Zap,
  DollarSign,
  CheckCircle,
  RefreshCw,
} from "lucide-react";
import dynamic from "next/dynamic";
import { useDashboardStats } from "@/hooks/use-dashboard-stats";
import { useModelSpend } from "@/hooks/use-model-spend";
import { useProviderSpend } from "@/hooks/use-provider-spend";
import { useState, useMemo } from "react";
import { ClientOnly } from "@/components/client-only";
import { useI18n } from "@/i18n/locale-provider";

const AreaChart = dynamic(
  () => import("@tremor/react").then((mod) => mod.AreaChart),
  { ssr: false }
);
const DonutChart = dynamic(
  () => import("@tremor/react").then((mod) => mod.DonutChart),
  { ssr: false }
);

// Animation variants
const containerVariants = {
  hidden: { opacity: 0 },
  show: {
    opacity: 1,
    transition: {
      staggerChildren: 0.05,
    },
  },
};

const itemVariants = {
  hidden: { opacity: 0, y: 20 },
  show: { opacity: 1, y: 0 },
};

// Formatting utilities
function formatNumber(num: number): string {
  return num.toLocaleString();
}

function formatCurrency(num: number): string {
  return `$${num.toFixed(2)}`;
}

function formatPercentage(num: number): string {
  return `${num.toFixed(1)}%`;
}

function formatLatency(ms: number): string {
  return `${ms}ms`;
}

// Date range options
type DateRange = "7d" | "30d" | "90d";

function getDateRange(range: DateRange): { startDate: string; endDate: string } {
  const now = new Date();
  const endDate = now.toISOString().split("T")[0];

  const startDateObj = new Date(now);
  switch (range) {
    case "7d":
      startDateObj.setDate(startDateObj.getDate() - 7);
      break;
    case "30d":
      startDateObj.setDate(startDateObj.getDate() - 30);
      break;
    case "90d":
      startDateObj.setDate(startDateObj.getDate() - 90);
      break;
  }
  const startDate = startDateObj.toISOString().split("T")[0];

  return { startDate, endDate };
}

// Skeleton component for loading state
function StatCardSkeleton() {
  return (
    <Card className="glass-card">
      <CardContent className="p-6">
        <div className="flex items-start justify-between">
          <div className="flex-1">
            <div className="h-4 w-24 bg-muted animate-pulse rounded mb-2" />
            <div className="h-8 w-32 bg-muted animate-pulse rounded mb-2" />
            <div className="h-4 w-16 bg-muted animate-pulse rounded" />
          </div>
          <div className="w-12 h-12 bg-muted animate-pulse rounded-xl" />
        </div>
      </CardContent>
    </Card>
  );
}

function ChartSkeleton({ className = "" }: { className?: string }) {
  return (
    <div className={`bg-muted animate-pulse rounded-lg ${className}`} />
  );
}

// Error component
function ErrorMessage({ message, onRetry }: { message: string; onRetry: () => void }) {
  const { t } = useI18n();
  return (
    <div
      data-testid="error-message"
      className="flex flex-col items-center justify-center p-8 text-center"
    >
      <div className="w-12 h-12 rounded-full bg-red-500/10 flex items-center justify-center mb-4">
        <Activity className="w-6 h-6 text-red-400" />
      </div>
      <p className="text-muted-foreground mb-4">{t("dashboard.overview.error.loadFailed")}</p>
      <p className="text-sm text-muted-foreground mb-4">{message}</p>
      <button
        onClick={onRetry}
        className="flex items-center gap-2 px-4 py-2 bg-primary text-primary-foreground rounded-lg hover:bg-primary/90 transition-colors"
      >
        <RefreshCw className="w-4 h-4" />
        {t("common.retry")}
      </button>
    </div>
  );
}

export default function DashboardPage() {
  const { t } = useI18n();
  const [dateRange, setDateRange] = useState<DateRange>("30d");

  // Use useMemo for stable date calculation (P3 fix: makes render function pure)
  const { startDate, endDate } = useMemo(() => getDateRange(dateRange), [dateRange]);

  const {
    dailyData,
    totalRequests,
    totalTokens,
    totalCost,
    avgLatency,
    successRate,
    isLoading: statsLoading,
    error: statsError,
    refresh: refreshStats,
  } = useDashboardStats({ startDate, endDate });

  const {
    models,
    isLoading: modelsLoading,
    error: modelsError,
    refresh: refreshModels,
  } = useModelSpend({ startDate, endDate, limit: 10 });

  const {
    providers,
    isLoading: providersLoading,
    error: providersError,
    refresh: refreshProviders,
  } = useProviderSpend({ startDate, endDate, limit: 10 });

  const isLoading = statsLoading || modelsLoading || providersLoading;
  const error = statsError || modelsError || providersError;

  // Build stats array from API data
  const stats = [
    {
      name: t("dashboard.overview.stats.totalRequests"),
      value: formatNumber(totalRequests),
      testId: "requests",
      icon: Activity,
      color: "text-blue-400",
      bgColor: "bg-blue-500/10",
    },
    {
      name: t("dashboard.overview.stats.totalTokens"),
      value: formatNumber(totalTokens),
      testId: "tokens",
      icon: Zap,
      color: "text-purple-400",
      bgColor: "bg-purple-500/10",
    },
    {
      name: t("dashboard.overview.stats.totalCost"),
      value: formatCurrency(totalCost),
      testId: "cost",
      icon: DollarSign,
      color: "text-orange-400",
      bgColor: "bg-orange-500/10",
    },
    {
      name: t("dashboard.overview.stats.avgLatency"),
      value: formatLatency(avgLatency),
      testId: "latency",
      icon: Zap,
      color: "text-green-400",
      bgColor: "bg-green-500/10",
    },
    {
      name: t("dashboard.overview.stats.successRate"),
      value: formatPercentage(successRate),
      testId: "success-rate",
      icon: CheckCircle,
      color: "text-emerald-400",
      bgColor: "bg-emerald-500/10",
    },
  ];

  const requestsLabel = t("dashboard.overview.chart.legend.requests");
  const tokensLabel = t("dashboard.overview.chart.legend.tokens");

  // Transform daily data for chart (localize legend labels)
  const chartData = useMemo(
    () =>
      dailyData.map((d) => ({
        date: d.date,
        [requestsLabel]: d.api_requests,
        [tokensLabel]: d.total_tokens,
      })),
    [dailyData, requestsLabel, tokensLabel]
  );

  const sortedModels = useMemo(
    () => [...models].sort((a, b) => b.spend - a.spend),
    [models]
  );

  const sortedProviders = useMemo(
    () => [...providers].sort((a, b) => b.spend - a.spend),
    [providers]
  );

  const providerChartData = sortedProviders.map((p) => ({
    name: p.provider,
    value: p.spend,
  }));

  const totalProviderSpend = sortedProviders.reduce((sum, p) => sum + p.spend, 0);
  const providerPercentages = sortedProviders.map((p) => ({
    ...p,
    percentage: totalProviderSpend > 0 ? (p.spend / totalProviderSpend) * 100 : 0,
  }));

  const handleRefresh = () => {
    refreshStats();
    refreshModels();
    refreshProviders();
  };

  // Error state
  if (error && !isLoading) {
    return (
      <div className="space-y-8">
        <motion.div
          initial={{ opacity: 0, y: -20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.5 }}
        >
          <h1 className="text-4xl font-bold tracking-tight mb-2">{t("dashboard.overview.title")}</h1>
          <p className="text-muted-foreground">
            {t("dashboard.overview.subtitle")}
          </p>
        </motion.div>
        <ErrorMessage
          message={error.message}
          onRetry={handleRefresh}
        />
      </div>
    );
  }

  // Note: Removed isMounted check here. Charts are already wrapped in ClientOnly
  // components, so there's no need to block the entire page render.

  return (
    <div className="space-y-8">
      {/* Header */}
      <motion.div
        initial={{ opacity: 0, y: -20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.5 }}
        className="flex items-center justify-between"
      >
        <div>
          <h1 className="text-4xl font-bold tracking-tight mb-2">{t("dashboard.overview.title")}</h1>
          <p className="text-muted-foreground">
            {t("dashboard.overview.subtitle")}
          </p>
        </div>

        {/* Date Range Picker */}
        <div
          data-testid="date-range-picker"
          className="flex items-center gap-2 bg-secondary/50 rounded-lg p-1"
        >
          {(["7d", "30d", "90d"] as DateRange[]).map((range) => (
            <button
              key={range}
              data-testid={`date-range-${range}`}
              onClick={() => setDateRange(range)}
              className={`px-3 py-1.5 rounded-md text-sm font-medium transition-colors ${dateRange === range
                ? "bg-primary text-primary-foreground"
                : "text-muted-foreground hover:text-foreground"
                }`}
            >
              {range === "7d"
                ? t("dashboard.overview.range.7d")
                : range === "30d"
                  ? t("dashboard.overview.range.30d")
                  : t("dashboard.overview.range.90d")}
            </button>
          ))}
        </div>
      </motion.div>

      {/* Stats Grid */}
      {isLoading ? (
        <div data-testid="skeleton-stats" className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-5 gap-4">
          {[1, 2, 3, 4, 5].map((i) => (
            <StatCardSkeleton key={i} />
          ))}
        </div>
      ) : (
        <motion.div
          className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-5 gap-4"
          variants={containerVariants}
          initial="hidden"
          animate="show"
        >
          {stats.map((stat) => {
            const Icon = stat.icon;

            return (
              <motion.div
                key={stat.name}
                variants={itemVariants}
                data-testid={`stat-card-${stat.testId}`}
              >
                <Card className="glass-card group hover:shadow-lg transition-all duration-300">
                  <CardContent className="p-6">
                    <div className="flex items-start justify-between">
                      <div className="flex-1">
                        <p className="text-sm font-medium text-muted-foreground mb-1">
                          {stat.name}
                        </p>
                        <p
                          className="text-2xl font-bold tracking-tight mb-2"
                          data-testid={`stat-value-${stat.testId}`}
                        >
                          {stat.value}
                        </p>
                      </div>
                      <div
                        className={`w-10 h-10 rounded-xl ${stat.bgColor} flex items-center justify-center group-hover:scale-110 transition-transform duration-300`}
                      >
                        <Icon className={`w-5 h-5 ${stat.color}`} />
                      </div>
                    </div>
                  </CardContent>
                </Card>
              </motion.div>
            );
          })}
        </motion.div>
      )}

      {/* Bento Grid Layout */}
      <motion.div
        className="grid grid-cols-1 lg:grid-cols-3 gap-4"
        variants={containerVariants}
        initial="hidden"
        animate="show"
      >
        {/* Requests Chart - Large */}
        <motion.div variants={itemVariants} className="lg:col-span-2">
          <Card className="glass-card h-full" data-testid="chart-request-volume">
            <CardHeader>
              <CardTitle className="text-xl">{t("dashboard.overview.charts.requestVolume.title")}</CardTitle>
              <CardDescription>{t("dashboard.overview.charts.requestVolume.desc")}</CardDescription>
            </CardHeader>
            <CardContent>
              {isLoading ? (
                <ChartSkeleton className="h-80" />
              ) : chartData.length > 0 ? (
                <ClientOnly fallback={<ChartSkeleton className="h-80" />}>
                  <AreaChart
                    className="h-80"
                    data={chartData}
                    index="date"
                    categories={[requestsLabel, tokensLabel]}
                    colors={["blue", "purple"]}
                    showLegend={true}
                    showGridLines={false}
                    showXAxis={true}
                    showYAxis={true}
                    startEndOnly={true}
                    curveType="natural"
                  />
                </ClientOnly>
              ) : (
                <div className="h-80 flex items-center justify-center text-muted-foreground">
                  {t("dashboard.overview.charts.requestVolume.empty")}
                </div>
              )}
            </CardContent>
          </Card>
        </motion.div>

        {/* Model Distribution - Small */}
        <motion.div variants={itemVariants}>
          <Card className="glass-card h-full" data-testid="chart-model-distribution">
            <CardHeader>
              <CardTitle className="text-xl">{t("dashboard.overview.charts.modelDistribution.title")}</CardTitle>
              <CardDescription>{t("dashboard.overview.charts.modelDistribution.desc")}</CardDescription>
            </CardHeader>
            <CardContent>
              {isLoading ? (
                <>
                  <ChartSkeleton className="h-48" />
                  <div className="mt-4 space-y-2">
                    {[1, 2, 3].map((i) => (
                      <div key={i} className="flex items-center justify-between">
                        <div className="h-4 w-20 bg-muted animate-pulse rounded" />
                        <div className="h-4 w-12 bg-muted animate-pulse rounded" />
                      </div>
                    ))}
                  </div>
                </>
              ) : providerChartData.length > 0 ? (
                <>
                  <ClientOnly fallback={<ChartSkeleton className="h-48" />}>
                    <DonutChart
                      className="h-48"
                      data={providerChartData}
                      category="value"
                      index="name"
                      colors={["blue", "purple", "green", "orange", "pink"]}
                      showLabel={true}
                      showAnimation={true}
                    />
                  </ClientOnly>
                  <div className="mt-4 space-y-2">
                    {providerPercentages.map((provider) => (
                      <div
                        key={provider.provider}
                        className="flex items-center justify-between text-sm"
                      >
                        <span className="text-muted-foreground">{provider.provider}</span>
                        <span className="font-medium">{provider.percentage.toFixed(1)}%</span>
                      </div>
                    ))}
                  </div>
                </>
              ) : (
                <div className="h-48 flex items-center justify-center text-muted-foreground">
                  {t("dashboard.overview.charts.modelDistribution.empty")}
                </div>
              )}
            </CardContent>
          </Card>
        </motion.div>

        {/* Top Models by Spend */}
        <motion.div variants={itemVariants} className="lg:col-span-2">
          <Card className="glass-card h-full">
            <CardHeader>
              <CardTitle className="text-xl">{t("dashboard.overview.topModels.title")}</CardTitle>
              <CardDescription>{t("dashboard.overview.topModels.desc")}</CardDescription>
            </CardHeader>
            <CardContent>
              {isLoading ? (
                <div className="space-y-3">
                  {[1, 2, 3, 4].map((i) => (
                    <div key={i} className="flex items-center justify-between p-3">
                      <div className="flex items-center gap-3">
                        <div className="w-8 h-8 bg-muted animate-pulse rounded-lg" />
                        <div>
                          <div className="h-4 w-32 bg-muted animate-pulse rounded mb-1" />
                          <div className="h-3 w-24 bg-muted animate-pulse rounded" />
                        </div>
                      </div>
                      <div className="h-4 w-16 bg-muted animate-pulse rounded" />
                    </div>
                  ))}
                </div>
              ) : models.length > 0 ? (
                <div className="space-y-3">
                  {sortedModels.map((model, index) => (
                    <motion.div
                      key={model.model}
                      className="flex items-center justify-between p-3 rounded-lg hover:bg-secondary/50 transition-colors group"
                      whileHover={{ x: 4 }}
                      transition={{ type: "spring", stiffness: 400, damping: 30 }}
                    >
                      <div className="flex items-center gap-3 flex-1">
                        <div className="w-8 h-8 rounded-lg bg-primary/10 flex items-center justify-center font-mono text-sm font-semibold">
                          {index + 1}
                        </div>
                        <div className="flex-1 min-w-0">
                          <p className="font-mono text-sm truncate">
                            {model.model}
                          </p>
                          <p className="text-xs text-muted-foreground">
                            {formatNumber(model.api_requests)} requests • {formatNumber(model.total_tokens)} tokens
                          </p>
                        </div>
                      </div>
                      <div className="flex items-center gap-1 text-sm font-medium text-green-400">
                        <DollarSign className="w-4 h-4" />
                        <span>{model.spend.toFixed(2)}</span>
                      </div>
                    </motion.div>
                  ))}
                </div>
              ) : (
                <div className="h-48 flex items-center justify-center text-muted-foreground">
                  {t("dashboard.overview.charts.modelDistribution.empty")}
                </div>
              )}
            </CardContent>
          </Card>
        </motion.div>

        {/* Quick Stats */}
        <motion.div variants={itemVariants}>
          <Card className="glass-card h-full">
            <CardHeader>
              <CardTitle className="text-xl">{t("dashboard.overview.quickStats.title")}</CardTitle>
              <CardDescription>{t("dashboard.overview.quickStats.desc")}</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="space-y-4">
                <div className="flex items-start gap-3 p-3 rounded-lg bg-secondary/30">
                  <div className="w-10 h-10 rounded-lg bg-blue-500/10 flex items-center justify-center">
                    <Activity className="w-5 h-5 text-blue-400" />
                  </div>
                  <div>
                    <p className="text-sm font-medium">{t("dashboard.overview.quickStats.activeModels")}</p>
                    <p className="text-2xl font-bold">{uniqueModels}</p>
                  </div>
                </div>

                <div className="flex items-start gap-3 p-3 rounded-lg bg-secondary/30">
                  <div className="w-10 h-10 rounded-lg bg-green-500/10 flex items-center justify-center">
                    <CheckCircle className="w-5 h-5 text-green-400" />
                  </div>
                  <div>
                    <p className="text-sm font-medium">{t("dashboard.overview.stats.successRate")}</p>
                    <p className="text-2xl font-bold">{formatPercentage(successRate)}</p>
                  </div>
                </div>

                <div className="flex items-start gap-3 p-3 rounded-lg bg-secondary/30">
                  <div className="w-10 h-10 rounded-lg bg-purple-500/10 flex items-center justify-center">
                    <Users className="w-5 h-5 text-purple-400" />
                  </div>
                  <div>
                    <p className="text-sm font-medium">{t("dashboard.overview.quickStats.avgDailyRequests")}</p>
                    <p className="text-2xl font-bold">
                      {dailyData.length > 0
                        ? formatNumber(
                          Math.round(
                            dailyData.reduce((sum, d) => sum + d.api_requests, 0) /
                            dailyData.length
                          )
                        )
                        : "0"}
                    </p>
                  </div>
                </div>
              </div>
            </CardContent>
          </Card>
        </motion.div>
      </motion.div>
    </div>
  );
}
