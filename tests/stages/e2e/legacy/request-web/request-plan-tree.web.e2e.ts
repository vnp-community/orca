/**
 * E2E for the Request Plan tab (FE-REQ-TASK-021-06), web SPA + mocked gateway WebSocket.
 * The tree is read from task.list (CONTRACT 2.1: no plan.* channels); gates from approval.*.
 * (b) Board toggle lives in Project Workspace and is covered by
 * frontend/src/renderer/src/components/task/__tests__/TaskGraph-planning-toggle.test.tsx.
 */
import { expect, test, type Page } from '@playwright/test'
import {
  approvalView,
  bootRequestApp,
  createFakeRequestBackend,
  mockRequestApp,
  openRequestPageViaStore,
  requestView,
  RpcFailure,
  type FakeRequestBackend
} from './support/mock-request-ws'

const at = '2026-10-08T09:00:00Z'
const task = (id: string, o: Record<string, unknown>) => ({
  id,
  projectId: 'proj-1',
  title: id,
  type: 'task',
  status: 'todo',
  priority: 'medium',
  labels: [],
  visibility: 'private',
  progressPercent: 0,
  createdAt: at,
  updatedAt: at,
  ...o
})

const tasks = [
  task('plan-1', { type: 'plan', title: 'Parser plan' }),
  task('ph-1', { type: 'phase', parentId: 'plan-1', title: 'Phase 1: Prepare' }),
  task('ph-2', { type: 'phase', parentId: 'plan-1', title: 'Phase 2: Ship' }),
  task('a1', { parentId: 'ph-1', title: 'Write tests', status: 'done' }),
  task('a2', { parentId: 'ph-1', title: 'Refactor lexer', status: 'in_progress' }),
  task('b1', { parentId: 'ph-2', title: 'Release' })
]

function planBackend(): FakeRequestBackend {
  const approvals = new Map<string, Record<string, unknown>>([
    ['ap-plan', approvalView({ id: 'ap-plan', subjectType: 'plan', subjectId: 'plan-1' })],
    ['ap-ph1', approvalView({ id: 'ap-ph1', subjectType: 'phase', subjectId: 'ph-1' })]
  ])
  const list = () => ({ approvals: [...approvals.values()] })
  const request = requestView({ status: 'awaiting_plan_approval', planTaskId: 'plan-1' })
  return createFakeRequestBackend({
    'request.list': () => ({ requests: [request] }),
    'request.get': () => ({ request }),
    'task.list': () => ({ tasks }),
    'approval.list': list,
    'approval.listPending': () => ({
      approvals: [...approvals.values()].filter((a) => a.status === 'pending')
    }),
    'approval.approve': (p) => {
      const next = { ...approvals.get(String(p.id)), status: 'approved', version: 2 }
      approvals.set(String(p.id), next)
      return { approval: next, requestStatus: 'executing' }
    },
    'request.startPhase': (p) => ({
      phaseTaskId: p.phaseTaskId,
      alreadyStarted: false,
      dispatchedTaskIds: []
    })
  })
}

async function openPlan(page: Page, backend: FakeRequestBackend): Promise<void> {
  await mockRequestApp(page, backend)
  await bootRequestApp(page)
  await openRequestPageViaStore(page, { section: 'requests', requestId: 'req-1' })
  await page.getByRole('tab', { name: 'Plan' }).click()
}

test.describe('Request plan tree (mocked request-service)', () => {
  test('a: planning change_request shows Plan > 2 Phases in order with progress', async ({
    page
  }) => {
    await openPlan(page, planBackend())
    const panel = page.getByRole('tabpanel', { name: 'Plan' })
    await expect(panel.getByRole('heading', { name: 'Parser plan' })).toBeVisible()
    await expect(panel.getByText('2 phases, 3 tasks')).toBeVisible()
    const phases = panel.getByRole('button', { name: /^Phase \d/ })
    await expect(phases).toHaveText(['Phase 1: Prepare', 'Phase 2: Ship'])
    await expect(panel.getByText(/1\/2 tasks done/)).toBeVisible()
    await expect(panel.getByRole('progressbar', { name: 'Plan progress' })).toBeVisible()
  })

  test.skip('b: Board hides Plan/Phase by default; the toggle shows them; children stay at root', async () => {
    // Why skipped: the Board is inside Project Workspace, which needs project.get + file-tree
    // channels this mock does not model; TaskGraph-planning-toggle.test.tsx covers the behaviour.
  })

  test('c: approving the plan calls approval.approve; a short reject reason stays locked', async ({
    page
  }) => {
    const backend = planBackend()
    await openPlan(page, backend)
    const panel = page.getByRole('tabpanel', { name: 'Plan' })
    await panel.getByRole('button', { name: 'Reject' }).first().click()
    const dialog = page.getByRole('dialog')
    await dialog.getByRole('textbox').fill('too short')
    await expect(dialog.getByRole('button', { name: 'Reject' })).toBeDisabled()
    await dialog.getByRole('button', { name: 'Cancel' }).click()
    expect(backend.callsTo('approval.reject')).toHaveLength(0)

    await panel.getByRole('button', { name: 'Approve plan' }).click()
    await expect.poll(() => backend.callsTo('approval.approve').length).toBe(1)
    expect(backend.callsTo('approval.approve')[0]).toMatchObject({
      id: 'ap-plan',
      expectedVersion: 1,
      expectedDigest: 'digest-1'
    })
  })

  test('d: unapproved phase locks Start; approving then Start phase calls request.startPhase', async ({
    page
  }) => {
    const backend = planBackend()
    await openPlan(page, backend)
    const bar = page.getByTestId('phase-approval-bar-ph-1')
    const start = bar.getByRole('button', { name: 'Start phase' })
    await expect(start).toBeDisabled()
    await bar.getByRole('button', { name: 'Approve phase' }).click()
    await expect.poll(() => backend.callsTo('approval.approve').length).toBe(1)
    await expect(start).toBeEnabled()
    await start.click()
    await expect.poll(() => backend.callsTo('request.startPhase').length).toBe(1)
    // CONTRACT 2.1: {id, phaseTaskId}; never the old `phaseId`.
    expect(backend.callsTo('request.startPhase')[0]).toEqual({ id: 'req-1', phaseTaskId: 'ph-1' })
  })

  test('e: runtime without the task channel shows a notice, no red error', async ({ page }) => {
    const backend = planBackend()
    backend.on('task.list', () => {
      throw new RpcFailure('method_not_found', 'task.list is not yet implemented')
    })
    const errors: string[] = []
    page.on('pageerror', (e) => errors.push(e.message))
    await openPlan(page, backend)
    const panel = page.getByRole('tabpanel', { name: 'Plan' })
    await expect.poll(() => backend.callsTo('task.list').length).toBeGreaterThan(0)
    await expect(panel.getByRole('heading', { name: 'Parser plan' })).toHaveCount(0)
    await expect(panel.locator('.text-destructive')).toHaveCount(0)
    await expect(panel).not.toBeEmpty()
    expect(errors).toEqual([])
  })
})
