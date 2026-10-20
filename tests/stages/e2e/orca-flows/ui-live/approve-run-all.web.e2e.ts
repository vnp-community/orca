/**
 * "Approve and run all steps" through the real UI (CR-REQ-043 / FE-REQ-TASK-043-06), on the NATIVE engine with the fake claude.
 *
 *   python3 tests/stages/e2e/orca-flows/run_ui_flow.py --ai-stub -- approve-run-all
 *
 * Four tests, each with its own throwaway project: (1) the choice, its confirmation and what it pins; (2) a normal approval and a
 * gateway that does not advertise `approval.runAll`; (3) a project that already runs on its own; (4) the project switch in
 * Project Settings. Under the stub no task really runs, so "the run finishes without another click" is the real-Claude case of
 * continuous-execute-real.web.e2e.ts (run_ui_flow.py --real continuous).
 */
import { expect, test } from '@playwright/test'
import { configureProject } from './live-continuous-execution-steps'
import { requestState } from './live-gateway-crosschecks'
import { runLiveFlow } from './live-flow-harness'
import { advertiseNoRunAllFeature, RUN_ALL_FEATURE } from './live-run-all-gateway-features'
import {
  expectAsksBeforeEachStep,
  expectBothApproveButtons,
  expectNothingDecided,
  expectRunsOnItsOwn,
  openProjectSettings,
  openRunAllDialogAndRead,
  phaseCount,
  planBar,
  reachPlanApproval,
  readEngineSettings,
  runAllButton,
  runAllDialog,
  saveModeInProjectSettings
} from './live-run-all-steps'
import {
  openRequestsPage,
  openDetailTab,
  selectRequestRow,
  detailPane
} from './live-request-ui-steps'
import { dismissFirstRunTip } from './live-ui-session'
import { isLiveRun, loadLiveStackConfig } from './live-stack-config'

const STEP_BY_STEP = {
  engine: 'native',
  solutionAgent: false,
  executionMode: 'step_by_step'
} as const

test.describe('Approve and run all steps (live stack)', () => {
  test.skip(!isLiveRun(), 'live-stack suite: set ORCA_E2E_LIVE=1 (see orca-flows/README.md)')
  // Why: the type and the Plan come from the fake claude, so the Plan has Phases and the type is the one the title steers to.
  // The Project Settings test below needs no AI at all and also runs against the dev server's own Claude (nothing is classified).
  test.describe('with the fake claude', () => {
    test.skip(
      () => loadLiveStackConfig().aiMode !== 'stub',
      'this part drives the fake claude: run it with run_ui_flow.py --ai-stub'
    )

    test('the choice names the steps, "Not now" and Escape decide nothing, confirming pins continuous', async ({
      page,
      context
    }, testInfo) => {
      await runLiveFlow({ page, context, testInfo, slug: 'run-all-choice' }, async (f) => {
        const before = await configureProject(f, STEP_BY_STEP)
        try {
          const r = await reachPlanApproval(f, 'weekly limit run all')
          const steps = await phaseCount(f, r)
          expect(
            steps,
            'a change_request Plan has Phases (the choice is offered only for them)'
          ).toBeGreaterThan(0)
          await expectBothApproveButtons(f)
          const versionBefore = (await requestState(f.rpc, r.requestId)).version
          await f.step('"Not now" and Escape close the dialog and decide nothing', async () => {
            await openRunAllDialogAndRead(f, steps)
            await runAllDialog(f.page).getByRole('button', { name: 'Not now' }).click()
            await expect(runAllDialog(f.page)).toBeHidden()
            await expect(
              runAllButton(f.page),
              'focus returns to the button that opened it'
            ).toBeFocused()
            await runAllButton(f.page).click()
            await f.page.keyboard.press('Escape')
            await expect(runAllDialog(f.page)).toBeHidden()
            await expectNothingDecided(f, r, versionBefore)
          })
          await f.step('confirming approves once and pins the continuous mode', async () => {
            await openRunAllDialogAndRead(f, steps)
            await runAllDialog(f.page).getByRole('button', { name: 'Run all steps' }).click()
            await expect(runAllDialog(f.page)).toBeHidden({ timeout: 45_000 })
          })
          await expectRunsOnItsOwn(f, r)
        } finally {
          await configureProject(f, before).catch(() => undefined)
        }
      })
    })

    test('a normal approval is unchanged, and a gateway without approval.runAll hides the choice', async ({
      page,
      context
    }, testInfo) => {
      await runLiveFlow({ page, context, testInfo, slug: 'run-all-normal' }, async (f) => {
        const before = await configureProject(f, STEP_BY_STEP)
        try {
          const r = await reachPlanApproval(f, 'weekly limit step by step')
          await expectBothApproveButtons(f)
          await f.step(
            `a window whose gateway lacks "${RUN_ALL_FEATURE}" shows only Approve plan`,
            async () => {
              const older = await f.page.context().newPage()
              try {
                const stripper = await advertiseNoRunAllFeature(older)
                await older.goto('/')
                await dismissFirstRunTip(older)
                await openRequestsPage(older)
                await selectRequestRow(older, r.title)
                await openDetailTab(older, 'Plan')
                await expect(planBar(older).getByTestId('plan-approve')).toBeVisible({
                  timeout: 30_000
                })
                await expect
                  .poll(() => stripper.strippedAnswers(), {
                    message: 'the SPA was told the feature is missing'
                  })
                  .toBeGreaterThan(0)
                await expect(
                  runAllButton(older),
                  'no run-all choice without the feature'
                ).toHaveCount(0)
                await older.screenshot({ path: f.testInfo.outputPath('run-all-flag-off.png') })
              } finally {
                await older.close()
              }
            }
          )
          await f.step('"Approve plan" approves as before', async () => {
            await planBar(f.page).getByTestId('plan-approve').click()
          })
          await expectAsksBeforeEachStep(f, r)
        } finally {
          await configureProject(f, before).catch(() => undefined)
        }
      })
    })

    test('a project that already runs on its own shows the single card and its button approves', async ({
      page,
      context
    }, testInfo) => {
      await runLiveFlow({ page, context, testInfo, slug: 'run-all-continuous' }, async (f) => {
        const before = await configureProject(f, { ...STEP_BY_STEP, executionMode: 'continuous' })
        try {
          const r = await reachPlanApproval(f, 'weekly limit continuous project')
          await f.step('one approval card and no run-all choice', async () => {
            await openDetailTab(f.page, 'Plan')
            await expect(detailPane(f.page).getByTestId('plan-approval-card')).toBeVisible({
              timeout: 30_000
            })
            await expect(runAllButton(f.page)).toHaveCount(0)
            await f.shots.take('run-all-continuous-card')
            await detailPane(f.page)
              .getByRole('button', { name: 'Approve and start execution' })
              .click()
          })
          await expectRunsOnItsOwn(f, r)
        } finally {
          await configureProject(f, before).catch(() => undefined)
        }
      })
    })
  })

  test('Project Settings: an administrator switches the mode, it survives a reload, other settings stay', async ({
    page,
    context
  }, testInfo) => {
    await runLiveFlow({ page, context, testInfo, slug: 'run-all-settings' }, async (f) => {
      const before = await configureProject(f, STEP_BY_STEP)
      try {
        const stored = await readEngineSettings(f)
        const dialog = await openProjectSettings(f)
        await f.step('the section shows the current mode and who may change it', async () => {
          await expect(dialog.getByRole('radio', { name: 'Ask before each step' })).toHaveAttribute(
            'aria-checked',
            'true'
          )
          await expect(dialog).toContainText('Requests already running keep their mode.')
          await f.shots.take('project-settings-execution-before')
        })
        await f.step('switch to "Run all steps after one approval" and save', async () => {
          await saveModeInProjectSettings(f, dialog, 'Run all steps after one approval')
          await expect(dialog.getByTestId('project-execution-access-note')).toBeVisible()
          await f.shots.take('project-settings-execution-saved')
          const after = await readEngineSettings(f)
          expect(after.executionMode).toBe('continuous')
          expect({
            ...after,
            executionMode: stored.executionMode,
            version: stored.version
          }).toEqual(stored)
          expect(after.version, 'one write').toBe(stored.version + 1)
        })
        await f.step('after a reload the section still shows it', async () => {
          await f.page.reload({ waitUntil: 'domcontentloaded' })
          await dismissFirstRunTip(f.page)
          const reopened = await openProjectSettings(f)
          await expect(
            reopened.getByRole('radio', { name: 'Run all steps after one approval' })
          ).toHaveAttribute('aria-checked', 'true')
        })
        await f.step('put the original mode back through the same screen', async () => {
          const reopened = f.page.getByTestId('project-settings-dialog')
          await saveModeInProjectSettings(f, reopened, 'Ask before each step')
          expect((await readEngineSettings(f)).executionMode).toBe('step_by_step')
        })
      } finally {
        await configureProject(f, before).catch(() => undefined)
      }
    })
  })
})
