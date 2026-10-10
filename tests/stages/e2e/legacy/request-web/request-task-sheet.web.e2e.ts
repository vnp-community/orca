/**
 * E2E for the task sheet opened from a Request's Plan tab (TASK-FE-E2E-013), web SPA + mocked
 * gateway WebSocket: "Open in Git" lands on the worktree's Source Control, the sheet fits narrow and
 * wide layouts, Escape closes only the sheet, and the agent panel points to the saved Result tab.
 *
 *   npx playwright test -c tests/configs/playwright.web.config.ts --project=request-web \
 *     tests/stages/e2e/legacy/request-web/request-task-sheet.web.e2e.ts
 */
import { expect, test, type Page } from '@playwright/test'
import {
  E2E_WORKTREE_ID,
  WORKSPACE_CHANNEL_STUBS
} from '../code-intel-web/support/code-intel-workspace-stubs'
import {
  bootRequestApp,
  createFakeRequestBackend,
  mockRequestApp,
  openRequestPageViaStore,
  requestView,
  type FakeRequestBackend
} from './support/mock-request-ws'

const now = '2026-10-08T09:00:00Z'
const LONG_TITLE = 'Negate the value returned by calculate() and update its unit tests accordingly'
const executing = requestView({
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
  status: 'todo',
  priority: 'medium',
  type: 'task',
  requestId: 'req-r',
  createdAt: now,
  updatedAt: now,
  ...over
})

type Options = {
  worktreeId?: string
  /** Worktrees `worktree.list` returns (bookkept rows); the worktree stubs below list the one on disk. */
  listed?: unknown[]
  onDisk?: unknown[]
  executionOutput?: string
}

function fixture(opts: Options = {}): FakeRequestBackend {
  const worktreeId = 'worktreeId' in opts ? opts.worktreeId : E2E_WORKTREE_ID
  const workspace = WORKSPACE_CHANNEL_STUBS as Record<
    string,
    { worktrees?: unknown[] } & Record<string, unknown>
  >
  const tasks = [
    task('plan-1', { type: 'plan', title: 'The plan' }),
    task('ph-1', { type: 'phase', parentId: 'plan-1', title: 'Phase One' }),
    task('t-1', { parentId: 'ph-1', title: LONG_TITLE, status: 'review', worktreeId })
  ]
  return createFakeRequestBackend({
    'request.list': () => ({ requests: [executing], nextPageToken: null }),
    'request.get': () => ({ request: executing }),
    'task.list': () => ({ tasks, nextPageToken: '' }),
    'task.get': () => tasks[2],
    'task.getDependencies': () => [],
    'project.list': () => WORKSPACE_CHANNEL_STUBS['project.list'],
    'repo.list': () => WORKSPACE_CHANNEL_STUBS['repo.list'],
    'git.status': () => WORKSPACE_CHANNEL_STUBS['git.status'],
    'git.branchCompare': () => WORKSPACE_CHANNEL_STUBS['git.branchCompare'],
    'worktree.list': () => ({ worktrees: opts.listed ?? workspace['worktree.list'].worktrees }),
    'worktree.detectedList': () => ({
      ...workspace['worktree.detectedList'],
      worktrees: opts.onDisk ?? workspace['worktree.detectedList'].worktrees
    }),
    ...(opts.executionOutput !== undefined
      ? {
          'execution.get': () => ({
            taskId: 't-1',
            attempt: 1,
            parseStatus: 'missing',
            filesChanged: [],
            checksRun: [],
            source: 'last_execution_output',
            stdoutTail: opts.executionOutput
          })
        }
      : {})
  })
}

async function openTaskSheet(page: Page, backend: FakeRequestBackend): Promise<void> {
  await mockRequestApp(page, backend)
  await bootRequestApp(page)
  await openRequestPageViaStore(page, { section: 'requests', requestId: 'req-r', focus: 'plan' })
  await page.getByTestId('plan-task-row-t-1').click()
  await expect(page.getByTestId('task-detail')).toBeVisible()
}

const store = <T>(page: Page, read: string): Promise<T> =>
  page.evaluate(
    (src) =>
      new Function('s', `return (${src})`)(
        (window as unknown as { __store: { getState: () => unknown } }).__store.getState()
      ),
    read
  ) as Promise<T>

test.describe('Task sheet of a Request (mocked gateway)', () => {
  test('a: "Open in Git" leaves the Requests page for the task worktree and shows its Source Control', async ({
    page
  }) => {
    const backend = fixture()
    await openTaskSheet(page, backend)
    await page.getByTestId('open-in-git-btn').click()

    await expect.poll(() => store<string>(page, 's.activeView')).toBe('terminal')
    expect(await store<string | null>(page, 's.activeWorktreeId')).toBe(E2E_WORKTREE_ID)
    expect(await store<string>(page, 's.rightSidebarTab')).toBe('source-control')
    expect(await store<boolean>(page, 's.rightSidebarOpen')).toBe(true)
    await expect(page.getByTestId('task-detail')).toHaveCount(0)
    // The worktree was already loaded by the sidebar, so no lookup is needed.
    expect(backend.callsTo('worktree.list')).toEqual([])
  })

  test('a2: a worktree the sidebar has not loaded yet is looked up and loaded first', async ({
    page
  }) => {
    const backend = fixture({ onDisk: [] })
    await openTaskSheet(page, backend)
    expect(
      await store<boolean>(page, `!s.getKnownWorktreeById(${JSON.stringify(E2E_WORKTREE_ID)})`)
    ).toBe(true)
    // The host reports the checkout from now on (e.g. it was created while the page was open).
    const onDisk = WORKSPACE_CHANNEL_STUBS['worktree.detectedList']
    backend.on('worktree.detectedList', () => onDisk)
    await page.getByTestId('open-in-git-btn').click()

    await expect.poll(() => store<string>(page, 's.activeView')).toBe('terminal')
    expect(await store<string | null>(page, 's.activeWorktreeId')).toBe(E2E_WORKTREE_ID)
    expect(await store<string>(page, 's.rightSidebarTab')).toBe('source-control')
    expect(backend.callsTo('worktree.list')).toEqual([{ projectId: 'proj-1' }])
  })

  test('b: a worktree that is not on disk any more says so instead of staying silent', async ({
    page
  }) => {
    const backend = fixture({ onDisk: [] })
    await openTaskSheet(page, backend)
    await page.getByTestId('open-in-git-btn').click()

    await expect(page.getByText("This task's worktree is no longer on this host")).toBeVisible()
    expect(await store<string>(page, 's.activeView')).toBe('requests')
    await expect(page.getByTestId('task-detail')).toBeVisible()
    await expect(page.getByTestId('open-in-git-btn')).toBeEnabled()
  })

  test('c: a worktree id the host does not know says so', async ({ page }) => {
    const backend = fixture({ listed: [], onDisk: [] })
    await openTaskSheet(page, backend)
    await page.getByTestId('open-in-git-btn').click()
    await expect(page.getByText("This task's worktree no longer exists")).toBeVisible()
    expect(await store<string>(page, 's.activeView')).toBe('requests')
  })

  test('d: Escape closes only the sheet; the Request stays selected on the Requests page', async ({
    page
  }) => {
    const backend = fixture()
    await openTaskSheet(page, backend)
    await page.keyboard.press('Escape')

    await expect(page.getByRole('dialog')).toHaveCount(0)
    expect(await store<string>(page, 's.activeView')).toBe('requests')
    expect(await store<string | null>(page, 's.requestPage.requestId')).toBe('req-r')
    await expect(page.getByTestId('plan-task-row-t-1')).toBeVisible()

    // With nothing on top of it, Escape still leaves the Requests page.
    await page.keyboard.press('Escape')
    await expect.poll(() => store<string>(page, 's.activeView')).toBe('terminal')
  })

  test('f: after a run the idle agent panel points to the saved output in the Result tab', async ({
    page
  }) => {
    const output = `▸ Edit src/calc.py\n${'a very long line of agent output that has to wrap instead of running off the sheet '.repeat(6)}\nAll 4 tests pass`
    const backend = fixture({ executionOutput: output })
    await openTaskSheet(page, backend)
    const panel = page.getByTestId('task-agent-activity')
    await expect(panel).toHaveAttribute('data-state', 'idle')
    await expect(panel.getByTestId('task-agent-activity-empty')).toContainText('Result tab')

    await panel.getByTestId('task-agent-activity-open-result').click()
    const text = page.getByTestId('execution-agent-output-text')
    await expect(text).toContainText('▸ Edit src/calc.py')
    await expect(text).toContainText('All 4 tests pass')
    // Long lines wrap inside the block: it never scrolls sideways.
    expect(await text.evaluate((el) => el.scrollWidth - el.clientWidth)).toBeLessThanOrEqual(1)
  })

  test('g: with no saved output the idle agent panel has no pointer', async ({ page }) => {
    const backend = fixture()
    await openTaskSheet(page, backend)
    await expect(page.getByTestId('task-agent-activity-empty')).toContainText('Nothing to show yet')
    await expect(page.getByTestId('task-agent-activity-open-result')).toHaveCount(0)
    await expect(page.getByRole('tab', { name: 'Result' })).toHaveCount(0)
  })

  for (const [name, width] of [
    ['wide', 1440],
    ['narrow', 700],
    ['phone', 390]
  ] as const) {
    test(`e: the sheet fits a ${name} window (${width}px): nothing overflows, no button is cut`, async ({
      page
    }) => {
      await page.setViewportSize({ width, height: 900 })
      const backend = fixture()
      await openTaskSheet(page, backend)
      const sheet = page.getByRole('dialog')
      // Measure after the slide-in animation has finished.
      await sheet.evaluate((el) => Promise.all(el.getAnimations().map((a) => a.finished)))
      const box = (await sheet.boundingBox())!
      expect(box.x + box.width).toBeLessThanOrEqual(width + 1)
      // No horizontal scrolling inside the sheet.
      expect(await sheet.evaluate((el) => el.scrollWidth - el.clientWidth)).toBeLessThanOrEqual(1)
      for (const id of ['run-agent-btn', 'open-in-git-btn', 'task-title-input']) {
        const b = (await page.getByTestId(id).boundingBox())!
        expect(b.x, id).toBeGreaterThanOrEqual(box.x - 1)
        expect(b.x + b.width, id).toBeLessThanOrEqual(box.x + box.width + 1)
      }
      // The run button keeps its whole label on one line.
      const run = page.getByTestId('run-agent-btn')
      expect(await run.evaluate((el) => el.scrollWidth - el.clientWidth)).toBeLessThanOrEqual(1)
      await expect(run).toContainText('Execute with Agent')
    })
  }
})
