/**
 * E2E for clarification, risk gating and the execution result panel
 * (FE-REQ-TASK-036-03, 036-06, 036-08) against the web SPA with a mocked gateway WebSocket.
 * `impact.*`, `readiness.*` and `execution.get` channel names are provisional (CONTRACT section 7).
 *
 *   MCP_E2E_BASE_URL=http://127.0.0.1:5174 npx playwright test -c tests/playwright.web.config.ts \
 *     --project=mcp-web tests/e2e/request-web/request-clarification-risk.web.e2e.ts
 */
import { expect, test } from '@playwright/test'
import {
  approvalView,
  bootRequestApp,
  createFakeRequestBackend,
  mockRequestApp,
  openRequestPageViaStore,
  openRequestsFromSidebar,
  requestView
} from './support/mock-request-ws'

const now = '2026-10-08T09:00:00Z'

const awaiting = requestView({
  id: 'req-c',
  number: 7,
  title: 'Needs answers',
  status: 'awaiting_information'
})
const clarification = {
  id: 'cl-1',
  display_id: 'CL-1',
  request_id: 'req-c',
  status: 'open',
  version: 4,
  round: 1,
  resume_status: 'planning',
  assignee_ids: ['u1'],
  questions: [
    { id: 'q1', seq: 1, kind: 'text', prompt: 'Which table holds the orders?', required: true },
    { id: 'q2', seq: 2, kind: 'boolean', prompt: 'Is downtime allowed?', required: false }
  ]
}

const planned = requestView({
  id: 'req-r',
  number: 8,
  title: 'Risky plan',
  status: 'awaiting_plan_approval',
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
const planTasks = [
  task('plan-1', { type: 'plan', title: 'The plan' }),
  task('ph-1', { type: 'phase', parentId: 'plan-1', title: 'Phase One' }),
  task('t-1', { parentId: 'ph-1', title: 'Drop legacy column', status: 'done' })
]

test.describe('Clarification, risk gate and execution result (mocked request-service)', () => {
  test('clarification: "Awaiting information" chip, header "Answer", one submit with every answer', async ({
    page
  }) => {
    const backend = createFakeRequestBackend({
      'request.list': (p) => ({
        requests:
          (p.status as string[] | undefined)?.includes('awaiting_information') || !p.status
            ? [awaiting]
            : [],
        nextPageToken: null
      }),
      'request.get': () => ({ request: awaiting }),
      'clarification.list': () => ({ clarifications: [clarification] }),
      'clarification.answer': () => ({ still_missing: false })
    })
    await mockRequestApp(page, backend)
    await bootRequestApp(page)
    await openRequestsFromSidebar(page)

    await page.getByRole('button', { name: 'Awaiting information' }).click()
    await expect(page.getByRole('button', { name: 'Awaiting information' })).toHaveAttribute(
      'aria-pressed',
      'true'
    )
    await expect
      .poll(() => backend.callsTo('request.list').at(-1)?.status)
      .toEqual(['awaiting_information'])

    await page
      .getByRole('listbox', { name: 'Requests' })
      .getByRole('option', { name: /#7 Needs answers/ })
      .click()
    const header = page.getByTestId('request-detail-header')
    await header.getByRole('button', { name: 'Answer' }).click()
    const panel = page.getByTestId('clarification-panel')
    await expect(panel).toBeFocused()
    // The timeline marks the step that resumes after the answer (resumeStatus=planning).
    await expect(
      page.getByTestId('request-stage-timeline').locator('[aria-current="step"]')
    ).toContainText('Plan')

    const submit = panel.getByRole('button', { name: 'Submit answers' })
    await expect(submit).toBeDisabled()
    await panel.getByRole('textbox').fill('orders_v2')
    await submit.click()
    await expect.poll(() => backend.callsTo('clarification.answer').length).toBe(1)
    expect(backend.callsTo('clarification.answer')[0]).toMatchObject({
      clarificationId: 'cl-1',
      complete: true,
      expectedVersion: 4,
      answers: expect.arrayContaining([
        expect.objectContaining({ questionId: 'q1', valueJson: '"orders_v2"' })
      ])
    })
    await expect(panel.getByTestId('clarification-submitted')).toBeVisible()
  })

  test('risk gate on plan approval: high finding needs an acceptance; approve carries acceptedFindingIds', async ({
    page
  }) => {
    const backend = createFakeRequestBackend({
      'request.list': () => ({ requests: [planned], nextPageToken: null }),
      'request.get': () => ({ request: planned }),
      'task.list': () => ({ tasks: planTasks, nextPageToken: '' }),
      'task.getDependencies': () => [],
      'approval.list': () => ({
        approvals: [
          approvalView({
            id: 'ap-plan',
            requestId: 'req-r',
            subjectType: 'plan',
            subjectId: 'plan-1'
          })
        ]
      }),
      'impact.get': (p) =>
        p.subjectType === 'plan'
          ? {
              assessment_id: 'as-1',
              digest: 'dg-1',
              level: 'high',
              score: 71,
              status: 'ready',
              mode: 'enforce',
              tool: 'codegraph'
            }
          : {},
      'impact.findings': () => ({
        findings: [
          { id: 'f-1', dimension: 'data', level: 'high', title: 'Drops a column that reports read' }
        ]
      }),
      'impact.accept': () => ({ acceptance: { acceptedBy: 'u1', createdAt: now } }),
      'approval.approve': () => ({ approval: approvalView({ id: 'ap-plan', status: 'approved' }) })
    })
    await mockRequestApp(page, backend)
    await bootRequestApp(page)
    await openRequestPageViaStore(page, { section: 'requests', requestId: 'req-r', focus: 'plan' })

    const bar = page.getByTestId('plan-approval-bar')
    const approve = bar.getByTestId('plan-approve')
    await expect(bar.getByTestId('risk-gate-plan')).toBeVisible()
    await expect(approve).toBeDisabled()
    await bar.getByLabel('Reason for accepting').fill('Reports migrated to orders_v2 last sprint')
    await bar.getByRole('button', { name: 'Record acceptance' }).click()
    await expect.poll(() => backend.callsTo('impact.accept').length).toBe(1)
    expect(backend.callsTo('impact.accept')[0]).toMatchObject({
      findingId: 'f-1',
      assessmentDigest: 'dg-1'
    })
    await expect(approve).toBeEnabled()
    await approve.click()
    await expect.poll(() => backend.callsTo('approval.approve').length).toBe(1)
    expect(backend.callsTo('approval.approve')[0]).toMatchObject({
      approvalId: 'ap-plan',
      acceptedFindingIds: ['f-1']
    })
  })

  test('execution result panel: TaskDetail "Result" tab shows agent claims next to the Orca re-run', async ({
    page
  }) => {
    const running = { ...planned, status: 'executing' }
    const backend = createFakeRequestBackend({
      'request.list': () => ({ requests: [running], nextPageToken: null }),
      'request.get': () => ({ request: running }),
      'task.list': () => ({ tasks: planTasks, nextPageToken: '' }),
      'task.getDependencies': () => [],
      'execution.get': () => ({
        result: {
          task_id: 't-1',
          attempt: 2,
          parse_status: 'ok',
          status: 'done',
          summary: 'Dropped legacy_total <script>x</script>',
          files_changed: ['db/migrations/0042_drop_legacy.sql'],
          checks_run: [{ id: 'unit', exit: 0 }],
          verdict: {
            status: 'failed',
            findings: [{ code: 'CHECK_MISMATCH', message: 'unit failed on re-run' }]
          }
        }
      })
    })
    await mockRequestApp(page, backend)
    await bootRequestApp(page)
    await openRequestPageViaStore(page, { section: 'requests', requestId: 'req-r', focus: 'plan' })
    await page.getByTestId('plan-task-row-t-1').click()

    await page.getByRole('tab', { name: 'Result' }).click()
    const panel = page.getByTestId('execution-result-panel')
    await expect(panel).toBeVisible()
    await expect(panel).toContainText('Attempt 2')
    // Agent text is rendered as text, never as HTML.
    await expect(panel).toContainText('Dropped legacy_total <script>x</script>')
    await expect(panel).toContainText('db/migrations/0042_drop_legacy.sql')
    await expect(panel).toContainText('unit')
    expect(backend.callsTo('execution.get')[0]).toMatchObject({ taskId: 't-1', latestOnly: true })
  })
})
