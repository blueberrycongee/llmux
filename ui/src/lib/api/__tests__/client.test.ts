/**
 * API Client Tests
 * 
 * 测试 LLMuxApiClient 的核心功能
 */

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { LLMuxApiClient, LLMuxApiError } from '../client';

// Mock fetch globally
const mockFetch = vi.fn();
global.fetch = mockFetch;

describe('LLMuxApiClient', () => {
    let client: LLMuxApiClient;

    beforeEach(() => {
        client = new LLMuxApiClient('http://localhost:8080');
        mockFetch.mockReset();
    });

    afterEach(() => {
        vi.clearAllMocks();
    });

    describe('初始化', () => {
        it('应该使用默认 URL 创建客户端', () => {
            const defaultClient = new LLMuxApiClient();
            // 在浏览器环境（vitest/jsdom）下，默认使用当前 origin（代理模式）
            expect(defaultClient.getBaseUrl()).toBe(window.location.origin);
        });

        it('应该使用自定义 URL 创建客户端', () => {
            const customClient = new LLMuxApiClient('https://api.example.com');
            expect(customClient.getBaseUrl()).toBe('https://api.example.com');
        });
    });

    describe('Token 管理', () => {
        it('应该能设置和清除 token', async () => {
            mockFetch.mockResolvedValueOnce({
                ok: true,
                headers: new Headers({ 'content-type': 'application/json' }),
                json: async () => ({ data: [], total: 0 }),
            });

            // 设置 token 并发起请求
            client.setToken('test-token');
            await client.listKeys();

            // 验证请求头包含 Authorization
            expect(mockFetch).toHaveBeenCalledWith(
                expect.any(String),
                expect.objectContaining({
                    headers: expect.objectContaining({
                        'Authorization': 'Bearer test-token',
                    }),
                })
            );

            // 清除 token
            client.clearToken();
            mockFetch.mockResolvedValueOnce({
                ok: true,
                headers: new Headers({ 'content-type': 'application/json' }),
                json: async () => ({ data: [], total: 0 }),
            });

            await client.listKeys();

            // 验证请求头不包含 Authorization
            const lastCall = mockFetch.mock.calls[1];
            expect(lastCall[1].headers['Authorization']).toBeUndefined();
        });
    });

    describe('Key Management API', () => {
        it('listKeys - 应该正确调用 GET /key/list', async () => {
            const mockResponse = {
                data: [
                    { id: 'key-1', name: 'Test Key', key_prefix: 'sk-test', is_active: true, blocked: false, spent_budget: 0 },
                ],
                total: 1,
            };

            mockFetch.mockResolvedValueOnce({
                ok: true,
                headers: new Headers({ 'content-type': 'application/json' }),
                json: async () => mockResponse,
            });

            const result = await client.listKeys({ team_id: 'team-1', limit: 10 });

            expect(mockFetch).toHaveBeenCalledWith(
                'http://localhost:8080/key/list?team_id=team-1&limit=10',
                expect.objectContaining({ method: 'GET' })
            );
            expect(result).toEqual(mockResponse);
        });

        it('generateKey - 应该正确调用 POST /key/generate', async () => {
            const mockResponse = {
                key: 'sk-test-full-key',
                key_prefix: 'sk-test',
                key_id: 'key-123',
            };

            mockFetch.mockResolvedValueOnce({
                ok: true,
                headers: new Headers({ 'content-type': 'application/json' }),
                json: async () => mockResponse,
            });

            const result = await client.generateKey({
                name: 'New Test Key',
                team_id: 'team-1',
                max_budget: 100,
            });

            expect(mockFetch).toHaveBeenCalledWith(
                'http://localhost:8080/key/generate',
                expect.objectContaining({
                    method: 'POST',
                    body: JSON.stringify({
                        name: 'New Test Key',
                        team_id: 'team-1',
                        max_budget: 100,
                    }),
                })
            );
            expect(result).toEqual(mockResponse);
        });

        it('blockKey - 应该正确调用 POST /key/block', async () => {
            mockFetch.mockResolvedValueOnce({
                ok: true,
                headers: new Headers({ 'content-type': 'application/json' }),
                json: async () => ({ success: true }),
            });

            const result = await client.blockKey('key-123');

            expect(mockFetch).toHaveBeenCalledWith(
                'http://localhost:8080/key/block',
                expect.objectContaining({
                    method: 'POST',
                    body: JSON.stringify({ key: 'key-123' }),
                })
            );
            expect(result).toEqual({ success: true });
        });

        it('deleteKeys - 应该正确批量删除', async () => {
            mockFetch.mockResolvedValueOnce({
                ok: true,
                headers: new Headers({ 'content-type': 'application/json' }),
                json: async () => ({ deleted_count: 3 }),
            });

            const result = await client.deleteKeys(['key-1', 'key-2', 'key-3']);

            expect(mockFetch).toHaveBeenCalledWith(
                'http://localhost:8080/key/delete',
                expect.objectContaining({
                    method: 'POST',
                    body: JSON.stringify({ keys: ['key-1', 'key-2', 'key-3'] }),
                })
            );
            expect(result.deleted_count).toBe(3);
        });
    });

    describe('Team Management API', () => {
        it('listTeams - 应该正确调用 GET /team/list', async () => {
            const mockResponse = {
                data: [
                    { team_id: 'team-1', team_alias: 'Engineering', is_active: true, blocked: false, spend: 0 },
                ],
                total: 1,
            };

            mockFetch.mockResolvedValueOnce({
                ok: true,
                headers: new Headers({ 'content-type': 'application/json' }),
                json: async () => mockResponse,
            });

            const result = await client.listTeams({ organization_id: 'org-1' });

            expect(mockFetch).toHaveBeenCalledWith(
                'http://localhost:8080/team/list?organization_id=org-1',
                expect.objectContaining({ method: 'GET' })
            );
            expect(result).toEqual(mockResponse);
        });

        it('createTeam - 应该正确调用 POST /team/new', async () => {
            const mockTeam = {
                team_id: 'team-new',
                team_alias: 'New Team',
                is_active: true,
                blocked: false,
                spend: 0,
                created_at: '2026-01-09T00:00:00Z',
                updated_at: '2026-01-09T00:00:00Z',
            };

            mockFetch.mockResolvedValueOnce({
                ok: true,
                headers: new Headers({ 'content-type': 'application/json' }),
                json: async () => mockTeam,
            });

            const result = await client.createTeam({
                team_alias: 'New Team',
                max_budget: 1000,
            });

            expect(mockFetch).toHaveBeenCalledWith(
                'http://localhost:8080/team/new',
                expect.objectContaining({
                    method: 'POST',
                    body: JSON.stringify({
                        team_alias: 'New Team',
                        max_budget: 1000,
                    }),
                })
            );
            expect(result.team_alias).toBe('New Team');
        });

        it('addTeamMember - 应该正确添加成员', async () => {
            mockFetch.mockResolvedValueOnce({
                ok: true,
                headers: new Headers({ 'content-type': 'application/json' }),
                json: async () => ({ success: true }),
            });

            await client.addTeamMember('team-1', 'user-1', 'member');

            expect(mockFetch).toHaveBeenCalledWith(
                'http://localhost:8080/team/member_add',
                expect.objectContaining({
                    method: 'POST',
                    body: JSON.stringify({
                        team_id: 'team-1',
                        user_id: 'user-1',
                        role: 'member',
                    }),
                })
            );
        });
    });

    describe('Global Analytics API', () => {
        it('getGlobalActivity - 应该正确调用 GET /global/activity', async () => {
            const mockResponse = {
                daily_data: [
                    { date: '2026-01-08', api_requests: 100, total_tokens: 50000, spend: 5.0 },
                ],
                sum_api_requests: 100,
                sum_total_tokens: 50000,
                total_cost: 5.0,
                avg_latency_ms: 234,
                success_rate: 98.5,
                unique_models: 3,
                unique_providers: 2,
            };

            mockFetch.mockResolvedValueOnce({
                ok: true,
                headers: new Headers({ 'content-type': 'application/json' }),
                json: async () => mockResponse,
            });

            const result = await client.getGlobalActivity({
                start_date: '2026-01-01',
                end_date: '2026-01-08',
            });

            expect(mockFetch).toHaveBeenCalledWith(
                'http://localhost:8080/global/activity?start_date=2026-01-01&end_date=2026-01-08',
                expect.objectContaining({ method: 'GET' })
            );
            expect(result.sum_api_requests).toBe(100);
            expect(result.success_rate).toBe(98.5);
        });

        it('getSpendByModels - 应该正确调用 GET /global/spend/models', async () => {
            const mockResponse = [
                { model: 'gpt-4', spend: 100, api_requests: 500, total_tokens: 200000 },
                { model: 'claude-3', spend: 50, api_requests: 300, total_tokens: 100000 },
            ];

            mockFetch.mockResolvedValueOnce({
                ok: true,
                headers: new Headers({ 'content-type': 'application/json' }),
                json: async () => mockResponse,
            });

            const result = await client.getSpendByModels({ limit: 10 });

            expect(result).toHaveLength(2);
            expect(result[0].model).toBe('gpt-4');
        });
    });

    describe('错误处理', () => {
        it('应该正确处理 API 错误响应', async () => {
            mockFetch.mockResolvedValueOnce({
                ok: false,
                status: 400,
                json: async () => ({
                    error: {
                        message: 'Invalid request',
                        type: 'validation_error',
                    },
                }),
            });

            try {
                await client.listKeys();
                expect.fail('Should have thrown');
            } catch (error) {
                expect(error).toBeInstanceOf(LLMuxApiError);
                expect((error as LLMuxApiError).message).toBe('Invalid request');
            }
        });

        it('应该处理网络错误', async () => {
            mockFetch.mockRejectedValueOnce(new Error('Network error'));

            await expect(client.listKeys()).rejects.toThrow('Network error');
        });

        it('应该处理非 JSON 错误响应', async () => {
            mockFetch.mockResolvedValueOnce({
                ok: false,
                status: 500,
                statusText: 'Internal Server Error',
                json: async () => { throw new Error('Invalid JSON'); },
            });

            try {
                await client.listKeys();
                expect.fail('Should have thrown');
            } catch (error) {
                expect(error).toBeInstanceOf(LLMuxApiError);
                expect((error as LLMuxApiError).status).toBe(500);
            }
        });

        it('LLMuxApiError 应该包含正确的属性', async () => {
            mockFetch.mockResolvedValueOnce({
                ok: false,
                status: 403,
                json: async () => ({
                    error: {
                        message: 'Forbidden',
                        type: 'authorization_error',
                        code: 'INSUFFICIENT_PERMISSIONS',
                    },
                }),
            });

            try {
                await client.listKeys();
                expect.fail('Should have thrown');
            } catch (error) {
                expect(error).toBeInstanceOf(LLMuxApiError);
                const apiError = error as LLMuxApiError;
                expect(apiError.message).toBe('Forbidden');
                expect(apiError.type).toBe('authorization_error');
                expect(apiError.status).toBe(403);
                expect(apiError.code).toBe('INSUFFICIENT_PERMISSIONS');
            }
        });
    });

    describe('Audit Logs API', () => {
        it('getAuditLogs - 应该正确调用 GET /audit/logs', async () => {
            const mockResponse = {
                logs: [
                    {
                        id: 'audit-1',
                        timestamp: '2026-01-09T00:00:00Z',
                        actor_id: 'user-1',
                        actor_type: 'user',
                        action: 'api_key_create',
                        object_type: 'api_key',
                        object_id: 'key-1',
                        success: true,
                    },
                ],
                total: 1,
            };

            mockFetch.mockResolvedValueOnce({
                ok: true,
                headers: new Headers({ 'content-type': 'application/json' }),
                json: async () => mockResponse,
            });

            const result = await client.getAuditLogs({ action: 'api_key_create', limit: 50 });

            expect(mockFetch).toHaveBeenCalledWith(
                'http://localhost:8080/audit/logs?action=api_key_create&limit=50',
                expect.objectContaining({ method: 'GET' })
            );
            expect(result.logs).toHaveLength(1);
        });
    });

    describe('Invitation API', () => {
        it('createInvitation - 应该正确调用 POST /invitation/new', async () => {
            const mockInvitation = {
                id: 'inv-1',
                token: 'abc123token',
                team_id: 'team-1',
                is_active: true,
                current_uses: 0,
                created_by: 'user-1',
                created_at: '2026-01-09T00:00:00Z',
                updated_at: '2026-01-09T00:00:00Z',
            };

            mockFetch.mockResolvedValueOnce({
                ok: true,
                headers: new Headers({ 'content-type': 'application/json' }),
                json: async () => mockInvitation,
            });

            const result = await client.createInvitation({
                team_id: 'team-1',
                max_uses: 10,
                duration: '7d',
            });

            expect(result.token).toBe('abc123token');
            expect(result.team_id).toBe('team-1');
        });

        it('acceptInvitation - 应该正确接受邀请', async () => {
            mockFetch.mockResolvedValueOnce({
                ok: true,
                headers: new Headers({ 'content-type': 'application/json' }),
                json: async () => ({ success: true, team_id: 'team-1' }),
            });

            const result = await client.acceptInvitation('abc123token');

            expect(mockFetch).toHaveBeenCalledWith(
                'http://localhost:8080/invitation/accept',
                expect.objectContaining({
                    method: 'POST',
                    body: JSON.stringify({ token: 'abc123token' }),
                })
            );
            expect(result.success).toBe(true);
        });
    });
});
