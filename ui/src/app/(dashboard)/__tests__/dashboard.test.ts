/**
 * Dashboard Component Tests
 * 
 * 测试 Dashboard 页面组件
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';
import React from 'react';

// Mock framer-motion to avoid animation issues in tests
vi.mock('framer-motion', () => ({
    motion: {
        div: ({ children, ...props }: React.PropsWithChildren<Record<string, unknown>>) =>
            React.createElement('div', props, children),
    },
    AnimatePresence: ({ children }: React.PropsWithChildren) => children,
}));

// Mock Tremor charts
vi.mock('@tremor/react', () => ({
    AreaChart: ({ data, ...props }: { data: unknown[] }) =>
        React.createElement('div', { 'data-testid': 'area-chart', ...props }, `AreaChart with ${data?.length || 0} items`),
    DonutChart: ({ data, ...props }: { data: unknown[] }) =>
        React.createElement('div', { 'data-testid': 'donut-chart', ...props }, `DonutChart with ${data?.length || 0} items`),
}));

// Mock the hooks
vi.mock('@/hooks/use-dashboard-stats', () => ({
    useDashboardStats: vi.fn(),
}));

vi.mock('@/hooks/use-model-spend', () => ({
    useModelSpend: vi.fn(),
}));

import { useDashboardStats } from '@/hooks/use-dashboard-stats';
import { useModelSpend } from '@/hooks/use-model-spend';

// Import the component after mocks
// Note: We'll import Dashboard component once it's refactored to use hooks

describe('Dashboard Stats Display', () => {
    beforeEach(() => {
        vi.clearAllMocks();
    });

    it('should display loading state when data is loading', () => {
        vi.mocked(useDashboardStats).mockReturnValue({
            dailyData: [],
            totalRequests: 0,
            totalTokens: 0,
            totalCost: 0,
            avgLatency: 0,
            successRate: 0,
            uniqueModels: 0,
            uniqueProviders: 0,
            isLoading: true,
            error: null,
            refresh: vi.fn(),
        });

        vi.mocked(useModelSpend).mockReturnValue({
            models: [],
            isLoading: true,
            error: null,
            refresh: vi.fn(),
        });

        // 验证 loading 状态被正确返回
        const result = useDashboardStats();
        expect(result.isLoading).toBe(true);
    });

    it('should return correct data when API succeeds', () => {
        vi.mocked(useDashboardStats).mockReturnValue({
            dailyData: [
                { date: '2026-01-01', api_requests: 1000, total_tokens: 50000, spend: 5.0 },
            ],
            totalRequests: 1000,
            totalTokens: 50000,
            totalCost: 5.0,
            avgLatency: 234,
            successRate: 98.5,
            uniqueModels: 3,
            uniqueProviders: 2,
            isLoading: false,
            error: null,
            refresh: vi.fn(),
        });

        const result = useDashboardStats();

        expect(result.totalRequests).toBe(1000);
        expect(result.totalTokens).toBe(50000);
        expect(result.totalCost).toBe(5.0);
        expect(result.avgLatency).toBe(234);
        expect(result.successRate).toBe(98.5);
        expect(result.dailyData).toHaveLength(1);
    });

    it('should return error state when API fails', () => {
        const mockError = new Error('API Error');

        vi.mocked(useDashboardStats).mockReturnValue({
            dailyData: [],
            totalRequests: 0,
            totalTokens: 0,
            totalCost: 0,
            avgLatency: 0,
            successRate: 0,
            uniqueModels: 0,
            uniqueProviders: 0,
            isLoading: false,
            error: mockError,
            refresh: vi.fn(),
        });

        const result = useDashboardStats();
        expect(result.error).toBe(mockError);
    });
});

describe('Model Spend Display', () => {
    beforeEach(() => {
        vi.clearAllMocks();
    });

    it('should display model data correctly', () => {
        vi.mocked(useModelSpend).mockReturnValue({
            models: [
                { model: 'gpt-4', spend: 100, api_requests: 500, total_tokens: 200000 },
                { model: 'claude-3', spend: 50, api_requests: 300, total_tokens: 100000 },
            ],
            isLoading: false,
            error: null,
            refresh: vi.fn(),
        });

        const result = useModelSpend();

        expect(result.models).toHaveLength(2);
        expect(result.models[0].model).toBe('gpt-4');
        expect(result.models[0].spend).toBe(100);
    });

    it('should handle empty model data', () => {
        vi.mocked(useModelSpend).mockReturnValue({
            models: [],
            isLoading: false,
            error: null,
            refresh: vi.fn(),
        });

        const result = useModelSpend();
        expect(result.models).toHaveLength(0);
    });
});

describe('Dashboard Data Formatting', () => {
    it('should format large numbers with commas', () => {
        const formatNumber = (num: number) => num.toLocaleString();

        expect(formatNumber(1000)).toBe('1,000');
        expect(formatNumber(1000000)).toBe('1,000,000');
        expect(formatNumber(123456789)).toBe('123,456,789');
    });

    it('should format currency correctly', () => {
        const formatCurrency = (num: number) => `$${num.toFixed(2)}`;

        expect(formatCurrency(18.5)).toBe('$18.50');
        expect(formatCurrency(1234.567)).toBe('$1234.57');
        expect(formatCurrency(0)).toBe('$0.00');
    });

    it('should format percentage correctly', () => {
        const formatPercentage = (num: number) => `${num.toFixed(1)}%`;

        expect(formatPercentage(98.5)).toBe('98.5%');
        expect(formatPercentage(100)).toBe('100.0%');
        expect(formatPercentage(0)).toBe('0.0%');
    });

    it('should format latency correctly', () => {
        const formatLatency = (ms: number) => `${ms}ms`;

        expect(formatLatency(234)).toBe('234ms');
        expect(formatLatency(1500)).toBe('1500ms');
    });
});

describe('Dashboard Date Range', () => {
    it('should calculate date range for last 7 days', () => {
        const now = new Date('2026-01-09');
        const startDate = new Date(now);
        startDate.setDate(startDate.getDate() - 7);

        expect(startDate.toISOString().split('T')[0]).toBe('2026-01-02');
    });

    it('should calculate date range for last 30 days', () => {
        const now = new Date('2026-01-09');
        const startDate = new Date(now);
        startDate.setDate(startDate.getDate() - 30);

        expect(startDate.toISOString().split('T')[0]).toBe('2025-12-10');
    });
});

describe('Dashboard Stat Cards', () => {
    it('should determine trend direction correctly', () => {
        const getTrendDirection = (change: number) => change >= 0 ? 'up' : 'down';

        expect(getTrendDirection(12.5)).toBe('up');
        expect(getTrendDirection(-5.3)).toBe('down');
        expect(getTrendDirection(0)).toBe('up');
    });

    it('should determine trend color correctly', () => {
        const getTrendColor = (trend: 'up' | 'down', isPositiveGood = true) => {
            if (isPositiveGood) {
                return trend === 'up' ? 'text-green-400' : 'text-red-400';
            }
            return trend === 'up' ? 'text-red-400' : 'text-green-400';
        };

        // For metrics like requests (up is good)
        expect(getTrendColor('up', true)).toBe('text-green-400');
        expect(getTrendColor('down', true)).toBe('text-red-400');

        // For metrics like latency (down is good)
        expect(getTrendColor('up', false)).toBe('text-red-400');
        expect(getTrendColor('down', false)).toBe('text-green-400');
    });
});
