# FE-SOL-005: `useWorkflowExecution` — polling interim thay cho `window.api.on` (CR-PW-006 Phase A)

> **✅ ĐÃ IMPLEMENT (2026-09-06)** — Phase A only (execution-level status polling); Phase B/C/D/E
> **KHÔNG** implement trong solution này, xem CR-PW-006's "Trạng thái triển khai". `vitest run`
> trên `useWorkflowExecution.test.ts`: **10/10 pass** (5 test cũ không đổi + 5 test polling mới,
> dùng `vi.useFakeTimers()`). `gitnexus impact(useWorkflowExecution, upstream)`: risk **LOW**, 1
> caller trực tiếp (`ExecutionMonitor.tsx`), 0 execution flow bị ảnh hưởng.

## CR Reference
- **CR:** [CR-PW-006](../../../../../../docs/crs/v3/project-workspace/CR-PW-006-execution-monitoring-architecture.md) — chỉ Phase A
- **Mức độ:** 🟡 P1
- **Depends on:** [CR-PW-005](../../../../../../docs/crs/v3/project-workspace/CR-PW-005-wscompat-missing-workflow-rpcs.md) / [BE-SOL-001](../../../../../backend-go/crs/v3/project-workspace/solutions/BE-SOL-001-workflow-wscompat-wiring.md) — `workflow.getExecution` phải được nối dây ở backend-go's wscompat trước khi polling này hoạt động ở Web mode (đã xong).

---

## Root Cause (nhắc lại từ CR-PW-006)

`window.api.on('workflow:stepStatus'|'workflow:stepOutput'|'workflow:complete', cb)` là bridge
Electron-only. Guard `if (!(window as any).api?.on) {return}` KHÔNG bắt được trường hợp Web mode vì
`window.api` là 1 `Proxy` (`withFallback`/`createFallbackProxy` trong `web-preload-api.ts`) mà
`.on` resolve qua fallback — 1 stub callable, truthy, trả về `noopUnsubscribe` ngay lập tức mà
không đăng ký gì cả. Kết quả: event bị rớt âm thầm mãi mãi ở Web mode, không lỗi, không cảnh báo.

Không có transport push JSON chung nào có sẵn để thay thế ngay (xem CR-PW-006's "Không có
transport push JSON chung nào để tái dùng") — xây 1 transport như vậy là Phase D/E, cross-repo,
rủi ro cao, ngoài phạm vi an toàn của phiên này.

## Giải pháp: polling `workflow.getExecution`

**File:** `frontend/src/renderer/src/hooks/useWorkflowExecution.ts` (MODIFY)

Thay `useEffect` đăng ký `window.api.on(...)` bằng 1 `useEffect` poll `workflow.getExecution` mỗi
4 giây, CHỈ khi `execution.status === 'running'` (dừng hẳn khi status là terminal
`completed`/`failed`/`cancelled`, hoặc không xác định):

```typescript
import { useEffect, useCallback } from 'react'
import { useAppStore } from '../store'
import { callRuntimeRpc, getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'
import { Tracers } from '../../../shared/trace/tracers'
import type { WorkflowExecution } from '@shared/workflow-types'

const EXECUTION_POLL_INTERVAL_MS = 4_000

export function useWorkflowExecution(executionId: string) {
  const { stepStatuses, streamingOutput } = useAppStore(s => ({
    stepStatuses:    s.stepStatuses[executionId] ?? {},
    streamingOutput: s.streamingOutput,
  }))
  const execution = useAppStore(s => s.executions.find(e => e.id === executionId))
  const executionStatus = execution?.status

  useEffect(() => {
    if (!executionId || executionStatus !== 'running') {return}
    let cancelled = false
    const poll = async () => {
      try {
        const target = getActiveRuntimeTarget(useAppStore.getState().settings)
        const result = await callRuntimeRpc<WorkflowExecution>(target, 'workflow.getExecution', {
          executionId,
        })
        if (!cancelled) {
          useAppStore.getState().updateExecutionStatus(executionId, result.status)
        }
      } catch {
        // Transient RPC failure — next tick retries.
      }
    }
    const intervalId = setInterval(() => { void poll() }, EXECUTION_POLL_INTERVAL_MS)
    return () => {
      cancelled = true
      clearInterval(intervalId)
    }
  }, [executionId, executionStatus])

  // ...cancelExecution unchanged...
}
```

Điểm quan trọng trong thiết kế:

- **Poll chỉ chạy khi `status === 'running'`** — effect's dependency array `[executionId,
  executionStatus]` khiến effect re-chạy (và cleanup interval cũ) mỗi khi status đổi; khi status
  chuyển sang terminal, effect cleanup xoá interval, không còn poll nào tiếp diễn.
- **`callRuntimeRpc` đã cross-platform sẵn** — cùng cơ chế `WorkflowMonitor.tsx` (CR-PW-003) dùng
  cho `workflow.listExecutions`: `target.kind === 'local'` gọi `window.api.runtime.call`,
  `'environment'` gọi `window.api.runtimeEnvironments.call` — cả 2 đường đều là RPC thật, không đi
  qua `window.api.on`'s Proxy fallback bug.
  Vì vậy Web mode giờ THẬT SỰ nhận được cập nhật status, dù chỉ ở tần suất polling, không phải
  push tức thời.
- **`cancelled` flag + `clearInterval` trong cleanup** — tránh race: unmount hoặc executionId đổi
  giữa lúc 1 poll đang in-flight không được phép ghi đè state của execution khác.
  - **Lỗi RPC transient bị nuốt (catch rỗng)** — cố ý: đây là poll nền, 1 lần fail không nên hiện
  error state (tick tiếp theo sẽ tự retry); khác với `cancelExecution` (1 hành động do user chủ
  động bấm) vẫn giữ nguyên hành vi throw lại lỗi.

## Giới hạn đã biết (ghi rõ trong code comment + CR doc, không che giấu)

- Chỉ cập nhật `execution.status` — KHÔNG có per-step (`stepStatuses`)/streaming output nào được
  cập nhật qua polling này. `stepStatuses`/`streamingOutput` trong store vẫn giữ nguyên state cũ
  (từ lúc mount, nếu có) cho tới khi Phase B (`ListStepExecutions`) tồn tại.
- Độ trễ tối đa 4 giây giữa lúc status thật sự đổi và lúc UI phản ánh — chấp nhận được cho 1
  interim measure, không phải giải pháp cuối.

## Files cần sửa

| File | Action |
|------|--------|
| `frontend/src/renderer/src/hooks/useWorkflowExecution.ts` | MODIFY |
| `frontend/src/renderer/src/hooks/__tests__/useWorkflowExecution.test.ts` | MODIFY — thêm 5 test case polling |

## Task breakdown

- [FE-TASK-007](../tasks/FE-TASK-007-execution-status-polling.md)

## Verification

```bash
cd frontend && npx vitest run src/renderer/src/hooks/__tests__/useWorkflowExecution.test.ts
# → 10/10 pass
cd frontend && npx vitest run src/renderer/src/components/workflow/ src/renderer/src/hooks/__tests__/useWorkflowExecution.test.ts src/renderer/src/store/slices/
# → broader regression sweep: 113 test files, 1761 tests, all pass
```

## Không làm ở solution này

- Per-step polling qua `ListStepExecutions` — RPC này **không tồn tại** ở backend-go hôm nay
  (CR-PW-006 Phase B, cần đổi `.proto`); không implement trong phiên này (quyết định rõ trong
  CR-PW-006's "Trạng thái triển khai" — rủi ro làm cùng lúc với Phase C không cần thiết).
- Bất kỳ push-transport mới nào (Phase D/E) — xem CR-PW-006.
- Sửa `ExecutionMonitor.tsx`'s wave/step rendering khi chạy trên model Go mỏng (thiếu
  `execution.definition`) — gap riêng, ghi nhận ở CR-PW-006, không sửa ở đây.
