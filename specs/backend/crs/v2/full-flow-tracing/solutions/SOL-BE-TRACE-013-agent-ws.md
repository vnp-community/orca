# SOL-BE-TRACE-013: Agent WebSocket — Backend-Side Tracing Implementation

**CR Ref:** [CR-TRACE-013](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-013-agent-ws.md)
**TDD Ref:** TDD-04 (WebSocket/Unix RPC Server — §Addendum F.4 "3 Connection Modes": `relay-ssh`, `relay-websocket`, `direct-websocket`)
**Date:** 2026-08-02
**Status:** Proposed
**Strategy:** Additive-only — 2 tracer mới, không duplicate tracer đã ship (`agentWs:lifecycle`, `relay:agentCall`, `agentToken:register`, `agent:tokenManager`)

---

## 1. Phân tích phạm vi (Backend-side only)

### 1.1 Vị trí trong 3 Connection Modes (TDD-04 Addendum F.4)

```
Mode 1: relay-ssh          — ssh2 exec channel (không có WS handshake — ngoài phạm vi CR này)
Mode 2: relay-websocket     — Backend (Orca) là WS CLIENT, chủ động connect ra Agent
                              file: src/main/dev-server/dev-server-relay-bridge.ts   ← BACKEND, trong phạm vi
Mode 3: direct-websocket    — Backend (Orca) là WS SERVER, Agent chủ động connect vào
                              file: src/main/dev-server/agent-ws-server.ts           ← BACKEND, trong phạm vi
```

Cả 2 file transport đều nằm trong `src/main/dev-server/` — thuộc Backend/Gateway process (Electron Main hoặc Node.js server mode), **không phải** code chạy trên Dev Server. `src/main/dev-server/ws-handshake.ts` (dùng chung bởi cả 2 mode, chứa `runOrcaInitiatorHandshake`/`runOrcaReceiverHandshake`) cũng là Backend-side — cả 2 hàm này chạy trong Main process, chỉ giao tiếp qua WebSocket với phía Agent.

### 1.2 Gap Analysis (verified qua Read trực tiếp code, không chỉ dựa vào CR)

| Sub-flow | File:function (verified) | Hiện trạng | Việc cần làm |
|----------|---------------------------|-----------|---------------|
| BL-AWS-01 (relay-websocket) | `src/main/dev-server/dev-server-relay-bridge.ts` — `connectRelayWebSocket()` (khai báo dòng ~380), `attempt()` closure bên trong, `ws.on('open', ...)` gọi `runOrcaInitiatorHandshake()` | Không có span nào bọc TCP connect + handshake; chỉ `relayCallTracer` (`relay:agentCall`, khai báo dòng 21) tồn tại cho các RPC call SAU KHI đã connect | Thêm `agentWsHandshakeFlow`, 1 span/`attempt()` (kể cả reconnect) |
| BL-AWS-02 (direct-websocket) | `src/main/dev-server/agent-ws-server.ts` — `handleConnection()`, gọi `runOrcaReceiverHandshake()` | `Tracers.agentWsFlow` (`agentWs:lifecycle`) chỉ được tạo SAU KHI handshake `.then()` resolve thành công; nhánh `.catch()` gọi `Tracers.agentWsFlow.start().fail(err, {phase:'handshake'})` — span `id` ngẫu nhiên, không liên kết được với attempt connect nào | Thêm `agentWsTokenVerifyFlow`, bắt đầu ngay khi `handleConnection()` được gọi (trước khi biết kết quả handshake) |
| BL-AWS-03 (Token Management) — `POST /api/agent-token` | `src/server/agent-token-routes.ts` — verified: `tokenTracer = createTracer('agentToken:register')` dòng 27, dùng trong nhánh POST | **Đã có tracer** | Không sửa |
| BL-AWS-03 — `GET /api/agent-token` (list, debug) | `agent-token-routes.ts`, nhánh `if (method === 'GET')` verified dòng ~92-103 | Không có tracer nào bọc nhánh GET | Thêm 1 `tokenTracer.start({op:'list'}).ok({count})` — TÁI DÙNG tracer đã có, không tạo tracer mới |
| BL-AWS-03 — Agent token renewal | `src/relay/agent-token-manager.ts:24` — `createTracer('agent:tokenManager')` | **Đã có tracer, nhưng file này thuộc `src/relay/` (AGENT-side)** | KHÔNG sửa — ngoài phạm vi Backend, thuộc companion solution phía Agent |
| BL-AWS-03 — Admin token revoke/CRUD | Flow doc mô tả `DELETE /admin/api/agent-tokens/:id` + bảng `orca_agent_tokens` | Verified: **không tồn tại trong code hiện tại** — không có route, không có bảng SQL nào tên này trong `src/main/db/migrations/` | Không implement — feature gốc chưa tồn tại |

### 1.3 Ngoài phạm vi solution này (Agent-side hoặc chưa tồn tại)

- `src/relay/agent-token-manager.ts` (`agent:tokenManager`, đã có tracer) — AGENT side, companion solution xử lý nếu cần bổ sung
- `src/relay/agent-rpc-dispatch.ts` (`agent:rpc`, dòng 21/128) — generic JSON-RPC dispatch SAU KHI handshake xong; CR-TRACE-013 mục 5.2 xác nhận việc đọc `params._trace.id` phụ thuộc core API `resume` (CR-TRACE-000 mục 3) ship trước — **không sửa trong solution này**, chỉ ghi nhận dependency
- Admin token revoke/CRUD (`orca_agent_tokens` SQL, `requireAdmin()` guard) — feature chưa tồn tại, không trace path ảo
- HMAC-SHA256 Signer / `RpcExecutionContext` (30s TTL) mô tả trong flow doc — verified: **không tìm thấy implementation khớp** trong `src/main/dev-server/*` hay `src/relay/*`; nếu tồn tại dưới tên khác (vd. `SignedExecutionContext`, CR-DS-005) thì nằm ngoài phạm vi CR-TRACE-013

---

## 2. Full Implementation

### 2.1 `src/shared/trace/tracers.ts` — thêm 2 tracer

```typescript
// src/shared/trace/tracers.ts
import { createTracer } from './index'

export const Tracers = {
  browseDirFlow: createTracer('devServer:browseDir'),
  mkdirFlow:     createTracer('devServer:mkdir'),
  rmdirFlow:     createTracer('devServer:rmdir'),
  agentWsFlow:   createTracer('agentWs:lifecycle'),
  ipcProxyFlow:  createTracer('ipc:devServerProxy'),

  // ─── CR-TRACE-013: Agent WebSocket (handshake/auth phase) ─────────────────
  /** BL-AWS-01: Orca initiator handshake (relay-websocket mode) — TCP connect
   *  + agent.handshake round-trip, TRƯỚC khi agentWs:lifecycle bắt đầu. */
  agentWsHandshakeFlow:   createTracer('agentWs:handshake'),
  /** BL-AWS-02: Orca receiver handshake + token validation (direct-websocket
   *  mode) — từ lúc socket upgrade tới accept/reject, TRƯỚC agentWs:lifecycle. */
  agentWsTokenVerifyFlow: createTracer('agentWs:tokenVerify'),
} as const
```

Không thêm `agentWs:tokenManage` (đề xuất ban đầu trong CR-TRACE-000 §4) — trùng lặp với `agentToken:register` + `agent:tokenManager` đã tồn tại, vi phạm nguyên tắc "1 tracer = 1 sub-flow" theo hướng ngược (CR-TRACE-013 mục 3 đã phân tích).

### 2.2 `src/main/dev-server/dev-server-relay-bridge.ts` — BL-AWS-01

```typescript
// src/main/dev-server/dev-server-relay-bridge.ts
import { Tracers } from '../../shared/trace/tracers'
// relayCallTracer (relay:agentCall) đã import sẵn — giữ nguyên, không đổi

private connectRelayWebSocket(
  rawUrl: string,
  opts: { testOnly?: boolean }
): Promise<RelayHandshakeInfo> {
  const url = new URL(rawUrl)
  const token = url.searchParams.get('token') ?? ''
  url.searchParams.delete('token')
  const cleanUrl = url.toString()
  const orcaVersion = getPlatform().app.getVersion()
  this._relayWsActive = !opts.testOnly

  return new Promise<RelayHandshakeInfo>((resolve, reject) => {
    let initialResolved = false

    const attempt = () => {
      if (!this._relayWsActive) return

      // [NEW] mỗi lần attempt() (kể cả reconnect qua setTimeout(attempt, delayMs))
      // là 1 span mới — điểm rẽ nhánh quan trọng theo CR-TRACE-000 mục 5 rule 3.
      const span = Tracers.agentWsHandshakeFlow.start({ devServerId: this.config.id })

      const ws = new WebSocket(cleanUrl, {
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      })
      ;(ws as unknown as { binaryType: string }).binaryType = 'nodebuffer'

      const connectionTimeout = setTimeout(() => {
        ws.terminate()
        const timeoutMsg =
          `relay-websocket: TCP connection timed out after 10s to ${cleanUrl}. ` +
          `Verify the agent is running and the address is reachable.`
        span.fail(timeoutMsg, { phase: 'tcpConnect', devServerId: this.config.id })
        if (!initialResolved) {
          reject(new Error(timeoutMsg))
        } else {
          console.warn(`[RelayBridge] ${timeoutMsg} Retry in 15s.`)
        }
      }, 10_000)

      ws.on('error', (err: Error) => {
        clearTimeout(connectionTimeout)
        span.fail(err, { phase: 'tcpConnect', devServerId: this.config.id })
        if (!initialResolved) {
          reject(new Error(
            `relay-websocket: WebSocket error connecting to ${cleanUrl}: ${err.message}`
          ))
        } else {
          console.warn(`[RelayBridge] relay-ws error: ${err.message}`)
        }
      })

      ws.on('open', () => {
        clearTimeout(connectionTimeout)
        span.step('tcpConnected', { devServerId: this.config.id })

        runOrcaInitiatorHandshake(ws, orcaVersion)
          .then((info) => {
            span.ok({ platform: info.platform, nodeVersion: info.nodeVersion, agentVersion: info.agentVersion })

            const transport = createWebSocketTransport(ws)
            this.session = new SshChannelMultiplexer(transport, {
              connectionLostMessage: 'Connection lost, reconnecting...'
            })

            if (opts.testOnly) {
              void this.disconnect()
            } else {
              ws.on('close', () => {
                if (this.session) {
                  console.log('[RelayBridge] relay-ws disconnected — clearing session')
                  this.session = null
                  this.onSessionDropped()
                }
                if (this._relayWsActive) {
                  const delayMs = calcBackoffDelay(this._relayWsReconnectAttempt++)
                  console.log(`[RelayBridge] relay-ws will reconnect in ${Math.round(delayMs / 1000)}s (attempt ${this._relayWsReconnectAttempt})...`)
                  // attempt() gọi lại → tạo span agentWsHandshakeFlow MỚI cho lần thử này
                  this._relayWsReconnectTimer = setTimeout(attempt, delayMs)
                }
              })
            }

            if (!initialResolved) {
              initialResolved = true
              resolve({
                platform: (info.platform as NodeJS.Platform) ?? 'linux',
                arch: info.arch,
                nodeVersion: info.nodeVersion,
                relayVersion: info.agentVersion,
              })
            } else {
              this._relayWsReconnectAttempt = 0
            }
          })
          .catch((err: Error) => {
            span.fail(err, { phase: 'handshake', devServerId: this.config.id })
            // ...existing retry/reject logic không đổi...
          })
      })
    }

    attempt()
  })
}
```

Lưu ý quan trọng: `connectionTimeout`/`ws.on('error')` đều có thể fire SAU `ws.on('open')` đã set `span` mới (do reconnect) — code trên dùng `span` trong closure của `attempt()`, mỗi lần gọi lại `attempt()` tạo `span` mới hoàn toàn độc lập nên không có race giữa các lần thử.

### 2.3 `src/main/dev-server/agent-ws-server.ts` — BL-AWS-02

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

### 2.4 `src/server/agent-token-routes.ts` — BL-AWS-03, nhánh GET

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

### 2.5 Lan truyền `traceId` — hiện trạng và giới hạn

Theo CR-TRACE-013 mục 5.1, mô hình lý tưởng là `devServer.connect` RPC (Browser→Backend) → `DevServerManager.connect()` → `connectRelayWebSocket()` cùng chia sẻ 1 `id` xuyên suốt 3 lớp, dùng `resume: { id }`. Việc này phụ thuộc `Tracer.start(fields?, resume?)` (CR-TRACE-000 §3.1) — **API core này chưa ship** tại thời điểm viết solution (verified: `src/shared/trace/index.ts:46-48` hiện tại `Tracer.start(fields?: TraceFields): TraceSpan`, không có tham số `resume`). Solution này KHÔNG tự thêm `resume` param vào core API (thuộc phạm vi CR-TRACE-000, một solution riêng) — code ở mục 2.2/2.3 viết `Tracers.xxxFlow.start({...})` không có `resume`, span `id` độc lập cho tới khi core API đó ship. Khi CR-TRACE-000 core API solution merge trước, patch bổ sung 1 dòng: forward `resume: { id: parentSpanId }` vào các lời gọi `start()` ở mục 2.2/2.3.

---

## 3. Test Plan (Vitest)

### 3.1 File test mới

```
src/main/dev-server/__tests__/dev-server-relay-bridge-tracing.test.ts
src/main/dev-server/__tests__/agent-ws-server-tracing.test.ts
src/server/__tests__/agent-token-routes-tracing.test.ts
```

### 3.2 Test cases

**`dev-server-relay-bridge-tracing.test.ts`**
- `connectRelayWebSocket() success`: mock WebSocket `open` → handshake resolve → assert `agentWsHandshakeFlow` nhận 1 `start()`, 1 `step('tcpConnected')`, 1 `ok()` với `platform`/`nodeVersion`
- `connectRelayWebSocket() — TCP timeout`: không bao giờ fire `open` trong 10s (dùng fake timers) → assert `span.fail()` với `phase:'tcpConnect'`
- `connectRelayWebSocket() — WS error trước open`: fire `ws.on('error')` → assert `fail()` với `phase:'tcpConnect'`
- `connectRelayWebSocket() — handshake reject`: `open` fires nhưng `runOrcaInitiatorHandshake` reject → assert `fail()` với `phase:'handshake'`
- `connectRelayWebSocket() — reconnect tạo span mới`: simulate `close` event sau khi đã connect thành công 1 lần → assert `attempt()` gọi lại tạo **span thứ 2** với `id` khác span đầu (không tái dùng `id` cũ)

**`agent-ws-server-tracing.test.ts`**
- `handleConnection() — span bắt đầu trước khi biết kết quả handshake`: assert `agentWsTokenVerifyFlow.start()` được gọi ngay khi `handleConnection()` invoke, TRƯỚC khi `runOrcaReceiverHandshake` resolve/reject
- `handleConnection() — token hợp lệ, slot tồn tại`: assert `step('tokenLookup', {tokenPrefix})` rồi `ok({devServerId, sessionId})`; verify `tokenPrefix` KHÔNG chứa token đầy đủ (length assertion: `tokenPrefix.length <= 15` — 12 ký tự + `...`)
- `handleConnection() — slot expired (race)`: mock handshake resolve nhưng slot đã bị xoá → assert `fail('slot-expired', {devServerId})`
- `handleConnection() — invalid token`: mock handshake reject → assert `fail(err, {reason:'invalid-token'})` dùng ĐÚNG span đã mở từ đầu (so sánh `span.id` giữa lần `start()` và lần `fail()` — phải là cùng 1 span, không phải span ngẫu nhiên mới)
- `handleConnection() — agentWsFlow (lifecycle) không bị ảnh hưởng`: assert `Tracers.agentWsFlow.start()` vẫn được gọi đúng như hành vi cũ (connected → disconnect), độc lập `id` với `agentWsTokenVerifyFlow`

**`agent-token-routes-tracing.test.ts`**
- `GET /api/agent-token — có ít nhất 1 trace event`: mock `pendingMeta` có 2 entries → assert `tokenTracer.start({op:'list'}).ok({count:2})` được gọi
- `GET /api/agent-token — không tạo tracer mới`: assert chỉ `agentToken:register` xuất hiện trong danh sách flow name của mọi event phát sinh từ route này (không có flow name mới nào)

### 3.3 Test Targets

| Test file | Target số test |
|-----------|---------------|
| `dev-server-relay-bridge-tracing.test.ts` | ≥ 5 |
| `agent-ws-server-tracing.test.ts` | ≥ 5 |
| `agent-token-routes-tracing.test.ts` | ≥ 2 |
| **Total** | **≥ 12** |

---

## 4. Acceptance Criteria

- [ ] `Tracers.agentWsHandshakeFlow` (`agentWs:handshake`) bọc đúng `connectRelayWebSocket()` — mỗi lần `attempt()` (kể cả reconnect) là 1 span mới, không tái dùng `id` cũ
- [ ] `Tracers.agentWsTokenVerifyFlow` (`agentWs:tokenVerify`) bắt đầu ngay khi `handleConnection()` được gọi (socket upgrade xong), không đợi handshake resolve — khác hành vi hiện tại
- [ ] `agentWs:tokenVerify` không log token đầy đủ trong bất kỳ field nào — chỉ `tokenPrefix` (12 ký tự đầu + `...`)
- [ ] `Tracers.agentWsFlow` (`agentWs:lifecycle`, connect/disconnect) giữ nguyên hành vi, không bị merge/duplicate với `agentWs:tokenVerify` — 2 span độc lập với `id` khác nhau cho cùng 1 connection
- [ ] Nhánh `GET /api/agent-token` trong `agent-token-routes.ts` có ít nhất 1 trace event, tái dùng `tokenTracer` (`agentToken:register`) sẵn có — không tạo tracer mới
- [ ] Không tracer nào trong solution này trùng tên với `agentWs:lifecycle`, `agent:rpc`, `agentToken:register`, `agent:tokenManager` đã tồn tại
- [ ] `src/relay/agent-token-manager.ts` (`agent:tokenManager`) và `src/relay/agent-rpc-dispatch.ts` (`agent:rpc`) KHÔNG bị sửa bởi solution này — xác nhận qua diff review, thuộc phạm vi solution phía Agent
- [ ] Admin token revoke/CRUD và HMAC-SHA256 Signer/`RpcExecutionContext` — xác nhận lại (trước khi bắt đầu implement bất kỳ CR follow-up nào) rằng 2 tính năng này thực sự chưa tồn tại trong code, tránh lặp lại việc "chưa xác định" của CR-TRACE-013
