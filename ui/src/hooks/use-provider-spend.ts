/**
 * Provider Spend Hook
 *
 * Fetch provider spend distribution for charts.
 * Data source: GET /global/spend/provider
 */

import { useQuery } from "@tanstack/react-query";
import { apiClient } from "@/lib/api";
import type { ProviderSpend } from "@/types/api";

interface UseProviderSpendOptions {
  startDate?: string;
  endDate?: string;
  limit?: number;
}

interface ProviderSpendReturn {
  providers: ProviderSpend[];
  isLoading: boolean;
  error: Error | null;
  refresh: () => void;
}

export function useProviderSpend(
  options: UseProviderSpendOptions = {}
): ProviderSpendReturn {
  const { startDate, endDate, limit = 10 } = options;

  const { data, error, isLoading, refetch } = useQuery<ProviderSpend[]>({
    queryKey: ["provider-spend", startDate, endDate, limit],
    queryFn: () =>
      apiClient.getSpendByProviders({
        start_date: startDate,
        end_date: endDate,
        limit,
      }),
    refetchInterval: 60000,
    refetchOnWindowFocus: true,
    staleTime: 5000,
  });

  return {
    providers: data ?? [],
    isLoading,
    error: error ?? null,
    refresh: () => {
      refetch();
    },
  };
}

