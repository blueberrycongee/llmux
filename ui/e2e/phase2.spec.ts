/**
 * Phase 2 E2E Tests
 * 
 * Covering Teams, Users, Organizations, and API Keys management.
 */

import { test, expect } from '@playwright/test';

test.describe('Teams Management', () => {
    test.beforeEach(async ({ page }) => {
        // Mock Team List
        await page.route('**/team/list*', async (route) => {
            await route.fulfill({
                status: 200,
                contentType: 'application/json',
                body: JSON.stringify({
                    data: [
                        { team_id: 'team-1', team_alias: 'Engineering', is_active: true, blocked: false, spend: 100, models: [], members: [] },
                        { team_id: 'team-2', team_alias: 'Marketing', is_active: true, blocked: false, spend: 50, models: [], members: [] },
                    ],
                    total: 2
                }),
            });
        });

        // Mock Team Info
        await page.route('**/team/info*', async (route) => {
            await route.fulfill({
                status: 200,
                contentType: 'application/json',
                body: JSON.stringify({
                    team_id: 'team-1',
                    team_alias: 'Engineering',
                    is_active: true,
                    blocked: false,
                    max_budget: 1000,
                    spend: 100,
                    models: ['gpt-4'],
                    members: [
                        { user_id: 'user-1', role: 'admin' },
                        { user_id: 'user-2', role: 'user' }
                    ],
                    created_at: new Date().toISOString(),
                    updated_at: new Date().toISOString()
                }),
            });
        });

        await page.goto('/teams');
    });

    test('should display teams list', async ({ page }) => {
        // Use more specific selector - the main content h1
        await expect(page.getByRole('heading', { name: 'Teams', level: 1 })).toBeVisible();
        await expect(page.getByTestId('team-card-team-1')).toBeVisible();
        await expect(page.getByTestId('team-name-team-1')).toContainText('Engineering');
    });

    test('should navigate to team details', async ({ page }) => {
        // Click the "View Details" link within the team card
        await page.getByTestId('team-card-team-1').locator('a').click();
        await expect(page).toHaveURL(/.*teams\/team-1/);
        await expect(page.getByRole('heading', { name: 'Engineering' })).toBeVisible();
    });
});

test.describe('Users Management', () => {
    test.beforeEach(async ({ page }) => {
        // Mock User List
        await page.route('**/user/list*', async (route) => {
            await route.fulfill({
                status: 200,
                contentType: 'application/json',
                body: JSON.stringify({
                    data: [
                        { user_id: 'user-1', user_alias: 'Alice', user_email: 'alice@example.com', user_role: 'admin', spend: 50, max_budget: 500 },
                        { user_id: 'user-2', user_alias: 'Bob', user_email: 'bob@example.com', user_role: 'user', spend: 20, max_budget: 200 },
                    ],
                    total: 2
                }),
            });
        });

        await page.goto('/users');
    });

    test('should display users list', async ({ page }) => {
        await expect(page.getByRole('heading', { name: 'Users', level: 1 })).toBeVisible();
        await expect(page.getByText('Alice').first()).toBeVisible();
        await expect(page.getByText('bob@example.com').first()).toBeVisible();
    });

    test('should filter users', async ({ page }) => {
        await page.getByTestId('search-input').fill('Alice');
        // Wait for filter to apply
        await page.waitForTimeout(1000);
        await expect(page.getByText('Alice').first()).toBeVisible();
        // Bob should not be visible after filtering
        await expect(page.getByText('Bob')).not.toBeVisible();
    });
});

test.describe('API Keys Management', () => {
    test.beforeEach(async ({ page }) => {
        // Mock Key List
        await page.route('**/key/list*', async (route) => {
            await route.fulfill({
                status: 200,
                contentType: 'application/json',
                body: JSON.stringify({
                    data: [
                        {
                            id: 'key-1',
                            name: 'Prod Key',
                            key_prefix: 'sk-1234',
                            is_active: true,
                            blocked: false,
                            spent_budget: 10,
                            max_budget: 100,
                            created_at: new Date().toISOString()
                        },
                        {
                            id: 'key-2',
                            name: 'Dev Key',
                            key_prefix: 'sk-5678',
                            is_active: true,
                            blocked: true,
                            spent_budget: 0,
                            max_budget: 50,
                            created_at: new Date().toISOString()
                        },
                    ],
                    total: 2
                }),
            });
        });

        await page.goto('/api-keys');
    });

    test('should display api keys', async ({ page }) => {
        await expect(page.getByRole('heading', { name: 'API Keys', level: 1 })).toBeVisible();
        await expect(page.getByTestId('key-name-key-1')).toContainText('Prod Key');
        await expect(page.getByTestId('key-name-key-2')).toContainText('Dev Key');
    });

    test('should filter api keys', async ({ page }) => {
        await page.getByTestId('search-input').fill('Prod');
        await page.waitForTimeout(1000);
        await expect(page.getByTestId('key-name-key-1')).toBeVisible();
        await expect(page.getByTestId('key-row-key-2')).not.toBeVisible();
    });

    test('should open details sheet', async ({ page }) => {
        await page.getByTestId('key-row-key-1').click();
        await expect(page.getByText('API Key Details')).toBeVisible();
    });
});

test.describe('Organizations Management', () => {
    test.beforeEach(async ({ page }) => {
        // Mock Organization List
        await page.route('**/organization/list*', async (route) => {
            await route.fulfill({
                status: 200,
                contentType: 'application/json',
                body: JSON.stringify({
                    data: [
                        {
                            organization_id: 'org-1',
                            organization_alias: 'Acme Corp',
                            spend: 500,
                            max_budget: 5000,
                            models: [],
                            created_at: new Date().toISOString(),
                            updated_at: new Date().toISOString()
                        },
                    ],
                    total: 1
                }),
            });
        });

        await page.goto('/organizations');
    });

    test('should display organizations list', async ({ page }) => {
        await expect(page.getByRole('heading', { name: 'Organizations', level: 1 })).toBeVisible();
        await expect(page.getByTestId('org-card-org-1')).toBeVisible();
        await expect(page.getByTestId('org-name-org-1')).toContainText('Acme Corp');
    });
});
