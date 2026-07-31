# TASK-036: TaskDAGValidator

**Phase:** 6 — Task Graph  
**Solution ref:** [SOL-V5-005](../solutions/SOL-V5-005-task-graph.md) §5  
**Prerequisite:** TASK-001 (migration 0010)  
**Status:** ✅ DONE — 2026-07-29

---

## File cần tạo: `src/main/task/TaskDAGValidator.ts`

BFS cycle detection cho task edges:

```typescript
export class TaskDAGValidator {
  constructor(private readonly pool: IConnectionPool) {}

  /**
   * Returns true if adding edge fromTaskId→toTaskId would create a cycle.
   * BFS from toTaskId: if we can reach fromTaskId, it's a cycle.
   */
  async detectCycle(fromTaskId: string, toTaskId: string): Promise<boolean> {
    const visited = new Set<string>()
    const queue = [toTaskId]
    while (queue.length > 0) {
      const current = queue.shift()!
      if (current === fromTaskId) return true
      if (visited.has(current)) continue
      visited.add(current)
      const rows = await this.pool.query<{ toTaskId: string }>(
        'SELECT to_task_id as toTaskId FROM orca_task_edges WHERE from_task_id = ?',
        [current]
      )
      queue.push(...rows.map(r => r.toTaskId))
    }
    return false
  }

  async getReachable(fromTaskId: string): Promise<string[]> {
    // BFS from fromTaskId, return all reachable task IDs
  }
}
```

## Acceptance Criteria

- [x] `detectCycle` returns true for cycle
- [x] `detectCycle` returns false for valid edge
- [x] BFS handles disconnected graphs
- [x] `getReachable` returns transitive closure
- [x] 16 tests pass
- [x] Không TypeScript errors
