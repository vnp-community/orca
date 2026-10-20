/**
 * The real model answers an ENGLISH question in Vietnamese (CR-REQ-046, cost class B: one read-only analysis, a small cost):
 * the Output language directive must produce valid JSON with English keys and a Vietnamese `answer_markdown`, whole, not cut.
 *
 *   python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real language
 *
 * Needs REQUEST_DEFAULT_OUTPUT_LANGUAGE=vi on the stack (the compose default). Prints the run's usage figures (tokens, cost, duration) and
 * the first lines of the answer so a person can read them; the figures are what the English comparison run is measured against.
 */
import { expect, test } from '@playwright/test'
import { answerClarificationIfAsked, createClassifyAndConfirm } from './live-flow-steps'
import { approveAnalysis } from './live-analysis-approval-steps'
import { isLiveRun } from './live-stack-config'
import { detailPane, openDetailTab } from './live-request-ui-steps'
import { runRealFlow } from './real-flow-harness'
import { vietnameseShare } from './live-vietnamese-text-check'

const READ_ONLY_TOOLS = new Set(['Read', 'Glob', 'Grep'])

type RunRow = {
  kind: string
  status: string
  errorCode?: string
  errorMessage?: string
  stats?: Record<string, unknown>
}
type SolutionRow = { kind: string; options?: Record<string, unknown> }

test.describe('Vietnamese answer from the real Claude, through the UI', () => {
  test.skip(!isLiveRun(), 'live-stack suite: set ORCA_E2E_LIVE=1 (see orca-flows/README.md)')

  test('an English question is answered in Vietnamese, as valid JSON, and shown whole', async ({
    page,
    context
  }, testInfo) => {
    await runRealFlow(
      { page, context, testInfo, slug: 'real-language-question', mode: 'readonly' },
      async (f, fx) => {
        const headBefore = fx.headSha()
        const r = await createClassifyAndConfirm(f, {
          titleWords: 'explain max_of and its test',
          body: 'What does the max_of function in src/calc.py do, which edge cases does it handle, and which test covers it? Explain it in detail.',
          typeLabel: 'Question',
          typeKey: 'question',
          size: 'S'
        })
        const asked = await answerClarificationIfAsked(f, r)
        await approveAnalysis(f, r, {
          autoStarted: asked,
          next: { api: 'completed', chip: /Completed/i }
        })
        await f.step('the run succeeded and the document is valid, with English keys', async () => {
          const listed = await f.rpc.call<{ runs?: RunRow[]; solutions?: SolutionRow[] }>(
            'solution.list',
            { requestId: r.requestId }
          )
          const run = (listed.runs ?? []).findLast((x) => x.kind === 'answer')
          expect(
            run?.status,
            `the answer run succeeded (${run?.errorCode ?? ''} ${run?.errorMessage ?? ''})`
          ).toBe('succeeded')
          const doc = (listed.solutions ?? []).findLast((s) => s.kind === 'answer')?.options ?? {}
          expect(Object.keys(doc).sort(), 'JSON keys stay English').toEqual(
            expect.arrayContaining(['answer_markdown', 'confidence', 'kind', 'schema_version'])
          )
          expect(doc.kind).toBe('answer')
          const answer = String(doc.answer_markdown ?? '')
          const share = vietnameseShare(answer)
          console.log(
            `[language-real] usage ${JSON.stringify(run?.stats ?? {})}; answer ${answer.length} chars, accented letters ${(share.accentedLetterRatio * 100).toFixed(1)}%, ` +
              `Vietnamese stop words ${share.vietnameseStopWords}, English stop-word ratio ${(share.englishStopWordRatio * 100).toFixed(1)}%`
          )
          console.log(`[language-real] answer head: ${answer.slice(0, 400).replace(/\s+/g, ' ')}`)
          expect(
            share.accentedLetterRatio,
            'the answer is mostly Vietnamese (accented letters)'
          ).toBeGreaterThanOrEqual(0.15)
          expect(
            share.vietnameseStopWords,
            'the answer uses common Vietnamese words'
          ).toBeGreaterThanOrEqual(3)
          expect(share.englishStopWordRatio, 'the answer is not English').toBeLessThan(0.1)
          expect(answer, 'no replacement character from a cut multi-byte letter').not.toContain('�')
          expect(answer.trimEnd(), 'the answer is not cut mid-sentence').toMatch(
            /[.!?:)`*\p{L}\d]$/u
          )
        })
        await f.step('the repo was only read and the screen shows the answer whole', async () => {
          expect(fx.repoStatus(), 'the fixture working tree is clean').toBe('')
          expect(fx.headSha(), 'no commit was made').toBe(headBefore)
          const tools = fx.guardCalls().map((l) => l.split(' ')[1])
          expect(tools.length, 'the tool guard saw the agent work').toBeGreaterThan(0)
          expect(
            tools.filter((t) => !READ_ONLY_TOOLS.has(t)),
            'read-only tools only'
          ).toEqual([])
          await openDetailTab(f.page, 'Analysis')
          await expect(detailPane(f.page)).toContainText(/max_of/)
          await f.shots.take('answer-vietnamese')
        })
      }
    )
  })
})
