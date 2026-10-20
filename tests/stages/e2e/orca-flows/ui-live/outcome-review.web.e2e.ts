/**
 * The Outcome Review of a Request through the real UI on a LedgerLite repo (a `bug`, native engine, fake claude): FE-OR-T330.
 *
 *   python3 tests/stages/e2e/orca-flows/run_ui_flow.py --ai-stub -- outcome-review
 *
 * Needs a stack that has the Outcome Review in the gateway, request-service and the frontend. With the fake claude a Request
 * can reach its Plan but never the end of Execution (the stub refuses to write), so this spec covers the design phase: the
 * switch that keeps the tab away, the tab after Execution, reading only when opened, the facts-only review, and what the screen
 * says when the agent cannot run. The after-code phase, the decisions and "Request changes" are covered by the component tests
 * of the tab and by orca-flows/suite_outcome_review.py (ORV-20 for the real Claude).
 *
 * (a) with the review off there is no tab and no `request.review.*` traffic; (b) with it on the tab sits right after Execution,
 * opening it only reads, and a facts-only review shows the design view with no decision controls and survives a reload without
 * a new generate; (c) asking for the agent's reading either starts a run or says why not in one sentence, never a raw code.
 */
import { expect, test } from '@playwright/test'
import {
  approveAnalysisAndLeavePlanToOrca,
  reachAnalysisApproval,
  watchBrowserCalls
} from './live-auto-plan-steps'
import { configureProject } from './live-continuous-execution-steps'
import { waitForApiStatus } from './live-gateway-crosschecks'
import { runLedgerliteFlow } from './live-ledgerlite-project-flow'
import {
  detailTabNames,
  openOutcomeReviewTab,
  outcomeReviewTab,
  reviewInvite,
  reviewTab,
  setOutcomeReview
} from './live-outcome-review-steps'
import {
  cancelOpenRequestThroughTheUi,
  openRequestsPage,
  reloadAndReopen,
  selectRequestRow
} from './live-request-ui-steps'
import { isLiveRun, loadLiveStackConfig } from './live-stack-config'

const NATIVE = {
  engine: 'native',
  solutionAgent: false,
  executionMode: 'step_by_step',
  planAutoGenerate: true
} as const
const PLAN_WAIT_MS = 120_000

test.describe('the Outcome Review of a Request (live stack)', () => {
  test.skip(!isLiveRun(), 'live-stack suite: set ORCA_E2E_LIVE=1 (see orca-flows/README.md)')
  test.skip(
    () => loadLiveStackConfig().aiMode !== 'stub',
    'this spec drives the fake claude: run it with run_ui_flow.py --ai-stub'
  )

  test('(a) with the review off there is no tab and no review traffic; (b) with it on the tab follows Execution and only reads', async ({
    page,
    context
  }, testInfo) => {
    await runLedgerliteFlow(
      { page, context, testInfo, slug: 'outcome-review-design' },
      async (f) => {
        const settingsBefore = await configureProject(f, NATIVE)
        const reviewBefore = await setOutcomeReview(f, 'off')
        try {
          const { r, asked } = await reachAnalysisApproval(f, 'settings page crash on review')
          await openRequestsPage(f.page)
          await selectRequestRow(f.page, r.title)
          await approveAnalysisAndLeavePlanToOrca(f, r, asked)
          await waitForApiStatus(f.rpc, r.requestId, ['awaiting_plan_approval'], PLAN_WAIT_MS)

          await f.step(
            '(a) review off: no Outcome review tab, and nothing about reviews was sent',
            async () => {
              const calls = await watchBrowserCalls(f.page)
              await reloadAndReopen(f.page, r.title)
              expect(await detailTabNames(f.page)).not.toContain('Outcome review')
              for (const method of [
                'request.review.get',
                'request.review.estimate',
                'request.review.generate',
                'request.review.runs'
              ]) {
                expect(calls.count(method), `${method} was sent with the review off`).toBe(0)
              }
            }
          )

          await f.step('(b) review on: the tab comes right after Execution', async () => {
            await setOutcomeReview(f, 'manual')
            await reloadAndReopen(f.page, r.title)
            const names = await detailTabNames(f.page)
            const execution = names.findIndex((n) => n.startsWith('Execution'))
            expect(execution, 'the Request has an Execution tab').toBeGreaterThan(-1)
            expect(names[execution + 1]).toBe('Outcome review')
            await f.shots.take('outcome-review-tab-present')
          })

          await f.step(
            '(b) opening the tab only reads: an invitation with what asking would do, no run started',
            async () => {
              const calls = await watchBrowserCalls(f.page)
              await reloadAndReopen(f.page, r.title)
              await openOutcomeReviewTab(f.page)
              await expect(reviewInvite(f.page)).toBeVisible()
              await expect(reviewInvite(f.page)).toHaveAttribute('data-kind', 'design')
              await expect(f.page.getByTestId('outcome-review-estimate')).toContainText(
                'No cost figures have been measured yet.'
              )
              expect(
                calls.count('request.review.generate'),
                'opening the tab did not ask for a review'
              ).toBe(0)
              expect(calls.count('request.review.get')).toBeGreaterThan(0)
              await f.shots.take('outcome-review-invitation')
            }
          )

          await f.step(
            '(b) a facts-only review shows the design view and offers no decisions',
            async () => {
              await f.page.getByTestId('outcome-review-facts-only').click()
              await expect(reviewTab(f.page)).toBeVisible({ timeout: 30_000 })
              await expect(reviewTab(f.page)).toHaveAttribute('data-phase', 'design')
              await expect(f.page.getByTestId('outcome-banner-design_phase')).toContainText(
                'The after-code view appears when execution completes.'
              )
              await expect(f.page.getByTestId('outcome-needs-decision-later')).toContainText(
                'Decisions open after execution completes.'
              )
              await expect(f.page.getByTestId('outcome-decision-bar')).toHaveCount(0)
              await expect(f.page.getByTestId('outcome-approve')).toHaveCount(0)
              const modes = f.page.getByTestId('outcome-mode-switch')
              await expect(modes.locator('[data-mode="design"]')).toHaveAttribute(
                'aria-pressed',
                'true'
              )
              await expect(modes.locator('[data-mode="after_code"]')).toHaveAttribute(
                'aria-disabled',
                'true'
              )
              await f.shots.take('outcome-review-design-phase')
            }
          )

          await f.step(
            '(b) after a reload the review is still there and nothing is generated again',
            async () => {
              const calls = await watchBrowserCalls(f.page)
              await reloadAndReopen(f.page, r.title)
              await outcomeReviewTab(f.page).click()
              await expect(reviewTab(f.page)).toBeVisible({ timeout: 30_000 })
              expect(
                calls.count('request.review.generate'),
                'a reload and a tab change generated nothing'
              ).toBe(0)
            }
          )

          await f.step(
            "(c) the agent's reading: a run starts, or one sentence says why not, and no raw code is ever shown",
            async () => {
              await f.page.getByTestId('outcome-header-regenerate').click()
              const panel = f.page.getByTestId('regenerate-panel')
              await expect(panel).toBeVisible()
              await expect(f.page.getByTestId('regenerate-estimate')).not.toContainText(
                'Working out what this would do'
              )
              await f.page.getByLabel('What to do').selectOption('context')
              await f.page.getByLabel('Why').selectOption('looks_wrong')
              await f.page.getByTestId('regenerate-submit').click()
              const failure = f.page.getByTestId('regenerate-failure')
              // Either the server took the request (the panel closes) or it said no (the panel stays with its sentence).
              await expect
                .poll(async () => (await failure.isVisible()) || !(await panel.isVisible()), {
                  timeout: 30_000
                })
                .toBe(true)
              if (await failure.isVisible()) {
                // One sentence in words; the code behind it stays out of sight.
                await expect(failure).not.toContainText(/REQUEST_REVIEW_/)
                await expect(failure).not.toBeEmpty()
              }
              expect(await f.page.locator('body').innerText()).not.toMatch(/REQUEST_REVIEW_[A-Z_]+/)
              await f.shots.take('outcome-review-regenerate-outcome')
            }
          )

          await cancelOpenRequestThroughTheUi(f.page, 'outcome review check')
        } finally {
          await setOutcomeReview(
            f,
            reviewBefore === 'manual' || reviewBefore === 'off' ? reviewBefore : 'default'
          ).catch(() => undefined)
          await configureProject(f, settingsBefore).catch(() => undefined)
        }
      }
    )
  })
})
