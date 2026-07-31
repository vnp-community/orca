# TASK-035: TaskService CRUD + Tree Ops

**Phase:** 6 — Task Graph  
**Solution ref:** [SOL-V5-005](../solutions/SOL-V5-005-task-graph.md) §4  
**Prerequisite:** TASK-001 (migration 0010), TASK-004 (task-types.ts), TASK-036 (TaskDAGValidator)  
**Status:** ✅ DONE — 2026-07-29

---

## File cần tạo: `src/main/task/TaskService.ts`

Constructor:
```typescript
constructor(
  private readonly pool: IConnectionPool,
  private readonly dagValidator: TaskDAGValidator
) {}
```

**Public API (full list từ SOL-V5-005 §4):**
- `create(params)` → `OrcaTask`
- `get(taskId)` → `OrcaTask | null`
- `update(taskId, patch)` → `void`
- `delete(taskId)` → `void`
- `getChildren(taskId)` → `OrcaTask[]`
- `getAncestors(taskId)` → `OrcaTask[]` (walk parent chain up)
- `getSubtree(taskId)` → `OrcaTask[]` (BFS all descendants)
- `addEdge(fromTaskId, toTaskId, edgeType)` → `void` — validate no cycle first
- `removeEdge(fromTaskId, toTaskId, edgeType)` → `void`
- `getDependencies(taskId)` → `{ task, edgeType }[]`
- `getDependents(taskId)` → `{ task, edgeType }[]`
- `recalculateProgress(taskId)` → `number` (recursive avg of children)
- `list(filters)` → `OrcaTask[]`
- `findByRef(ref)` → `OrcaTask | null` (search by id prefix or label)
- `addComment(taskId, userId, content, type)` → `void`

**Column mapping:**
- `project_id` → `projectId`, `parent_id` → `parentId`, `reporter_id` → `reporterId`
- `assignee_id` → `assigneeId`, `estimated_hours` → `estimatedHours`
- `progress_percent` → `progressPercent`, `ai_context` → `aiContext`
- `labels`: JSON parse/stringify
- `created_at`, `updated_at`: `new Date(timestamp)`

**`recalculateProgress` logic:**
- Leaf node: status='done'→100, 'review'→80, 'in_progress'→40, else→0
- Parent: avg of children's progress (recursive)

## Acceptance Criteria

- [x] `TaskService` class export
- [x] 15 methods implemented (+ `TaskDAGValidator` companion created)
- [x] `addEdge` validates cycle before insert
- [x] `recalculateProgress` recursive
- [x] `labels` JSON parse/stringify
- [x] Không TypeScript errors
