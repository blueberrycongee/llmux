/**
 * Team Members Hook
 * 
 * 获取和管理团队成员数据
 * 数据来源: POST /team/member_add, POST /team/member_delete
 */

import { useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/lib/api';

interface UseTeamMembersOptions {
    teamId: string;
}

interface TeamMembersReturn {
    // 操作方法
    addMember: (userId: string, role?: string) => Promise<void>;
    removeMember: (userId: string) => Promise<void>;
}

export function useTeamMembers(options: UseTeamMembersOptions): TeamMembersReturn {
    const { teamId } = options;
    const queryClient = useQueryClient();

    const invalidateQueries = async () => {
        await queryClient.invalidateQueries({ queryKey: ['team-info', teamId] });
        await queryClient.invalidateQueries({ queryKey: ['teams'] });
        await queryClient.invalidateQueries({ queryKey: ['users'] });
    };

    const addMember = async (userId: string, role?: string): Promise<void> => {
        await apiClient.addTeamMember(teamId, userId, role);
        await invalidateQueries();
    };

    const removeMember = async (userId: string): Promise<void> => {
        await apiClient.removeTeamMember(teamId, userId);
        await invalidateQueries();
    };

    return {
        addMember,
        removeMember,
    };
}
