/**
 * Organizations Hook
 * 
 * 获取和管理组织数据
 * 数据来源: GET /organization/list, POST /organization/new, etc.
 */

import { useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/lib/api';
import type {
    Organization,
    CreateOrganizationRequest,
    OrganizationMembership,
    PaginatedResponse
} from '@/types/api';

interface UseOrganizationsOptions {
    limit?: number;
    offset?: number;
}

interface OrganizationsReturn {
    organizations: Organization[];
    total: number;
    isLoading: boolean;
    error: Error | null;
    refresh: () => void;

    // 操作方法
    createOrganization: (data: CreateOrganizationRequest) => Promise<Organization>;
    deleteOrganization: (orgId: string) => Promise<void>;
    updateOrganization: (orgId: string, updates: Partial<CreateOrganizationRequest>) => Promise<Organization>;
}

export function useOrganizations(options: UseOrganizationsOptions = {}): OrganizationsReturn {
    const { limit = 50, offset = 0 } = options;
    const queryClient = useQueryClient();

    const queryKey = ['organizations', limit, offset];

    const { data, error, isLoading, refetch } = useQuery<PaginatedResponse<Organization>>({
        queryKey,
        queryFn: () => apiClient.listOrganizations({
            limit,
            offset,
        }),
        refetchOnWindowFocus: true,
        staleTime: 2000,
    });

    const invalidateQueries = async () => {
        await queryClient.invalidateQueries({ queryKey: ['organizations'] });
    };

    const createOrganization = async (createData: CreateOrganizationRequest): Promise<Organization> => {
        const result = await apiClient.createOrganization(createData);
        await invalidateQueries();
        return result;
    };

    const deleteOrganization = async (orgId: string): Promise<void> => {
        await apiClient.deleteOrganization(orgId);
        await invalidateQueries();
    };

    const updateOrganization = async (
        orgId: string,
        updates: Partial<CreateOrganizationRequest>
    ): Promise<Organization> => {
        const result = await apiClient.updateOrganization(orgId, updates);
        await invalidateQueries();
        return result;
    };

    return {
        organizations: data?.data ?? [],
        total: data?.total ?? 0,
        isLoading,
        error: error ?? null,
        refresh: () => { refetch(); },
        createOrganization,
        deleteOrganization,
        updateOrganization,
    };
}

/**
 * 单个 Organization 详情的 Hook
 */
export function useOrganizationInfo(orgId: string | undefined) {
    const queryClient = useQueryClient();

    const { data, error, isLoading, refetch } = useQuery<Organization>({
        queryKey: ['organization-info', orgId],
        queryFn: () => apiClient.getOrganizationInfo(orgId!),
        enabled: !!orgId,
        refetchOnWindowFocus: true,
    });

    const invalidateQueries = async () => {
        await queryClient.invalidateQueries({ queryKey: ['organization-info', orgId] });
        await queryClient.invalidateQueries({ queryKey: ['organizations'] });
    };

    const updateOrganization = async (updates: Partial<CreateOrganizationRequest>): Promise<Organization> => {
        const result = await apiClient.updateOrganization(orgId!, updates);
        await invalidateQueries();
        return result;
    };

    return {
        organization: data,
        isLoading,
        error: error ?? null,
        refresh: () => { refetch(); },
        updateOrganization,
    };
}

/**
 * 组织成员管理 Hook
 */
export function useOrganizationMembers(orgId: string | undefined) {
    const queryClient = useQueryClient();

    const { data, error, isLoading, refetch } = useQuery<OrganizationMembership[]>({
        queryKey: ['organization-members', orgId],
        queryFn: () => apiClient.listOrganizationMembers(orgId!),
        enabled: !!orgId,
        refetchOnWindowFocus: true,
    });

    const invalidateQueries = async () => {
        await queryClient.invalidateQueries({ queryKey: ['organization-members', orgId] });
    };

    const addMember = async (userId: string, role?: string): Promise<void> => {
        await apiClient.addOrganizationMember(orgId!, userId, role);
        await invalidateQueries();
    };

    const removeMember = async (userId: string): Promise<void> => {
        await apiClient.removeOrganizationMember(orgId!, userId);
        await invalidateQueries();
    };

    const updateMemberRole = async (userId: string, role: string): Promise<void> => {
        await apiClient.updateOrganizationMember(orgId!, userId, role);
        await invalidateQueries();
    };

    return {
        members: data ?? [],
        isLoading,
        error: error ?? null,
        refresh: () => { refetch(); },
        addMember,
        removeMember,
        updateMemberRole,
    };
}
