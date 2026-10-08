/**
 * E2E for the Request graph entry points (FE-REQ-TASK-032-08), against the web SPA with a
 * mocked gateway WebSocket. `impact.*` channel names are provisional (CONTRACT section 7).
 *
 *   MCP_E2E_BASE_URL=http://127.0.0.1:5174 npx playwright test -c tests/playwright.web.config.ts \
 *     --project=mcp-web tests/e2e/request-web/request-graph.web.e2e.ts
 */
import { expect, test, type Page } from '@playwright/test'
import {
  bootRequestApp,
  createFakeRequestBackend,
  mockRequestApp,
  openRequestPageViaStore,
  requestView
} from './support/mock-request-ws'

const analyzing = requestView({
  id: 'req-g',
  number: 4,
  title: 'Graph request',
  status: 'analyzing'
})
const executing = requestView({
  id: 'req-p',
  number: 5,
  title: 'Planned request',
  status: 'executing',
  planTaskId: 'plan-1'
})

const task = (id: string, over: Record<string, unknown> = {}) => ({
  id,
  projectId: 'proj-1',
  title: id,
  status: 'todo',
  priority: 'medium',
  type: 'task',
  requestId: 'req-p',
  createdAt: '2026-10-08T09:00:00Z',
  updatedAt: '2026-10-08T09:00:00Z',
  ...over
})
const planTasks = [
  task('plan-1', { type: 'plan', title: 'The plan', status: 'in_progress' }),
  task('ph-1', { type: 'phase', parentId: 'plan-1', title: 'Phase One', status: 'in_progress' }),
  task('t-1', { parentId: 'ph-1', title: 'Write migration' }),
  task('t-2', { parentId: 'ph-1', title: 'Backfill data' })
]

function impactGraph(count: number, total: number) {
  const nodes = Array.from({ length: count }, (_, i) => ({
    id: `node-${String(i).padStart(3, '0')}`,
    label: `node-${String(i).padStart(3, '0')}`,
    kind: 'file',
    group: `svc-${i % 4}/mod`,
    risk: i % 10 === 0 ? 'high' : 'low'
  }))
  const edges = nodes
    .slice(1)
    .map((n, i) => ({ from: nodes[i].id, to: n.id, kind: 'affects', change: 'unchanged' }))
  return { nodes, edges, totalNodes: total, truncated: total > count, tool: 'codegraph' }
}

function graphBackend(request: Record<string, unknown>) {
  return createFakeRequestBackend({
    'request.list': () => ({ requests: [request], nextPageToken: null }),
    'request.get': () => ({ request }),
    'task.list': () => ({ tasks: planTasks, nextPageToken: '' }),
    'task.getDependencies': () => []
  })
}

async function openGraphSheet(page: Page): Promise<ReturnType<Page['getByTestId']>> {
  await page.getByRole('button', { name: 'View graph' }).click()
  const sheet = page.getByTestId('request-graph-sheet')
  await expect(sheet).toBeVisible()
  return sheet
}

test.describe('Request graph (mocked request-service)', () => {
  test('a: runtime without impact.* still opens "View graph"; backend lenses disabled; flow lens shown', async ({
    page
  }) => {
    const errors: string[] = []
    page.on('pageerror', (e) => errors.push(e.message))
    const backend = graphBackend(analyzing)
    await mockRequestApp(page, backend)
    await bootRequestApp(page)
    await openRequestPageViaStore(page, { section: 'requests', requestId: 'req-g' })
    await expect(page.getByRole('heading', { level: 2, name: 'Graph request' })).toBeVisible()

    const sheet = await openGraphSheet(page)
    await expect(sheet.getByRole('radio', { name: 'Flow' })).toHaveAttribute('aria-checked', 'true')
    for (const lens of ['Architecture', 'Contracts', 'Data', 'Impact']) {
      await expect(sheet.getByRole('radio', { name: lens })).toBeDisabled()
    }
    // xyflow canvas (lazy chunk) renders the client-built flow lens.
    await expect(sheet.getByTestId('graph-canvas')).toBeVisible()
    await expect(sheet.getByRole('button', { name: /Classification/ }).first()).toBeVisible()
    await sheet.getByRole('radio', { name: 'List' }).click()
    await expect(sheet.getByTestId('graph-list')).toContainText('Analysis')
    expect(errors).toEqual([])
  })

  test('b: impact.graph with 120 of 300 nodes defaults to the list; "/" opens search; Enter selects the node', async ({
    page
  }) => {
    const backend = graphBackend(executing)
    backend.on('impact.graph', () => impactGraph(120, 300))
    await mockRequestApp(page, backend)
    await bootRequestApp(page)
    await openRequestPageViaStore(page, { section: 'requests', requestId: 'req-p' })
    const sheet = await openGraphSheet(page)
    await sheet.getByRole('radio', { name: 'Impact' }).click()

    await expect(sheet.getByTestId('graph-list')).toBeVisible()
    await expect(sheet.getByText('Showing 120 of 300 nodes')).toBeVisible()
    expect(backend.callsTo('impact.graph').at(-1)).toMatchObject({
      requestId: 'req-p',
      subjectType: 'plan',
      subjectId: 'plan-1',
      lens: 'impact'
    })

    await sheet.getByTestId('graph-list').click({ position: { x: 5, y: 5 } })
    await page.keyboard.press('/')
    const search = page.getByPlaceholder(/Search/)
    await expect(search).toBeVisible()
    await search.fill('node-077')
    await page.keyboard.press('Enter')
    await expect(search).toHaveCount(0)
    await expect(sheet.getByRole('row', { name: /node-077/ })).toHaveAttribute(
      'aria-selected',
      'true'
    )
  })

  test('c: Plan tab Tree | Graph toggle switches between the tree and the plan lens', async ({
    page
  }) => {
    const backend = graphBackend(executing)
    await mockRequestApp(page, backend)
    await bootRequestApp(page)
    await openRequestPageViaStore(page, { section: 'requests', requestId: 'req-p', focus: 'plan' })
    await expect(page.getByTestId('phase-node-ph-1')).toBeVisible()

    const planView = page.getByRole('group', { name: 'Plan view' })
    await planView.getByRole('radio', { name: 'Graph' }).click()
    const panel = page.getByTestId('graph-panel')
    await expect(panel.getByRole('radio', { name: 'Plan' })).toHaveAttribute('aria-checked', 'true')
    await expect(panel.getByTestId('graph-canvas')).toBeVisible()
    await expect(panel.getByRole('button', { name: /Phase One/ }).first()).toBeVisible()
    await expect(page.getByTestId('phase-node-ph-1')).toHaveCount(0)

    await planView.getByRole('radio', { name: 'Tree' }).click()
    await expect(page.getByTestId('phase-node-ph-1')).toBeVisible()
    await expect(page.getByTestId('graph-panel')).toHaveCount(0)
  })
})
