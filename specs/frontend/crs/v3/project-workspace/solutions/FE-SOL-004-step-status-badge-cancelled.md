# FE-SOL-004: `StepStatusBadge` xử lý `'cancelled'` (`WorkflowExecutionStatus`)

> **✅ ĐÃ IMPLEMENT (2026-09-06)** — đúng như kế hoạch dưới, không lệch. `StepStatusBadge.test.tsx`
> mới (3 test case) + 1 test case mới thêm vào `ExecutionMonitor.test.tsx` (regression cho đúng
> chỗ crash cũ). `vitest run` trên cả 2 file: **13/13 pass**. `gitnexus impact(StepStatusBadge,
> upstream)`: risk **LOW**, 1 caller trực tiếp (`ExecutionMonitor.tsx`), 0 execution flow bị ảnh
> hưởng. `gitnexus detect_changes({scope:"all"})` sau khi xong toàn bộ phiên làm việc (CR-PW-004 +
> 005 + 006): risk **low**, 0 affected processes.

## CR Reference
- **CR:** [CR-PW-004](../../../../../../docs/crs/v3/project-workspace/CR-PW-004-step-status-badge-cancelled-crash.md)
- **Mức độ:** 🔴 P0 (crash)
- **Impact analysis (gitnexus):** `StepStatusBadge` (`components/workflow/StepStatusBadge.tsx`) —
  risk LOW, impactedCount 1, 0 execution flow, module `Workflow`.

---

## Root Cause

`STEP_STATUS: Record<StepStatus, ...>` chỉ có 5 key (`pending/running/completed/failed/skipped`).
`ExecutionMonitor.tsx` truyền `execution.status` (type `WorkflowExecutionStatus`, có thêm
`cancelled`, không có `skipped`) vào component này qua 1 cast `as any` — cast này che mất lỗi kiểu
lẽ ra TypeScript phải bắt được, và ở runtime `STEP_STATUS['cancelled']` là `undefined`.

## Giải pháp

### Bước 1 — Mở rộng `StepStatusBadge`'s prop type + map

**File:** `frontend/src/renderer/src/components/workflow/StepStatusBadge.tsx` (MODIFY)

```tsx
import type { StepStatus, WorkflowExecutionStatus } from '@shared/workflow-types'
import { CheckCircle2, Loader2, Clock, XCircle, SkipForward, Ban } from 'lucide-react'
import { cn } from '../../lib/utils'
import type { ReactNode } from 'react'

// Accepts both StepStatus (a single step) and WorkflowExecutionStatus (a whole
// execution) — ExecutionMonitor.tsx renders this badge for both. The two enums
// overlap on 4 values but diverge on the 5th ('skipped' vs 'cancelled'), so the
// map below is keyed on their union rather than either type alone (CR-PW-004;
// was `STEP_STATUS[status]` throwing on 'cancelled' via an `as any` cast).
type BadgeStatus = StepStatus | WorkflowExecutionStatus

const STEP_STATUS: Record<BadgeStatus, { icon: ReactNode; className: string; label: string }> = {
  pending:   { icon: <Clock      size={14} />, className: 'text-gray-400',  label: 'Pending'   },
  running:   { icon: <Loader2    size={14} className="animate-spin" />, className: 'text-blue-500',  label: 'Running'   },
  completed: { icon: <CheckCircle2 size={14} />, className: 'text-green-500', label: 'Completed' },
  failed:    { icon: <XCircle    size={14} />, className: 'text-red-500',   label: 'Failed'    },
  skipped:   { icon: <SkipForward size={14} />, className: 'text-gray-400', label: 'Skipped'   },
  cancelled: { icon: <Ban        size={14} />, className: 'text-gray-500', label: 'Cancelled' },
}

export function StepStatusBadge({ status }: { status: BadgeStatus }) {
  const { icon, className, label } = STEP_STATUS[status]
  return (
    <span className={cn('flex items-center gap-1 text-xs font-medium', className)}>
      {icon} {label}
    </span>
  )
}
```

`Ban` (lucide-react) chọn cho `cancelled` vì đã có sẵn trong bộ icon dùng chung, khác hình dạng rõ
ràng với 5 icon còn lại (không trùng `XCircle` của `failed`, tránh nhầm "failed" với "cancelled").

### Bước 2 — Bỏ cast `as any` ở call site thật sự bug

**File:** `frontend/src/renderer/src/components/workflow/ExecutionMonitor.tsx` (MODIFY, 1 dòng)

```diff
- <StepStatusBadge status={execution.status as any} />
+ <StepStatusBadge status={execution.status} />
```

Call site còn lại (dòng ~53, `<StepStatusBadge status={status} />` trong vòng lặp step) đã đúng
kiểu `StepStatus` sẵn — không cần đổi, vẫn compile được vì `StepStatus` là subset của
`BadgeStatus`.

## Files cần sửa

| File | Action |
|------|--------|
| `frontend/src/renderer/src/components/workflow/StepStatusBadge.tsx` | MODIFY |
| `frontend/src/renderer/src/components/workflow/ExecutionMonitor.tsx` | MODIFY — bỏ `as any` |
| `frontend/src/renderer/src/components/workflow/__tests__/StepStatusBadge.test.tsx` | CREATE |
| `frontend/src/renderer/src/components/workflow/__tests__/ExecutionMonitor.test.tsx` | MODIFY — thêm 1 regression test |

## Task breakdown

- [FE-TASK-006](../tasks/FE-TASK-006-step-status-badge-cancelled.md)

## Verification

```bash
cd frontend && npx vitest run src/renderer/src/components/workflow/__tests__/StepStatusBadge.test.tsx src/renderer/src/components/workflow/__tests__/ExecutionMonitor.test.tsx
# → 13/13 pass
```

`npx tsc --noEmit -p .` trong worktree này có ~146 lỗi pre-existing không liên quan (xác nhận
bằng `git stash` trước khi sửa: cùng số lỗi, cùng danh sách file, không đổi sau khi thêm CR-PW-004)
— chủ yếu do path alias `@shared` không nằm trong `tsconfig.json` của worktree này dù `vite`/
`vitest` alias nó đúng (soi từ `StepStatusBadge.tsx`/`ExecutionMonitor.tsx` vốn đã import
`@shared/workflow-types` **trước cả CR-PW-004**). Không phải lỗi do CR này gây ra — verification
thật của thay đổi này là `vitest run` (chạy trên Vite's module resolver, có alias đúng), không
phải `tsc -p .` (dùng tsconfig của Node-style resolver, thiếu alias) trên worktree này.

## Không làm ở solution này

- Không đổi `useWorkflowExecution.ts` — đó là CR-PW-006.
- Không sửa `tsconfig.json` để thêm `@shared` alias cho `tsc -p .` — đây là 1 baseline gap có sẵn,
  ảnh hưởng ~146 lỗi không liên quan đến CR-PW-004/005/006; sửa nó là thay đổi cấu hình build ảnh
  hưởng toàn bộ project, ngoài phạm vi 1 bug fix UI component. Ghi nhận riêng, không sửa ở đây.
