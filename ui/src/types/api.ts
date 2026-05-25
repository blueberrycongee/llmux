/**
 * LLMux API Type Definitions
 * 
 * 对应后端 Go 类型定义:
 * - internal/auth/types.go
 * - internal/auth/audit.go
 * - internal/auth/invitation.go
 * - internal/auth/budget.go
 */

export type KeyType = 'llm_api' | 'management' | 'read_only' | 'default';
export type BudgetDuration = '1d' | '7d' | '30d' | '';

export interface APIKey {
    id: string;
    key_prefix: string;
    name: string;
    key_alias?: string;
    team_id?: string;
    user_id?: string;
    organization_id?: string;
    allowed_models?: string[];
    key_type?: KeyType;
    tpm_limit?: number;
    rpm_limit?: number;
    max_parallel_requests?: number;
    model_tpm_limit?: Record<string, number>;
    model_rpm_limit?: Record<string, number>;
    max_budget?: number;
    soft_budget?: number;
    spent_budget: number;
    model_max_budget?: Record<string, number>;
    model_spend?: Record<string, number>;
    budget_duration?: BudgetDuration;
    budget_reset_at?: string;
    is_active: boolean;
    blocked: boolean;
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
    key: string;
    key_prefix: string;
    key_id: string;
    expires_at?: string;
}

export interface Team {
    team_id: string;
    team_alias?: string;
    organization_id?: string;
    members?: string[];
    max_budget?: number;
    spend: number;
    model_max_budget?: Record<string, number>;
    model_spend?: Record<string, number>;
    budget_duration?: string;
    budget_reset_at?: string;
    tpm_limit?: number;
    rpm_limit?: number;
    max_parallel_requests?: number;
    model_tpm_limit?: Record<string, number>;
    model_rpm_limit?: Record<string, number>;
    models?: string[];
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

export type UserRole = 'proxy_admin' | 'proxy_admin_viewer' | 'org_admin' | 'internal_user' | 'internal_user_viewer' | 'team' | 'customer';

export interface User {
    user_id: string;
    user_alias?: string;
    user_email?: string;
    team_id?: string;
    teams?: string[];
    organization_id?: string;
    user_role: UserRole;
    max_budget?: number;
    spend: number;
    model_max_budget?: Record<string, number>;
    model_spend?: Record<string, number>;
    budget_duration?: string;
    budget_reset_at?: string;
    tpm_limit?: number;
    rpm_limit?: number;
    max_parallel_requests?: number;
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
    user_role?: string;
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

export interface KeySpend { key_id: string; key_prefix: string; key_name: string; spend: number; max_budget?: number; }
export interface TeamSpend { team_id: string; team_alias?: string; spend: number; max_budget?: number; }
export interface UserSpend { user_id: string; user_alias?: string; user_email?: string; spend: number; max_budget?: number; }

export interface GlobalActivityData { date: string; api_requests: number; total_tokens: number; spend: number; }
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

export interface ModelSpend { model: string; spend: number; api_requests: number; total_tokens: number; }
export interface ProviderSpend { provider: string; spend: number; api_requests: number; total_tokens: number; }

export type AuditActorType = 'user' | 'api_key' | 'system';
export type AuditAction = 'create' | 'update' | 'delete' | 'read' | 'login' | 'logout' | 'login_failed' | 'token_refresh' | 'api_key_create' | 'api_key_revoke' | 'api_key_block' | 'api_key_unblock' | 'team_create' | 'team_update' | 'team_delete' | 'team_member_add' | 'team_member_remove' | 'team_block' | 'org_create' | 'org_update' | 'org_delete' | 'org_member_add' | 'org_member_remove' | 'user_create' | 'user_update' | 'user_delete' | 'user_role_change' | 'budget_exceeded' | 'budget_reset' | 'budget_update' | 'config_update' | 'sso_update';
export type AuditObjectType = 'api_key' | 'team' | 'organization' | 'user' | 'end_user' | 'budget' | 'config' | 'sso' | 'model' | 'membership';

export interface AuditLog {
    id: string;
    timestamp: string;
    actor_id: string;
    actor_type: AuditActorType;
    actor_email?: string;
    actor_ip?: string;
    action: AuditAction;
    object_type: AuditObjectType;
    object_id: string;
    team_id?: string;
    organization_id?: string;
    before_value?: Record<string, unknown>;
    after_value?: Record<string, unknown>;
    diff?: Record<string, unknown>;
    request_id?: string;
    user_agent?: string;
    request_uri?: string;
    success: boolean;
    error?: string;
    metadata?: Record<string, unknown>;
}

export interface AuditLogsResponse { logs: AuditLog[]; total: number; }
export interface AuditStats {
    total_logs: number;
    success_count: number;
    failure_count: number;
    action_distribution: Record<AuditAction, number>;
    object_type_distribution: Record<AuditObjectType, number>;
}

export interface InvitationLink {
    id: string;
    token: string;
    team_id?: string;
    organization_id?: string;
    role?: string;
    max_uses?: number;
    current_uses: number;
    max_budget?: number;
    expires_at?: string;
    is_active: boolean;
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
    duration?: string;
    description?: string;
    metadata?: Record<string, unknown>;
}

export interface ApiError { error: { message: string; type: string; code?: string; }; }
export interface PaginatedResponse<T> { data: T[]; total: number; limit: number; offset: number; }
export interface ListParams { limit?: number; offset?: number; }

export interface SandboxDistrict { name: string; x: number; y: number; w: number; h: number; color: string; }
export interface SandboxAgent { id: string; name: string; role: string; from: string; to: string; action: string; status: string; memory: string; progress: number; speed: number; }
export interface SandboxLog { tick: number; text: string; }
export interface SandboxState {
    id: string;
    name: string;
    premise: string;
    theme: 'harbor' | 'industry';
    protocol: string;
    institutions: string[];
    rules: string[];
    templates: string[];
    archive: string[];
    districts: SandboxDistrict[];
    agents: SandboxAgent[];
    tick: number;
    logs: SandboxLog[];
}

export interface GenerateSandboxRequest { prompt: string; }
export interface RoutingMemoryItem {
    id: string;
    timestamp: string;
    category: string;
    signal: string;
    observation: string;
    decision: string;
    outcome: string;
    confidence: number;
    tags: string[];
}

export interface SchedulingAdvisor {
    mode: string;
    recommended_strategy: string;
    recommended_provider: string;
    recommended_model?: string;
    confidence: number;
    summary: string;
    reasons: string[];
    risks: string[];
    next_actions: string[];
    inputs: Record<string, unknown>;
    memory: RoutingMemoryItem[];
    generated_at: string;
}

export interface RoutingOptimizationSnapshot {
    strategy: string;
    provider: string;
    avg_latency_ms: number;
    success_rate: number;
    cost_per_1k_requests: number;
}

export interface RoutingOptimizationComparison {
    baseline: RoutingOptimizationSnapshot;
    optimized: RoutingOptimizationSnapshot;
    latency_delta_ms: number;
    success_rate_delta: number;
    cost_delta_per_1k_requests: number;
    improvement_summary: string;
    why_it_improved: string[];
    recommended_rollout: string[];
    generated_at: string;
}

export interface TrafficSimulationLayer {
    layer: string;
    title: string;
    status: string;
    summary: string;
    touches_llm: boolean;
    highlighted: boolean;
}

export interface TrafficSimulationResponse {
    request_id: string;
    prompt: string;
    traffic_type: string;
    recommended_strategy: string;
    recommended_provider: string;
    recommended_model: string;
    execution_mode: string;
    memory_influence: string;
    layers: TrafficSimulationLayer[];
    final_summary: string;
    generated_at: string;
}

export interface RealTrafficRunResponse {
    request_id: string;
    prompt: string;
    traffic_type: string;
    recommended_strategy: string;
    recommended_provider: string;
    recommended_model: string;
    execution_mode: string;
    memory_influence: string;
    scheduler_input: string;
    scheduler_decision: string;
    gateway_request: Record<string, unknown>;
    gateway_response?: Record<string, unknown>;
    llm_output_text?: string;
    provider_reported?: string;
    error_message?: string;
    succeeded: boolean;
    generated_at: string;
}

export interface CandidateModel {
    provider: string;
    model: string;
    weight?: number;
    rpm_limit?: number;
    tpm_limit?: number;
}

export interface CandidateModelState extends CandidateModel {
    current_rpm: number;
    current_tpm: number;
    selection_score?: number;
    status: string;
    selected_count: number;
    failover_count: number;
    last_error?: string;
    cooldown_until?: string;
}

export interface CandidateFailover {
    provider: string;
    model: string;
    outcome: string;
    reason?: string;
}

export interface ConversationAgent {
    id: string;
    name: string;
    description: string;
    category: string;
    provider: string;
    model: string;
    candidate_models?: CandidateModel[];
    candidate_states?: CandidateModelState[];
    strategy: string;
    capabilities: string[];
    system_prompt: string;
    accent: string;
    tools?: string[];
    enabled: boolean;
    created_at: string;
    updated_at: string;
}

export interface AgentTeam {
    id: string;
    name: string;
    description: string;
    mode: string;
    agent_ids: string[];
    enabled: boolean;
    created_at: string;
    updated_at: string;
}

export interface TeamMemoryRecord {
    id: string;
    session_id: string;
    query: string;
    intent: string;
    selected_team_id: string;
    selected_agent_ids: string[];
    summary: string;
    compressed_summary?: string;
    distilled_learnings?: string[];
    representative_queries?: string[];
    retrieval_hints?: string[];
    memory_stage?: string;
    source_count?: number;
    success_count?: number;
    consult_count?: number;
    reuse_count?: number;
    compression_ratio?: number;
    first_recorded_at?: string;
    last_reinforced_at?: string;
    succeeded: boolean;
    outcome_score: number;
    created_at: string;
}

export interface RouteMemoryRecord {
    id: string;
    session_id: string;
    query: string;
    intent: string;
    selected_agent_id: string;
    selected_model: string;
    selected_path: string;
    route_source: string;
    answer_preview: string;
    compressed_summary?: string;
    distilled_learnings?: string[];
    representative_queries?: string[];
    retrieval_hints?: string[];
    memory_stage?: string;
    source_count?: number;
    success_count?: number;
    consult_count?: number;
    reuse_count?: number;
    compression_ratio?: number;
    first_recorded_at?: string;
    last_reinforced_at?: string;
    succeeded: boolean;
    outcome_score: number;
    created_at: string;
}

export interface AgentChatResponse {
    request_id: string;
    session_id: string;
    intent: string;
    selected_team?: AgentTeam;
    team_participants?: ConversationAgent[];
    selected_agent: ConversationAgent;
    selected_candidate?: CandidateModel;
    candidate_failovers?: CandidateFailover[];
    route_source: string;
    routing_reasoning: string[];
    memory_influence?: 'none' | 'consulted' | 'reused' | string;
    consulted_memories?: RouteMemoryRecord[];
    memory_hit?: RouteMemoryRecord;
    team_consulted_memories?: TeamMemoryRecord[];
    team_memory_hit?: TeamMemoryRecord;
    gateway_request: Record<string, unknown>;
    gateway_response?: Record<string, unknown>;
    assistant_message?: string;
    recorded_memory: RouteMemoryRecord;
    succeeded: boolean;
    error_message?: string;
    token_optimization_enabled?: boolean;
    optimization_notes?: string[];
    generated_at: string;
}

export interface ConversationLabRun {
    mode: string;
    succeeded: boolean;
    error_message?: string;
    provider?: string;
    model?: string;
    latency_ms: number;
    prompt_tokens?: number;
    completion_tokens?: number;
    total_tokens?: number;
    estimated_cost?: number;
    response_chars?: number;
    response_text?: string;
    finish_reason?: string;
    memory_influence?: string;
    route_source?: string;
    selected_team_id?: string;
    selected_agent_id?: string;
    selected_candidate?: string;
    used_team?: boolean;
    used_memory?: boolean;
    required_tools?: string[];
    bound_tools?: string[];
    routing_reasoning?: string[];
    candidate_failovers?: CandidateFailover[];
    quality: {
        label: string;
        tool_used: boolean;
        tool_calls: number;
        tool_names?: string[];
        groundedness_score: number;
        hallucination_risk: number;
        route_depth: number;
        memory_hit: boolean;
        team_route: boolean;
        quality_notes?: string[];
    };
    token_optimization_enabled?: boolean;
    optimization_notes?: string[];
}

export interface ConversationLabResponse {
    request_id: string;
    scenario: string;
    prompt: string;
    traffic_type: string;
    baseline: ConversationLabRun;
    optimized: ConversationLabRun;
    latency_delta_ms: number;
    total_token_delta: number;
    cost_delta: number;
    response_chars_delta: number;
    winning_dimensions: string[];
    recommended_winner: string;
    value_summary: string[];
    generated_at: string;
}

export interface TeamRehearsalStep {
    agent_id: string;
    agent_name: string;
    selected_candidate?: CandidateModel;
    output_preview: string;
    succeeded: boolean;
    error_message?: string;
}

export interface AgentTeamRehearsalResponse {
    request_id: string;
    session_id: string;
    selected_team: AgentTeam;
    participating_agents: ConversationAgent[];
    steps: TeamRehearsalStep[];
    recorded_memory: TeamMemoryRecord;
    succeeded: boolean;
    generated_at: string;
}

export interface AgentToolOption {
    id: string;
    client_id: string;
    tool_name: string;
    label: string;
}


export interface ToolPresetClientTemplate {
    id: string;
    name: string;
    type: string;
    command?: string;
    args?: string[];
    envs?: string[];
    tools_to_execute?: string[];
    headers?: Record<string, string>;
}

export interface ToolPreset {
    id: string;
    name: string;
    description: string;
    category: string;
    tags: string[];
    client: ToolPresetClientTemplate;
    tools: ToolMarketplaceItem[];
}
export interface ToolMarketplaceItem {
    id: string;
    name: string;
    description: string;
    category: string;
    source_client_id: string;
    source_tool_name: string;
    tags: string[];
    enabled: boolean;
    created_at: string;
    updated_at: string;
}
