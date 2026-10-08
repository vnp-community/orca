import { expect, test, type Page } from '@playwright/test'
import { CHANGE_OVERLAY_SMALL } from '../../../frontend/src/renderer/src/test-support/code-intel-fixtures'
import {
  bootApp,
  bootIntoReview,
  openRightSidebarTab,
  openWorktree,
  waitForSettingsPoll
} from './support/code-intel-app-navigation'
import { devStackUrl } from './support/code-intel-dev-backend'
import {
  createFakeCodeIntelBackend,
  mockCodeIntelApp,
  mockUser,
  type FakeCodeIntelBackend
} from './support/mock-code-intel-ws'

test.skip(!!devStackUrl, 'mocked variants only; the real stack runs dev-stack.web.e2e.ts')

// Review entry points (FE-CV-SOL-061) and the Review tab summary/reading order (FE-CV-SOL-051/052).

async function bootIntoWorktree(page: Page, backend: FakeCodeIntelBackend): Promise<void> {
  await mockCodeIntelApp(page, { backend, user: mockUser(), workspace: true })
  await bootApp(page)
  await openWorktree(page)
  await waitForSettingsPoll(backend)
}

const reviewEditorTab = (page: Page) => page.getByRole('tab', { name: 'Impact', exact: true })

test('flag off: Source Control has no Review entry and the palette has no Review action', async ({
  page
}) => {
  const backend = createFakeCodeIntelBackend()
  backend.setSettings({ codeIntelEnabled: false })
  await bootIntoWorktree(page, backend)
  await openRightSidebarTab(page, 'source-control')
  await expect(page.getByRole('button', { name: 'Commit', exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Review changes' })).toHaveCount(0)
  await page.getByRole('button', { name: 'Search worktrees and browser tabs' }).click()
  const input = page.getByRole('dialog', { name: 'Jump to...' }).getByRole('combobox')
  await expect(input).toBeFocused()
  await input.fill('review changes')
  await expect(page.getByRole('option', { name: /Review Changes/ })).toHaveCount(0)
  expect(backend.streamCount()).toBe(0)
})

test('Source Control "Review changes" opens the Review tab on the Impact lens', async ({
  page
}) => {
  const backend = createFakeCodeIntelBackend()
  backend.setOverlay(CHANGE_OVERLAY_SMALL)
  await bootIntoWorktree(page, backend)
  await openRightSidebarTab(page, 'source-control')
  await page.getByRole('button', { name: 'Review changes' }).click()
  await expect(reviewEditorTab(page)).toHaveAttribute('aria-selected', 'true')
  await expect(page.getByText('Reading order')).toBeVisible()
})

test('right-sidebar summary shows overlay counts and "Open full" opens the tab', async ({
  page
}) => {
  const backend = createFakeCodeIntelBackend()
  backend.setOverlay(CHANGE_OVERLAY_SMALL)
  await bootIntoWorktree(page, backend)
  // Why: the summary scope comes from the git branch compare that Source Control loads.
  await openRightSidebarTab(page, 'source-control')
  await expect(page.getByRole('button', { name: 'Review changes' })).toBeVisible()
  await openRightSidebarTab(page, 'review')
  await expect(page.getByText('vs main')).toBeVisible()
  await expect(page.getByRole('button', { name: '3 files' })).toBeVisible()
  await expect(page.getByRole('button', { name: '1 symbols' })).toBeVisible()
  await expect(page.getByText('Medium risk').first()).toBeVisible()
  // Why: the sidebar asks for the cheap summary; the full overlay belongs to the Review tab.
  expect(
    backend.callsTo('codeIntel.changeOverlay').map((c) => (c.params as { detail?: string }).detail)
  ).toContain('summary')
  await page.getByRole('button', { name: 'Open full' }).click()
  await expect(reviewEditorTab(page)).toBeVisible()
})

test('Review tab: summary bar, index chip and reading order with keyboard navigation', async ({
  page
}) => {
  const backend = createFakeCodeIntelBackend()
  backend.setOverlay(CHANGE_OVERLAY_SMALL)
  await mockCodeIntelApp(page, { backend, user: mockUser(), workspace: true })
  await bootIntoReview(page, backend)
  await expect(page.getByText('Index ready')).toBeVisible()
  await expect(page.getByRole('button', { name: /3 files/ }).first()).toBeVisible()
  await page.getByRole('radio', { name: 'Reading order', exact: true }).click()
  const list = page.getByRole('listbox', { name: 'Reading order' })
  // Component group header ("Order usecase", 0/2 read) followed by the two reading steps.
  await expect(list.getByRole('option')).toHaveCount(3)
  await expect(list.getByRole('option').first()).toContainText('0/2')
  await expect(list.getByRole('option').filter({ hasText: 'create.go' }).first()).toBeVisible()
  await list.focus()
  await page.keyboard.press('ArrowDown')
  await expect(list).toHaveAttribute('aria-activedescendant', /.+/)
  const first = await list.getAttribute('aria-activedescendant')
  await page.keyboard.press('ArrowDown')
  await expect(list).not.toHaveAttribute('aria-activedescendant', first ?? '')
  // Why: every string goes through translate(); a raw key leaking means a missing locale entry.
  await expect(page.getByText(/auto\.components\.reviewMap\./)).toHaveCount(0)
})

test('jump palette lists "Review Changes" and opens Review', async ({ page }) => {
  const backend = createFakeCodeIntelBackend()
  backend.setOverlay(CHANGE_OVERLAY_SMALL)
  await bootIntoWorktree(page, backend)
  await page.getByRole('button', { name: 'Search worktrees and browser tabs' }).click()
  const input = page.getByRole('dialog', { name: 'Jump to...' }).getByRole('combobox')
  await expect(input).toBeFocused()
  await input.fill('review changes')
  const action = page.getByRole('option', { name: /Review Changes/ })
  await expect(action).toBeVisible()
  await action.click()
  await expect(reviewEditorTab(page)).toBeVisible()
})
