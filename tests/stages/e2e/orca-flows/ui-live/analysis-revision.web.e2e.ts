/**
 * Revising an analysis by prompt (CR-REQ-041 / FE-REQ-TASK-041-04), through the real UI on a LedgerLite repo and the fake claude.
 *
 *   python3 tests/stages/e2e/orca-flows/run_ui_flow.py --ai-stub -- analysis-revision
 *
 * The fake claude answers the same document whatever the feedback says, so what is checked is structure: the versions and their
 * statuses, the new Approval of every round, the feedback each version carries, read-only old versions, the Activity line and
 * the refusals. The analysis is never approved (that would start the next stage) and the Request is cancelled at the end.
 * Not covered here: a second account (an approver who is not the reporter, a read-only account); the stack has no way to make
 * accounts from a spec without the admin API, so those two roles are left to the backend authorization tests.
 */
import { expect, test } from '@playwright/test'
import {
  expectOlderVersionsReadOnly,
  listVersions,
  openAnalysisApprovals,
  reachAnalysisReady,
  reviseAnalysis,
  versionButtons
} from './live-analysis-revision-steps'
import {
  expectRevisionActivity,
  expectRevisionAfterRequestMovedOn,
  expectTooLongKeepsText
} from './live-analysis-revision-outcome-steps'
import {
  answerClarificationIfAsked,
  cancelThroughTheUi,
  createClassifyAndConfirm,
  type RequestUnderTest
} from './live-flow-steps'
import type { LiveFlow } from './live-flow-harness'
import { configureProject } from './live-continuous-execution-steps'
import { runLedgerliteFlow } from './live-ledgerlite-project-flow'
import { isLiveRun, loadLiveStackConfig } from './live-stack-config'

// Why: the checks are about structure, which the fake claude makes deterministic; a real model would spend usage to prove nothing more.
const STUB_ONLY = 'this spec drives the fake claude: run it with run_ui_flow.py --ai-stub'

type Kind = {
  slug: string
  titleWords: string
  body: string
  typeLabel: string
  typeKey: string
  size: string
}

const CHANGE_REQUEST: Kind = {
  slug: 'revision-change-request',
  titleWords: 'add support for a weekly limit',
  body: 'New feature: add support for a weekly limit next to the daily limit of the ledger.',
  typeLabel: 'Change request',
  typeKey: 'change_request',
  size: 'M'
}

/** A project on the native engine with the plain model (no knowledge hub), put back after the body. */
async function withNativeProject(f: LiveFlow, body: () => Promise<void>): Promise<void> {
  const before = await configureProject(f, {
    engine: 'native',
    solutionAgent: false,
    executionMode: 'step_by_step'
  })
  try {
    await body()
  } finally {
    await configureProject(f, before).catch(() => undefined)
  }
}

async function analysisReady(f: LiveFlow, kind: Kind): Promise<RequestUnderTest> {
  const r = await createClassifyAndConfirm(f, {
    titleWords: kind.titleWords,
    body: kind.body,
    typeLabel: kind.typeLabel,
    typeKey: kind.typeKey,
    size: kind.size
  })
  const asked = await answerClarificationIfAsked(f, r)
  await reachAnalysisReady(f, r, asked)
  return r
}

// Why: the fake claude answers in milliseconds; this code makes it pause so the panel's "Analyzing version n…" line is on screen.
const SLOW_ANSWER = '[stub:slow]'

test.describe('revising an analysis by prompt (live stack)', () => {
  test.skip(!isLiveRun(), 'live-stack suite: set ORCA_E2E_LIVE=1 (see orca-flows/README.md)')
  test.skip(() => loadLiveStackConfig().aiMode !== 'stub', STUB_ONLY)

  test('change request: revised twice, three versions, the old ones read-only, one Approval at a time', async ({
    page,
    context
  }, testInfo) => {
    await runLedgerliteFlow({ page, context, testInfo, slug: CHANGE_REQUEST.slug }, async (f) => {
      await withNativeProject(f, async () => {
        const r = await analysisReady(f, CHANGE_REQUEST)
        const first = `${f.cfg.runPrefix} revision one: compare the options on cost. ${SLOW_ANSWER}`
        const second = `${f.cfg.runPrefix} revision two: add a rollback step\nand name the owner of each task. ${SLOW_ANSWER}`
        const approval1 = await openAnalysisApprovals(f, r.requestId)
        expect(approval1.length, 'the first version has one open Approval').toBe(1)

        const approval2 = await reviseAnalysis(f, r, {
          text: first,
          version: 2,
          previousApprovals: approval1
        })
        const approval3 = await reviseAnalysis(f, r, {
          text: second,
          version: 3,
          previousApprovals: [...approval1, approval2]
        })
        expect(
          new Set([...approval1, approval2, approval3]).size,
          'three different Approvals'
        ).toBe(3)

        await f.step('the service lists three versions: two superseded, one waiting', async () => {
          const versions = await listVersions(f, r.requestId)
          expect(versions.map((v) => v.revisionFeedback ?? '')).toEqual(['', first, second])
          expect(versions.slice(0, 2).every((v) => v.status === 'superseded')).toBe(true)
          await expect(versionButtons(f)).toHaveCount(3)
        })

        await expectOlderVersionsReadOnly(f, [first, second])
        await expectRevisionActivity(f, r, {
          lines: 2,
          secrets: [first, 'revision two', 'rollback step']
        })
        await expectTooLongKeepsText(f, r)
        await cancelThroughTheUi(f, r, /Awaiting Analysis Approval/i)
      })
    })
  })

  test('change request: a revision sent from an out-of-date screen is refused and keeps the text', async ({
    page,
    context
  }, testInfo) => {
    await runLedgerliteFlow(
      { page, context, testInfo, slug: 'revision-stale-screen' },
      async (f) => {
        await withNativeProject(f, async () => {
          const r = await analysisReady(f, CHANGE_REQUEST)
          await expectRevisionAfterRequestMovedOn(
            f,
            r,
            `${f.cfg.runPrefix} too late: compare on cost.`
          )
        })
      }
    )
  })

  const readOnlyKinds: (Kind & { label: string })[] = [
    {
      slug: 'revision-bug',
      label: 'bug: the diagnosis is revised once',
      titleWords: 'settings page crash on save',
      body: 'Saving the settings page crashes with an error when the badge colour is empty. It worked last week.',
      typeLabel: 'Bug',
      typeKey: 'bug',
      size: 'M'
    },
    {
      slug: 'revision-spike',
      label: 'spike: the findings are revised once',
      titleWords: 'investigate caching of daily totals',
      body: 'Time-boxed: investigate whether the daily totals of the ledger could be cached, and what would invalidate them.',
      typeLabel: 'Spike',
      typeKey: 'spike',
      size: 'S'
    },
    {
      slug: 'revision-question',
      label: 'question: the answer is revised once',
      titleWords: 'how does the daily limit work',
      body: 'How does the daily limit of the ledger work, and which test covers it?',
      typeLabel: 'Question',
      typeKey: 'question',
      size: 'S'
    }
  ]

  for (const kind of readOnlyKinds) {
    test(kind.label, async ({ page, context }, testInfo) => {
      await runLedgerliteFlow({ page, context, testInfo, slug: kind.slug }, async (f) => {
        await withNativeProject(f, async () => {
          const r = await analysisReady(f, kind)
          const approval1 = await openAnalysisApprovals(f, r.requestId)
          expect(approval1.length).toBe(1)
          const text = `${f.cfg.runPrefix} one revision: say what is still unknown. ${SLOW_ANSWER}`
          await reviseAnalysis(f, r, { text, version: 2, previousApprovals: approval1 })
          await expectOlderVersionsReadOnly(f, [text])
          await expectRevisionActivity(f, r, { lines: 1, secrets: [text] })
          await cancelThroughTheUi(f, r, /Awaiting Analysis Approval/i)
        })
      })
    })
  }
})
