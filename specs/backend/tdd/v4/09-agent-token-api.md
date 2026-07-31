# TDD-BE-09: Agent Token API

**Version:** 4.0  
**Date:** 2026-07-28  
**Source:** `src/server/agent-token-routes.ts`

---

## 1. Mục tiêu

Cho phép deployment scripts (systemd service, CI/CD) đăng ký agent token để:
1. Kết nối agent vào Orca Server qua direct-WebSocket
2. Agent xuất hiện trong Dev Servers list trên UI

---

## 2. Endpoints

```
POST /api/agent-token
  Auth: Bearer <ORCA_AGENT_API_SECRET>   (hoặc X-Orca-Admin: 1 nếu không có secret)
  Body: { devServerId?, name?, ttl? }     (ttl max 600 giây)

  Response: {
    token:        string,    // unique agent token
    devServerId:  string,
    name:         string,
    expiresIn:    number,    // seconds
    created:      boolean,   // true = DevServer record mới được tạo
    agentCommand: string     // ORCA_URL=... AGENT_TOKEN=... node agent.js
  }

GET /api/agent-token
  Auth: same
  Response: { tokens: [{ token, devServerId, expiresIn }] }  // debug listing
```

---

## 3. Auth Logic

```typescript
function isAuthorized(req): boolean {
  const apiSecret = process.env['ORCA_AGENT_API_SECRET']
  if (apiSecret) {
    // Production: Bearer token
    return req.headers['authorization'] === `Bearer ${apiSecret}`
  }
  // Dev fallback: X-Orca-Admin: 1 header
  return req.headers['x-orca-admin'] === '1'
}
```

---

## 4. POST Flow (Path A — với DevServerManager)

```
POST /api/agent-token { devServerId: 'prod-server', name: 'Production', ttl: 300 }

1. isAuthorized() check
2. Generate token: generateAgentToken(devServerId)
3. devServerManager.connectDaemonAgent({ devServerId, name, token, ttlMs })
   a. DevServerStore.findOrCreate(devServerId, name)
   b. DevServerRelayBridge.registerSlot(token, onConnected, onExpired)
   c. Emit 'devServer:added' (nếu mới) + 'devServer:statusChanged' → 'connecting'
4. Return { token, expiresIn: 300, created: true/false }

Agent nhận token → kết nối:
  ws://<orca-host>:6769/agent?token=<token>
  → AgentWebSocketServer nhận → handshake → DevServerRelayBridge
  → Emit 'devServer:statusChanged' → 'connected'
```

---

## 5. POST Flow (Path B — không có DevServerManager)

```
Fallback mode (test/edge case):
  agentWsServer.registerSlot(token, onConnected, onExpired)
  Agent kết nối được nhưng KHÔNG xuất hiện trong Dev Servers UI
  Response có warning: 'DevServerManager not available'
```

---

## 6. Token Format

```typescript
import { generateAgentToken } from '../shared/agent-wire-protocol'

// Format: '<devServerId>-<uuid4>'
// e.g., 'prod-server-550e8400-e29b-41d4-a716-446655440000'
```

---

## 7. TTL & Cleanup

- TTL max: 600 giây (10 phút)
- Pending metadata tracked in-memory: `Map<token, { devServerId, createdAt, expiresAt }>`
- Cleanup: `setTimeout(() => pendingMeta.delete(token), ttlSec * 1000)`
- AgentWebSocketServer slot cũng tự expire sau `AGENT_CONNECT_TIMEOUT_MS` (60s)
- Token là one-time use: sau khi agent connect → slot bị consume

---

## 8. apiHandler Interface

```typescript
// createAgentTokenApiHandler() trả về function compatible với HttpServerOptions.apiHandler
export function createAgentTokenApiHandler(
  agentWsServer: AgentWebSocketServer,
  devServerManager: DevServerManager | null = null
): (req: IncomingMessage, res: ServerResponse) => boolean

// Return true nếu handled (URL là /api/agent-token)
// Return false để HTTP server tiếp tục với static file handler
```
