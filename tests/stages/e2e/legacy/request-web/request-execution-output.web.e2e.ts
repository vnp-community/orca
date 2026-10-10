/**
 * E2E for the Result tab with `execution.get` (TASK-FE-E2E-012 F3) against the web SPA with a mocked gateway:
 * a Request task has no structured record, so the tab shows the tail of the last run as neutral "Agent output".
 *
 *   npx playwright test -c tests/configs/playwright.web.config.ts --project=request-web \
 *     tests/stages/e2e/legacy/request-web/request-execution-output.web.e2e.ts
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

const now = '2026-10-08T09:00:00Z'
const request = requestView({
  id: 'req-r',
  number: 9,
  title: 'Negate',
  status: 'executing',
  planTaskId: 'plan-1'
})
const task = (id: string, over: Record<string, unknown> = {}) => ({
  id,
  projectId: 'proj-1',
  title: id,
  status: 'done',
  priority: 'medium',
  type: 'task',
  requestId: 'req-r',
  createdAt: now,
  updatedAt: now,
  ...over
})
const tasks = [
  task('plan-1', { type: 'plan', title: 'The plan' }),
  task('ph-1', { type: 'phase', parentId: 'plan-1', title: 'Phase One' }),
  task('t-1', { parentId: 'ph-1', title: 'Negate the value' })
]

function backendWith(executionGet: () => unknown): FakeRequestBackend {
  return createFakeRequestBackend({
    'request.list': () => ({ requests: [request], nextPageToken: null }),
    'request.get': () => ({ request }),
    'task.list': () => ({ tasks, nextPageToken: '' }),
    'task.get': () => tasks[2],
    'task.getDependencies': () => [],
    'execution.get': executionGet
  })
}

async function openSheet(page: Page, backend: FakeRequestBackend): Promise<void> {
  await mockRequestApp(page, backend)
  await bootRequestApp(page)
  await openRequestPageViaStore(page, { section: 'requests', requestId: 'req-r', focus: 'plan' })
  await page.getByTestId('plan-task-row-t-1').click()
  await expect(page.getByTestId('task-agent-activity')).toBeVisible()
}

test.describe('Result tab from execution.get (mocked gateway)', () => {
  test('a: the last output of a Request task is shown as neutral Agent output, masked, with the gateway notes', async ({
    page
  }) => {
    const secret = 'sk-ant-abcdefghijklmnopqrstuvwxyz0123456789'
    const backend = backendWith(() => ({
      taskId: 't-1',
      parseStatus: 'missing',
      filesChanged: [],
      checksRun: [],
      stdoutTail: `Ran 3 tests in 0.001s\nOK\nkey ${secret}\n<img src=x onerror="window.__pwned=1">`,
      source: 'last_execution_output',
      redacted: true,
      truncated: true
    }))
    await openSheet(page, backend)
    await page.getByRole('tab', { name: 'Result' }).click()

    const view = page.getByTestId('execution-agent-output')
    await expect(view).toBeVisible()
    await expect(view).toContainText('Agent output')
    await expect(view).toContainText('Secrets were hidden.')
    await expect(view).toContainText('Only the last part is shown.')
    const text = page.getByTestId('execution-agent-output-text')
    await expect(text).toContainText('Ran 3 tests in 0.001s')
    await expect(text).not.toContainText('sk-ant')
    await expect(text).toContainText('<img src=x onerror="window.__pwned=1">')
    expect(
      await page.evaluate(() => (window as unknown as { __pwned?: number }).__pwned)
    ).toBeUndefined()
    await expect(page.getByTestId('execution-malformed')).toHaveCount(0)
    await expect(
      page.getByTestId('execution-result-panel').or(view).locator('.text-destructive')
    ).toHaveCount(0)
    expect(backend.callsTo('execution.get')[0]).toMatchObject({ taskId: 't-1', latestOnly: true })
  })

  test('b: a contract run keeps the structured result view', async ({ page }) => {
    const backend = backendWith(() => ({
      taskId: 't-1',
      attempt: 2,
      parseStatus: 'ok',
      status: 'done',
      summary: 'Negated the value',
      filesChanged: ['src/calc.py'],
      checksRun: [{ id: 'unit', exit: 0 }],
      source: 'execution_record'
    }))
    await openSheet(page, backend)
    await page.getByRole('tab', { name: 'Result' }).click()
    const panel = page.getByTestId('execution-result-panel')
    await expect(panel).toContainText('Attempt 2')
    await expect(panel).toContainText('src/calc.py')
    await expect(page.getByTestId('execution-agent-output')).toHaveCount(0)
  })

  test('c: no Result tab when the task has nothing to show (TASK_NOT_FOUND) or the gateway lacks the channel', async ({
    page
  }) => {
    const backend = backendWith(() => {
      throw new RpcFailure('TASK_NOT_FOUND', 'TASK_NOT_FOUND: nothing yet')
    })
    await openSheet(page, backend)
    await expect.poll(() => backend.callsTo('execution.get').length).toBeGreaterThan(0)
    await expect(page.getByRole('tab', { name: 'Result' })).toHaveCount(0)
    await expect(page.getByRole('tab', { name: 'Details' })).toBeVisible()
  })

  test('d: a gateway without execution.get shows no Result tab either', async ({ page }) => {
    const backend = backendWith(() => ({}))
    backend.on('execution.get', () => {
      throw new RpcFailure('method_not_found', 'execution.get is not yet implemented')
    })
    await openSheet(page, backend)
    await expect.poll(() => backend.callsTo('execution.get').length).toBeGreaterThan(0)
    await expect(page.getByRole('tab', { name: 'Result' })).toHaveCount(0)
  })
})
