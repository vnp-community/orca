# TASK-FE-002.1: Đăng ký tracer + tạo registry span đang mở cho Agent Orchestration

**Phase:** 1
**SOL Ref:** [SOL-FE-TRACE-002 §2.1, §2.2](../solutions/SOL-FE-TRACE-002-agent-orchestration.md)
**CR Ref:** [CR-TRACE-002](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-002-agent-orchestration.md)
**Prerequisite:** Phase 0 (TASK-FE-000, TASK-FE-001)
**Status:** ✅ Done (2026-08-03) — Drift (same collision pattern as TASK-FE-001.1, flagged by coordinator): existing `Tracers.agentOrchSpawn/Stop/Resume/Switch/StatusPoll` (flow `agentOrch:*`) are agent-domain-owned (`pty-agent-bridge.ts`/agent-spawner), not to be renamed — added 5 NEW distinct entries `uiAgentOrchSpawnFlow/StopFlow/ResumeFlow/SwitchFlow/StatusPollFlow` (`ui:agentOrch.*`) instead. Created `src/renderer/src/lib/agent-orchestration-active-spans.ts` registry (registerOpenAgentOrchSpan/takeOpenAgentOrchSpan/peekOpenAgentOrchSpan) + colocated test file (no `__tests__` subfolder convention exists in `lib/`, matched sibling files instead); confirmed via codegraph_explore this serves `AgentPanel.tsx`, an orphan/unmounted component tree — expected per task note. `pnpm tsc --noEmit` clean, 4/4 new tests pass.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "Tracers"
```

Nếu symbol đã tồn tại (MODIFY case, áp dụng cho phần sửa `tracers.ts`): chạy thêm

```
gitnexus_impact({ target: "Tracers", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, component/hook bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

**Lưu ý:** `tracers.ts` là registry isomorphic dùng chung giữa domain frontend/agent/backend — `gitnexus_impact` ở đây đặc biệt hữu ích để phát hiện xung đột tên biến/flow-string với các task domain khác đang thêm entry song song vào cùng file.

**File mới `agent-orchestration-active-spans.ts`:** không có symbol hiện có để chạy `gitnexus_impact`. Thay vào đó, chạy `codegraph explore "TraceSpan"` để hiểu type mà registry mới này bọc quanh, trước khi viết code.

**Lưu ý orphan component:** registry này chỉ phục vụ `AgentPanel.tsx`, hiện KHÔNG được mount/render ở đâu trong app (xem mục Mô tả) — `gitnexus_impact`/kết quả explore liên quan nhiều khả năng cho thấy risk LOW hoặc không có caller thực nào ngoài chính cây component chưa mount này. Đây là kết quả ĐÚNG NHƯ MONG ĐỢI, không phải dấu hiệu sai sót.

## Mô tả

**⚠️ Cảnh báo mount status:** `AgentPanel.tsx` + `use-agent-orchestration-events.ts` + `src/main/ipc/agent-orchestration.ts` (tag `BUG-FE-ORCH-001`) là một implementation đầy đủ chức năng qua Electron IPC (`window.api.agentOrchestration.*`) nhưng **KHÔNG được import/mount ở đâu trong app** — đã grep toàn bộ `src/renderer/src` cho `workspace/AgentPanel`, chỉ chính file này tự tham chiếu. Task này (và TASK-FE-002.2/002.3) vẫn instrument đầy đủ theo đúng yêu cầu CR, nhưng acceptance criteria KHÔNG thể bao gồm "verify qua click thật trong app" cho tới khi gap mount này được vá (ngoài phạm vi CR tracing).

Đây là instance thứ 2 (sau BL-WT-01) của pattern kiến trúc "renderer luôn có 2 nhánh — WS RPC vs Electron IPC" — ở đây toàn bộ đi qua Electron `contextBridge` IPC, KHÔNG nằm trong 6 hàng transport CR-TRACE-000 §3.3.

Task này tạo file registry mới (`agent-orchestration-active-spans.ts`) — cần thiết vì `startAgent()`/`resumeAgent()` mở span nhưng kết quả "running" thật chỉ tới sau qua push event `agentOrchestration:statusChanged` (IPC), không phải response của lệnh start/resume.

## File: `src/shared/trace/tracers.ts` [MODIFY, additive]

```typescript
// src/shared/trace/tracers.ts
export const Tracers = {
  // ...existing entries unchanged...
  agentOrchSpawn:      createTracer('ui:agentOrch.spawn'),      // BL-AG-01
  agentOrchStop:       createTracer('ui:agentOrch.stop'),       // BL-AG-02
  agentOrchResume:     createTracer('ui:agentOrch.resume'),     // BL-AG-03
  agentOrchSwitch:     createTracer('ui:agentOrch.switch'),     // BL-AG-04 — chưa có UI, đặt tên sẵn
  agentOrchStatusPoll: createTracer('ui:agentOrch.statusPoll'), // BL-AG-05 — dự phòng, không dùng làm span riêng (xem TASK-FE-002.3)
} as const
```

> N.B. prefix `ui:`: bắt buộc theo convention chung (xem TASK-FE-001.1, `00-index.md` mục 1) — 5 tracer trên dùng prefix `ui:` nhất quán với toàn bộ 10 CR, để `isBackend` heuristic của `TracePanel.tsx:42` không gắn nhầm badge "backend" cho event browser tự phát.

## File: `src/renderer/src/lib/agent-orchestration-active-spans.ts` [NEW]

```typescript
// src/renderer/src/lib/agent-orchestration-active-spans.ts
// Why: startAgent()/resumeAgent() mở span ui:agentOrch.spawn|resume nhưng kết quả
// "running" thật chỉ tới sau, qua push event agentOrchestration:statusChanged
// (IPC, không phải response của lệnh start/resume). Registry này cho phép hook
// nhận event gắn step()/ok()/fail() vào đúng span đang mở theo BL-AG-05.
import type { TraceSpan } from '../../../../shared/trace'

const openSpansByWorktreeId = new Map<string, TraceSpan>()

export function registerOpenAgentOrchSpan(worktreeId: string, span: TraceSpan): void {
  openSpansByWorktreeId.set(worktreeId, span)
}

/** Lấy và xoá span đang mở cho worktree này (nếu có) — dùng khi đóng span. */
export function takeOpenAgentOrchSpan(worktreeId: string): TraceSpan | undefined {
  const span = openSpansByWorktreeId.get(worktreeId)
  openSpansByWorktreeId.delete(worktreeId)
  return span
}

/** Xem span đang mở mà không xoá — dùng cho step() giữa chừng (status vẫn 'starting'). */
export function peekOpenAgentOrchSpan(worktreeId: string): TraceSpan | undefined {
  return openSpansByWorktreeId.get(worktreeId)
}
```

Đặt tên file theo đúng nội dung nó chứa (không dùng tên chung chung như `agent-utils.ts`), theo quy tắc đặt tên file của dự án.

## Verification

```bash
pnpm tsc --noEmit
pnpm test --run src/renderer/src/lib/__tests__/agent-orchestration-active-spans.test.ts
pnpm test --run src/shared/trace/__tests__/tracers.test.ts
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `Tracers.agentOrchSpawn/Stop/Resume/Switch/StatusPoll` thêm vào `tracers.ts` đúng tên `ui:agentOrch.spawn|stop|resume|switch|statusPoll`
- [ ] File mới `src/renderer/src/lib/agent-orchestration-active-spans.ts` export `registerOpenAgentOrchSpan`, `takeOpenAgentOrchSpan`, `peekOpenAgentOrchSpan`
- [ ] `registerOpenAgentOrchSpan()` rồi `takeOpenAgentOrchSpan()` cùng `worktreeId` → trả về đúng span, lần gọi thứ 2 trả `undefined`
- [ ] `peekOpenAgentOrchSpan()` không xoá span khỏi registry
- [ ] 2 `worktreeId` khác nhau không đụng nhau (`Map` theo key riêng)
- [ ] `ui:agentOrch.switch` (BL-AG-04) chỉ có tên tracer trong `tracers.ts`, không có call site code nào (chưa có UI switch-account)
- [ ] Test suite đạt ≥ 3 test case mới cho registry
