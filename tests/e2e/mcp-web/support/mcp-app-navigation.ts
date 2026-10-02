import { expect, type Page } from '@playwright/test'
import type { FakeMcpBackend } from './mock-orca-ws'

export const SPA_URL = 'http://127.0.0.1:5174'

/** Boots the web SPA and waits until the MCP event stream is open (so pushes are not missed). */
export async function bootApp(page: Page, backend: FakeMcpBackend): Promise<void> {
  await page.goto('/web-index.html')
  await expect(page.getByRole('button', { name: 'Settings' }).first()).toBeVisible()
  await expect.poll(() => backend.streamCount(), { timeout: 15_000 }).toBeGreaterThan(0)
}

/** Settings > MCP > <tab>, through the UI (Settings is store-navigated, there is no URL). */
export async function openMcpTab(page: Page, tab: string): Promise<void> {
  const mcpNav = page.getByRole('button', { name: 'MCP', exact: true })
  const settings = page.getByRole('button', { name: 'Settings' }).first()
  await expect(settings.or(mcpNav).first()).toBeVisible()
  // Why: after a reload the app may restore straight into Settings.
  if (!(await mcpNav.isVisible())) {
    await settings.click()
  }
  await mcpNav.click()
  await page.getByRole('tab', { name: tab }).click()
}

/** Same message the service worker posts on notificationclick. */
export async function simulatePushClick(page: Page, url: string): Promise<void> {
  await page.evaluate((u) => {
    navigator.serviceWorker.dispatchEvent(
      new MessageEvent('message', { data: { type: 'orca:navigate', url: u } })
    )
  }, url)
}
