# TASK-08: Create agent-connection-direct.ts + agent-connection-relay.ts

**Phase:** 4  
**SOL Ref:** SOL-07  
**Estimated time:** 2h  
**Precondition:** TASK-07 (agent-session.ts) hoàn thành  

---

## File 1: `src/relay/agent-connection-direct.ts`

```typescript
// src/relay/agent-connection-direct.ts
import WebSocket from 'ws'
import type { AgentConfig } from './agent-config'
import type { ToolDefinition } from './agent-tool-registry'
import { createSession } from './agent-session'
import type { AgentLogger } from './agent-logger'

export async function connectDirect(
  config: AgentConfig,
  tools: ToolDefinition[],
  log: AgentLogger
): Promise<never>
```

**Logic:**
1. Guard: `!config.agentToken` → log error + `process.exit(1)`
2. Tạo `ws = new WebSocket(config.orcaUrl, { rejectUnauthorized: config.tlsRejectUnauthorized })`
3. Tạo session, `session.onHandshakeOk(() => lastHandshakeOk = true)`
4. `session.start(ws)`
5. `ws.once('close', code => ...)`:
   - `code === 1000` → `process.exit(0)`
   - else → `setTimeout(() => process.exit(2), 200)` (systemd restart)
   - **KHÔNG retry internally** — token là one-time use
6. Return `new Promise<never>(() => {})` — never resolves

---

## File 2: `src/relay/agent-connection-relay.ts`

```typescript
// src/relay/agent-connection-relay.ts
import { WebSocketServer } from 'ws'
import type WebSocket from 'ws'
import type { IncomingMessage } from 'node:http'
import type { AgentConfig } from './agent-config'
import type { ToolDefinition } from './agent-tool-registry'
import { createSession } from './agent-session'
import type { AgentLogger } from './agent-logger'

export async function listenRelay(
  config: AgentConfig,
  tools: ToolDefinition[],
  log: AgentLogger
): Promise<never>
```

**Logic:**
1. Token = `config.agentToken || 'relay-secret'`
2. `wss = new WebSocketServer({ port: config.agentPort, path: '/orca-relay' })`
3. `wss.once('listening', ...)` → log ready message
4. `wss.on('connection', (ws, req) => ...)`:
   - `authenticate(ws, req, token, log)` → return nếu false
   - Tạo session, `session.start(ws)`
   - `ws.once('close', () => session.stop())`
5. Return `new Promise<never>((_, reject) => ...)` — reject only on server error

**authenticate() function:**
- Lấy token từ `?token=` trong URL hoặc `Authorization: Bearer <token>` header
- Không match → `ws.close(1008, 'Unauthorized')`, return false
- Match → return true

---

## Verification

```bash
pnpm run typecheck:node 2>&1 | grep "agent-connection" || echo "No errors"
```

## Definition of Done

- [x] `src/relay/agent-connection-direct.ts` created
- [x] `src/relay/agent-connection-relay.ts` created
- [x] `connectDirect()` và `listenRelay()` exported
- [x] Direct mode: KHÔNG có retry loop sau disconnect
- [x] Relay mode: authenticate() kiểm tra cả URL ?token= và Authorization header
- [x] `pnpm run typecheck:node` passes
