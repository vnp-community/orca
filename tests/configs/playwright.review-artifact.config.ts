import { defineConfig, devices } from '@playwright/test'

/**
 * The Outcome Review's drawing sandbox, in a real browser (FE-OR-T220). No app, no stack and no web server: the spec serves its
 * own host page.
 *
 *   npx playwright test -c tests/configs/playwright.review-artifact.config.ts
 */
export default defineConfig({
  testDir: '../stages/e2e/orca-flows/ui-live',
  testMatch: '**/review-artifact-sandbox.web.e2e.ts',
  timeout: 60_000,
  expect: { timeout: 10_000 },
  retries: 0,
  reporter: 'list',
  use: {
    ...devices['Desktop Chrome'],
    trace: 'off',
    screenshot: 'off',
    ...(process.env.MCP_E2E_CHROMIUM_PATH
      ? { launchOptions: { executablePath: process.env.MCP_E2E_CHROMIUM_PATH } }
      : {})
  }
})
