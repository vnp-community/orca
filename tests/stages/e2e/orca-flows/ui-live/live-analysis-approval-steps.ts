/**
 * The Analysis tab step of the Request flow: the analysis (Solution, diagnosis, findings or answer) is chosen and approved, with
 * the "Ask back" round trip when the AI left a blocking question. Moved out of live-flow-steps.ts to keep both under the line budget.
 */
import { expect } from '@playwright/test'
import type { LiveFlow } from './live-flow-harness'
import type { RequestUnderTest } from './live-flow-steps'
import { requestState } from './live-gateway-crosschecks'
import {
  answerClarificationInPanel,
  chooseAndApproveSolutionOption,
  detailPane,
  openDetailTab,
  reloadAndReopen,
  solutionApproveButton
} from './live-request-ui-steps'
import { expectScreenFollowsService, UI_REFRESH_GRACE_MS } from './live-status-sync'

const AUTO_START_GRACE_MS = 20_000

const MAX_ASK_BACKS = 2

/** "Ask back" for the blocking questions, answer the Clarification it opens, and wait for the Request to resume analysis. */
async function askBackAndAnswer(
  f: LiveFlow,
  r: RequestUnderTest,
  round: number,
  seen: Set<string>,
  answer?: (f: LiveFlow) => Promise<boolean>
): Promise<void> {
  const { rpc, page, shots, gaps } = f
  await f.step(`ask back the blocking question(s) (round ${round})`, async () => {
    await detailPane(page).getByTestId('solution-ask-back').first().click()
    const dialog = page.getByTestId('solution-ask-back-dialog')
    await expect(dialog).toBeVisible()
    const boxes = dialog.getByRole('checkbox')
    for (let i = 0; i < (await boxes.count()); i++) {
      if ((await boxes.nth(i).getAttribute('aria-checked')) !== 'true') {
        await boxes.nth(i).click()
      }
    }
    const reasons = dialog.getByTestId('solution-ask-back-reason')
    for (let i = 0; i < (await reasons.count()); i++) {
      await reasons.nth(i).fill('We need this decided before an option can be chosen.')
    }
    await shots.take(`ask-back-dialog-${round}`)
    await dialog.getByTestId('solution-ask-back-submit').click()
    if (
      !(await expect(dialog)
        .toBeHidden({ timeout: 20_000 })
        .then(
          () => true,
          () => false
        ))
    ) {
      await shots.take(`ask-back-refused-${round}`)
      const shown = (
        await dialog
          .getByTestId('solution-ask-back-error')
          .innerText()
          .catch(() => '')
      ).replace(/\s+/g, ' ')
      const refused = f.noise.rpcErrors
        .filter((x) => x.startsWith('clarification.request'))
        .slice(-1)
        .join('')
      throw new Error(
        `"Send to reporter" did not close the dialog (screen says "${shown}"; service answered: ${refused || 'nothing seen'})`
      )
    }
    await expectScreenFollowsService(page, rpc, gaps, {
      requestId: r.requestId,
      title: r.title,
      step: `ASK-BACK-${round}`,
      apiStatus: ['awaiting_information'],
      chip: /Awaiting Information/i,
      apiWaitMs: 30_000
    })
    expect(
      await (answer?.(f) ??
        answerClarificationInPanel(page, `${f.cfg.runPrefix} ask-back answer ${round}`)),
      'the ask-back Clarification can be submitted'
    ).toBe(true)
    await expectScreenFollowsService(page, rpc, gaps, {
      requestId: r.requestId,
      title: r.title,
      step: `AFTER-ASK-BACK-${round}`,
      apiStatus: ['analyzing', 'awaiting_analysis_approval'],
      chip: /Analyzing|Awaiting Analysis Approval/i,
      apiWaitMs: 30_000
    })
    // Why: the Request can look "awaiting approval" for a moment while the answer is still retiring the old Solution.
    const deadline = Date.now() + f.budgets.solutionMs
    for (;;) {
      const now = await rpc.call<{ solutions?: { id: string; status: string }[] }>(
        'solution.list',
        { requestId: r.requestId }
      )
      const fresh = (now.solutions ?? []).find((x) => x.status !== 'superseded' && !seen.has(x.id))
      if (fresh) {
        break
      }
      expect(Date.now(), 'a regenerated Solution appears after the answer').toBeLessThan(deadline)
      await page.waitForTimeout(1_500)
    }
    await expectScreenFollowsService(page, rpc, gaps, {
      requestId: r.requestId,
      title: r.title,
      step: `NEW-SOLUTION-${round}`,
      apiStatus: ['awaiting_analysis_approval'],
      chip: /Awaiting Analysis Approval/i,
      apiWaitMs: f.budgets.solutionMs
    })
    await shots.take(`ask-back-answered-${round}`)
  })
}

/**
 * Solution/analysis reaches the screen, is approved. `autoStarted`: the run must begin by itself (after a Clarification);
 * otherwise the "Generate solution" button is the way in and is pressed. When the Solution has a blocking question (or the
 * service refuses the approval because of one) the user asks back, answers, and chooses again, at most MAX_ASK_BACKS times.
 */
export async function approveAnalysis(
  f: LiveFlow,
  r: RequestUnderTest,
  o: {
    autoStarted: boolean
    next: { api: string; chip: RegExp }
    expectAskBacks?: number
    /** Fills and submits the Clarification that an ask-back opens; default is a placeholder sentence (specs that read the questions pass their own). */
    answerAskBack?: (f: LiveFlow) => Promise<boolean>
  }
): Promise<{ askBacks: number }> {
  const { rpc, page, shots, gaps, lags } = f
  let askBacks = 0
  let firstSolutionId = ''
  const seenSolutionIds = new Set<string>()
  let autoStarted = o.autoStarted
  await f.step('the analysis reaches the Analysis tab and is approved', async () => {
    for (;;) {
      await openDetailTab(page, 'Analysis')
      const start = detailPane(page).getByTestId('solution-generate')
      const ready = solutionApproveButton(page)
      await expect(
        start.or(ready).or(detailPane(page).getByTestId('solution-generation-pending')).first()
      ).toBeVisible({ timeout: 30_000 })
      if (await start.isVisible()) {
        // Why: the automatic start is event driven, so the prompt can show for a moment before the run row exists.
        const startedByItself =
          autoStarted &&
          (await expect(start)
            .toBeHidden({ timeout: AUTO_START_GRACE_MS })
            .then(
              () => true,
              () => false
            ))
        if (!startedByItself) {
          if (autoStarted) {
            gaps.record(
              'SOLUTION-NOT-AUTO-STARTED',
              `${AUTO_START_GRACE_MS / 1000}s after the Clarification the Request is analyzing with no run; the "Generate solution" button was pressed instead`
            )
          }
          await shots.take('generate-solution-button')
          await start.click()
        }
      }
      // Why: a failed run (e.g. REQUEST_ANALYSIS_NO_REPO_PATH) must fail here with the screen's own words, not after a long status wait.
      const failed = detailPane(page).getByTestId('solution-generation-failed')
      await expect(ready.or(failed).or(detailPane(page).getByRole('radio')).first()).toBeVisible({
        timeout: f.budgets.solutionMs
      })
      if (await failed.isVisible()) {
        await shots.take('analysis-failed')
        const status = await requestState(rpc, r.requestId)
        throw new Error(
          `the analysis run failed on screen (${(await failed.innerText()).replace(/\s+/g, ' ').slice(0, 200)}); service status ${status.status}`
        )
      }
      const lag = await expectScreenFollowsService(page, rpc, gaps, {
        requestId: r.requestId,
        title: r.title,
        step: 'AFTER-SOLUTION-READY',
        apiStatus: ['awaiting_analysis_approval'],
        chip: /Awaiting Analysis Approval/i,
        apiWaitMs: f.budgets.solutionMs
      })
      lags.push(`analysis ready ${lag}ms`)
      await openDetailTab(page, 'Analysis')
      await expect(ready, 'the approve bar appears without a reload').toBeVisible({
        timeout: UI_REFRESH_GRACE_MS
      })
      const solutions = await rpc.call<{
        solutions?: {
          id: string
          status: string
          options?: { open_questions?: { blocking?: boolean }[] }
        }[]
      }>('solution.list', { requestId: r.requestId })
      firstSolutionId ||= solutions.solutions?.[0]?.id ?? ''
      // Why: right after an ask-back the old Solution can still be on screen; what the screen shows must match the live Solution.
      const live = (solutions.solutions ?? []).toReversed().find((x) => x.status !== 'superseded')
      const serviceBlocking = (live?.options?.open_questions ?? []).filter((q) => q.blocking).length
      const blockingOnScreen = detailPane(page).getByTestId('solution-blocking-question')
      const matches = await expect
        .poll(async () => (await blockingOnScreen.count()) > 0 === serviceBlocking > 0, {
          timeout: UI_REFRESH_GRACE_MS
        })
        .toBe(true)
        .then(
          () => true,
          () => false
        )
      for (const x of solutions.solutions ?? []) {
        seenSolutionIds.add(x.id)
      }
      if (!matches) {
        gaps.record(
          'UI-STALE-SOLUTION',
          `the service's live Solution has ${serviceBlocking} blocking question(s) but the screen shows ${await blockingOnScreen.count()} (the superseded Solution is still displayed); reloaded`
        )
        await reloadAndReopen(page, r.title)
        await openDetailTab(page, 'Analysis')
        await expect(ready).toBeVisible({ timeout: UI_REFRESH_GRACE_MS })
      }
      await shots.take('solution-options')

      let needAskBack = (await blockingOnScreen.count()) > 0
      if (!needAskBack) {
        await chooseAndApproveSolutionOption(page)
        const refusedNotice = detailPane(page).locator(
          '[data-testid="solution-blocking-notice"][data-refused="true"]'
        )
        const deadline = Date.now() + 30_000
        while (!needAskBack && Date.now() < deadline) {
          if ((await requestState(rpc, r.requestId)).status === o.next.api) {
            break
          }
          needAskBack = await refusedNotice.isVisible()
          await page.waitForTimeout(1_000)
        }
      }
      if (!needAskBack) {
        break
      }
      const seen = (solutions.solutions ?? [])
        .map(
          (x) =>
            `${x.id.slice(0, 8)}:${x.status}:${(x.options?.open_questions ?? []).filter((q) => q.blocking).length}q`
        )
        .join(', ')
      expect(
        askBacks,
        `the AI asked a blocking question again after ${MAX_ASK_BACKS} ask-backs (solutions: ${seen})`
      ).toBeLessThan(MAX_ASK_BACKS)
      askBacks++
      autoStarted = true // the answer restarts the analysis by itself
      await askBackAndAnswer(f, r, askBacks, seenSolutionIds, o.answerAskBack)
    }
    try {
      await expectScreenFollowsService(page, rpc, gaps, {
        requestId: r.requestId,
        title: r.title,
        step: 'AFTER-SOLUTION-APPROVAL',
        apiStatus: [o.next.api],
        chip: o.next.chip,
        apiWaitMs: 30_000
      })
    } catch (e) {
      // Why: the screen's alert alone hides the real refusal, so name what the gateway answered.
      const refused = f.noise.rpcErrors
        .filter((x) => /^(approval|solution|decision)\./.test(x))
        .slice(-2)
        .join(' | ')
      throw new Error(
        `${(e as Error).message}; refused by the service: ${refused || 'nothing seen'}`
      )
    }
    if (askBacks > 0) {
      // The reporter's answer retires the questioned Solution; the approved one is a new row.
      const after = await rpc.call<{ solutions?: { id: string; status: string }[] }>(
        'solution.list',
        { requestId: r.requestId }
      )
      const old = after.solutions?.find((x) => x.id === firstSolutionId)
      expect(after.solutions?.length ?? 0, 'ask-back produced new Solution(s)').toBeGreaterThan(
        askBacks
      )
      expect(old?.status, 'the questioned Solution is superseded').toBe('superseded')
    }
    lags.push(`ask-back rounds ${askBacks}`)
    console.log(`[live-flow] ask-back rounds: ${askBacks}`)
    if (o.expectAskBacks !== undefined) {
      expect(askBacks, 'ask-back rounds').toBe(o.expectAskBacks)
    }
    await shots.take('solution-approved')
  })
  return { askBacks }
}
