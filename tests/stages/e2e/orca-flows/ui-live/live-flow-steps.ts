/**
 * The steps of the Request flow, each driven through the UI and cross-checked against the service. A step that the UI
 * cannot complete fails the run (no gateway fallback); only delays and stale screens are recorded as gaps.
 */
import { expect } from '@playwright/test'
import type { LiveFlow } from './live-flow-harness'
import {
  findRequestByTitle,
  pendingApprovals,
  requestState,
  waitForApiStatus
} from './live-gateway-crosschecks'
import {
  answerClarificationInPanel,
  approvalRowFor,
  cancelOpenRequestThroughTheUi,
  confirmTypeInCard,
  createRequestThroughTheDialog,
  detailHeader,
  detailPane,
  openDetailTab,
  openRequestsPage,
  rejectApprovalRow,
  selectRequestRow
} from './live-request-ui-steps'
import { expectScreenFollowsService, UI_REFRESH_GRACE_MS } from './live-status-sync'

export type RequestUnderTest = { requestId: string; title: string }

/** Create in the dialog, wait for classification, confirm the given type in the card. */
export async function createClassifyAndConfirm(
  f: LiveFlow,
  o: {
    titleWords: string
    body: string
    typeLabel: string
    typeKey: string
    size: string
    /** Replaces the default "open the dialog, pick f.project by name, Create" when the spec checks the dialog itself. */
    create?: (title: string) => Promise<void>
  }
): Promise<RequestUnderTest> {
  const { rpc, page, shots, gaps, lags, step } = f
  const title = `${f.cfg.runPrefix} ${o.titleWords}`
  let requestId = ''
  await step('create the Request in the dialog', async () => {
    await openRequestsPage(page)
    await (o.create?.(title) ??
      createRequestThroughTheDialog(page, { title, body: o.body, projectName: f.project.name }))
    await expect(detailHeader(page)).toContainText(title)
    await expect
      .poll(async () => (await findRequestByTitle(rpc, f.project.id, title))?.id ?? '', {
        timeout: 15_000
      })
      .not.toBe('')
    requestId = (await findRequestByTitle(rpc, f.project.id, title))!.id
    await shots.take('request-created')
  })
  await step('classification finishes and asks for type confirmation', async () => {
    const lag = await expectScreenFollowsService(page, rpc, gaps, {
      requestId,
      title,
      step: 'CLASSIFY',
      apiStatus: ['awaiting_type_confirmation'],
      chip: /Awaiting Type Confirmation/i,
      apiWaitMs: f.budgets.classifyMs
    })
    lags.push(`classification ${lag}ms`)
    await expect(page.getByTestId('type-confirmation-card')).toBeVisible()
    await shots.take('type-confirmation')
  })
  await step(`confirm the type (${o.typeKey}, size ${o.size}) in the card`, async () => {
    await confirmTypeInCard(page, o.typeLabel, o.size)
    const after = await waitForApiStatus(
      rpc,
      requestId,
      ['awaiting_information', 'analyzing', 'planning'],
      30_000
    )
    expect(after.type).toBe(o.typeKey)
    await shots.take('type-confirmed')
  })
  return { requestId, title }
}

/** Answers the readiness Clarification when the Request has one. Returns whether it did. */
export async function answerClarificationIfAsked(
  f: LiveFlow,
  r: RequestUnderTest,
  // Why: a type without analysis (task, docs) goes from the Clarification straight to planning.
  after: { api: string[]; chip: RegExp } = {
    api: ['analyzing', 'awaiting_analysis_approval'],
    chip: /Analyzing|Awaiting Analysis Approval/i
  }
): Promise<boolean> {
  const { rpc, page, shots, gaps, lags } = f
  if ((await requestState(rpc, r.requestId)).status !== 'awaiting_information') {
    return false
  }
  await f.step('answer the readiness Clarification on the screen', async () => {
    await expectScreenFollowsService(page, rpc, gaps, {
      requestId: r.requestId,
      title: r.title,
      step: 'AWAITING-INFORMATION',
      apiStatus: ['awaiting_information'],
      chip: /Awaiting Information/i,
      apiWaitMs: 15_000
    })
    await shots.take('clarification-open')
    expect(
      await answerClarificationInPanel(page, `${f.cfg.runPrefix} answer`),
      'Submit answers enabled once every control is filled'
    ).toBe(true)
    // Why: the Solution now starts by itself, so `analyzing` may already be over by the time the screen is checked.
    const lag = await expectScreenFollowsService(page, rpc, gaps, {
      requestId: r.requestId,
      title: r.title,
      step: 'AFTER-CLARIFICATION',
      apiStatus: after.api,
      chip: after.chip,
      apiWaitMs: 30_000
    })
    lags.push(`after Clarification answer ${lag}ms`)
    const answered = await rpc.call<{ clarifications: unknown[] }>('clarification.list', {
      requestId: r.requestId,
      status: 'answered'
    })
    expect(answered.clarifications.length, 'the answers were recorded').toBeGreaterThan(0)
    await shots.take('clarification-answered')
  })
  return true
}

/** "Generate plan" in the Plan tab; the UI itself proposes, waits and commits. */
export async function generatePlanThroughTheUi(f: LiveFlow, r: RequestUnderTest): Promise<void> {
  const { rpc, page, shots, gaps } = f
  await f.step('"Generate plan" produces a Plan awaiting approval', async () => {
    await openDetailTab(page, 'Plan')
    await detailPane(page)
      .getByTestId('plan-generate')
      .or(detailPane(page).getByRole('button', { name: 'Generate plan' }))
      .first()
      .click()
    await waitForApiStatus(rpc, r.requestId, ['awaiting_plan_approval'], f.budgets.planMs)
    await expectScreenFollowsService(page, rpc, gaps, {
      requestId: r.requestId,
      title: r.title,
      step: 'AFTER-PLAN-COMMIT',
      apiStatus: ['awaiting_plan_approval'],
      chip: /Awaiting Plan Approval/i,
      apiWaitMs: 30_000
    })
    await openDetailTab(page, 'Plan')
    await expect(page.getByTestId('request-tab-plan')).toBeVisible({ timeout: 15_000 })
    await expect
      .soft(detailPane(page).getByText('Plan not found'), 'the Plan tab shows the committed Plan')
      .toHaveCount(0, { timeout: 15_000 })
    await shots.take('plan-committed')
  })
}

/** The Plan Approval is in the inbox; reject it (back to the backlog) and cancel the Request, all from the UI. Never approves. */
export async function rejectPlanApprovalAndCancel(f: LiveFlow, r: RequestUnderTest): Promise<void> {
  const { rpc, page, shots } = f
  await f.step(
    'the next Approval (Plan) is in the inbox; reject it (it is never approved)',
    async () => {
      const pending = await pendingApprovals(rpc, r.requestId)
      expect(
        pending.some((a) => ['plan', 'task_list'].includes(a.subjectType)),
        `a pending plan/task_list approval (got ${pending.map((a) => a.subjectType).join(',') || 'none'})`
      ).toBe(true)
      await page.getByRole('tab', { name: /Approvals/ }).click()
      const row = approvalRowFor(page, r.title, /\bPlan\b/)
      await expect(row).toBeVisible({ timeout: 30_000 })
      await shots.take('plan-approval-in-inbox')
      if (f.cfg.executeAgent) {
        f.testInfo.annotations.push({
          type: 'not-run',
          description:
            'ORCA_E2E_EXECUTE_AGENT=1 is set but this test never approves the Plan; approval runs live in the guarded execute-real specs (run_ui_flow.py --real execute)'
        })
      }
      await rejectApprovalRow(page, row, `${f.cfg.runPrefix}: not executing this plan`)
      await waitForApiStatus(rpc, r.requestId, ['request_backlog'], 30_000)
    }
  )
  await cancelThroughTheUi(f, r, /backlog/i)
}

/** Cancel the open Request from its detail header; `from` is the status chip expected before the click. */
export async function cancelThroughTheUi(
  f: LiveFlow,
  r: RequestUnderTest,
  from: RegExp
): Promise<void> {
  const { rpc, page, shots } = f
  await f.step('cancel the Request from the UI', async () => {
    await page.getByRole('tab', { name: 'Requests' }).click()
    await selectRequestRow(page, r.title)
    await expect(detailHeader(page)).toContainText(from, { timeout: UI_REFRESH_GRACE_MS })
    await shots.take('request-before-cancel')
    await cancelOpenRequestThroughTheUi(page, `${f.cfg.runPrefix}: live UI suite cleanup`)
    await waitForApiStatus(rpc, r.requestId, ['cancelled'], 30_000)
    await expect(detailHeader(page)).toContainText(/Cancelled/i, { timeout: UI_REFRESH_GRACE_MS })
    await shots.take('request-cancelled')
  })
}
