/**
 * Steps of a demanding user (ORCA_E2E_DEMANDING=1): someone who does not accept the first answer. Every action goes through the
 * screen like any person's: the Analysis and the Plan are sent back with a list of demands, an older version is opened and must be
 * read-only, the page is reloaded in the middle of the work, and an after-code finding is sent back as a child Request.
 *
 * What the AI does with the demands is recorded, not asserted: a demand it ignores is a finding for the report, not a failed run.
 */
import { expect } from '@playwright/test'
import type { LiveFlow } from './live-flow-harness'
import type { RequestUnderTest } from './live-flow-steps'
import { listVersions, openAnalysisApprovals, reviseAnalysis } from './live-analysis-revision-steps'
import {
  chooseVersion,
  openPlanApprovals,
  planRevisionInput,
  revisePlan
} from './live-plan-revision-steps'
import { detailPane, openDetailTab, uiFindings } from './live-request-ui-steps'
import {
  reloadAndReopen,
  cancelOpenRequestThroughTheUi,
  openRequestsPage,
  selectRequestRow
} from './live-request-ui-steps'
import { outcomeReviewTab, reviewTab } from './live-outcome-review-steps'
import { requestState } from './live-gateway-crosschecks'

export const demanding = (): boolean => process.env.ORCA_E2E_DEMANDING === '1'

/** Vietnamese, specific and checkable: what a reader who is not satisfied would write. */
export const ANALYSIS_DEMAND = [
  'Tôi chưa chấp nhận bản phân tích này. Sửa lại như sau:',
  '1. Tách riêng ba phần: Google Chat, Google Sheets, nhập bug từ Sheet; mỗi phần có tiêu chí kiểm chứng riêng.',
  '2. Nói rõ làm sao đạt "nhập 200 hàng, chạy lại không tạo trùng": khoá chống trùng là gì, nằm ở đâu.',
  '3. Liệt kê rủi ro bảo mật của secret Google (sealed) và những nơi secret có thể lộ, kể cả log.',
  '4. Ghi rõ phần nào KHÔNG làm trong lượt này.',
  '5. Số liệu nào lấy từ tài liệu thì ghi nguồn; chỗ nào đoán thì ghi "chưa xác minh".',
  'Viết bằng tiếng Việt, ngắn gọn.'
].join('\n')

export const PLAN_DEMAND = [
  'Kế hoạch này chưa đạt. Sửa lại:',
  '- Tách việc nhập Sheet thành ba task riêng: kiểm schema trước khi nhập, nhập từng hàng, chống trùng khi chạy lại.',
  '- Thêm task kiểm thử cho 200 hàng và cho hàng sai (báo lỗi cụ thể từng hàng).',
  '- Không gộp Chat và Sheets vào một task.',
  '- Không có task nào được chạm docker, deploy hay dịch vụ đang chạy; lệnh Verify của mỗi task phải chạy được cục bộ.'
].join('\n')

export type DemandRecord = { what: string; versions: number; note: string }

/** Words of a demand that the revised text should now contain, to record whether the AI listened. */
const echoes = (text: string, words: string[]): string[] =>
  words.filter((w) => text.toLowerCase().includes(w.toLowerCase()))

/** The Analysis is on screen and waiting: send it back once with the demands, check what the screen promises, then reload. */
export async function demandAnalysisChanges(
  f: LiveFlow,
  r: RequestUnderTest,
  record: DemandRecord[]
): Promise<void> {
  const previous = await openAnalysisApprovals(f, r.requestId)
  await reviseAnalysis(f, r, { text: ANALYSIS_DEMAND, version: 2, previousApprovals: previous })
  await f.step(
    'demanding: after a reload the Analysis still shows version 2 and keeps what was asked',
    async () => {
      await reloadAndReopen(f.page, r.title)
      const versions = await listVersions(f, r.requestId)
      expect(versions.length, 'both versions survive a reload').toBe(2)
      expect(versions.at(-1)?.revisionFeedback, 'the demand is stored with the version').toBe(
        ANALYSIS_DEMAND
      )
      const shown = await detailPane(f.page).innerText()
      const heard = echoes(shown, ['idempotent', 'trùng', 'secret', 'chưa xác minh', 'không làm'])
      record.push({
        what: 'analysis demands',
        versions: versions.length,
        note: `the new text mentions: ${heard.join(', ') || 'none of the demanded terms'}`
      })
      await f.shots.take('demanding-analysis-after-reload')
    }
  )
}

/** Sends the Plan back when the screen offers to; a Plan the screen cannot be sent back is itself a finding for the report. */
export async function demandPlanChangesWhenPossible(
  f: LiveFlow,
  r: RequestUnderTest,
  record: DemandRecord[]
): Promise<void> {
  await openDetailTab(f.page, 'Plan')
  const offered = await planRevisionInput(f.page)
    .isVisible({ timeout: 10_000 })
    .catch(() => false)
  if (!offered) {
    record.push({
      what: 'plan demands',
      versions: 1,
      note: 'the Plan tab offers no way to send the plan back (no revision box)'
    })
    await f.shots.take('demanding-plan-no-revision-box')
    return
  }
  await demandPlanChanges(f, r, record)
}

/** The Plan is on screen and waiting: send it back once, open the first version (it must be read-only), come back. */
export async function demandPlanChanges(
  f: LiveFlow,
  r: RequestUnderTest,
  record: DemandRecord[]
): Promise<void> {
  const previous = await openPlanApprovals(f, r.requestId)
  await revisePlan(f, r, { text: PLAN_DEMAND, version: 2, previousApprovals: previous })
  await f.step(
    'demanding: the first version of the Plan is read-only and the way back works',
    async () => {
      const pane = detailPane(f.page)
      await chooseVersion(f, 1)
      await expect(pane.getByTestId('plan-older-version-banner')).toBeVisible()
      await expect(
        pane.getByTestId('plan-revision-input'),
        'no revision box on an old version'
      ).toHaveCount(0)
      await expect(
        pane
          .getByTestId('plan-proposal-view')
          .or(pane.getByRole('tab', { name: 'Proposal' }))
          .first(),
        'the old version can be read'
      ).toBeVisible({ timeout: 15_000 })
      // Why: a person looking at a version the page itself calls older should not be offered to approve it as if it were the plan.
      if (await pane.getByRole('button', { name: 'Approve plan', exact: true }).isVisible()) {
        uiFindings.push(
          'an older version of the Plan is on screen ("You\'re viewing an older version") and "Approve plan" and "Approve and run all steps" are still offered'
        )
      }
      await f.shots.take('demanding-plan-old-version')
      await pane.getByTestId('plan-back-to-current').click()
      await expect(pane.getByTestId('plan-older-version-banner')).toHaveCount(0)
      await expect(
        pane.getByTestId('plan-revision-input'),
        'the box is back on the current version'
      ).toBeVisible()
    }
  )
  const shown = await detailPane(f.page).innerText()
  const heard = echoes(shown, ['schema', '200', 'chống trùng', 'Verify'])
  record.push({
    what: 'plan demands',
    versions: 2,
    note: `the new plan mentions: ${heard.join(', ') || 'none of the demanded terms'}`
  })
}

/**
 * On the after-code review: send one finding back as a child Request (the dialog is prefilled, the title is made stricter), see that
 * the parent did not move, then cancel the child through the screen so nothing is left running.
 */
export async function sendBackAsDemandingReviewer(
  f: LiveFlow,
  r: RequestUnderTest
): Promise<string> {
  let childTitle = ''
  await f.step(
    'demanding: send a finding back; the dialog is prefilled and the child is a new Request',
    async () => {
      await reloadAndReopen(f.page, r.title)
      await outcomeReviewTab(f.page).click()
      await expect(reviewTab(f.page)).toBeVisible({ timeout: 60_000 })
      const cards = f.page.getByTestId('outcome-decision-card')
      const open = f.page.getByRole('button', { name: 'Send back' })
      if ((await open.count()) === 0) {
        await f.page.getByRole('button', { name: 'Change decision' }).first().click()
      }
      await expect(open.first()).toBeVisible()
      await open.first().click()
      await f.page.getByTestId('outcome-request-changes').click()
      const dialog = f.page.getByTestId('outcome-send-back-dialog')
      await expect(dialog).toBeVisible()
      expect(
        await dialog.getByTestId('outcome-send-back-candidate').count(),
        'the finding that was sent back is offered'
      ).toBeGreaterThan(0)
      await dialog
        .locator('#sb-note')
        .fill('Làm lại phần này cho đúng tiêu chí nghiệm thu, kèm test chứng minh.')
      await f.shots.take('demanding-send-back-dialog')
      await dialog.getByTestId('outcome-send-back-continue').click()

      const child = f.page.getByRole('dialog').filter({ hasText: 'Create child request' })
      await expect(child).toBeVisible({ timeout: 30_000 })
      const title = child.locator('#spawn-child-title')
      expect((await title.inputValue()).length, 'the child title is prefilled').toBeGreaterThan(0)
      expect(
        (await child.locator('#spawn-child-body').inputValue()).length,
        'the child description is prefilled'
      ).toBeGreaterThan(20)
      childTitle = `[Làm lại] ${(await title.inputValue()).slice(0, 90)}`
      await title.fill(childTitle)
      await f.shots.take('demanding-child-dialog')
      await child.getByRole('button', { name: 'Create', exact: true }).click()
      await expect(f.page.getByText('Child request created')).toBeVisible({ timeout: 30_000 })
      expect(
        (await requestState(f.rpc, r.requestId)).status,
        'sending a finding back leaves the parent as it was'
      ).toBe('completed')
      void cards
    }
  )
  await f.step(
    'demanding: the child shows in the list and is cancelled through the screen',
    async () => {
      await openRequestsPage(f.page)
      await selectRequestRow(f.page, childTitle)
      await cancelOpenRequestThroughTheUi(
        f.page,
        'demanding reviewer: only checking the send-back path'
      )
      await f.shots.take('demanding-child-cancelled')
    }
  )
  return childTitle
}

export function demandLines(record: DemandRecord[], child: string): string[] {
  return [
    ...uiFindings.map((x) => `phát hiện về giao diện: ${x}`),
    ...record.map((x) => `${x.what}: ${x.versions} phiên bản; ${x.note}`),
    child
      ? `gửi lại phát hiện sau code thành Request con "${child}" (đã huỷ qua giao diện)`
      : 'không gửi lại phát hiện nào'
  ]
}
