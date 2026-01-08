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
  TrendingUp,
  Users,
  Zap,
  DollarSign,
  ArrowUpRight,
  ArrowDownRight,
} from "lucide-react";
import dynamic from "next/dynamic";

const AreaChart = dynamic(() => import("@tremor/react").then((mod) => mod.AreaChart), { ssr: false });
const DonutChart = dynamic(() => import("@tremor/react").then((mod) => mod.DonutChart), { ssr: false });

// Mock data
const stats = [
  {
    name: "Total Requests",
    value: "847,382",
    change: "+12.5%",
    trend: "up",
    icon: Activity,
    color: "text-blue-400",
    bgColor: "bg-blue-500/10",
  },
  {
    name: "Active Users",
    value: "2,847",
    change: "+8.2%",
    trend: "up",
    icon: Users,
    color: "text-purple-400",
    bgColor: "bg-purple-500/10",
  },
  {
    name: "Avg Response Time",
    value: "234ms",
    change: "-15.3%",
    trend: "down",
    icon: Zap,
    color: "text-green-400",
    bgColor: "bg-green-500/10",
  },
  {
    name: "Monthly Cost",
    value: "$12,847",
    change: "+5.1%",
    trend: "up",
    icon: DollarSign,
    color: "text-orange-400",
    bgColor: "bg-orange-500/10",
  },
];

const requestsData = Array.from({ length: 30 }, (_, i) => {
  // Use deterministic math instead of Math.random() to prevent hydration errors
  const baseRequest = 20000;
  const requestVariance = 5000;
  const baseToken = 500000;
  const tokenVariance = 100000;

  // Create a pseudo-random looking pattern using sine waves
  const pattern = Math.sin(i * 0.5) * 0.5 + Math.cos(i * 0.3) * 0.5;

  return {
    date: `Day ${i + 1}`,
    Requests: Math.floor(baseRequest + Math.abs(pattern * requestVariance)),
    Tokens: Math.floor(baseToken + Math.abs(pattern * tokenVariance)),
  };
});

const modelUsage = [
  { name: "GPT-4", value: 45, color: "blue" },
  { name: "Claude 3", value: 30, color: "purple" },
  { name: "Gemini Pro", value: 15, color: "green" },
  { name: "Others", value: 10, color: "slate" },
];

const topEndpoints = [
  { endpoint: "/v1/chat/completions", requests: 124567, change: 12.5 },
  { endpoint: "/v1/embeddings", requests: 98432, change: 8.3 },
  { endpoint: "/v1/completions", requests: 45123, change: -3.2 },
  { endpoint: "/v1/images/generations", requests: 23456, change: 15.7 },
];

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

export default function DashboardPage() {
  return (
    <div className="space-y-8">
      {/* Header */}
      <motion.div
        initial={{ opacity: 0, y: -20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.5 }}
      >
        <h1 className="text-4xl font-bold tracking-tight mb-2">Overview</h1>
        <p className="text-muted-foreground">
          Welcome back! Here's what's happening with your LLM gateway.
        </p>
      </motion.div>

      {/* Stats Grid */}
      <motion.div
        className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4"
        variants={containerVariants}
        initial="hidden"
        animate="show"
      >
        {stats.map((stat, index) => {
          const Icon = stat.icon;
          const isPositive = stat.trend === "up";
          const TrendIcon = isPositive ? ArrowUpRight : ArrowDownRight;

          return (
            <motion.div key={stat.name} variants={itemVariants}>
              <Card className="glass-card group hover:shadow-lg transition-all duration-300">
                <CardContent className="p-6">
                  <div className="flex items-start justify-between">
                    <div className="flex-1">
                      <p className="text-sm font-medium text-muted-foreground mb-1">
                        {stat.name}
                      </p>
                      <p className="text-3xl font-bold tracking-tight mb-2">
                        {stat.value}
                      </p>
                      <div
                        className={`flex items-center gap-1 text-sm font-medium ${isPositive ? "text-green-400" : "text-red-400"
                          }`}
                      >
                        <TrendIcon className="w-4 h-4" />
                        <span>{stat.change}</span>
                      </div>
                    </div>
                    <div
                      className={`w-12 h-12 rounded-xl ${stat.bgColor} flex items-center justify-center group-hover:scale-110 transition-transform duration-300`}
                    >
                      <Icon className={`w-6 h-6 ${stat.color}`} />
                    </div>
                  </div>
                </CardContent>
              </Card>
            </motion.div>
          );
        })}
      </motion.div>

      {/* Bento Grid Layout */}
      <motion.div
        className="grid grid-cols-1 lg:grid-cols-3 gap-4"
        variants={containerVariants}
        initial="hidden"
        animate="show"
      >
        {/* Requests Chart - Large */}
        <motion.div variants={itemVariants} className="lg:col-span-2">
          <Card className="glass-card h-full">
            <CardHeader>
              <CardTitle className="text-xl">Request Volume</CardTitle>
              <CardDescription>Daily requests and token usage</CardDescription>
            </CardHeader>
            <CardContent>
              <AreaChart
                className="h-80"
                data={requestsData}
                index="date"
                categories={["Requests", "Tokens"]}
                colors={["blue", "purple"]}
                showLegend={true}
                showGridLines={false}
                showXAxis={true}
                showYAxis={true}
                startEndOnly={true}
                curveType="natural"
              />
            </CardContent>
          </Card>
        </motion.div>

        {/* Model Distribution - Small */}
        <motion.div variants={itemVariants}>
          <Card className="glass-card h-full">
            <CardHeader>
              <CardTitle className="text-xl">Model Distribution</CardTitle>
              <CardDescription>Usage by model provider</CardDescription>
            </CardHeader>
            <CardContent>
              <DonutChart
                className="h-64"
                data={modelUsage}
                category="value"
                index="name"
                colors={["blue", "purple", "green", "slate"]}
                showLabel={true}
                showAnimation={true}
              />
              <div className="mt-4 space-y-2">
                {modelUsage.map((model) => (
                  <div
                    key={model.name}
                    className="flex items-center justify-between text-sm"
                  >
                    <span className="text-muted-foreground">{model.name}</span>
                    <span className="font-medium">{model.value}%</span>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
        </motion.div>

        {/* Top Endpoints */}
        <motion.div variants={itemVariants} className="lg:col-span-2">
          <Card className="glass-card h-full">
            <CardHeader>
              <CardTitle className="text-xl">Top Endpoints</CardTitle>
              <CardDescription>Most frequently accessed endpoints</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="space-y-3">
                {topEndpoints.map((endpoint, index) => (
                  <motion.div
                    key={endpoint.endpoint}
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
                          {endpoint.endpoint}
                        </p>
                        <p className="text-xs text-muted-foreground">
                          {endpoint.requests.toLocaleString()} requests
                        </p>
                      </div>
                    </div>
                    <div
                      className={`flex items-center gap-1 text-sm font-medium ${endpoint.change > 0 ? "text-green-400" : "text-red-400"
                        }`}
                    >
                      {endpoint.change > 0 ? (
                        <ArrowUpRight className="w-4 h-4" />
                      ) : (
                        <ArrowDownRight className="w-4 h-4" />
                      )}
                      <span>{Math.abs(endpoint.change)}%</span>
                    </div>
                  </motion.div>
                ))}
              </div>
            </CardContent>
          </Card>
        </motion.div>

        {/* Recent Activity */}
        <motion.div variants={itemVariants}>
          <Card className="glass-card h-full">
            <CardHeader>
              <CardTitle className="text-xl">Recent Activity</CardTitle>
              <CardDescription>Latest system events</CardDescription>
            </CardHeader>
            <CardContent>
              <div className="space-y-4">
                {[
                  {
                    action: "New team created",
                    user: "admin@company.com",
                    time: "2 minutes ago",
                  },
                  {
                    action: "API key rotated",
                    user: "dev@company.com",
                    time: "15 minutes ago",
                  },
                  {
                    action: "User invited",
                    user: "manager@company.com",
                    time: "1 hour ago",
                  },
                  {
                    action: "Budget updated",
                    user: "admin@company.com",
                    time: "3 hours ago",
                  },
                ].map((activity, index) => (
                  <div
                    key={index}
                    className="flex items-start gap-3 p-2 rounded-lg hover:bg-secondary/50 transition-colors"
                  >
                    <div className="w-2 h-2 rounded-full bg-primary mt-2" />
                    <div className="flex-1 min-w-0">
                      <p className="text-sm font-medium">{activity.action}</p>
                      <p className="text-xs text-muted-foreground truncate">
                        {activity.user}
                      </p>
                      <p className="text-xs text-muted-foreground">
                        {activity.time}
                      </p>
                    </div>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
        </motion.div>
      </motion.div>
    </div>
  );
}
