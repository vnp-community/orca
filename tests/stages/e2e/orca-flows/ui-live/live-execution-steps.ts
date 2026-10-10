/**
 * Plan approval and execution steps of the real-AI spec (TASK-FE-E2E-010): review the Plan before it is approved, approve it
 * and every gate that follows through the UI, watch the agent's progress on screen, and check what it did in the fixture repo.
 */
import { expect, type Locator } from '@playwright/test'
import type { RpcRow } from './gateway-rpc-client'
import type { LiveFlow } from './live-flow-harness'
import type { RequestUnderTest } from './live-flow-steps'
import { requestState } from './live-gateway-crosschecks'
import { claudeRunningUnder } from './live-safety-invariants'
import { detailPane, openDetailTab, reloadAndReopen } from './live-request-ui-steps'
import { expectScreenFollowsService, UI_REFRESH_GRACE_MS } from './live-status-sync'

const PLAN_TASK_TYPES = ['plan', 'phase', 'task', 'bug', 'feature', 'epic']
const MAX_LEAF_TASKS = 8
const PHASE_GATE_WAIT_MS = 90_000
const STUCK_TASK_MS = 90_000
// Anything in a Plan that points outside the fixture or at the network stops the run before the agent is let loose.
const UNSAFE_PLAN_TEXT =
  /(\/Users\/|~\/|\$HOME|\brm\s+-rf\b|git\s+push|\bsudo\b|\bcurl\b|\bwget\b|\b(npm|pnpm|yarn)\s+(i|install|add)\b|\bdocker\b|\bdeploy\b|production)/i

export type PlanTask = { id: string; title: string; type: string; status: string; parentId: string }

async function listPlanTasks(f: LiveFlow, r: RequestUnderTest): Promise<RpcRow[]> {
  const res = await f.rpc.call<{ tasks?: RpcRow[] }>('task.list', {
    projectId: f.project.id,
    requestId: r.requestId,
    taskTypes: PLAN_TASK_TYPES,
    pageSize: 100
  })
  return res.tasks ?? []
}

const asTask = (t: RpcRow): PlanTask => ({
  id: t.id,
  title: String(t.title ?? ''),
  type: String(t.type ?? ''),
  status: String(t.status ?? ''),
  parentId: String(t.parentId ?? '')
})

/** The Plan the AI wrote, read before approving: small, and nothing that reaches outside the throwaway repo. */
export async function reviewPlanBeforeApproving(
  f: LiveFlow,
  r: RequestUnderTest
): Promise<PlanTask[]> {
  let tasks: PlanTask[] = []
  await f.step('review the Plan before approving it (size and safety)', async () => {
    const rows = await listPlanTasks(f, r)
    tasks = rows.map(asTask)
    const leaves = tasks.filter((t) => !['plan', 'phase'].includes(t.type))
    console.log(
      `[plan] ${tasks.length} rows: ${tasks.map((t) => `${t.type}:${t.title.slice(0, 60)}`).join(' | ')}`
    )
    expect(leaves.length, 'the Plan has working tasks').toBeGreaterThan(0)
    expect(
      leaves.length,
      `the Plan stays small (at most ${MAX_LEAF_TASKS} tasks)`
    ).toBeLessThanOrEqual(MAX_LEAF_TASKS)
    const unsafe = rows.filter((row) => UNSAFE_PLAN_TEXT.test(JSON.stringify(row)))
    expect(
      unsafe.map((u) => `${u.type}:${u.title}`),
      'no Plan task mentions paths outside the fixture, installs, network, deploys or pushes'
    ).toEqual([])
    f.lags.push(`plan ${leaves.length} task(s)`)
  })
  return tasks
}

/** "Approve plan" on the Plan tab; the Request must start executing. */
export async function approvePlanThroughTheUi(f: LiveFlow, r: RequestUnderTest): Promise<void> {
  const { rpc, page, shots, gaps, lags } = f
  await f.step('approve the Plan on the Plan tab', async () => {
    await openDetailTab(page, 'Plan')
    const approve = detailPane(page).getByTestId('plan-approve')
    await expect(approve, 'the Plan tab offers "Approve plan"').toBeVisible({ timeout: 30_000 })
    await shots.take('plan-before-approval')
    await approve.click()
    const lag = await expectScreenFollowsService(page, rpc, gaps, {
      requestId: r.requestId,
      title: r.title,
      step: 'AFTER-PLAN-APPROVAL',
      apiStatus: ['executing'],
      chip: /Executing/i,
      apiWaitMs: 60_000
    })
    lags.push(`plan approved to executing ${lag}ms`)
    await shots.take('plan-approved-executing')
  })
}

type ApprovalRow = { id: string; subjectType: string; subjectId: string }

async function pendingGates(f: LiveFlow, r: RequestUnderTest): Promise<ApprovalRow[]> {
  const res = await f.rpc.call<{ approvals?: ApprovalRow[] }>('approval.list', {
    requestId: r.requestId,
    status: 'pending'
  })
  return res.approvals ?? []
}

/** Approve and start a Phase from its bar on the Plan tab. */
async function approveAndStartPhase(
  f: LiveFlow,
  r: RequestUnderTest,
  phaseId: string
): Promise<void> {
  const { page, shots } = f
  await f.step(`approve and start Phase ${phaseId.slice(0, 8)} on the Plan tab`, async () => {
    await openDetailTab(page, 'Plan')
    const approve = detailPane(page).getByTestId(`phase-${phaseId}-approve`)
    await expect(approve, 'the Phase bar offers "Approve phase"').toBeVisible({
      timeout: UI_REFRESH_GRACE_MS
    })
    await shots.take('phase-gate')
    await approve.click()
    const start = detailPane(page).getByTestId(`phase-${phaseId}-start`)
    await expect(start, '"Start phase" enables once the Phase is approved').toBeEnabled({
      timeout: 30_000
    })
    await start.click()
    await expect
      .poll(async () => (await listPlanTasks(f, r)).find((t) => t.id === phaseId)?.status, {
        timeout: 60_000,
        message: 'the Phase leaves "open" after Start'
      })
      .not.toBe('open')
  })
}

export type ExecutionReport = { seen: string[]; phasesStarted: number; ranInUi: boolean }

/**
 * Drives the Request from `executing` to a terminal status through the UI only: every gate that opens is approved on the
 * screen, task transitions are logged with their time, and the screen must show the running agent and the final counts.
 */
export async function driveExecutionToTheEnd(
  f: LiveFlow,
  r: RequestUnderTest,
  limitMs: number,
  fixtureBase: string
): Promise<ExecutionReport> {
  const report: ExecutionReport = { seen: [], phasesStarted: 0, ranInUi: false }
  const { rpc, page, shots, gaps } = f
  await f.step('the real agent executes the tasks; gates are approved on screen', async () => {
    const t0 = Date.now()
    const last = new Map<string, string>()
    const handledPhases = new Set<string>()
    let gateWaitingSince = 0
    let idleRunningSince = 0
    for (;;) {
      const req = await requestState(rpc, r.requestId)
      const tasks = (await listPlanTasks(f, r)).map(asTask)
      for (const t of tasks) {
        if (last.get(t.id) !== t.status) {
          const line = `${Math.round((Date.now() - t0) / 1000)}s ${t.type} "${t.title.slice(0, 50)}": ${last.get(t.id) ?? '-'} -> ${t.status}`
          console.log(`[exec] ${line}`)
          report.seen.push(line)
          last.set(t.id, t.status)
          if (
            t.status === 'in_progress' &&
            !['plan', 'phase'].includes(t.type) &&
            !report.ranInUi
          ) {
            report.ranInUi = await showsRunningTask(f, r, t.id)
            await shots.take('task-running')
          }
        }
      }
      if (req.status === 'completed') {
        break
      }
      if (req.status !== 'executing') {
        const failed = tasks.filter((t) => t.status === 'blocked' || t.status === 'cancelled')
        throw new Error(
          `the Request left executing for ${req.status} before completing (tasks: ${tasks.map((t) => `${t.type}:${t.status}`).join(', ')}; blocked/cancelled: ${failed.length})`
        )
      }
      // Why: an in_progress task with no claude process behind it is a dispatch that went nowhere; waiting out the limit hides it.
      const working = tasks.filter(
        (t) => t.status === 'in_progress' && !['plan', 'phase'].includes(t.type)
      )
      if (working.length > 0 && !claudeRunningUnder(fixtureBase)) {
        idleRunningSince ||= Date.now()
        if (Date.now() - idleRunningSince > STUCK_TASK_MS) {
          throw new Error(
            `TASK-STUCK-IN-PROGRESS: ${working.map((t) => `"${t.title.slice(0, 40)}"`).join(', ')} in_progress for ${Math.round(STUCK_TASK_MS / 1000)}s with no claude process running (docker logs orca-go-task / orca-go-orchestration: was it routed to the orchestration engine?)`
          )
        }
      } else {
        idleRunningSince = 0
      }
      const gates = await pendingGates(f, r)
      // Why: with Phases a gate must open by itself once the Request is executing; waiting forever hides a stuck backend.
      const hasPhases = tasks.some((t) => t.type === 'phase')
      const phaseRunning = tasks.some((t) => t.type === 'phase' && t.status !== 'open')
      if (hasPhases && !phaseRunning && gates.length === 0) {
        gateWaitingSince ||= Date.now()
        if (Date.now() - gateWaitingSince > PHASE_GATE_WAIT_MS) {
          throw new Error(
            `PHASE-GATE-NEVER-OPENED: the Request has been executing for ${Math.round(PHASE_GATE_WAIT_MS / 1000)}s with an open Phase and no pending Approval (see docker logs orca-go-request for REQUEST_PHASE_NOT_IN_PLAN)`
          )
        }
      } else {
        gateWaitingSince = 0
      }
      for (const gate of gates) {
        if (gate.subjectType !== 'phase') {
          throw new Error(
            `an unexpected gate opened while executing: ${gate.subjectType} (${gate.id})`
          )
        }
        if (!handledPhases.has(gate.subjectId)) {
          handledPhases.add(gate.subjectId)
          await approveAndStartPhase(f, r, gate.subjectId)
          report.phasesStarted++
        }
      }
      expect(
        Date.now() - t0,
        `execution finished within ${Math.round(limitMs / 60_000)} min`
      ).toBeLessThan(limitMs)
      await page.waitForTimeout(3_000)
    }
    await expectScreenFollowsService(page, rpc, gaps, {
      requestId: r.requestId,
      title: r.title,
      step: 'AFTER-EXECUTION',
      apiStatus: ['completed'],
      chip: /Completed/i,
      apiWaitMs: 30_000
    })
    await shots.take('request-completed')
  })
  return report
}

/** The Plan tab shows the agent running (a "running" chip on the Phase or a running task badge) within the UI's refresh grace. */
async function showsRunningTask(
  f: LiveFlow,
  r: RequestUnderTest,
  taskId: string
): Promise<boolean> {
  const { page, gaps } = f
  await openDetailTab(page, 'Plan')
  const running = detailPane(page)
    .getByText(/\b\d+ running\b/i)
    .or(
      detailPane(page)
        .getByTestId(`plan-task-row-${taskId}`)
        .getByText(/in progress|running/i)
    )
  const seen = await expect(running.first())
    .toBeVisible({ timeout: UI_REFRESH_GRACE_MS })
    .then(
      () => true,
      () => false
    )
  if (!seen) {
    gaps.record(
      'UI-STALE-TASK-RUNNING',
      `task ${taskId} is in_progress in the service but the Plan tab shows no running task after ${UI_REFRESH_GRACE_MS / 1000}s (request ${r.requestId})`
    )
  }
  return seen
}

/**
 * After completion the Plan tab must read N/N tasks done and 100%. A tab that stays on the old counts although the Request is
 * completed is a gap (UI-STALE-PLAN-TAB); the page is then reloaded so the later steps still run.
 */
export async function expectPlanTabFinished(f: LiveFlow, r: RequestUnderTest): Promise<void> {
  await f.step('the Plan tab shows every task done', async () => {
    await openDetailTab(f.page, 'Plan')
    const progress = (): Locator => detailPane(f.page).getByTestId('plan-progress-value')
    const fresh = await expect(progress())
      .toContainText('100%', { timeout: UI_REFRESH_GRACE_MS })
      .then(
        () => true,
        () => false
      )
    if (!fresh) {
      const shown = (
        await progress()
          .innerText()
          .catch(() => '')
      ).replace(/\s+/g, ' ')
      f.gaps.record(
        'UI-STALE-PLAN-TAB',
        `[frontend] the Request is completed in the service but the Plan tab still reads "${shown}" after ${UI_REFRESH_GRACE_MS / 1000}s (request ${r.requestId}); needed a reload`
      )
      await reloadAndReopen(f.page, r.title)
      await openDetailTab(f.page, 'Plan')
      await expect(progress()).toContainText('100%', { timeout: 20_000 })
    }
    const pane = detailPane(f.page)
    const summaries = pane.locator('[data-testid^="phase-summary-"]')
    for (let i = 0; i < (await summaries.count()); i++) {
      const m = /(\d+)\/(\d+) tasks done/.exec(await summaries.nth(i).innerText())
      expect(m && m[1] === m[2], `phase summary reads all done (${m?.[0] ?? 'no match'})`).toBe(
        true
      )
    }
    await f.shots.take('plan-tab-finished')
  })
}
