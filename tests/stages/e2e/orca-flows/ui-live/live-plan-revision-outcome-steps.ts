/**
 * What a plan revision leaves behind and how it fails (CR-REQ-045): the Activity row that never carries the reviewer's text,
 * the refusal of a text that is too long, a revision sent from a screen that is out of date, and a round that fails and is
 * tried again. In every refusal the typed text stays and no version is made.
 */
import { expect } from '@playwright/test'
import type { LiveFlow } from './live-flow-harness'
import type { RequestUnderTest } from './live-flow-steps'
import { requestState } from './live-gateway-crosschecks'
import { PushFrameGate } from './live-push-frame-gate'
import {
  listPlanVersions,
  MAX_FEEDBACK,
  openPlanApprovals,
  planRevisionInput,
  planRevisionPanel,
  type PlanCalls
} from './live-plan-revision-steps'
import { readPlanPanelRecorder, startPlanPanelRecorder } from './live-plan-revision-panel-recorder'
import {
  detailHeader,
  detailPane,
  openDetailTab,
  openRequestsPage,
  selectRequestRow
} from './live-request-ui-steps'
import { UI_REFRESH_GRACE_MS } from './live-status-sync'
import { dismissFirstRunTip } from './live-ui-session'

const REVISION_LINE = 'Requested changes to the plan'

/** History shows one line per revision and none of the reviewers' words, on screen or in the service's answer. */
export async function expectPlanRevisionActivity(
  f: LiveFlow,
  r: RequestUnderTest,
  o: { lines: number; secrets: string[] }
): Promise<void> {
  await f.step(`History has ${o.lines} "${REVISION_LINE}" line(s) and never the text`, async () => {
    await openDetailTab(f.page, 'History')
    const pane = detailPane(f.page)
    await expect(
      pane.getByTestId('activity-entry').filter({ hasText: REVISION_LINE }),
      'one line per revision on the screen'
    ).toHaveCount(o.lines, { timeout: 30_000 })
    for (const secret of o.secrets) {
      await expect(pane, 'the text is not on the History tab').not.toContainText(secret)
    }
    await expect(
      pane.getByTestId('activity-entry').filter({ hasText: 'Requested changes to the analysis' }),
      'a plan round is not reported as a change to the analysis'
    ).toHaveCount(0)
    await f.shots.take('history-plan-revision-lines')

    const feed = await f.rpc.call<{ entries?: { kind: string }[] }>('request.activity', {
      id: r.requestId,
      pageSize: 100
    })
    const entries = feed.entries ?? []
    expect(entries.filter((e) => e.kind === 'plan.revision_requested').length).toBe(o.lines)
    expect(entries.filter((e) => e.kind === 'analysis.revision_requested').length).toBe(0)
    const wire = JSON.stringify(entries)
    for (const secret of o.secrets) {
      expect(wire, 'the service does not send the text in the activity feed').not.toContain(secret)
    }
  })
}

/** A text over the limit is refused on the spot: the box keeps it, the service hears nothing. */
export async function expectPlanTooLongKeepsText(f: LiveFlow, r: RequestUnderTest): Promise<void> {
  await f.step('a text over 2000 characters is refused and kept in the box', async () => {
    await openDetailTab(f.page, 'Plan')
    const before = (await listPlanVersions(f, r.requestId)).length
    const long = 'x'.repeat(MAX_FEEDBACK + 1)
    const input = planRevisionInput(f.page)
    await input.fill(long)
    await detailPane(f.page).getByTestId('plan-revision-submit').click()
    await expect(detailPane(f.page).getByTestId('plan-revision-error')).toContainText(
      `${MAX_FEEDBACK} characters or fewer`
    )
    await expect(input, 'the typed text is still there').toHaveValue(long)
    await f.shots.take('plan-revision-too-long')
    expect((await listPlanVersions(f, r.requestId)).length, 'no version was made').toBe(before)
    await input.fill('')
  })
}

/**
 * The Request is cancelled while another window still shows its plan awaiting approval. Sending a revision from that window is
 * refused with a message that says so; the typed text is in the box when the message shows, and nothing is created. The second
 * window's push events are held, so the refusal is not raced by the refresh the cancel would cause.
 */
export async function expectPlanRevisionAfterRequestMovedOn(
  f: LiveFlow,
  r: RequestUnderTest,
  text: string
): Promise<void> {
  await f.step(
    'revising from a screen that is out of date is refused and keeps the text',
    async () => {
      const before = (await listPlanVersions(f, r.requestId)).length
      const stale = await f.page.context().newPage()
      try {
        const gate = await PushFrameGate.install(stale)
        await stale.goto('/')
        await dismissFirstRunTip(stale)
        await openRequestsPage(stale)
        await selectRequestRow(stale, r.title)
        await openDetailTab(stale, 'Plan')
        const panel = planRevisionPanel(stale)
        await expect(panel).toBeVisible({ timeout: UI_REFRESH_GRACE_MS })
        await planRevisionInput(stale).fill(text)
        await startPlanPanelRecorder(stale)

        gate.hold()
        const now = await requestState(f.rpc, r.requestId)
        await f.rpc.call('request.cancel', {
          id: r.requestId,
          reason: `${f.cfg.runPrefix}: the plan revision spec moves the Request on`,
          expectedVersion: now.version
        })
        await panel.getByTestId('plan-revision-submit').click()

        await expect
          .poll(async () => (await readPlanPanelRecorder(stale)).some((s) => s.error !== ''), {
            timeout: 15_000,
            message: 'the panel showed an error'
          })
          .toBe(true)
        const refusal = (await readPlanPanelRecorder(stale)).find((s) => s.error !== '')!
        expect(refusal.error).toMatch(/already revising this plan or has decided/)
        expect(refusal.input, 'the text was still in the box when the refusal showed').toBe(text)
        gate.release()
        await stale.screenshot({
          path: f.testInfo.outputPath('plan-revision-after-request-moved-on.png')
        })
        await expect(
          detailHeader(stale),
          'the screen then learns the Request was cancelled'
        ).toContainText(/Cancelled/i, { timeout: UI_REFRESH_GRACE_MS })
      } finally {
        await stale.close()
      }
      expect((await listPlanVersions(f, r.requestId)).length, 'no version was made').toBe(before)
      expect((await requestState(f.rpc, r.requestId)).status).toBe('cancelled')
    }
  )
}

/**
 * A round that fails (the fake claude fails the first answer to a text carrying `[stub:fail-once]`): the failure stays on screen
 * with "Try again", the Request waits in planning with its old plan and no open Approval, and Try again writes the version with
 * the same request, so nothing is retyped. The browser asks `request.generatePlan` again and never `request.retryPlan`.
 */
export async function expectFailedRoundCanBeTriedAgain(
  f: LiveFlow,
  r: RequestUnderTest,
  o: { text: string; calls: PlanCalls; previousApprovals: string[] }
): Promise<string> {
  let approvalId = ''
  await f.step('a failed round shows the failure, and Try again writes the version', async () => {
    await openDetailTab(f.page, 'Plan')
    const before = (await listPlanVersions(f, r.requestId)).length
    await planRevisionInput(f.page).fill(o.text)
    await detailPane(f.page).getByTestId('plan-revision-submit').click()

    const card = detailPane(f.page).getByTestId('plan-run-failed')
    await expect(card, 'the failure is on screen').toBeVisible({ timeout: f.budgets.planMs })
    await expect(card).toContainText('The plan could not be revised')
    await expect(card).toContainText('Or write a new request below.')
    await f.shots.take('revision-failed')
    expect((await requestState(f.rpc, r.requestId)).status, 'the Request waits in planning').toBe(
      'planning'
    )
    expect(
      await openPlanApprovals(f, r.requestId),
      'no Approval is open while the round is failed'
    ).toEqual([])
    expect((await listPlanVersions(f, r.requestId)).length, 'a failed round makes no version').toBe(
      before
    )
    await expect(planRevisionInput(f.page), 'a new request can be written').toBeEnabled()

    const sentBefore = o.calls.count('request.generatePlan')
    await card.getByRole('button', { name: 'Try again' }).click()
    await expect
      .poll(async () => (await listPlanVersions(f, r.requestId)).length, {
        timeout: f.budgets.planMs,
        message: 'the retried round makes the version'
      })
      .toBe(before + 1)
    const latest = (await listPlanVersions(f, r.requestId)).at(-1)!
    expect(latest.revisionFeedback, 'the retry kept the request that failed').toBe(o.text)
    await expect
      .poll(async () => (await openPlanApprovals(f, r.requestId)).length, { timeout: 30_000 })
      .toBe(1)
    const [open] = await openPlanApprovals(f, r.requestId)
    expect(o.previousApprovals, 'a new Approval').not.toContain(open)
    approvalId = open
    expect(o.calls.count('request.generatePlan') - sentBefore, 'Try again asked once').toBe(1)
    expect(o.calls.count('request.retryPlan'), 'and not through the retry of the first plan').toBe(
      0
    )
    expect(
      o.calls.generateModes().every((m) => m === 'propose'),
      'the browser never saved a plan itself'
    ).toBe(true)
    await expect(card).toHaveCount(0, { timeout: UI_REFRESH_GRACE_MS })
    await f.shots.take('revision-after-try-again')
  })
  return approvalId
}
