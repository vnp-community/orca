/**
 * Browser-session plumbing for the live UI flow: sign in through the real form, keep the password out of traces,
 * collect browser-side errors and take numbered screenshots.
 */
import {
  expect,
  type BrowserContext,
  type Locator,
  type Page,
  type TestInfo
} from '@playwright/test'
import type { LiveStackConfig } from './live-stack-config'

const SHELL_READY_MS = 45_000

/**
 * Sidebar entry of the Requests page. The icon fallback covers a build whose button loses its name while the
 * pending-approvals badge is shown (the badge text then becomes the accessible name).
 */
export function requestsNavButton(page: Page): Locator {
  return page
    .getByTestId('sidebar-requests')
    .or(page.getByRole('button', { name: 'Requests', exact: true }))
    .or(page.locator('button:has(svg.lucide-inbox)'))
    .first()
}

export type BrowserNoise = {
  consoleErrors: string[]
  pageErrors: string[]
  failedCalls: string[]
  rpcErrors: string[]
}

export function watchBrowserNoise(page: Page): BrowserNoise {
  const noise: BrowserNoise = { consoleErrors: [], pageErrors: [], failedCalls: [], rpcErrors: [] }
  // Why: an RPC the UI sent and the gateway refused is the evidence behind most "button did nothing" findings.
  page.on('websocket', (ws) => {
    const methods = new Map<string, string>()
    ws.on('framesent', (f) => {
      try {
        const m = JSON.parse(String(f.payload)) as { id?: string; method?: string }
        if (m.id && m.method) {
          methods.set(m.id, m.method)
        }
      } catch {
        // not a JSON frame
      }
    })
    ws.on('framereceived', (f) => {
      try {
        const m = JSON.parse(String(f.payload)) as {
          id?: string
          ok?: boolean
          error?: { code?: string; message?: string }
        }
        if (m.ok === false && m.id) {
          noise.rpcErrors.push(
            `${methods.get(m.id) ?? '?'} -> ${m.error?.code ?? ''} ${(m.error?.message ?? '').slice(0, 160)}`
          )
        }
      } catch {
        // not a JSON frame
      }
    })
  })
  // Why: the app asks "Leave site?" on unload; an unanswered beforeunload dialog makes page.reload() wait forever.
  page.on('dialog', (d) => void d.accept().catch(() => {}))
  page.on('console', (m) => {
    if (m.type() === 'error') {
      noise.consoleErrors.push(m.text().slice(0, 200))
    }
  })
  page.on('pageerror', (e) => noise.pageErrors.push(String(e).slice(0, 200)))
  page.on('response', (r) => {
    const url = new URL(r.url())
    // Why: /auth/me answers 401 before sign-in by design; only failures after that are findings.
    if (
      r.status() >= 400 &&
      !url.pathname.startsWith('/assets/') &&
      !(r.status() === 401 && url.pathname === '/auth/me')
    ) {
      noise.failedCalls.push(`${r.status()} ${url.pathname}`)
    }
  })
  return noise
}

/**
 * Signs in through the login form. Tracing is started only afterwards (see `startTraceAfterSignIn`) because a trace
 * records typed values, and the password must never reach an artifact.
 */
export async function signInThroughTheForm(page: Page, cfg: LiveStackConfig): Promise<void> {
  await page.goto('/')
  const email = page.locator('#login-email')
  const settings = page.getByRole('button', { name: 'Settings' }).first()
  await expect(email.or(settings)).toBeVisible({ timeout: SHELL_READY_MS })
  if (await email.isVisible()) {
    await email.fill(cfg.adminEmail)
    await page.locator('#login-password').fill(cfg.adminPassword)
    await page.getByRole('button', { name: 'Sign In' }).click()
  }
  await expect(requestsNavButton(page)).toBeVisible({ timeout: SHELL_READY_MS })
  await dismissFirstRunTip(page)
}

/** Closes the first-run tip and feature-tour dialogs ("Got it", "Maybe Later") that can cover the shell after a load. */
export async function dismissFirstRunTip(page: Page): Promise<void> {
  const dismiss = page.getByRole('button', { name: /^(Got it|Maybe Later)$/ })
  for (let i = 0; i < 3; i++) {
    // Why: tours appear a moment after the shell, so wait briefly before deciding there is none.
    const clicked = await dismiss
      .first()
      .click({ timeout: i === 0 ? 8_000 : 2_000 })
      .then(
        () => true,
        () => false
      )
    if (!clicked) {
      return
    }
  }
}

export async function startTraceAfterSignIn(context: BrowserContext): Promise<void> {
  await context.tracing.start({ screenshots: true, snapshots: true })
}

export async function stopTrace(
  context: BrowserContext,
  testInfo: TestInfo,
  keep: boolean
): Promise<void> {
  await context.tracing
    .stop(keep ? { path: testInfo.outputPath('trace.zip') } : undefined)
    .catch(() => {})
}

export class StepShots {
  private n = 0

  constructor(
    private readonly page: Page,
    private readonly testInfo: TestInfo
  ) {}

  async take(label: string): Promise<void> {
    this.n++
    const name = `${String(this.n).padStart(2, '0')}-${label}.png`
    await this.page.screenshot({ path: this.testInfo.outputPath(name) }).catch(() => {})
  }
}
