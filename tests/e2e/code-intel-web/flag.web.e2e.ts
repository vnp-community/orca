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
  mockUser
} from './support/mock-code-intel-ws'

test.skip(!!devStackUrl, 'mocked variants only; the real stack runs dev-stack.web.e2e.ts')

const errorsOn = (page: Page): string[] => {
  const errors: string[] = []
  page.on('pageerror', (e) => errors.push(e.message))
  return errors
}

// Right-sidebar activity button; its name is the tab title (plus an attention suffix).
const reviewActivity = (page: Page) => page.getByRole('button', { name: /^Review(,|$)/ })
const nonSettingsCalls = (methods: string[]): string[] =>
  methods.filter((m) => m !== 'codeIntel.settings.get')

test('flag off: no Review entry, no code-intel stream, only settings.get', async ({ page }) => {
  const backend = createFakeCodeIntelBackend()
  backend.setSettings({ codeIntelEnabled: false })
  const { wsMethods } = await mockCodeIntelApp(page, { backend, user: mockUser(), workspace: true })
  const errors = errorsOn(page)
  await bootApp(page)
  await openWorktree(page)
  await waitForSettingsPoll(backend)
  await openRightSidebarTab(page, 'explorer')
  await expect(page.getByRole('button', { name: /^Explorer/ })).toBeVisible()
  await expect(reviewActivity(page)).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Open full' })).toHaveCount(0)
  expect(backend.streamCount()).toBe(0)
  expect(wsMethods).not.toContain('codeIntel.subscribe')
  expect(nonSettingsCalls(backend.calls.map((c) => c.method))).toEqual([])
  expect(errors.filter((m) => /code-?intel/i.test(m))).toEqual([])
})

test('flag on: Review entry shows and the Review tab opens', async ({ page }) => {
  const backend = createFakeCodeIntelBackend()
  backend.setOverlay(CHANGE_OVERLAY_SMALL)
  const { wsMethods } = await mockCodeIntelApp(page, { backend, user: mockUser(), workspace: true })
  await bootIntoReview(page, backend)
  await expect(reviewActivity(page)).toBeVisible()
  await expect(page.getByText('Reading order')).toBeVisible()
  // One app-wide push stream, opened only once the flag reads enabled.
  await expect.poll(() => backend.streamCount()).toBe(1)
  expect(wsMethods.filter((m) => m === 'codeIntel.subscribe')).toHaveLength(1)
  await expect(page.getByRole('tab', { name: 'Impact', exact: true })).toBeVisible()
})

test('quality off keeps Review and refuses quality channels', async ({ page }) => {
  const backend = createFakeCodeIntelBackend()
  backend.setOverlay(CHANGE_OVERLAY_SMALL)
  backend.setSettings({ qualityGateEnabled: false })
  await mockCodeIntelApp(page, { backend, user: mockUser(), workspace: true })
  await bootIntoReview(page, backend)
  await expect(page.getByText('Reading order')).toBeVisible()
  // Why: with the gate off the client must not even try quality.* (contract §6 levels).
  expect(
    backend.calls.map((c) => c.method).filter((m) => m.startsWith('codeIntel.quality.'))
  ).toEqual([])
  await expect(backend.call('codeIntel.quality.gate', backend.selector)).rejects.toThrow(
    /CODEINTEL_QUALITY_GATE_DISABLED/
  )
})

test('turned off mid-session: the 60 s settings poll hides Review', async ({ page }) => {
  await page.clock.install()
  const backend = createFakeCodeIntelBackend()
  backend.setOverlay(CHANGE_OVERLAY_SMALL)
  await mockCodeIntelApp(page, { backend, user: mockUser(), workspace: true })
  await bootIntoReview(page, backend)
  await expect(reviewActivity(page)).toBeVisible()
  const polls = backend.callsTo('codeIntel.settings.get').length
  backend.setSettings({ codeIntelEnabled: false })
  await page.clock.fastForward(60_000)
  await expect.poll(() => backend.callsTo('codeIntel.settings.get').length).toBeGreaterThan(polls)
  await expect(page.getByText('Code review is turned off')).toBeVisible()
  await expect(reviewActivity(page)).toHaveCount(0)
})

test('CODEINTEL_DISABLED on the next call hides Review without a toast', async ({ page }) => {
  const backend = createFakeCodeIntelBackend({ role: 'admin' })
  await mockCodeIntelApp(page, { backend, user: mockUser('admin'), workspace: true })
  await bootApp(page)
  await openWorktree(page)
  await waitForSettingsPoll(backend)
  await backend.call('codeIntel.settings.set', { codeIntelEnabled: false })
  await expect(backend.call('codeIntel.status', backend.selector)).rejects.toThrow(
    /CODEINTEL_DISABLED/
  )
  await expect(page.locator('[data-sonner-toast]').filter({ hasText: /code ?intel/i })).toHaveCount(
    0
  )
})

test('CODEINTEL_UNAVAILABLE on settings.get: no entry and no red error', async ({ page }) => {
  const backend = createFakeCodeIntelBackend()
  backend.setHandler('codeIntel.settings.get', () => {
    throw new Error('CODEINTEL_UNAVAILABLE: code intelligence is not deployed')
  })
  await mockCodeIntelApp(page, { backend, user: mockUser(), workspace: true })
  const errors = errorsOn(page)
  await bootApp(page)
  await openWorktree(page)
  await waitForSettingsPoll(backend)
  await openRightSidebarTab(page, 'explorer')
  await expect(page.getByRole('button', { name: /^Explorer/ })).toBeVisible()
  await expect(reviewActivity(page)).toHaveCount(0)
  await expect(page.locator('[data-sonner-toast]')).toHaveCount(0)
  expect(nonSettingsCalls(backend.calls.map((c) => c.method))).toEqual([])
  expect(errors.filter((m) => /code-?intel/i.test(m))).toEqual([])
})
