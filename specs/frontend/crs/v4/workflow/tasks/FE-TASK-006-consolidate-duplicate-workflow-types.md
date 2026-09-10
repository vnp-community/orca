# FE-TASK-006: Dọn dẹp 2 file `workflow-types.ts` trùng lặp — retire file chết

**Domain:** workflow
**Solution Ref:** không trực tiếp — phát hiện khi làm [FE-TASK-003](./FE-TASK-003-pause-resume-execution-status-type.md), không phải gap của FE-SOL-001/002
**Priority:** 🟡 P2 — technical debt, không chặn chức năng nào, nhưng rủi ro cao cho người sửa sau (sửa nhầm file im lặng không lỗi)
**Estimated:** 30 phút
**Status:** [x] DONE

---

## Kết quả thực thi (2026-09-09)

- Xác nhận lại bằng `grep`: chỉ có 1 nơi import file chết (`DAGPreview.tsx`, chỉ dùng `WorkflowStep`).
- Đổi import trong `DAGPreview.tsx` sang `@shared/workflow-types` (alias đã dùng ở 7 file
  khác trong domain workflow), rồi xoá hẳn `frontend/src/renderer/src/types/workflow-types.ts`.
- `grep -rn "renderer/src/types/workflow-types"` sau khi xoá → 0 kết quả, không còn tham chiếu treo.
- `npx vitest run DAGPreview.test.tsx` → 5/5 pass. `pnpm typecheck` chạy chung ở cuối đợt (xem báo
  cáo tổng); không phát sinh lỗi liên quan file này.
- Làm task này SỚM (trước cả FE-TASK-003/004 của workflow) đúng khuyến nghị trong ghi chú điều phối
  — loại bỏ hẳn rủi ro nhầm lẫn 2 file `workflow-types.ts` cho các task sau.

## Mục tiêu

Có **2 file `workflow-types.ts` độc lập** trong frontend, nội dung gần
giống nhau nhưng đã lệch (`diff` xác nhận: khác comment style, khác field
`timeout`, và `frontend/src/shared/workflow-types.ts` có thêm
`WorkflowExecution.rootTraceId?` mà file kia không có):

- `frontend/src/shared/workflow-types.ts` — file **thật sự đang dùng**,
  xác nhận `grep -rln "@shared/workflow-types\|shared/workflow-types'"
  frontend/src` → 7 file (mọi component/hook Workflow thật:
  `useWorkflowExecution.ts`, `useWorkflow.ts`, `ExecutionMonitor.tsx`,
  `WorkflowMonitor.tsx`, `StepStatusBadge.tsx`, `StepList.tsx`,
  `store/slices/workflow.ts`).
- `frontend/src/renderer/src/types/workflow-types.ts` — chỉ **1 nơi**
  import: `DAGPreview.tsx` (chỉ dùng `WorkflowStep`, không dùng
  `WorkflowExecutionStatus`).

Đây chính là bug-magnet đã gây sai lệch thật: [FE-TASK-003](./FE-TASK-003-pause-resume-execution-status-type.md)
phải ghi chú rõ "sửa đúng file `shared/`, không phải file `renderer/src/
types/`" vì FE-SOL-001 chỉ ghi "workflow-types.ts" không phân biệt — nếu
không dọn dẹp, mỗi task sau này chạm vào workflow types đều phải tự re-
verify lại từ đầu.

## Files cần sửa

| File | Action |
|------|--------|
| `frontend/src/renderer/src/types/workflow-types.ts` | XOÁ |
| `frontend/src/renderer/src/components/workflow/DAGPreview.tsx` | MODIFY — đổi import sang `@shared/workflow-types` |
| `frontend/src/renderer/src/components/workflow/__tests__/DAGPreview.test.tsx` | MODIFY — cập nhật import nếu test tự import type trực tiếp |

## Các bước thực thi

### 1. Xác nhận `shared/workflow-types.ts` có đủ mọi export `DAGPreview.tsx` cần

```bash
grep -n "^import.*workflow-types" frontend/src/renderer/src/components/workflow/DAGPreview.tsx
# Xác nhận DAGPreview chỉ dùng WorkflowStep (và có thể WorkflowDefinition) — cả 2 đã có ở shared/workflow-types.ts
```

### 2. Đổi import trong `DAGPreview.tsx`

```diff
- import type { WorkflowStep } from '../../types/workflow-types'
+ import type { WorkflowStep } from '@shared/workflow-types'
```

### 3. Xoá file cũ

```bash
rm frontend/src/renderer/src/types/workflow-types.ts
```

### 4. Build lại để xác nhận không còn import nào trỏ tới file đã xoá

```bash
cd /opt/repos/orca/frontend
grep -rn "renderer/src/types/workflow-types\|'\.\./\.\./types/workflow-types'\|'\.\./types/workflow-types'" src/
# Kỳ vọng: 0 kết quả
pnpm typecheck
```

## Test cases cần cover

- `DAGPreview.tsx` render đúng như trước sau khi đổi import (snapshot/existing test không đổi behavior).
- `pnpm typecheck` sạch, không còn reference nào tới file đã xoá.

## Verify

```bash
cd /opt/repos/orca/frontend
pnpm typecheck
pnpm test -- DAGPreview
```
