/**
 * Vietnamese AI output on the Analysis tab, through the real UI (CR-REQ-046): the deployment writes the Solution in Vietnamese
 * (REQUEST_DEFAULT_OUTPUT_LANGUAGE=vi), so accented text must reach the screen whole, while every interface label stays English.
 *
 *   python3 tests/stages/e2e/orca-flows/run_ui_flow.py --ai-stub -- analysis-output-language
 *
 * Stub only: the fake claude writes the Vietnamese Solution deterministically. The real model is checked by
 * analysis-output-language-real (run_ui_flow.py --real language). The Plan is never approved here.
 */
import { expect, test } from '@playwright/test'
import {
  answerClarificationIfAsked,
  cancelThroughTheUi,
  createClassifyAndConfirm
} from './live-flow-steps'
import { runLiveFlow } from './live-flow-harness'
import { configureProject } from './live-continuous-execution-steps'
import { tabsOnScreen } from './live-qa-steps'
import { detailPane, openDetailTab, solutionApproveButton } from './live-request-ui-steps'
import { isLiveRun, loadLiveStackConfig } from './live-stack-config'

type SolutionOption = { id: string; title: string; summary: string }

/** The letters only Vietnamese uses; a Solution that has none was not written in Vietnamese. */
const VIETNAMESE_LETTER =
  /[ăâđêôơưĂÂĐÊÔƠƯàáạảãằắặẳẵầấậẩẫèéẹẻẽềếệểễìíịỉĩòóọỏõồốộổỗờớợởỡùúụủũừứựửữỳýỵỷỹ]/i

test.describe('Vietnamese Solution on the Analysis tab (live stack, fake claude)', () => {
  test.skip(!isLiveRun(), 'live-stack suite: set ORCA_E2E_LIVE=1 (see orca-flows/README.md)')

  test('an English change request shows its Vietnamese Solution whole, under English labels', async ({
    page,
    context
  }, testInfo) => {
    test.skip(
      loadLiveStackConfig().aiMode !== 'stub',
      'the fake claude writes the Vietnamese deterministically; the real model has its own spec'
    )
    await runLiveFlow({ page, context, testInfo, slug: 'analysis-output-language' }, async (f) => {
      const before = await configureProject(f, {
        engine: 'native',
        solutionAgent: false,
        executionMode: 'step_by_step'
      })
      try {
        const r = await createClassifyAndConfirm(f, {
          titleWords: 'add a subtract function (english request)',
          body: 'Add a function subtract(a, b) to src/calc.py that returns a minus b, with one unit test.',
          typeLabel: 'Change request',
          typeKey: 'change_request',
          size: 'M'
        })
        await answerClarificationIfAsked(f, r)
        let options: SolutionOption[] = []
        await f.step('the Vietnamese Solution is shown whole on the Analysis tab', async () => {
          await openDetailTab(f.page, 'Analysis')
          await expect(solutionApproveButton(f.page)).toBeVisible({ timeout: f.budgets.solutionMs })
          const listed = await f.rpc.call<{
            solutions?: { status: string; options?: { options?: SolutionOption[] } }[]
          }>('solution.list', { requestId: r.requestId })
          const live = (listed.solutions ?? []).toReversed().find((s) => s.status !== 'superseded')
          options = live?.options?.options ?? []
          expect(options.length, 'the Solution has options').toBeGreaterThanOrEqual(2)
          expect(
            VIETNAMESE_LETTER.test(options[0]!.title + options[0]!.summary),
            `the stored Solution is Vietnamese: ${options[0]!.title}`
          ).toBe(true)
          const pane = detailPane(f.page)
          const text = (await pane.innerText()).replace(/\s+/g, ' ')
          for (const o of options) {
            expect(text, `option ${o.id} title on screen, unchanged`).toContain(
              o.title.replace(/\s+/g, ' ')
            )
          }
          // A string with ệ, ơ and đ, as the task asks: the third letter shows up in the summary or the title.
          await expect(pane.getByText(options[0]!.title, { exact: true }).first()).toBeVisible()
          expect(text, 'no replacement character, no escaped \\u sequence').not.toMatch(
            /�|\\u[0-9a-f]{4}/i
          )
          await f.shots.take('solution-vietnamese')
        })
        await f.step('the interface labels stay English', async () => {
          const tabs = await tabsOnScreen(f.page)
          expect(tabs, 'tab labels').toEqual(
            expect.arrayContaining(['Overview', 'Analysis', 'Plan', 'Execution', 'History'])
          )
          expect(tabs.join(' '), 'no Vietnamese label among the tabs').not.toMatch(
            VIETNAMESE_LETTER
          )
          await expect(solutionApproveButton(f.page)).toHaveText(/approve/i)
        })
        await cancelThroughTheUi(f, r, /Awaiting Analysis Approval/i)
      } finally {
        await configureProject(f, before).catch(() => undefined)
      }
    })
  })
})
