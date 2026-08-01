# SOLUTION: agent-ws — Code-Level Exact Fixes

**Source-verified:** ✅ Dựa trên source code thực tế  
**Files nguồn đã đọc:** `agent-token-routes.ts`, `agent-ws-server.ts`, `dev-server-relay-bridge.ts`

---

## BUG-AWS-004: `X-Orca-Admin: 1` auth bypass (CRITICAL SECURITY)

**File:** [`src/server/agent-token-routes.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/server/agent-token-routes.ts)  
**Lines:** 36–44

### Code sai thực tế:
```typescript
function isAuthorized(req: IncomingMessage): boolean {
  const apiSecret = process.env['ORCA_AGENT_API_SECRET']
  if (apiSecret) {
    const auth = req.headers['authorization'] ?? ''
    return auth === `Bearer ${apiSecret}`
  }
  // Dev fallback: X-Orca-Admin: 1 header (no secret configured)
  return req.headers['x-orca-admin'] === '1'  // ← BUG: no secret needed!
}
```

### Fix:
```typescript
// src/server/agent-token-routes.ts — Replace lines 36–44:
function isAuthorized(req: IncomingMessage): boolean {
  const apiSecret = process.env['ORCA_AGENT_API_SECRET']?.trim()
  if (!apiSecret) {
    // SECURITY FIX: If no secret configured, BLOCK all requests.
    // Do NOT allow X-Orca-Admin bypass — this is a production security requirement.
    console.error(
      '[SECURITY] ORCA_AGENT_API_SECRET not configured. ' +
      'POST /api/agent-token is DISABLED. Set ORCA_AGENT_API_SECRET to enable.'
    )
    return false
  }
  const auth = req.headers['authorization'] ?? ''
  return auth === `Bearer ${apiSecret}`
}
```

**Thêm startup warning** trong `src/main/server-bootstrap.ts`:
```typescript
// Thêm vào cuối server startup, TRƯỚC khi listen():
if (!process.env['ORCA_AGENT_API_SECRET']) {
  console.warn(
    '[SECURITY WARNING] ORCA_AGENT_API_SECRET not set. ' +
    'POST /api/agent-token endpoint is DISABLED. ' +
    'Dev servers cannot be added until this is configured.'
  )
}
```

---

## BUG-AWS-002: Token không SHA-256 hash

**File:** [`src/server/agent-token-routes.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/server/agent-token-routes.ts)  
**Line:** 113

### Code sai thực tế:
```typescript
const token = generateAgentToken(devServerId)
// ...
pendingMeta.set(token, { devServerId, createdAt: Date.now(), expiresAt })
```

### Phân tích
Bug report nói token không hash. Nhưng `agent-ws-server.ts` verify bằng `pendingSlots.has(token)` (raw token). Nếu memory bị dump, raw tokens bị lộ.

**Mức độ thực tế:** MEDIUM — ephemeral in-memory token, TTL 10 phút. Rủi ro thấp hơn so với persistent DB token.

### Fix cho in-memory ephemeral case (không cần DB table ngay):
```typescript
// src/server/agent-token-routes.ts — thêm import:
import { createHash } from 'node:crypto'

// Helper để hash token:
function hashToken(raw: string): string {
  return createHash('sha256').update(raw).digest('hex')
}

// Khi store vào pendingMeta, lưu hash thay vì raw:
const token = generateAgentToken(devServerId)
const tokenHash = hashToken(token)
pendingMeta.set(tokenHash, { devServerId, createdAt: Date.now(), expiresAt })
// Response vẫn trả raw token về client (agent cần raw token để connect):
sendJson(res, 200, { token, expiresIn: ttlSec, devServerId })
```

**Quan trọng:** `AgentWebSocketServer.registerSlot()` cũng phải nhận `tokenHash`:
```typescript
// src/main/dev-server/dev-server-relay-bridge.ts:
// Khi gọi agentWsServer.registerSlot(), pass tokenHash thay vì raw token:
const agentToken = generateAgentToken(this.config.id)
const tokenHash = createHash('sha256').update(agentToken).digest('hex')
const disposer = this.agentWsServer!.registerSlot(
  tokenHash,  // ← store hash
  ...
)
// Emit raw token cho UI (agent cần connect với raw token):
this.emit('agentTokenGenerated', { devServerId: this.config.id, agentToken, orcaUrl: orcaWsUrl })
```

**Agent-ws-server.ts** — verify bằng hash:
```typescript
// agent-ws-server.ts:116 — validator:
import { createHash } from 'node:crypto'

runOrcaReceiverHandshake(
  ws,
  (rawToken) => {
    const hash = createHash('sha256').update(rawToken).digest('hex')
    return this.pendingSlots.has(hash)  // ← check hash not raw
  },
  this.orcaVersion
)
// Sau handshake, cũng dùng hash để lookup:
const tokenHash = createHash('sha256').update(info.agentToken ?? '').digest('hex')
const slot = this.pendingSlots.get(tokenHash)
```

---

## BUG-AWS-003: Token TTL max 10 phút — không có long-lived token

**File:** [`src/server/agent-token-routes.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/server/agent-token-routes.ts)  
**Line:** 111

### Code sai thực tế:
```typescript
const ttlSec = Math.min(Number(body['ttl'] ?? 300), 600)   // max 10 min
```

### Fix — thêm `permanent` option cho production servers:
```typescript
// src/server/agent-token-routes.ts:111 — thay thế:
const requestedTtl = Number(body['ttl'] ?? 300)
const isPermanent  = body['permanent'] === true

let ttlSec: number
let expiresAt: number

if (isPermanent) {
  // Long-lived token for production dev servers: 30 ngày
  ttlSec    = 30 * 24 * 60 * 60  // 30 days in seconds
  expiresAt = Date.now() + ttlSec * 1000
} else {
  ttlSec    = Math.min(requestedTtl, 600)  // max 10 min for ephemeral
  expiresAt = Date.now() + ttlSec * 1000
}
```

**Long-lived token flow:**
- Production dev server gọi `POST /api/agent-token { permanent: true }` 1 lần
- `AgentWebSocketServer.registerSlot()` với TTL = 30 ngày
- Agent reconnect với cùng token mỗi khi khởi động
- Admin revoke bằng `DELETE /api/agent-token/:tokenHash`

---

## BUG-AWS-001: relay-ws mode topology

**File:** [`src/main/dev-server/agent-ws-server.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/dev-server/agent-ws-server.ts) và docs  

**Phân tích từ code thực tế:**
- `agent-ws-server.ts:4–9` — comment nói:
  ```
  // Browser  → ws://:6768/        (existing OrcaRuntimeRpcServer — unchanged)
  // Agent    → ws://:6769/agent   (NEW — this file handles /agent path on HTTP server)
  ```
- Agent connect **inbound** vào Orca: `ws://orca-host:6769/agent` ✅
- Orca không connect outbound → **topology đúng**

**Bug thực tế:** Comment nói port 6769, nhưng TDD-11 nói 6768. Xem BUG-BE-TM-004 fix.

---

## BUG-BE-AWS-001: AgentWebSocketServer verify in-memory thay vì DB

**File:** [`src/main/dev-server/agent-ws-server.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/dev-server/agent-ws-server.ts)  
**Lines:** 114–118

### Phân tích
Code hiện tại dùng `pendingSlots.has(token)` — ephemeral in-memory. HLD mô tả `orca_agent_tokens` DB table với long-lived tokens.

**Hai loại agent cần support:**
1. **Dev Server Agent** (ephemeral): temp token từ DevServerRelayBridge → connect 1 lần → slot consumed ✅
2. **Custom AI Agent** (persistent): long-lived token từ DB → connect bất kỳ lúc ❌ (chưa implement)

### Fix — Thêm DB lookup fallback:
```typescript
// src/main/dev-server/agent-ws-server.ts
// Thêm dependency IAgentTokenRepository:

export class AgentWebSocketServer {
  constructor(
    orcaVersion: string,
    private readonly tokenRepo?: IAgentTokenRepository  // optional — existing code không break
  ) {
    this.orcaVersion = orcaVersion
  }

  private handleConnection(ws: WebSocket): void {
    runOrcaReceiverHandshake(
      ws,
      async (rawToken) => {
        // 1. Check in-memory slot (Dev Server ephemeral token)
        const hash = createHash('sha256').update(rawToken).digest('hex')
        if (this.pendingSlots.has(hash)) return true

        // 2. Check DB (Custom AI Agent long-lived token)
        if (this.tokenRepo) {
          const dbToken = await this.tokenRepo.findActiveByHash(hash)
          if (dbToken) {
            await this.tokenRepo.updateLastSeen(dbToken.id)
            return true
          }
        }

        return false
      },
      this.orcaVersion
    )
  }
}

// src/main/repositories/agent-token-repository.ts (NEW):
export interface IAgentTokenRepository {
  findActiveByHash(tokenHash: string): Promise<AgentTokenRecord | null>
  updateLastSeen(id: string): Promise<void>
  create(record: Omit<AgentTokenRecord, 'id' | 'createdAt'>): Promise<AgentTokenRecord>
  deactivate(id: string): Promise<void>
}
```

---

## Tóm tắt thay đổi

| Bug | File | Lines | Thay đổi |
|-----|------|-------|---------|
| AWS-004 | `agent-token-routes.ts` | 36–44 | Xóa `X-Orca-Admin` bypass, block nếu không có secret |
| AWS-002 | `agent-token-routes.ts` + `agent-ws-server.ts` | 113 + 116 | SHA-256 hash token trước khi store |
| AWS-003 | `agent-token-routes.ts` | 111 | Thêm `permanent` option: TTL 30 ngày |
| AWS-001 | `dev-server-relay-bridge.ts` | 254 | Port `6769` → `6768` (xem BE-TM-004) |
| BE-AWS-001 | `agent-ws-server.ts` | 114–118 | Thêm DB lookup fallback cho long-lived tokens |
