/**
 * A plan that runs on its own, with the REAL Claude Code of the dev server, through the UI (CR-REQ-038 + FE-REQ-037 + CR-REQ-043).
 * The project stays `step_by_step`; the Plan tab offers "Approve and run all steps", the Plan is approved once with that choice
 * and its confirmation, every step runs on its own with no Phase approval and no further click, the Execution tab follows the
 * run, and the Request completes. Safety net: real-flow-harness.ts.
 *
 *   python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real continuous
 */
import { expect, test } from '@playwright/test'
import {
  answerClarificationIfAsked,
  createClassifyAndConfirm,
  generatePlanThroughTheUi
} from './live-flow-steps'
import { approveAnalysis } from './live-analysis-approval-steps'
import { reviewPlanBeforeApproving } from './live-execution-steps'
import {
  expectFinishedRunScreens,
  followContinuousRun,
  configureProject
} from './live-continuous-execution-steps'
import { approveAndRunAllThroughTheUi, expectBothApproveButtons } from './live-run-all-steps'
import { expectExpectedChangeInFixture } from './live-fixture-change-checks'
import { isLiveRun } from './live-stack-config'
import { realRunBudgetMs, runRealFlow } from './real-flow-harness'

test.describe('A plan that runs on its own, with the real Claude, through the UI', () => {
  test.skip(!isLiveRun(), 'live-stack suite: set ORCA_E2E_LIVE=1 (see orca-flows/README.md)')

  test('change_request (native engine): "Approve and run all steps" once, every step runs on its own, the Request completes', async ({
    page,
    context
  }, testInfo) => {
    await runRealFlow(
      { page, context, testInfo, slug: 'real-continuous', mode: 'execute' },
      async (f, fx) => {
        const headBefore = fx.headSha()
        // Why: the deployment default (openspec, agent-written Solution) needs the project's tdd-agent knowledge hub
        // (.tdd-agent/config.toml); this throwaway repo has none, so the run uses the native engine. The OpenSpec screens
        // (Handover row, Verify) are checked on the user's knowledge-hub project instead.
        const before = await configureProject(f, {
          engine: 'native',
          solutionAgent: false,
          executionMode: 'step_by_step'
        })
        try {
          const r = await createClassifyAndConfirm(f, {
            titleWords: 'add add and subtract',
            body:
              'Work in TWO steps (two phases). Step 1: add a function add(a, b) to src/calc.py that returns the sum, with one test in ' +
              'tests/test_calc.py. Step 2: add a function subtract(a, b) to src/calc.py that returns a minus b, with one test in ' +
              'tests/test_calc.py. One working task per step, no inspection or review task. Change nothing else.',
            typeLabel: 'Change request',
            typeKey: 'change_request',
            size: 'M'
          })
          const asked = await answerClarificationIfAsked(f, r)
          await approveAnalysis(f, r, {
            autoStarted: asked,
            next: { api: 'planning', chip: /Planning/i }
          })
          await generatePlanThroughTheUi(f, r)
          await reviewPlanBeforeApproving(f, r)
          await expectBothApproveButtons(f)
          await approveAndRunAllThroughTheUi(f, r)
          const run = await followContinuousRun(f, r, realRunBudgetMs('execute'), fx.base)
          expect(run.sawStrip, 'the Execution tab showed the summary strip').toBe(true)
          expect(run.sawRunning, 'the Execution tab showed the run as running').toBe(true)
          await expectFinishedRunScreens(f, r)
          await expectExpectedChangeInFixture(f, fx, 'add')
          expect(fx.headSha(), 'the main checkout of the fixture kept its commit').toBe(headBefore)
          expect(fx.guardDenials(), 'the agent never tried to leave the fixture').toEqual([])
        } finally {
          // Why: leave the project as it was found.
          await configureProject(f, before).catch(() => undefined)
        }
      }
    )
  })
})
