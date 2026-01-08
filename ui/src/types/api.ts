/**
 * LLMux API Type Definitions
 * 
 * 对应后端 Go 类型定义:
 * - internal/auth/types.go
 * - internal/auth/audit.go
 * - internal/auth/invitation.go
 * - internal/auth/budget.go
 */

// ===== API Key Types =====
// 对应 internal/auth/types.go L18-63

export type KeyType = 'llm_api' | 'management' | 'read_only' | 'default';
export type BudgetDuration = '1d' | '7d' | '30d' | '';

export interface APIKey {
    id: string;
    key_prefix: string;           // 前8字符用于识别
    name: string;
    key_alias?: string;
    team_id?: string;
    user_id?: string;
    organization_id?: string;
    allowed_models?: string[];
    key_type?: KeyType;

    // 速率限制
    tpm_limit?: number;           // Tokens/min
    rpm_limit?: number;           // Requests/min
    max_parallel_requests?: number;
    model_tpm_limit?: Record<string, number>;
    model_rpm_limit?: Record<string, number>;

    // 预算管理
    max_budget?: number;          // 硬预算
    soft_budget?: number;         // 软预算（告警阈值）
    spent_budget: number;         // 已使用
    model_max_budget?: Record<string, number>;
    model_spend?: Record<string, number>;
    budget_duration?: BudgetDuration;
    budget_reset_at?: string;

    // 状态
    is_active: boolean;
    blocked: boolean;

    // 时间戳
    created_at: string;
    updated_at: string;
    expires_at?: string;
    last_used_at?: string;

    metadata?: Record<string, unknown>;
}

export interface GenerateKeyRequest {
    name: string;
    team_id?: string;
    user_id?: string;
    organization_id?: string;
    key_alias?: string;
    duration?: string;
    models?: string[];
    max_budget?: number;
    soft_budget?: number;
    budget_duration?: BudgetDuration;
    tpm_limit?: number;
    rpm_limit?: number;
    max_parallel_requests?: number;
    metadata?: Record<string, unknown>;
}

export interface GenerateKeyResponse {
    key: string;                  // 完整密钥（仅创建时返回）
    key_prefix: string;
    key_id: string;
    expires_at?: string;
}

// ===== Team Types =====
// 对应 internal/auth/types.go L65-98

export interface Team {
    team_id: string;
    team_alias?: string;
    organization_id?: string;
    members?: string[];           // 成员 User ID 列表

    // 预算管理
    max_budget?: number;
    spend: number;                // 已使用预算
    model_max_budget?: Record<string, number>;
    model_spend?: Record<string, number>;
    budget_duration?: string;
    budget_reset_at?: string;

    // 速率限制
    tpm_limit?: number;
    rpm_limit?: number;
    max_parallel_requests?: number;
    model_tpm_limit?: Record<string, number>;
    model_rpm_limit?: Record<string, number>;

    // 访问控制
    models?: string[];            // 允许使用的模型

    // 状态
    is_active: boolean;
    blocked: boolean;

    metadata?: Record<string, unknown>;
    created_at: string;
    updated_at: string;
}

export interface CreateTeamRequest {
    team_alias: string;
    organization_id?: string;
    models?: string[];
    max_budget?: number;
    tpm_limit?: number;
    rpm_limit?: number;
    max_parallel_requests?: number;
    budget_duration?: string;
    metadata?: Record<string, unknown>;
}

// ===== User Types =====
// 对应 internal/auth/types.go L100-154

export type UserRole =
    | 'proxy_admin'
    | 'proxy_admin_viewer'
    | 'org_admin'
    | 'internal_user'
    | 'internal_user_viewer'
    | 'team'
    | 'customer';

export interface User {
    user_id: string;
    user_alias?: string;
    user_email?: string;
    team_id?: string;
    teams?: string[];
    organization_id?: string;
    user_role: UserRole;

    // 预算
    max_budget?: number;
    spend: number;
    model_max_budget?: Record<string, number>;
    model_spend?: Record<string, number>;
    budget_duration?: string;
    budget_reset_at?: string;

    // 速率限制
    tpm_limit?: number;
    rpm_limit?: number;
    max_parallel_requests?: number;

    // 访问控制
    models?: string[];

    is_active: boolean;
    metadata?: Record<string, unknown>;
    created_at?: string;
    updated_at?: string;
}

export interface CreateUserRequest {
    user_alias?: string;
    user_email?: string;
    team_id?: string;
    organization_id?: string;
    user_role?: UserRole;
    max_budget?: number;
    models?: string[];
    metadata?: Record<string, unknown>;
}

// ===== Organization Types =====
// 对应 internal/auth/types.go L315-327

export interface Organization {
    organization_id: string;
    organization_alias: string;
    budget_id?: string;
    models?: string[];
    max_budget?: number;
    spend: number;
    model_spend?: Record<string, number>;
    metadata?: Record<string, unknown>;
    created_at: string;
    updated_at: string;
}

export interface OrganizationMembership {
    user_id: string;
    organization_id: string;
    user_role?: string;           // org_admin, member
    spend: number;
    budget_id?: string;
    joined_at?: string;
}

export interface CreateOrganizationRequest {
    organization_alias: string;
    models?: string[];
    max_budget?: number;
    metadata?: Record<string, unknown>;
}

// ===== Spend & Analytics Types =====
// 对应 internal/api/spend_endpoints.go

export interface DailyUsage {
    date: string;
    api_requests: number;
    input_tokens: number;
    output_tokens: number;
    total_tokens: number;
    spend: number;
}

export interface SpendLogsSummary {
    total_requests: number;
    total_cost: number;
    input_tokens: number;
    output_tokens: number;
    avg_latency_ms: number;
    success_rate: number;
}

export interface SpendLogsResponse {
    summary: SpendLogsSummary;
    daily_usage: DailyUsage[];
    filters: Record<string, string>;
}

export interface KeySpend {
    key_id: string;
    key_prefix: string;
    key_name: string;
    spend: number;
    max_budget?: number;
}

export interface TeamSpend {
    team_id: string;
    team_alias?: string;
    spend: number;
    max_budget?: number;
}

export interface UserSpend {
    user_id: string;
    user_alias?: string;
    user_email?: string;
    spend: number;
    max_budget?: number;
}

// ===== Global Analytics Types =====
// 对应 internal/api/spend_endpoints.go L204-408

export interface GlobalActivityData {
    date: string;
    api_requests: number;
    total_tokens: number;
    spend: number;
}

export interface GlobalActivityResponse {
    daily_data: GlobalActivityData[];
    sum_api_requests: number;
    sum_total_tokens: number;
    total_cost: number;
    avg_latency_ms: number;
    success_rate: number;
    unique_models: number;
    unique_providers: number;
}

export interface ModelSpend {
    model: string;
    spend: number;
    api_requests: number;
    total_tokens: number;
}

export interface ProviderSpend {
    provider: string;
    spend: number;
    api_requests: number;
    total_tokens: number;
}

// ===== Audit Log Types =====
// 对应 internal/auth/audit.go L79-144

export type AuditActorType = 'user' | 'api_key' | 'system';

export type AuditAction =
    | 'create' | 'update' | 'delete' | 'read'
    | 'login' | 'logout' | 'login_failed' | 'token_refresh'
    | 'api_key_create' | 'api_key_revoke' | 'api_key_block' | 'api_key_unblock'
    | 'team_create' | 'team_update' | 'team_delete' | 'team_member_add' | 'team_member_remove' | 'team_block'
    | 'org_create' | 'org_update' | 'org_delete' | 'org_member_add' | 'org_member_remove'
    | 'user_create' | 'user_update' | 'user_delete' | 'user_role_change'
    | 'budget_exceeded' | 'budget_reset' | 'budget_update'
    | 'config_update' | 'sso_update';

export type AuditObjectType =
    | 'api_key' | 'team' | 'organization' | 'user'
    | 'end_user' | 'budget' | 'config' | 'sso' | 'model' | 'membership';

export interface AuditLog {
    id: string;
    timestamp: string;

    // 操作者
    actor_id: string;
    actor_type: AuditActorType;
    actor_email?: string;
    actor_ip?: string;

    // 操作类型
    action: AuditAction;

    // 目标对象
    object_type: AuditObjectType;
    object_id: string;

    // 上下文
    team_id?: string;
    organization_id?: string;

    // 变更记录
    before_value?: Record<string, unknown>;
    after_value?: Record<string, unknown>;
    diff?: Record<string, unknown>;

    // 请求信息
    request_id?: string;
    user_agent?: string;
    request_uri?: string;

    // 结果
    success: boolean;
    error?: string;

    metadata?: Record<string, unknown>;
}

export interface AuditLogsResponse {
    logs: AuditLog[];
    total: number;
}

export interface AuditStats {
    total_logs: number;
    success_count: number;
    failure_count: number;
    action_distribution: Record<AuditAction, number>;
    object_type_distribution: Record<AuditObjectType, number>;
}

// ===== Invitation Link Types =====
// 对应 internal/auth/invitation.go L13-44

export interface InvitationLink {
    id: string;
    token: string;                // 用于 URL 的唯一 token（仅创建时返回）

    // 目标
    team_id?: string;
    organization_id?: string;

    // 权限
    role?: string;

    // 限制
    max_uses?: number;            // 0 = 无限
    current_uses: number;
    max_budget?: number;

    // 有效性
    expires_at?: string;
    is_active: boolean;

    // 创建者
    created_by: string;
    created_at: string;
    updated_at: string;

    description?: string;
    metadata?: Record<string, unknown>;
}

export interface CreateInvitationRequest {
    team_id?: string;
    organization_id?: string;
    role?: string;
    max_uses?: number;
    max_budget?: number;
    duration?: string;            // e.g., "7d", "30d"
    description?: string;
    metadata?: Record<string, unknown>;
}

// ===== Common API Response Types =====

export interface ApiError {
    error: {
        message: string;
        type: string;
        code?: string;
    };
}

export interface PaginatedResponse<T> {
    data: T[];
    total: number;
    limit: number;
    offset: number;
}

export interface ListParams {
    limit?: number;
    offset?: number;
}
