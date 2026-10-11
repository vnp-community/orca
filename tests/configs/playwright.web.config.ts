import { resolve } from 'node:path'
import { defineConfig, devices } from '@playwright/test'
import { isLiveRun, liveFrontendUrl } from '../stages/e2e/orca-flows/ui-live/live-stack-config'

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
 *   ORCA_E2E_LIVE=1 npx playwright test -c tests/configs/playwright.web.config.ts --project=request-live
 *                                                                    # Request flow through the UI on the local deploy/dev stack
 */
const BASE = process.env.MCP_E2E_BASE_URL ?? process.env.CODE_INTEL_E2E_BASE_URL
// Why: an already-served SPA (e.g. `vite preview` of a `--mode development` build) for mocked
// specs; Vite dev's ~2k module loads abort with ERR_NETWORK_CHANGED on hosts with churning NICs.
const SPA = process.env.CODE_INTEL_E2E_SPA_URL
const LOCAL = SPA ?? 'http://127.0.0.1:5174'
// Why: the live project talks to the already running stack; booting Vite for it would only hide a missing stack.
const LIVE_ONLY = isLiveRun() || process.argv.some((a) => a.includes('request-live'))

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
      testIgnore: ['**/code-intel-web/**'],
      use: { ...devices['Desktop Chrome'] }
    },
    {
      name: 'code-intel-web',
      testDir: '../stages/e2e/legacy/code-intel-web',
      use: { ...devices['Desktop Chrome'] }
    },
    {
      // Why: no mocks; the spec skips itself unless ORCA_E2E_LIVE=1. Trace is off because a trace records typed values
      // and the spec signs in with the stack's admin password; the spec starts its own trace after the login form.
      name: 'request-live',
      testDir: '../stages/e2e/orca-flows/ui-live',
      timeout: 20 * 60_000,
      use: {
        ...devices['Desktop Chrome'],
        baseURL: liveFrontendUrl(),
        trace: 'off',
        screenshot: 'off',
        // Why: Playwright's default action timeout is none, so a missing element would hang until the 20 min test timeout.
        actionTimeout: 30_000,
        navigationTimeout: 45_000,
        viewport: { width: 1440, height: 900 }
      }
    }
  ],
  // Why: mocked specs need the web SPA served by Vite; against a real stack the stack serves it.
  webServer:
    BASE || SPA || LIVE_ONLY
      ? undefined
      : {
          command: 'npx vite --port 5174 --host 127.0.0.1 --strictPort',
          cwd: resolve(__dirname, '../../frontend'),
          url: `${LOCAL}/web-index.html`,
          reuseExistingServer: !process.env.CI,
          timeout: 120_000,
          // Why: shared/CI hosts often exhaust inotify watchers (ENOSPC) and Vite then exits at boot;
          // polling is slower but always starts. Opt out with CODE_INTEL_E2E_NO_POLLING=1.
          env: process.env.CODE_INTEL_E2E_NO_POLLING ? {} : { CHOKIDAR_USEPOLLING: '1' }
        }
})
