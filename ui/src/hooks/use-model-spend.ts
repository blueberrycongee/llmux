/**
 * Model Spend Hook
 * 
 * 获取模型消费分布数据，用于饼图展示
 * 数据来源: GET /global/spend/models
 */

import { useQuery } from '@tanstack/react-query';
import { apiClient } from '@/lib/api';
import type { ModelSpend } from '@/types/api';

interface UseModelSpendOptions {
    startDate?: string;
    endDate?: string;
    limit?: number;
}

interface ModelSpendReturn {
    models: ModelSpend[];
    isLoading: boolean;
    error: Error | null;
    refresh: () => void;
}

export function useModelSpend(options: UseModelSpendOptions = {}): ModelSpendReturn {
    const { startDate, endDate, limit = 10 } = options;

    const { data, error, isLoading, refetch } = useQuery<ModelSpend[]>({
        queryKey: ['model-spend', startDate, endDate, limit],
        queryFn: () => apiClient.getSpendByModels({ start_date: startDate, end_date: endDate, limit }),
        refetchInterval: 60000,
        refetchOnWindowFocus: true,
        staleTime: 5000,
    });

    return {
        models: data ?? [],
        isLoading,
        error: error ?? null,
        refresh: () => { refetch(); },
    };
}
