/**
 * A one-task Plan on the user's repo deepseek-harness, run by the real Claude while the spec follows the agent through the UI
 * (TASK-FE-E2E-028): the task sheet opened from the Plan tab, the dev server's busy state, the live agent output, the Result tab,
 * and the task worktree in Source Control, judged by the same rules as the throwaway-repo spec (live-agent-session-judge.ts).
 *
 *   python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real deepseek-task
 */
import { expect, test } from '@playwright/test'
import { expectSandboxChange } from './deepseek-change-checks'
import { runDeepseekFlow } from './deepseek-flow-harness'
import { DEEPSEEK_REPO } from './deepseek-repo-fixture'
import { createDeepseekRequest } from './deepseek-request-steps'
import {
  expectGoneWorktreeIsExplained,
  expectTaskSheetAfterRun
} from './deepseek-task-sheet-checks'
import { worktreeChangeState } from './live-fixture-change-checks'
import { AgentSessionMonitor } from './live-agent-session-monitor'
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
import { TaskActivityTap } from './live-task-activity-tap'
import { realRunBudgetMs } from './real-flow-harness'

const BODY =
  'Add a Node module scripts/e2e-sandbox/clamp.mjs that exports clamp(value, min, max) returning value limited to the range, and ' +
  'scripts/e2e-sandbox/clamp.test.mjs using node:test and node:assert with at least three cases; run it with ' +
  '`node --test scripts/e2e-sandbox/clamp.test.mjs`. This is a single step: plan exactly ONE task that does both, with no inspection, ' +
  'verification or review task and no dependencies. Use only the modules that come with Node, create exactly those two files, ' +
  'leave every other file untouched, and do not commit.'

test.describe("Following the agent on the user's repo from the UI (TASK-FE-E2E-028)", () => {
  test.skip(!isLiveRun(), 'live-stack suite: set ORCA_E2E_LIVE=1 (see orca-flows/README.md)')

  test('task, single step: the running agent, its dev server and its session are visible in the UI', async ({
    page,
    context
  }, testInfo) => {
    await runDeepseekFlow(
      { page, context, testInfo, slug: 'deepseek-task', mode: 'execute' },
      async (f, run) => {
        const headBefore = run.fx.headSha()
        const r = await createClassifyAndConfirm(f, {
          titleWords: 'add clamp',
          body: BODY,
          typeLabel: 'Task',
          typeKey: 'task',
          size: 'S',
          create: (title) => createDeepseekRequest(f, run, title, BODY)
        })
        await answerClarificationIfAsked(f, r, { api: ['planning'], chip: /Planning/i })
        await generatePlanThroughTheUi(f, r)
        const plan = await reviewPlanBeforeApproving(f, r, { paths: [DEEPSEEK_REPO] })
        const leaves = plan.filter((t) => !['plan', 'phase'].includes(t.type))
        // Why: subscribe before approving so the first output frame cannot be missed.
        const tap = await TaskActivityTap.open(
          f.rpc,
          leaves.map((t) => t.id)
        )
        const monitor = new AgentSessionMonitor(f, context, r, tap, run.fx.workRoots!, {
          changedFile: /clamp\.mjs/,
          worktreeCall: /deepseek-harness-task/,
          diffWord: 'clamp',
          trace: {
            file: /clamp\.mjs/i,
            testsRan: /(clamp\.test\.mjs|node --test|tests? (passed|pass|ran)|# pass)/i,
            fileLabel: 'scripts/e2e-sandbox/clamp.mjs'
          }
        })
        try {
          await monitor.prepare()
          await approvePlanThroughTheUi(f, r)
          await monitor.watchTask(leaves[0])
          const report = await driveExecutionToTheEnd(
            f,
            r,
            realRunBudgetMs('execute'),
            run.fx.workRoots!
          )
          console.log(`[exec] transitions:\n${report.seen.join('\n')}`)
          await expectPlanTabFinished(f, r)
          await expectSandboxChange(f, run.fx, {
            module: 'clamp.mjs',
            test: 'clamp.test.mjs',
            fn: 'clamp'
          })
          await expectTaskSheetAfterRun(f, leaves[0].id)
          expect(run.fx.headSha(), 'the main checkout kept its commit').toBe(headBefore)
          expect(run.fx.guardDenials(), 'nothing outside the task worktree was touched').toEqual([])
          await monitor.judgeAfterRun(worktreeChangeState(run.fx))
          await expectGoneWorktreeIsExplained(f, run.fx, leaves[0].id, r.title)
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
