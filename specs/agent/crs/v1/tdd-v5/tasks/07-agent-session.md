# TASK-07: Create src/relay/agent-session.ts

**Phase:** 3  
**SOL Ref:** SOL-06  
**Estimated time:** 2h  
**Precondition:** TASK-04 (agent-wire), TASK-06 (agent-rpc-dispatch) hoàn thành  

---

## Tạo file mới: `src/relay/agent-session.ts`

### Imports chính xác

```typescript
import type WebSocket from 'ws'
import type { AgentConfig } from './agent-config'
import type { ToolDefinition } from './agent-tool-registry'
import type { AgentLogger } from './agent-logger'
import { createWireState, decodeFrame, encodeDataFrame, encodeKeepaliveFrame, parseJsonPayload } from './agent-wire'
import { createRpcDispatcher } from './agent-rpc-dispatch'
import { AGENT_HANDSHAKE_METHOD, AGENT_KEEPALIVE_INTERVAL_MS } from '../shared/agent-wire-protocol'
import { MessageType } from '../main/ssh/relay-protocol'
```

### AgentSession interface

```typescript
export interface AgentSession {
  start(ws: WebSocket): void
  stop(): void
  onHandshakeOk(callback: () => void): void
}

export function createSession(
  config: AgentConfig,
  tools: ToolDefinition[],
  log: AgentLogger
): AgentSession
```

### Logic chính trong `start(ws)`

1. Tạo `wireState = createWireState()` — **per-session, không phải per-module**
2. Nếu `ws.readyState === 1` (OPEN): gọi `sendHandshake()` và `startKeepalive()` ngay
3. Else: đăng ký `ws.once('open', ...)` để gọi sau
4. `ws.on('message', handler)`:
   - Bỏ qua non-Buffer data
   - `decodeFrame()` → null? bỏ qua
   - `frame.type === MessageType.KeepAlive (= 9)` → gửi keepalive pong ngay
   - `frame.payload.length === 0` → bỏ qua
   - `parseJsonPayload()` → null? log warn, bỏ qua
   - Chưa handshake: chỉ xử lý `result.ok === true` (set handshakeDone, fire callbacks) hoặc `error` (close ws)
   - Sau handshake: `dispatcher.dispatch(ws, wireState, rpc)`

### sendHandshake() — payload format

```typescript
{
  jsonrpc: '2.0',
  id: 1,
  method: 'agent.handshake',       // = AGENT_HANDSHAKE_METHOD
  params: {
    agentVersion: '2.1.0',
    platform: process.platform,
    arch: process.arch,
    nodeVersion: process.version,
    capabilities: ['fs', 'git', 'preflight'],
    // agentToken chỉ gửi khi config.agentToken không rỗng:
    ...(config.agentToken ? { agentToken: config.agentToken } : {}),
    devServerId: config.devServerId,
    tools: tools.map(t => t.name),
  }
}
```

Encode với `encodeDataFrame(wireState, JSON.stringify(rpc))`.

### startKeepalive() — interval

```typescript
keepaliveTimer = setInterval(() => {
  if (ws.readyState === 1) ws.send(encodeKeepaliveFrame(wireState))
}, AGENT_KEEPALIVE_INTERVAL_MS)  // = 5000ms từ shared/agent-wire-protocol
```

### stop()

```typescript
if (keepaliveTimer !== null) {
  clearInterval(keepaliveTimer)
  keepaliveTimer = null
}
```

---

## Verification

```bash
pnpm run typecheck:node 2>&1 | grep "agent-session" || echo "No errors"
```

## Definition of Done

- [x] `src/relay/agent-session.ts` created
- [x] `createSession()` exported
- [x] `MessageType.KeepAlive === 9` dùng đúng cho keepalive detection
- [x] `AGENT_KEEPALIVE_INTERVAL_MS` imported từ `shared/agent-wire-protocol`
- [x] `wireState` tạo trong `start()` — không phải module level
- [x] `pnpm run typecheck:node` passes
