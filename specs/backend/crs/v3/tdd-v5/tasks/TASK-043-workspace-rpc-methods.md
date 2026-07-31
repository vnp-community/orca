# TASK-043: Workspace RPC Methods + Bootstrap Step 12

**Phase:** 7 — Workspace + Remote Git  
**Prerequisite:** TASK-042  
**Status:** ✅ DONE

---

## File cần tạo: `src/main/workspace/workspace-rpc-handler.ts`

**Methods:**
- `workspace.init` → `workspaceService.initWorkspace(params.projectId, session.userId)`
- `workspace.teardown` → `workspaceService.teardownWorkspace(params.projectId)`
- `workspace.refreshFileTree` → `workspaceService.refreshFileTree(params.projectId, session.userId, params.path?)`
- `workspace.refreshGitStatus` → `workspaceService.refreshGitStatus(params.projectId, session.userId, params.worktreePath)`

---

## Bootstrap Step 12 (trong `src/main/server-bootstrap.ts`)

Thêm sau step 11:

```typescript
// 12. WorkspaceService [v5.0 TDD-19]
const { WorkspaceService } = await import('./workspace/WorkspaceService')
const workspaceService = new WorkspaceService(
  projectRouter, profileResolver, taskService, workflowOrchestrator, relayConnectionPool
)
console.log('[ServerBootstrap] ✅ WorkspaceService initialized (v5.0)')
```

## Acceptance Criteria

- [x] 4 RPC methods registered
- [x] Step 12 thêm vào bootstrap
- [x] Không TypeScript errors
