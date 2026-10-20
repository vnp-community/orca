/**
 * Reading the plan the AI wrote (CR-REQ-045 / FE-REQ-TASK-045-08), through the real UI on a LedgerLite repo.
 *
 *   python3 tests/stages/e2e/orca-flows/run_ui_flow.py --ai-stub -- plan-documents
 *
 * (a) a native `bug`: the Plan tab offers "Proposal", not "Documents", and shows the summary, the notes and every task.
 * (b) an OpenSpec change: the fake claude cannot author one, so the rows are planted by plan_documents_seed.py (BE-REQ-TASK-045-09)
 *     on a native Request that waits for plan approval; the spec reads them through the screen. The seed is fake data: it is
 *     removed again, and it is not for a shared stack.
 * (c) a hotfix has neither entry: needs an urgent hotfix flow, not built here.
 * (d) the real OpenSpec case costs about 7 to 13 USD per generated plan, so it is written but skipped unless ORCA_E2E_OPENSPEC=1:
 *
 *   ORCA_E2E_OPENSPEC=1 python3 tests/stages/e2e/orca-flows/run_ui_flow.py -- plan-documents
 *
 * (run on a stack whose dev server has `openspec` and the design knowledge of the fixture; never combined with --ai-stub).
 */
import { test } from '@playwright/test'
import { cancelThroughTheUi } from './live-flow-steps'
import { tddAgentAvailable } from './fixture-repo-ledgerlite'
import {
  expectNativeProposal,
  expectOpenspecDocuments,
  plantOpenspecChange,
  removePlantedChange,
  seedAvailable
} from './live-plan-documents-steps'
import { expectRealOpenspecPlan } from './live-plan-documents-openspec-real-steps'
import { reachPlanReady, watchPlanCalls, withNativeProject } from './live-plan-revision-steps'
import { runLedgerliteFlow } from './live-ledgerlite-project-flow'
import { isLiveRun, loadLiveStackConfig } from './live-stack-config'

const STUB_ONLY = 'this spec drives the fake claude: run it with run_ui_flow.py --ai-stub'
const AT_PLAN_GATE = /Awaiting Plan Approval/i
const OPENSPEC_REAL = ['1', 'true', 'yes'].includes(
  (process.env.ORCA_E2E_OPENSPEC ?? '').toLowerCase()
)

test.describe('reading the plan the AI wrote (live stack)', () => {
  test.skip(!isLiveRun(), 'live-stack suite: set ORCA_E2E_LIVE=1 (see orca-flows/README.md)')

  test.describe('with the fake claude', () => {
    test.skip(() => loadLiveStackConfig().aiMode !== 'stub', STUB_ONLY)

    test('(a) native bug: Proposal shows summary, notes and tasks; there is no Documents entry', async ({
      page,
      context
    }, testInfo) => {
      await runLedgerliteFlow(
        { page, context, testInfo, slug: 'plan-proposal-native' },
        async (f) => {
          await withNativeProject(f, async () => {
            const r = await reachPlanReady(f, 'settings page crash on save')
            await expectNativeProposal(f, r)
            await cancelThroughTheUi(f, r, AT_PLAN_GATE)
          })
        }
      )
    })

    test('(b) OpenSpec change from planted rows: tabs in order, read on open, tasks as list and source, a large file, no secret', async ({
      page,
      context
    }, testInfo) => {
      test.skip(
        !seedAvailable(),
        'plan_documents_seed.py (BE-REQ-TASK-045-09) is not in this checkout'
      )
      await runLedgerliteFlow(
        { page, context, testInfo, slug: 'plan-documents-openspec' },
        async (f) => {
          await withNativeProject(f, async () => {
            const r = await reachPlanReady(f, 'settings page crash on documents')
            const calls = await watchPlanCalls(f, r)
            console.log(
              'plan_documents_seed.py plants FAKE data in the request database: use it on a local stack only'
            )
            const facts = plantOpenspecChange(r.requestId)
            try {
              await expectOpenspecDocuments(f, r, facts, calls)
            } finally {
              removePlantedChange(r.requestId)
            }
            await cancelThroughTheUi(f, r, AT_PLAN_GATE)
          })
        }
      )
    })

    // Why: a hotfix needs the urgent flow and a diagnosis; the component test "offers neither for a hotfix" covers it.
    test.fixme('(c) hotfix: the plan has neither Documents nor Proposal, and no revision panel', async () => {})
  })

  test.describe('real Claude with OpenSpec (about 7 to 13 USD per generated plan; asks for ORCA_E2E_OPENSPEC=1)', () => {
    test.skip(() => !OPENSPEC_REAL, 'costs 7 to 13 USD per plan: set ORCA_E2E_OPENSPEC=1 to run it')
    test.skip(() => loadLiveStackConfig().aiMode === 'stub', 'real-AI case: drop --ai-stub')
    test.skip(
      () => !tddAgentAvailable(),
      'needs tdd-agent to generate the design knowledge of the fixture'
    )

    test('(d) transfer fee on release/1.x: all documents, one revision, two versions, the right files changed, no secret', async ({
      page,
      context
    }, testInfo) => {
      test.setTimeout(3 * 60 * 60_000)
      await runLedgerliteFlow(
        {
          page,
          context,
          testInfo,
          slug: 'plan-documents-openspec-real',
          repo: { knowledge: true, releaseBranch: true }
        },
        async (f) => {
          await expectRealOpenspecPlan(f)
        }
      )
    })
  })
})
