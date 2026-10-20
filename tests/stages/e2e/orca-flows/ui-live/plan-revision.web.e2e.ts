/**
 * Revising a plan by prompt (CR-REQ-045 / FE-REQ-TASK-045-08), through the real UI on a LedgerLite repo and the fake claude.
 *
 *   python3 tests/stages/e2e/orca-flows/run_ui_flow.py --ai-stub -- plan-revision
 *
 * The fake claude writes the same plan whatever the feedback says, so what is checked is structure: the versions and their
 * statuses, the new Approval of every round, the request each version carries, read-only old versions, the History line, the
 * refusals and a failed round that is tried again. The plan is never approved and the Request is cancelled at the end.
 * Not covered here: a second account (an approver who is not the reporter, a read-only account) and a dev server that is
 * offline. The stack has no way to make accounts or take the dev server away from a spec; the backend suite PRV-05/PRV-09 does
 * both at the API.
 *
 *   python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real plan-revision
 *
 * runs the last test only: one round with the real Claude on a throwaway repo (a small cost; only when someone asks for it).
 */
import { expect, test } from '@playwright/test'
import {
  expectFailedRoundCanBeTriedAgain,
  expectPlanRevisionActivity,
  expectPlanRevisionAfterRequestMovedOn,
  expectPlanTooLongKeepsText
} from './live-plan-revision-outcome-steps'
import {
  expectOlderPlanVersionsReadOnly,
  listPlanVersions,
  openPlanApprovals,
  reachPlanReady,
  revisePlan,
  watchPlanCalls,
  withNativeProject
} from './live-plan-revision-steps'
import { cancelThroughTheUi } from './live-flow-steps'
import { runLedgerliteFlow } from './live-ledgerlite-project-flow'
import { isLiveRun, loadLiveStackConfig } from './live-stack-config'
import { runRealFlow } from './real-flow-harness'

// Why: the checks are about structure, which the fake claude makes deterministic; a real model would spend usage to prove nothing more.
const STUB_ONLY = 'this spec drives the fake claude: run it with run_ui_flow.py --ai-stub'
// Why: the fake claude answers in milliseconds; this code makes it pause so the panel's "Writing version n…" line is on screen.
const SLOW_ANSWER = '[stub:slow]'
// Why: the first answer to a request carrying this code fails, the retry with the same text succeeds.
const FAIL_ONCE = '[stub:fail-once]'
const AT_PLAN_GATE = /Awaiting Plan Approval/i

test.describe('revising a plan by prompt (live stack)', () => {
  test.skip(!isLiveRun(), 'live-stack suite: set ORCA_E2E_LIVE=1 (see orca-flows/README.md)')

  test.describe('with the fake claude', () => {
    test.skip(() => loadLiveStackConfig().aiMode !== 'stub', STUB_ONLY)

    test('bug: revised twice, three versions, the old ones read-only, one Approval at a time, nothing saved by the browser', async ({
      page,
      context
    }, testInfo) => {
      await runLedgerliteFlow(
        { page, context, testInfo, slug: 'plan-revision-twice' },
        async (f) => {
          await withNativeProject(f, async () => {
            const r = await reachPlanReady(f, 'settings page crash on save')
            const calls = await watchPlanCalls(f, r)
            const first = `${f.cfg.runPrefix} revision one: split the work in two. ${SLOW_ANSWER}`
            const second = `${f.cfg.runPrefix} revision two: add a rollback step\nand name the owner of each task. ${SLOW_ANSWER}`
            const approval1 = await openPlanApprovals(f, r.requestId)
            expect(approval1.length, 'the first version has one open Approval').toBe(1)

            const approval2 = await revisePlan(f, r, {
              text: first,
              version: 2,
              previousApprovals: approval1
            })
            const approval3 = await revisePlan(f, r, {
              text: second,
              version: 3,
              previousApprovals: [...approval1, approval2]
            })
            expect(
              new Set([...approval1, approval2, approval3]).size,
              'three different Approvals'
            ).toBe(3)

            await f.step(
              'the service lists three versions: two superseded, one waiting',
              async () => {
                const versions = await listPlanVersions(f, r.requestId)
                expect(versions.map((v) => v.revisionFeedback ?? '')).toEqual(['', first, second])
                expect(versions.map((v) => v.status)).toEqual([
                  'superseded',
                  'superseded',
                  'current'
                ])
                expect(
                  calls.generateModes(),
                  'the browser asked twice and never saved a plan itself'
                ).toEqual(['propose', 'propose'])
                expect(calls.count('approval.reject'), 'a change request is not a rejection').toBe(
                  0
                )
              }
            )

            await expectOlderPlanVersionsReadOnly(f, [first, second])
            await expectPlanRevisionActivity(f, r, {
              lines: 2,
              secrets: [first, 'revision two', 'rollback step']
            })
            await expectPlanTooLongKeepsText(f, r)
            await cancelThroughTheUi(f, r, AT_PLAN_GATE)
          })
        }
      )
    })

    test('bug: a revision sent from an out-of-date screen is refused and keeps the text', async ({
      page,
      context
    }, testInfo) => {
      await runLedgerliteFlow(
        { page, context, testInfo, slug: 'plan-revision-stale-screen' },
        async (f) => {
          await withNativeProject(f, async () => {
            const r = await reachPlanReady(f, 'settings page crash on stale screen')
            await expectPlanRevisionAfterRequestMovedOn(
              f,
              r,
              `${f.cfg.runPrefix} too late: split the work.`
            )
          })
        }
      )
    })

    test('bug: a round that fails shows the failure, and Try again writes the version with the same request', async ({
      page,
      context
    }, testInfo) => {
      await runLedgerliteFlow(
        { page, context, testInfo, slug: 'plan-revision-failed' },
        async (f) => {
          await withNativeProject(f, async () => {
            const r = await reachPlanReady(f, 'settings page crash then retry')
            const calls = await watchPlanCalls(f, r)
            const approval1 = await openPlanApprovals(f, r.requestId)
            await expectFailedRoundCanBeTriedAgain(f, r, {
              text: `${f.cfg.runPrefix} revision that fails once: add tests. ${FAIL_ONCE}`,
              calls,
              previousApprovals: approval1
            })
            expect((await listPlanVersions(f, r.requestId)).length).toBe(2)
            await cancelThroughTheUi(f, r, AT_PLAN_GATE)
          })
        }
      )
    })
  })

  // Type B (a small cost, a throwaway repo): run only with run_ui_flow.py --real plan-revision.
  test('real Claude: one round on a bug makes version 2 with a new Approval, the repo untouched', async ({
    page,
    context
  }, testInfo) => {
    await runRealFlow(
      { page, context, testInfo, slug: 'real-plan-revision', mode: 'readonly' },
      async (f, fx) => {
        await withNativeProject(f, async () => {
          const headBefore = fx.headSha()
          const r = await reachPlanReady(f, 'clamp ignores the upper bound', {
            real: true,
            body: 'clamp(15, 0, 10) in src/calc.py returns 0 but it should return 10.'
          })
          const calls = await watchPlanCalls(f, r)
          const approval1 = await openPlanApprovals(f, r.requestId)
          const text = `${f.cfg.runPrefix} split the verification into its own task.`
          await revisePlan(f, r, { text, version: 2, previousApprovals: approval1 })
          await f.step(
            'only the plan changed: the browser proposed, the repo was not touched',
            async () => {
              expect(calls.generateModes()).toEqual(['propose'])
              expect(fx.repoStatus(), 'the fixture working tree is clean').toBe('')
              expect(fx.headSha(), 'no commit was made in the fixture repo').toBe(headBefore)
            }
          )
          await cancelThroughTheUi(f, r, AT_PLAN_GATE)
        })
      }
    )
  })
})
