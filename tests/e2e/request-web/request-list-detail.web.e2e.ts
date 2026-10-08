/**
 * E2E for the Requests page shell, list and detail (FE-REQ-TASK-018-05, 019-06, 019-07).
 * Runs against the web SPA with a mocked gateway WebSocket (no request-service needed).
 *
 *   MCP_E2E_BASE_URL=http://127.0.0.1:5174 npx playwright test -c tests/playwright.web.config.ts \
 *     --project=mcp-web tests/e2e/request-web
 */
import { expect, test } from '@playwright/test'
import {
  approvalView,
  bootRequestApp,
  createFakeRequestBackend,
  mockRequestApp,
  openRequestPageViaStore,
  openRequestsFromSidebar,
  requestView,
  RpcFailure
} from './support/mock-request-ws'

const first = requestView()
const second = requestView({ id: 'req-2', number: 2, title: 'Second request', status: 'analyzing' })

function listBackend() {
  return createFakeRequestBackend({
    'request.list': () => ({ requests: [first, second], nextPageToken: null }),
    'request.get': (p) => ({ request: p.id === 'req-2' ? second : first }),
    'request.confirmType': (p) => ({
      request: { ...first, status: 'analyzing', version: Number(p.expectedVersion ?? 3) + 1 }
    }),
    'approval.listPending': () => ({ approvals: [approvalView(), approvalView({ id: 'ap-2' })] })
  })
}

test.describe('Requests page (mocked request-service)', () => {
  test('a: openRequestPage() lands on the Requests tab with the request detail open', async ({
    page
  }) => {
    const backend = listBackend()
    await mockRequestApp(page, backend)
    await bootRequestApp(page)
    await openRequestPageViaStore(page, { section: 'requests', requestId: 'req-2' })
    await expect(page.getByRole('tab', { name: 'Requests' })).toHaveAttribute(
      'aria-selected',
      'true'
    )
    await expect(page.getByRole('heading', { level: 2, name: 'Second request' })).toBeVisible()
    expect(backend.callsTo('request.get')).toContainEqual({ id: 'req-2' })
  })

  test('b: list, detail and type confirmation call request.confirmType', async ({ page }) => {
    const backend = listBackend()
    await mockRequestApp(page, backend)
    await bootRequestApp(page)
    await openRequestsFromSidebar(page)
    const list = page.getByRole('listbox', { name: 'Requests' })
    await expect(list.getByRole('option')).toHaveCount(2)
    await list.getByRole('option', { name: /#1 Request/ }).click()
    const card = page.getByRole('region', { name: 'Confirm request type' })
    await expect(card).toBeVisible()
    await card.getByRole('button', { name: 'Confirm type' }).click()
    await expect.poll(() => backend.callsTo('request.confirmType').length).toBe(1)
    expect(backend.callsTo('request.confirmType')[0]).toMatchObject({
      id: 'req-1',
      type: 'change_request'
    })
  })

  test('c: runtime without request channels hides the sidebar entry, no page error', async ({
    page
  }) => {
    const backend = createFakeRequestBackend({
      'request.flowStatus': () => {
        throw new RpcFailure('method_not_found', 'request.flowStatus is not yet implemented')
      }
    })
    const errors: string[] = []
    page.on('pageerror', (e) => errors.push(e.message))
    await mockRequestApp(page, backend)
    await bootRequestApp(page)
    await expect.poll(() => backend.callsTo('request.flowStatus').length).toBeGreaterThan(0)
    await expect(page.getByRole('button', { name: 'Requests', exact: true })).toHaveCount(0)
    // Forcing the view still shows the notice instead of a red error.
    await openRequestPageViaStore(page, { section: 'requests' })
    await expect(page.getByRole('tablist')).toHaveCount(0)
    expect(errors).toEqual([])
  })

  test('d: j/k move the selected row', async ({ page }) => {
    const backend = listBackend()
    await mockRequestApp(page, backend)
    await bootRequestApp(page)
    await openRequestsFromSidebar(page)
    const list = page.getByRole('listbox', { name: 'Requests' })
    await expect(list.getByRole('option')).toHaveCount(2)
    await list.focus()
    await page.keyboard.press('j')
    await expect(list.getByRole('option', { name: /#1 Request/ })).toHaveAttribute(
      'aria-selected',
      'true'
    )
    await page.keyboard.press('j')
    await expect(list.getByRole('option', { name: /#2 Second request/ })).toHaveAttribute(
      'aria-selected',
      'true'
    )
    await page.keyboard.press('k')
    await expect(list.getByRole('option', { name: /#1 Request/ })).toHaveAttribute(
      'aria-selected',
      'true'
    )
  })

  test('shell: pending count on sidebar and tab, tab switch, Escape closes', async ({ page }) => {
    const backend = listBackend()
    await mockRequestApp(page, backend)
    await bootRequestApp(page)
    await expect(page.getByLabel('2 pending approvals')).toBeVisible()
    await openRequestsFromSidebar(page)
    await page.getByRole('tab', { name: /Approvals/ }).click()
    await expect(page.getByRole('tab', { name: /Approvals/ })).toHaveAttribute(
      'aria-selected',
      'true'
    )
    await page.getByRole('tab', { name: 'Backlog' }).click()
    await expect(page.getByRole('tab', { name: 'Backlog' })).toHaveAttribute(
      'aria-selected',
      'true'
    )
    await page
      .locator('body')
      .click({ position: { x: 5, y: 5 } })
      .catch(() => {})
    await page.keyboard.press('Escape')
    await expect(page.getByRole('heading', { level: 1, name: 'Requests' })).toHaveCount(0)
  })

  test('create: manual request sends no source and opens the new request', async ({ page }) => {
    const created = requestView({
      id: 'req-9',
      number: 9,
      title: 'Add dark mode',
      status: 'new',
      type: undefined
    })
    const backend = listBackend()
    backend.on('project.list', () => [{ id: 'proj-1', name: 'Orca', path: '/tmp/orca' }])
    backend.on('request.create', () => ({ request: created, created: true }))
    backend.on('request.get', (p) => ({ request: p.id === 'req-9' ? created : first }))
    await mockRequestApp(page, backend)
    await bootRequestApp(page)
    await openRequestsFromSidebar(page)
    await page.getByRole('button', { name: 'Create request' }).click()
    const dialog = page.getByRole('dialog')
    await dialog.getByLabel('Title').fill('Add dark mode')
    await dialog.getByLabel('Description').fill('Users want a dark theme')
    await dialog.getByRole('button', { name: 'Create', exact: true }).click()
    await expect.poll(() => backend.callsTo('request.create').length).toBe(1)
    const params = backend.callsTo('request.create')[0]
    expect(params).toMatchObject({
      projectId: 'proj-1',
      title: 'Add dark mode',
      body: 'Users want a dark theme'
    })
    expect(params).not.toHaveProperty('source')
    await expect(page.getByRole('heading', { level: 2, name: 'Add dark mode' })).toBeVisible()
  })
})
