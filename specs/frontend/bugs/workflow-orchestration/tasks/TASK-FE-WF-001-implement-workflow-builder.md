# TASK-FE-WF-001: Implement Workflow Builder UI

**Priority:** 🟡 MEDIUM  
**Effort:** ~180 phút  
**Status:** ✅ DONE — Implemented  
**Bug refs:** BUG-FE-WF-001  
**Solution ref:** [SOL-FE-WF-001](../solutions/SOL-FE-WF-001-workflow-builder-ui-implementation.md)

## Mục tiêu

Implement visual workflow builder cho phép user tạo DAG workflow với step types: `agent.exec`, `git.push`, `condition`, `wait`.

## Thay đổi

Xem [SOL-FE-WF-001](../solutions/SOL-FE-WF-001-workflow-builder-ui-implementation.md) để biết full implementation.

## Files cần tạo

- `src/renderer/src/components/workflow/WorkflowBuilder.tsx`
- `src/renderer/src/components/workflow/StepNode.tsx`
- `src/renderer/src/components/workflow/StepConfigPanel.tsx`
- `src/renderer/src/hooks/useWorkflow.ts`

## Verification

```bash
pnpm tsc --noEmit
# Test: render WorkflowBuilder → add steps → connect steps → save workflow
```
