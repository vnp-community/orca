# TASK-FE-002.3: Đóng span đang mở khi `useAgentOrchestrationEvents` nhận `statusChanged`

**Phase:** 1
**SOL Ref:** [SOL-FE-TRACE-002 §1.6, §2.4](../solutions/SOL-FE-TRACE-002-agent-orchestration.md)
**CR Ref:** [CR-TRACE-002](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-002-agent-orchestration.md)
**Prerequisite:** Phase 0 (TASK-FE-000, TASK-FE-001) + TASK-FE-002.1 (registry) + TASK-FE-002.2 (span mở từ `startAgent`/`resumeAgent`)
**Status:** ✅ Done (2026-08-03) — Instrumented `useAgentOrchestrationEvents()` to step()/ok()/fail() the already-open registry span from TASK-FE-002.1/002.2 on `statusChanged` push events, exactly matching BL-AG-05 (no dedicated span per event); confirmed via gitnexus_impact LOW/0 upstream callers (orphan hook, same AgentPanel.tsx tree). New test file `hooks/__tests__/use-agent-orchestration-events.test.ts` with 7 cases covering running→ok+registry-clear, running-with-no-open-span (no throw), error→fail, error-with-no-open-span (no throw), starting→step-without-removal, stopped→registry-untouched, and no-extra-start-event-per-status-push. `pnpm tsc --noEmit` clean, 7/7 tests pass.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "useAgentOrchestrationEvents"
```

Nếu symbol đã tồn tại (MODIFY case): chạy thêm

```
gitnexus_impact({ target: "useAgentOrchestrationEvents", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, component/hook bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

**Lưu ý orphan component:** hook này chỉ phục vụ cây `AgentPanel.tsx`, hiện KHÔNG được mount/render ở đâu trong app (xem mục Mô tả) — `gitnexus_impact` nhiều khả năng trả về risk LOW hoặc không có caller thực nào. Đây là kết quả ĐÚNG NHƯ MONG ĐỢI, không phải dấu hiệu sai sót.

## Mô tả

BL-AG-05 (Monitor Trạng thái Real-time): monitor thật là **push event rời rạc** qua `window.api.agentOrchestration.onStatusChanged` — không phải stream byte-level như CR-TRACE-002 gốc giả định (OSC parsing không tồn tại). `status` chỉ nhận 4 giá trị (`starting|running|stopped|error`). Nguyên tắc bắt buộc: **KHÔNG tạo tracer/span riêng nào cho mỗi sự kiện `statusChanged`** — chỉ `step()`/`ok()`/`fail()` trên span đã có sẵn trong registry (chống over-instrumentation, CR-TRACE-000 §5).

## File: `src/renderer/src/hooks/use-agent-orchestration-events.ts` [MODIFY]

```typescript
// src/renderer/src/hooks/use-agent-orchestration-events.ts
import { useEffect } from 'react'
import { useAppStore } from '../store'
import { peekOpenAgentOrchSpan, takeOpenAgentOrchSpan } from '@/lib/agent-orchestration-active-spans'

export function useAgentOrchestrationEvents(): void {
  const updateAgentStatus = useAppStore(s => s.updateAgentStatus)

  useEffect(() => {
    const unsubscribe = window.api.agentOrchestration.onStatusChanged(event => {
      updateAgentStatus({
        worktreeId: event.worktreeId,
        sessionId: event.sessionId,
        status: event.status,
        errorMessage: event.errorMessage,
      })

      // BL-AG-05: không tạo span riêng cho mỗi statusChanged — chỉ step()/đóng span
      // của BL-AG-01/03 NẾU nó còn mở cho worktree này.
      if (event.status === 'running') {
        const span = takeOpenAgentOrchSpan(event.worktreeId)
        span?.ok({ worktreeId: event.worktreeId, sessionId: event.sessionId ?? '', status: event.status })
      } else if (event.status === 'error') {
        const span = takeOpenAgentOrchSpan(event.worktreeId)
        span?.fail(new Error(event.errorMessage ?? 'agent error'), { worktreeId: event.worktreeId })
      } else if (event.status === 'starting') {
        // Trung gian — vẫn còn ui:agentOrch.spawn/resume đang chạy, chỉ log step().
        peekOpenAgentOrchSpan(event.worktreeId)?.step('statusChanged', { status: event.status })
      }
      // 'stopped' không đụng vào registry: stopAgent() đã tự đóng span ui:agentOrch.stop
      // của chính nó (TASK-FE-002.2) — statusChanged 'stopped' ở đây chỉ đồng bộ store.
    })
    return unsubscribe
  }, [updateAgentStatus])
}
```

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/renderer/src/hooks/__tests__/use-agent-orchestration-events.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `useAgentOrchestrationEvents()` đóng đúng span đang mở khi nhận `statusChanged` với `status: 'running'` (→ `ok()`) hoặc `status: 'error'` (→ `fail()`)
- [ ] Không throw khi không có span nào đang mở cho worktree đó (branch an toàn — `takeOpenAgentOrchSpan`/`peekOpenAgentOrchSpan` trả `undefined`)
- [ ] `status: 'starting'` → `span.step('statusChanged', { status: 'starting' })`, span KHÔNG bị xoá khỏi registry
- [ ] `status: 'stopped'` → KHÔNG đụng registry (không gọi `peek`/`take`)
- [ ] KHÔNG có tracer/span riêng nào được tạo cho mỗi sự kiện `statusChanged` — chỉ thao tác trên span đã có sẵn trong registry
- [ ] Test suite đạt ≥ 5 test case mới: `running` với span mở → `ok()` + registry rỗng sau đó; `running` không có span mở → không throw, không gọi gì; `error` với span mở → `fail(Error(errorMessage), ...)`; `starting` → `step()`, span không bị xoá; `stopped` → không đụng registry
