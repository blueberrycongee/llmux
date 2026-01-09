/**
 * Dashboard E2E Tests
 * 
 * 测试 Dashboard 页面的完整功能
 * 使用 Playwright 进行端到端测试
 */

import { test, expect } from '@playwright/test';

test.describe('Dashboard Page', () => {
    test.beforeEach(async ({ page }) => {
        // 拦截 API 请求并返回 mock 数据
        await page.route('**/global/activity**', async (route) => {
            await route.fulfill({
                status: 200,
                contentType: 'application/json',
                body: JSON.stringify({
                    daily_data: [
                        { date: '2026-01-01', api_requests: 1000, total_tokens: 50000, spend: 5.0 },
                        { date: '2026-01-02', api_requests: 1200, total_tokens: 60000, spend: 6.0 },
                        { date: '2026-01-03', api_requests: 1500, total_tokens: 75000, spend: 7.5 },
                    ],
                    sum_api_requests: 3700,
                    sum_total_tokens: 185000,
                    total_cost: 18.5,
                    avg_latency_ms: 234,
                    success_rate: 98.5,
                    unique_models: 3,
                    unique_providers: 2,
                }),
            });
        });

        await page.route('**/global/spend/models**', async (route) => {
            await route.fulfill({
                status: 200,
                contentType: 'application/json',
                body: JSON.stringify([
                    { model: 'gpt-4', spend: 100, api_requests: 500, total_tokens: 200000 },
                    { model: 'claude-3', spend: 50, api_requests: 300, total_tokens: 100000 },
                    { model: 'gemini-pro', spend: 30, api_requests: 200, total_tokens: 80000 },
                ]),
            });
        });

        await page.route('**/global/spend/provider**', async (route) => {
            await route.fulfill({
                status: 200,
                contentType: 'application/json',
                body: JSON.stringify([
                    { provider: 'openai', spend: 100, api_requests: 500, total_tokens: 200000 },
                    { provider: 'anthropic', spend: 50, api_requests: 300, total_tokens: 100000 },
                ]),
            });
        });

        // 导航到 Dashboard 页面
        await page.goto('/');
    });

    test('should display the dashboard header', async ({ page }) => {
        // 检查页面标题
        await expect(page.locator('h1')).toContainText('Overview');
        await expect(page.locator('text=Welcome back')).toBeVisible();
    });

    test('should display stat cards with API data', async ({ page }) => {
        // 等待数据加载完成
        // 检查统计卡片是否显示
        await expect(page.getByTestId('stat-card-requests')).toBeVisible();
        await expect(page.getByTestId('stat-card-tokens')).toBeVisible();
        await expect(page.getByTestId('stat-card-cost')).toBeVisible();
        await expect(page.getByTestId('stat-card-latency')).toBeVisible();

        // 验证数据值（来自 mock API）
        await expect(page.getByTestId('stat-value-requests')).toContainText('3,700');
        await expect(page.getByTestId('stat-value-cost')).toContainText('$18.50');
        await expect(page.getByTestId('stat-value-latency')).toContainText('234ms');
    });

    test('should display request volume chart', async ({ page }) => {
        // 检查图表区域是否存在
        await expect(page.getByTestId('chart-request-volume')).toBeVisible();
        await expect(page.locator('text=Request Volume')).toBeVisible();
        await expect(page.locator('text=Daily requests and token usage')).toBeVisible();
    });

    test('should display model distribution chart', async ({ page }) => {
        // 检查模型分布图表
        await expect(page.getByTestId('chart-model-distribution')).toBeVisible();
        await expect(page.locator('text=Model Distribution')).toBeVisible();

        // 验证模型列表显示（来自 mock API）
        await expect(page.locator('text=gpt-4')).toBeVisible();
        await expect(page.locator('text=claude-3')).toBeVisible();
    });

    test('should show loading state initially', async ({ page }) => {
        // 阻止 API 响应以观察loading状态
        await page.route('**/global/activity**', async (route) => {
            // 延迟响应
            await new Promise(resolve => setTimeout(resolve, 2000));
            await route.fulfill({
                status: 200,
                contentType: 'application/json',
                body: JSON.stringify({
                    daily_data: [],
                    sum_api_requests: 0,
                    sum_total_tokens: 0,
                    total_cost: 0,
                    avg_latency_ms: 0,
                    success_rate: 0,
                    unique_models: 0,
                    unique_providers: 0,
                }),
            });
        });

        await page.goto('/');

        // 应该显示加载骨架屏
        await expect(page.getByTestId('skeleton-stats')).toBeVisible();
    });

    test('should handle API error gracefully', async ({ page }) => {
        // 模拟 API 错误
        await page.route('**/global/activity**', async (route) => {
            await route.fulfill({
                status: 500,
                contentType: 'application/json',
                body: JSON.stringify({
                    error: {
                        message: 'Internal server error',
                        type: 'server_error',
                    },
                }),
            });
        });

        await page.goto('/');

        // 应该显示错误状态
        await expect(page.getByTestId('error-message')).toBeVisible();
        await expect(page.locator('text=Failed to load')).toBeVisible();
    });

    test('should have responsive layout on mobile', async ({ page }) => {
        // 设置移动端视口
        await page.setViewportSize({ width: 375, height: 667 });
        await page.goto('/');

        // 检查移动端布局
        await expect(page.locator('h1')).toBeVisible();

        // 统计卡片应该堆叠显示
        const statCards = page.getByTestId('stat-card-requests');
        await expect(statCards).toBeVisible();
    });

    test('should display success rate correctly', async ({ page }) => {
        // 验证成功率显示
        await expect(page.getByTestId('stat-value-success-rate')).toContainText('98.5%');
    });

    test('should format large numbers with commas', async ({ page }) => {
        // 验证数字格式化
        await expect(page.getByTestId('stat-value-tokens')).toContainText('185,000');
    });
});

test.describe('Dashboard Navigation', () => {
    test('should navigate to API Keys page', async ({ page }) => {
        await page.goto('/');
        await page.click('text=API Keys');
        await expect(page).toHaveURL(/.*api-keys/);
    });

    test('should navigate to Teams page', async ({ page }) => {
        await page.goto('/');
        await page.click('text=Teams');
        await expect(page).toHaveURL(/.*teams/);
    });

    test('should navigate to Users page', async ({ page }) => {
        await page.goto('/');
        await page.click('text=Users');
        await expect(page).toHaveURL(/.*users/);
    });
});

test.describe('Dashboard Date Range', () => {
    test('should have date picker for custom range', async ({ page }) => {
        await page.goto('/');

        // 检查日期选择器是否存在
        await expect(page.getByTestId('date-range-picker')).toBeVisible();
    });

    test('should update data when date range changes', async ({ page }) => {
        let requestCount = 0;

        await page.route('**/global/activity**', async (route) => {
            requestCount++;
            await route.fulfill({
                status: 200,
                contentType: 'application/json',
                body: JSON.stringify({
                    daily_data: [],
                    sum_api_requests: requestCount * 1000,
                    sum_total_tokens: 0,
                    total_cost: 0,
                    avg_latency_ms: 0,
                    success_rate: 0,
                    unique_models: 0,
                    unique_providers: 0,
                }),
            });
        });

        await page.goto('/');

        // 点击日期选择器并选择新范围
        await page.getByTestId('date-range-picker').click();
        await page.getByTestId('date-range-7d').click();

        // 验证 API 被再次调用
        expect(requestCount).toBeGreaterThan(1);
    });
});
