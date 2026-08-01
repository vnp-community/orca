# BUG-BE-WF-001: `WorkflowOrchestrator`, `TemplateRegistry` và DAG execution engine chưa được implement

**Status:** ✅ FIXED — 2026-08-01  
**Task:** TASK-WF-001,002  
**Note:** WorkflowOrchestrator.ts implemented with wave execution + resume  

## Mức độ: 🔴 HIGH (Feature Missing)

## Tóm tắt

HLD (BL-WF-01 → BL-WF-03) mô tả:
```
WorkflowOrchestrator:
    - DAG build + topological sort
    - Wave-based parallel execution
    - StepExecutors: agent, shell, webhook, condition, parallel
    - Resume sau Orca restart
    
TemplateRegistry:
    - CRUD workflow templates
    - Inheritance chain (parent → child override)
    - TemplateResolver (deepMerge + inject/remove steps)
```

Grep toàn bộ `src/` không tìm thấy:
```
WorkflowOrchestrator      → No results
TemplateRegistry          → No results
orca_workflow_templates   → No results
orca_workflow_executions  → No results
orca_step_executions      → No results
WorkflowServerResolver    → No results
```

## Ảnh hưởng

1. **Toàn bộ BL-WF domain (BL-WF-01 → BL-WF-03) chưa implement**.
2. Multi-server workflow execution không có.
3. Template sharing/discovery không có.
4. `POST /api/workflows/execute` chưa tồn tại.

## Files không tồn tại (theo HLD)

- `src/main/workflow/workflow-orchestrator.ts`
- `src/main/workflow/template-registry.ts`
- `src/main/workflow/template-resolver.ts`
- `src/main/workflow/workflow-server-resolver.ts`
- `src/main/workflow/step-executors/agent.ts`
- `src/main/workflow/step-executors/shell.ts`
- DB migrations: `orca_workflow_templates`, `orca_workflow_executions`, `orca_step_executions`

## Liên quan đến luồng

- **BL-WF-01**: Template management — không có.
- **BL-WF-02**: Multi-server execution — không có.
- **BL-WF-03**: Sharing & discovery — không có.
