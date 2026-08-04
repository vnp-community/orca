# TASK-AG-013.1: Add agent:connectionRelay tracer to agent-connection-relay.ts

**Phase:** 2
**SOL Ref:** [SOL-AG-TRACE-013](../solutions/SOL-AG-TRACE-013-agent-ws.md)
**CR Ref:** [CR-TRACE-013](../../../../../../docs/crs/v2/full-flow-tracing/CR-TRACE-013-agent-ws.md)
**Precondition:** Phase 0 (`Tracer.start(fields?, resume?)`)
**Estimated time:** 1h
**Status:** ✅ Done (2026-08-03) — implemented as specified (local `createTracer('agent:connectionRelay')`); no drift from spec. `gitnexus_impact` on `listenRelay`/`authenticate` reported LOW risk (1 direct caller each, no execution flows affected). typecheck:node clean for `agent-connection-relay.ts` itself (pre-existing TS errors remain in `agent-connection-relay.test.ts`, fixed by TASK-AG-013.2).

---

## Trước khi sửa (bắt buộc theo CLAUDE.md)

```bash
codegraph explore "listenRelay"
codegraph explore "authenticate"
```

Cả 2 đều là symbol MODIFY (đã tồn tại) — chạy thêm impact analysis cho từng symbol:

```
gitnexus_impact({ target: "listenRelay", direction: "upstream" })
gitnexus_impact({ target: "authenticate", direction: "upstream" })
```

và báo cáo blast radius (caller trực tiếp, process bị ảnh hưởng, risk level) trước khi sửa. Nếu risk HIGH/CRITICAL, dừng lại và xác nhận với người dùng trước khi tiếp tục.

## Bối cảnh

CR-TRACE-013 mô tả gap phía backend (`dev-server-relay-bridge.ts`, `agent-ws-server.ts`, `agent-token-routes.ts` — ngoài phạm vi, 2 tracer `agentWs:handshake`/`agentWs:tokenVerify` sống ở đó). Task này chỉ bao phủ phía Dev Server Agent, cụ thể là `agent-connection-relay.ts` (mode `relay-websocket`, nơi **Agent là WS server**, Orca Server kết nối vào và Agent verify token trong `authenticate()`).

**Không sửa** `agent-connection-direct.ts` và `agent-session.ts` — 2 tracer đã có (`agent:connection`, `agent:session`) đã bao phủ đủ theo bảng đối chiếu trong SOL-AG-TRACE-013 §3.2, không cần code thay đổi.

**Đặt tên `agent:connectionRelay`** (không dùng `agentWs:` — namespace đó dành riêng cho backend/Main process, tránh nhầm lẫn 2 process khi filter log theo `flow`).

## File: `src/relay/agent-connection-relay.ts` [MODIFY]

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

**Ràng buộc bảo mật:** field `source` chỉ nhận giá trị `'query' | 'header' | 'none'` — không đưa `queryToken`/`bearerToken`/`expectedToken` vào bất kỳ `TraceFields` nào. `span` là tham số optional cuối cùng — backward-compatible, mọi call site cũ vẫn hoạt động.

## Không sửa (chỉ ghi chú tham khảo)

`agent-connection-direct.ts` (`agent:connection`) và `agent-session.ts` (`agent:session`) — KHÔNG có thay đổi code trong task này. Coverage bảng đối chiếu:

| Sự kiện | `agent:connection` | `agent:session` |
|---------|---------------------|-------------------|
| Bắt đầu thử kết nối | `start({url, attempt})` | — |
| WS mở, gửi handshake | — | `start({devServerId})` |
| Handshake gửi xong | — | `step('handshake-sent')` |
| Orca accept | `step('handshake-ok')` | `step('handshake-ok', {sessionId, orcaVersion})` |
| Orca reject | — | `fail('handshake: ...', {code})` |
| WS đóng sạch (1000) | `ok({code})` | `ok({code, reason})` |
| WS đóng bất thường | `fail(...)` | `fail('ws close code=...', {...})` |
| Lỗi WS transport | `fail(err)` | `fail(err, {phase: 'ws-error'})` |

## Verification

```bash
pnpm run typecheck:node 2>&1 | grep "agent-connection-relay" || echo "No errors"
```

```bash
# Sau khi sửa xong, trước khi coi task DONE:
gitnexus_detect_changes()
```

Xác nhận chỉ các symbol/flow dự kiến bị ảnh hưởng — nếu detect_changes báo thêm symbol ngoài phạm vi task này, điều tra trước khi tiếp tục.

## Definition of Done

- [ ] `agent:connectionRelay` bọc đúng toàn bộ vòng đời 1 kết nối relay-websocket: `start` khi TCP connection được accept, `step('tokenAccepted')`/`fail('unauthorized')` trong `authenticate()`, `ok`/`fail` khi WS đóng
- [ ] `authenticate()` không bao giờ đưa giá trị token thật vào `TraceFields` — chỉ `source: 'query'|'header'|'none'`
- [ ] `agent-connection-direct.ts` và `agent-session.ts` KHÔNG bị sửa — verify bằng `git diff --stat` chỉ có `agent-connection-relay.ts`
- [ ] Không tracer nào trong task này trùng tên với `agentWs:lifecycle`, `agent:rpc`, `agentToken:register`, `agent:tokenManager` (namespace backend/khác file, đã tồn tại)
- [ ] `span` là tham số optional cuối cùng trong `authenticate()` — backward-compatible
- [ ] `pnpm run typecheck:node` pass
