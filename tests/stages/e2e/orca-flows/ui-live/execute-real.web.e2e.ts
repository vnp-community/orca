/**
 * Plan execution with the REAL Claude Code of the dev server, driven through the UI (TASK-FE-E2E-010). A tiny change_request
 * on a throwaway repo goes request -> classify -> Clarification -> Solution -> Plan; the Plan is APPROVED on screen, the
 * dev-server agent runs `claude --dangerously-skip-permissions` on each task in a worktree next to the repo, the Phase gate is
 * approved on screen, and the Request completes. Safety net: real-flow-harness.ts. Never the fake claude.
 *
 *   python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real execute
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
  expectPlanTabFinished,
  reviewPlanBeforeApproving
} from './live-execution-steps'
import { expectExpectedChangeInFixture } from './live-fixture-change-checks'
import { isLiveRun } from './live-stack-config'
import { realRunBudgetMs, runRealFlow } from './real-flow-harness'

test.describe('Plan execution with the real Claude, through the UI', () => {
  test.skip(!isLiveRun(), 'live-stack suite: set ORCA_E2E_LIVE=1 (see orca-flows/README.md)')

  test('change_request: approve the Plan, the agent adds add() with a test, the Request completes', async ({
    page,
    context
  }, testInfo) => {
    await runRealFlow(
      { page, context, testInfo, slug: 'real-execute', mode: 'execute' },
      async (f, fx) => {
        const headBefore = fx.headSha()
        const r = await createClassifyAndConfirm(f, {
          titleWords: 'add an add function',
          body:
            'Add a function add(a, b) to src/calc.py that returns the sum of two numbers, ' +
            'and one test for it in tests/test_calc.py. Change nothing else.',
          typeLabel: 'Change request',
          typeKey: 'change_request',
          size: 'S'
        })
        const asked = await answerClarificationIfAsked(f, r)
        await approveAnalysis(f, r, {
          autoStarted: asked,
          next: { api: 'planning', chip: /Planning/i }
        })
        await generatePlanThroughTheUi(f, r)
        await reviewPlanBeforeApproving(f, r)
        await approvePlanThroughTheUi(f, r)
        const report = await driveExecutionToTheEnd(f, r, realRunBudgetMs('execute'), fx.base)
        console.log(`[exec] transitions:\n${report.seen.join('\n')}`)
        expect(report.phasesStarted, 'a Phase was approved and started on screen').toBeGreaterThan(
          0
        )
        expect(report.ranInUi, 'the screen showed the agent running').toBe(true)
        await expectPlanTabFinished(f, r)
        await expectExpectedChangeInFixture(f, fx, 'add')
        expect(fx.headSha(), 'the main checkout of the fixture kept its commit').toBe(headBefore)
        expect(
          fx.guardCalls().length,
          'the tool guard was live while the agent worked'
        ).toBeGreaterThan(0)
        expect(fx.guardDenials(), 'the agent never tried to leave the fixture').toEqual([])
      }
    )
  })

  // Why: a `task` has no Phases, so its execution path (Plan gate -> dispatch -> outcome -> completed) is checked on its own
  // while the Phase gate of change_request is separately blocked or green.
  test('task: approve the task list, the agent adds subtract() with a test, the Request completes', async ({
    page,
    context
  }, testInfo) => {
    await runRealFlow(
      { page, context, testInfo, slug: 'real-execute-task', mode: 'execute' },
      async (f, fx) => {
        const headBefore = fx.headSha()
        const r = await createClassifyAndConfirm(f, {
          titleWords: 'add a subtract function',
          body:
            'Add a function subtract(a, b) to src/calc.py that returns a minus b, ' +
            'and one test for it in tests/test_calc.py. Change nothing else.',
          typeLabel: 'Task',
          typeKey: 'task',
          size: 'S'
        })
        await answerClarificationIfAsked(f, r, { api: ['planning'], chip: /Planning/i })
        await generatePlanThroughTheUi(f, r)
        await reviewPlanBeforeApproving(f, r)
        await approvePlanThroughTheUi(f, r)
        const report = await driveExecutionToTheEnd(f, r, realRunBudgetMs('execute'), fx.base)
        console.log(`[exec] transitions:\n${report.seen.join('\n')}`)
        expect(report.phasesStarted, 'a task list has no Phase gate').toBe(0)
        expect(report.ranInUi, 'the screen showed the agent running').toBe(true)
        await expectPlanTabFinished(f, r)
        await expectExpectedChangeInFixture(f, fx, 'subtract')
        expect(fx.headSha(), 'the main checkout of the fixture kept its commit').toBe(headBefore)
        expect(
          fx.guardCalls().length,
          'the tool guard was live while the agent worked'
        ).toBeGreaterThan(0)
        expect(fx.guardDenials(), 'the agent never tried to leave the fixture').toEqual([])
      }
    )
  })

  // Why: separates "a one-task Plan runs end to end" from "a Plan whose tasks depend on each other does": the first can be green
  // while the second is blocked by the engine routing of dependent tasks (TASK-FE-E2E-010, finding B2).
  test('task, single step: a one-task Plan runs end to end and the Request completes', async ({
    page,
    context
  }, testInfo) => {
    await runRealFlow(
      { page, context, testInfo, slug: 'real-execute-single', mode: 'execute' },
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
        await reviewPlanBeforeApproving(f, r)
        await approvePlanThroughTheUi(f, r)
        const report = await driveExecutionToTheEnd(f, r, realRunBudgetMs('execute'), fx.base)
        console.log(`[exec] transitions:\n${report.seen.join('\n')}`)
        expect(report.ranInUi, 'the screen showed the agent running').toBe(true)
        await expectPlanTabFinished(f, r)
        await expectExpectedChangeInFixture(f, fx, 'negate')
        expect(fx.headSha(), 'the main checkout of the fixture kept its commit').toBe(headBefore)
        expect(
          fx.guardCalls().length,
          'the tool guard was live while the agent worked'
        ).toBeGreaterThan(0)
        expect(fx.guardDenials(), 'the agent never tried to leave the fixture').toEqual([])
      }
    )
  })
})
