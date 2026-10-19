/**
 * execution-step-model.ts — FE-REQ-TASK-037-05
 *
 * Splits the plan tree into the steps a run goes through and says which one it is at, so the list can fold what is done and
 * count what comes later.
 *
 * @module components/request/execution/execution-step-model
 */

import type { PlanSubtree } from '../../../../../shared/task-hierarchy'
import type { OrcaTask } from '../../../../../shared/task-types'

export type ExecutionStepState = 'done' | 'current' | 'later'

export type ExecutionStep = {
  /** Phase id; null for the flat list of a plan without phases. */
  id: string | null
  title: string
  tasks: OrcaTask[]
  state: ExecutionStepState
  done: number
}

const FINISHED = new Set(['done', 'cancelled'])

export function buildExecutionSteps(tree: PlanSubtree, currentPhaseId?: string): ExecutionStep[] {
  const raw: { id: string | null; title: string; tasks: OrcaTask[] }[] = tree.phases.map((p) => ({
    id: p.id,
    title: p.title,
    tasks: tree.tasksByPhase[p.id] ?? []
  }))
  if (tree.flatTasks.length > 0) {
    raw.push({ id: null, title: '', tasks: tree.flatTasks })
  }
  const counted = raw.map((r) => ({
    ...r,
    done: r.tasks.filter((t) => FINISHED.has(t.status)).length
  }))
  const finished = (s: { tasks: OrcaTask[]; done: number }): boolean =>
    s.tasks.length > 0 && s.done === s.tasks.length
  // Why: the step the backend names wins; otherwise it is the first one with work left.
  const named = currentPhaseId ? counted.findIndex((s) => s.id === currentPhaseId) : -1
  const firstOpen = counted.findIndex((s) => !finished(s))
  const current = named >= 0 && !finished(counted[named]) ? named : firstOpen
  return counted.map((s, i) => ({
    ...s,
    state: finished(s) ? 'done' : i === current ? 'current' : 'later'
  }))
}

/** The unfinished task right before `index` in the same step. */
export function previousOpenTask(tasks: OrcaTask[], index: number): OrcaTask | undefined {
  for (let i = index - 1; i >= 0; i--) {
    if (!FINISHED.has(tasks[i].status)) {
      return tasks[i]
    }
  }
  return undefined
}
