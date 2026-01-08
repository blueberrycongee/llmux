/**
 * React Hooks Tests
 * 
 * 测试 API 数据获取 Hooks
 */

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import React from 'react';

// Mock API client
vi.mock('@/lib/api', () => ({
    apiClient: {
        getGlobalActivity: vi.fn(),
        getSpendByModels: vi.fn(),
        listKeys: vi.fn(),
        generateKey: vi.fn(),
        deleteKeys: vi.fn(),
        blockKey: vi.fn(),
        unblockKey: vi.fn(),
        regenerateKey: vi.fn(),
        listTeams: vi.fn(),
        createTeam: vi.fn(),
        deleteTeams: vi.fn(),
        blockTeam: vi.fn(),
        unblockTeam: vi.fn(),
        getTeamInfo: vi.fn(),
        getKeyInfo: vi.fn(),
    },
}));

import { apiClient } from '@/lib/api';
import { useDashboardStats } from '../use-dashboard-stats';
import { useModelSpend } from '../use-model-spend';
import { useApiKeys, useApiKeyInfo } from '../use-api-keys';
import { useTeams, useTeamInfo } from '../use-teams';

// 创建测试用的 QueryClient wrapper
function createWrapper() {
    const queryClient = new QueryClient({
        defaultOptions: {
            queries: {
                retry: false,
                gcTime: 0,
            },
        },
    });

    return function Wrapper({ children }: { children: React.ReactNode }) {
        return React.createElement(
            QueryClientProvider,
            { client: queryClient },
            children
        );
    };
}

describe('useDashboardStats', () => {
    beforeEach(() => {
        vi.clearAllMocks();
    });

    it('应该正确获取全局活动数据', async () => {
        const mockData = {
            daily_data: [
                { date: '2026-01-08', api_requests: 100, total_tokens: 50000, spend: 5.0 },
            ],
            sum_api_requests: 100,
            sum_total_tokens: 50000,
            total_cost: 5.0,
            avg_latency_ms: 234,
            success_rate: 98.5,
            unique_models: 3,
            unique_providers: 2,
        };

        vi.mocked(apiClient.getGlobalActivity).mockResolvedValue(mockData);

        const { result } = renderHook(() => useDashboardStats(), {
            wrapper: createWrapper(),
        });

        // 初始状态应该是 loading
        expect(result.current.isLoading).toBe(true);

        // 等待数据加载
        await waitFor(() => {
            expect(result.current.isLoading).toBe(false);
        });

        // 验证数据
        expect(result.current.totalRequests).toBe(100);
        expect(result.current.totalTokens).toBe(50000);
        expect(result.current.totalCost).toBe(5.0);
        expect(result.current.avgLatency).toBe(234);
        expect(result.current.successRate).toBe(98.5);
        expect(result.current.dailyData).toHaveLength(1);
    });

    it('应该处理空数据', async () => {
        vi.mocked(apiClient.getGlobalActivity).mockResolvedValue({
            daily_data: [],
            sum_api_requests: 0,
            sum_total_tokens: 0,
            total_cost: 0,
            avg_latency_ms: 0,
            success_rate: 0,
            unique_models: 0,
            unique_providers: 0,
        });

        const { result } = renderHook(() => useDashboardStats(), {
            wrapper: createWrapper(),
        });

        await waitFor(() => {
            expect(result.current.isLoading).toBe(false);
        });

        expect(result.current.totalRequests).toBe(0);
        expect(result.current.dailyData).toEqual([]);
    });

    it('应该支持日期范围参数', async () => {
        vi.mocked(apiClient.getGlobalActivity).mockResolvedValue({
            daily_data: [],
            sum_api_requests: 0,
            sum_total_tokens: 0,
            total_cost: 0,
            avg_latency_ms: 0,
            success_rate: 0,
            unique_models: 0,
            unique_providers: 0,
        });

        renderHook(
            () => useDashboardStats({ startDate: '2026-01-01', endDate: '2026-01-08' }),
            { wrapper: createWrapper() }
        );

        await waitFor(() => {
            expect(apiClient.getGlobalActivity).toHaveBeenCalledWith({
                start_date: '2026-01-01',
                end_date: '2026-01-08',
            });
        });
    });
});

describe('useModelSpend', () => {
    beforeEach(() => {
        vi.clearAllMocks();
    });

    it('应该正确获取模型消费数据', async () => {
        const mockModels = [
            { model: 'gpt-4', spend: 100, api_requests: 500, total_tokens: 200000 },
            { model: 'claude-3', spend: 50, api_requests: 300, total_tokens: 100000 },
        ];

        vi.mocked(apiClient.getSpendByModels).mockResolvedValue(mockModels);

        const { result } = renderHook(() => useModelSpend(), {
            wrapper: createWrapper(),
        });

        await waitFor(() => {
            expect(result.current.isLoading).toBe(false);
        });

        expect(result.current.models).toHaveLength(2);
        expect(result.current.models[0].model).toBe('gpt-4');
        expect(result.current.models[0].spend).toBe(100);
    });
});

describe('useApiKeys', () => {
    beforeEach(() => {
        vi.clearAllMocks();
    });

    it('应该正确获取 API Keys 列表', async () => {
        const mockKeys = {
            data: [
                {
                    id: 'key-1',
                    name: 'Test Key',
                    key_prefix: 'sk-test',
                    is_active: true,
                    blocked: false,
                    spent_budget: 10,
                },
            ],
            total: 1,
        };

        vi.mocked(apiClient.listKeys).mockResolvedValue(mockKeys);

        const { result } = renderHook(() => useApiKeys(), {
            wrapper: createWrapper(),
        });

        await waitFor(() => {
            expect(result.current.isLoading).toBe(false);
        });

        expect(result.current.keys).toHaveLength(1);
        expect(result.current.keys[0].name).toBe('Test Key');
        expect(result.current.total).toBe(1);
    });

    it('应该支持筛选参数', async () => {
        vi.mocked(apiClient.listKeys).mockResolvedValue({ data: [], total: 0 });

        renderHook(
            () => useApiKeys({ teamId: 'team-1', limit: 20 }),
            { wrapper: createWrapper() }
        );

        await waitFor(() => {
            expect(apiClient.listKeys).toHaveBeenCalledWith({
                team_id: 'team-1',
                user_id: undefined,
                organization_id: undefined,
                limit: 20,
                offset: 0,
            });
        });
    });

    it('createKey 应该创建密钥并刷新列表', async () => {
        vi.mocked(apiClient.listKeys).mockResolvedValue({ data: [], total: 0 });
        vi.mocked(apiClient.generateKey).mockResolvedValue({
            key: 'sk-full-key',
            key_prefix: 'sk-full',
            key_id: 'key-new',
        });

        const { result } = renderHook(() => useApiKeys(), {
            wrapper: createWrapper(),
        });

        await waitFor(() => {
            expect(result.current.isLoading).toBe(false);
        });

        const newKey = await result.current.createKey({ name: 'New Key' });

        expect(newKey.key).toBe('sk-full-key');
        expect(apiClient.generateKey).toHaveBeenCalledWith({ name: 'New Key' });
    });

    it('deleteKey 应该删除密钥', async () => {
        vi.mocked(apiClient.listKeys).mockResolvedValue({ data: [], total: 0 });
        vi.mocked(apiClient.deleteKeys).mockResolvedValue({ deleted_count: 1 });

        const { result } = renderHook(() => useApiKeys(), {
            wrapper: createWrapper(),
        });

        await waitFor(() => {
            expect(result.current.isLoading).toBe(false);
        });

        await result.current.deleteKey('key-1');

        expect(apiClient.deleteKeys).toHaveBeenCalledWith(['key-1']);
    });

    it('blockKey 应该封禁密钥', async () => {
        vi.mocked(apiClient.listKeys).mockResolvedValue({ data: [], total: 0 });
        vi.mocked(apiClient.blockKey).mockResolvedValue({ success: true });

        const { result } = renderHook(() => useApiKeys(), {
            wrapper: createWrapper(),
        });

        await waitFor(() => {
            expect(result.current.isLoading).toBe(false);
        });

        await result.current.blockKey('key-1');

        expect(apiClient.blockKey).toHaveBeenCalledWith('key-1');
    });
});

describe('useApiKeyInfo', () => {
    beforeEach(() => {
        vi.clearAllMocks();
    });

    it('应该获取单个 API Key 详情', async () => {
        const mockKey = {
            id: 'key-1',
            name: 'Test Key',
            key_prefix: 'sk-test',
            is_active: true,
            blocked: false,
            spent_budget: 10,
            max_budget: 100,
            created_at: '2026-01-01T00:00:00Z',
            updated_at: '2026-01-01T00:00:00Z',
        };

        vi.mocked(apiClient.getKeyInfo).mockResolvedValue(mockKey);

        const { result } = renderHook(() => useApiKeyInfo('key-1'), {
            wrapper: createWrapper(),
        });

        await waitFor(() => {
            expect(result.current.isLoading).toBe(false);
        });

        expect(result.current.key?.name).toBe('Test Key');
        expect(result.current.key?.max_budget).toBe(100);
    });

    it('当没有 keyId 时不应该发起请求', async () => {
        renderHook(() => useApiKeyInfo(undefined), {
            wrapper: createWrapper(),
        });

        // 等待一段时间确保没有请求发出
        await new Promise(resolve => setTimeout(resolve, 100));

        expect(apiClient.getKeyInfo).not.toHaveBeenCalled();
    });
});

describe('useTeams', () => {
    beforeEach(() => {
        vi.clearAllMocks();
    });

    it('应该正确获取团队列表', async () => {
        const mockTeams = {
            data: [
                {
                    team_id: 'team-1',
                    team_alias: 'Engineering',
                    is_active: true,
                    blocked: false,
                    spend: 100,
                    created_at: '2026-01-01T00:00:00Z',
                    updated_at: '2026-01-01T00:00:00Z',
                },
            ],
            total: 1,
        };

        vi.mocked(apiClient.listTeams).mockResolvedValue(mockTeams);

        const { result } = renderHook(() => useTeams(), {
            wrapper: createWrapper(),
        });

        await waitFor(() => {
            expect(result.current.isLoading).toBe(false);
        });

        expect(result.current.teams).toHaveLength(1);
        expect(result.current.teams[0].team_alias).toBe('Engineering');
    });

    it('createTeam 应该创建团队', async () => {
        vi.mocked(apiClient.listTeams).mockResolvedValue({ data: [], total: 0 });
        vi.mocked(apiClient.createTeam).mockResolvedValue({
            team_id: 'team-new',
            team_alias: 'New Team',
            is_active: true,
            blocked: false,
            spend: 0,
            created_at: '2026-01-09T00:00:00Z',
            updated_at: '2026-01-09T00:00:00Z',
        });

        const { result } = renderHook(() => useTeams(), {
            wrapper: createWrapper(),
        });

        await waitFor(() => {
            expect(result.current.isLoading).toBe(false);
        });

        const newTeam = await result.current.createTeam({ team_alias: 'New Team' });

        expect(newTeam.team_alias).toBe('New Team');
    });
});

describe('useTeamInfo', () => {
    beforeEach(() => {
        vi.clearAllMocks();
    });

    it('应该获取单个团队详情', async () => {
        const mockTeam = {
            team_id: 'team-1',
            team_alias: 'Engineering',
            members: ['user-1', 'user-2'],
            is_active: true,
            blocked: false,
            spend: 500,
            max_budget: 1000,
            created_at: '2026-01-01T00:00:00Z',
            updated_at: '2026-01-01T00:00:00Z',
        };

        vi.mocked(apiClient.getTeamInfo).mockResolvedValue(mockTeam);

        const { result } = renderHook(() => useTeamInfo('team-1'), {
            wrapper: createWrapper(),
        });

        await waitFor(() => {
            expect(result.current.isLoading).toBe(false);
        });

        expect(result.current.team?.team_alias).toBe('Engineering');
        expect(result.current.team?.members).toHaveLength(2);
    });
});
