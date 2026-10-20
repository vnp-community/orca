/**
 * COST CLASS C, run with the owner's explicit consent: a change_request taken from backlog BL-E07 (ci-cd-kit) of the owner's own
 * project vnp-qa-platform, driven through the UI with the REAL Claude Code: Create request from the branch picked in the dropdown,
 * AI classification, the readiness Clarification answered from the backlog, the Vietnamese Analysis approved, the OpenSpec Plan that
 * appears without Generate, its Documents, "Approve and run all steps", and the tasks run in worktrees next to the repo.
 *
 * The owner's checkout, its refs, its worktrees and every remote stay untouched (fingerprints before/after, `git push` blocked for the
 * whole run by run_ui_flow.py); the new worktrees and branches are KEPT and listed. Stop-loss: 110 min hard limit, a task that fails
 * twice or a failed plan run stops the run, the Request is cancelled through the UI and the claude processes are killed.
 *
 *   python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real ci-cd-kit
 *   ORCA_E2E_CICDKIT_STOP=dialog python3 .../run_ui_flow.py --real ci-cd-kit   # rehearsal: fills the dialog, creates nothing, no AI cost
 */
import { copyFileSync, mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test } from '@playwright/test'
import {
  CICDKIT_BRANCH,
  CICDKIT_PROJECT_NAME,
  CICDKIT_REPO,
  CICDKIT_TITLE,
  readBacklog,
  requestBody
} from './ci-cd-kit-backlog'
import { ownRequestIds } from './ci-cd-kit-fixture'
import { runCicdKitFlow } from './ci-cd-kit-harness'
import { describeNewWorktrees, staticChecksOfWorktree } from './ci-cd-kit-git-evidence'
import {
  readPlanDocuments,
  reviewPlanBeforeRunAll,
  waitForAutoPlan,
  type AutoPlanSeen,
  type DocumentsSeen,
  type PlanTaskRow
} from './ci-cd-kit-plan-steps'
import { renderReportVi, Timeline, type ReportFacts, type WorktreeFact } from './ci-cd-kit-report'
import {
  answerAskBackFromBacklog,
  answerClarificationFromBacklog,
  type QuestionAnswered
} from './ci-cd-kit-clarification-steps'
import {
  classifyAndConfirm,
  createRequestFromBacklog,
  recordAnalysisLanguage,
  type ClassificationSeen,
  type LanguageFinding
} from './ci-cd-kit-request-steps'
import {
  approveRunAll,
  collectCost,
  followRunAll,
  recordFinishedScreens,
  type RunRecord
} from './ci-cd-kit-run-steps'
import { chooseStartingBranch, openCreateDialog } from './live-base-branch-steps'
import { approveAnalysis } from './live-analysis-approval-steps'
import type { RequestUnderTest } from './live-flow-steps'
import { cancelThroughTheUi } from './live-flow-steps'
import { requestState } from './live-gateway-crosschecks'
import { openRequestsPage } from './live-request-ui-steps'
import { killClaudeUnder } from './live-safety-invariants'
import { isLiveRun } from './live-stack-config'
import { realRunBudgetMs } from './real-flow-harness'

const MINUTE = 60_000

test.describe("A change request from the owner's backlog item BL-E07, from the UI, with the real Claude", () => {
  test.skip(!isLiveRun(), 'live-stack suite: set ORCA_E2E_LIVE=1 (see orca-flows/README.md)')

  test('BL-E07 ci-cd-kit: create, clarify, analyse in Vietnamese, auto plan, documents, run all steps, worktrees from the chosen branch', async ({
    page,
    context
  }, testInfo) => {
    await runCicdKitFlow({ page, context, testInfo }, async (f, run) => {
      const item = readBacklog()
      const timeline = new Timeline()
      const findings: string[] = []
      const facts: Partial<ReportFacts> = {}
      let r: RequestUnderTest | undefined
      let classification: ClassificationSeen = { type: '', size: '' }
      let questions: QuestionAnswered[] = []
      let language: LanguageFinding | null = null
      let plan: AutoPlanSeen | null = null
      let docs: DocumentsSeen = { tabs: [], leaks: [] }
      let tasks: PlanTaskRow[] = []
      let record: RunRecord | null = null
      let runAllUsed = false
      let worktrees: WorktreeFact[] = []
      const staticChecks: string[] = []
      let costLines: string[] = []
      const limitMs = realRunBudgetMs('execute')
      f.budgets.classifyMs = 3 * MINUTE
      f.budgets.solutionMs = 25 * MINUTE
      f.budgets.planMs = 45 * MINUTE

      // Rehearsals that spend (almost) nothing: `dialog` creates no Request; `clarify` creates one, lets the AI classify it (a few cents),
      // fills the Clarification without submitting it, and cancels the Request through the UI.
      const stopAt = process.env.ORCA_E2E_CICDKIT_STOP ?? ''
      if (stopAt === 'dialog') {
        await f.step(
          'REHEARSAL: the dialog is filled and the branch picked, nothing is created',
          async () => {
            await openRequestsPage(f.page)
            const dialog = await openCreateDialog(f.page, {
              title: CICDKIT_TITLE,
              body: requestBody(item),
              projectName: CICDKIT_PROJECT_NAME
            })
            await expect(dialog.getByTestId('create-request-repo-note')).toContainText(
              `${CICDKIT_PROJECT_NAME} (${CICDKIT_REPO}) on ${f.cfg.devServerName}`,
              { timeout: 15_000 }
            )
            await chooseStartingBranch(f.page, CICDKIT_BRANCH)
            await f.shots.take('rehearsal-dialog')
            await f.page.keyboard.press('Escape')
            await dialog.getByRole('button', { name: 'Cancel' }).click()
            await expect(dialog).toBeHidden({ timeout: 10_000 })
          }
        )
        return
      }

      try {
        r = await createRequestFromBacklog(
          f,
          run.tap,
          item,
          timeline,
          stopAt === 'clarify'
            ? CICDKIT_TITLE.replace('(BL-E07)', '(BL-E07 rehearsal)')
            : CICDKIT_TITLE
        )
        classification = await classifyAndConfirm(f, r, timeline)
        questions = await answerClarificationFromBacklog(f, r, item, timeline, {
          submit: stopAt !== 'clarify'
        })
        if (stopAt === 'clarify') {
          await cancelThroughTheUi(f, r, /./)
          return
        }
        const askedBack: QuestionAnswered[] = []
        await approveAnalysis(f, r, {
          autoStarted: questions.length > 0,
          next: { api: 'planning', chip: /Planning/i },
          answerAskBack: answerAskBackFromBacklog(item, askedBack)
        })
        questions.push(...askedBack)
        timeline.mark('analysis approved')
        const lang = await recordAnalysisLanguage(f, r)
        language = lang
        if (!lang.isVietnamese) {
          findings.push(
            `the Analysis is not Vietnamese although REQUEST_DEFAULT_OUTPUT_LANGUAGE=vi (${lang.sample.slice(0, 120)})`
          )
        }
        await f.shots.take('analysis-language')

        plan = await waitForAutoPlan(f, r, timeline, 45 * MINUTE)
        docs = await readPlanDocuments(f)
        timeline.mark('plan documents read')
        tasks = await reviewPlanBeforeRunAll(f, r)
        const proposal = describeNewWorktrees(run.fx.taskWorktrees(), run.fx.baseTip).filter((w) =>
          /-request-\d+-proposal$/.test(w.path)
        )
        if (proposal.length === 0 || !proposal.every((w) => w.descendsFromBase)) {
          findings.push(
            `the proposal worktree is missing or does not descend from ${CICDKIT_BRANCH}: ${JSON.stringify(proposal.map((w) => [w.path, w.descendsFromBase]))}`
          )
        }

        await approveRunAll(f, r, timeline)
        runAllUsed = true
        record = await followRunAll(
          f,
          r,
          () => run.fx.workRoots!(),
          timeline,
          Math.max(MINUTE, limitMs - timeline.totalMs() - 5 * MINUTE)
        )
        findings.push(...record.notes)
        await recordFinishedScreens(f, r, findings)
        expect(run.fx.headSha(), "the owner's checkout kept its commit").toBe(run.fx.baseTip)
      } catch (e) {
        findings.push(`STOPPED: ${(e as Error).message.split('\n')[0].slice(0, 300)}`)
        const needsYou = (e as Error).message.startsWith('NEEDS-YOU')
        // Why: stop-loss. Stop spending first (kill the claude processes of the run), then cancel the Request through the UI;
        // a run that only waits for the owner's approval is left open for them (and out of the cleanup net).
        if (needsYou && r) {
          ownRequestIds.delete(r.requestId)
        } else {
          killClaudeUnder(run.fx.workRoots!())
        }
        if (r && !needsYou) {
          const status = String(
            (await requestState(f.rpc, r.requestId).catch(() => ({ status: '' }))).status
          )
          if (status && !['completed', 'cancelled'].includes(status)) {
            await f.shots.take('stop-loss-before-cancel').catch(() => {})
            await cancelThroughTheUi(f, r, /./).catch((c: Error) =>
              findings.push(`cancel through the UI failed: ${c.message.split('\n')[0]}`)
            )
          }
        }
        throw e
      } finally {
        if (r) {
          const state = await requestState(f.rpc, r.requestId).catch(() => ({
            status: '?',
            number: null
          }))
          worktrees = describeNewWorktrees(run.fx.taskWorktrees(), run.fx.baseTip)
          for (const w of worktrees) {
            staticChecks.push(...staticChecksOfWorktree(w, run.fx.baseTip))
            if (!w.descendsFromBase) {
              findings.push(
                `worktree ${w.path} does not descend from ${CICDKIT_BRANCH} (${run.fx.baseTip.slice(0, 10)})`
              )
            }
          }
          costLines = await collectCost(f, r).catch((c: Error) => [
            `cost unavailable: ${c.message.slice(0, 100)}`
          ])
          Object.assign(facts, {
            requestNumber: Number(state.number) || null,
            requestId: r.requestId,
            finalStatus: String(state.status)
          })
        }
        const dir = process.env.ORCA_E2E_REPORT_DIR || testInfo.outputDir
        mkdirSync(dir, { recursive: true })
        const report = renderReportVi({
          requestNumber: null,
          requestId: '',
          finalStatus: 'chưa tạo',
          baseBranch: CICDKIT_BRANCH,
          baseTip: run.fx.baseTip,
          aiProposed: classification,
          questions,
          language: language
            ? {
                isVietnamese: language.isVietnamese,
                accentedPercent: language.service.accentedLetterRatio * 100,
                sample: language.sample
              }
            : null,
          planWrittenWithoutGenerate: plan ? !plan.generateSeen : null,
          planWaitMs: plan?.waitMs ?? null,
          documentTabs: docs.tabs,
          secretsOnScreen: docs.leaks,
          runAllUsed,
          tasks: (
            record?.timings ??
            tasks.map((t) => ({ type: t.type, title: t.title, status: t.status }))
          ).map((t) => ({ type: t.type, title: t.title, status: t.status })),
          worktrees,
          staticChecks,
          costLines,
          findings: [...findings, ...f.lags.map((l) => `độ trễ màn hình: ${l}`)],
          timeline: timeline.lines(),
          screenshotsDir: testInfo.outputDir,
          ...facts
        })
        writeFileSync(join(dir, 'ci-cd-kit-report.vi.md'), report)
        if (dir !== testInfo.outputDir) {
          copyFileSync(
            join(dir, 'ci-cd-kit-report.vi.md'),
            join(testInfo.outputDir, 'ci-cd-kit-report.vi.md')
          )
        }
        console.log(
          `[ci-cd-kit] report written to ${join(dir, 'ci-cd-kit-report.vi.md')}\n${report}`
        )
        testInfo.annotations.push({
          type: 'new-worktrees',
          description: worktrees.map((w) => `${w.path} [${w.branch}]`).join(' | ')
        })
      }
      expect(findings, 'findings of the run (language, proposal base, screens)').toEqual([])
      for (const w of worktrees) {
        expect(w.descendsFromBase, `${w.path} descends from ${CICDKIT_BRANCH}`).toBe(true)
      }
    })
  })
})
