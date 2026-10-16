/**
 * Classification flow of `bug` and `task` on the NATIVE engine, through the real UI (BUG-QA-02, specs/frontend/bugs/qa-2026-10-16).
 *
 *   python3 tests/stages/e2e/orca-flows/run_ui_flow.py --ai-stub -- qa-bug-task-native-flow     # deterministic, no AI cost, stops at the plan approval
 *   ORCA_E2E_READONLY_ANALYSIS=1 ... --real  -- qa-bug-task-native-flow                         # the same flow with the real Claude (read-only diagnosis)
 *   python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real native-execute                    # + executes the plan (spends AI usage)
 *
 * The throwaway project is set to engine=native (Solution by the plain model, no knowledge hub) and restored afterwards, so the
 * test does not depend on the deployment default (REQUEST_DEFAULT_ENGINE=openspec on the local stack). Plans are never approved
 * in the stub run: that would start an agent.
 */
import { expect, test } from '@playwright/test'
import {
  answerClarificationIfAsked,
  createClassifyAndConfirm,
  generatePlanThroughTheUi,
  rejectPlanApprovalAndCancel
} from './live-flow-steps'
import { approveAnalysis } from './live-analysis-approval-steps'
import { runLiveFlow } from './live-flow-harness'
import { configureProject } from './live-continuous-execution-steps'
import { expectApprovalLabel, expectStagesAndTabs } from './live-qa-steps'
import { isLiveRun, loadLiveStackConfig } from './live-stack-config'

test.describe('bug and task on the native engine (live stack)', () => {
  test.skip(!isLiveRun(), 'live-stack suite: set ORCA_E2E_LIVE=1 (see orca-flows/README.md)')

  test('bug: classified, diagnosed, planned on the native engine', async ({
    page,
    context
  }, testInfo) => {
    const cfg = loadLiveStackConfig()
    test.skip(
      cfg.aiMode !== 'stub' && process.env.ORCA_E2E_READONLY_ANALYSIS !== '1',
      'a real run starts a read-only Claude for the diagnosis: set ORCA_E2E_READONLY_ANALYSIS=1'
    )
    await runLiveFlow({ page, context, testInfo, slug: 'qa-native-bug' }, async (f) => {
      const before = await configureProject(f, {
        engine: 'native',
        solutionAgent: false,
        executionMode: 'step_by_step'
      })
      try {
        // "crash" is a bug cue for the fake claude; the real classifier reads the sentence.
        const r = await createClassifyAndConfirm(f, {
          titleWords: 'settings page crash on save',
          body: 'Saving the settings page crashes with an error when the badge colour is empty. It worked last week.',
          typeLabel: 'Bug',
          typeKey: 'bug',
          size: 'M'
        })
        const asked = await answerClarificationIfAsked(f, r)
        await approveAnalysis(f, r, {
          autoStarted: asked,
          next: { api: 'planning', chip: /Planning/i }
        })
        await generatePlanThroughTheUi(f, r)
        await expectStagesAndTabs(f, {
          requestId: r.requestId,
          engine: 'native',
          stages: ['classification', 'analysis', 'plan', 'execution'],
          tabs: ['Overview', 'Analysis', 'Plan', 'Execution', 'History', 'Related']
        })
        await expectApprovalLabel(f, r, /approve plan/i)
        await rejectPlanApprovalAndCancel(f, r)
      } finally {
        await configureProject(f, before).catch(() => undefined)
      }
    })
  })

  test('task: classified straight to a task list on the native engine, with no analysis', async ({
    page,
    context
  }, testInfo) => {
    await runLiveFlow({ page, context, testInfo, slug: 'qa-native-task' }, async (f) => {
      const before = await configureProject(f, {
        engine: 'native',
        solutionAgent: false,
        executionMode: 'step_by_step'
      })
      try {
        const r = await createClassifyAndConfirm(f, {
          titleWords: 'add a subtract function',
          body: 'Add a function subtract(a, b) to src/calc.py that returns a minus b, with one unit test.',
          typeLabel: 'Task',
          typeKey: 'task',
          size: 'S'
        })
        await answerClarificationIfAsked(f, r, { api: ['planning'], chip: /Planning/i })
        await generatePlanThroughTheUi(f, r)
        await expectStagesAndTabs(f, {
          requestId: r.requestId,
          engine: 'native',
          stages: ['classification', 'plan', 'execution'],
          tabs: ['Overview', 'Plan', 'Execution', 'History', 'Related']
        })
        await expectApprovalLabel(f, r, /approve task list/i)
        await rejectPlanApprovalAndCancel(f, r)
      } finally {
        await configureProject(f, before).catch(() => undefined)
      }
    })
  })

  test('bug and task keep the native engine when the deployment default is openspec', async ({
    page,
    context
  }, testInfo) => {
    await runLiveFlow({ page, context, testInfo, slug: 'qa-native-default' }, async (f) => {
      // No configureProject here: the project runs with whatever REQUEST_DEFAULT_ENGINE the stack has.
      const r = await createClassifyAndConfirm(f, {
        titleWords: 'settings page crash on save (default engine)',
        body: 'Saving the settings page crashes with an error when the badge colour is empty.',
        typeLabel: 'Bug',
        typeKey: 'bug',
        size: 'M'
      })
      await f.step(
        'the Request reports engine native and its timeline has no Handover',
        async () => {
          const res = await f.rpc.call<{ request?: { engine?: string } }>('request.get', {
            id: r.requestId
          })
          expect(
            res.request?.engine,
            'BUG-QA-02: a bug must be native unless the project chose otherwise'
          ).toBe('native')
          await expect(f.page.locator('[data-step-id="handover"]')).toHaveCount(0)
        }
      )
      await f.rpc
        .call('request.cancel', { id: r.requestId, reason: 'qa native default check' })
        .catch(() => undefined)
    })
  })
})
