# TASK-034: Wire WorkflowOrchestrator to Bootstrap (Step 10)

**Phase:** 5 — Workflow Orchestration  
**Solution ref:** [SOL-V5-000](../solutions/SOL-V5-000-server-bootstrap-changes.md) §2  
**Prerequisite:** TASK-027, TASK-033 (all tests pass)  
**Status:** ✅ DONE — 2026-07-29

---

## Thay đổi trong `src/main/server-bootstrap.ts`

Thêm Step 10 sau step 9 (AIProviderService):

```typescript
// 10. WorkflowOrchestrator [v5.0 TDD-17]
const { DAGBuilder } = await import('./workflow/DAGBuilder')
const { WorkflowOrchestrator } = await import('./workflow/WorkflowOrchestrator')
const { StepExecutors } = await import('./workflow/StepExecutors')
const dagBuilder = new DAGBuilder()
const stepExecutors = new StepExecutors(projectRouter)
const workflowOrchestrator = new WorkflowOrchestrator(pool, dagBuilder, stepExecutors, projectRouter)
await workflowOrchestrator.resumeRunningExecutions().catch(err =>
  console.warn('[ServerBootstrap] resumeRunningExecutions (non-fatal):', err.message)
)
console.log('[ServerBootstrap] ✅ WorkflowOrchestrator initialized (v5.0)')
```

Update `return` block: add `workflowOrchestrator`.

## Acceptance Criteria

- [x] Step 12 thêm sau step 11 (AIProviderService)
- [x] `resumeRunningExecutions()` called (non-fatal)
- [x] `workflowOrchestrator` trong return block
