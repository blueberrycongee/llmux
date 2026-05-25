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
    SandboxState,
    RoutingMemoryItem,
    SchedulingAdvisor,
    RoutingOptimizationComparison,
    TrafficSimulationResponse,
    RealTrafficRunResponse,
    ConversationAgent,
    AgentTeam,
    TeamMemoryRecord,
    AgentTeamRehearsalResponse,
    RouteMemoryRecord,
    AgentChatResponse,
    ConversationLabResponse,
    AgentToolOption,
    ToolMarketplaceItem,
    ToolPreset,
} from '@/types/api';

import { LOCALE_COOKIE, LOCALE_STORAGE_KEY, localeToHtmlLang, normalizeLocale, type AppLocale } from "@/i18n/i18n";

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8081';

function readCookie(name: string): string | null {
    if (typeof document === "undefined") return null;
    const match = document.cookie.match(new RegExp(`(?:^|;\\s*)${name}=([^;]+)`));
    return match ? decodeURIComponent(match[1]) : null;
}

function getPreferredLocale(): AppLocale {
    const fromCookie = readCookie(LOCALE_COOKIE);
    if (fromCookie) return normalizeLocale(fromCookie);
    if (typeof window !== "undefined") {
        try {
            const fromStorage = window.localStorage.getItem(LOCALE_STORAGE_KEY);
            if (fromStorage) return normalizeLocale(fromStorage);
        } catch {
            // ignore
        }
    }
    return normalizeLocale(null);
}

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

class LLMuxApiClient {
    private baseUrl: string;
    private token: string | null = null;

    constructor(baseUrl?: string) {
        if (baseUrl) {
            this.baseUrl = baseUrl;
        } else if (API_BASE_URL) {
            // Prefer the explicit backend origin in development to avoid Next.js
            // dev-server proxy resets on long/expensive management requests.
            this.baseUrl = API_BASE_URL;
        } else if (typeof window !== 'undefined') {
            this.baseUrl = window.location.origin;
        } else {
            this.baseUrl = 'http://localhost:8081';
        }
    }

    setToken(token: string): void { this.token = token; }
    clearToken(): void { this.token = null; }
    getBaseUrl(): string { return this.baseUrl; }

    private async request<T>(method: string, path: string, body?: unknown, params?: Record<string, string | number | boolean | undefined>): Promise<T> {
        const url = new URL(path, this.baseUrl);
        if (params) Object.entries(params).forEach(([key, value]) => { if (value !== undefined && value !== null) url.searchParams.set(key, String(value)); });
        const headers: HeadersInit = { 'Content-Type': 'application/json' };
        const locale = getPreferredLocale();
        headers['Accept-Language'] = localeToHtmlLang(locale);
        headers['X-LLMux-Locale'] = locale;
        if (this.token) headers['Authorization'] = `Bearer ${this.token}`;
        const response = await fetch(url.toString(), { method, headers, body: body ? JSON.stringify(body) : undefined });
        if (!response.ok) {
            let errorData: ApiError;
            try { errorData = await response.json(); } catch { throw new LLMuxApiError(`HTTP ${response.status}: ${response.statusText}`, 'http_error', response.status); }
            throw new LLMuxApiError(errorData.error?.message || 'Unknown API error', errorData.error?.type || 'api_error', response.status, errorData.error?.code);
        }
        const contentType = response.headers.get('content-type');
        if (!contentType || !contentType.includes('application/json')) return {} as T;
        return response.json();
    }

    generateKey(data: GenerateKeyRequest): Promise<GenerateKeyResponse> { return this.request('POST', '/key/generate', data); }
    listKeys(params?: { team_id?: string; user_id?: string; organization_id?: string; limit?: number; offset?: number; }): Promise<PaginatedResponse<APIKey>> { return this.request('GET', '/key/list', undefined, params); }
    getKeyInfo(key: string): Promise<APIKey> { return this.request('GET', '/key/info', undefined, { key }); }
    updateKey(keyId: string, updates: Partial<GenerateKeyRequest>): Promise<APIKey> { return this.request('POST', '/key/update', { key: keyId, ...updates }); }
    deleteKeys(keys: string[]): Promise<{ deleted_count: number }> { return this.request('POST', '/key/delete', { keys }); }
    blockKey(key: string): Promise<{ success: boolean }> { return this.request('POST', '/key/block', { key }); }
    unblockKey(key: string): Promise<{ success: boolean }> { return this.request('POST', '/key/unblock', { key }); }
    regenerateKey(key: string): Promise<GenerateKeyResponse> { return this.request('POST', '/key/regenerate', { key }); }
    createTeam(data: CreateTeamRequest): Promise<Team> { return this.request('POST', '/team/new', data); }
    listTeams(params?: { organization_id?: string; limit?: number; offset?: number; }): Promise<PaginatedResponse<Team>> { return this.request('GET', '/team/list', undefined, params); }
    getTeamInfo(teamId: string): Promise<Team> { return this.request('GET', '/team/info', undefined, { team_id: teamId }); }
    updateTeam(teamId: string, updates: Partial<CreateTeamRequest>): Promise<Team> { return this.request('POST', '/team/update', { team_id: teamId, ...updates }); }
    deleteTeams(teamIds: string[]): Promise<{ deleted_count: number }> { return this.request('POST', '/team/delete', { team_ids: teamIds }); }
    blockTeam(teamId: string): Promise<{ success: boolean }> { return this.request('POST', '/team/block', { team_id: teamId }); }
    unblockTeam(teamId: string): Promise<{ success: boolean }> { return this.request('POST', '/team/unblock', { team_id: teamId }); }
    addTeamMember(teamId: string, userId: string, role?: string): Promise<{ success: boolean }> { return this.request('POST', '/team/member_add', { team_id: teamId, user_id: userId, role }); }
    removeTeamMember(teamId: string, userId: string): Promise<{ success: boolean }> { return this.request('POST', '/team/member_delete', { team_id: teamId, user_id: userId }); }
    createUser(data: CreateUserRequest): Promise<User> { return this.request('POST', '/user/new', data); }
    listUsers(params?: { team_id?: string; organization_id?: string; limit?: number; offset?: number; }): Promise<PaginatedResponse<User>> { return this.request('GET', '/user/list', undefined, params); }
    getUserInfo(userId: string): Promise<User> { return this.request('GET', '/user/info', undefined, { user_id: userId }); }
    updateUser(userId: string, updates: Partial<CreateUserRequest>): Promise<User> { return this.request('POST', '/user/update', { user_id: userId, ...updates }); }
    deleteUsers(userIds: string[]): Promise<{ deleted_count: number }> { return this.request('POST', '/user/delete', { user_ids: userIds }); }
    createOrganization(data: CreateOrganizationRequest): Promise<Organization> { return this.request('POST', '/organization/new', data); }
    listOrganizations(params?: { limit?: number; offset?: number; }): Promise<PaginatedResponse<Organization>> { return this.request('GET', '/organization/list', undefined, params); }
    getSpendLogs(params?: { key_id?: string; team_id?: string; user_id?: string; organization_id?: string; start_date?: string; end_date?: string; limit?: number; offset?: number; }): Promise<SpendLogsResponse> { return this.request('GET', '/spend/logs', undefined, params); }
    getSpendByKeys(params?: { organization_id?: string; start_date?: string; end_date?: string; limit?: number; }): Promise<KeySpend[]> { return this.request('GET', '/spend/keys', undefined, params); }
    getSpendByTeams(params?: { organization_id?: string; start_date?: string; end_date?: string; limit?: number; }): Promise<TeamSpend[]> { return this.request('GET', '/spend/teams', undefined, params); }
    getSpendByUsers(params?: { team_id?: string; organization_id?: string; start_date?: string; end_date?: string; limit?: number; }): Promise<UserSpend[]> { return this.request('GET', '/spend/users', undefined, params); }
    getGlobalActivity(params?: { start_date?: string; end_date?: string; }): Promise<GlobalActivityResponse> { return this.request('GET', '/global/activity', undefined, params); }
    getSpendByModels(params?: { start_date?: string; end_date?: string; limit?: number; }): Promise<ModelSpend[]> { return this.request('GET', '/global/spend/models', undefined, params); }
    getSpendByProviders(params?: { start_date?: string; end_date?: string; limit?: number; }): Promise<ProviderSpend[]> { return this.request('GET', '/global/spend/provider', undefined, params); }
    getAuditLogs(params?: { actor_id?: string; action?: string; object_type?: string; object_id?: string; team_id?: string; organization_id?: string; start_time?: string; end_time?: string; limit?: number; offset?: number; }): Promise<AuditLogsResponse> { return this.request('GET', '/audit/logs', undefined, params); }
    getAuditLog(id: string): Promise<AuditLog> { return this.request('GET', '/audit/log', undefined, { id }); }
    getAuditStats(params?: { start_time?: string; end_time?: string; }): Promise<AuditStats> { return this.request('GET', '/audit/stats', undefined, params); }
    deleteAuditLogs(olderThanDays: number): Promise<{ deleted_count: number }> { return this.request('POST', '/audit/delete', { older_than_days: olderThanDays }); }
    createInvitation(data: CreateInvitationRequest): Promise<InvitationLink> { return this.request('POST', '/invitation/new', data); }
    acceptInvitation(token: string): Promise<{ success: boolean; team_id?: string; organization_id?: string }> { return this.request('POST', '/invitation/accept', { token }); }
    getInvitationInfo(invitationId: string): Promise<InvitationLink> { return this.request('GET', '/invitation/info', undefined, { id: invitationId }); }
    listInvitations(params?: { team_id?: string; organization_id?: string; is_active?: boolean; limit?: number; offset?: number; }): Promise<PaginatedResponse<InvitationLink>> { return this.request('GET', '/invitation/list', undefined, params); }
    deactivateInvitation(invitationId: string): Promise<{ success: boolean }> { return this.request('POST', '/invitation/deactivate', { id: invitationId }); }
    deleteInvitation(invitationId: string): Promise<{ success: boolean }> { return this.request('POST', '/invitation/delete', { id: invitationId }); }
    getRoutingMemory(): Promise<{ data: RoutingMemoryItem[] }> { return this.request('GET', '/control/routing-memory'); }
    getSchedulingAdvisor(): Promise<SchedulingAdvisor> { return this.request('GET', '/control/scheduling/advisor'); }
    getRoutingOptimizationComparison(): Promise<RoutingOptimizationComparison> { return this.request('GET', '/control/scheduling/compare'); }
    simulateTraffic(data: { prompt: string; traffic_type?: string }): Promise<TrafficSimulationResponse> { return this.request('POST', '/control/simulate-traffic', data); }
    realRun(data: { prompt: string; traffic_type?: string }): Promise<RealTrafficRunResponse> { return this.request('POST', '/control/real-run', data); }
    listConversationAgents(): Promise<{ data: ConversationAgent[] }> { return this.request('GET', '/control/conversation/agents'); }
    createConversationAgent(data: Partial<ConversationAgent>): Promise<ConversationAgent> { return this.request('POST', '/control/conversation/agents/new', data); }
    updateConversationAgent(data: Partial<ConversationAgent>): Promise<ConversationAgent> { return this.request('POST', '/control/conversation/agents/update', data); }
    deleteConversationAgent(id: string): Promise<{ success: boolean }> { return this.request('POST', '/control/conversation/agents/delete', { id }); }
    listConversationTools(): Promise<{ data: AgentToolOption[] }> { return this.request('GET', '/control/conversation/tools'); }
    listAgentTeams(): Promise<{ data: AgentTeam[] }> { return this.request('GET', '/control/conversation/agent-teams'); }
    createAgentTeam(data: Partial<AgentTeam>): Promise<AgentTeam> { return this.request('POST', '/control/conversation/agent-teams/new', data); }
    updateAgentTeam(data: Partial<AgentTeam>): Promise<AgentTeam> { return this.request('POST', '/control/conversation/agent-teams/update', data); }
    deleteAgentTeam(id: string): Promise<{ success: boolean }> { return this.request('POST', '/control/conversation/agent-teams/delete', { id }); }
    getTeamMemory(): Promise<{ data: TeamMemoryRecord[] }> { return this.request('GET', '/control/conversation/team-memory'); }
    rehearseAgentTeam(data: { session_id?: string; team_id: string; prompt: string }): Promise<AgentTeamRehearsalResponse> { return this.request('POST', '/control/conversation/rehearse-team', data); }
    listToolPresets(): Promise<{ data: ToolPreset[] }> { return this.request('GET', '/control/conversation/tool-presets'); }
    importToolPreset(preset_id: string): Promise<{ preset: ToolPreset; imported: ToolMarketplaceItem[] }> { return this.request('POST', '/control/conversation/tool-presets/import', { preset_id }); }
    listToolMarketplace(): Promise<{ data: ToolMarketplaceItem[] }> { return this.request('GET', '/control/conversation/tool-marketplace'); }
    createToolMarketplaceItem(data: Partial<ToolMarketplaceItem>): Promise<ToolMarketplaceItem> { return this.request('POST', '/control/conversation/tool-marketplace/new', data); }
    updateToolMarketplaceItem(data: Partial<ToolMarketplaceItem>): Promise<ToolMarketplaceItem> { return this.request('POST', '/control/conversation/tool-marketplace/update', data); }
    deleteToolMarketplaceItem(id: string): Promise<{ success: boolean }> { return this.request('POST', '/control/conversation/tool-marketplace/delete', { id }); }
    importToolMarketplaceItem(data: Partial<ToolMarketplaceItem> & { source_client_id: string; source_tool_name: string }): Promise<ToolMarketplaceItem> { return this.request('POST', '/control/conversation/tool-marketplace/import', data); }
    getConversationMemory(): Promise<{ data: RouteMemoryRecord[] }> { return this.request('GET', '/control/conversation/memory'); }
    conversationChat(data: {
        session_id?: string;
        messages: { role: string; content: string }[];
        token_optimization?: boolean;
        cost_optimization?: boolean;
    }): Promise<AgentChatResponse> { return this.request('POST', '/control/conversation/chat', data); }
    runConversationLab(data: {
        scenario?: string;
        prompt: string;
        traffic_type?: string;
        session_id?: string;
        complexity_hint?: number;
        require_team?: boolean;
        required_tools?: string[];
        required_capabilities?: string[];
        baseline_provider?: string;
        baseline_model?: string;
        token_optimization?: boolean;
        cost_optimization?: boolean;
    }): Promise<ConversationLabResponse> { return this.request('POST', '/control/lab/compare', data); }
    generateSandbox(data: { prompt: string }): Promise<SandboxState> { return this.request('POST', '/sandbox/generate', data); }
    getSandboxState(id: string): Promise<SandboxState> { return this.request('GET', '/sandbox/state', undefined, { id }); }
    stepSandbox(id: string): Promise<SandboxState> { return this.request('POST', '/sandbox/step', { id }); }
}

export const apiClient = new LLMuxApiClient();
export { LLMuxApiClient };
