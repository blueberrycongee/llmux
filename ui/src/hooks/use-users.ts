/**
 * Users Hook
 * 
 * 获取和管理用户数据
 * 数据来源: GET /user/list, POST /user/new, etc.
 */

import { useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/lib/api';
import type { User, CreateUserRequest, PaginatedResponse } from '@/types/api';

interface UseUsersOptions {
    teamId?: string;
    organizationId?: string;
    role?: string;
    search?: string;
    limit?: number;
    offset?: number;
}

interface UsersReturn {
    users: User[];
    total: number;
    isLoading: boolean;
    error: Error | null;
    refresh: () => void;

    // 操作方法
    createUser: (data: CreateUserRequest) => Promise<User>;
    deleteUser: (userId: string) => Promise<void>;
    updateUser: (userId: string, updates: Partial<CreateUserRequest>) => Promise<User>;
}

export function useUsers(options: UseUsersOptions = {}): UsersReturn {
    const { teamId, organizationId, role, search, limit = 50, offset = 0 } = options;
    const queryClient = useQueryClient();

    const queryKey = ['users', teamId, organizationId, role, search, limit, offset];

    const { data, error, isLoading, refetch } = useQuery<PaginatedResponse<User>>({
        queryKey,
        queryFn: () => apiClient.listUsers({
            team_id: teamId,
            organization_id: organizationId,
            role,
            search,
            limit,
            offset,
        }),
        refetchOnWindowFocus: true,
        staleTime: 2000,
    });

    const invalidateQueries = async () => {
        await queryClient.invalidateQueries({ queryKey: ['users'] });
    };

    const createUser = async (createData: CreateUserRequest): Promise<User> => {
        const result = await apiClient.createUser(createData);
        await invalidateQueries();
        return result;
    };

    const deleteUser = async (userId: string): Promise<void> => {
        await apiClient.deleteUsers([userId]);
        await invalidateQueries();
    };

    const updateUser = async (userId: string, updates: Partial<CreateUserRequest>): Promise<User> => {
        const result = await apiClient.updateUser(userId, updates);
        await invalidateQueries();
        return result;
    };

    return {
        users: data?.data ?? [],
        total: data?.total ?? 0,
        isLoading,
        error: error ?? null,
        refresh: () => { refetch(); },
        createUser,
        deleteUser,
        updateUser,
    };
}

/**
 * 单个 User 详情的 Hook
 */
export function useUserInfo(userId: string | undefined) {
    const queryClient = useQueryClient();

    const { data, error, isLoading, refetch } = useQuery<User>({
        queryKey: ['user-info', userId],
        queryFn: () => apiClient.getUserInfo(userId!),
        enabled: !!userId,
        refetchOnWindowFocus: true,
    });

    const invalidateQueries = async () => {
        await queryClient.invalidateQueries({ queryKey: ['user-info', userId] });
        await queryClient.invalidateQueries({ queryKey: ['users'] });
    };

    const updateUser = async (updates: Partial<CreateUserRequest>): Promise<User> => {
        const result = await apiClient.updateUser(userId!, updates);
        await invalidateQueries();
        return result;
    };

    return {
        user: data,
        isLoading,
        error: error ?? null,
        refresh: () => { refetch(); },
        updateUser,
    };
}
