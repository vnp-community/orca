/**
 * DAGBuilder — Kahn's algorithm topological sort for workflow steps (TDD-17)
 *
 * Converts a flat list of WorkflowSteps with dependsOn edges into an
 * ordered array of "waves". Steps in the same wave can execute in parallel;
 * each wave must complete before the next begins.
 *
 * Algorithm (Kahn's BFS topological sort):
 * 1. Build in-degree map: number of unresolved deps per step
 * 2. Seed queue with all steps that have in-degree 0 (no deps)
 * 3. While queue not empty:
 *    a. Current queue = one wave
 *    b. For each step in the wave, decrement in-degree of dependents
 *    c. Enqueue dependents whose in-degree reaches 0
 * 4. If processed steps < total steps → cycle detected → throw WorkflowCycleError
 *
 * @module main/workflow/DAGBuilder
 */

import type { WorkflowStep } from './WorkflowTypes'
import { WorkflowCycleError } from './WorkflowTypes'

export class DAGBuilder {
  /**
   * Compute execution waves from a flat step list.
   *
   * @param steps - All steps in the workflow definition
   * @returns Array of waves, each wave is an array of steps safe to run in parallel
   * @throws Error if a step references a non-existent dependency
   * @throws WorkflowCycleError if a dependency cycle is detected
   */
  buildWaves(steps: WorkflowStep[]): WorkflowStep[][] {
    if (steps.length === 0) {return []}

    // 1. Build lookup map and validate all dependsOn references
    const stepMap = new Map<string, WorkflowStep>()
    for (const step of steps) {
      stepMap.set(step.id, step)
    }

    // Validate that all dependsOn references exist
    for (const step of steps) {
      for (const depId of step.dependsOn ?? []) {
        if (!stepMap.has(depId)) {
          throw new Error(`STEP_NOT_FOUND: step "${step.id}" depends on unknown step "${depId}"`)
        }
      }
    }

    // 2. Compute in-degree for each step
    const inDegree = new Map<string, number>()
    // Build reverse adjacency: dependentId → set of step ids that depend on it
    const dependents = new Map<string, Set<string>>()

    for (const step of steps) {
      if (!inDegree.has(step.id)) {inDegree.set(step.id, 0)}
      if (!dependents.has(step.id)) {dependents.set(step.id, new Set())}
    }

    for (const step of steps) {
      for (const depId of step.dependsOn ?? []) {
        inDegree.set(step.id, (inDegree.get(step.id) ?? 0) + 1)
        dependents.get(depId)!.add(step.id)
      }
    }

    // 3. Kahn's BFS
    const waves: WorkflowStep[][] = []
    let queue: string[] = []

    // Seed with zero in-degree
    for (const [id, deg] of inDegree) {
      if (deg === 0) {queue.push(id)}
    }

    let processed = 0

    while (queue.length > 0) {
      const wave: WorkflowStep[] = queue.map(id => stepMap.get(id)!)
      waves.push(wave)
      processed += queue.length

      const nextQueue: string[] = []
      for (const id of queue) {
        for (const depId of dependents.get(id)!) {
          const newDeg = (inDegree.get(depId) ?? 0) - 1
          inDegree.set(depId, newDeg)
          if (newDeg === 0) {
            nextQueue.push(depId)
          }
        }
      }
      queue = nextQueue
    }

    // 4. Cycle detection
    if (processed < steps.length) {
      // Find nodes still in a cycle (in-degree > 0)
      const cycleNodes = [...inDegree.entries()]
        .filter(([, deg]) => deg > 0)
        .map(([id]) => id)
      throw new WorkflowCycleError(cycleNodes)
    }

    return waves
  }
}
