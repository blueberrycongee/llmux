import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright 配置
 * @see https://playwright.dev/docs/test-configuration
 */
export default defineConfig({
    testDir: './e2e',

    /* 测试并行执行 */
    fullyParallel: true,

    /* CI 环境下禁止 test.only */
    forbidOnly: !!process.env.CI,

    /* CI 环境下重试失败的测试 */
    retries: process.env.CI ? 2 : 0,

    /* CI 环境下限制并行 workers */
    workers: process.env.CI ? 1 : undefined,

    /* 测试报告 */
    reporter: 'html',

    /* 共享设置 */
    use: {
        /* 基础 URL */
        baseURL: 'http://localhost:3000',

        /* 收集失败测试的 trace */
        trace: 'on-first-retry',

        /* 截图 */
        screenshot: 'only-on-failure',
    },

    /* 项目配置 - 多浏览器测试 */
    projects: [
        {
            name: 'chromium',
            use: { ...devices['Desktop Chrome'] },
        },

        {
            name: 'firefox',
            use: { ...devices['Desktop Firefox'] },
        },

        {
            name: 'webkit',
            use: { ...devices['Desktop Safari'] },
        },

        /* 移动端测试 */
        {
            name: 'Mobile Chrome',
            use: { ...devices['Pixel 5'] },
        },

        {
            name: 'Mobile Safari',
            use: { ...devices['iPhone 12'] },
        },
    ],

    /* 开发环境自动启动 dev server */
    webServer: {
        command: 'npm run dev',
        url: 'http://localhost:3000',
        reuseExistingServer: !process.env.CI,
        timeout: 120 * 1000,
    },
});
