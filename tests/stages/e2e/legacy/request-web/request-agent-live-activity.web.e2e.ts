/**
 * E2E for the live "Agent activity" panel of a task sheet (TASK-FE-E2E-012) against the web SPA with a
 * mocked gateway WebSocket. The mock answers `task.activity.subscribe` and pushes the frames task-service
 * publishes (cumulative stdout of the run); `task.get` drives the polled status.
 *
 *   npx playwright test -c tests/configs/playwright.web.config.ts --project=request-web \
 *     tests/stages/e2e/legacy/request-web/request-agent-live-activity.web.e2e.ts
 */
import { expect, test, type Page } from '@playwright/test'
import {
  agentOutputFrame,
  bootRequestApp,
  createFakeRequestBackend,
  mockRequestApp,
  openRequestPageViaStore,
  requestView,
  type FakeRequestBackend
} from './support/mock-request-ws'

const now = '2026-10-08T09:00:00Z'
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

type Fixture = { backend: FakeRequestBackend; setStatus: (status: string) => void }

function fixture(initialStatus = 'in_progress'): Fixture {
  let status = initialStatus
  const tasks = () => [
    task('plan-1', { type: 'plan', title: 'The plan' }),
    task('ph-1', { type: 'phase', parentId: 'plan-1', title: 'Phase One' }),
    task('t-1', { parentId: 'ph-1', title: 'Negate the value', status })
  ]
  const backend = createFakeRequestBackend({
    'request.list': () => ({ requests: [executing], nextPageToken: null }),
    'request.get': () => ({ request: executing }),
    'task.list': () => ({ tasks: tasks(), nextPageToken: '' }),
    'task.get': () => tasks()[2],
    'task.getDependencies': () => []
  })
  return { backend, setStatus: (s) => (status = s) }
}

async function openTaskSheet(page: Page, backend: FakeRequestBackend): Promise<void> {
  await mockRequestApp(page, backend)
  await bootRequestApp(page)
  await openRequestPageViaStore(page, { section: 'requests', requestId: 'req-r', focus: 'plan' })
  await page.getByTestId('plan-task-row-t-1').click()
  await expect(page.getByTestId('task-agent-activity')).toBeVisible()
}

const lineTexts = (page: Page) => page.getByTestId('task-agent-activity-line').allTextContents()

test.describe('Agent live activity in the task sheet (mocked gateway)', () => {
  test('a: shows what the agent does while it runs, then Finished once the task leaves in_progress', async ({
    page
  }) => {
    const { backend, setStatus } = fixture()
    await openTaskSheet(page, backend)
    const panel = page.getByTestId('task-agent-activity')

    await expect.poll(() => backend.activity.openTaskIds()).toEqual(['t-1'])
    expect(backend.callsTo('task.activity.subscribe')).toEqual([{ taskId: 't-1' }])
    await expect(panel.getByTestId('task-agent-activity-live')).toBeVisible()
    await expect(panel.getByTestId('task-agent-activity-empty')).toContainText(
      'Waiting for the agent'
    )

    const first = '· Session started (claude-test)\n» Reading the calculator.\n▸ Read src/calc.py\n'
    backend.activity.push('t-1', agentOutputFrame('t-1', first))
    await expect(page.getByTestId('task-agent-activity-line')).toHaveCount(3)
    expect(await lineTexts(page)).toEqual([
      '·Session started (claude-test)',
      '»Reading the calculator.',
      '▸Read src/calc.py'
    ])
    await expect(page.getByTestId('task-agent-activity-line').nth(2)).toHaveAttribute(
      'data-kind',
      'tool'
    )

    // Frames are cumulative: the next one carries everything again plus the new lines.
    const second = `${first}↳ ok · 12 lines\n▸ Bash: python3 -m unittest\n↳ ok · OK\n■ Done · 3 turns · 8.2s\n`
    backend.activity.push('t-1', agentOutputFrame('t-1', second))
    await expect(page.getByTestId('task-agent-activity-line')).toHaveCount(7)
    expect((await lineTexts(page)).at(-1)).toBe('■Done · 3 turns · 8.2s')
    await expect(page.getByTestId('task-agent-activity-line').last()).toHaveAttribute(
      'data-tone',
      'ok'
    )

    // The polled status (every 4 s) turns the Live badge into Finished; the lines stay.
    setStatus('done')
    await expect(panel.getByTestId('task-agent-activity-live')).toHaveCount(0, { timeout: 12_000 })
    await expect(panel.getByTestId('task-agent-activity-status')).toHaveText('Finished')
    await expect(page.getByTestId('task-agent-activity-line')).toHaveCount(7)
  })

  test('b: a Request task never shows the "no orchestration dispatch" notice', async ({ page }) => {
    const { backend } = fixture()
    await openTaskSheet(page, backend)
    await expect(page.getByTestId('task-live-status')).toContainText('in_progress')
    await expect(page.getByTestId('task-dispatch-none')).toHaveCount(0)
    await expect(page.getByText('Task này')).toHaveCount(0)
  })

  test('c: opening the sheet mid-run shows the whole run from the next frame; re-opening replays it without a second subscription', async ({
    page
  }) => {
    const { backend } = fixture()
    // Frames sent before anyone listens are gone: the backend keeps no history.
    backend.activity.push('t-1', agentOutputFrame('t-1', '» earlier line\n'))
    await openTaskSheet(page, backend)
    await expect(page.getByTestId('task-agent-activity-line')).toHaveCount(0)

    backend.activity.push('t-1', agentOutputFrame('t-1', '» earlier line\n▸ Read a.py\n'))
    await expect.poll(() => lineTexts(page)).toEqual(['»earlier line', '▸Read a.py'])

    await page.getByRole('dialog').getByRole('button', { name: 'Close' }).click()
    await expect(page.getByTestId('task-agent-activity')).toHaveCount(0)
    // Output that arrives while the sheet is closed is still collected.
    backend.activity.push(
      't-1',
      agentOutputFrame('t-1', '» earlier line\n▸ Read a.py\n» while closed\n')
    )

    await page.getByTestId('plan-task-row-t-1').click()
    await expect
      .poll(() => lineTexts(page))
      .toEqual(['»earlier line', '▸Read a.py', '»while closed'])
    expect(backend.callsTo('task.activity.subscribe')).toHaveLength(1)
  })

  test('d: when the stream is closed by the server the panel reconnects and keeps the lines', async ({
    page
  }) => {
    const { backend } = fixture()
    await openTaskSheet(page, backend)
    await expect.poll(() => backend.activity.openTaskIds()).toEqual(['t-1'])
    backend.activity.push('t-1', agentOutputFrame('t-1', '» before the drop\n'))
    await expect(page.getByTestId('task-agent-activity-line')).toHaveCount(1)

    backend.activity.end('t-1')
    await expect(page.getByTestId('task-agent-activity-reconnecting')).toBeVisible()
    await expect(page.getByTestId('task-agent-activity-line')).toHaveCount(1)
    await expect
      .poll(() => backend.callsTo('task.activity.subscribe').length, { timeout: 8_000 })
      .toBe(2)
    await expect(page.getByTestId('task-agent-activity-reconnecting')).toHaveCount(0)

    backend.activity.push('t-1', agentOutputFrame('t-1', '» before the drop\n» after the drop\n'))
    await expect.poll(() => lineTexts(page)).toEqual(['»before the drop', '»after the drop'])
  })

  test('e: credentials and markup in the output are shown masked / as plain text', async ({
    page
  }) => {
    const { backend } = fixture()
    await openTaskSheet(page, backend)
    await expect.poll(() => backend.activity.openTaskIds()).toEqual(['t-1'])
    const secret = 'sk-ant-abcdefghijklmnopqrstuvwxyz0123456789'
    backend.activity.push(
      't-1',
      agentOutputFrame('t-1', `» key is ${secret}\n» <img src=x onerror="window.__pwned=1">\n`, {
        stderr: `oops ghp_abcdefghijklmnopqrstuvwxyz0123456789\n`
      })
    )
    await expect(page.getByTestId('task-agent-activity-line')).toHaveCount(3)
    const text = (await lineTexts(page)).join('\n')
    expect(text).not.toContain('sk-ant')
    expect(text).not.toContain('ghp_')
    expect(text).toContain('<img src=x onerror="window.__pwned=1">')
    expect(
      await page.evaluate(() => (window as unknown as { __pwned?: number }).__pwned)
    ).toBeUndefined()
    await expect(page.getByTestId('task-agent-activity').locator('img')).toHaveCount(0)
  })

  test('f: a long run keeps a bounded window of the newest lines and the user can scroll away and jump back', async ({
    page
  }) => {
    const { backend } = fixture()
    await openTaskSheet(page, backend)
    await expect.poll(() => backend.activity.openTaskIds()).toEqual(['t-1'])
    const total = 700
    const out = Array.from({ length: total }, (_, i) => `» line ${i}`).join('\n')
    backend.activity.push('t-1', agentOutputFrame('t-1', `${out}\n`))
    await expect(page.getByTestId('task-agent-activity-line')).toHaveCount(400)
    await expect(page.getByTestId('task-agent-activity-older')).toBeVisible()
    await expect(page.getByTestId('task-agent-activity-line').last()).toContainText(
      `line ${total - 1}`
    )

    // Following by default: the newest line is in view.
    const box = page.getByTestId('task-agent-activity-lines')
    const atBottom = () =>
      box.evaluate((el) => el.scrollHeight - el.scrollTop - el.clientHeight < 4)
    expect(await atBottom()).toBe(true)

    await box.evaluate((el) => (el.scrollTop = 0))
    await expect(page.getByTestId('task-agent-activity-jump')).toBeVisible()
    backend.activity.push('t-1', agentOutputFrame('t-1', `${out}\n» line ${total}\n`))
    await expect(page.getByTestId('task-agent-activity-line').last()).toContainText(`line ${total}`)
    expect(await atBottom()).toBe(false) // new output does not yank the view away from what the user reads

    await page.getByTestId('task-agent-activity-jump').click()
    await expect.poll(atBottom).toBe(true)
    await expect(page.getByTestId('task-agent-activity-jump')).toHaveCount(0)
  })
})
