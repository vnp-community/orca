/**
 * Continuous-run steps of the real-AI spec (CR-REQ-038 / FE-REQ-037): the project runs its plans on their own, so the Plan tab
 * offers ONE approval and everything after it is followed on the Execution tab, with nothing to approve per step.
 */
import { expect } from '@playwright/test'
import type { LiveFlow } from './live-flow-harness'
import type { RequestUnderTest } from './live-flow-steps'
import { requestState } from './live-gateway-crosschecks'
import { claudeRunningUnder } from './live-safety-invariants'
import { detailPane, openDetailTab } from './live-request-ui-steps'
import { expectScreenFollowsService } from './live-status-sync'

type EngineSettings = {
  engine?: string
  openspecMinVersion?: string
  solutionAgent?: boolean
  executionMode?: string
  version: number
}
export type ProjectRunSettings = {
  engine: string
  solutionAgent: boolean
  executionMode: 'continuous' | 'step_by_step'
}

/**
 * Test setup (not a product step): which engine and mode the throwaway project runs with. The settings have no screen in this
 * build. Returns what was stored before, so the spec can put it back.
 */
export async function configureProject(
  f: LiveFlow,
  want: ProjectRunSettings
): Promise<ProjectRunSettings> {
  let before: ProjectRunSettings = {
    engine: 'native',
    solutionAgent: false,
    executionMode: 'step_by_step'
  }
  await f.step(`set the project to engine ${want.engine}, ${want.executionMode}`, async () => {
    const cur = await f.rpc.call<EngineSettings>('request.engineSettings', {
      projectId: f.project.id,
      check: want.engine === 'openspec'
    })
    before = {
      engine: cur.engine ?? 'native',
      solutionAgent: cur.solutionAgent ?? false,
      executionMode: cur.executionMode === 'continuous' ? 'continuous' : 'step_by_step'
    }
    const next = await f.rpc.call<
      EngineSettings & { preflight?: { ok?: boolean; blockers?: unknown[] } }
    >('request.engineSettingsSet', {
      projectId: f.project.id,
      engine: want.engine,
      openspecMinVersion: cur.openspecMinVersion ?? '',
      solutionAgent: want.solutionAgent,
      executionMode: want.executionMode,
      // Why: these flows press Generate plan themselves; automatic plan generation (CR-REQ-044) has its own spec.
      planAutoGenerate: false,
      expectedVersion: cur.version
    })
    expect(next.executionMode, 'the mode was stored').toBe(want.executionMode)
    expect(next.engine, 'the engine was stored').toBe(want.engine)
  })
  return before
}

/** The Plan tab of a continuous request: one approval card, a read-only outline, nothing per step. */
export async function expectSingleApprovalCard(f: LiveFlow, _r: RequestUnderTest): Promise<void> {
  await f.step('the Plan tab offers one approval and no per-step controls', async () => {
    await openDetailTab(f.page, 'Plan')
    const pane = detailPane(f.page)
    await expect(pane.getByTestId('plan-approval-card'), 'the single approval card').toBeVisible({
      timeout: 30_000
    })
    await expect(pane.getByTestId('plan-outline'), 'the read-only outline').toBeVisible()
    await expect(
      pane.getByRole('button', { name: /approve phase|start phase/i }),
      'no per-step buttons'
    ).toHaveCount(0)
    await expect(
      pane.locator('[data-testid^="phase-approval-bar-"]'),
      'no per-step approval bars'
    ).toHaveCount(0)
    await expect(pane.getByTestId('plan-approve')).toHaveText(/approve and start execution/i)
    await f.shots.take('continuous-plan-card')
  })
}

/**
 * Waits for the run to finish while following it on the Execution tab. Nothing is approved on the way: a gate that opens is a
 * failure of "approve once". Returns what the tab showed so the spec can assert it.
 */
export async function followContinuousRun(
  f: LiveFlow,
  r: RequestUnderTest,
  limitMs: number,
  fixtureBase: string | (() => string[])
): Promise<{ sawStrip: boolean; sawRunning: boolean; samples: string[] }> {
  const out = { sawStrip: false, sawRunning: false, samples: [] as string[] }
  await f.step('the agent runs every step on its own; the Execution tab follows it', async () => {
    const t0 = Date.now()
    await openDetailTab(f.page, 'Execution')
    let idleSince = 0
    for (;;) {
      const req = await requestState(f.rpc, r.requestId)
      const approvals = await f.rpc.call<{ approvals?: { subjectType: string; id: string }[] }>(
        'approval.list',
        {
          requestId: r.requestId,
          status: 'pending'
        }
      )
      const unexpected = (approvals.approvals ?? []).filter((a) => a.subjectType === 'phase')
      expect(
        unexpected.map((a) => a.id),
        'no Phase approval opens in a continuous run'
      ).toEqual([])
      const strip = detailPane(f.page).getByTestId('execution-summary-strip')
      if (await strip.isVisible().catch(() => false)) {
        out.sawStrip = true
        const text = (
          await detailPane(f.page)
            .getByTestId('execution-progress-text')
            .innerText()
            .catch(() => '')
        ).replace(/\s+/g, ' ')
        if (text && out.samples.at(-1) !== text) {
          out.samples.push(text)
          console.log(`[exec-ui] ${Math.round((Date.now() - t0) / 1000)}s ${text}`)
        }
        if (
          /running/i.test(
            await detailPane(f.page)
              .getByTestId('execution-run-label')
              .innerText()
              .catch(() => '')
          )
        ) {
          out.sawRunning = true
        }
      }
      if (req.status === 'completed') {
        break
      }
      if (req.status !== 'executing') {
        throw new Error(`the Request left executing for ${req.status} before completing`)
      }
      const base = typeof fixtureBase === 'function' ? fixtureBase() : fixtureBase
      if (out.sawRunning && !claudeRunningUnder(base)) {
        idleSince ||= Date.now()
        if (Date.now() - idleSince > 120_000) {
          throw new Error('the run stalled: tasks in progress with no claude process for 120s')
        }
      } else {
        idleSince = 0
      }
      expect(
        Date.now() - t0,
        `the run finished within ${Math.round(limitMs / 60_000)} min`
      ).toBeLessThan(limitMs)
      await f.page.waitForTimeout(3_000)
    }
    await expectScreenFollowsService(f.page, f.rpc, f.gaps, {
      requestId: r.requestId,
      title: r.title,
      step: 'AFTER-CONTINUOUS-RUN',
      apiStatus: ['completed'],
      chip: /Completed/i,
      apiWaitMs: 30_000
    })
    await f.shots.take('continuous-run-completed')
  })
  return out
}

/** After the run: the Execution tab reads Done with every task counted, and History tells what happened. */
export async function expectFinishedRunScreens(f: LiveFlow, r: RequestUnderTest): Promise<void> {
  await f.step('the Execution tab and History tell the finished run', async () => {
    const pane = detailPane(f.page)
    await openDetailTab(f.page, 'Execution')
    await expect(pane.getByTestId('execution-run-label')).toHaveText(/done/i, { timeout: 30_000 })
    const text = (await pane.getByTestId('execution-progress-text').innerText()).replace(
      /\s+/g,
      ' '
    )
    const m = /(\d+)\/(\d+) tasks/.exec(text)
    expect(m && m[1] === m[2], `every task is counted done (${text})`).toBe(true)
    await expect(
      pane.getByTestId('execution-failure-card'),
      'no failure card on a finished run'
    ).toHaveCount(0)
    await f.shots.take('continuous-execution-done')

    const summary = await f.rpc.call<{
      summary?: { progress?: { tasksDone: number; tasksTotal: number }; failed?: unknown[] }
    }>('request.executionSummary', { id: r.requestId })
    expect(summary.summary?.progress?.tasksDone, 'the service agrees with the screen').toBe(
      summary.summary?.progress?.tasksTotal
    )
    expect(summary.summary?.failed ?? [], 'nothing failed').toEqual([])

    await openDetailTab(f.page, 'History')
    await expect(
      pane.getByText(/Phase started automatically|Phase started/).first(),
      'History lists the phases that started'
    ).toBeVisible({ timeout: 30_000 })
    await expect(pane.getByText(/All tasks finished|finished/i).first()).toBeVisible()
    await f.shots.take('continuous-history')
    await openDetailTab(f.page, 'Plan')
    await expect(
      pane.getByTestId('plan-approved-strip'),
      'the Plan tab says it was approved and where to follow it'
    ).toBeVisible()
  })
}
