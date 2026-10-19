/**
 * `bug` and `task` executed on the NATIVE engine with the REAL Claude, through the UI, plus BUG-QA-05 (specs/frontend/bugs/qa-2026-10-16):
 * a finished task's agent log must not show a harmless CLI diagnostic such as `[claude-code:unrecognized_model]` as an error.
 *
 *   python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real native-execute      (spends AI usage on a throwaway repo; safety net as execute-real)
 */
import { expect, test } from '@playwright/test'
import {
  answerClarificationIfAsked,
  createClassifyAndConfirm,
  generatePlanThroughTheUi
} from './live-flow-steps'
import { approveAnalysis } from './live-analysis-approval-steps'
import {
  approvePlanThroughTheUi,
  driveExecutionToTheEnd,
  reviewPlanBeforeApproving
} from './live-execution-steps'
import { configureProject } from './live-continuous-execution-steps'
import { expectStagesAndTabs } from './live-qa-steps'
import { detailPane, openDetailTab } from './live-request-ui-steps'
import { isLiveRun } from './live-stack-config'
import { realRunBudgetMs, runRealFlow } from './real-flow-harness'
import type { LiveFlow } from './live-flow-harness'

/** QA-UI-LOG-01: open the finished task's log; no red line may carry a `[claude-code:…]` diagnostic. */
async function expectNoDiagnosticShownAsError(f: LiveFlow): Promise<void> {
  await f.step(
    'the finished task log shows no CLI diagnostic as an error (BUG-QA-05)',
    async () => {
      await openDetailTab(f.page, 'Execution')
      const pane = detailPane(f.page)
      // Fold-out finished steps, then open the first task.
      for (const toggle of await pane
        .locator('[data-testid^="execution-step-"] button[aria-expanded="false"]')
        .all()) {
        await toggle.click().catch(() => undefined)
      }
      await pane.locator('[data-testid^="execution-task-open-"]').first().click()
      const panel = f.page.getByText('Agent activity').first()
      await expect(panel, 'the task sheet shows the agent log').toBeVisible({ timeout: 20_000 })
      await f.shots.take('agent-log')
      const redDiagnostics = f.page.locator('[role="dialog"] .text-destructive', {
        hasText: /\[claude-code:/
      })
      expect(
        await redDiagnostics.count(),
        `a "[claude-code:…]" diagnostic is drawn as an error: ${(await redDiagnostics.allInnerTexts()).join(' | ').slice(0, 200)}`
      ).toBe(0)
    }
  )
}

test.describe('bug and task executed on the native engine with the real Claude', () => {
  test.skip(!isLiveRun(), 'live-stack suite: set ORCA_E2E_LIVE=1 (see orca-flows/README.md)')

  test('bug: diagnosed, planned, approved, executed to completion (native)', async ({
    page,
    context
  }, testInfo) => {
    await runRealFlow(
      { page, context, testInfo, slug: 'qa-real-native-bug', mode: 'execute' },
      async (f, fx) => {
        const headBefore = fx.headSha()
        const before = await configureProject(f, {
          engine: 'native',
          solutionAgent: false,
          executionMode: 'step_by_step'
        })
        try {
          const r = await createClassifyAndConfirm(f, {
            titleWords: 'clamp ignores the upper bound',
            body:
              'clamp(15, 0, 10) in src/calc.py returns 0 but it should return 10. Fix clamp and add one regression test in ' +
              'tests/test_calc.py. Plan exactly ONE task that does both; no inspection or review task. Change nothing else.',
            typeLabel: 'Bug',
            typeKey: 'bug',
            size: 'S'
          })
          const asked = await answerClarificationIfAsked(f, r)
          await approveAnalysis(f, r, {
            autoStarted: asked,
            next: { api: 'planning', chip: /Planning/i }
          })
          await generatePlanThroughTheUi(f, r)
          await reviewPlanBeforeApproving(f, r)
          await expectStagesAndTabs(f, {
            requestId: r.requestId,
            engine: 'native',
            stages: ['classification', 'analysis', 'plan', 'execution'],
            tabs: ['Overview', 'Analysis', 'Plan', 'Execution', 'History', 'Related']
          })
          await approvePlanThroughTheUi(f, r)
          await driveExecutionToTheEnd(f, r, realRunBudgetMs('execute'), fx.base)
          await expectNoDiagnosticShownAsError(f)
          expect(fx.headSha(), 'the main checkout of the fixture kept its commit').toBe(headBefore)
          expect(fx.guardDenials(), 'the agent never tried to leave the fixture').toEqual([])
        } finally {
          await configureProject(f, before).catch(() => undefined)
        }
      }
    )
  })

  test('task: planned as a task list, approved, executed to completion (native)', async ({
    page,
    context
  }, testInfo) => {
    await runRealFlow(
      { page, context, testInfo, slug: 'qa-real-native-task', mode: 'execute' },
      async (f, fx) => {
        const headBefore = fx.headSha()
        const before = await configureProject(f, {
          engine: 'native',
          solutionAgent: false,
          executionMode: 'step_by_step'
        })
        try {
          const r = await createClassifyAndConfirm(f, {
            titleWords: 'add a negate function',
            body:
              'Add a function negate(a) to src/calc.py that returns minus a, and one test for it in tests/test_calc.py. ' +
              'This is a single step: plan exactly ONE task, no inspection or review task. Change nothing else.',
            typeLabel: 'Task',
            typeKey: 'task',
            size: 'S'
          })
          await answerClarificationIfAsked(f, r, { api: ['planning'], chip: /Planning/i })
          await generatePlanThroughTheUi(f, r)
          await reviewPlanBeforeApproving(f, r)
          await expectStagesAndTabs(f, {
            requestId: r.requestId,
            engine: 'native',
            stages: ['classification', 'plan', 'execution'],
            tabs: ['Overview', 'Plan', 'Execution', 'History', 'Related']
          })
          await approvePlanThroughTheUi(f, r)
          await driveExecutionToTheEnd(f, r, realRunBudgetMs('execute'), fx.base)
          await expectNoDiagnosticShownAsError(f)
          expect(fx.headSha(), 'the main checkout of the fixture kept its commit').toBe(headBefore)
          expect(fx.guardDenials(), 'the agent never tried to leave the fixture').toEqual([])
        } finally {
          await configureProject(f, before).catch(() => undefined)
        }
      }
    )
  })
})
