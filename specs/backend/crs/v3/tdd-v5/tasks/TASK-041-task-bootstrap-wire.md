# TASK-041: Wire TaskService to Bootstrap (Step 11)

**Phase:** 6 — Task Graph  
**Solution ref:** [SOL-V5-000](../solutions/SOL-V5-000-server-bootstrap-changes.md) §2  
**Prerequisite:** TASK-034, TASK-039 all tests pass  
**Status:** ✅ DONE

---

## Thay đổi trong `src/main/server-bootstrap.ts`

Thêm Step 11 sau step 10 (WorkflowOrchestrator):

```typescript
// 11. TaskService + TaskAgentExecutor [v5.0 TDD-18]
const { TaskDAGValidator } = await import('./task/TaskDAGValidator')
const { TaskService } = await import('./task/TaskService')
const { TaskGrantService } = await import('./task/TaskGrantService')
const { ProfileAwareAgentSpawner } = await import('./project/ProfileAwareAgentSpawner')
const { TaskAgentExecutor } = await import('./task/TaskAgentExecutor')
const taskDagValidator = new TaskDAGValidator(pool)
const taskService = new TaskService(pool, taskDagValidator)
const taskGrantService = new TaskGrantService(pool, taskService)
const agentSpawner = new ProfileAwareAgentSpawner(projectRouter, profileResolver, aiProviderService)
const taskAgentExecutor = new TaskAgentExecutor(taskService, agentSpawner, taskGrantService)
console.log('[ServerBootstrap] ✅ TaskService + TaskAgentExecutor initialized (v5.0)')
```

Update `return` block: add `taskService`.

## Acceptance Criteria

- [x] Step 11 thêm sau step 10
- [x] `taskService` trong return block
- [x] Existing tests still pass
