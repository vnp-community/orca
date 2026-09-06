# FE-TASK-005: Nối `WorkflowMonitor` vào `workflow.listExecutions` + `ExecutionMonitor`

**Domain:** project-workspace
**Solution Ref:** FE-SOL-003 Bước 2-3
**Priority:** 🟡 P1
**Estimated:** 45 phút
**Status:** ✅ DONE (2026-09-06)

**Kết quả thực tế:** Đúng như kế hoạch, cả 5 test case đã liệt kê đều viết đúng như dự kiến.
`vitest run`: 5/5 pass trên file này.

---

## Mục tiêu

Thay stub tĩnh bằng component thật: fetch `workflow.listExecutions({projectId})`, render danh
sách, bấm vào 1 dòng mở `ExecutionMonitor` đã có sẵn.

## Files cần sửa

1. `frontend/src/renderer/src/components/workflow/WorkflowMonitor.tsx` (REWRITE — code mẫu đầy đủ
   ở FE-SOL-003 Bước 2)
2. `frontend/src/renderer/src/components/workspace/WorkspaceLayout.tsx` — truyền
   `projectId={project.id}`
3. `frontend/src/renderer/src/components/workflow/__tests__/WorkflowMonitor.test.tsx` (CREATE)

## Test cases cần cover

- Loading state khi đang fetch
- Empty state khi `executions.length === 0`
- Error state khi RPC throw
- Render đúng số dòng execution + status label
- Click 1 dòng → render `ExecutionMonitor` với đúng `executionId`, có nút quay lại danh sách

## Verify

```bash
cd frontend && npx vitest run src/renderer/src/components/workflow/__tests__/WorkflowMonitor.test.tsx
cd frontend && npx tsc --noEmit -p .
```

## Depends on
FE-TASK-004

## Blocking
Không có
