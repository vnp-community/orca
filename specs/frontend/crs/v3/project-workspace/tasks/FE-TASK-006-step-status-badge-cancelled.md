# FE-TASK-006: `StepStatusBadge` xử lý `'cancelled'`

**Domain:** project-workspace
**Solution Ref:** FE-SOL-004
**Priority:** 🔴 P0 (crash)
**Estimated:** 20 phút
**Status:** ✅ DONE (2026-09-06)

**Kết quả thực tế:** Đúng như kế hoạch, không lệch. 3 test case mới trong
`StepStatusBadge.test.tsx` + 1 regression test mới trong `ExecutionMonitor.test.tsx`.
`vitest run` trên cả 2 file: **13/13 pass** (10 pre-existing `ExecutionMonitor.test.tsx` tests
vẫn pass nguyên, không có test nào bị sửa/xoá).

---

## Mục tiêu

Sửa crash khi `ExecutionMonitor` render badge cho execution có `status: 'cancelled'`.

## Files cần sửa

1. `frontend/src/renderer/src/components/workflow/StepStatusBadge.tsx` (MODIFY — code mẫu đầy đủ
   ở FE-SOL-004 Bước 1)
2. `frontend/src/renderer/src/components/workflow/ExecutionMonitor.tsx` (MODIFY — bỏ `as any`,
   1 dòng)
3. `frontend/src/renderer/src/components/workflow/__tests__/StepStatusBadge.test.tsx` (CREATE)
4. `frontend/src/renderer/src/components/workflow/__tests__/ExecutionMonitor.test.tsx` (MODIFY —
   thêm 1 test case)

## Test cases cần cover

- `StepStatusBadge.test.tsx`: render đủ cả 6 status (`pending/running/completed/failed/skipped/
  cancelled`) không throw, đúng label cho từng status, có 1 test riêng nêu rõ đây là regression
  cho CR-PW-004 (`'cancelled'` không nằm trong `StepStatus`).
- `ExecutionMonitor.test.tsx`: mock `useWorkflowExecution` trả `execution.status = 'cancelled'`,
  assert `render()` không throw, badge hiện "Cancelled", và nút Cancel không hiện (chỉ hiện khi
  `status === 'running'`).

## Verify

```bash
cd frontend && npx vitest run src/renderer/src/components/workflow/__tests__/StepStatusBadge.test.tsx src/renderer/src/components/workflow/__tests__/ExecutionMonitor.test.tsx
```

**Kết quả thật:** `Test Files 2 passed (2)`, `Tests 13 passed (13)`.

## gitnexus

- `impact({target:"StepStatusBadge", direction:"upstream"})` trước khi sửa: risk **LOW**,
  impactedCount 1 (chỉ `ExecutionMonitor.tsx`), 0 execution flow bị ảnh hưởng — an toàn để sửa
  trực tiếp, không cần feature flag.

## Depends on
Không có (độc lập với FE-TASK-001..005)

## Blocking
Không có
