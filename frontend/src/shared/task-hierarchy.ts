/**
 * Task Hierarchy Utilities — CR-REQ-021-01
 *
 * Pure functions for working with plan/phase task types:
 * - isPlanningTask / isWorkTask classifiers
 * - computeEffectiveParents: builds a map from task id → {parentId, trail}
 *   that "promotes" tasks whose direct parent is a filtered-out plan/phase node
 *   up to the nearest visible ancestor (or null for root).
 *
 * Performance: O(n) with Set-based cycle detection. Results should be memoised
 * by the caller (useTasks) since the input changes rarely.
 *
 * @module shared/task-hierarchy
 */

import type { OrcaTask } from './task-types'

// ---------------------------------------------------------------------------
// Classifiers
// ---------------------------------------------------------------------------

/** Returns true if this task is a plan or phase structural node */
export function isPlanningTask(t: OrcaTask): boolean {
  return t.type === 'plan' || t.type === 'phase'
}

/** Returns true if this task is a work task (not plan/phase) */
export function isWorkTask(t: OrcaTask): boolean {
  return !isPlanningTask(t)
}

// ---------------------------------------------------------------------------
// Effective parent computation
// ---------------------------------------------------------------------------

export type EffectiveParentEntry = {
  /** The nearest visible ancestor's id, or null if at root */
  parentId: string | null
  /** Names of plan/phase nodes traversed to reach this parent (for chip display) */
  trail: string[]
}

function isInParentCycle(task: OrcaTask, byId: Map<string, OrcaTask>): boolean {
  const seen = new Set<string>([task.id])
  let cur = task.parentId ? byId.get(task.parentId) : undefined
  while (cur) {
    if (cur.id === task.id) {
      return true
    }
    if (seen.has(cur.id)) {
      return false
    }
    seen.add(cur.id)
    cur = cur.parentId ? byId.get(cur.parentId) : undefined
  }
  return false
}

/**
 * For each task in `tasks`, compute the effective parent id after
 * removing all plan/phase intermediate nodes.
 *
 * A task whose direct parent is a plan/phase gets "promoted" up until we reach
 * either a work task or null (root). Cycle detection prevents infinite loops
 * from malformed data.
 *
 * Returns a Map<taskId, EffectiveParentEntry>. Tasks with no plan/phase in their
 * ancestry will have entries identical to their natural parentId.
 */
export function computeEffectiveParents(tasks: OrcaTask[]): Map<string, EffectiveParentEntry> {
  const byId = new Map<string, OrcaTask>()
  for (const t of tasks) {
    byId.set(t.id, t)
  }

  const result = new Map<string, EffectiveParentEntry>()

  for (const task of tasks) {
    if (result.has(task.id)) {
      continue
    } // already resolved

    // A task that is its own ancestor is malformed data: surface it at root
    // rather than hanging it under a node that hangs back under it.
    if (isInParentCycle(task, byId)) {
      result.set(task.id, { parentId: null, trail: [] })
      continue
    }

    // Walk up the ancestor chain to find first visible ancestor
    const visited = new Set<string>()
    const trail: string[] = []
    let currentId: string | null = task.parentId ?? null

    while (currentId !== null) {
      if (visited.has(currentId)) {
        // Cycle detected — break out; task goes to root
        currentId = null
        break
      }
      visited.add(currentId)

      const parent = byId.get(currentId)
      if (!parent) {
        // Parent not found in current task list; treat as root
        break
      }

      if (isPlanningTask(parent)) {
        // Keep climbing; record this plan/phase title for the chip
        trail.push(parent.title)
        currentId = parent.parentId ?? null
      } else {
        // Found a visible ancestor — stop
        break
      }
    }

    result.set(task.id, {
      parentId: currentId,
      trail
    })
  }

  return result
}

// ---------------------------------------------------------------------------
// Hide plan/phase nodes from work views
// ---------------------------------------------------------------------------

/** A work task that remembers which Plan / Phase it was lifted out of. */
export type TaskWithPlanPath = OrcaTask & { planPath?: string[] }

/**
 * Drop plan/phase nodes and re-parent their children onto the nearest visible
 * ancestor. Returns the SAME array when there are no plan/phase nodes so
 * projects that don't use Requests keep identical references and behavior.
 */
export function hidePlanningTasks(
  visible: OrcaTask[],
  allTasks: OrcaTask[],
  hasPlanning: boolean
): TaskWithPlanPath[] {
  if (!hasPlanning) {
    return visible
  }
  const effective = computeEffectiveParents(allTasks)
  const out: TaskWithPlanPath[] = []
  for (const t of visible) {
    if (isPlanningTask(t)) {
      continue
    }
    const entry = effective.get(t.id)
    if (!entry || (entry.parentId === (t.parentId ?? null) && entry.trail.length === 0)) {
      out.push(t)
      continue
    }
    out.push({
      ...t,
      parentId: entry.parentId ?? undefined,
      // trail is nearest-first; the chip reads Plan / Phase
      planPath: entry.trail.length > 0 ? [...entry.trail].toReversed() : undefined
    })
  }
  return out
}

// ---------------------------------------------------------------------------
// Plan subtree builder — CR-REQ-021-02
// ---------------------------------------------------------------------------

export type PlanSubtree = {
  plan: OrcaTask | null
  phases: OrcaTask[]
  /** phase id → tasks in that phase */
  tasksByPhase: Record<string, OrcaTask[]>
  /** Tasks directly under the plan (for task_list plan kind) */
  flatTasks: OrcaTask[]
}

/**
 * Build the plan subtree rooted at planTaskId using BFS.
 * Cycle-safe: uses a Set of visited ids.
 */
export function buildPlanSubtree(tasks: OrcaTask[], planTaskId: string): PlanSubtree {
  const byId = new Map<string, OrcaTask>()
  const childrenOf = new Map<string, OrcaTask[]>()
  for (const t of tasks) {
    byId.set(t.id, t)
    if (t.parentId) {
      const siblings = childrenOf.get(t.parentId) ?? []
      siblings.push(t)
      childrenOf.set(t.parentId, siblings)
    }
  }

  const plan = byId.get(planTaskId) ?? null
  if (!plan) {
    return { plan: null, phases: [], tasksByPhase: {}, flatTasks: [] }
  }

  const phases: OrcaTask[] = []
  const tasksByPhase: Record<string, OrcaTask[]> = {}
  const flatTasks: OrcaTask[] = []
  const visited = new Set<string>([planTaskId])

  const directChildren = childrenOf.get(planTaskId) ?? []

  for (const child of directChildren) {
    if (visited.has(child.id)) {
      continue
    }
    visited.add(child.id)

    if (child.type === 'phase') {
      phases.push(child)
      tasksByPhase[child.id] = []

      // BFS into phase children
      const queue: OrcaTask[] = [child]
      while (queue.length > 0) {
        const curr = queue.shift()!
        const phaseChildren = childrenOf.get(curr.id) ?? []
        for (const pc of phaseChildren) {
          if (visited.has(pc.id)) {
            continue
          }
          visited.add(pc.id)
          if (pc.type === 'phase') {
            phases.push(pc)
            tasksByPhase[pc.id] = []
            queue.push(pc)
          } else {
            tasksByPhase[child.id].push(pc)
          }
        }
      }
    } else {
      // Direct work task under plan (task_list kind)
      flatTasks.push(child)
    }
  }

  return { plan, phases, tasksByPhase, flatTasks }
}
