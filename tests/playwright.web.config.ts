import { resolve } from 'node:path'
import { defineConfig, devices } from '@playwright/test'

/**
 * Browser-only Playwright config for the MCP web specs (FE-MCP-SOL-012).
 *
 * Separate from playwright.config.ts on purpose: that one is Electron-only and its globalSetup
 * builds out/main/index.js. Here there is no globalSetup, and the `.web.e2e.ts` suffix never
 * matches the Electron project's `**\/*.spec.ts`.
 *
 *   npx playwright test -c tests/playwright.web.config.ts            # mocked WebSocket backend
 *   MCP_E2E_BASE_URL=http://localhost:8081 npx playwright test -c tests/playwright.web.config.ts
 *                                                                    # also runs @dev-stack specs
 */
const BASE = process.env.MCP_E2E_BASE_URL ?? process.env.CODE_INTEL_E2E_BASE_URL
// Why: an already-served SPA (e.g. `vite preview` of a `--mode development` build) for mocked
// specs; Vite dev's ~2k module loads abort with ERR_NETWORK_CHANGED on hosts with churning NICs.
const SPA = process.env.CODE_INTEL_E2E_SPA_URL
const LOCAL = SPA ?? 'http://127.0.0.1:5174'

export default defineConfig({
  testMatch: '**/*.web.e2e.ts',
  timeout: 60_000,
  expect: { timeout: 10_000 },
  retries: 0,
  forbidOnly: !!process.env.CI,
  reporter: 'list',
  use: {
    baseURL: BASE ?? LOCAL,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    // Why: lets a machine whose cached browser build differs from this Playwright's pinned one
    // (offline CI images) point at an installed Chromium instead of failing on launch.
    ...(process.env.MCP_E2E_CHROMIUM_PATH
      ? { launchOptions: { executablePath: process.env.MCP_E2E_CHROMIUM_PATH } }
      : {})
  },
  // Why: one project per feature so `--project` selects a suite and fake backends never mix.
  projects: [
    {
      name: 'mcp-web',
      // Why: no testDir here (other suites under tests/e2e run through this project), but
      // code-intel specs need their own fake backend and run only in `code-intel-web`.
      testIgnore: '**/code-intel-web/**',
      use: { ...devices['Desktop Chrome'] }
    },
    {
      name: 'code-intel-web',
      testDir: './e2e/code-intel-web',
      use: { ...devices['Desktop Chrome'] }
    }
  ],
  // Why: mocked specs need the web SPA served by Vite; against a real stack the stack serves it.
  webServer:
    BASE || SPA
      ? undefined
      : {
          command: 'npx vite --port 5174 --host 127.0.0.1 --strictPort',
          cwd: resolve(__dirname, '../frontend'),
          url: `${LOCAL}/web-index.html`,
          reuseExistingServer: !process.env.CI,
          timeout: 120_000,
          // Why: shared/CI hosts often exhaust inotify watchers (ENOSPC) and Vite then exits at boot;
          // polling is slower but always starts. Opt out with CODE_INTEL_E2E_NO_POLLING=1.
          env: process.env.CODE_INTEL_E2E_NO_POLLING ? {} : { CHOKIDAR_USEPOLLING: '1' }
        }
})
