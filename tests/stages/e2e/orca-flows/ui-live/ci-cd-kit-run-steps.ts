/**
 * The run itself: "Approve and run all steps" on the Plan tab, then the real agent works task after task while the spec follows it
 * on the Execution tab, timing every step. The stop-loss lives here: a gate that opens, a task that fails twice, a Request that
 * leaves `executing`, or a run that makes no progress ends the spec (which then cancels the Request through the UI).
 */
import { expect } from '@playwright/test'
import type { Timeline } from './ci-cd-kit-report'
import type { RpcRow } from './gateway-rpc-client'
import type { LiveFlow } from './live-flow-harness'
import type { RequestUnderTest } from './live-flow-steps'
import { pendingApprovals, requestState } from './live-gateway-crosschecks'
import { approveAndRunAllThroughTheUi } from './live-run-all-steps'
import { claudeRunningUnder } from './live-safety-invariants'
import { detailPane, openDetailTab } from './live-request-ui-steps'
import { expectScreenFollowsService } from './live-status-sync'

const POLL_MS = 5_000
const NO_PROGRESS_MS = 20 * 60_000
const LEAF_TYPES = ['task', 'bug', 'feature', 'epic']

export type TaskTiming = {
  type: string
  title: string
  startedS: number | null
  doneS: number | null
  status: string
}
export type RunRecord = {
  transitions: string[]
  timings: TaskTiming[]
  failures: number
  sawRunningOnScreen: boolean
  /** Things the run showed that are not failures but that the report names (an approval of another kind). */
  notes: string[]
  samples: string[]
}

export async function approveRunAll(
  f: LiveFlow,
  r: RequestUnderTest,
  timeline: Timeline
): Promise<void> {
  await approveAndRunAllThroughTheUi(f, r)
  timeline.mark('plan approved with run-all')
}

type Row = { id: string; type: string; title: string; status: string }

async function listRows(f: LiveFlow, r: RequestUnderTest): Promise<Row[]> {
  const res = await f.rpc.call<{ tasks?: RpcRow[] }>('task.list', {
    projectId: f.project.id,
    requestId: r.requestId,
    taskTypes: ['plan', 'phase', ...LEAF_TYPES],
    pageSize: 100
  })
  // Why: tasks of an older plan version are cancelled rows and are not part of the run.
  return (res.tasks ?? [])
    .filter((t) => String(t.status ?? '') !== 'cancelled')
    .map((t) => ({
      id: String(t.id),
      type: String(t.type ?? ''),
      title: String(t.title ?? ''),
      status: String(t.status ?? '')
    }))
}

/** Follows the run to `completed`; throws the stop-loss reasons. `workRoots` are where claude may be working (repo + new worktrees). */
export async function followRunAll(
  f: LiveFlow,
  r: RequestUnderTest,
  workRoots: () => string[],
  timeline: Timeline,
  limitMs: number
): Promise<RunRecord> {
  const rec: RunRecord = {
    transitions: [],
    timings: [],
    failures: 0,
    sawRunningOnScreen: false,
    notes: [],
    samples: []
  }
  await f.step('the agent runs every task on its own; the Execution tab follows it', async () => {
    const t0 = Date.now()
    const last = new Map<string, string>()
    const timing = new Map<string, TaskTiming>()
    const bounced = new Map<string, number>()
    let lastChangeAt = Date.now()
    await openDetailTab(f.page, 'Execution')
    for (;;) {
      const req = await requestState(f.rpc, r.requestId)
      const rows = await listRows(f, r)
      const at = Math.round((Date.now() - t0) / 1000)
      for (const t of rows) {
        if (last.get(t.id) === t.status) {
          continue
        }
        const previous = last.get(t.id)
        const line = `${at}s ${t.type} "${t.title.slice(0, 60)}": ${previous ?? '-'} -> ${t.status}`
        console.log(`[ci-cd-kit] ${line}`)
        rec.transitions.push(line)
        last.set(t.id, t.status)
        lastChangeAt = Date.now()
        const tm = timing.get(t.id) ?? {
          type: t.type,
          title: t.title,
          startedS: null,
          doneS: null,
          status: t.status
        }
        tm.status = t.status
        if (t.status === 'in_progress' && tm.startedS === null) {
          tm.startedS = at
          if (!LEAF_TYPES.includes(t.type)) {
            timeline.mark(`${t.type} started: ${t.title.slice(0, 50)}`)
          }
        }
        if (t.status === 'done') {
          tm.doneS = at
          if (!LEAF_TYPES.includes(t.type)) {
            timeline.mark(`${t.type} done: ${t.title.slice(0, 50)}`)
          }
        }
        // Why: `blocked` is the auto-block of an unmet dependency, not a failure; a leaf that goes back to `open` after `in_progress` is one.
        if (LEAF_TYPES.includes(t.type) && t.status === 'open' && previous === 'in_progress') {
          rec.failures++
          const times = (bounced.get(t.id) ?? 0) + 1
          bounced.set(t.id, times)
          if (times >= 2 || rec.failures >= 2) {
            await f.shots.take('stop-loss-task-failed-twice')
            throw new Error(
              `STOP-LOSS: ${rec.failures} task failure(s), "${t.title.slice(0, 60)}" failed ${times} time(s)`
            )
          }
        }
        timing.set(t.id, tm)
      }
      rec.timings = [...timing.values()]
      if (req.status === 'completed') {
        break
      }
      if (req.status !== 'executing') {
        await f.shots.take('stop-loss-left-executing')
        throw new Error(
          `STOP-LOSS: the Request left executing for ${String(req.status)} (tasks: ${rows.map((t) => `${t.type}:${t.status}`).join(', ')})`
        )
      }
      const gates = await pendingApprovals(f.rpc, r.requestId)
      const phaseGates = gates.filter((g) => g.subjectType === 'phase')
      if (phaseGates.length > 0) {
        await f.shots.take('stop-loss-gate-opened')
        throw new Error(
          'STOP-LOSS: a run-all run opened a Phase approval; run-all means no further approval'
        )
      }
      for (const g of gates) {
        const note = `an approval of kind ${String(g.subjectType)} opened during the run (${String(g.id)})`
        if (!rec.notes.includes(note)) {
          rec.notes.push(note)
          await f.shots.take('approval-during-run')
        }
      }
      const pane = detailPane(f.page)
      if (!rec.sawRunningOnScreen) {
        const label = await pane
          .getByTestId('execution-run-label')
          .innerText({ timeout: 1_000 })
          .catch(() => '')
        if (/running/i.test(label)) {
          rec.sawRunningOnScreen = true
          await f.shots.take('execution-running')
        }
      }
      const text = (
        await pane
          .getByTestId('execution-progress-text')
          .innerText({ timeout: 1_000 })
          .catch(() => '')
      )
        .replace(/\s+/g, ' ')
        .trim()
      if (text && rec.samples.at(-1) !== text) {
        rec.samples.push(text)
        console.log(`[ci-cd-kit] execution tab ${at}s: ${text}`)
        lastChangeAt = Date.now()
      }
      if (Date.now() - lastChangeAt > NO_PROGRESS_MS) {
        await f.shots.take('stop-loss-no-progress')
        // Why: a run that waits for a person (an approval that is not a Phase gate) is not failing; it is left open for the owner.
        if (gates.length > 0) {
          throw new Error(
            `NEEDS-YOU: no progress for ${Math.round(NO_PROGRESS_MS / 60_000)} min while an approval waits (${gates.map((g) => String(g.subjectType)).join(', ')})`
          )
        }
        throw new Error(
          `STOP-LOSS: no task or progress change for ${Math.round(NO_PROGRESS_MS / 60_000)} min (claude running under the repo: ${claudeRunningUnder(workRoots())})`
        )
      }
      expect(
        Date.now() - t0,
        `the run finished within ${Math.round(limitMs / 60_000)} min`
      ).toBeLessThan(limitMs)
      await f.page.waitForTimeout(POLL_MS)
    }
    await expectScreenFollowsService(f.page, f.rpc, f.gaps, {
      requestId: r.requestId,
      title: r.title,
      step: 'AFTER-EXECUTION',
      apiStatus: ['completed'],
      chip: /Completed/i,
      apiWaitMs: 30_000
    })
    timeline.mark('request completed')
    await f.shots.take('request-completed')
  })
  return rec
}

/** Execution tab, Plan tab and History after the run; a screen that disagrees with the service is a finding, not a stop. */
export async function recordFinishedScreens(
  f: LiveFlow,
  r: RequestUnderTest,
  findings: string[]
): Promise<void> {
  await f.step('the finished run on the Execution, Plan and History tabs', async () => {
    const pane = detailPane(f.page)
    try {
      await openDetailTab(f.page, 'Execution')
      await expect(pane.getByTestId('execution-run-label')).toHaveText(/done/i, { timeout: 30_000 })
      await expect(pane.getByTestId('execution-failure-card')).toHaveCount(0)
      await f.shots.take('execution-done')
      await openDetailTab(f.page, 'Plan')
      await expect(pane.getByTestId('plan-progress-value')).toContainText('100%', {
        timeout: 45_000
      })
      await f.shots.take('plan-tab-finished')
      await openDetailTab(f.page, 'History')
      await f.shots.take('history')
    } catch (e) {
      findings.push(`finished-run screens: ${(e as Error).message.split('\n')[0]}`)
      await f.shots.take('finished-screens-problem')
    }
  })
}

/** The usage figures the product itself keeps for the Request: analysis runs, the plan run and the execution summary. */
export async function collectCost(f: LiveFlow, r: RequestUnderTest): Promise<string[]> {
  const lines: string[] = []
  const listed = await f.rpc.call<{
    runs?: { kind: string; status: string; stats?: Record<string, unknown> }[]
  }>('solution.list', { requestId: r.requestId })
  let total = 0
  for (const run of listed.runs ?? []) {
    const cost = Number(run.stats?.costUsd ?? 0)
    total += cost
    lines.push(`run ${run.kind} ${run.status}: ${JSON.stringify(run.stats ?? {}).slice(0, 260)}`)
  }
  try {
    const plan = await f.rpc.call<{ stats?: Record<string, unknown>; elapsedMs?: number }>(
      'request.planProposal',
      { id: r.requestId }
    )
    total += Number(plan.stats?.costUsd ?? 0)
    lines.push(
      `plan run ${plan.elapsedMs ?? '?'} ms: ${JSON.stringify(plan.stats ?? {}).slice(0, 260)}`
    )
  } catch (e) {
    lines.push(`plan run stats unavailable: ${(e as Error).message.slice(0, 80)}`)
  }
  try {
    const sum = await f.rpc.call<unknown>('request.executionSummary', { id: r.requestId })
    lines.push(`execution summary: ${JSON.stringify(sum).slice(0, 700)}`)
  } catch (e) {
    lines.push(`execution summary unavailable: ${(e as Error).message.slice(0, 80)}`)
  }
  lines.unshift(
    `tổng chi phí Analysis + Plan theo costUsd của run: ${total.toFixed(2)} USD (task chạy thật: xem log agent)`
  )
  return lines
}
