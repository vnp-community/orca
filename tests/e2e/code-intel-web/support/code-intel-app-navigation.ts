import { expect, type Page } from '@playwright/test'
import type { FakeCodeIntelBackend } from './mock-code-intel-ws'

export const SPA_URL = 'http://127.0.0.1:5174'

/** Boots the web SPA and waits until the shell is interactive. */
export async function bootApp(page: Page): Promise<void> {
  await page.goto('/web-index.html')
  await expect(page.getByRole('button', { name: 'Settings' }).first()).toBeVisible()
}

/** Review tab entry points only exist once a worktree is open; callers poll the backend instead. */
export async function waitForSettingsPoll(backend: FakeCodeIntelBackend): Promise<void> {
  await expect
    .poll(() => backend.callsTo('codeIntel.settings.get').length, { timeout: 15_000 })
    .toBeGreaterThan(0)
}
