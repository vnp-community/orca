# TASK-032: Workflow RPC Methods

**Phase:** 5 — Workflow Orchestration  
**Prerequisite:** TASK-029  
**Status:** ✅ DONE — 2026-07-29

---

## File cần tạo: `src/main/workflow/workflow-rpc-handler.ts`

**Methods:**
- `workflow.execute` → `orchestrator.execute(params.definition, params.inputs, session.userId, params.projectId?)`
- `workflow.getExecution` → `orchestrator.getExecution(params.executionId)`
- `workflow.listExecutions` → `orchestrator.listExecutions(params)`
- `workflow.cancel` → `orchestrator.cancel(params.executionId)`
- `workflow.template.create` → `templateResolver.create(params)`
- `workflow.template.list` → `templateResolver.list(params.scope, session.userId)`
- `workflow.template.resolve` → `templateResolver.resolve(params.templateId)`

**Access control:**
- All ops check project membership if `projectId` provided
- `workflow.cancel`: only triggeredBy user or admin

## Acceptance Criteria

- [x] 7 RPC methods registered
- [x] `execute` returns execution ID (non-blocking)
- [x] Không TypeScript errors
