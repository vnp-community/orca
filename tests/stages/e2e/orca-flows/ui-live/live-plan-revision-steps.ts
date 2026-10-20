/**
 * Steps of the plan-revision spec (CR-REQ-045): the Plan tab's "Request changes to this plan" panel, the version list, the
 * read-only older versions. The fake claude writes the same plan whatever the feedback says, so the checks are about structure:
 * how many versions exist, their statuses, the new Approval of every round, the request each version carries.
 */
import { expect, type Locator, type Page } from '@playwright/test'
import type { LiveFlow } from './live-flow-harness'
import { approveAnalysis } from './live-analysis-approval-steps'
import {
  answerClarificationIfAsked,
  createClassifyAndConfirm,
  type RequestUnderTest
} from './live-flow-steps'
import { pendingApprovals, requestState, waitForApiStatus } from './live-gateway-crosschecks'
import {
  approveAnalysisAndLeavePlanToOrca,
  planApproveButton,
  reachAnalysisApproval
} from './live-auto-plan-steps'
import { configureProject } from './live-continuous-execution-steps'
import { readPlanPanelRecorder, startPlanPanelRecorder } from './live-plan-revision-panel-recorder'
import {
  detailPane,
  openDetailTab,
  openRequestsPage,
  selectRequestRow
} from './live-request-ui-steps'
import { expectScreenFollowsService, UI_REFRESH_GRACE_MS } from './live-status-sync'
import { dismissFirstRunTip } from './live-ui-session'

// Why: a real OpenSpec plan runs up to 64 minutes (the tab's own budget); a native one takes seconds to a few minutes.
const PLAN_WAIT_MS = 70 * 60_000
// Why: the fake claude writes a Plan in seconds, so a Plan that has not reached the screen after three minutes is a defect, and it should fail with its screenshot, not at the test timeout.
const STUB_PLAN_WAIT_MS = 3 * 60_000
const PLAN_SUBJECTS = new Set(['plan', 'task_list'])
export const MAX_FEEDBACK = 2000

/** A project on the native engine with the plain model (no knowledge hub), put back after the body. */
export async function withNativeProject(f: LiveFlow, body: () => Promise<void>): Promise<void> {
  const before = await configureProject(f, {
    engine: 'native',
    solutionAgent: false,
    executionMode: 'step_by_step',
    planAutoGenerate: true
  })
  try {
    await body()
  } finally {
    await configureProject(f, before).catch(() => undefined)
  }
}

export type PlanVersionRow = {
  revision: number
  runId: string
  status: string
  revisionFeedback?: string
  textKept?: boolean
}

export const planRevisionPanel = (page: Page): Locator =>
  detailPane(page).getByTestId('plan-revision-panel')
export const planRevisionInput = (page: Page): Locator =>
  detailPane(page).getByTestId('plan-revision-input')
export const planVersionBar = (page: Page): Locator =>
  detailPane(page).getByTestId('plan-version-bar')
export const planVersionButton = (page: Page, n: number): Locator =>
  detailPane(page).getByRole('button', { name: new RegExp(`^Version ${n}\\b`) })

/** Saving the settings page crashes: "crash" is a bug cue for the fake claude. */
const STUB_BUG_BODY =
  'Saving the settings page crashes with an error when the badge colour is empty. It worked last week.'

/**
 * Test setup of the whole flow: a native `bug` whose Plan Orca wrote and which waits for approval. It is never approved. With
 * `real` the analysis is approved the way a real model needs (it may ask a blocking question), and the Plan may take minutes.
 */
export async function reachPlanReady(
  f: LiveFlow,
  titleWords: string,
  o: { body?: string; real?: boolean } = {}
): Promise<RequestUnderTest> {
  let r: RequestUnderTest
  if (o.real) {
    r = await createClassifyAndConfirm(f, {
      titleWords,
      body: o.body ?? STUB_BUG_BODY,
      typeLabel: 'Bug',
      typeKey: 'bug',
      size: 'S'
    })
    const asked = await answerClarificationIfAsked(f, r)
    await approveAnalysis(f, r, {
      autoStarted: asked,
      next: { api: 'planning', chip: /Planning/i }
    })
  } else {
    const reached = await reachAnalysisApproval(f, titleWords)
    r = reached.r
    await approveAnalysisAndLeavePlanToOrca(f, r, reached.asked)
  }
  await f.step(
    'the Plan is written and waits for approval; the revision panel is offered',
    async () => {
      await openDetailTab(f.page, 'Plan')
      await expect(planApproveButton(f.page)).toBeVisible({
        timeout: o.real ? PLAN_WAIT_MS : STUB_PLAN_WAIT_MS
      })
      await waitForApiStatus(f.rpc, r.requestId, ['awaiting_plan_approval'], 30_000)
      await expectScreenFollowsService(f.page, f.rpc, f.gaps, {
        requestId: r.requestId,
        title: r.title,
        step: 'PLAN-READY',
        apiStatus: ['awaiting_plan_approval'],
        chip: /Awaiting Plan Approval/i,
        apiWaitMs: f.budgets.planMs
      })
      await expect(planRevisionPanel(f.page), 'the revision panel is offered').toBeVisible({
        timeout: UI_REFRESH_GRACE_MS
      })
      await f.shots.take('plan-ready')
    }
  )
  return r
}

/** The versions of the Request's plan, oldest first, as the service lists them. */
export async function listPlanVersions(f: LiveFlow, requestId: string): Promise<PlanVersionRow[]> {
  const res = await f.rpc.call<{ revisions?: PlanVersionRow[] }>('request.planRevisions', {
    id: requestId,
    pageSize: 100
  })
  return [...(res.revisions ?? [])].sort((a, b) => a.revision - b.revision)
}

/** Ids of the Approvals that decide the plan and are still open. */
export async function openPlanApprovals(f: LiveFlow, requestId: string): Promise<string[]> {
  return (await pendingApprovals(f.rpc, requestId))
    .filter((a) => PLAN_SUBJECTS.has(a.subjectType))
    .map((a) => a.id)
}

export type PlanCalls = {
  /** The `mode` of every `request.generatePlan` this window sent: a revision must send `propose` only. */
  generateModes: () => string[]
  /** How many times this window sent the method to the gateway. */
  count: (method: string) => number
}

/**
 * Watches what this window asks the gateway for. The socket of a signed-in page predates the listener, so the page is reloaded
 * once and the Plan tab opened again.
 */
export async function watchPlanCalls(f: LiveFlow, r: RequestUnderTest): Promise<PlanCalls> {
  const modes: string[] = []
  const sent = new Map<string, number>()
  f.page.on('websocket', (ws) => {
    ws.on('framesent', (frame) => {
      for (const line of String(frame.payload).split('\n')) {
        try {
          const call = JSON.parse(line) as { method?: string; params?: unknown }
          if (call.method) {
            sent.set(call.method, (sent.get(call.method) ?? 0) + 1)
          }
          if (call.method === 'request.generatePlan') {
            const args = Array.isArray(call.params) ? call.params[0] : call.params
            modes.push(String((args as { mode?: string } | undefined)?.mode ?? ''))
          }
        } catch {
          // not a JSON frame
        }
      }
    })
  })
  await f.page.reload({ waitUntil: 'domcontentloaded' })
  await dismissFirstRunTip(f.page)
  await openRequestsPage(f.page)
  await selectRequestRow(f.page, r.title)
  await openDetailTab(f.page, 'Plan')
  await expect(planRevisionPanel(f.page)).toBeVisible({ timeout: UI_REFRESH_GRACE_MS })
  return { generateModes: () => [...modes], count: (method) => sent.get(method) ?? 0 }
}

/**
 * Types `text` in the panel and presses "Revise plan". Checks the busy line, then waits for version `version` and its new
 * Approval in the service and on the screen. Returns the id of the new Approval.
 */
export async function revisePlan(
  f: LiveFlow,
  r: RequestUnderTest,
  o: { text: string; version: number; previousApprovals: string[] }
): Promise<string> {
  let approvalId = ''
  await f.step(
    `request changes: "Writing version ${o.version}…", then version ${o.version}`,
    async () => {
      await openDetailTab(f.page, 'Plan')
      await planRevisionInput(f.page).fill(o.text)
      await expect(detailPane(f.page).getByTestId('plan-revision-count')).toContainText(
        `${[...o.text].length} / ${MAX_FEEDBACK}`
      )
      await f.shots.take(`revision-${o.version}-typed`)
      await startPlanPanelRecorder(f.page)
      await detailPane(f.page).getByTestId('plan-revision-submit').click()

      await expect
        .poll(async () => (await listPlanVersions(f, r.requestId)).length, {
          timeout: f.budgets.planMs,
          message: `the service holds ${o.version} versions`
        })
        .toBe(o.version)
      await waitForApiStatus(f.rpc, r.requestId, ['awaiting_plan_approval'], f.budgets.planMs)
      await expectScreenFollowsService(f.page, f.rpc, f.gaps, {
        requestId: r.requestId,
        title: r.title,
        step: `PLAN-REVISION-${o.version}`,
        apiStatus: ['awaiting_plan_approval'],
        chip: /Awaiting Plan Approval/i,
        apiWaitMs: f.budgets.planMs
      })

      const log = await readPlanPanelRecorder(f.page)
      expect(
        log.map((s) => s.status),
        `the panel said which version is being written while it worked (saw ${JSON.stringify(log.map((x) => x.status))})`
      ).toContain(`Writing version ${o.version}…`)

      const versions = await listPlanVersions(f, r.requestId)
      const latest = versions.at(-1)!
      expect(latest.revisionFeedback, 'the new version carries the request that produced it').toBe(
        o.text
      )
      expect(latest.status, 'the new version is the current one').toBe('current')
      expect(
        versions.slice(0, -1).map((v) => v.status),
        'every earlier version is superseded'
      ).toEqual(versions.slice(0, -1).map(() => 'superseded'))
      expect(
        versions.map((v) => v.revision),
        'versions are numbered in order'
      ).toEqual(versions.map((_, i) => i + 1))

      await expect
        .poll(async () => (await openPlanApprovals(f, r.requestId)).length, {
          timeout: 30_000,
          message: 'one Approval is open for the new version'
        })
        .toBe(1)
      const [open] = await openPlanApprovals(f, r.requestId)
      expect(
        o.previousApprovals,
        'the Approval is a new one, not the old one reopened'
      ).not.toContain(open)
      approvalId = open

      await expect(
        planApproveButton(f.page),
        'the new Approval can be decided on screen'
      ).toBeVisible({
        timeout: UI_REFRESH_GRACE_MS
      })
      await expect(planRevisionInput(f.page), 'the box is emptied').toHaveValue('')
      expect((await requestState(f.rpc, r.requestId)).status).toBe('awaiting_plan_approval')
      await f.shots.take(`revision-${o.version}-done`)
    }
  )
  return approvalId
}

/** Opens "Versions (n)" and chooses a version in the list. */
export async function chooseVersion(f: LiveFlow, n: number): Promise<void> {
  const bar = planVersionBar(f.page)
  if (!(await planVersionButton(f.page, n).isVisible())) {
    await bar.getByRole('button', { name: /^Versions \(\d+\)/ }).click()
  }
  await planVersionButton(f.page, n).click()
}

/** The request that produced the version on screen, in the "What was asked for this version" block (opened when closed). */
async function expectAskedFor(f: LiveFlow, text: string): Promise<void> {
  const bar = planVersionBar(f.page)
  const block = bar.getByTestId('plan-version-asked')
  const shown = block.locator('p')
  if (!(await shown.isVisible())) {
    await block.getByRole('button', { name: 'What was asked for this version' }).click()
  }
  await expect(shown).toHaveText(text)
}

/**
 * Older versions are read-only and show what was asked for them; "Back to current" returns to the version to decide on.
 * `asked[i]` is the request that produced version i + 2 (the first proposal had none); the last one is the current version.
 */
export async function expectOlderPlanVersionsReadOnly(f: LiveFlow, asked: string[]): Promise<void> {
  await f.step('an older version is read-only and shows what was asked for it', async () => {
    const pane = detailPane(f.page)
    await chooseVersion(f, 1)
    await expect(pane.getByTestId('plan-older-version-banner')).toBeVisible()
    await expect(planRevisionPanel(f.page), 'no revision box on an old version').toHaveCount(0)
    await expect(
      pane.getByTestId('plan-proposal-view'),
      'the old version is read in the Proposal view'
    ).toBeVisible({
      timeout: UI_REFRESH_GRACE_MS
    })
    await expect(
      pane.getByTestId('plan-version-asked'),
      'the first proposal had no request'
    ).toHaveCount(0)
    await f.shots.take('older-version-1')
    for (const [i, text] of asked.slice(0, -1).entries()) {
      await chooseVersion(f, i + 2)
      await expect(pane.getByTestId('plan-older-version-banner')).toBeVisible()
      await expectAskedFor(f, text)
    }
    await pane.getByTestId('plan-back-to-current').click()
    await expect(pane.getByTestId('plan-older-version-banner')).toHaveCount(0)
    await expect(planRevisionPanel(f.page), 'the box is back on the current version').toBeVisible()
    await expectAskedFor(f, asked.at(-1)!)
    await f.shots.take('current-version-asked-for')
  })
}
