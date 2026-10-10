/**
 * Live-stack acceptance of the Request flow THROUGH THE REAL UI (TASK-FE-E2E-008). Nothing is mocked. Runs only with
 * ORCA_E2E_LIVE=1:
 *
 *   ORCA_E2E_LIVE=1 npx playwright test -c tests/configs/playwright.web.config.ts --project=request-live
 *   python3 tests/stages/e2e/orca-flows/run_ui_flow.py --ai-stub        # deterministic AI (swaps and restores the agent)
 *
 * Test 3 (stub only): the first Solution carries a blocking question (trigger code in the title), so the user asks back,
 * answers, and approves the regenerated Solution. With a real AI test 1 takes the same branch whenever the AI happens to ask.
 * Test 1: change_request: confirm type -> Clarification -> automatic Solution -> approve option -> Plan -> Plan Approval ->
 * reject -> cancel. Test 2: question: the other analysis kind (an answer), ending in `completed`; if the type needs no
 * Clarification the "Generate solution" button is the way in and is pressed.
 * The gateway only cross-checks and cleans up. The Plan Approval is never approved: that would run an AI agent on the dev machine.
 */
import { test } from '@playwright/test'
import {
  answerClarificationIfAsked,
  approveAnalysis,
  cancelThroughTheUi,
  createClassifyAndConfirm,
  generatePlanThroughTheUi,
  rejectPlanApprovalAndCancel
} from './live-flow-steps'
import { runLiveFlow } from './live-flow-harness'
import { isLiveRun, loadLiveStackConfig } from './live-stack-config'

test.describe('Request flow through the real UI (live stack)', () => {
  test.skip(
    !isLiveRun(),
    'live-stack suite: set ORCA_E2E_LIVE=1 and run deploy/dev (see orca-flows/README.md)'
  )

  test('change_request: Clarification -> Solution -> Plan -> Approval, then clean up', async ({
    page,
    context
  }, testInfo) => {
    await runLiveFlow({ page, context, testInfo, slug: 'change-request' }, async (f) => {
      // Why: "add support for" is a change_request cue for the fake claude, so stub and real runs share one path.
      const r = await createClassifyAndConfirm(f, {
        titleWords: 'health badge change',
        body: 'Add support for a health badge on the settings page. The badge is green when the stack is ok.',
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
      await rejectPlanApprovalAndCancel(f, r)
    })
  })

  test('ask-back: a blocking question is asked back, answered, and the regenerated Solution is approved', async ({
    page,
    context
  }, testInfo) => {
    test.skip(
      loadLiveStackConfig().aiMode !== 'stub',
      'needs the fake claude trigger code; with a real AI the ask-back branch runs inside the change_request test when the AI asks'
    )
    await runLiveFlow({ page, context, testInfo, slug: 'ask-back' }, async (f) => {
      const r = await createClassifyAndConfirm(f, {
        titleWords: 'badge ask back [stub:blocking-question]',
        body: 'Add support for a health badge on the settings page. The badge is green when the stack is ok.',
        typeLabel: 'Change request',
        typeKey: 'change_request',
        size: 'M'
      })
      const asked = await answerClarificationIfAsked(f, r)
      await approveAnalysis(f, r, {
        autoStarted: asked,
        expectAskBacks: 1,
        next: { api: 'planning', chip: /Planning/i }
      })
      await cancelThroughTheUi(f, r, /Planning/i)
    })
  })

  test('question: answer is generated and approved, the Request completes', async ({
    page,
    context
  }, testInfo) => {
    await runLiveFlow({ page, context, testInfo, slug: 'question' }, async (f) => {
      const r = await createClassifyAndConfirm(f, {
        titleWords: 'how does the badge decide its colour',
        body: 'How does the health badge decide its colour? Which checks feed it?',
        typeLabel: 'Question',
        typeKey: 'question',
        size: 'S'
      })
      const asked = await answerClarificationIfAsked(f, r)
      await approveAnalysis(f, r, {
        autoStarted: asked,
        next: { api: 'completed', chip: /Completed/i }
      })
    })
  })
})
