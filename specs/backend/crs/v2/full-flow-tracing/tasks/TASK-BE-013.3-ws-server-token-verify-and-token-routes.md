# TASK-BE-013.3: Instrument `handleConnection()` (BL-AWS-02) — sửa bug span mồ côi + wire nhánh GET token routes (BL-AWS-03)

**Phase:** 2
**SOL Ref:** [SOL-BE-TRACE-013](../solutions/SOL-BE-TRACE-013-agent-ws.md) §2.3, §2.4
**CR Ref:** [CR-TRACE-013](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-013-agent-ws.md)
**Prerequisite:** Phase 0 (TASK-BE-000) + TASK-BE-013.1
**Status:** ✅ Done (2026-08-04) — fixed orphaned-span bug: `agentWsTokenVerifyFlow` span now opens at the top of `handleConnection()` (socket-upgrade time), reused (not recreated) in the `.catch()` branch on handshake reject (`fail(err, {reason:'invalid-token'})`), `step('tokenLookup')`/`ok()`/`fail('slot-expired')` wired; `agentWsFlow` (lifecycle) kept as an independent span, renamed local var to `lifecycleSpan` to avoid shadowing, no behavior change. Wired `GET /api/agent-token` to reuse existing `tokenTracer` (`op:'list'`), no new tracer created. `src/relay/agent-token-manager.ts` and `agent-rpc-dispatch.ts` not touched by this task (agent-rpc-dispatch.ts shows modified in git status from a concurrent unrelated background agent). typecheck:node clean for all 3 edited files.

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "AgentWsServer.handleConnection"
```

Symbol đã tồn tại (MODIFY case) — task này bundle 1 bugfix hành vi thật (span mồ côi) cùng với tracing, nên đặc biệt quan trọng phải hiểu rõ code hiện tại trước khi sửa. Chạy:

```
gitnexus_impact({ target: "AgentWsServer.handleConnection", direction: "upstream" })
```

Báo cáo blast radius trước khi sửa. Với nhánh `GET /api/agent-token` (`src/server/agent-token-routes.ts`), cũng chạy `codegraph explore "agent-token-routes GET handler"` trước khi thêm dòng tracer tái dùng `tokenTracer`. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Mô tả

Task này bundle 1 bugfix hành vi thật với việc thêm tracing (đúng theo phát hiện của SOL-BE-TRACE-013): **hiện tại**, khi `handleConnection()` trong `src/main/dev-server/agent-ws-server.ts` (Backend là WS SERVER, Agent chủ động connect vào) nhận một handshake thất bại, nhánh `.catch()` gọi `Tracers.agentWsFlow.start().fail(err, {phase:'handshake'})` — tức là **tạo một span MỚI ngay tại thời điểm fail**, với `id` ngẫu nhiên hoàn toàn không liên kết được với bất kỳ attempt connect nào trước đó (span "mồ côi"). Task này sửa bug đó bằng cách mở span `agentWsTokenVerifyFlow` **ngay khi `handleConnection()` được gọi** (tức ngay lúc socket-upgrade thành công), TRƯỚC khi biết kết quả handshake — nhánh `.catch()` sau đó dùng lại đúng `span` đã mở từ đầu thay vì tạo span mới, nên giờ liên kết được "socket X connect lúc nào → fail ở bước nào". `agentWsFlow` (`agentWs:lifecycle`, connect→disconnect) là span độc lập khác, giữ nguyên hành vi, không đổi.

Ngoài ra, task này wire thêm 1 dòng tracer cho nhánh `GET /api/agent-token` (list, debug) trong `src/server/agent-token-routes.ts` — tái dùng `tokenTracer` (`agentToken:register`) đã tồn tại, KHÔNG tạo tracer mới.

## File: `src/main/dev-server/agent-ws-server.ts` [MODIFY]

```typescript
// src/main/dev-server/agent-ws-server.ts
import { Tracers } from '../../shared/trace/tracers'
// Tracers.agentWsFlow (agentWs:lifecycle) đã import sẵn — giữ nguyên logic, không đổi

private handleConnection(ws: WebSocket): void {
  // [NEW] span bắt đầu NGAY khi socket upgrade thành công, trước khi biết
  // kết quả handshake — khác hành vi hiện tại (agentWsFlow chỉ tạo sau khi
  // handshake .then() resolve, hoặc span ngẫu nhiên trong .catch()).
  const span = Tracers.agentWsTokenVerifyFlow.start()

  runOrcaReceiverHandshake(
    ws,
    (token) => {
      // KHÔNG log token đầy đủ — chỉ 12 ký tự đầu, theo tiền lệ dòng log hiện có.
      span.step('tokenLookup', { tokenPrefix: token.slice(0, 12) + '...' })
      return this.pendingSlots.has(hashToken(token))
    },
    this.orcaVersion
  )
    .then((info) => {
      const agentToken = info.agentToken ?? ''
      const tokenHash  = hashToken(agentToken)
      const slot = this.pendingSlots.get(tokenHash)

      if (!slot) {
        span.fail('slot-expired', { devServerId: info.devServerId ?? 'unknown' })
        ws.close(1008, 'Slot expired — agent token is no longer registered')
        return
      }

      this.removeSlotByHash(tokenHash)
      span.ok({ devServerId: info.devServerId ?? 'unknown', sessionId: info.sessionId })

      // ── agentWs:lifecycle (KHÔNG ĐỔI — vẫn tạo span riêng cho connect→disconnect) ──
      const lifecycleSpan = Tracers.agentWsFlow.start({
        devServerId: info.devServerId ?? 'unknown',
        platform: info.platform ?? 'unknown',
        node: info.nodeVersion ?? 'unknown'
      })

      const transport = createWebSocketTransport(ws)
      const mux = new SshChannelMultiplexer(transport, {
        connectionLostMessage: 'Connection lost, reconnecting...'
      })

      ws.once('close', (code, reason) => {
        const reasonStr = reason?.toString() || '(none)'
        lifecycleSpan.step('disconnect', { code, reason: reasonStr })
      })

      lifecycleSpan.step('connected', { token: agentToken.slice(0, 12) + '...' })
      slot.callback(mux, info)
    })
    .catch((err: Error) => {
      // [CHANGED] dùng lại `span` đã mở từ đầu handleConnection() thay vì tạo
      // span ngẫu nhiên mới — nay liên kết được "socket X connect lúc nào → fail ở bước nào".
      span.fail(err, { reason: 'invalid-token' })
      console.warn('[AgentWsServer] Handshake rejected:', err.message)
    })
}
```

`agentWsTokenVerifyFlow` và `agentWsFlow` (lifecycle) là 2 span độc lập theo đúng ranh giới 2 mối quan tâm khác nhau (auth vs. lifecycle) — `agentWsTokenVerifyFlow` kết thúc bằng `ok()` ngay khi handshake accept, `agentWsFlow` bắt đầu sau đó và sống tới lúc `disconnect`.

## File: `src/server/agent-token-routes.ts` [MODIFY]

```typescript
// src/server/agent-token-routes.ts
// tokenTracer (agentToken:register) đã tồn tại — TÁI DÙNG, không tạo tracer mới

if (method === 'GET') {
  const now = Date.now()
  const tokens = Array.from(pendingMeta.entries())
    .filter(([, meta]) => meta.expiresAt > now)
    .map(([token, meta]) => ({
      token,
      devServerId: meta.devServerId,
      expiresIn: Math.round((meta.expiresAt - now) / 1000),
    }))
  // [NEW] nhánh GET trước đây không có tracer nào — tái dùng tokenTracer,
  // op:'list' để phân biệt với op ngầm định của nhánh POST (register).
  tokenTracer.start({ op: 'list' }).ok({ count: tokens.length })
  sendJson(res, 200, { tokens })
  return
}
```

**Ràng buộc bắt buộc:**
- Không sửa `runOrcaReceiverHandshake()`, `hashToken()`, `pendingSlots`, `removeSlotByHash()` — chỉ thêm/di chuyển tracer calls.
- `agentWsTokenVerifyFlow` không log token đầy đủ trong bất kỳ field nào — chỉ `tokenPrefix` (12 ký tự đầu + `...`).
- Nhánh `GET` trong `agent-token-routes.ts` KHÔNG tạo tracer mới — bắt buộc dùng `tokenTracer` đã tồn tại (`agentToken:register`).
- `src/relay/agent-token-manager.ts` (`agent:tokenManager`) và `src/relay/agent-rpc-dispatch.ts` (`agent:rpc`) KHÔNG được sửa trong task này — thuộc phạm vi solution phía Agent.

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

- [ ] `Tracers.agentWsTokenVerifyFlow` bắt đầu ngay khi `handleConnection()` được gọi (socket upgrade xong), không đợi handshake resolve — khác hành vi hiện tại (bugfix)
- [ ] Nhánh `.catch()` của `runOrcaReceiverHandshake()` dùng lại đúng `span` đã mở từ đầu `handleConnection()` — KHÔNG tạo span mới trong `.catch()` (xác nhận bug span mồ côi đã được sửa: `span.id` giống nhau giữa lúc `start()` và lúc `fail()`)
- [ ] `agentWs:tokenVerify` không log token đầy đủ trong bất kỳ field nào — chỉ `tokenPrefix` (12 ký tự đầu + `...`)
- [ ] `Tracers.agentWsFlow` (`agentWs:lifecycle`, connect/disconnect) giữ nguyên hành vi, không bị merge/duplicate với `agentWs:tokenVerify` — 2 span độc lập với `id` khác nhau cho cùng 1 connection
- [ ] Nhánh `GET /api/agent-token` trong `agent-token-routes.ts` có ít nhất 1 trace event, tái dùng `tokenTracer` (`agentToken:register`) sẵn có — không tạo tracer mới
- [ ] `src/relay/agent-token-manager.ts` và `src/relay/agent-rpc-dispatch.ts` KHÔNG bị sửa bởi task này
- [ ] `pnpm run typecheck:node` pass, không lỗi mới
