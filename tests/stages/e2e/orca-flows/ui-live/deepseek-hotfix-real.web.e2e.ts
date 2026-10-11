/**
 * A hotfix on the user's repo has no Solution to review (TASK-FE-E2E-028): the Analysis tab offers no Approve or Reject. On this
 * one-user stack the hotfix diagnosis cannot open its post-hoc approval (REQUEST_APPROVAL_NO_ELIGIBLE_APPROVER: the approver must not
 * be the reporter), so the Request stays in analysis; the spec records that as a stack limit and cancels the Request from the UI.
 *
 *   python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real deepseek-review
 */
import { expect, test } from '@playwright/test'
import { runDeepseekFlow } from './deepseek-flow-harness'
import { createDeepseekRequest } from './deepseek-request-steps'
import {
  answerClarificationIfAsked,
  cancelThroughTheUi,
  createClassifyAndConfirm
} from './live-flow-steps'
import { requestState } from './live-gateway-crosschecks'
import { detailHeader, detailPane } from './live-request-ui-steps'
import { isLiveRun } from './live-stack-config'

const BODY =
  'Hotfix: scripts/e2e-sandbox/slug.mjs returns an empty string for input made only of punctuation. Only to check the screen; it will be cancelled.'

test.describe("A hotfix on the user's repo (TASK-FE-E2E-028)", () => {
  test.skip(!isLiveRun(), 'live-stack suite: set ORCA_E2E_LIVE=1 (see orca-flows/README.md)')

  test('hotfix: there is no Solution to review, so no decision bar', async ({
    page,
    context
  }, testInfo) => {
    await runDeepseekFlow(
      { page, context, testInfo, slug: 'deepseek-hotfix', mode: 'readonly' },
      async (f, run) => {
        const r = await createClassifyAndConfirm(f, {
          titleWords: 'hotfix no decision bar',
          body: BODY,
          typeLabel: 'Hotfix',
          typeKey: 'hotfix',
          size: 'S',
          urgency: 'Urgent',
          reason: 'A production defect that needs the fast path.',
          create: (title) => createDeepseekRequest(f, run, title, BODY)
        })
        await answerClarificationIfAsked(f, r, {
          api: ['analyzing', 'planning'],
          chip: /Analyzing|Planning/i
        })
        await f.step(
          'the hotfix diagnosis runs without asking for a Solution approval',
          async () => {
            await expect
              .poll(
                async () => {
                  const runs = (
                    await f.rpc.call<{
                      runs?: {
                        kind: string
                        status: string
                        errorCode?: string
                        startedAt?: string
                      }[]
                    }>('solution.list', { requestId: r.requestId })
                  ).runs
                  const last = (runs ?? []).toSorted((a, b) =>
                    (b.startedAt ?? '').localeCompare(a.startedAt ?? '')
                  )[0]
                  if (
                    last?.status === 'failed' &&
                    last.errorCode === 'REQUEST_HOTFIX_APPROVAL_FAILED'
                  ) {
                    testInfo.annotations.push({
                      type: 'stack-limit',
                      description:
                        'the hotfix approval needs an approver other than the reporter (REQUEST_APPROVAL_NO_ELIGIBLE_APPROVER); this stack has one user'
                    })
                    return 'stack-limit'
                  }
                  return (await requestState(f.rpc, r.requestId)).status === 'planning'
                    ? 'planning'
                    : 'waiting'
                },
                {
                  timeout: f.budgets.solutionMs,
                  message: 'the diagnosis reaches planning or its stack limit'
                }
              )
              .not.toBe('waiting')
          }
        )
        await f.step('a hotfix shows no Approve or Reject of a Solution', async () => {
          await expect(detailHeader(f.page)).toContainText(r.title)
          const analysis = detailPane(f.page).getByRole('tab', { name: 'Analysis' })
          if (await analysis.count()) {
            await analysis.click()
          }
          await expect(detailPane(f.page).getByRole('button', { name: /^Approve/ })).toHaveCount(0)
          await expect(detailPane(f.page).getByRole('button', { name: 'Reject' })).toHaveCount(0)
          await f.shots.take('hotfix-no-decision-bar')
        })
        await cancelThroughTheUi(f, r, /Analyzing|Planning/i)
      }
    )
  })
})
