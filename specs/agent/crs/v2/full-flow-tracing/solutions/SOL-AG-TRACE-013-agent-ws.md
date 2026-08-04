# SOL-AG-TRACE-013: Agent WebSocket Protocol — Agent-Side Tracing Implementation

**CR Ref:** [CR-TRACE-013](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-013-agent-ws.md)
**TDD Ref:** TDD-AG-02 (Wire Protocol), TDD-AG-03 (Connection Modes), TDD-AG-04 (Handshake & Session)
**File(s):**
- `src/relay/agent-connection-relay.ts` [MODIFY]
- `src/relay/agent-connection-direct.ts` [DOCUMENT ONLY — no code change]
- `src/relay/agent-session.ts` [DOCUMENT ONLY — no code change]
**Mức độ:** 🟢 Đơn giản
**Thời gian ước tính:** 1.5h
**Status:** Proposed

---

## 1. Phạm vi (Agent-side only)

CR-TRACE-013 mô tả gap ở **phía Orca/backend** (`dev-server-relay-bridge.ts` cho BL-AWS-01, `agent-ws-server.ts` cho BL-AWS-02, `agent-token-routes.ts` cho BL-AWS-03) và đề xuất 2 tracer mới `agentWs:handshake`/`agentWs:tokenVerify` **sống trong `src/main/dev-server/*.ts`** — hoàn toàn ngoài phạm vi solution này. Solution này chỉ bao phủ **phía Dev Server Agent** (`src/relay/agent-connection-*.ts`, `agent-session.ts` — tiến trình `agent.js`), tức là đầu bên kia của cùng 1 WebSocket.

Vì 2 connection mode có **hướng khởi tạo TCP ngược nhau**, vai trò "ai verify token" cũng đảo ngược giữa Agent và Orca theo từng mode — bảng dưới đối chiếu rõ để tránh nhầm lẫn với các tracer đã tồn tại:

| Mode | Ai khởi tạo TCP | Ai verify token | Backend-side tracer (CR-TRACE-013, NGOÀI phạm vi) | Agent-side file (phạm vi solution này) | Agent-side tracer hiện tại |
|------|------------------|-------------------|-----------------------------------------------------|-------------------------------------------|------------------------------|
| `direct-websocket` (BL-AWS-02) | **Agent** (`agent-connection-direct.ts`) | **Orca** (`agent-ws-server.ts`, `agentWs:tokenVerify` — mới, backend) | `agentWs:tokenVerify` | `agent-connection-direct.ts` + `agent-session.ts` | **Đã có**: `agent:connection` + `agent:session` |
| `relay-websocket` (BL-AWS-01) | **Orca** (`dev-server-relay-bridge.ts`) | **Agent** (`agent-connection-relay.ts`, hàm `authenticate()`) | `agentWs:handshake` | `agent-connection-relay.ts` | **CHƯA có tracer nào** |

Không tracer nào trong solution này trùng tên với 4 tracer CR-TRACE-013 liệt kê là "không được đụng": `agentWs:lifecycle` (backend, `agent-ws-server.ts`), `agent:rpc` (dispatch generic, `agent-rpc-dispatch.ts`, không sửa trong CR này), `agentToken:register` (backend, `agent-token-routes.ts`), `agent:tokenManager` (`agent-token-manager.ts` — đã có sẵn, không sửa). Solution này cũng **xây cạnh** (không thay thế) 2 tracer agent-side đã tồn tại: `agent:connection` (`agent-connection-direct.ts:24`) và `agent:session` (`agent-session.ts:37`).

## 2. Gap hiện tại

| # | File:function | Trạng thái | Hành động |
|---|----------------|-----------|-----------|
| 1 | `agent-connection-direct.ts` — `connTracer` (`agent:connection`), span per reconnect `attempt()` (dòng 70), `step('handshake-ok')` (84), `ok({code})`/`fail(...)` trên close (93/100/103)/error (119) | **Đã đầy đủ** — mỗi lần thử kết nối (kể cả reconnect) là 1 span mới, đúng nguyên tắc CR-TRACE-000 §5 rule 3 | Không sửa — chỉ document |
| 2 | `agent-session.ts` — `sessionTracer` (`agent:session`), span trong `start()` (dòng 189), `step('handshake-sent')` (194), `step('handshake-ok')` (252), `fail` khi handshake lỗi (199, 256) hoặc ws error (280), `ok`/`fail` trên close (272/274) | **Đã đầy đủ** cho vai trò "Agent gửi handshake, chờ Orca accept/reject" | Không sửa — chỉ document |
| 3 | `agent-connection-relay.ts` — toàn bộ file (`listenRelay()`, `authenticate()`) | **Không có `createTracer`/span nào** — xác nhận qua `grep -rn "createTracer(" src/relay/`, file này không xuất hiện trong danh sách 9 tracer ad-hoc đã có | **Thêm mới** tracer `agent:connectionRelay` |

## 3. Full Implementation

### 3.1 `agent-connection-relay.ts` — tracer mới `agent:connectionRelay`

Đặt tên theo convention local đã thiết lập trong `src/relay/` (`agent:xxx`, xem 9 tracer hiện có), **không** dùng prefix `agentWs:` vì đó là namespace CR-TRACE-013 dành riêng cho backend process (`src/main/dev-server/*.ts`) — dùng chung prefix sẽ gây nhầm lẫn 2 process khác nhau cùng 1 tên tracer khi filter log theo `flow`. Đặt tên `agent:connectionRelay` (khác `agent:connection` của `agent-connection-direct.ts`) vì đây là **sub-flow khác cấu trúc** (Agent là WS server thay vì WS client) — tuân thủ nguyên tắc "1 tracer = 1 sub-flow" (CR-TRACE-000 §4).

```typescript
// src/relay/agent-connection-relay.ts
// (thêm import + tracer, sửa listenRelay() và authenticate())

import { WebSocketServer } from 'ws'
import type WebSocket from 'ws'
import type { IncomingMessage } from 'node:http'
import type { AgentConfig } from './agent-config'
import type { ToolDefinition } from './agent-tool-registry'
import { createSession } from './agent-session'
import type { AgentLogger } from './agent-logger'
import { createTracer } from '../shared/trace'
import type { TraceSpan } from '../shared/trace'

const relayConnTracer = createTracer('agent:connectionRelay')

export async function listenRelay(
  config: AgentConfig,
  tools: ToolDefinition[],
  log: AgentLogger
): Promise<never> {
  const token = config.agentToken?.trim()
  if (!token) {
    log.error('FATAL: agentToken (ORCA_AGENT_TOKEN) is not set or empty.')
    log.error('In relay-websocket mode, a shared secret is required for authentication.')
    process.exit(1)
  }

  return new Promise<never>((_, reject) => {
    const wss = new WebSocketServer({ port: config.agentPort, path: '/orca-relay' })

    wss.once('listening', () => {
      log.info(`✅ Relay server ready: ws://0.0.0.0:${config.agentPort}/orca-relay`)
      log.info(`Orca UI config → Type: relay-websocket  URL: ws://<devServerHost>:${config.agentPort}/orca-relay`)
    })

    wss.on('connection', (ws: WebSocket, req: IncomingMessage) => {
      const remoteAddr = req.socket.remoteAddress ?? 'unknown'
      const span = relayConnTracer.start({ remoteAddr })

      if (!authenticate(ws, req, token, log, span)) return

      log.info(`Orca Server connected from ${remoteAddr}`)
      span.step('accepted', { remoteAddr })

      const session = createSession(config, tools, log)
      session.start(ws)

      ws.once('close', (code: number) => {
        session.stop()
        if (code === 1000) {
          span.ok({ code, remoteAddr })
        } else {
          span.fail(`ws close code=${code}`, { code, remoteAddr })
        }
        log.info(`Orca Server disconnected from ${remoteAddr} (code=${code})`)
      })
    })

    wss.once('error', (err: Error) => {
      log.error(`Relay server fatal error: ${err.message}`)
      reject(err)
    })
  })
}

/**
 * Authenticate an incoming connection.
 * Accepts token from ?token=<token> (query) or Authorization: Bearer <token> (header).
 *
 * `span` optional — khi truyền vào, ghi lại step('tokenAccepted')/fail('unauthorized')
 * KHÔNG BAO GIỜ log giá trị token, chỉ classify nguồn (query/header/none) — cùng nguyên
 * tắc "không log token" như ORCH-013 (xem comment gốc trong listenRelay()).
 */
function authenticate(
  ws: WebSocket,
  req: IncomingMessage,
  expectedToken: string,
  log: AgentLogger,
  span?: TraceSpan
): boolean {
  const rawUrl = req.url ?? ''

  let queryToken = ''
  try {
    queryToken = new URL(`ws://localhost${rawUrl}`).searchParams.get('token') ?? ''
  } catch {
    // Ignore URL parse errors — treat as empty token
  }

  const authHeader = (req.headers['authorization'] ?? '') as string
  const bearerToken = authHeader.replace(/^Bearer\s+/i, '').trim()

  const incoming = queryToken || bearerToken
  const source: 'query' | 'header' | 'none' = queryToken ? 'query' : bearerToken ? 'header' : 'none'

  if (incoming !== expectedToken) {
    span?.fail('unauthorized', { source })
    log.warn(`Rejected unauthorized connection from ${req.socket.remoteAddress ?? 'unknown'}`)
    ws.close(1008, 'Unauthorized')
    return false
  }

  span?.step('tokenAccepted', { source })
  return true
}
```

**Ràng buộc bảo mật:** field `source` chỉ nhận giá trị `'query' | 'header' | 'none'` — không đưa `queryToken`/`bearerToken`/`expectedToken` vào bất kỳ `TraceFields` nào. Backward-compatible: `span` là tham số optional cuối cùng, mọi call site cũ (kể cả test hiện có replicate hàm này mà không truyền `span`) vẫn hoạt động.

### 3.2 `agent-connection-direct.ts` / `agent-session.ts` — không sửa code, chỉ đối chiếu coverage

Để tránh trùng lặp công sức, bảng dưới xác nhận 2 tracer đã có (`agent:connection`, `agent:session`) đã bao phủ đủ những gì CR-TRACE-013 mô tả cho BL-AWS-02 ở phía backend (`agentWs:tokenVerify`) tương ứng phía agent — không có hành động sửa code nào ở đây, chỉ dùng để review khi implement:

| Sự kiện | `agent:connection` (mỗi lần `runConnection()`, dòng 70) | `agent:session` (mỗi lần `start()`, dòng 189) |
|---------|------------------------------------------------------------|-----------------------------------------------|
| Bắt đầu thử kết nối | `start({url, attempt})` | — |
| WS mở, gửi handshake | — | `start({devServerId})` ngay khi `session.start(ws)` được gọi trong `runConnection()` |
| Handshake gửi xong (async, chờ capability check) | — | `step('handshake-sent')` |
| Orca accept | `step('handshake-ok')` (qua callback `onHandshakeOk`) | `step('handshake-ok', {sessionId, orcaVersion})` |
| Orca reject | — (session đã `fail()` trước khi `close` event bắn lên `connSpan`) | `fail('handshake: ...', {code})` |
| WS đóng sạch (code=1000) | `ok({code})` | `ok({code, reason})` |
| WS đóng bất thường | `fail('connection dropped after handshake' \| 'closed before handshake', {code})` | `fail('ws close code=...', {code, reason})` |
| Lỗi WS tầng transport | `fail(err)` | `fail(err, {phase: 'ws-error'})` |

Không cần thêm bất kỳ instrumentation nào — 2 span này cùng tồn tại song song (1 ở tầng "connection attempt/reconnect loop", 1 ở tầng "session protocol trong 1 connection"), đúng mô hình "mỗi layer đo latency riêng" của CR-TRACE-000 §3.1.

### 3.3 Lan truyền `traceId` — phụ thuộc CR-TRACE-000 §3 (chưa ship)

`agent:rpc` (`agent-rpc-dispatch.ts:21`) hiện **không đọc** `rpc.params._trace?.id` — xác nhận qua Read: `rpcTracer.start({ method, id, ...ctxFields })` gọi `Tracer.start(fields)` một tham số, và `src/shared/trace/index.ts` hiện tại (`export interface Tracer { start(fields?: TraceFields): TraceSpan }`) **chưa có tham số `resume`** mà CR-TRACE-000 §3.1 đề xuất. Đây là dependency đã được chính CR-TRACE-013 §5 mục 2 ghi nhận — solution này **không** tự thêm `resume` vào core API (thuộc CR-TRACE-000, ngoài phạm vi 3 sub-flow BL-AWS-01→03), chỉ nhắc lại như một điều kiện tiên quyết: khi `Tracer.start(fields?, resume?)` ship, `agent:connectionRelay`/`agent:connection`/`agent:session` đều là các span **khởi đầu** một trace mới (không có `traceId` nào để resume từ trước, vì đây là điểm bắt đầu kết nối) — không cần sửa gì thêm ở 3 tracer này khi core API ship.

## 4. Test Plan (Vitest)

### 4.1 Mở rộng `src/relay/__tests__/agent-connection-relay.test.ts` (đã tồn tại)

File test hiện tại **replicate `authenticate()` inline** (không import từ module thật) để test mà không cần khởi động `WebSocketServer` thật. Cần cập nhật bản replicate để khớp signature mới (`span?` param) và thêm test case cho hành vi trace:

```typescript
// src/relay/__tests__/agent-connection-relay.test.ts
// (bổ sung sau describe('authenticate() — URL query string') hiện có)

import { registerTraceSink } from '../../shared/trace'
import type { TraceEvent, TraceSpan } from '../../shared/trace'

// Cập nhật bản replicate để nhận thêm `span?: TraceSpan`
function authenticate(
  ws: { close: (code: number, reason: string) => void },
  req: { url?: string; headers: Record<string, string>; socket: { remoteAddress?: string } },
  expectedToken: string,
  log: { warn: (msg: string) => void },
  span?: TraceSpan
): boolean {
  const rawUrl = req.url ?? ''
  let queryToken = ''
  try { queryToken = new URL(`ws://localhost${rawUrl}`).searchParams.get('token') ?? '' } catch {}
  const authHeader  = (req.headers['authorization'] ?? '')
  const bearerToken = authHeader.replace(/^Bearer\s+/i, '').trim()
  const incoming    = queryToken || bearerToken
  const source: 'query' | 'header' | 'none' = queryToken ? 'query' : bearerToken ? 'header' : 'none'

  if (incoming !== expectedToken) {
    span?.fail('unauthorized', { source })
    log.warn(`Rejected unauthorized connection from ${req.socket.remoteAddress ?? 'unknown'}`)
    ws.close(1008, 'Unauthorized')
    return false
  }
  span?.step('tokenAccepted', { source })
  return true
}

describe('authenticate() — agent:connectionRelay tracing', () => {
  it('span.step("tokenAccepted", {source:"query"}) khi token hợp lệ qua query string', () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink(e => events.push(e))
    const span = relayConnTracerForTest.start({ remoteAddr: '127.0.0.1' })

    const ok = authenticate(makeWs(), makeReq(`/orca-relay?token=${TOKEN}`), TOKEN, mockLog, span)

    unregister()
    expect(ok).toBe(true)
    const step = events.find(e => e.level === 'step' && e.label === 'tokenAccepted')
    expect(step?.fields.source).toBe('query')
  })

  it('span.fail("unauthorized", {source:"none"}) khi thiếu token, KHÔNG có field nào chứa token thật', () => {
    const events: TraceEvent[] = []
    const unregister = registerTraceSink(e => events.push(e))
    const span = relayConnTracerForTest.start({ remoteAddr: '127.0.0.1' })

    const ok = authenticate(makeWs(), makeReq(''), TOKEN, mockLog, span)

    unregister()
    expect(ok).toBe(false)
    const fail = events.find(e => e.level === 'fail')
    expect(fail?.fields.source).toBe('none')
    expect(JSON.stringify(events)).not.toContain(TOKEN)
  })

  it('authenticate() vẫn hoạt động khi span không được truyền (backward-compat)', () => {
    expect(authenticate(makeWs(), makeReq(`/orca-relay?token=${TOKEN}`), TOKEN, mockLog)).toBe(true)
  })
})
```

*Ghi chú khuyến nghị (không bắt buộc trong CR này):* nên chuyển từ pattern "replicate inline" sang `import { authenticate } from '../agent-connection-relay'` (cần export hàm này trước) để tránh bản replicate lệch khỏi implementation thật theo thời gian — rủi ro đã tồn tại từ trước khi có solution này, chỉ ghi nhận lại.

### 4.2 `src/relay/__tests__/agent-connection-direct.test.ts` / `agent-session.test.ts` (đã tồn tại)

Không cần thêm test mới cho tracing (coverage đã đủ theo mục 3.2) — chỉ xác nhận qua CI hiện có rằng các test case liên quan `handshake-ok`, `close code=1000`, `close trước handshake` vẫn pass sau khi `agent-connection-relay.ts` thay đổi (không có phụ thuộc chéo giữa 2 file).

## 5. Acceptance Criteria

- [ ] `agent:connectionRelay` bọc đúng toàn bộ vòng đời 1 kết nối relay-websocket: `start` khi TCP connection được accept, `step('tokenAccepted')`/`fail('unauthorized')` trong `authenticate()`, `ok`/`fail` khi WS đóng
- [ ] `authenticate()` không bao giờ đưa giá trị token thật (`queryToken`/`bearerToken`/`expectedToken`) vào bất kỳ `TraceFields` nào — chỉ `source: 'query'|'header'|'none'`
- [ ] `agent:connection` (`agent-connection-direct.ts`) và `agent:session` (`agent-session.ts`) giữ nguyên hành vi — không có thay đổi code, chỉ xác nhận coverage qua bảng mục 3.2
- [ ] Không tracer nào trong solution này trùng tên với `agentWs:lifecycle`, `agent:rpc`, `agentToken:register`, `agent:tokenManager`, `agentWs:handshake`/`agentWs:tokenVerify` (2 tracer mới phía backend, CR-TRACE-013, khác process)
- [ ] `agent-connection-relay.test.ts` có thêm ≥ 3 test case theo mục 4.1, bao gồm 1 test khẳng định không có token thật lọt vào `TraceEvent`
- [ ] `pnpm vitest run src/relay/__tests__/agent-connection-relay.test.ts` pass
- [ ] Khi CR-TRACE-000 §3 (core API `resume`) ship, xác nhận lại rằng `agent:connectionRelay`/`agent:connection`/`agent:session` KHÔNG cần sửa (đều là điểm bắt đầu trace, không có `traceId` nào để resume) — chỉ `agent:rpc` mới cần đọc `params._trace.id`, việc đó KHÔNG thuộc phạm vi CR-TRACE-013/solution này
