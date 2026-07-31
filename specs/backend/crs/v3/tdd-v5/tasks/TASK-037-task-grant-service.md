# TASK-037: TaskGrantService BFS Ancestor Resolution

**Phase:** 6 — Task Graph  
**Solution ref:** [SOL-V5-005](../solutions/SOL-V5-005-task-graph.md) §5  
**Prerequisite:** TASK-035, TASK-036  
**Status:** ✅ DONE — 2026-07-29

---

## File cần tạo: `src/main/task/TaskGrantService.ts`

BFS ancestor grant resolution:

```typescript
export class TaskGrantService {
  constructor(
    private readonly pool: IConnectionPool,
    private readonly taskService: TaskService
  ) {}

  /**
   * Grant permission on a task.
   * If applyTree=true, grant applies to all descendants too (resolved via BFS).
   */
  async grantPermission(params: {
    taskId: string
    scope: TaskGrant['scope']
    scopeId?: string
    permission: TaskPermission
    applyTree?: boolean
    grantedBy: string
    expiresAt?: Date
  }): Promise<string>

  /**
   * Resolve effective permission for a user on a task.
   * BFS up ancestor chain to find inherited grants.
   * Returns highest permission found (manage > execute > edit > comment > view).
   */
  async resolvePermission(userId: string, taskId: string): Promise<TaskPermission | null>

  async revokeGrant(grantId: string): Promise<void>
  async listGrants(taskId: string): Promise<TaskGrant[]>
}
```

**Permission hierarchy:** `manage > execute > edit > comment > view`

**BFS resolution:**
1. Check grants on taskId directly
2. Walk ancestors (via parent_id chain)
3. For each ancestor: check grants with `apply_tree=1`
4. Return highest permission found across all grants that match userId (by user/team/role/everyone scope)

## Acceptance Criteria

- [x] `TaskGrantService` class export
- [x] `resolvePermission` BFS up parent chain
- [x] `applyTree=true` propagates via apply_tree=1 filter
- [x] Permission hierarchy correct (manage > execute > edit > comment > view)
- [x] Scope matching: user/team/role/everyone
- [x] Không TypeScript errors
