/**
 * TaskDAGValidator — Cycle detection for task dependency edges (TDD-18)
 *
 * Validates that adding a new edge (fromTaskId → toTaskId) would not
 * introduce a cycle in the task dependency graph.
 *
 * Two public entry-points:
 * - `wouldCreateCycle(from, to, edgeType)` — used by TaskService.addEdge (DFS, edge-type aware)
 * - `detectCycle(from, to)` — BFS across ALL edge types (spec-required alias)
 * - `getReachable(fromTaskId)` — BFS all reachable task IDs across all edge types
 *
 * @module main/task/TaskDAGValidator
 */

import type { IConnectionPool } from '../db/pool'
import type { TaskEdgeType } from '../../shared/task-types'

export class TaskDAGValidator {
  constructor(private readonly pool: IConnectionPool) {}

  /**
   * Check if adding edge from → to would create a cycle.
   * Returns true if the edge would create a cycle (unsafe), false if safe.
   *
   * DFS from `to` through existing edges of the same edgeType.
   * If we can reach `from`, adding edge would create a cycle.
   */
  async wouldCreateCycle(
    fromTaskId: string,
    toTaskId: string,
    edgeType: TaskEdgeType
  ): Promise<boolean> {
    // A self-loop always creates a cycle
    if (fromTaskId === toTaskId) {return true}

    const visited = new Set<string>()
    const stack = [toTaskId]

    while (stack.length > 0) {
      const current = stack.pop()!
      if (current === fromTaskId) {return true}
      if (visited.has(current)) {continue}
      visited.add(current)

      const rows = await this.pool.withConnection((db) =>
        db.query<{ toTaskId: string }>(
          `SELECT to_task_id as "toTaskId" FROM orca_task_edges
           WHERE from_task_id = ? AND edge_type = ?`,
          [current, edgeType]
        )
      )

      for (const row of rows) {
        if (!visited.has(row.toTaskId)) {
          stack.push(row.toTaskId)
        }
      }
    }

    return false
  }

  /**
   * BFS cycle check across ALL edge types.
   * Returns true if adding fromTaskId → toTaskId would create a cycle.
   *
   * Spec-required entry-point that ignores edge type filtering.
   */
  async detectCycle(fromTaskId: string, toTaskId: string): Promise<boolean> {
    if (fromTaskId === toTaskId) {return true}

    const visited = new Set<string>()
    const queue = [toTaskId]

    while (queue.length > 0) {
      const current = queue.shift()!
      if (current === fromTaskId) {return true}
      if (visited.has(current)) {continue}
      visited.add(current)

      const rows = await this.pool.withConnection((db) =>
        db.query<{ toTaskId: string }>(
          `SELECT to_task_id as "toTaskId" FROM orca_task_edges WHERE from_task_id = ?`,
          [current]
        )
      )

      for (const row of rows) {
        if (!visited.has(row.toTaskId)) {
          queue.push(row.toTaskId)
        }
      }
    }

    return false
  }

  /**
   * BFS from fromTaskId — return all reachable task IDs (all edge types).
   * Used for impact analysis (e.g. recalculating downstream progress).
   * The origin task itself is NOT included in the result.
   */
  async getReachable(fromTaskId: string): Promise<string[]> {
    const visited = new Set<string>()
    const queue = [fromTaskId]
    const result: string[] = []

    while (queue.length > 0) {
      const current = queue.shift()!
      if (visited.has(current)) {continue}
      visited.add(current)
      if (current !== fromTaskId) {result.push(current)}

      const rows = await this.pool.withConnection((db) =>
        db.query<{ toTaskId: string }>(
          `SELECT to_task_id as "toTaskId" FROM orca_task_edges WHERE from_task_id = ?`,
          [current]
        )
      )

      for (const row of rows) {
        if (!visited.has(row.toTaskId)) {
          queue.push(row.toTaskId)
        }
      }
    }

    return result
  }
}
