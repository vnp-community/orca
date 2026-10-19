/**
 * The starting branch of a Request (CR-REQ-040 / FE-REQ-TASK-040-05), through the real UI on a LedgerLite repo with three branches
 * (main, release/1.x, and a third one the primary checkout stays on).
 *
 *   python3 tests/stages/e2e/orca-flows/run_ui_flow.py --ai-stub -- request-base-branch
 *
 * No agent writes anything: the native engine makes no worktree before a task runs, and a stub run never approves a Plan, so the
 * lock this spec can reach is the one of a finished Request ("terminal"). The locks by a proposal, task or execution worktree
 * need a running agent; they are covered by RequestBaseRefRow.test.tsx.
 */
import { expect, test } from '@playwright/test'
import { advertiseNoBaseRefFeature, BASE_REF_FEATURE } from './live-base-branch-gateway-features'
import {
  baseRefRow,
  changeBranchOnOverview,
  chooseStartingBranch,
  expectRowLocked,
  openCreateDialog,
  startFromField,
  startFromTrigger,
  storedBaseRef,
  submitCreateDialog,
  typeStartingBranch
} from './live-base-branch-steps'
import { LEDGERLITE_RELEASE_BRANCH } from './fixture-repo-ledgerlite'
import { requestState, waitForApiStatus } from './live-gateway-crosschecks'
import { runLedgerliteFlow } from './live-ledgerlite-project-flow'
import {
  cancelOpenRequestThroughTheUi,
  detailHeader,
  openRequestsPage,
  selectRequestRow
} from './live-request-ui-steps'
import { dismissFirstRunTip } from './live-ui-session'
import { isLiveRun, loadLiveStackConfig } from './live-stack-config'
import { expectScreenFollowsService, UI_REFRESH_GRACE_MS } from './live-status-sync'

const THIRD_BRANCH = 'feature/wip'
// "add support for" and "new feature" steer the fake classifier to change_request, a type whose flow makes worktrees.
const BODY = 'New feature: add support for a weekly limit next to the daily limit of the ledger.'

test.describe('starting branch of a Request (live stack)', () => {
  test.skip(!isLiveRun(), 'live-stack suite: set ORCA_E2E_LIVE=1 (see orca-flows/README.md)')
  // Why: the Request is classified by the fake claude, so the type (and with it whether the row is shown) is the one the title steers to.
  test.skip(
    () => loadLiveStackConfig().aiMode !== 'stub',
    'this spec drives the fake claude: run it with run_ui_flow.py --ai-stub'
  )

  test('a chosen branch is stored and shown, can be changed until the Request is finished, then it is locked', async ({
    page,
    context
  }, testInfo) => {
    await runLedgerliteFlow(
      {
        page,
        context,
        testInfo,
        slug: 'base-branch-choose',
        repo: { releaseBranch: true, thirdBranch: THIRD_BRANCH }
      },
      async (f, repo) => {
        const title = `${f.cfg.runPrefix} weekly limit on release`
        let requestId = ''
        await f.step(
          'the Create request dialog offers "Default branch" as the starting branch',
          async () => {
            await openRequestsPage(f.page)
            await openCreateDialog(f.page, { title, body: BODY, projectName: f.project.name })
            await expect(startFromField(f.page)).toBeVisible({ timeout: 30_000 })
            await expect(startFromTrigger(f.page)).toContainText(/^Default branch/)
            await f.shots.take('create-dialog-default-chip')
          }
        )
        await f.step(`choose ${LEDGERLITE_RELEASE_BRANCH} and create`, async () => {
          await chooseStartingBranch(f.page, LEDGERLITE_RELEASE_BRANCH)
          await expect(
            f.page.getByRole('button', { name: 'Use default branch' }),
            'a chosen branch can be dropped again'
          ).toBeVisible()
          await f.shots.take('create-dialog-branch-chosen')
          requestId = await submitCreateDialog(f, title)
        })
        await f.step(
          'the service stored the branch and the Overview says where worktrees start',
          async () => {
            expect(await storedBaseRef(f, requestId)).toBe(LEDGERLITE_RELEASE_BRANCH)
            const stored = await requestState(f.rpc, requestId)
            expect(stored.baseRefLocked, 'no worktree exists yet').toBe(false)
            await expect(baseRefRow(f.page)).toContainText(
              `Worktrees start from ${LEDGERLITE_RELEASE_BRANCH}`
            )
            await expect(baseRefRow(f.page).getByTestId('base-ref-edit')).toBeVisible()
            await f.shots.take('overview-branch-row')
            expect(repo.currentBranch(), 'the primary checkout was never moved').toBe(THIRD_BRANCH)
          }
        )
        await f.step(
          'wait for a state that waits for the person, so the screen holds the current version before the edits',
          async () => {
            await expectScreenFollowsService(f.page, f.rpc, f.gaps, {
              requestId,
              title,
              step: 'CLASSIFY',
              apiStatus: ['awaiting_type_confirmation'],
              chip: /Awaiting Type Confirmation/i,
              apiWaitMs: f.budgets.classifyMs
            })
          }
        )
        await f.step(
          `change the branch to ${THIRD_BRANCH} on the Overview; it survives a reload`,
          async () => {
            await changeBranchOnOverview(f, requestId, THIRD_BRANCH)
            await f.shots.take('overview-branch-changed')
            await f.page.reload({ waitUntil: 'domcontentloaded' })
            await dismissFirstRunTip(f.page)
            await openRequestsPage(f.page)
            await selectRequestRow(f.page, title)
            await expect(baseRefRow(f.page)).toContainText(`Worktrees start from ${THIRD_BRANCH}`, {
              timeout: UI_REFRESH_GRACE_MS
            })
          }
        )
        await f.step('choosing "Default branch" on the Overview stores the empty ref', async () => {
          await changeBranchOnOverview(f, requestId, null)
        })
        await f.step(
          'cancel the Request: the row is locked with the reason, and the service refuses a change',
          async () => {
            await cancelOpenRequestThroughTheUi(f.page, `${f.cfg.runPrefix}: starting branch spec`)
            await waitForApiStatus(f.rpc, requestId, ['cancelled'], 30_000)
            await expect(detailHeader(f.page)).toContainText(/Cancelled/i, {
              timeout: UI_REFRESH_GRACE_MS
            })
            await expectRowLocked(f.page, /finished, so the branch can no longer be changed/)
            await f.shots.take('overview-branch-locked')
            const after = await requestState(f.rpc, requestId)
            expect(after.baseRefLocked).toBe(true)
            expect(after.baseRefLockReason).toBe('terminal')
            await expect(
              f.rpc.call('request.setBaseRef', {
                id: requestId,
                baseRef: 'main',
                expectedVersion: after.version
              })
            ).rejects.toThrow(/REQUEST_BASE_REF_LOCKED/)
          }
        )
      }
    )
  })

  test('without a choice the default branch applies; a branch that does not exist is refused under the field and the dialog stays', async ({
    page,
    context
  }, testInfo) => {
    await runLedgerliteFlow(
      {
        page,
        context,
        testInfo,
        slug: 'base-branch-default',
        repo: { releaseBranch: true, thirdBranch: THIRD_BRANCH }
      },
      async (f) => {
        const title = `${f.cfg.runPrefix} weekly limit default branch`
        const missing = 'release/9.99'
        await f.step(`typing ${missing} and creating is refused under the field`, async () => {
          await openRequestsPage(f.page)
          await openCreateDialog(f.page, { title, body: BODY, projectName: f.project.name })
          await expect(startFromField(f.page)).toBeVisible({ timeout: 30_000 })
          await typeStartingBranch(f.page, missing)
          await f.page
            .getByRole('dialog', { name: 'Create request' })
            .getByRole('button', { name: 'Create', exact: true })
            .click()
          await expect(startFromField(f.page).getByRole('alert')).toContainText(
            /not found in the repository/i,
            { timeout: 30_000 }
          )
          await expect(
            f.page.getByRole('dialog', { name: 'Create request' }),
            'the dialog stays open'
          ).toBeVisible()
          await expect(
            f.page.getByRole('dialog', { name: 'Create request' }).getByLabel('Title')
          ).toHaveValue(title)
          await expect(
            startFromTrigger(f.page),
            'the typed branch is kept to correct'
          ).toContainText(missing)
          await f.shots.take('create-dialog-branch-not-found')
          const listed = await f.rpc.call<{ requests?: { title: string }[] }>('request.list', {
            projectId: f.project.id
          })
          expect(
            (listed.requests ?? []).some((r) => r.title === title),
            'a refused branch creates no Request'
          ).toBe(false)
        })
        let requestId = ''
        await f.step(
          '"Use default branch" and Create: the Request follows the default',
          async () => {
            await f.page.getByRole('button', { name: 'Use default branch' }).click()
            await expect(startFromTrigger(f.page)).toContainText(/^Default branch/)
            requestId = await submitCreateDialog(f, title)
            expect(await storedBaseRef(f, requestId), 'nothing is pinned').toBe('')
            await expect(baseRefRow(f.page)).toContainText(
              'Worktrees start from the default branch'
            )
            await f.shots.take('overview-default-branch')
          }
        )
      }
    )
  })

  test('a gateway that does not advertise the feature hides the field and the row', async ({
    page,
    context
  }, testInfo) => {
    await runLedgerliteFlow(
      {
        page,
        context,
        testInfo,
        slug: 'base-branch-flag-off',
        repo: { releaseBranch: true, thirdBranch: THIRD_BRANCH }
      },
      async (f) => {
        const title = `${f.cfg.runPrefix} weekly limit feature off`
        await f.step('control: the full gateway shows the field and the row', async () => {
          await openRequestsPage(f.page)
          await openCreateDialog(f.page, { title, body: BODY, projectName: f.project.name })
          await expect(startFromField(f.page)).toBeVisible({ timeout: 30_000 })
          await chooseStartingBranch(f.page, LEDGERLITE_RELEASE_BRANCH)
          await submitCreateDialog(f, title)
          await expect(baseRefRow(f.page)).toContainText(LEDGERLITE_RELEASE_BRANCH)
        })
        await f.step(
          `a second window whose gateway lacks "${BASE_REF_FEATURE}" shows neither`,
          async () => {
            const older = await f.page.context().newPage()
            try {
              const stripper = await advertiseNoBaseRefFeature(older)
              await older.goto('/')
              await dismissFirstRunTip(older)
              await openRequestsPage(older)
              await selectRequestRow(older, title)
              await expect(older.getByTestId('request-overview')).toBeVisible()
              await expect
                .poll(() => stripper.strippedAnswers(), {
                  message: 'the SPA was told the feature is missing'
                })
                .toBeGreaterThan(0)
              await openCreateDialog(older, {
                title: `${title} second`,
                body: BODY,
                projectName: f.project.name
              })
              // The repo note appears when the project's repos are known, which is when the field would have appeared.
              await expect(older.getByTestId('create-request-repo-note')).toBeVisible({
                timeout: 30_000
              })
              await older.waitForTimeout(2_000)
              await expect(startFromField(older)).toHaveCount(0)
              await older.screenshot({ path: f.testInfo.outputPath('flag-off-create-dialog.png') })
              await older.keyboard.press('Escape')
              await expect(older.getByRole('dialog')).toBeHidden()
              await expect(baseRefRow(older), 'no row without the feature').toHaveCount(0)
            } finally {
              await older.close()
            }
          }
        )
      }
    )
  })
})
