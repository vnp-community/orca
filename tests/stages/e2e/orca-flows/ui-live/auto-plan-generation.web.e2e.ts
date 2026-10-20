/**
 * The Plan that Orca writes by itself once the Analysis is approved (CR-REQ-044 / FE-REQ-TASK-044-04), through the real UI on a
 * LedgerLite repo (a `bug`, native engine, fake claude).
 *
 *   python3 tests/stages/e2e/orca-flows/run_ui_flow.py --ai-stub -- auto-plan-generation
 *
 * Needs the dev server on the machine that runs the specs: the failures of (b) and (c) are made by a marker file in the fake
 * claude's state directory (run_ui_flow.py passes it as ORCA_E2E_FAKE_CLAUDE_STATE_DIR). The stub is fast, so the writing
 * state is sampled every 200 ms rather than awaited.
 *
 * (a) the Plan appears without a click; (b) a failure that will not fix itself stays on screen across a reload and Retry works;
 * (c) a passing failure is tried again on its own; (d) a gateway without `request.planAuto` and (e) a project with the switch off
 * both keep the Generate button of before.
 */
import { expect, test } from '@playwright/test'
import {
  approveAnalysisAndLeavePlanToOrca,
  armPlanFailure,
  disarmPlanFailure,
  fakeClaudeStateDir,
  planApproveButton,
  planFailedCard,
  planGenerate,
  planRetry,
  planWriting,
  reachAnalysisApproval,
  readPlanProposal,
  setPlanAutoGenerate,
  watchBrowserCalls,
  watchPlanTab
} from './live-auto-plan-steps'
import { configureProject } from './live-continuous-execution-steps'
import { requestState, waitForApiStatus } from './live-gateway-crosschecks'
import { runLedgerliteFlow } from './live-ledgerlite-project-flow'
import { advertiseNoPlanAutoFeature, PLAN_AUTO_FEATURE } from './live-plan-auto-gateway-features'
import {
  cancelOpenRequestThroughTheUi,
  openDetailTab,
  openRequestsPage,
  reloadAndReopen,
  selectRequestRow
} from './live-request-ui-steps'
import { isLiveRun, loadLiveStackConfig } from './live-stack-config'
import { dismissFirstRunTip } from './live-ui-session'

const NATIVE = { engine: 'native', solutionAgent: false, executionMode: 'step_by_step' } as const
const PLAN_WAIT_MS = 120_000

test.describe('the Plan written by Orca after the Analysis is approved (live stack)', () => {
  test.skip(!isLiveRun(), 'live-stack suite: set ORCA_E2E_LIVE=1 (see orca-flows/README.md)')
  // Why: the Plan and its failures come from the fake claude; the failure marker needs its state directory on this machine.
  test.skip(
    () => loadLiveStackConfig().aiMode !== 'stub',
    'this spec drives the fake claude: run it with run_ui_flow.py --ai-stub'
  )
  test.skip(
    () => fakeClaudeStateDir() === '',
    'ORCA_E2E_FAKE_CLAUDE_STATE_DIR is not set: the dev server must run on this machine under run_ui_flow.py --ai-stub'
  )

  test.afterEach(() => disarmPlanFailure())

  test('(a) the Plan appears without a click: writing is shown, Generate never is', async ({
    page,
    context
  }, testInfo) => {
    await runLedgerliteFlow({ page, context, testInfo, slug: 'auto-plan-appears' }, async (f) => {
      const before = await configureProject(f, NATIVE)
      try {
        disarmPlanFailure()
        const { r, asked } = await reachAnalysisApproval(f, 'settings page crash on save')
        const calls = await watchBrowserCalls(f.page)
        await openRequestsPage(f.page)
        await selectRequestRow(f.page, r.title)
        await approveAnalysisAndLeavePlanToOrca(f, r, asked)
        await f.step(
          'the Plan tab shows the writing state, never the Generate button, then the Plan',
          async () => {
            await openDetailTab(f.page, 'Plan')
            const watch = await watchPlanTab(f, planApproveButton(f.page), PLAN_WAIT_MS)
            expect(watch.sawGenerate, 'Generate plan was never offered while Orca works').toBe(
              false
            )
            if (!watch.sawWriting) {
              f.testInfo.annotations.push({
                type: 'note',
                description:
                  'PLAN-WRITING-NOT-SEEN: the Plan was there before the writing state could be sampled; the absence of Generate still holds'
              })
            }
            await expect(planGenerate(f.page)).toHaveCount(0)
            await f.shots.take('plan-written-by-orca')
          }
        )
        await f.step(
          'the service agrees: a Plan awaiting approval, written without a call from the browser',
          async () => {
            const request = await waitForApiStatus(
              f.rpc,
              r.requestId,
              ['awaiting_plan_approval'],
              30_000
            )
            expect(request.planTaskId, 'the Plan exists').toBeTruthy()
            expect(
              calls.count('request.generatePlan'),
              'the browser never asked for the Plan'
            ).toBe(0)
            expect(calls.count('request.retryPlan')).toBe(0)
            const answer = await readPlanProposal(f, r)
            // Why: the job is reported only while the Request is planning, so a finished one may read as empty.
            expect(answer?.autoPlan?.enabled, 'the automatic mode is on').toBe(true)
            expect(['done', ''], 'the job finished').toContain(answer?.autoPlan?.state ?? '')
          }
        )
        await cancelOpenRequestThroughTheUi(f.page, 'auto plan check')
      } finally {
        await configureProject(f, before).catch(() => undefined)
      }
    })
  })

  test('(b) a failure that will not fix itself stays on screen after a reload, and Retry writes the Plan', async ({
    page,
    context
  }, testInfo) => {
    await runLedgerliteFlow({ page, context, testInfo, slug: 'auto-plan-failed' }, async (f) => {
      const before = await configureProject(f, NATIVE)
      try {
        // Two answers: the native engine corrects once before it gives up on an unreadable answer.
        armPlanFailure(2, 'invalid')
        const { r, asked } = await reachAnalysisApproval(f, 'settings page crash on retry')
        const calls = await watchBrowserCalls(f.page)
        await openRequestsPage(f.page)
        await selectRequestRow(f.page, r.title)
        await approveAnalysisAndLeavePlanToOrca(f, r, asked)
        await f.step('the card names the failure and offers Retry', async () => {
          await openDetailTab(f.page, 'Plan')
          await expect(planFailedCard(f.page)).toBeVisible({ timeout: PLAN_WAIT_MS })
          await expect(planFailedCard(f.page)).toContainText('REQUEST_PLAN_AI_INVALID_JSON')
          await expect(
            planGenerate(f.page),
            'the manual way stays, as a second choice'
          ).toContainText('Generate manually')
          await f.shots.take('plan-run-failed-card')
          const answer = await readPlanProposal(f, r)
          expect(answer?.status).toBe('failed')
          expect(answer?.errorCode).toBe('REQUEST_PLAN_AI_INVALID_JSON')
          expect(answer?.autoPlan?.state).toBe('gave_up')
          expect(
            (await requestState(f.rpc, r.requestId)).status,
            'the Request waits in planning'
          ).toBe('planning')
        })
        await f.step('after a reload the card is still there', async () => {
          await reloadAndReopen(f.page, r.title)
          await openDetailTab(f.page, 'Plan')
          await expect(planFailedCard(f.page)).toContainText('REQUEST_PLAN_AI_INVALID_JSON', {
            timeout: 30_000
          })
          await f.shots.take('plan-run-failed-after-reload')
        })
        await f.step(
          'Retry writes the Plan on the server (the fake claude no longer fails)',
          async () => {
            const reloaded = await watchBrowserCalls(f.page)
            await openRequestsPage(f.page)
            await selectRequestRow(f.page, r.title)
            await openDetailTab(f.page, 'Plan')
            await expect(planRetry(f.page)).toBeEnabled({ timeout: 30_000 })
            await planRetry(f.page).click()
            const watch = await watchPlanTab(f, planApproveButton(f.page), PLAN_WAIT_MS)
            expect(watch.sawGenerate, 'Generate plan is not offered while the retry runs').toBe(
              false
            )
            expect(reloaded.count('request.retryPlan'), 'Retry went through the server').toBe(1)
            expect(reloaded.count('request.generatePlan'), 'and not through the browser').toBe(0)
            expect(calls.count('request.generatePlan')).toBe(0)
            await waitForApiStatus(f.rpc, r.requestId, ['awaiting_plan_approval'], 30_000)
          }
        )
        await cancelOpenRequestThroughTheUi(f.page, 'auto plan check')
      } finally {
        await configureProject(f, before).catch(() => undefined)
      }
    })
  })

  test('(c) a failure that may pass is tried again by Orca and the Plan still appears', async ({
    page,
    context
  }, testInfo) => {
    await runLedgerliteFlow({ page, context, testInfo, slug: 'auto-plan-transient' }, async (f) => {
      const before = await configureProject(f, NATIVE)
      try {
        armPlanFailure(1, 'error')
        const { r, asked } = await reachAnalysisApproval(f, 'settings page crash twice')
        await openRequestsPage(f.page)
        await selectRequestRow(f.page, r.title)
        await approveAnalysisAndLeavePlanToOrca(f, r, asked)
        await f.step('the second attempt writes the Plan without a click', async () => {
          await openDetailTab(f.page, 'Plan')
          const watch = await watchPlanTab(f, planApproveButton(f.page), PLAN_WAIT_MS)
          expect(watch.sawGenerate, 'Generate plan was never offered').toBe(false)
          expect(watch.sawFailedCard, 'a failure that Orca retries is not shown as a failure').toBe(
            false
          )
          if (watch.attempts.length === 0) {
            f.testInfo.annotations.push({
              type: 'note',
              description:
                'PLAN-ATTEMPT-NOT-SEEN: the "Attempt 2 of 2" line was not sampled; the Plan below still shows the second attempt worked'
            })
          } else {
            expect(watch.attempts.join(' | ')).toContain('Attempt 2 of 2')
          }
          await f.shots.take('plan-after-retry-by-orca')
        })
        await f.step('the Plan is there and it took a second attempt', async () => {
          const request = await waitForApiStatus(
            f.rpc,
            r.requestId,
            ['awaiting_plan_approval'],
            30_000
          )
          expect(request.planTaskId).toBeTruthy()
        })
        await cancelOpenRequestThroughTheUi(f.page, 'auto plan check')
      } finally {
        await configureProject(f, before).catch(() => undefined)
      }
    })
  })

  test('(d, e) with the project switch off, or a gateway without request.planAuto, Generate plan works as before', async ({
    page,
    context
  }, testInfo) => {
    await runLedgerliteFlow({ page, context, testInfo, slug: 'auto-plan-off' }, async (f) => {
      const before = await configureProject(f, NATIVE)
      const switchBefore = await setPlanAutoGenerate(f, false)
      try {
        const { r, asked } = await reachAnalysisApproval(f, 'settings page crash switch off')
        const calls = await watchBrowserCalls(f.page)
        await openRequestsPage(f.page)
        await selectRequestRow(f.page, r.title)
        await approveAnalysisAndLeavePlanToOrca(f, r, asked)
        await f.step(
          '(e) switch off: Request waits in planning with the Generate button, nothing is written',
          async () => {
            await openDetailTab(f.page, 'Plan')
            await expect(planGenerate(f.page)).toContainText('Generate plan', { timeout: 30_000 })
            await expect(planWriting(f.page)).toHaveCount(0)
            await f.page.waitForTimeout(9_000)
            await expect(
              planGenerate(f.page),
              'still the button after the start grace period'
            ).toBeVisible()
            await expect(planWriting(f.page)).toHaveCount(0)
            const answer = await readPlanProposal(f, r)
            expect(answer?.autoPlan?.enabled, 'the service reports the mode off').toBe(false)
            expect((await requestState(f.rpc, r.requestId)).status).toBe('planning')
            await f.shots.take('plan-generate-project-off')
          }
        )
        await f.step(
          `(d) a window whose gateway lacks "${PLAN_AUTO_FEATURE}" shows the same, and the click writes the Plan`,
          async () => {
            const older = await f.page.context().newPage()
            try {
              const stripper = await advertiseNoPlanAutoFeature(older)
              const olderCalls = await watchBrowserCalls(older, { reload: false })
              await older.goto('/')
              await dismissFirstRunTip(older)
              await openRequestsPage(older)
              await selectRequestRow(older, r.title)
              await openDetailTab(older, 'Plan')
              await expect(planGenerate(older)).toContainText('Generate plan', { timeout: 30_000 })
              await expect
                .poll(() => stripper.strippedAnswers(), {
                  message: 'the SPA was told the feature is missing'
                })
                .toBeGreaterThan(0)
              await older.screenshot({ path: f.testInfo.outputPath('plan-generate-flag-off.png') })
              await planGenerate(older).click()
              await expect(planApproveButton(older)).toBeVisible({ timeout: PLAN_WAIT_MS })
              expect(
                olderCalls.count('request.generatePlan'),
                'the manual flow asked for the Plan'
              ).toBeGreaterThan(0)
              expect(olderCalls.count('request.retryPlan')).toBe(0)
              await waitForApiStatus(f.rpc, r.requestId, ['awaiting_plan_approval'], 30_000)
            } finally {
              await older.close()
            }
            expect(calls.count('request.generatePlan'), 'the first window never asked').toBe(0)
          }
        )
        await cancelOpenRequestThroughTheUi(f.page, 'auto plan check')
      } finally {
        await setPlanAutoGenerate(f, switchBefore).catch(() => undefined)
        await configureProject(f, before).catch(() => undefined)
      }
    })
  })
})
