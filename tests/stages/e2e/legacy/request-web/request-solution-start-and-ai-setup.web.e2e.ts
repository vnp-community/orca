/**
 * E2E for TASK-FE-E2E-009 (and the mock coverage TASK-FE-E2E-002/005/007 left open), web SPA +
 * mocked gateway WebSocket: the explicit "Generate solution" action, the AI-setup banner,
 * the blocked Clarification notice and the Plan tab's task.list filters.
 */
import { expect, test, type Page } from '@playwright/test'
import {
  bootRequestApp,
  createFakeRequestBackend,
  mockRequestApp,
  openRequestPageViaStore,
  requestView,
  RpcFailure,
  type FakeRequestBackend
} from './support/mock-request-ws'

const at = '2026-10-08T09:00:00Z'
const runningRun = { id: 'run-1', kind: 'solution', status: 'running', startedAt: at }

async function open(page: Page, backend: FakeRequestBackend, tab?: string): Promise<void> {
  await mockRequestApp(page, backend)
  await bootRequestApp(page)
  await openRequestPageViaStore(page, { section: 'requests', requestId: 'req-1' })
  if (tab) {
    await page.getByRole('tab', { name: tab }).click()
  }
}

function analyzingBackend(initialRuns: unknown[] = []): {
  backend: FakeRequestBackend
  runs: unknown[]
} {
  const state = { runs: initialRuns }
  const request = requestView({ status: 'analyzing' })
  const backend = createFakeRequestBackend({
    'request.list': () => ({ requests: [request] }),
    'request.get': () => ({ request }),
    'solution.list': () => ({ solutions: [], runs: state.runs }),
    'solution.generate': () => {
      state.runs = [runningRun]
      return { solutionId: 'sol-1', runId: 'run-1' }
    }
  })
  return { backend, runs: state.runs }
}

test.describe('Generate solution (mocked request-service)', () => {
  test('a: analyzing with no run offers the action; one click starts exactly one run', async ({
    page
  }) => {
    const { backend } = analyzingBackend()
    await open(page, backend, 'Analysis')
    const button = page.getByTestId('solution-generate')
    await expect(button).toBeEnabled()
    await expect(page.getByTestId('solution-start-prompt')).toContainText('No analysis has started')
    await button.dblclick()
    await expect(page.getByTestId('solution-generation-pending')).toBeVisible()
    await expect(button).toHaveCount(0)
    expect(backend.callsTo('solution.generate')).toHaveLength(1)
    expect(backend.callsTo('solution.generate')[0]).toMatchObject({ requestId: 'req-1' })
    expect(backend.callsTo('solution.generate')[0].idempotencyKey).toBeTruthy()
  })

  test('b: no action while a run is already running', async ({ page }) => {
    const { backend } = analyzingBackend([runningRun])
    await open(page, backend, 'Analysis')
    await expect(page.getByTestId('solution-generation-pending')).toBeVisible()
    await expect(page.getByTestId('solution-generate')).toHaveCount(0)
    expect(backend.callsTo('solution.generate')).toHaveLength(0)
  })

  test('c: a run the backend started meanwhile is not duplicated', async ({ page }) => {
    const { backend } = analyzingBackend()
    await open(page, backend, 'Analysis')
    const button = page.getByTestId('solution-generate')
    await expect(button).toBeEnabled()
    // The backend resumed on its own after the page last listed the runs.
    backend.on('solution.list', () => ({ solutions: [], runs: [runningRun] }))
    await button.click()
    await expect(page.getByTestId('solution-generation-pending')).toBeVisible()
    expect(backend.callsTo('solution.generate')).toHaveLength(0)
  })

  test('d: a failed start shows the coded error instead of staying silent', async ({ page }) => {
    const { backend } = analyzingBackend()
    backend.on('solution.generate', () => {
      throw new RpcFailure(
        'runtime_error',
        'REQUEST_SOLUTION_WRONG_STATE: request is not analyzing'
      )
    })
    await open(page, backend, 'Analysis')
    await page.getByTestId('solution-generate').click()
    await expect(page.getByTestId('solution-generation-error-code')).toContainText(
      'REQUEST_SOLUTION_WRONG_STATE'
    )
  })
})

test.describe('AI setup banner (mocked request-service)', () => {
  test('e: a failed Solution run for lack of AI names the fix', async ({ page }) => {
    const { backend } = analyzingBackend([
      {
        id: 'run-1',
        kind: 'solution',
        status: 'failed',
        errorCode: 'REQUEST_AI_NOT_CONFIGURED',
        startedAt: at
      }
    ])
    await open(page, backend, 'Analysis')
    const banner = page.getByTestId('request-ai-setup-banner')
    await expect(banner).toHaveAttribute('data-ai-code', 'REQUEST_AI_NOT_CONFIGURED')
    await expect(banner.locator('code')).toContainText('claude auth login')
    await expect(page.getByTestId('solution-generate')).toHaveCount(0)
  })

  test('f: a classification that failed for lack of AI shows the banner on the type card', async ({
    page
  }) => {
    const request = requestView({
      status: 'awaiting_type_confirmation',
      type: 'unknown',
      confidence: 0,
      classificationReason: 'REQUEST_AI_NOT_CONFIGURED'
    })
    const backend = createFakeRequestBackend({
      'request.list': () => ({ requests: [request] }),
      'request.get': () => ({ request })
    })
    await open(page, backend)
    await expect(page.getByTestId('request-ai-setup-banner')).toHaveAttribute(
      'data-ai-code',
      'REQUEST_AI_NOT_CONFIGURED'
    )
  })
})

test.describe('Clarification blocked (mocked request-service)', () => {
  test('g: a runtime without clarification.list explains the dead end', async ({ page }) => {
    const request = requestView({ status: 'awaiting_information' })
    const backend = createFakeRequestBackend({
      'request.list': () => ({ requests: [request] }),
      'request.get': () => ({ request })
    })
    await open(page, backend)
    await expect(page.getByTestId('clarification-load-failed')).toBeVisible()
    await expect(page.getByTestId('clarification-load-failed-code')).toContainText('Error code')
  })
})

test.describe('Plan tab task.list filters (mocked request-service)', () => {
  const task = (id: string, o: Record<string, unknown>) => ({
    id,
    projectId: 'proj-1',
    requestId: 'req-1',
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
  const all = [
    task('plan-1', { type: 'plan', title: 'Parser plan' }),
    task('ph-1', { type: 'phase', parentId: 'plan-1', title: 'Phase 1: Prepare' }),
    task('a1', { parentId: 'ph-1', title: 'Write tests' }),
    task('free', { requestId: undefined, title: 'Unrelated board task' })
  ]

  test('h: the tab names the request and the plan node types, like task-service requires', async ({
    page
  }) => {
    const request = requestView({ status: 'awaiting_plan_approval', planTaskId: 'plan-1' })
    const backend = createFakeRequestBackend({
      'request.list': () => ({ requests: [request] }),
      'request.get': () => ({ request }),
      // Mirrors task-service: plan and phase are hidden unless the caller names them.
      'task.list': (p) => {
        const types = (p.taskTypes as string[] | undefined) ?? ['task']
        return { tasks: all.filter((t) => types.includes(t.type as string)), nextPageToken: '' }
      }
    })
    await open(page, backend, 'Plan')
    const panel = page.getByRole('tabpanel', { name: 'Plan' })
    await expect(panel.getByRole('heading', { name: 'Parser plan' })).toBeVisible()
    await expect(panel.getByText('Plan not found.')).toHaveCount(0)
    expect(backend.callsTo('task.list')[0]).toMatchObject({
      projectId: 'proj-1',
      requestId: 'req-1',
      taskTypes: ['plan', 'phase', 'task', 'bug', 'feature', 'epic']
    })
  })
})
