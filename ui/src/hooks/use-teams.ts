/**
 * Teams Hook
 * 
 * 获取和管理团队数据
 * 数据来源: GET /team/list, POST /team/new, etc.
 */

import { useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/lib/api';
import type { Team, CreateTeamRequest, PaginatedResponse } from '@/types/api';

interface UseTeamsOptions {
    organizationId?: string;
    limit?: number;
    offset?: number;
}

interface TeamsReturn {
    teams: Team[];
    total: number;
    isLoading: boolean;
    error: Error | null;
    refresh: () => void;

    // 操作方法
    createTeam: (data: CreateTeamRequest) => Promise<Team>;
    deleteTeam: (teamId: string) => Promise<void>;
    blockTeam: (teamId: string) => Promise<void>;
    unblockTeam: (teamId: string) => Promise<void>;
}

export function useTeams(options: UseTeamsOptions = {}): TeamsReturn {
    const { organizationId, limit = 50, offset = 0 } = options;
    const queryClient = useQueryClient();

    const queryKey = ['teams', organizationId, limit, offset];

    const { data, error, isLoading, refetch } = useQuery<PaginatedResponse<Team>>({
        queryKey,
        queryFn: () => apiClient.listTeams({
            organization_id: organizationId,
            limit,
            offset,
        }),
        refetchOnWindowFocus: true,
        staleTime: 2000,
    });

    const invalidateQueries = async () => {
        await queryClient.invalidateQueries({ queryKey: ['teams'] });
    };

    const createTeam = async (createData: CreateTeamRequest): Promise<Team> => {
        const result = await apiClient.createTeam(createData);
        await invalidateQueries();
        return result;
    };

    const deleteTeam = async (teamId: string): Promise<void> => {
        await apiClient.deleteTeams([teamId]);
        await invalidateQueries();
    };

    const blockTeam = async (teamId: string): Promise<void> => {
        await apiClient.blockTeam(teamId);
        await invalidateQueries();
    };

    const unblockTeam = async (teamId: string): Promise<void> => {
        await apiClient.unblockTeam(teamId);
        await invalidateQueries();
    };

    return {
        teams: data?.data ?? [],
        total: data?.total ?? 0,
        isLoading,
        error: error ?? null,
        refresh: () => { refetch(); },
        createTeam,
        deleteTeam,
        blockTeam,
        unblockTeam,
    };
}

/**
 * 单个 Team 详情的 Hook
 */
export function useTeamInfo(teamId: string | undefined) {
    const { data, error, isLoading, refetch } = useQuery<Team>({
        queryKey: ['team-info', teamId],
        queryFn: () => apiClient.getTeamInfo(teamId!),
        enabled: !!teamId,
        refetchOnWindowFocus: true,
    });

    return {
        team: data,
        isLoading,
        error: error ?? null,
        refresh: () => { refetch(); },
    };
}
