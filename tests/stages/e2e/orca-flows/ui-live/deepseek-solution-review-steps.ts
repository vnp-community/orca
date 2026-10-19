/**
 * Steps of the review of a REAL Solution on the user's repo (TASK-FE-E2E-028): open it, reject it with the keyboard, bring the Request
 * back from the backlog, ask for the new Solution, and choose an option. Written against what the stack really does, which differs from
 * the old mock in three places: a rejected Solution sends the Request to the BACKLOG (not back to analysis), Reopen restarts at the type
 * confirmation, and the service refuses an option that does not answer an acceptance criterion.
 */
import { expect, type Locator } from '@playwright/test'
import type { DeepseekRun } from './deepseek-flow-harness'
import { callsTo } from './live-add-project-own-project-steps'
import type { LiveFlow } from './live-flow-harness'
import { answerClarificationIfAsked, type RequestUnderTest } from './live-flow-steps'
import { requestState, waitForApiStatus } from './live-gateway-crosschecks'
import {
  confirmTypeInCard,
  detailPane,
  openDetailTab,
  solutionApproveButton
} from './live-request-ui-steps'
import { expectScreenFollowsService } from './live-status-sync'

type Run = { status: string; errorCode?: string; startedAt?: string }

/** solution.list answers newest first; the run to judge is the one that started last, whatever the order. */
function latestRun(runs: Run[]): Run | undefined {
  return runs.toSorted((a, b) => (b.startedAt ?? '').localeCompare(a.startedAt ?? ''))[0]
}

export type ApproveOutcome =
  | 'accepted'
  | 'refused-then-recommended'
  | 'high-risk-confirmed'
  | 'high-risk-unreachable-then-recommended'

export const solutionOptions = (f: LiveFlow): Locator =>
  detailPane(f.page).getByRole('radiogroup', { name: 'Options' }).getByRole('radio')

/** The Analysis tab with a Solution ready to review (waits for the automatic run; presses "Generate" only if it never starts). */
export async function openSolution(f: LiveFlow, r: RequestUnderTest): Promise<void> {
  await openDetailTab(f.page, 'Analysis')
  const approve = solutionApproveButton(f.page)
  const generate = detailPane(f.page).getByTestId('solution-generate')
  await expect(
    approve.or(generate).or(detailPane(f.page).getByTestId('solution-generation-pending')).first()
  ).toBeVisible({ timeout: 60_000 })
  if (await generate.isVisible()) {
    // Why: the prompt shows for a moment before the automatic start creates the run; click only if it stays.
    const byItself = await expect(generate)
      .toBeHidden({ timeout: 20_000 })
      .then(
        () => true,
        () => false
      )
    if (!byItself) {
      await generate.click()
    }
  }
  await expect(approve, 'the Solution is ready to review').toBeVisible({
    timeout: f.budgets.solutionMs
  })
  await expectScreenFollowsService(f.page, f.rpc, f.gaps, {
    requestId: r.requestId,
    title: r.title,
    step: 'SOLUTION-READY',
    apiStatus: ['awaiting_analysis_approval'],
    chip: /Awaiting Analysis Approval/i,
    apiWaitMs: f.budgets.solutionMs
  })
}

/** Reject dialog: a short reason keeps Reject locked, plain Enter does not submit, Mod+Enter with a valid reason does. */
export async function rejectWithModEnter(
  f: LiveFlow,
  run: DeepseekRun,
  tapFrom: number
): Promise<string> {
  const { page } = f
  await detailPane(page).getByRole('button', { name: 'Reject' }).first().click()
  const dialog = page.getByRole('dialog')
  const submit = dialog.getByRole('button', { name: 'Reject' })
  const box = dialog.getByRole('textbox')
  await box.fill('too short')
  await expect(submit, 'a short reason keeps Reject locked').toBeDisabled()
  const reason = `${f.cfg.runPrefix}: cần nêu rõ chi phí di chuyển của từng phương án`
  await box.fill(reason)
  await expect(submit).toBeEnabled()
  await box.press('Enter')
  expect(callsTo(run.tap, 'approval.reject', tapFrom), 'plain Enter only adds a line').toHaveLength(
    0
  )
  // Why: the app picks the modifier from the page's userAgent, not from the host OS that ControlOrMeta follows.
  const pageIsMac = await page.evaluate(() => navigator.userAgent.includes('Mac'))
  await box.press(pageIsMac ? 'Meta+Enter' : 'Control+Enter')
  await expect(dialog).toHaveCount(0, { timeout: 20_000 })
  return reason
}

/** After a rejection: the Request is in the backlog with the reason on screen; Reopen restarts it at the type confirmation. */
export async function reopenFromBacklog(
  f: LiveFlow,
  r: RequestUnderTest,
  reason: string
): Promise<void> {
  await waitForApiStatus(f.rpc, r.requestId, ['request_backlog'], 30_000)
  await expect(
    detailPane(f.page).getByText('Returned to backlog from Analysis'),
    'the banner says where the Request went and why'
  ).toBeVisible({ timeout: 45_000 })
  await expect(detailPane(f.page)).toContainText(reason)
  await f.shots.take('returned-to-backlog')
  await detailPane(f.page).getByRole('button', { name: 'Reopen' }).last().click()
  await waitForApiStatus(f.rpc, r.requestId, ['awaiting_type_confirmation'], 30_000)
  await confirmTypeInCard(f.page, 'Change request', 'S')
  await waitForApiStatus(
    f.rpc,
    r.requestId,
    ['awaiting_information', 'analyzing', 'awaiting_analysis_approval'],
    30_000
  )
  await answerClarificationIfAsked(f, r)
}

/** The rejected Solution stays on the tab with "Regenerate"; it asks for a new one, and a new Solution (not the old id) appears. */
export async function regenerateSolution(
  f: LiveFlow,
  r: RequestUnderTest,
  oldSolutionId: string
): Promise<void> {
  await openDetailTab(f.page, 'Analysis')
  const regenerate = detailPane(f.page).getByRole('button', { name: 'Regenerate' })
  await expect(regenerate, 'the rejected Solution offers "Regenerate"').toBeVisible({
    timeout: 60_000
  })
  // Why: a real model can time out (REQUEST_SOLUTION_AI_TIMEOUT); that is the AI's, not the screen's, so ask again, at most twice.
  for (let attempt = 0; ; attempt++) {
    await regenerate.click()
    const result = await expect
      .poll(
        async () => {
          const listed = await f.rpc.call<{
            solutions?: { id: string; status: string }[]
            runs?: Run[]
          }>('solution.list', { requestId: r.requestId })
          if (
            (listed.solutions ?? []).some(
              (x) => x.id !== oldSolutionId && x.status !== 'superseded'
            )
          ) {
            return 'new'
          }
          const last = latestRun(listed.runs ?? [])
          return last?.status === 'failed' ? `failed:${last.errorCode ?? ''}` : 'waiting'
        },
        { timeout: f.budgets.solutionMs, message: 'a regenerated Solution appears' }
      )
      .not.toBe('waiting')
      .then(async () => {
        const listed = await f.rpc.call<{
          solutions?: { id: string; status: string }[]
          runs?: Run[]
        }>('solution.list', { requestId: r.requestId })
        return (listed.solutions ?? []).some(
          (x) => x.id !== oldSolutionId && x.status !== 'superseded'
        )
          ? 'new'
          : `failed:${latestRun(listed.runs ?? [])?.errorCode ?? ''}`
      })
    if (result === 'new') {
      break
    }
    expect(result, 'only an AI timeout is worth asking again').toBe(
      'failed:REQUEST_SOLUTION_AI_TIMEOUT'
    )
    expect(attempt, 'the AI timed out three times in a row').toBeLessThan(2)
    f.testInfo.annotations.push({
      type: 'ai-timeout-retry',
      description: `regenerate attempt ${attempt + 1} timed out; asked again`
    })
    await expect(regenerate, 'a failed run offers Regenerate again').toBeVisible({
      timeout: 30_000
    })
  }
  await openSolution(f, r)
  await f.shots.take('solution-regenerated')
}

/**
 * Chooses a non-recommended option (with its rationale) and approves. The service may refuse it because the option does not answer an
 * acceptance criterion; then the screen must say why, and the recommended option goes through. Returns what happened.
 */
export async function chooseAndApprove(
  f: LiveFlow,
  run: DeepseekRun,
  r: RequestUnderTest
): Promise<ApproveOutcome> {
  const options = solutionOptions(f)
  const approve = solutionApproveButton(f.page)
  const from = run.tap.calls.length
  // Why: Compare switches the Analysis tab to the table; the cards (the radios that choose an option) come back with "Cards".
  const cards = detailPane(f.page).getByRole('button', { name: 'Cards' })
  if (await cards.isVisible()) {
    await cards.click()
  }
  await expect(options.first(), 'the option cards are shown').toBeVisible({ timeout: 30_000 })
  if ((await options.count()) >= 2) {
    await options.filter({ hasNotText: 'Recommended' }).first().click()
    await expect(approve, 'a non-recommended option needs a rationale first').toBeDisabled()
    await detailPane(f.page)
      .getByRole('textbox', { name: 'Reason for choosing' })
      .fill('Dễ bảo trì hơn về lâu dài')
  } else {
    await options.first().click()
  }
  await expect(approve).toBeEnabled()
  await approve.click()
  const refusedByService = (): boolean =>
    f.noise.rpcErrors.some((e) => /^approval\.approve .*AC_UNCOVERED_BY_OPTION/.test(e))
  // Why: three real endings: accepted; refused because the option does not answer an AC; or a HIGH-RISK option, where the screen opens a
  // dialog to retype the option name (the approval is refused until then). Each is handled the way a user would.
  const riskDialog = f.page.getByTestId('high-risk-decision-dialog')
  // Why: the high-risk dialog needs the Decision, which the api-gateway does not serve yet (decision.* is on the contract allowlist of
  // channels it lacks), so the UI never opens it; the option can then not be approved from the web: a known gap, then the recommended one.
  const decisionUnreachable = (): boolean =>
    f.noise.rpcErrors.some((e) => /^approval\.approve .*REQUEST_DECISION_NOT_EFFECTIVE/.test(e))
  const settled = async (): Promise<
    'planning' | 'refused' | 'high-risk' | 'unreachable' | 'waiting'
  > => {
    if ((await requestState(f.rpc, r.requestId)).status === 'planning') {
      return 'planning'
    }
    if (refusedByService()) {
      return 'refused'
    }
    if (await riskDialog.isVisible()) {
      return 'high-risk'
    }
    return decisionUnreachable() ? 'unreachable' : 'waiting'
  }
  await expect
    .poll(settled, {
      timeout: 60_000,
      message: 'the approval is accepted, refused or asks to confirm a risk'
    })
    .not.toBe('waiting')
  const first = await settled()
  let outcome: ApproveOutcome = 'accepted'
  if (first === 'high-risk') {
    outcome = 'high-risk-confirmed'
    const label = (await riskDialog.locator('label').innerText()).trim()
    const title = /"(.*)"/.exec(label)?.[1] ?? ''
    expect(title, 'the dialog names the option to retype').not.toBe('')
    await f.shots.take('high-risk-dialog')
    await riskDialog.getByRole('textbox').fill(title)
    await riskDialog.getByRole('button', { name: 'Confirm choice' }).click()
    await expect(riskDialog).toHaveCount(0, { timeout: 30_000 })
  } else if (first === 'unreachable') {
    outcome = 'high-risk-unreachable-then-recommended'
    f.testInfo.annotations.push({
      type: 'known-gap',
      description:
        'a high-risk option cannot be confirmed from the web: approval.approve is refused with REQUEST_DECISION_NOT_EFFECTIVE and no decision.* channel in the api-gateway lets the dialog open (the screen says "This was already decided.")'
    })
    console.log('[known-gap] high-risk decision confirmation is unreachable through the gateway')
    await f.shots.take('high-risk-unreachable')
    await options.filter({ hasText: 'Recommended' }).first().click()
    await expect(approve).toBeEnabled()
    await approve.click()
  } else if (first === 'refused') {
    outcome = 'refused-then-recommended'
    const shown = await expect(detailPane(f.page))
      .toContainText(/does not answer|coverage|acceptance/i, { timeout: 10_000 })
      .then(
        () => true,
        () => false
      )
    if (!shown) {
      f.gaps.record(
        'UI-APPROVE-REFUSAL-SILENT',
        '[frontend] approval.approve was refused with REQUEST_ARTIFACT_AC_UNCOVERED_BY_OPTION and the Analysis tab shows no reason'
      )
    }
    await f.shots.take('approve-refused')
    await options.filter({ hasText: 'Recommended' }).first().click()
    await expect(approve).toBeEnabled()
    await approve.click()
  }
  await waitForApiStatus(f.rpc, r.requestId, ['planning'], 60_000)
  const order = run.tap.calls
    .slice(from)
    .map((c) => c.method)
    .filter((m) => m === 'solution.choose' || m === 'approval.approve')
  expect(order.slice(0, 2), 'solution.choose goes before approval.approve').toEqual([
    'solution.choose',
    'approval.approve'
  ])
  return outcome
}
