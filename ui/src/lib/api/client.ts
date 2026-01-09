/**
 * LLMux API Client
 * 
 * 统一的 API 客户端，封装所有后端 API 调用
 * 源码参考: internal/api/routes.go
 */

import type {
    APIKey,
    GenerateKeyRequest,
    GenerateKeyResponse,
    Team,
    CreateTeamRequest,
    User,
    CreateUserRequest,
    Organization,
    OrganizationMembership,
    CreateOrganizationRequest,
    SpendLogsResponse,
    KeySpend,
    TeamSpend,
    UserSpend,
    GlobalActivityResponse,
    ModelSpend,
    ProviderSpend,
    AuditLog,
    AuditLogsResponse,
    AuditStats,
    InvitationLink,
    CreateInvitationRequest,
    ApiError,
    PaginatedResponse,
} from '@/types/api';

// ===== Configuration =====

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

// ===== Error Handling =====

export class LLMuxApiError extends Error {
    public readonly type: string;
    public readonly code?: string;
    public readonly status: number;

    constructor(message: string, type: string, status: number, code?: string) {
        super(message);
        this.name = 'LLMuxApiError';
        this.type = type;
        this.status = status;
        this.code = code;
    }
}

// ===== API Client Class =====

class LLMuxApiClient {
    private baseUrl: string;
    private token: string | null = null;

    constructor(baseUrl: string = API_BASE_URL) {
        this.baseUrl = baseUrl;
    }

    /**
     * Set the authentication token for API requests
     */
    setToken(token: string): void {
        this.token = token;
    }

    /**
     * Clear the authentication token
     */
    clearToken(): void {
        this.token = null;
    }

    /**
     * Get the current base URL
     */
    getBaseUrl(): string {
        return this.baseUrl;
    }

    /**
     * Internal request method with error handling
     */
    private async request<T>(
        method: string,
        path: string,
        body?: unknown,
        params?: Record<string, string | number | boolean | undefined>
    ): Promise<T> {
        const url = new URL(path, this.baseUrl);

        // Add query parameters
        if (params) {
            Object.entries(params).forEach(([key, value]) => {
                if (value !== undefined && value !== null) {
                    url.searchParams.set(key, String(value));
                }
            });
        }

        // Build headers
        const headers: HeadersInit = {
            'Content-Type': 'application/json',
        };
        if (this.token) {
            headers['Authorization'] = `Bearer ${this.token}`;
        }

        // Make request
        const response = await fetch(url.toString(), {
            method,
            headers,
            body: body ? JSON.stringify(body) : undefined,
        });

        // Handle error responses
        if (!response.ok) {
            let errorData: ApiError;
            try {
                errorData = await response.json();
            } catch {
                throw new LLMuxApiError(
                    `HTTP ${response.status}: ${response.statusText}`,
                    'http_error',
                    response.status
                );
            }
            throw new LLMuxApiError(
                errorData.error?.message || 'Unknown API error',
                errorData.error?.type || 'api_error',
                response.status,
                errorData.error?.code
            );
        }

        // Handle empty responses
        const contentType = response.headers.get('content-type');
        if (!contentType || !contentType.includes('application/json')) {
            return {} as T;
        }

        return response.json();
    }

    // ===== Key Management =====
    // 源码: internal/api/management.go L79-523

    /**
     * Generate a new API key
     * POST /key/generate
     */
    generateKey(data: GenerateKeyRequest): Promise<GenerateKeyResponse> {
        return this.request<GenerateKeyResponse>('POST', '/key/generate', data);
    }

    /**
     * List API keys with optional filters
     * GET /key/list
     */
    listKeys(params?: {
        team_id?: string;
        user_id?: string;
        organization_id?: string;
        limit?: number;
        offset?: number;
    }): Promise<PaginatedResponse<APIKey>> {
        return this.request<PaginatedResponse<APIKey>>('GET', '/key/list', undefined, params);
    }

    /**
     * Get API key info by key ID or key prefix
     * GET /key/info
     */
    getKeyInfo(key: string): Promise<APIKey> {
        return this.request<APIKey>('GET', '/key/info', undefined, { key });
    }

    /**
     * Update an API key
     * POST /key/update
     */
    updateKey(keyId: string, updates: Partial<GenerateKeyRequest>): Promise<APIKey> {
        return this.request<APIKey>('POST', '/key/update', { key: keyId, ...updates });
    }

    /**
     * Delete API keys
     * POST /key/delete
     */
    deleteKeys(keys: string[]): Promise<{ deleted_count: number }> {
        return this.request<{ deleted_count: number }>('POST', '/key/delete', { keys });
    }

    /**
     * Block an API key
     * POST /key/block
     */
    blockKey(key: string): Promise<{ success: boolean }> {
        return this.request<{ success: boolean }>('POST', '/key/block', { key });
    }

    /**
     * Unblock an API key
     * POST /key/unblock
     */
    unblockKey(key: string): Promise<{ success: boolean }> {
        return this.request<{ success: boolean }>('POST', '/key/unblock', { key });
    }

    /**
     * Regenerate an API key (creates new key with same config)
     * POST /key/regenerate
     */
    regenerateKey(key: string): Promise<GenerateKeyResponse> {
        return this.request<GenerateKeyResponse>('POST', '/key/regenerate', { key });
    }

    // ===== Team Management =====
    // 源码: internal/api/team_endpoints.go L44-394

    /**
     * Create a new team
     * POST /team/new
     */
    createTeam(data: CreateTeamRequest): Promise<Team> {
        return this.request<Team>('POST', '/team/new', data);
    }

    /**
     * List teams
     * GET /team/list
     */
    listTeams(params?: {
        organization_id?: string;
        limit?: number;
        offset?: number;
    }): Promise<PaginatedResponse<Team>> {
        return this.request<PaginatedResponse<Team>>('GET', '/team/list', undefined, params);
    }

    /**
     * Get team info by ID
     * GET /team/info
     */
    getTeamInfo(teamId: string): Promise<Team> {
        return this.request<Team>('GET', '/team/info', undefined, { team_id: teamId });
    }

    /**
     * Update a team
     * POST /team/update
     */
    updateTeam(teamId: string, updates: Partial<CreateTeamRequest>): Promise<Team> {
        return this.request<Team>('POST', '/team/update', { team_id: teamId, ...updates });
    }

    /**
     * Delete teams
     * POST /team/delete
     */
    deleteTeams(teamIds: string[]): Promise<{ deleted_count: number }> {
        return this.request<{ deleted_count: number }>('POST', '/team/delete', { team_ids: teamIds });
    }

    /**
     * Block a team
     * POST /team/block
     */
    blockTeam(teamId: string): Promise<{ success: boolean }> {
        return this.request<{ success: boolean }>('POST', '/team/block', { team_id: teamId });
    }

    /**
     * Unblock a team
     * POST /team/unblock
     */
    unblockTeam(teamId: string): Promise<{ success: boolean }> {
        return this.request<{ success: boolean }>('POST', '/team/unblock', { team_id: teamId });
    }

    /**
     * Add member to team
     * POST /team/member_add
     */
    addTeamMember(teamId: string, userId: string, role?: string): Promise<{ success: boolean }> {
        return this.request<{ success: boolean }>('POST', '/team/member_add', {
            team_id: teamId,
            user_id: userId,
            role,
        });
    }

    /**
     * Remove member from team
     * POST /team/member_delete
     */
    removeTeamMember(teamId: string, userId: string): Promise<{ success: boolean }> {
        return this.request<{ success: boolean }>('POST', '/team/member_delete', {
            team_id: teamId,
            user_id: userId,
        });
    }

    // ===== User Management =====
    // 源码: internal/api/user_endpoints.go L37-269

    /**
     * Create a new user
     * POST /user/new
     */
    createUser(data: CreateUserRequest): Promise<User> {
        return this.request<User>('POST', '/user/new', data);
    }

    /**
     * List users
     * GET /user/list
     */
    listUsers(params?: {
        team_id?: string;
        organization_id?: string;
        role?: string;
        search?: string;
        limit?: number;
        offset?: number;
    }): Promise<PaginatedResponse<User>> {
        return this.request<PaginatedResponse<User>>('GET', '/user/list', undefined, params);
    }

    /**
     * Get user info by ID
     * GET /user/info
     */
    getUserInfo(userId: string): Promise<User> {
        return this.request<User>('GET', '/user/info', undefined, { user_id: userId });
    }

    /**
     * Update a user
     * POST /user/update
     */
    updateUser(userId: string, updates: Partial<CreateUserRequest>): Promise<User> {
        return this.request<User>('POST', '/user/update', { user_id: userId, ...updates });
    }

    /**
     * Delete users
     * POST /user/delete
     */
    deleteUsers(userIds: string[]): Promise<{ deleted_count: number }> {
        return this.request<{ deleted_count: number }>('POST', '/user/delete', { user_ids: userIds });
    }

    // ===== Organization Management =====
    // 源码: internal/api/organization_endpoints.go L32-485

    /**
     * Create a new organization
     * POST /organization/new
     */
    createOrganization(data: CreateOrganizationRequest): Promise<Organization> {
        return this.request<Organization>('POST', '/organization/new', data);
    }

    /**
     * List organizations
     * GET /organization/list
     */
    listOrganizations(params?: {
        limit?: number;
        offset?: number;
    }): Promise<PaginatedResponse<Organization>> {
        return this.request<PaginatedResponse<Organization>>('GET', '/organization/list', undefined, params);
    }

    /**
     * Get organization info by ID
     * GET /organization/info
     */
    getOrganizationInfo(organizationId: string): Promise<Organization> {
        return this.request<Organization>('GET', '/organization/info', undefined, { organization_id: organizationId });
    }

    /**
     * Update an organization
     * PATCH /organization/update
     */
    updateOrganization(organizationId: string, updates: Partial<CreateOrganizationRequest>): Promise<Organization> {
        return this.request<Organization>('PATCH', '/organization/update', { organization_id: organizationId, ...updates });
    }

    /**
     * Delete an organization
     * DELETE /organization/delete
     */
    deleteOrganization(organizationId: string): Promise<{ success: boolean }> {
        return this.request<{ success: boolean }>('DELETE', '/organization/delete', { organization_id: organizationId });
    }

    /**
     * List organization members
     * GET /organization/members
     */
    listOrganizationMembers(organizationId: string): Promise<OrganizationMembership[]> {
        return this.request<OrganizationMembership[]>('GET', '/organization/members', undefined, { organization_id: organizationId });
    }

    /**
     * Add member to organization
     * POST /organization/member_add
     */
    addOrganizationMember(organizationId: string, userId: string, role?: string): Promise<{ success: boolean }> {
        return this.request<{ success: boolean }>('POST', '/organization/member_add', {
            organization_id: organizationId,
            user_id: userId,
            role,
        });
    }

    /**
     * Update organization member
     * POST /organization/member_update
     */
    updateOrganizationMember(organizationId: string, userId: string, role: string): Promise<{ success: boolean }> {
        return this.request<{ success: boolean }>('POST', '/organization/member_update', {
            organization_id: organizationId,
            user_id: userId,
            role,
        });
    }

    /**
     * Remove member from organization
     * POST /organization/member_delete
     */
    removeOrganizationMember(organizationId: string, userId: string): Promise<{ success: boolean }> {
        return this.request<{ success: boolean }>('POST', '/organization/member_delete', {
            organization_id: organizationId,
            user_id: userId,
        });
    }

    // ===== Spend Tracking =====
    // 源码: internal/api/spend_endpoints.go L16-202

    /**
     * Get spend logs with filters
     * GET /spend/logs
     */
    getSpendLogs(params?: {
        key_id?: string;
        team_id?: string;
        user_id?: string;
        start_date?: string;
        end_date?: string;
        limit?: number;
    }): Promise<SpendLogsResponse> {
        return this.request<SpendLogsResponse>('GET', '/spend/logs', undefined, params);
    }

    /**
     * Get spend by API keys
     * GET /spend/keys
     */
    getSpendByKeys(params?: {
        team_id?: string;
        start_date?: string;
        end_date?: string;
        limit?: number;
    }): Promise<KeySpend[]> {
        return this.request<KeySpend[]>('GET', '/spend/keys', undefined, params);
    }

    /**
     * Get spend by teams
     * GET /spend/teams
     */
    getSpendByTeams(params?: {
        organization_id?: string;
        start_date?: string;
        end_date?: string;
        limit?: number;
    }): Promise<TeamSpend[]> {
        return this.request<TeamSpend[]>('GET', '/spend/teams', undefined, params);
    }

    /**
     * Get spend by users
     * GET /spend/users
     */
    getSpendByUsers(params?: {
        team_id?: string;
        organization_id?: string;
        start_date?: string;
        end_date?: string;
        limit?: number;
    }): Promise<UserSpend[]> {
        return this.request<UserSpend[]>('GET', '/spend/users', undefined, params);
    }

    // ===== Global Analytics =====
    // 源码: internal/api/spend_endpoints.go L204-408

    /**
     * Get global activity metrics
     * GET /global/activity
     */
    getGlobalActivity(params?: {
        start_date?: string;
        end_date?: string;
    }): Promise<GlobalActivityResponse> {
        return this.request<GlobalActivityResponse>('GET', '/global/activity', undefined, params);
    }

    /**
     * Get spend by models
     * GET /global/spend/models
     */
    getSpendByModels(params?: {
        start_date?: string;
        end_date?: string;
        limit?: number;
    }): Promise<ModelSpend[]> {
        return this.request<ModelSpend[]>('GET', '/global/spend/models', undefined, params);
    }

    /**
     * Get spend by providers
     * GET /global/spend/provider
     */
    getSpendByProviders(params?: {
        start_date?: string;
        end_date?: string;
        limit?: number;
    }): Promise<ProviderSpend[]> {
        return this.request<ProviderSpend[]>('GET', '/global/spend/provider', undefined, params);
    }

    // ===== Audit Logs =====
    // 源码: internal/api/audit_endpoints.go L39-217

    /**
     * List audit logs
     * GET /audit/logs
     */
    getAuditLogs(params?: {
        actor_id?: string;
        action?: string;
        object_type?: string;
        object_id?: string;
        team_id?: string;
        organization_id?: string;
        start_time?: string;
        end_time?: string;
        limit?: number;
        offset?: number;
    }): Promise<AuditLogsResponse> {
        return this.request<AuditLogsResponse>('GET', '/audit/logs', undefined, params);
    }

    /**
     * Get single audit log entry
     * GET /audit/log
     */
    getAuditLog(logId: string): Promise<AuditLog> {
        return this.request<AuditLog>('GET', '/audit/log', undefined, { id: logId });
    }

    /**
     * Get audit statistics
     * GET /audit/stats
     */
    getAuditStats(params?: {
        start_time?: string;
        end_time?: string;
    }): Promise<AuditStats> {
        return this.request<AuditStats>('GET', '/audit/stats', undefined, params);
    }

    /**
     * Delete old audit logs
     * POST /audit/delete
     */
    deleteAuditLogs(olderThanDays: number): Promise<{ deleted_count: number }> {
        return this.request<{ deleted_count: number }>('POST', '/audit/delete', { older_than_days: olderThanDays });
    }

    // ===== Invitation Links =====
    // 源码: internal/api/invitation_endpoints.go L73-312

    /**
     * Create a new invitation link
     * POST /invitation/new
     */
    createInvitation(data: CreateInvitationRequest): Promise<InvitationLink> {
        return this.request<InvitationLink>('POST', '/invitation/new', data);
    }

    /**
     * Accept an invitation
     * POST /invitation/accept
     */
    acceptInvitation(token: string): Promise<{ success: boolean; team_id?: string; organization_id?: string }> {
        return this.request<{ success: boolean; team_id?: string; organization_id?: string }>('POST', '/invitation/accept', { token });
    }

    /**
     * Get invitation info
     * GET /invitation/info
     */
    getInvitationInfo(invitationId: string): Promise<InvitationLink> {
        return this.request<InvitationLink>('GET', '/invitation/info', undefined, { id: invitationId });
    }

    /**
     * List invitations
     * GET /invitation/list
     */
    listInvitations(params?: {
        team_id?: string;
        organization_id?: string;
        is_active?: boolean;
        limit?: number;
        offset?: number;
    }): Promise<PaginatedResponse<InvitationLink>> {
        return this.request<PaginatedResponse<InvitationLink>>('GET', '/invitation/list', undefined, params);
    }

    /**
     * Deactivate an invitation
     * POST /invitation/deactivate
     */
    deactivateInvitation(invitationId: string): Promise<{ success: boolean }> {
        return this.request<{ success: boolean }>('POST', '/invitation/deactivate', { id: invitationId });
    }

    /**
     * Delete an invitation
     * POST /invitation/delete
     */
    deleteInvitation(invitationId: string): Promise<{ success: boolean }> {
        return this.request<{ success: boolean }>('POST', '/invitation/delete', { id: invitationId });
    }
}

// ===== Export Singleton Instance =====

export const apiClient = new LLMuxApiClient();

// Export the class for testing or custom instances
export { LLMuxApiClient };
