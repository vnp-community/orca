import { expect, type Locator, type Page } from '@playwright/test'
import type { FakeCodeIntelBackend } from './mock-code-intel-ws'
import { E2E_WORKTREE_NAME } from './code-intel-workspace-stubs'

export const SPA_URL = 'http://127.0.0.1:5174'

const RENDERER_ERROR = 'Orca web hit a renderer error.'

/**
 * Boots the web SPA and waits until the shell is interactive. Retries the navigation because Vite
 * dev serves hundreds of modules and Chromium aborts in-flight loads with ERR_NETWORK_CHANGED
 * whenever the host's interfaces churn (Docker veths), which leaves a blank page or the
 * renderer-error boundary.
 */
export async function bootApp(page: Page, attempts = 3): Promise<void> {
  const settings = page.getByRole('button', { name: 'Settings' }).first()
  for (let i = 1; ; i++) {
    await page.goto('/web-index.html')
    try {
      await expect(settings.or(page.getByText(RENDERER_ERROR))).toBeVisible({ timeout: 20_000 })
      if (await settings.isVisible()) {
        return
      }
    } catch (e) {
      if (i >= attempts) {
        throw e
      }
      continue
    }
    if (i >= attempts) {
      throw new Error(`web SPA showed "${RENDERER_ERROR}" on ${attempts} boots`)
    }
  }
}

/** Closes one-off overlays (shortcut tips, onboarding dialogs) that pop up on their own timer. */
export async function dismissOverlays(page: Page): Promise<void> {
  for (let i = 0; i < 3; i++) {
    const later = page.getByRole('button', { name: /^(Got it|Maybe Later)$/ }).first()
    if (await later.isVisible().catch(() => false)) {
      await later.click()
      continue
    }
    if (
      await page
        .locator('[data-slot="dialog-overlay"]')
        .first()
        .isVisible()
        .catch(() => false)
    ) {
      await page.keyboard.press('Escape')
      continue
    }
    return
  }
}

/** Activates the stub worktree (`mockCodeIntelApp({ workspace: true })`) from the left sidebar. */
export async function openWorktree(page: Page): Promise<void> {
  const card = page.getByText(E2E_WORKTREE_NAME, { exact: true }).first()
  await expect(card).toBeVisible()
  for (let i = 0; ; i++) {
    await dismissOverlays(page)
    try {
      await card.click({ timeout: 5_000 })
      break
    } catch (e) {
      if (i >= 3) {
        throw e
      }
    }
  }
  await page.waitForTimeout(300)
  await dismissOverlays(page)
}

/**
 * Shows a right-sidebar tab. Setup goes through the store (allowed by tests/e2e/AGENTS.md): at
 * 1280px the activity bar overflows into a menu covered by the title-bar overlay, and whether the
 * sidebar starts open depends on restored UI state.
 */
export async function openRightSidebarTab(page: Page, tab: string): Promise<void> {
  await dismissOverlays(page)
  await page.evaluate((t) => {
    const store = (window as unknown as { __store: { getState: () => Record<string, unknown> } })
      .__store
    const s = store.getState() as {
      setRightSidebarOpen: (open: boolean) => void
      setRightSidebarTab: (tab: string) => void
    }
    s.setRightSidebarOpen(true)
    s.setRightSidebarTab(t)
  }, tab)
}

/** The right-sidebar Review summary panel (FE-CV-SOL-061 entry point). */
export function reviewSummaryPanel(page: Page): Locator {
  return page.getByRole('button', { name: 'Open full' }).locator('xpath=../..')
}

/** Opens the Review editor tab through the right-sidebar summary ("Open full"). */
export async function openReviewTab(page: Page): Promise<void> {
  await openRightSidebarTab(page, 'review')
  const openFull = page.getByRole('button', { name: 'Open full' })
  await expect(openFull).toBeVisible()
  await dismissOverlays(page)
  await openFull.click()
}

/** Review tab entry points only exist once a worktree is open; callers poll the backend instead. */
export async function waitForSettingsPoll(backend: FakeCodeIntelBackend): Promise<void> {
  await expect
    .poll(() => backend.callsTo('codeIntel.settings.get').length, { timeout: 15_000 })
    .toBeGreaterThan(0)
}

/** Boot → stub worktree → flag poll → Review tab via the sidebar summary ("Open full"). */
export async function bootIntoReview(page: Page, backend: FakeCodeIntelBackend): Promise<void> {
  await bootApp(page)
  await openWorktree(page)
  await waitForSettingsPoll(backend)
  await openReviewTab(page)
  await expect(reviewHeading(page)).toBeVisible()
}

/** The Review editor tab's title heading ("Review · <branch>"), present in every body state. */
export function reviewHeading(page: Page): Locator {
  return page.getByRole('heading', { level: 1, name: /^Review/ })
}

/** Visible Review editor body (the summary bar, lenses and dock live under it). */
export function reviewLensTab(page: Page, name: string): Locator {
  return page.getByRole('tab', { name, exact: true })
}
