# TASK-BE-013.1: Thêm 2 tracer cho Agent WebSocket handshake vào `tracers.ts`

**Phase:** 2
**SOL Ref:** [SOL-BE-TRACE-013](../solutions/SOL-BE-TRACE-013-agent-ws.md) §2.1
**CR Ref:** [CR-TRACE-013](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-013-agent-ws.md)
**Prerequisite:** Phase 0 (TASK-BE-000)
**Status:** ✅ Done (2026-08-04) — added `agentWsHandshakeFlow`/`agentWsTokenVerifyFlow` to `Tracers`, additive only; typecheck:node clean of new errors (pre-existing unrelated failures in renderer/store, ssh/dev-server-provisioner.ts, rpc/methods/dev-server.ts, other __tests__ files confirmed via `git status` as out-of-scope/pre-existing).

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "Tracers"
```

`Tracers` là object đã tồn tại (MODIFY case) — chạy thêm:

```
gitnexus_impact({ target: "Tracers", direction: "upstream" })
```

Task này chỉ thêm 2 entry mới (`agentWsHandshakeFlow`/`agentWsTokenVerifyFlow`), không đổi entry cũ (đặc biệt `agentWs:lifecycle`, `agent:rpc`). Fan-in lớn của `Tracers` là bình thường; chỉ dừng lại nếu risk HIGH/CRITICAL đến từ nguyên nhân khác, xác nhận với người dùng trước khi tiếp tục.

## Mô tả

Khai báo 2 tracer mới trong `src/shared/trace/tracers.ts` cho pha handshake/auth của Agent WebSocket — trước và tách biệt với `agentWs:lifecycle` (đã tồn tại, không đổi). Không thêm `agentWs:tokenManage` (đề xuất ban đầu trong CR-TRACE-000 §4) vì trùng lặp với `agentToken:register`/`agent:tokenManager` đã tồn tại — vi phạm nguyên tắc "1 tracer = 1 sub-flow" theo hướng ngược.

## File: `src/shared/trace/tracers.ts` [MODIFY]

Thêm khối sau vào object `Tracers` đã tồn tại (giữ nguyên các tracer hiện có, chỉ append):

```typescript
// src/shared/trace/tracers.ts
import { createTracer } from './index'

export const Tracers = {
  // ...các tracer hiện có (browseDirFlow, mkdirFlow, rmdirFlow, agentWsFlow, ipcProxyFlow, ...) giữ nguyên...

  // ─── CR-TRACE-013: Agent WebSocket (handshake/auth phase) ─────────────────
  /** BL-AWS-01: Orca initiator handshake (relay-websocket mode) — TCP connect
   *  + agent.handshake round-trip, TRƯỚC khi agentWs:lifecycle bắt đầu. */
  agentWsHandshakeFlow:   createTracer('agentWs:handshake'),
  /** BL-AWS-02: Orca receiver handshake + token validation (direct-websocket
   *  mode) — từ lúc socket upgrade tới accept/reject, TRƯỚC agentWs:lifecycle. */
  agentWsTokenVerifyFlow: createTracer('agentWs:tokenVerify'),
} as const
```

**Ràng buộc bắt buộc:**
- Không đổi tên hay xoá bất kỳ tracer nào đã tồn tại, đặc biệt `agentWs:lifecycle`, `agent:rpc`, `agentToken:register`, `agent:tokenManager`.
- Không thêm `agentWs:tokenManage` hay bất kỳ tracer thứ 3 nào ngoài 2 tracer trên.

## Verification

```bash
pnpm run typecheck:node
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Acceptance Criteria

- [ ] `Tracers.agentWsHandshakeFlow` (`agentWs:handshake`) và `Tracers.agentWsTokenVerifyFlow` (`agentWs:tokenVerify`) được export từ `tracers.ts`
- [ ] Không tracer nào trong task này trùng tên flow với `agentWs:lifecycle`, `agent:rpc`, `agentToken:register`, `agent:tokenManager` đã tồn tại
- [ ] `pnpm run typecheck:node` pass, không lỗi mới
