# TASK-020: Wire ProjectService to Bootstrap (Step 8)

**Phase:** 3 — Project Binding  
**Solution ref:** [SOL-V5-000](../solutions/SOL-V5-000-server-bootstrap-changes.md) §2  
**Prerequisite:** TASK-012, TASK-017, TASK-018 (all tests pass)  
**Status:** ✅ DONE — 2026-07-29

---

## Thay đổi trong `src/main/server-bootstrap.ts`

Thêm Step 8 sau step 7 (ProfileService):

```typescript
// 8. ProjectService + ProjectServerRouter [v5.0 TDD-15]
const { ProjectService } = await import('./project/ProjectService')
const { ProjectServerRouter } = await import('./project/ProjectServerRouter')
const projectService = new ProjectService(pool, devServerManager)
const projectRouter = new ProjectServerRouter(projectService, devServerManager, relayConnectionPool)
console.log('[ServerBootstrap] ✅ ProjectService + ProjectServerRouter initialized (v5.0)')
```

Update `return` block: replace `projectService` placeholder với thực.

## Acceptance Criteria

- [x] Step 8 thêm sau step 7
- [x] `projectService` trong return block là thực instance
- [x] Không TypeScript errors
- [x] Existing tests still pass
