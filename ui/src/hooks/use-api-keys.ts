/**
 * API Keys Hook
 * 
 * 获取和管理 API 密钥
 * 数据来源: GET /key/list, POST /key/generate, etc.
 */

import { useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/lib/api';
import type { APIKey, GenerateKeyRequest, GenerateKeyResponse, PaginatedResponse } from '@/types/api';

interface UseApiKeysOptions {
    teamId?: string;
    userId?: string;
    organizationId?: string;
    limit?: number;
    offset?: number;
}

interface ApiKeysReturn {
    keys: APIKey[];
    total: number;
    isLoading: boolean;
    error: Error | null;
    refresh: () => void;

    // 操作方法
    createKey: (data: GenerateKeyRequest) => Promise<GenerateKeyResponse>;
    deleteKey: (keyId: string) => Promise<void>;
    blockKey: (keyId: string) => Promise<void>;
    unblockKey: (keyId: string) => Promise<void>;
    regenerateKey: (keyId: string) => Promise<GenerateKeyResponse>;
}

export function useApiKeys(options: UseApiKeysOptions = {}): ApiKeysReturn {
    const { teamId, userId, organizationId, limit = 50, offset = 0 } = options;
    const queryClient = useQueryClient();

    const queryKey = ['api-keys', teamId, userId, organizationId, limit, offset];

    const { data, error, isLoading, refetch } = useQuery<PaginatedResponse<APIKey>>({
        queryKey,
        queryFn: () => apiClient.listKeys({
            team_id: teamId,
            user_id: userId,
            organization_id: organizationId,
            limit,
            offset,
        }),
        refetchOnWindowFocus: true,
        staleTime: 2000,
    });

    const invalidateQueries = async () => {
        await queryClient.invalidateQueries({ queryKey: ['api-keys'] });
    };

    const createKey = async (createData: GenerateKeyRequest): Promise<GenerateKeyResponse> => {
        const result = await apiClient.generateKey(createData);
        await invalidateQueries();
        return result;
    };

    const deleteKey = async (keyId: string): Promise<void> => {
        await apiClient.deleteKeys([keyId]);
        await invalidateQueries();
    };

    const blockKey = async (keyId: string): Promise<void> => {
        await apiClient.blockKey(keyId);
        await invalidateQueries();
    };

    const unblockKey = async (keyId: string): Promise<void> => {
        await apiClient.unblockKey(keyId);
        await invalidateQueries();
    };

    const regenerateKey = async (keyId: string): Promise<GenerateKeyResponse> => {
        const result = await apiClient.regenerateKey(keyId);
        await invalidateQueries();
        return result;
    };

    return {
        keys: data?.data ?? [],
        total: data?.total ?? 0,
        isLoading,
        error: error ?? null,
        refresh: () => { refetch(); },
        createKey,
        deleteKey,
        blockKey,
        unblockKey,
        regenerateKey,
    };
}

/**
 * 单个 API Key 详情的 Hook
 */
export function useApiKeyInfo(keyId: string | undefined) {
    const { data, error, isLoading, refetch } = useQuery<APIKey>({
        queryKey: ['api-key-info', keyId],
        queryFn: () => apiClient.getKeyInfo(keyId!),
        enabled: !!keyId,
        refetchOnWindowFocus: true,
    });

    return {
        key: data,
        isLoading,
        error: error ?? null,
        refresh: () => { refetch(); },
    };
}
