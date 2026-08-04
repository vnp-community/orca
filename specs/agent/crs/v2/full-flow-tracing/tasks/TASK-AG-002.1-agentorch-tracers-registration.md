# TASK-AG-002.1: Register agentOrch:* tracers in tracers.ts

**Phase:** 1
**SOL Ref:** [SOL-AG-TRACE-002](../solutions/SOL-AG-TRACE-002-agent-orchestration.md)
**CR Ref:** [CR-TRACE-002](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-002-agent-orchestration.md)
**Precondition:** Phase 0 + [TASK-AG-001.1](./TASK-AG-001.1-agent-git-handler-worktree-tracing.md) (cùng file `tracers.ts`, thêm cộng dồn — không bắt buộc thứ tự nhưng tránh conflict khi merge)
**Estimated time:** 0.5h
**Status:** ✅ Done (2026-08-03) — 5 entries added, additive, `pnpm run typecheck:node` clean.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "Tracers"
```

`Tracers` là symbol MODIFY (đã tồn tại, nhiều task khác cùng thêm entry vào — xem "Ghi chú điều phối liên-task" trong `00-index.md`) — chạy thêm

```
gitnexus_impact({ target: "Tracers", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, process bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Bối cảnh

CR-TRACE-002 định nghĩa 5 tracer domain mới cho luồng Agent Orchestration. Cả `agent-rpc-dispatch.ts` ([TASK-AG-002.2](./TASK-AG-002.2-agent-rpc-dispatch-resume-and-exec-span.md)) và `agent-spawner.ts` ([TASK-AG-002.3](./TASK-AG-002.3-agent-spawner-orchestration-spans.md)) đều import `Tracers` từ file này — thực thi task này trước 2 task kia.

## File: `src/shared/trace/tracers.ts` [MODIFY]

Thêm 5 entry mới vào object `Tracers` (giữ nguyên toàn bộ entry hiện có, kể cả `worktreeCreate`/`worktreeDelete` nếu TASK-AG-001.1 đã merge):

```typescript
// src/shared/trace/tracers.ts
import { createTracer } from './index'

export const Tracers = {
  // ...existing entries unchanged (browseDirFlow, mkdirFlow, rmdirFlow, agentWsFlow, ipcProxyFlow)...
  // ...worktreeCreate, worktreeDelete từ TASK-AG-001.1 nếu đã merge...

  // ─── CR-TRACE-002: Agent Orchestration ──────────────────────────────────────
  /** BL-AG-01 — spawn AI agent (agent.exec / agent.spawn) */
  agentOrchSpawn:      createTracer('agentOrch:spawn'),
  /** BL-AG-02 — stop agent (agent.kill / agent.sendInput Ctrl+C) */
  agentOrchStop:       createTracer('agentOrch:stop'),
  /** BL-AG-03 — resume agent session (agent.spawn với resumeId) */
  agentOrchResume:     createTracer('agentOrch:resume'),
  /** BL-AG-04 — switch account/provider (chưa có call site thật, đặt tên trước) */
  agentOrchSwitch:     createTracer('agentOrch:switch'),
  /** BL-AG-05 — polling loop rời rạc (KHÔNG dùng cho agent.output stream, xem CR-TRACE-002 §4) */
  agentOrchStatusPoll: createTracer('agentOrch:statusPoll'),
} as const
```

Lưu ý: `agentOrchSwitch` và `agentOrchStatusPoll` được đăng ký nhưng KHÔNG có call site `.start()` nào trong TASK-AG-002.2/002.3 (xem SOL-AG-TRACE-002 §1 — BL-AG-04 không có call site agent-side riêng, là chuỗi `agent.kill` + `agent.spawn`/`agent.exec` mới; BL-AG-05 dùng `step()` trên span đã mở, không phải tracer polling riêng). Đây là hành vi ĐÚNG theo solution — không xoá 2 entry này, chỉ chưa có caller.

## Verification

```bash
pnpm run typecheck:node 2>&1 | grep "tracers.ts" || echo "No errors"
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Definition of Done

- [ ] 5 entry `agentOrchSpawn/Stop/Resume/Switch/StatusPoll` thêm vào `Tracers` với tên tracer đúng `agentOrch:spawn|stop|resume|switch|statusPoll`
- [ ] Toàn bộ entry hiện có (kể cả từ TASK-AG-001.1 nếu đã merge) giữ nguyên, không xoá/đổi tên
- [ ] `as const` giữ nguyên ở cuối object
- [ ] `pnpm run typecheck:node` pass
