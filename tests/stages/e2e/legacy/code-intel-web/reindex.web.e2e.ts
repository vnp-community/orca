import { expect, test, type Page } from '@playwright/test'
import { CHANGE_OVERLAY_SMALL } from '../../../../../frontend/src/renderer/src/test-support/code-intel-fixtures'
import { bootIntoReview } from './support/code-intel-app-navigation'
import { devStackUrl } from './support/code-intel-dev-backend'
import { E2E_WORKTREE_ID } from './support/code-intel-workspace-stubs'
import {
  createFakeCodeIntelBackend,
  mockCodeIntelApp,
  mockUser,
  type FakeCodeIntelBackend
} from './support/mock-code-intel-ws'

test.skip(!!devStackUrl, 'mocked variants only; the real stack runs dev-stack.web.e2e.ts')

// Reindex from the index chip and the push stream (reindexProgress/changed/resync), FE-CV-SOL-051.

async function openReview(page: Page): Promise<FakeCodeIntelBackend> {
  const backend = createFakeCodeIntelBackend()
  backend.setOverlay(CHANGE_OVERLAY_SMALL)
  await mockCodeIntelApp(page, { backend, user: mockUser(), workspace: true })
  await bootIntoReview(page, backend)
  await expect.poll(() => backend.streamCount()).toBe(1)
  return backend
}

async function openIndexPopover(page: Page): Promise<void> {
  await page.getByRole('button', { name: /^Index status:/ }).click()
  await expect(page.getByRole('button', { name: 'Refresh index' })).toBeVisible()
}

test('Refresh index → reindexProgress → changed offers "Update now" and refetches', async ({
  page
}) => {
  const backend = await openReview(page)
  await openIndexPopover(page)
  await page.getByRole('button', { name: 'Refresh index' }).click()
  await expect.poll(() => backend.callsTo('codeIntel.reindex').length).toBe(1)
  expect(backend.callsTo('codeIntel.reindex')[0].params).toMatchObject({ mode: 'incremental' })
  // percent:null is "unknown", the fake pushes it on every reindex.
  backend.pushProgress(E2E_WORKTREE_ID, null, true)
  await page.keyboard.press('Escape')
  const overlays = backend.callsTo('codeIntel.changeOverlay').length
  backend.pushChanged(E2E_WORKTREE_ID)
  await expect(page.getByText('New data is available.')).toBeVisible()
  await page.getByRole('button', { name: 'Update now' }).click()
  await expect
    .poll(() => backend.callsTo('codeIntel.changeOverlay').length)
    .toBeGreaterThan(overlays)
  await expect(page.getByText('New data is available.')).toHaveCount(0)
})

test('REINDEX_IN_PROGRESS attaches to the running job without an error', async ({ page }) => {
  const backend = await openReview(page)
  backend.failNext(
    'codeIntel.reindex',
    'CODEINTEL_REINDEX_IN_PROGRESS: job running | {"jobId":"job-1"}'
  )
  await openIndexPopover(page)
  await page.getByRole('button', { name: 'Refresh index' }).click()
  await expect.poll(() => backend.callsTo('codeIntel.reindex').length).toBe(1)
  await expect(page.getByText(/CODEINTEL_REINDEX_IN_PROGRESS|job running/)).toHaveCount(0)
})

test('REINDEX_COOLDOWN shows when it is available again', async ({ page }) => {
  const backend = await openReview(page)
  backend.failNext(
    'codeIntel.reindex',
    'CODEINTEL_REINDEX_COOLDOWN: wait before reindexing | {"retryAfterSeconds":300}'
  )
  await openIndexPopover(page)
  await page.getByRole('button', { name: 'Refresh index' }).click()
  await expect(page.getByText(/Available again in \d+s/)).toBeVisible()
  await expect(page.getByRole('button', { name: 'Refresh index' })).toBeDisabled()
})

test('dropped stream: resync frame marks data stale and the stream is reopened', async ({
  page
}) => {
  const backend = await openReview(page)
  const subscribes = backend.streamCount()
  // Gateway order on a broken stream: a resync `changed` frame, then close (the fake's
  // dropStream() addresses its own selector, so the frame for the open worktree is sent first).
  backend.pushChanged(E2E_WORKTREE_ID, 'resync', true)
  backend.dropStream()
  await expect(page.getByText('New data is available.')).toBeVisible()
  // Why: the gateway closes the stream after resync; the client must resubscribe on its own.
  await expect.poll(() => backend.streamCount(), { timeout: 20_000 }).toBe(subscribes)
})
