/**
 * Dashboard Stats Hook
 * 
 * 获取 Dashboard 主页的全局统计数据
 * 数据来源: GET /global/activity
 */

import { useQuery } from '@tanstack/react-query';
import { apiClient } from '@/lib/api';
import type { GlobalActivityResponse } from '@/types/api';

interface UseDashboardStatsOptions {
    startDate?: string;
    endDate?: string;
}

interface DashboardStatsReturn {
    // 日期维度的数据
    dailyData: GlobalActivityResponse['daily_data'];

    // 汇总指标
    totalRequests: number;
    totalTokens: number;
    totalCost: number;
    avgLatency: number;
    successRate: number;
    uniqueModels: number;
    uniqueProviders: number;

    // 状态
    isLoading: boolean;
    error: Error | null;
    refresh: () => void;
}

export function useDashboardStats(options: UseDashboardStatsOptions = {}): DashboardStatsReturn {
    const { startDate, endDate } = options;

    const { data, error, isLoading, refetch } = useQuery<GlobalActivityResponse>({
        queryKey: ['global-activity', startDate, endDate],
        queryFn: () => apiClient.getGlobalActivity({ start_date: startDate, end_date: endDate }),
        refetchInterval: 60000, // 每分钟自动刷新
        refetchOnWindowFocus: true,
        staleTime: 5000,
    });

    return {
        dailyData: data?.daily_data ?? [],
        totalRequests: data?.sum_api_requests ?? 0,
        totalTokens: data?.sum_total_tokens ?? 0,
        totalCost: data?.total_cost ?? 0,
        avgLatency: data?.avg_latency_ms ?? 0,
        successRate: data?.success_rate ?? 0,
        uniqueModels: data?.unique_models ?? 0,
        uniqueProviders: data?.unique_providers ?? 0,
        isLoading,
        error: error ?? null,
        refresh: () => { refetch(); },
    };
}
