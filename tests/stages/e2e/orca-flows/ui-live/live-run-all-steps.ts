/**
 * Steps of the "Approve and run all steps" spec (CR-REQ-043 / FE-REQ-TASK-043-06), driven with roles and the data-testids the
 * components ship (plan-approve, plan-approve-run-all, run-all-confirm-dialog, execution-run-mode, project-execution-section).
 * Texts are the en locale. The gateway client only reads, except for the project's engine settings, which are test setup.
 */
import { expect, type Locator, type Page } from '@playwright/test'
import type { LiveFlow } from './live-flow-harness'
import {
  answerClarificationIfAsked,
  createClassifyAndConfirm,
  generatePlanThroughTheUi,
  type RequestUnderTest
} from './live-flow-steps'
import { approveAnalysis } from './live-analysis-approval-steps'
import { pendingApprovals, requestState, waitForApiStatus } from './live-gateway-crosschecks'
import { detailPane, openDetailTab } from './live-request-ui-steps'
import { expectScreenFollowsService } from './live-status-sync'

type EngineSettings = {
  engine?: string
  openspecMinVersion?: string
  solutionAgent?: boolean
  executionMode?: string
  version: number
}

export const planBar = (page: Page): Locator => detailPane(page).getByTestId('plan-approval-bar')
export const runAllButton = (page: Page): Locator =>
  detailPane(page).getByTestId('plan-approve-run-all')
export const runAllDialog = (page: Page): Locator => page.getByTestId('run-all-confirm-dialog')

/** A change_request (a flow with Phases) up to its Plan waiting for approval. "add support for" steers the fake classifier to it. */
export async function reachPlanApproval(
  f: LiveFlow,
  titleWords: string
): Promise<RequestUnderTest> {
  const r = await createClassifyAndConfirm(f, {
    titleWords,
    body: 'Add support for a weekly limit next to the daily limit of the ledger. Ship it in two steps.',
    typeLabel: 'Change request',
    typeKey: 'change_request',
    size: 'M'
  })
  const asked = await answerClarificationIfAsked(f, r)
  await approveAnalysis(f, r, { autoStarted: asked, next: { api: 'planning', chip: /Planning/i } })
  await generatePlanThroughTheUi(f, r)
  return r
}

/** Number of Phases the committed Plan has, from the service: the number the dialog has to name. */
export async function phaseCount(f: LiveFlow, r: RequestUnderTest): Promise<number> {
  const res = await f.rpc.call<{ tasks?: { type?: string; status?: string }[] }>('task.list', {
    projectId: f.project.id,
    requestId: r.requestId,
    taskTypes: ['phase'],
    pageSize: 100
  })
  // Why: a plan sent back keeps its older phases as cancelled rows; the dialog counts the live ones.
  return (res.tasks ?? []).filter((t) => t.type === 'phase' && t.status !== 'cancelled').length
}

/** The approval bar of a step-by-step request: Approve, then the run-all choice, then Reject, in that Tab order. */
export async function expectBothApproveButtons(f: LiveFlow): Promise<void> {
  await f.step('the Plan tab offers "Approve plan" and "Approve and run all steps"', async () => {
    await openDetailTab(f.page, 'Plan')
    const bar = planBar(f.page)
    await expect(bar.getByTestId('plan-approve')).toHaveText(/^\s*Approve plan\s*$/, {
      timeout: 30_000
    })
    await expect(runAllButton(f.page)).toHaveText(/Approve and run all steps/)
    const order = await bar
      .getByRole('button')
      .evaluateAll((els) => els.map((el) => el.getAttribute('data-testid')))
    expect(
      order.filter((id) =>
        ['plan-approve', 'plan-approve-run-all', 'plan-reject'].includes(id ?? '')
      ),
      'Tab order: Approve, Approve and run all steps, Reject'
    ).toEqual(['plan-approve', 'plan-approve-run-all', 'plan-reject'])
    await f.shots.take('run-all-two-buttons')
  })
}

/** Opens the confirmation and checks it names the real number of steps and what the run will do. */
export async function openRunAllDialogAndRead(f: LiveFlow, steps: number): Promise<void> {
  await runAllButton(f.page).click()
  const dialog = runAllDialog(f.page)
  await expect(dialog).toBeVisible()
  await expect(dialog.getByRole('heading')).toHaveText(
    `Run all ${steps} steps without asking again?`
  )
  await expect(dialog).toContainText('using your access')
  await expect(dialog).toContainText('needs information from you')
  await expect(dialog).toContainText('goes back to the backlog')
  await expect(dialog).toContainText('You can stop the run at any time.')
  // The dialog fades in; a shot taken at once shows it half transparent.
  await f.page.waitForTimeout(600)
  await f.shots.take('run-all-dialog')
}

/** After "Not now" / Escape: nothing was decided and the Request is exactly as it was. */
export async function expectNothingDecided(
  f: LiveFlow,
  r: RequestUnderTest,
  versionBefore: unknown
): Promise<void> {
  const now = await requestState(f.rpc, r.requestId)
  expect(now.status, 'still waiting for the plan approval').toBe('awaiting_plan_approval')
  expect(now.version, 'the request was not written').toBe(versionBefore)
  expect(now.executionMode, 'no mode was chosen').not.toBe('continuous')
  const pending = await pendingApprovals(f.rpc, r.requestId)
  expect(
    pending.some((a) => ['plan', 'task_list'].includes(String(a.subjectType))),
    'the plan approval is still pending'
  ).toBe(true)
}

/** The run goes on by itself: pinned continuous, no Phase approval, the Plan tab and the Execution tab say so. */
export async function expectRunsOnItsOwn(f: LiveFlow, r: RequestUnderTest): Promise<void> {
  await f.step('the request runs on its own and every screen says so', async () => {
    await waitForApiStatus(f.rpc, r.requestId, ['executing'], 60_000)
    expect((await requestState(f.rpc, r.requestId)).executionMode).toBe('continuous')
    const phaseGates = (await pendingApprovals(f.rpc, r.requestId)).filter(
      (a) => a.subjectType === 'phase'
    )
    expect(phaseGates, 'no Phase approval opens when the run goes on by itself').toEqual([])
    await openDetailTab(f.page, 'Plan')
    await expect(detailPane(f.page).getByTestId('plan-approved-strip')).toContainText(
      'The agent runs it on its own.',
      { timeout: 45_000 }
    )
    await expect(detailPane(f.page).getByTestId('plan-outline')).toBeVisible()
    await expect(detailPane(f.page).getByTestId('plan-approve-run-all')).toHaveCount(0)
    await openDetailTab(f.page, 'Execution')
    await expect(detailPane(f.page).getByTestId('execution-run-mode')).toHaveText(
      /Runs all steps on its own/,
      { timeout: 45_000 }
    )
    await f.shots.take('run-all-execution-label')
  })
}

/** A normal approval keeps the per-step behaviour: a Phase approval opens and the Execution tab says it asks first. */
export async function expectAsksBeforeEachStep(f: LiveFlow, r: RequestUnderTest): Promise<void> {
  await f.step('a normal approval still asks before each step', async () => {
    await waitForApiStatus(f.rpc, r.requestId, ['executing'], 60_000)
    await expect
      .poll(
        async () =>
          (await pendingApprovals(f.rpc, r.requestId)).filter((a) => a.subjectType === 'phase')
            .length,
        { timeout: 60_000, message: 'a Phase approval opens for the first step' }
      )
      .toBeGreaterThan(0)
    expect((await requestState(f.rpc, r.requestId)).executionMode).not.toBe('continuous')
    await openDetailTab(f.page, 'Plan')
    await expect(
      detailPane(f.page).locator('[data-testid^="phase-"][data-testid$="-approve"]').first()
    ).toBeVisible({ timeout: 45_000 })
    await openDetailTab(f.page, 'Execution')
    await expect(detailPane(f.page).getByTestId('execution-run-mode')).toHaveText(
      /Asks before each step/,
      { timeout: 45_000 }
    )
    await f.shots.take('step-by-step-execution-label')
  })
}

/** Test setup: the engine settings as stored, to compare after the screen saved and to put back. */
export async function readEngineSettings(f: LiveFlow): Promise<Required<EngineSettings>> {
  const s = await f.rpc.call<EngineSettings>('request.engineSettings', { projectId: f.project.id })
  return {
    engine: s.engine ?? '',
    openspecMinVersion: s.openspecMinVersion ?? '',
    solutionAgent: s.solutionAgent ?? false,
    executionMode: s.executionMode ?? 'step_by_step',
    version: s.version
  }
}

/** Opens the Project Settings dialog of the flow's project (sidebar entry, project switcher, gear) on its General tab. */
export async function openProjectSettings(f: LiveFlow): Promise<Locator> {
  const { page } = f
  await page.getByRole('button', { name: /Project Workspace/ }).click()
  const switcher = page.getByTestId('project-switcher-trigger')
  await expect(switcher).toBeVisible({ timeout: 30_000 })
  if (!(await switcher.innerText()).includes(f.project.name)) {
    await switcher.click()
    await page.getByRole('option', { name: f.project.name }).click()
    await expect(switcher).toContainText(f.project.name)
  }
  await page.getByTestId('project-settings-trigger').click()
  const dialog = page.getByTestId('project-settings-dialog')
  await expect(dialog).toBeVisible()
  await expect(dialog.getByTestId('project-execution-section')).toBeVisible({ timeout: 30_000 })
  return dialog
}

/** Picks a mode in the section and saves; the toast and the stored value say it worked. */
export async function saveModeInProjectSettings(
  f: LiveFlow,
  dialog: Locator,
  label: 'Ask before each step' | 'Run all steps after one approval'
): Promise<void> {
  await dialog.getByRole('radio', { name: label }).click()
  const save = dialog.getByTestId('project-execution-save')
  await expect(save).toBeEnabled()
  await save.click()
  await expect(f.page.getByText('Request execution updated')).toBeVisible({ timeout: 30_000 })
  await expect(save).toBeDisabled()
}

/** "Approve and run all steps" and its confirmation on the Plan tab; the Request must start executing. */
export async function approveAndRunAllThroughTheUi(
  f: LiveFlow,
  r: RequestUnderTest
): Promise<void> {
  await f.step('approve the Plan with "Approve and run all steps" and confirm', async () => {
    await openDetailTab(f.page, 'Plan')
    const steps = await phaseCount(f, r)
    expect(steps, 'the Plan has Phases, so the choice is offered').toBeGreaterThan(0)
    await expect(runAllButton(f.page), 'the Plan tab offers the run-all choice').toBeVisible({
      timeout: 30_000
    })
    await f.shots.take('plan-before-run-all')
    await openRunAllDialogAndRead(f, steps)
    await runAllDialog(f.page).getByRole('button', { name: 'Run all steps' }).click()
    await expect(runAllDialog(f.page)).toBeHidden({ timeout: 45_000 })
    const lag = await expectScreenFollowsService(f.page, f.rpc, f.gaps, {
      requestId: r.requestId,
      title: r.title,
      step: 'AFTER-RUN-ALL-APPROVAL',
      apiStatus: ['executing'],
      chip: /Executing/i,
      apiWaitMs: 60_000
    })
    f.lags.push(`run-all approval to executing ${lag}ms`)
    expect((await requestState(f.rpc, r.requestId)).executionMode, 'pinned by the approval').toBe(
      'continuous'
    )
  })
}
