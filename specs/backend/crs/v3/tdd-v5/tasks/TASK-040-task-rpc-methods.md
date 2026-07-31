# TASK-040: Task RPC Methods

**Phase:** 6 — Task Graph  
**Prerequisite:** TASK-035, TASK-037, TASK-038, TASK-039  
**Status:** ✅ DONE — 2026-07-29

---

## File cần tạo: `src/main/task/task-rpc-handler.ts`

**Methods:**
- `task.create` → `taskService.create`
- `task.get` → `taskService.get` (check view permission)
- `task.update` → `taskService.update` (check edit permission)
- `task.delete` → `taskService.delete` (check manage permission)
- `task.list` → `taskService.list(params)`
- `task.getChildren` → `taskService.getChildren(params.taskId)`
- `task.getAncestors` → `taskService.getAncestors(params.taskId)`
- `task.getSubtree` → `taskService.getSubtree(params.taskId)`
- `task.addEdge` → `taskService.addEdge` (check edit)
- `task.removeEdge` → `taskService.removeEdge` (check edit)
- `task.getDependencies` → `taskService.getDependencies`
- `task.recalculateProgress` → `taskService.recalculateProgress`
- `task.addComment` → check comment permission
- `task.grant` → `grantService.grantPermission` (check manage)
- `task.resolvePermission` → `grantService.resolvePermission`
- `task.aiDecompose` → `aiPlanner.decompose`
- `task.aiApply` → `aiPlanner.applyDecomposition`
- `task.execute` → `executor.executeTask` (check execute permission)

## Acceptance Criteria

- [x] 18 RPC methods registered
- [x] Permission checks on all write operations
- [x] Zod validation on all params
- [x] Không TypeScript errors
