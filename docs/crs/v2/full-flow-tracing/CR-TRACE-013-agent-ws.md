# CR-TRACE-013 — Agent WebSocket Flow Tracing

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-TRACE-013 |
| **Tên** | Agent WebSocket Protocol — Full-Flow Tracing Instrumentation |
| **Loại** | Observability |
| **Priority** | P2 |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-08-01 |
| **Trạng thái** | Proposed |
| **Phụ thuộc** | CR-TRACE-000 |
| **Tác động** | `docs/flows/logic/agent-ws.md`, `src/main/dev-server/ws-handshake.ts`, `src/main/dev-server/agent-ws-server.ts`, `src/main/dev-server/dev-server-relay-bridge.ts`, `src/main/dev-server/dev-server-manager.ts`, `src/server/agent-token-routes.ts`, `src/relay/agent-token-manager.ts`, `src/relay/agent-rpc-dispatch.ts` |

---

## 1. Vấn đề

BL-AWS-01→03 có **2 tracer nội bộ đã tồn tại** — `Tracers.agentWsFlow` (`agentWs:lifecycle`, connect/disconnect của kết nối agent tại `src/main/dev-server/agent-ws-server.ts:145`) và `rpcTracer` (`agent:rpc`, generic JSON-RPC dispatch tại `src/relay/agent-rpc-dispatch.ts:21,128`) — nhưng **cả hai đều KHÔNG bao phủ giai đoạn handshake/xác thực token**, chỉ bao phủ "đã kết nối xong" (`agentWs:lifecycle`) và "đang dispatch 1 RPC method cụ thể sau khi đã kết nối" (`agent:rpc`). Cụ thể:

- **BL-AWS-01 (relay-websocket, Orca chủ động kết nối ra Agent)**: `connectRelayWebSocket()` (`src/main/dev-server/dev-server-relay-bridge.ts:380`) thực hiện TCP connect (10s timeout) rồi `runOrcaInitiatorHandshake()` (`src/main/dev-server/ws-handshake.ts:49`, 20s timeout) — **không có span nào** bọc 2 bước này. Khi user báo "Dev Server mãi không connect", không thể biết đang kẹt ở TCP connect, ở handshake, hay agent từ chối handshake.
- **BL-AWS-02 (direct-websocket, Agent tự kết nối vào Orca)**: `AgentWebSocketServer.handleConnection()` (`src/main/dev-server/agent-ws-server.ts:121`) gọi `runOrcaReceiverHandshake()` (`ws-handshake.ts:127`) để verify token — span `Tracers.agentWsFlow` chỉ được tạo **SAU KHI** handshake đã resolve thành công (dòng 145), và khi handshake fail, code tạo một span mới ngẫu nhiên chỉ để gọi `.fail()` ngay lập tức (dòng 171: `Tracers.agentWsFlow.start().fail(err, ...)`) — span này có `id` ngẫu nhiên không liên kết được với lần connect nào, nên KHÔNG thể trace được "socket X connect lúc nào → fail ở bước nào" nếu có nhiều agent connect đồng thời.
- **BL-AWS-03 (Token Management)**: flow doc mô tả một hệ Admin SPA CRUD đầy đủ (generate/revoke/list token qua bảng SQL `orca_agent_tokens`, có `requireAdmin()` guard) — **implementation thực tế không khớp**: chỉ có 1 endpoint `POST/GET /api/agent-token` (`src/server/agent-token-routes.ts`) dùng in-memory `Map` (`pendingMeta`), auth bằng `ORCA_AGENT_API_SECRET` Bearer token (không phải `requireAdmin()`), **không có revoke endpoint, không có bảng SQL, không có admin SPA CRUD**. Route này đã có tracer `agentToken:register` (dòng 29) bọc nhánh `POST`, nhưng nhánh `GET` (list pending, dòng 92-103) hoàn toàn chưa được trace.

Không có instrumentation cho các khoảng trống trên nghĩa là khi kết nối Dev Server Agent chậm/fail, engineer phải đọc log rời rạc (`console.warn`/`console.error` không có `id` chung) thay vì 1 trace xuyên suốt.

## 2. Thành phần & Transport liên quan

| Thành phần (flow doc) | Thành phần thực tế trong code | Layer | Transport | CR-TRACE-000 §3.3 row áp dụng |
|------------------------|-------------------------------|-------|-----------|-------------------------------|
| AgentConnectionManager | Không tồn tại tên lớp này — vai trò tương đương do `DevServerManager` (`src/main/dev-server/dev-server-manager.ts`) + `DevServerRelayBridge` (`dev-server-relay-bridge.ts`) đảm nhiệm | Backend | — | — |
| Orca Web Server (`ws://orca:6768/agent`) | `AgentWebSocketServer` (`src/main/dev-server/agent-ws-server.ts`), mount tại `AGENT_WS_PATH` (`/agent`) qua `attach(httpServer)` | Backend | WebSocket (binary JSON-RPC 2.0 frame) | Agent WS JSON-RPC 2.0 — `params._trace.id` |
| HMAC-SHA256 Signer / RpcExecutionContext | Không tìm thấy implementation khớp mô tả (HMAC ký `RpcExecutionContext` 30s TTL) trong `src/main/dev-server/*` hay `src/relay/*` — **chưa xác định file cụ thể, cần điều tra thêm khi triển khai** (có thể liên quan CR-DS-005 `SignedExecutionContext`, không nằm trong phạm vi CR này) | Security | — | — |
| SSH Relay (relay-websocket) | `DevServerRelayBridge.connectRelayWebSocket()` (`dev-server-relay-bridge.ts:380`), dùng `runOrcaInitiatorHandshake()` (`ws-handshake.ts:49`) | Backend | WebSocket (Bearer token trong header) | Agent WS JSON-RPC 2.0 |
| Server Database `orca_agent_tokens` | Không tồn tại — token issuance thực tế dùng in-memory `Map` (`pendingMeta`, `src/server/agent-token-routes.ts:33`), không có bảng SQL | Persistence | — | — |
| (bổ sung) Token renewal loop phía Agent | `AgentTokenManager` (`src/relay/agent-token-manager.ts`), gọi `POST /api/agent-token` qua HTTP | Remote (Dev Server) | HTTP | HTTP/WS `:6768` row (agent đóng vai trò client tương tự CLI) |
| (bổ sung) Generic RPC dispatch sau handshake | `rpcTracer` (`agent:rpc`, `src/relay/agent-rpc-dispatch.ts:21`) | Backend/Remote | Agent WS JSON-RPC 2.0 | Đã tồn tại — không sửa trong CR này |

## 3. Tracer mới cần thêm vào `tracers.ts`

```typescript
export const Tracers = {
  // ...existing entries unchanged...
  agentWsHandshakeFlow:   createTracer('agentWs:handshake'),   // BL-AWS-01: Orca initiator handshake (relay-websocket)
  agentWsTokenVerifyFlow: createTracer('agentWs:tokenVerify'), // BL-AWS-02: Orca receiver handshake + token validation (direct-websocket)
}
```

**BL-AWS-03 không cần tracer mới trong `tracers.ts`.** Lý do: phần "generate token" đã có `agentToken:register` (`createTracer('agentToken:register')`, ad-hoc trong `src/server/agent-token-routes.ts:29`, cùng pattern ad-hoc như `relay:agentCall`/`agent:rpc` mà CR-TRACE-000 GAP-3 đã ghi nhận) và phần "renew token" đã có `agent:tokenManager` (`src/relay/agent-token-manager.ts:24`). Việc "revoke"/"admin CRUD" mà flow doc mô tả **chưa tồn tại trong code** — khi tính năng đó được xây, thêm tracer lúc đó (đề xuất tên `agentWs:tokenRevoke` để nhất quán prefix, xem mục 4.3).

> Lưu ý đặt tên: `agentToken:register` dùng prefix `agentToken:` thay vì `agentWs:` — không nhất quán với convention CR-TRACE-000 §4, nhưng đây là tracer **đã ship**; đổi tên sẽ phá vỡ dashboard/log parser hiện có, nên CR này giữ nguyên tên và chỉ tham chiếu, không rename (không dùng `gitnexus_rename` vì đây là docs-only CR, không sửa code).

## 4. Instrumentation theo từng sub-flow

### BL-AWS-01 — relay-websocket Mode (Orca → Agent)

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Bắt đầu (mỗi lần `attempt()` — kể cả reconnect) | `start` | `devServerId`, `attempt` (số lần thử) | `src/main/dev-server/dev-server-relay-bridge.ts:399` (`attempt()` closure trong `connectRelayWebSocket()`) |
| TCP connect xong, gửi handshake | `step('tcpConnected')` | `devServerId` | `dev-server-relay-bridge.ts:431` (`ws.on('open', ...)`) |
| Handshake resolve | `ok` | `platform`, `nodeVersion`, `agentVersion` | `dev-server-relay-bridge.ts:434-467` (`runOrcaInitiatorHandshake().then(...)`) |
| TCP timeout / WS error / handshake reject | `fail` | `phase: 'tcpConnect'\|'handshake'` | `dev-server-relay-bridge.ts:407-429` (timeout/error) và `:481-493` (`.catch(...)`) |

```typescript
// src/main/dev-server/dev-server-relay-bridge.ts — connectRelayWebSocket(), trong attempt()
const attempt = () => {
  if (!this._relayWsActive) return
  const span = Tracers.agentWsHandshakeFlow.start({ devServerId: this.config.id })
  const ws = new WebSocket(cleanUrl, { headers: token ? { Authorization: `Bearer ${token}` } : {} })
  // ...
  ws.on('open', () => {
    span.step('tcpConnected', { devServerId: this.config.id })
    runOrcaInitiatorHandshake(ws, orcaVersion)
      .then((info) => {
        span.ok({ platform: info.platform, nodeVersion: info.nodeVersion })
        // ...existing logic
      })
      .catch((err: Error) => {
        span.fail(err, { phase: 'handshake', devServerId: this.config.id })
        // ...existing retry logic
      })
  })
}
```

Vì `attempt()` chạy lại mỗi lần reconnect (`setTimeout(attempt, delayMs)`), mỗi lần thử là **1 span mới** — đây là điểm rẽ nhánh quan trọng (reconnect loop) nên xứng đáng có `start` riêng theo mục 5 CR-TRACE-000 (rule 3: "điểm rẽ nhánh quan trọng cho troubleshoot").

### BL-AWS-02 — direct-websocket Mode (Agent → Orca)

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| Socket vừa upgrade thành công | `start` | (chưa có `devServerId`/token ở bước này — để trống) | `src/main/dev-server/agent-ws-server.ts:121` (`handleConnection()`) |
| Token hash lookup | `step('tokenLookup')` | `tokenPrefix` (12 ký tự đầu — theo tiền lệ dòng 166, KHÔNG log token đầy đủ) | `agent-ws-server.ts:128` (validator callback) + `ws-handshake.ts:172-191` |
| Handshake OK, slot tìm thấy | gộp vào `ok()` | `sessionId`, `devServerId` | `agent-ws-server.ts:131-167` |
| Token invalid | `fail` | `reason: 'invalid-token'` | `ws-handshake.ts:173-190` |
| Slot expired (race) | `fail` | `reason: 'slot-expired'` | `agent-ws-server.ts:136-140` |

```typescript
// src/main/dev-server/agent-ws-server.ts — handleConnection()
private handleConnection(ws: WebSocket): void {
  const span = Tracers.agentWsTokenVerifyFlow.start()
  runOrcaReceiverHandshake(
    ws,
    (token) => {
      span.step('tokenLookup', { tokenPrefix: token.slice(0, 12) + '...' })
      return this.pendingSlots.has(hashToken(token))
    },
    this.orcaVersion
  )
    .then((info) => {
      const tokenHash = hashToken(info.agentToken ?? '')
      const slot = this.pendingSlots.get(tokenHash)
      if (!slot) {
        span.fail('slot-expired', { devServerId: info.devServerId ?? 'unknown' })
        ws.close(1008, 'Slot expired — agent token is no longer registered')
        return
      }
      this.removeSlotByHash(tokenHash)
      span.ok({ devServerId: info.devServerId ?? 'unknown', sessionId: info.sessionId })
      // ...existing Tracers.agentWsFlow (lifecycle) logic unchanged below
    })
    .catch((err: Error) => {
      span.fail(err, { reason: 'invalid-token' })
      console.warn('[AgentWsServer] Handshake rejected:', err.message)
    })
}
```

Lưu ý: `Tracers.agentWsFlow` (lifecycle) **giữ nguyên không đổi** — nó vẫn tạo span riêng khi handshake OK để trace connected→disconnected; `agentWs:tokenVerify` chỉ trace riêng giai đoạn xác thực (handshake attempt → accept/reject), 2 span độc lập theo đúng ranh giới 2 mối quan tâm khác nhau (auth vs. lifecycle).

### BL-AWS-03 — Agent Token Management

| Bước | span event | fields | File:function |
|------|-----------|--------|----------------|
| `POST /api/agent-token` (generate) | `start`/`ok` — **đã có** | `devServerId`, `name`, `ttl`, `permanent` | `src/server/agent-token-routes.ts:130` (`tokenTracer.start(...)`, tracer `agentToken:register`) — không sửa |
| `GET /api/agent-token` (list, debug) | **chưa có tracer** — đề xuất bổ sung `agentToken:register` step, KHÔNG tạo tracer mới (cùng route, cùng mối quan tâm) | `count` | `agent-token-routes.ts:92-103` |
| Agent connect bằng token đã đăng ký | `step('agent-connected')` — **đã có** | `platform`, `version` | `agent-token-routes.ts:173` |
| Token hết hạn trước khi agent connect | `fail` — **đã có** | `devServerId` | `agent-token-routes.ts:177` |
| Renew token (phía Agent, direct-websocket) | `start`/`ok`/`fail` — **đã có**, tracer `agent:tokenManager` | `op: 'init'\|'renew'`, `devServerId`, `ttl` | `src/relay/agent-token-manager.ts:86,159,164,171` |
| Revoke token (Admin) | **KHÔNG TỒN TẠI trong code hiện tại** | — | Flow doc mô tả `DELETE /admin/api/agent-tokens/:id` + `orca_agent_tokens` SQL — chưa xác định file cụ thể, cần điều tra thêm khi triển khai (tính năng này chưa được build) |

```typescript
// src/server/agent-token-routes.ts — bổ sung tracing cho nhánh GET (chưa có), tái dùng tokenTracer sẵn có
if (method === 'GET') {
  const now = Date.now()
  const tokens = Array.from(pendingMeta.entries())
    .filter(([, meta]) => meta.expiresAt > now)
    .map(([token, meta]) => ({
      token, devServerId: meta.devServerId,
      expiresIn: Math.round((meta.expiresAt - now) / 1000),
    }))
  tokenTracer.start({ op: 'list' }).ok({ count: tokens.length })
  sendJson(res, 200, { tokens })
  return
}
```

Không đề xuất tracer `agentWs:tokenManage` như gợi ý ban đầu trong CR-TRACE-000 §4 vì trùng lặp: `agentToken:register` + `agent:tokenManager` đã bao phủ đúng những gì tồn tại trong code. Thêm 1 tracer nữa với cùng phạm vi sẽ vi phạm nguyên tắc "1 tracer = 1 sub-flow" (mục 4 CR-TRACE-000) theo hướng ngược — 2 tracer cho cùng 1 sub-flow.

## 5. Lan truyền traceId qua transport của flow này

1. **`devServer.connect` RPC (Browser/Renderer → Orca WS RPC, `src/main/runtime/rpc/methods/dev-server.ts:109`) → `DevServerManager.connect()` (`dev-server-manager.ts:207`, tracer `devServer:manager`) → `connectRelayWebSocket()` (`agentWs:handshake`, BL-AWS-01)**: theo CR-TRACE-000 §3.1, đây là 3 lớp trong cùng Main process — không băng qua transport nào cho tới khi mở WebSocket ra Agent. Nếu `devServer.connect` RPC nhận `params.traceId` từ Browser (hàng "WebSocket RPC" §3.3), `DevServerManager.connect()` nên resume tracer `devServer:manager` bằng id đó, rồi khi gọi `bridge.connect()` → `connectRelayWebSocket()`, forward tiếp `resume: { id: span.id }` để `agentWs:handshake` cùng `id` xuyên suốt 3 lớp — đúng mô hình GAP-1 mà CR-TRACE-000 mô tả, hiện tại **chưa triển khai** (`mgr.start()` và tracer mới ở CR này đều tạo id độc lập cho tới khi core API `resume` param ship).
2. **Agent WS JSON-RPC 2.0 (sau khi BL-AWS-01/02 handshake xong)**: mọi request/response tiếp theo qua `agent:rpc` (`agent-rpc-dispatch.ts:128`) nên đọc `rpc.params._trace?.id` theo đúng hàng "Agent WS JSON-RPC 2.0" trong CR-TRACE-000 §3.3 (`params._trace.id`, không đụng field `id` JSON-RPC có sẵn dùng để match request/response). Hiện tại `rpcTracer.start({ method: rpc.method, id: String(rpc.id ?? 'notify'), ...ctxFields })` **không đọc** `_trace.id` — đây là việc CR-TRACE-000 mục 3 (core API change) cần hoàn thành trước, CR này chỉ ghi nhận điểm cần sửa, không tự sửa `agent:rpc` (không thuộc phạm vi 3 sub-flow BL-AWS-01→03).
3. **HTTP `POST /api/agent-token` (Agent daemon script / deploy script → Orca)**: đây là HTTP thuần (không phải Agent WS JSON-RPC), áp dụng hàng "HTTP/WS `:6768` (CLI ↔ Electron Main)" trong §3.3 — script gọi endpoint này nên tự tạo `traceId` bằng tracer riêng (nếu có) và gửi trong body/header; hiện tại body request (`devServerId, name, ttl, permanent`) không có field `traceId` — không bắt buộc vì đây là script vận hành một lần, không phải business flow người dùng tương tác trực tiếp, nhưng nên hỗ trợ optional field để liên kết với `agentToken:register` khi cần debug end-to-end.
4. **SSH Relay variant của BL-AWS-01** (agent sau firewall, "Orca → SSH Tunnel → Dev Server port → relay ws://<agent>"): không có transport propagation riêng — tunnel là lớp bên dưới WebSocket, `traceId` vẫn đi theo đúng cơ chế WebSocket ở mục 2 (SSH chỉ là kênh vật lý, không phải giao thức message).

## Acceptance Criteria

- [ ] `Tracers.agentWsHandshakeFlow` (`agentWs:handshake`) bọc đúng `connectRelayWebSocket()` — mỗi lần `attempt()` (kể cả reconnect) là 1 span mới, không tái dùng `id` cũ
- [ ] `Tracers.agentWsTokenVerifyFlow` (`agentWs:tokenVerify`) bắt đầu ngay khi socket upgrade thành công (không đợi handshake resolve), khác với hành vi hiện tại ở dòng 145/171 của `agent-ws-server.ts`
- [ ] `agentWs:tokenVerify` KHÔNG log token đầy đủ trong bất kỳ field nào — chỉ `tokenPrefix` (12 ký tự đầu, theo tiền lệ dòng 166 hiện có)
- [ ] `Tracers.agentWsFlow` (`agentWs:lifecycle`, connect/disconnect) giữ nguyên hành vi, không bị merge/duplicate với `agentWs:tokenVerify`
- [ ] Nhánh `GET /api/agent-token` trong `agent-token-routes.ts` có ít nhất 1 event trace (dùng lại `tokenTracer` sẵn có, không tạo tracer mới)
- [ ] Không tracer nào trong CR này trùng tên với `agentWs:lifecycle`, `agent:rpc`, `agentToken:register`, `agent:tokenManager` đã tồn tại
- [ ] Điểm "chưa xác định" (HMAC-SHA256 Signer/RpcExecutionContext, admin token revoke + `orca_agent_tokens` SQL) được điều tra và xác nhận có tồn tại hay không trước khi viết CR follow-up cho phần đó
- [ ] Khi CR-TRACE-000 mục 3 (core API `resume`) ship, `agent:rpc` được cập nhật đọc `params._trace.id` — theo dõi như dependency, không lặp lại việc này trong CR-TRACE-013
