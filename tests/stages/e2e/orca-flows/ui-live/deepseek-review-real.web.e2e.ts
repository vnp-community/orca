/**
 * Reviewing the real Solution of a change_request on the user's repo deepseek-harness (TASK-FE-E2E-028): the options and their
 * comparison, a short rejection reason that stays locked, Mod+Enter in the reject dialog, the Request returning to the backlog, Reopen,
 * "Regenerate" and the NEW Solution on screen, then choosing a non-recommended option with its rationale and approving (or the screen
 * explaining the service's refusal). The Plan is never approved: no agent writes anything; the Request is cancelled from the UI.
 *
 *   python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real deepseek-review
 */
import { expect, test } from '@playwright/test'
import { runDeepseekFlow } from './deepseek-flow-harness'
import { createDeepseekRequest } from './deepseek-request-steps'
import {
  chooseAndApprove,
  openSolution,
  regenerateSolution,
  reopenFromBacklog,
  rejectWithModEnter,
  solutionOptions
} from './deepseek-solution-review-steps'
import { callsTo } from './live-add-project-own-project-steps'
import {
  answerClarificationIfAsked,
  cancelThroughTheUi,
  createClassifyAndConfirm
} from './live-flow-steps'
import { isLiveRun } from './live-stack-config'
import { detailPane } from './live-request-ui-steps'

const BODY =
  'Thêm một hàm slugify(text) vào thư mục scripts/e2e-sandbox/ (một module Node, chỉ dùng thư viện có sẵn của Node). ' +
  'Hãy đề xuất đúng HAI phương án khác nhau cho cách cài đặt (ví dụ: dùng regex, hoặc duyệt từng ký tự), nêu ưu và nhược điểm của mỗi phương án, ' +
  'và chọn một phương án khuyến nghị. Chưa cần viết mã.'

test.describe("Reviewing a real Solution on the user's repo (TASK-FE-E2E-028)", () => {
  test.skip(!isLiveRun(), 'live-stack suite: set ORCA_E2E_LIVE=1 (see orca-flows/README.md)')

  test('change_request: options and comparison, reject with Mod+Enter, Reopen, the new Solution, choose and approve', async ({
    page,
    context
  }, testInfo) => {
    await runDeepseekFlow(
      { page, context, testInfo, slug: 'deepseek-review', mode: 'readonly' },
      async (f, run) => {
        const r = await createClassifyAndConfirm(f, {
          titleWords: 'slugify options',
          body: BODY,
          typeLabel: 'Change request',
          typeKey: 'change_request',
          size: 'S',
          create: (title) => createDeepseekRequest(f, run, title, BODY)
        })
        await answerClarificationIfAsked(f, r)
        await f.step('the Solution shows its options and the comparison', async () => {
          await openSolution(f, r)
          const count = await solutionOptions(f).count()
          console.log(`[deepseek] the Solution has ${count} option(s)`)
          expect(count, 'the Solution offers options').toBeGreaterThan(0)
          await expect(detailPane(f.page).getByText('Recommended').first()).toBeVisible()
          if (count >= 2) {
            await detailPane(f.page).getByRole('button', { name: 'Compare' }).click()
            await expect(f.page.getByRole('table'), 'Compare shows a table').toBeVisible()
          } else {
            testInfo.annotations.push({
              type: 'not-exercised',
              description: 'the AI offered a single option: no comparison table'
            })
          }
          await f.shots.take('solution-options')
        })
        const firstSolution =
          (
            await f.rpc.call<{ solutions?: { id: string }[] }>('solution.list', {
              requestId: r.requestId
            })
          ).solutions?.at(-1)?.id ?? ''
        await f.step(
          'reject by Mod+Enter: the Request returns to the backlog; Reopen',
          async () => {
            const from = run.tap.calls.length
            const reason = await rejectWithModEnter(f, run, from)
            const sent = callsTo(run.tap, 'approval.reject', from)
            expect(sent, 'the SPA sent one approval.reject').toHaveLength(1)
            expect(sent[0].params, 'approval.reject carries the reason').toMatchObject({
              comment: reason
            })
            await reopenFromBacklog(f, r, reason)
          }
        )
        await f.step('Regenerate: the screen follows the NEW Solution', async () => {
          await regenerateSolution(f, r, firstSolution)
        })
        await f.step(
          'choose a non-recommended option with its rationale, then approve',
          async () => {
            const outcome = await chooseAndApprove(f, run, r)
            console.log(`[deepseek] choose and approve: ${outcome}`)
            testInfo.annotations.push({ type: 'choose-and-approve', description: outcome })
          }
        )
        await cancelThroughTheUi(f, r, /Planning/i)
      }
    )
  })
})
