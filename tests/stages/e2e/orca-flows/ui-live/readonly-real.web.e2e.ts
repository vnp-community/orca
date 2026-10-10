/**
 * Read-only analysis with the REAL Claude Code of the dev server, driven through the UI (TASK-FE-E2E-010). The agent runs
 * `claude --print ... --permission-mode plan --tools Read,Glob,Grep` on a throwaway repo; the spec proves it only read:
 * the fixture repo is untouched and the tool guard saw nothing but read tools. Never the fake claude.
 *
 *   python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real readonly
 */
import { expect, test } from '@playwright/test'
import {
  answerClarificationIfAsked,
  cancelThroughTheUi,
  createClassifyAndConfirm
} from './live-flow-steps'
import { approveAnalysis } from './live-analysis-approval-steps'
import type { LiveFlow } from './live-flow-harness'
import { isLiveRun } from './live-stack-config'
import { detailPane, openDetailTab } from './live-request-ui-steps'
import { runRealFlow } from './real-flow-harness'
import type { RealFixture } from './real-repo-fixture'

const READ_ONLY_TOOLS = new Set(['Read', 'Glob', 'Grep'])

type SolutionRow = { kind: string; status: string; options?: Record<string, unknown> }
type RunRow = { kind: string; status: string; errorCode?: string; errorMessage?: string }

/** Backend proof that the analysis ran for real and the fixture repo was only read. */
async function expectReadOnlyAnalysis(
  f: LiveFlow,
  fx: RealFixture,
  requestId: string,
  kind: string,
  headBefore: string
): Promise<SolutionRow> {
  const listed = await f.rpc.call<{ runs?: RunRow[]; solutions?: SolutionRow[] }>('solution.list', {
    requestId
  })
  const run = (listed.runs ?? []).findLast((r) => r.kind === kind)
  expect(run, `a ${kind} run was recorded`).toBeTruthy()
  expect(
    run?.status,
    `the ${kind} run succeeded (${run?.errorCode ?? ''} ${run?.errorMessage ?? ''})`
  ).toBe('succeeded')
  const solution = (listed.solutions ?? []).findLast((s) => s.kind === kind)
  expect(solution, `a Solution of kind ${kind}`).toBeTruthy()

  expect(fx.repoStatus(), 'the fixture working tree is clean after the analysis').toBe('')
  expect(fx.headSha(), 'no commit was made in the fixture repo').toBe(headBefore)
  expect(fx.branches(), 'no branch was created').toEqual(['main'])
  expect(fx.worktrees().length, 'no worktree was created').toBe(1)
  const tools = fx.guardCalls().map((l) => l.split(' ')[1])
  expect(tools.length, 'the tool guard was live and saw the agent work').toBeGreaterThan(0)
  expect(
    tools.filter((t) => !READ_ONLY_TOOLS.has(t)),
    'the agent used read-only tools only'
  ).toEqual([])
  f.lags.push(`read-only tool calls ${tools.length}`)
  return solution!
}

test.describe('Read-only analysis with the real Claude, through the UI', () => {
  test.skip(!isLiveRun(), 'live-stack suite: set ORCA_E2E_LIVE=1 (see orca-flows/README.md)')

  test('question: the answer comes from the repo, the Request completes, the repo is untouched', async ({
    page,
    context
  }, testInfo) => {
    await runRealFlow(
      { page, context, testInfo, slug: 'real-question', mode: 'readonly' },
      async (f, fx) => {
        const headBefore = fx.headSha()
        const r = await createClassifyAndConfirm(f, {
          titleWords: 'what does max_of do',
          body: 'What does the max_of function in src/calc.py do, and which test covers it?',
          typeLabel: 'Question',
          typeKey: 'question',
          size: 'S'
        })
        const asked = await answerClarificationIfAsked(f, r)
        await approveAnalysis(f, r, {
          autoStarted: asked,
          next: { api: 'completed', chip: /Completed/i }
        })
        await f.step('the answer is the real one and the repo was only read', async () => {
          const s = await expectReadOnlyAnalysis(f, fx, r.requestId, 'answer', headBefore)
          const text = String(s.options?.answer_markdown ?? '')
          expect(text, 'the answer reads the repo (names max_of and its test file)').toMatch(
            /max_of/
          )
          expect(text).toMatch(/test_calc/)
          await openDetailTab(f.page, 'Analysis')
          await expect(detailPane(f.page)).toContainText(/max_of/)
          await f.shots.take('answer-on-screen')
        })
      }
    )
  })

  test('bug: a diagnosis finds the root cause, then the Request is cancelled at planning', async ({
    page,
    context
  }, testInfo) => {
    await runRealFlow(
      { page, context, testInfo, slug: 'real-bug', mode: 'readonly' },
      async (f, fx) => {
        const headBefore = fx.headSha()
        const r = await createClassifyAndConfirm(f, {
          titleWords: 'clamp ignores the upper bound',
          body: 'clamp(15, 0, 10) in src/calc.py returns 0 but it should return 10.',
          typeLabel: 'Bug',
          typeKey: 'bug',
          size: 'S'
        })
        const asked = await answerClarificationIfAsked(f, r)
        await approveAnalysis(f, r, {
          autoStarted: asked,
          next: { api: 'planning', chip: /Planning/i }
        })
        await f.step(
          'the diagnosis names the faulty branch and the repo was only read',
          async () => {
            const s = await expectReadOnlyAnalysis(f, fx, r.requestId, 'diagnosis', headBefore)
            const root = JSON.stringify(
              (s.options as { root_cause?: unknown } | undefined)?.root_cause ?? ''
            )
            expect(root, 'the root cause points at clamp').toMatch(/clamp/i)
          }
        )
        await cancelThroughTheUi(f, r, /Planning/i)
      }
    )
  })
})
