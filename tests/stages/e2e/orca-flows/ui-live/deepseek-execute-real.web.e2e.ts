/**
 * The agent really writes code in the user's repo deepseek-harness, ordered from the UI (TASK-FE-E2E-028). A change_request goes
 * request -> Clarification -> Solution -> Plan with Phases; the Plan tab is checked against the task service, a short reject reason
 * stays locked, the Phase gate locks Start until it is approved, and the real Claude (`--dangerously-skip-permissions`) works in a
 * task worktree next to the repo. What it wrote is read from disk and its tests are run. The main checkout, its refs, the user's own
 * worktree and the remote are untouched, and `git push` is blocked for the whole run (run_ui_flow.py).
 *
 *   python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real deepseek-execute
 */
import { expect, test } from '@playwright/test'
import { expectSandboxChange } from './deepseek-change-checks'
import { runDeepseekFlow } from './deepseek-flow-harness'
import {
  expectPlanTabShape,
  expectRequestGraphSheet,
  expectShortRejectStaysLocked,
  expectTreeGraphToggle
} from './deepseek-plan-checks'
import { DEEPSEEK_REPO } from './deepseek-repo-fixture'
import { createDeepseekRequest } from './deepseek-request-steps'
import { expectTaskSheetAfterRun } from './deepseek-task-sheet-checks'
import { callsTo } from './live-add-project-own-project-steps'
import { approveAnalysis } from './live-analysis-approval-steps'
import {
  approvePlanThroughTheUi,
  driveExecutionToTheEnd,
  expectPlanTabFinished,
  reviewPlanBeforeApproving
} from './live-execution-steps'
import {
  answerClarificationIfAsked,
  createClassifyAndConfirm,
  generatePlanThroughTheUi
} from './live-flow-steps'
import { isLiveRun } from './live-stack-config'
import { realRunBudgetMs } from './real-flow-harness'

const BODY =
  'Add a Node module scripts/e2e-sandbox/slug.mjs that exports a function slugify(text): it lowercases the text, replaces every run ' +
  'of characters that are not letters or digits with a single "-", and trims "-" from both ends. Add scripts/e2e-sandbox/slug.test.mjs ' +
  'using node:test and node:assert that covers at least three cases, and run it with `node --test scripts/e2e-sandbox/slug.test.mjs`. ' +
  'Use only the modules that come with Node. Create exactly those two files, leave every other file untouched, and do not commit.'

test.describe("The agent works on the user's repo from the UI (TASK-FE-E2E-028)", () => {
  test.skip(!isLiveRun(), 'live-stack suite: set ORCA_E2E_LIVE=1 (see orca-flows/README.md)')

  test('change_request with Phases: plan, gates, the agent writes slugify() with a test in a task worktree, the Request completes', async ({
    page,
    context
  }, testInfo) => {
    await runDeepseekFlow(
      { page, context, testInfo, slug: 'deepseek-execute', mode: 'execute' },
      async (f, run) => {
        const headBefore = run.fx.headSha()
        const r = await createClassifyAndConfirm(f, {
          titleWords: 'add slugify',
          body: BODY,
          typeLabel: 'Change request',
          typeKey: 'change_request',
          size: 'S',
          create: (title) => createDeepseekRequest(f, run, title, BODY)
        })
        const asked = await answerClarificationIfAsked(f, r)
        await approveAnalysis(f, r, {
          autoStarted: asked,
          next: { api: 'planning', chip: /Planning/i }
        })
        await generatePlanThroughTheUi(f, r)
        const plan = await reviewPlanBeforeApproving(f, r, { paths: [DEEPSEEK_REPO] })
        await expectPlanTabShape(f, r)
        await expectShortRejectStaysLocked(f, () => callsTo(run.tap, 'approval.reject').length)
        await expectTreeGraphToggle(f)
        await expectRequestGraphSheet(f)
        await approvePlanThroughTheUi(f, r)
        const report = await driveExecutionToTheEnd(
          f,
          r,
          realRunBudgetMs('execute'),
          run.fx.workRoots!
        )
        console.log(`[exec] transitions:\n${report.seen.join('\n')}`)
        expect(report.phasesStarted, 'a Phase was approved and started on screen').toBeGreaterThan(
          0
        )
        expect(report.ranInUi, 'the screen showed the agent running').toBe(true)
        await expectPlanTabFinished(f, r)
        await expectSandboxChange(f, run.fx, {
          module: 'slug.mjs',
          test: 'slug.test.mjs',
          fn: 'slugify'
        })
        const leaf = plan.find((t) => !['plan', 'phase'].includes(t.type))!
        await expectTaskSheetAfterRun(f, leaf.id)
        expect(run.fx.headSha(), 'the main checkout kept its commit').toBe(headBefore)
        expect(run.fx.guardDenials(), 'nothing outside the task worktree was touched').toEqual([])
      }
    )
  })
})
