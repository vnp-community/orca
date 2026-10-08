import { expect, test, type Page } from '@playwright/test'
import {
  CHANGE_OVERLAY_SMALL,
  CHANGE_OVERLAY_TRUNCATED,
  INDEX_STATUS_FIXTURES
} from '../../../frontend/src/renderer/src/test-support/code-intel-fixtures'
import { bootIntoReview } from './support/code-intel-app-navigation'
import { devStackUrl } from './support/code-intel-dev-backend'
import {
  createFakeCodeIntelBackend,
  mockCodeIntelApp,
  mockUser,
  type FakeCodeIntelBackend
} from './support/mock-code-intel-ws'

test.skip(!!devStackUrl, 'mocked variants only; the real stack runs dev-stack.web.e2e.ts')

// Review body states (FE-CV-SOL-051 decision table) driven by fake backend errors/fixtures.

async function openReview(
  page: Page,
  seed: (backend: FakeCodeIntelBackend) => void
): Promise<FakeCodeIntelBackend> {
  const backend = createFakeCodeIntelBackend()
  backend.setOverlay(CHANGE_OVERLAY_SMALL)
  seed(backend)
  await mockCodeIntelApp(page, { backend, user: mockUser(), workspace: true })
  await bootIntoReview(page, backend)
  return backend
}

const alwaysFail = (backend: FakeCodeIntelBackend, method: string, message: string): void =>
  backend.setHandler(method, () => {
    throw new Error(message)
  })

test('INDEX_MISSING: explains and "Build index" starts a reindex', async ({ page }) => {
  const backend = await openReview(page, (b) => b.setIndex(INDEX_STATUS_FIXTURES.missing))
  await expect(page.getByText('There is no index for this repository yet')).toBeVisible()
  await page.getByRole('button', { name: 'Build index' }).click()
  await expect.poll(() => backend.callsTo('codeIntel.reindex').length).toBe(1)
})

test('TOOL_UNAVAILABLE: install hint with a retry', async ({ page }) => {
  await openReview(page, (b) =>
    alwaysFail(b, 'codeIntel.changeOverlay', 'CODEINTEL_TOOL_UNAVAILABLE: no index tool on host')
  )
  await expect(page.getByText('No code index tool is installed')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Try again' })).toBeVisible()
})

test('NOT_AUTHORIZED: neutral access message, no raw error code', async ({ page }) => {
  await openReview(page, (b) =>
    alwaysFail(b, 'codeIntel.changeOverlay', 'CODEINTEL_NOT_AUTHORIZED: read access required')
  )
  await expect(page.getByText('You do not have access to this review')).toBeVisible()
  await expect(page.getByText('CODEINTEL_NOT_AUTHORIZED')).toHaveCount(0)
})

test('DEV_SERVER_OFFLINE: offline screen instead of an endless spinner', async ({ page }) => {
  await openReview(page, (b) =>
    alwaysFail(b, 'codeIntel.changeOverlay', 'CODEINTEL_DEV_SERVER_OFFLINE: dev server unreachable')
  )
  await expect(page.getByText('Cannot reach the code index')).toBeVisible()
  await expect(page.getByText('Loading changes')).toHaveCount(0)
})

test('RESPONSE_TOO_LARGE: error screen, then "Try again" recovers', async ({ page }) => {
  const backend = await openReview(page, (b) =>
    alwaysFail(b, 'codeIntel.changeOverlay', 'CODEINTEL_RESPONSE_TOO_LARGE: overlay too large')
  )
  await expect(page.getByText('The review could not be loaded')).toBeVisible()
  backend.setHandler('codeIntel.changeOverlay', () => ({
    worktreeId: backend.selector.worktreeId,
    view: 'changeOverlay',
    sources: [],
    headCommit: null,
    stale: false,
    truncated: false,
    totalCount: 3,
    etag: 'etag-recovered',
    fromCache: false,
    generatedAt: new Date().toISOString(),
    data: CHANGE_OVERLAY_SMALL
  }))
  await page.getByRole('button', { name: 'Try again' }).first().click()
  await expect(page.getByText('Reading order')).toBeVisible()
})

test('truncated overlay shows the "Showing N of M files" banner', async ({ page }) => {
  await openReview(page, (b) => b.setOverlay(CHANGE_OVERLAY_TRUNCATED))
  await expect(page.getByText(/Showing 3 of \d+ files/)).toBeVisible()
})

test('labels with HTML or <script> render as text (U9)', async ({ page }) => {
  const hostile = '<img src=x onerror="window.__xss=1"><script>window.__xss=2</script>'
  await openReview(page, (b) =>
    b.setOverlay({
      ...CHANGE_OVERLAY_SMALL,
      components: CHANGE_OVERLAY_SMALL.components.map((c, i) =>
        i === 0 ? { ...c, label: hostile } : c
      )
    })
  )
  await page.getByRole('radio', { name: 'Reading order', exact: true }).click()
  await expect(page.getByText(hostile).first()).toBeVisible()
  expect(await page.evaluate(() => (window as { __xss?: number }).__xss)).toBeUndefined()
})
