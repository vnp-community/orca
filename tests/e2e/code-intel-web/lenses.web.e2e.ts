import { expect, test, type Page } from '@playwright/test'
import { SYMBOL_REF_SERVICE } from '../../../frontend/src/renderer/src/test-support/code-intel-fixtures'
import { bootIntoReview } from './support/code-intel-app-navigation'
import { devStackUrl } from './support/code-intel-dev-backend'
import { DSN_CANARY_SECRET, seedReviewLenses } from './support/code-intel-lens-fixtures'
import { E2E_WORKTREE_ID } from './support/code-intel-workspace-stubs'
import {
  createFakeCodeIntelBackend,
  mockCodeIntelApp,
  mockUser,
  type FakeCodeIntelBackend
} from './support/mock-code-intel-ws'

test.skip(!!devStackUrl, 'mocked variants only; the real stack runs dev-stack.web.e2e.ts')

// Review lenses on golden fixtures: Impact (053), Architecture (055), Flows (056), ERD (057),
// Storage (058), Contracts + Findings (059). Assertions are on the DOM only.

async function openLens(
  page: Page,
  lens: string,
  seed?: (backend: FakeCodeIntelBackend) => void
): Promise<FakeCodeIntelBackend> {
  const backend = createFakeCodeIntelBackend()
  seedReviewLenses(backend)
  seed?.(backend)
  await mockCodeIntelApp(page, { backend, user: mockUser(), workspace: true })
  await bootIntoReview(page, backend)
  await page.getByRole('tab', { name: lens, exact: true }).click()
  return backend
}

const rawKeys = (page: Page) => page.getByText(/auto\.components\.reviewMap\./)

test('Impact: a selected changed symbol shows the symbols it affects', async ({ page }) => {
  const backend = await openLens(page, 'Impact')
  await expect(page.getByText('Select a changed symbol to see what it affects.')).toBeVisible()
  // Setup through the store (tests/e2e/AGENTS.md): the pick normally comes from a graph node.
  await page.evaluate(
    ([wt, key]) =>
      (
        window as unknown as {
          __store: { getState: () => { selectReviewSymbol: (w: string, k: string) => void } }
        }
      ).__store
        .getState()
        .selectReviewSymbol(wt, key),
    [E2E_WORKTREE_ID, SYMBOL_REF_SERVICE.key]
  )
  await expect.poll(() => backend.callsTo('codeIntel.impact').length).toBeGreaterThan(0)
  await expect(page.getByText('TestCreate').first()).toBeVisible()
  await expect(page.getByText('Select a changed symbol to see what it affects.')).toHaveCount(0)
  await expect(rawKeys(page)).toHaveCount(0)
})

test('Architecture: container view with the inferred-diagram notice', async ({ page }) => {
  const backend = await openLens(page, 'Architecture')
  await expect(page.getByText('usecase', { exact: true }).first()).toBeVisible()
  await expect(page.getByText(/inferred from the folder structure/)).toBeVisible()
  await expect(page.getByText('12 symbols').first()).toBeVisible()
  expect(backend.callsTo('codeIntel.architecture').length).toBeGreaterThan(0)
  await expect(rawKeys(page)).toHaveCount(0)
})

test('Flows: picking a flow loads its steps', async ({ page }) => {
  const backend = await openLens(page, 'Flows')
  await page.getByRole('combobox', { name: 'Flows' }).click()
  await page.getByRole('option', { name: /Create order/ }).click()
  await expect.poll(() => backend.callsTo('codeIntel.dataFlow').length).toBeGreaterThan(0)
  await expect(page.getByText('Pick a flow to see its sequence.')).toHaveCount(0)
  await expect(page.getByText('usecase').first()).toBeVisible()
  await expect(rawKeys(page)).toHaveCount(0)
})

test('ERD: tables with column change marks; the ghost table switches service', async ({ page }) => {
  const backend = await openLens(page, 'ERD')
  await expect(
    page.getByRole('group', { name: /^Table dev_servers, \d+ columns, Changed/ })
  ).toBeVisible()
  await expect(page.getByText('varchar(32) → text')).toBeVisible()
  await expect(page.getByText('old_flag')).toBeVisible()
  // Why: defaultExpr carries a password-looking literal; it must be masked, never shown raw.
  await expect(page.getByText(/hunter2/)).toHaveCount(0)
  const services = backend.callsTo('codeIntel.erd').length
  // Why: xyflow re-measures nodes continuously, so dispatch the click instead of waiting for stable.
  await page.getByRole('button', { name: /Owned by auth/ }).dispatchEvent('click')
  await expect
    .poll(() =>
      backend
        .callsTo('codeIntel.erd')
        .slice(services)
        .map((c) => (c.params as { service?: string }).service)
    )
    .toContain('auth')
  await expect(rawKeys(page)).toHaveCount(0)
})

test('Storage: secrets never shown, DSN canary absent, node detail links to the ERD', async ({
  page
}) => {
  await openLens(page, 'Storage')
  await expect(page.getByText('value is never shown')).toBeVisible()
  await expect(page.getByText(new RegExp(DSN_CANARY_SECRET))).toHaveCount(0)
  expect(await page.content()).not.toContain(DSN_CANARY_SECRET)
  await page.getByRole('radio', { name: 'prod' }).click()
  await expect(page.getByText('prod topology unknown')).toBeVisible()
  await page.getByRole('radio', { name: 'dev' }).click()
  // Why: xyflow keeps re-measuring nodes, so Playwright never sees the node as "stable".
  await page
    .locator('.react-flow__node')
    .filter({ has: page.getByRole('group', { name: 'postgres', exact: true }) })
    .dispatchEvent('click')
  const openErd = page.getByRole('button', { name: 'Open ERD of infra-fleet' })
  await expect(openErd).toBeVisible()
  await openErd.click()
  await expect(page.getByRole('tab', { name: 'ERD', exact: true })).toHaveAttribute(
    'aria-selected',
    'true'
  )
})

test('Storage: CODEINTEL_UNAVAILABLE hides the lens instead of an error', async ({ page }) => {
  const backend = createFakeCodeIntelBackend()
  seedReviewLenses(backend)
  backend.setHandler('codeIntel.storage', () => {
    throw new Error('CODEINTEL_UNAVAILABLE: storage map not deployed')
  })
  await mockCodeIntelApp(page, { backend, user: mockUser(), workspace: true })
  await bootIntoReview(page, backend)
  await expect.poll(() => backend.callsTo('codeIntel.storage').length).toBeGreaterThan(0)
  await expect(page.getByRole('tab', { name: 'Storage', exact: true })).toHaveCount(0)
  await expect(page.getByRole('tab', { name: 'ERD', exact: true })).toBeVisible()
})

test('Contracts: compatibility table, unknown is "Not classified", DSN masked', async ({
  page
}) => {
  await openLens(page, 'Contracts')
  await expect(page.getByText('RelayByDevServer.timeout_ms')).toBeVisible()
  await expect(page.getByText('2 Breaking')).toBeVisible()
  await expect(page.getByText('Not classified').first()).toBeVisible()
  await expect(page.getByText('note: dsn=•••')).toBeVisible()
  await expect(rawKeys(page)).toHaveCount(0)
})

test('Contracts: INDEX_MISSING explains the missing index with a retry', async ({ page }) => {
  await openLens(page, 'Contracts', (b) =>
    b.setHandler('codeIntel.contractDiff', () => {
      throw new Error('CODEINTEL_INDEX_MISSING: no index')
    })
  )
  await expect(page.getByText(/code index is not available/)).toBeVisible()
  await expect(page.getByRole('tabpanel').getByRole('button', { name: 'Retry' })).toBeVisible()
})

test('Findings dock lists findings with their evidence paths', async ({ page }) => {
  const backend = await openLens(page, 'Impact')
  await page.getByRole('button', { name: 'Findings', exact: true }).click()
  await expect.poll(() => backend.callsTo('codeIntel.findings').length).toBeGreaterThan(0)
  const region = page.getByRole('region', { name: 'Review panels' })
  await expect(region.getByText('svc/handlers/order.go').first()).toBeVisible()
  await expect(rawKeys(page)).toHaveCount(0)
})
