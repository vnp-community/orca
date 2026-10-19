/**
 * Watching the agent from the web while the REAL Claude Code executes a Plan (TASK-FE-E2E-011). A one-task Plan on a throwaway
 * repo (same safety net as execute-real) is approved on screen; from then on the spec looks at what a user has for following
 * the work: the task sheet opened from the Plan tab, the dev server's status in Settings, the agent's live output, and the
 * session afterwards (reload, trace of what was changed, "Open in Git"). The rules are in live-agent-session-judge.ts.
 *
 *   python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real monitor
 *
 * A screen that does not exist, or does not receive the data the backend sends, is a product gap and fails the run with its
 * code; it is never relaxed to turn the run green (ORCA_E2E_LIVE_GAPS=annotate only records them).
 */
import { expect, test } from '@playwright/test'
import { expectExpectedChangeInFixture, worktreeChangeState } from './live-fixture-change-checks'
import { AgentSessionMonitor } from './live-agent-session-monitor'
import {
  answerClarificationIfAsked,
  createClassifyAndConfirm,
  generatePlanThroughTheUi
} from './live-flow-steps'
import {
  approvePlanThroughTheUi,
  driveExecutionToTheEnd,
  expectPlanTabFinished,
  reviewPlanBeforeApproving
} from './live-execution-steps'
import { isLiveRun } from './live-stack-config'
import { TaskActivityTap } from './live-task-activity-tap'
import { realRunBudgetMs, runRealFlow } from './real-flow-harness'

test.describe('Following the real agent from the web UI', () => {
  test.skip(!isLiveRun(), 'live-stack suite: set ORCA_E2E_LIVE=1 (see orca-flows/README.md)')

  test('task, single step: the running agent, its dev server and its session are visible in the UI', async ({
    page,
    context
  }, testInfo) => {
    await runRealFlow(
      { page, context, testInfo, slug: 'real-monitor', mode: 'execute' },
      async (f, fx) => {
        const headBefore = fx.headSha()
        const r = await createClassifyAndConfirm(f, {
          titleWords: 'add a negate function',
          body:
            'Add a function negate(a) to src/calc.py that returns minus a, and one test for it in ' +
            'tests/test_calc.py. This is a single step: plan exactly ONE task that does both, with no inspection, ' +
            'verification or review task and no dependencies. Change nothing else.',
          typeLabel: 'Task',
          typeKey: 'task',
          size: 'S'
        })
        await answerClarificationIfAsked(f, r, { api: ['planning'], chip: /Planning/i })
        await generatePlanThroughTheUi(f, r)
        const plan = await reviewPlanBeforeApproving(f, r)
        const leaves = plan.filter((t) => !['plan', 'phase'].includes(t.type))
        // Why: subscribe before approving so the first output frame cannot be missed.
        const tap = await TaskActivityTap.open(
          f.rpc,
          leaves.map((t) => t.id)
        )
        const monitor = new AgentSessionMonitor(f, context, r, tap, fx.base)
        try {
          await monitor.prepare()
          await approvePlanThroughTheUi(f, r)
          await monitor.watchTask(leaves[0])
          const report = await driveExecutionToTheEnd(f, r, realRunBudgetMs('execute'), fx.base)
          console.log(`[exec] transitions:\n${report.seen.join('\n')}`)
          await expectPlanTabFinished(f, r)
          await expectExpectedChangeInFixture(f, fx, 'negate')
          expect(fx.headSha(), 'the main checkout of the fixture kept its commit').toBe(headBefore)
          expect(fx.guardDenials(), 'the agent never tried to leave the fixture').toEqual([])
          await monitor.judgeAfterRun(worktreeChangeState(fx))
        } finally {
          tap.close()
          testInfo.annotations.push({
            type: 'agent-session-monitor',
            description: monitor.summary()
          })
        }
      }
    )
  })
})
