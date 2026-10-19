/**
 * A `question` about the user's repo deepseek-harness answered by the REAL Claude Code through the UI (TASK-FE-E2E-028). The question
 * has a ground truth read from the repo itself (root package.json), so the answer is checked against the repo, not against a script.
 * Also checked on real data: Create request (note, what the SPA sends), the Requests page shell, the Answer view, and that the
 * analysis only read: the main checkout is untouched and no worktree or branch was made.
 *
 *   python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real deepseek-readonly
 */
import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test } from '@playwright/test'
import {
  createDeepseekRequest,
  expectEscapeClosesRequests,
  expectRequestsShell
} from './deepseek-request-steps'
import { runDeepseekFlow } from './deepseek-flow-harness'
import { DEEPSEEK_REPO } from './deepseek-repo-fixture'
import { approveAnalysis } from './live-analysis-approval-steps'
import { readAnswerOnScreen } from './live-add-project-own-project-steps'
import {
  answerClarificationIfAsked,
  cancelThroughTheUi,
  createClassifyAndConfirm
} from './live-flow-steps'
import { findRequestByTitle } from './live-gateway-crosschecks'
import { openRequestsPage } from './live-request-ui-steps'
import { isLiveRun } from './live-stack-config'

const QUESTION =
  'Đọc package.json ở thư mục gốc của repo: trường name là gì, engines.node yêu cầu phiên bản nào, ' +
  'và workspaces có bao nhiêu mục? Chỉ đọc, không sửa file nào.'

type Facts = { name: string; node: string; workspaces: number }

function rootFacts(): Facts {
  const pkg = JSON.parse(readFileSync(join(DEEPSEEK_REPO, 'package.json'), 'utf8')) as {
    name: string
    engines: { node: string }
    workspaces: string[]
  }
  return { name: pkg.name, node: pkg.engines.node, workspaces: pkg.workspaces.length }
}

test.describe("A question about the user's repo, answered by the real Claude (TASK-FE-E2E-028)", () => {
  test.skip(!isLiveRun(), 'live-stack suite: set ORCA_E2E_LIVE=1 (see orca-flows/README.md)')

  test('question: the answer comes from deepseek-harness and matches its package.json; the repo is only read', async ({
    page,
    context
  }, testInfo) => {
    const facts = rootFacts()
    await runDeepseekFlow(
      { page, context, testInfo, slug: 'deepseek-question', mode: 'readonly' },
      async (f, run) => {
        const headBefore = run.fx.headSha()
        const r = await createClassifyAndConfirm(f, {
          titleWords: 'root package facts',
          body: QUESTION,
          typeLabel: 'Question',
          typeKey: 'question',
          size: 'S',
          create: (title) => createDeepseekRequest(f, run, title, QUESTION)
        })
        const asked = await answerClarificationIfAsked(f, r)
        const screen = await readAnswerOnScreen(f, r)
        await f.step('the answer on screen is the stored answer and matches the repo', async () => {
          const listed = await f.rpc.call<{
            runs?: { kind: string; status: string; errorCode?: string }[]
            solutions?: { kind: string; options?: Record<string, unknown> }[]
          }>('solution.list', { requestId: r.requestId })
          const runRow = (listed.runs ?? []).findLast((x) => x.kind === 'answer')
          expect(runRow?.status, `the answer run succeeded (${runRow?.errorCode ?? ''})`).toBe(
            'succeeded'
          )
          const stored = (listed.solutions ?? []).findLast((x) => x.kind === 'answer')
          const markdown = String(stored?.options?.answer_markdown ?? '')
          expect(markdown.trim().length, 'answer_markdown is not empty').toBeGreaterThan(0)
          expect.soft(screen.view, 'the Answer box is not "No data"').not.toContain('No data')
          // Why: the screen shows facts of the repo, not a fixed phrase: the package name and node range of the stored answer are on it
          // (backslashes dropped: a table cell escapes `||` as `\\|\\|`, and tables are not rendered as tables yet).
          const onScreen = screen.view.replaceAll('\\', '')
          expect
            .soft(onScreen, 'the screen shows the package name of the stored answer')
            .toContain(facts.name)
          expect
            .soft(onScreen, 'the screen shows the node range of the stored answer')
            .toContain(facts.node)
          expect.soft(screen.confidence, 'answer-confidence shows a percentage').toMatch(/\d+%/)
          expect(markdown, `the answer names the package (${facts.name})`).toContain(facts.name)
          expect(markdown, `the answer gives the node range (${facts.node})`).toContain(facts.node)
          expect(
            new RegExp(`\\b${facts.workspaces}\\b`).test(markdown),
            `the answer counts ${facts.workspaces} workspaces`
          ).toBe(true)
          expect(screen.citations.length, 'answer-citations lists a citation').toBeGreaterThan(0)
          expect(
            screen.citations.some((c) => /package\.json/.test(c)),
            `a citation points at package.json (got ${JSON.stringify(screen.citations.slice(0, 4))})`
          ).toBe(true)
          await f.shots.take('answer-on-screen')
        })
        const second = `${f.cfg.runPrefix} second request`
        await f.step('a second Request, so the list has two rows to move between', async () => {
          await openRequestsPage(f.page)
          await createDeepseekRequest(f, run, second, 'Chỉ để kiểm tra danh sách; sẽ bị huỷ.')
        })
        await expectRequestsShell(f, [r.title, second])
        await approveAnalysis(f, r, {
          autoStarted: asked,
          next: { api: 'completed', chip: /Completed/i }
        })
        await expectEscapeClosesRequests(f, r.requestId)
        await openRequestsPage(f.page)
        const other = await findRequestByTitle(f.rpc, f.project.id, second)
        await cancelThroughTheUi(
          f,
          { requestId: other!.id, title: second },
          /Classif|New|Awaiting Type/i
        )
        await f.step(
          'the analysis only read: HEAD, status and worktrees of the repo are as before',
          async () => {
            expect(run.fx.headSha(), 'no commit was made').toBe(headBefore)
            expect(run.fx.worktrees(), 'no worktree was created for a read-only analysis').toEqual([
              run.fx.repoPath
            ])
            expect(run.fx.guardDenials(), 'the main checkout was never touched').toEqual([])
          }
        )
      }
    )
  })
})
