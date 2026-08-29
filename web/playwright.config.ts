import { defineConfig, devices } from '@playwright/test'

/**
 * Playwright E2E configuration.
 *
 * Strategy: run against the Vite dev server on port 3000 (same as `npm run dev`).
 * API calls to /api/v1 are intercepted/mocked in test files so the suite does
 * not require a running backend — keeping CI fast and hermetic.
 *
 * Only Chromium is enabled by default to keep run time short; add more
 * projects here when cross-browser coverage is needed.
 */
export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: process.env.CI ? [['github'], ['list']] : 'list',
  use: {
    baseURL: 'http://127.0.0.1:3000',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
        // Local Windows verification can reuse an installed Chrome when the
        // Playwright-managed browser download is unavailable. CI keeps using
        // the pinned Playwright Chromium by default.
        ...(process.env.PLAYWRIGHT_USE_SYSTEM_CHROME ? { channel: 'chrome' as const } : {}),
      },
    },
  ],
  webServer: process.env.PLAYWRIGHT_EXTERNAL_SERVER ? undefined : {
    // Launch Vite directly. On Windows, launching it through npm leaves the
    // child process attached after the suite and Playwright never exits.
    command: 'node ./node_modules/vite/bin/vite.js --host 127.0.0.1',
    url: 'http://127.0.0.1:3000',
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
    gracefulShutdown: {
      signal: 'SIGTERM',
      timeout: 1_000,
    },
  },
})
