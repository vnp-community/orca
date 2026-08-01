# BUG-FE-WF-001: Workflow Builder UI không tồn tại trong Renderer — không có template library, không có execution monitor

## Mức độ: 🔴 HIGH (Feature Missing)

## Tóm tắt

HLD (BL-WF-01 → BL-WF-03) mô tả UI:
```
[Browser] Workflow Library → "New Template"
    Input YAML/JSON: { name, steps: [...] }

[Browser] Workflow Library → chọn template → "Execute"

[Browser] Execution Monitor: SSE real-time progress
    Per-wave: progress bar, step status (running/done/failed)

[Browser] My Workflows → template → "Share" → visibility toggle
```

Grep toàn bộ `src/renderer/` không tìm thấy:
```
WorkflowBuilder    → No results
TemplateLibrary    → No results
WorkflowExecution  → No results
workflow.execute   → No results
workflow/templates → No results
```

## Ảnh hưởng

1. **BL-WF-01**: Template CRUD UI — không có.
2. **BL-WF-02**: Execution monitor với wave progress — không có.
3. **BL-WF-03**: Library discovery + share — không có.
4. User Lead/Developer không thể tạo hoặc chạy workflow từ browser.

## Files không tồn tại

- `src/renderer/src/components/workflow/workflow-builder.tsx`
- `src/renderer/src/components/workflow/workflow-template-library.tsx`
- `src/renderer/src/components/workflow/workflow-execution-monitor.tsx`
- `src/renderer/src/pages/workflows.tsx`

## Liên quan đến luồng

- **BL-WF-01 → BL-WF-03**: Toàn bộ Workflow UI không có.
