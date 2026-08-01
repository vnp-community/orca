# SOLUTION: Agent WebSocket (agent-ws) Domain — Fix tất cả Bugs

**Domain:** agent-ws  
**TDD Reference:** TDD-11 (Web Server Mode), TDD-04 (RPC Server), TDD-05 (SSH Relay), TDD-13 (Dev Server Onboarding)  
**Files cần thay đổi:** `src/main/dev-server/agent-ws-server.ts`, `src/main/dev-server/ws-handshake.ts`, `src/main/auth/auth-middleware.ts`  
**Tổng số bugs:** 5 (AWS-001 ~ AWS-004, BE-AWS-001)

---

## Tổng quan phụ thuộc

```
BUG-AWS-001 (topology wrong) — độc lập, fix agent-ws-server.ts
BUG-AWS-002 (token not hashed) — phụ thuộc BE-AWS-001 (verify flow)
BUG-AWS-003 (token TTL too short) — phụ thuộc AWS-002 (token storage)
BUG-AWS-004 (x-orca-admin bypass) — độc lập, fix auth-middleware.ts
BUG-BE-AWS-001 (token verify in-memory) — phụ thuộc AWS-002
```

**Thứ tự fix:** `004 → 002 → BE-001 → 003 → 001`

---

## BUG-AWS-001 — Fix relay-websocket mode topology wrong

**Mức độ:** 🔴 HIGH  
**Root cause:** Topology sai — relay-websocket mode cho phép Orca connect ra ngoài thay vì agent connect vào.

### Fix — Đảm bảo direct-websocket mode là default (theo TDD-AG-03)

Theo TDD v5 `00-index.md §A.4`:
```
Mode 3 (direct-websocket): Agent chủ động kết nối OUTBOUND → wss://orca-gateway:6768/agent
```

```typescript
// src/main/dev-server/agent-ws-server.ts

/**
 * AgentWebSocketServer — nhận INBOUND connections từ Dev Server Agent.
 * Agent tự kết nối vào endpoint này theo Mode 3 (direct-websocket).
 * 
 * ĐÚNG topology:
 *   Dev Server Agent → wss://orca-server:6768/agent
 *   NOT: Orca Server → SSH → Agent WS
 */
export class AgentWebSocketServer {
  private readonly wss: WebSocketServer
  private readonly sessions = new Map<string, AgentSession>()

  constructor(
    private readonly httpServer: Server,
    private readonly authManager: AuthManager,
    private readonly log: Logger,
  ) {
    // Mount tại /agent endpoint — Agent kết nối vào đây
    this.wss = new WebSocketServer({ server: httpServer, path: '/agent' })
    this.wss.on('connection', (ws, req) => this.handleInboundConnection(ws, req))
  }

  private async handleInboundConnection(ws: WebSocket, req: IncomingMessage): Promise<void> {
    // Agent gửi agentToken trong query string hoặc header
    const token = this.extractAgentToken(req)
    if (!token) {
      ws.close(4401, 'Missing agent token')
      return
    }

    // Verify token (DB-based — xem BUG-BE-AWS-001 fix)
    const devServer = await this.verifyAgentToken(token)
    if (!devServer) {
      ws.close(4403, 'Invalid agent token')
      return
    }

    const session = new AgentSession(ws, devServer, this.log)
    this.sessions.set(devServer.id, session)
    this.log.info(`[AgentWS] Agent connected: devServerId=${devServer.id}`)

    ws.on('close', () => {
      this.sessions.delete(devServer.id)
      this.log.info(`[AgentWS] Agent disconnected: devServerId=${devServer.id}`)
    })
  }

  private extractAgentToken(req: IncomingMessage): string | null {
    // Priority: Query param → Authorization header
    const url = new URL(req.url ?? '', 'http://localhost')
    return url.searchParams.get('token')
      ?? req.headers['x-agent-token'] as string
      ?? null
  }
}
```

---

## BUG-AWS-002 — Fix token không được SHA-256 hash trước khi lưu

**Mức độ:** 🔴 CRITICAL (Security)  
**Root cause:** Agent token lưu plaintext trong DB → nếu DB bị lộ, attacker có thể giả mạo agent.

### Fix — Hash token bằng SHA-256 trước khi lưu

```typescript
// src/main/dev-server/agent-token-manager.ts (NEW hoặc extend existing)

import { createHash, randomBytes } from 'node:crypto'

/**
 * Generate và store agent token an toàn.
 * Token gửi cho agent là raw (plaintext).
 * Token lưu trong DB là SHA-256 hash của raw token.
 * 
 * Theo TDD-05 (SSH Relay §security model): "Bearer token raw → SHA-256 → stored hash so sánh"
 */
export class AgentTokenManager {
  /**
   * Tạo agent token mới.
   * @returns { rawToken: string, hashedToken: string }
   *   rawToken → gửi cho agent (qua secure channel — SSH hoặc env var)
   *   hashedToken → lưu vào DB
   */
  static generate(): { rawToken: string; hashedToken: string } {
    const rawToken = randomBytes(32).toString('hex')  // 64-char hex
    const hashedToken = createHash('sha256').update(rawToken).digest('hex')
    return { rawToken, hashedToken }
  }

  /**
   * Verify agent token bằng cách hash rồi so sánh với DB.
   */
  static hash(rawToken: string): string {
    return createHash('sha256').update(rawToken).digest('hex')
  }
}

// Sử dụng trong AgentWebSocketServer.verifyAgentToken():
private async verifyAgentToken(rawToken: string): Promise<DevServer | null> {
  const hashedToken = AgentTokenManager.hash(rawToken)
  // Query DB với hashedToken (không phải rawToken)
  return await this.devServerRepository.findByAgentTokenHash(hashedToken)
}

// Sử dụng khi provision dev server:
const { rawToken, hashedToken } = AgentTokenManager.generate()
await this.devServerRepository.setAgentTokenHash(devServerId, hashedToken)
// Gửi rawToken cho agent qua SSH:
await sshClient.exec(`echo 'ORCA_AGENT_TOKEN=${rawToken}' > /etc/orca-agent/env`)
```

---

## BUG-AWS-003 — Fix token TTL too short (60s) và không có refresh

**Mức độ:** 🟠 HIGH  
**Root cause:** Slot TTL 60s quá ngắn → agent thường xuyên mất kết nối nếu startup chậm.

### Fix — Persistent slot (không TTL) + Refresh mechanism

```typescript
// src/main/dev-server/agent-ws-server.ts

// TRƯỚC (slot-based với TTL 60s):
registerSlot(agentToken: string, ttlMs = 60_000): void {
  this.slots.set(agentToken, { expiresAt: Date.now() + ttlMs })
}

// SAU — DB-based persistent token (không cần slot):
// Token được lưu trong DB (xem fix AWS-002).
// Không cần slot registration — agent connect bất kỳ lúc nào với token từ DB.

// Nếu cần session refresh (long-lived connection):
export class AgentSession {
  private keepaliveTimer?: NodeJS.Timeout

  startKeepalive(intervalMs = 30_000): void {
    this.keepaliveTimer = setInterval(() => {
      if (this.ws.readyState === WebSocket.OPEN) {
        // Send keepalive frame theo wire protocol (TYPE=0x09)
        this.ws.send(encodeKeepaliveFrame())
      }
    }, intervalMs)
  }

  stop(): void {
    clearInterval(this.keepaliveTimer)
    this.ws.close()
  }
}

// Token rotation (nếu cần refresh):
// Sau N ngày, generate token mới và notify agent qua current WS connection:
async rotateAgentToken(devServerId: string): Promise<void> {
  const { rawToken, hashedToken } = AgentTokenManager.generate()
  await this.devServerRepository.setAgentTokenHash(devServerId, hashedToken)
  
  // Gửi token mới qua connection hiện tại
  const session = this.sessions.get(devServerId)
  if (session) {
    session.send({ type: 'agent.tokenRotated', newToken: rawToken })
  }
}
```

---

## BUG-AWS-004 — Fix `x-orca-admin` auth bypass

**Mức độ:** 🔴 CRITICAL (Security)  
**Root cause:** Header `x-orca-admin: true` bypass authentication — cho phép bất kỳ request nào là admin.

### Fix — Xóa hoặc thay bằng proper admin auth

```typescript
// src/main/auth/auth-middleware.ts

// TRƯỚC (VULNERABLE):
export function requireAuth(req: Request, res: Response, next: NextFunction): void {
  // BUG: Admin bypass header — XÓA NGAY
  if (req.headers['x-orca-admin'] === 'true') {
    req.orcaSession = { role: 'admin', userId: 'system' } as any
    return next()
  }
  // ... normal auth
}

// SAU — Xóa hoàn toàn x-orca-admin bypass:
export function requireAuth(req: Request, res: Response, next: NextFunction): void {
  // Không có bypass header
  const session = req.orcaSession  // populated bởi cookie/bearer token middleware
  if (!session) {
    res.status(401).json({ error: 'Unauthorized' })
    return
  }
  next()
}

export function requireAdmin(req: Request, res: Response, next: NextFunction): void {
  if (!req.orcaSession || req.orcaSession.role !== 'admin') {
    res.status(403).json({ error: 'Forbidden: admin required' })
    return
  }
  next()
}

// Admin auth phải qua proper login flow:
// POST /auth/local { email, password } → session cookie
// Sau đó admin routes kiểm tra req.orcaSession.role === 'admin'
```

---

## BUG-BE-AWS-001 — Fix agent token verify trong memory thay vì DB

**Mức độ:** 🔴 HIGH  
**Root cause:** Token được verify bằng in-memory Map → restart server = mất tất cả registered agents.

### Fix — Lưu và verify token trong DB

```typescript
// src/main/repositories/dev-server-repository.ts (thêm methods)

export interface IDevServerRepository extends IStateRepository {
  // ... existing methods ...

  /**
   * Lưu hashed agent token cho dev server.
   * Called khi provision dev server (token được generate và gửi cho agent).
   */
  setAgentTokenHash(devServerId: string, hashedToken: string): Promise<void>

  /**
   * Tìm dev server theo hashed agent token.
   * Called khi agent kết nối vào /agent endpoint.
   */
  findByAgentTokenHash(hashedToken: string): Promise<DevServer | null>
}

// src/main/repositories/sql-dev-server-repository.ts (implementation)
async setAgentTokenHash(devServerId: string, hashedToken: string): Promise<void> {
  await this.pool.withConnection((db) =>
    db.query(
      `UPDATE orca_dev_servers SET agent_token_hash = ?, agent_token_updated_at = ? WHERE id = ?`,
      [hashedToken, Date.now(), devServerId]
    )
  )
}

async findByAgentTokenHash(hashedToken: string): Promise<DevServer | null> {
  const rows = await this.pool.withConnection((db) =>
    db.query<DevServer>(
      `SELECT * FROM orca_dev_servers WHERE agent_token_hash = ? AND status != 'deleted'`,
      [hashedToken]
    )
  )
  return rows[0] ?? null
}

// Migration cần thêm column:
// ALTER TABLE orca_dev_servers ADD COLUMN agent_token_hash TEXT;
// ALTER TABLE orca_dev_servers ADD COLUMN agent_token_updated_at INTEGER;
```

---

## Tóm tắt file changes

| File | Action | Bug |
|------|--------|-----|
| `src/main/dev-server/agent-ws-server.ts` | Rewrite topology — inbound only | AWS-001 |
| `src/main/dev-server/agent-token-manager.ts` | NEW — SHA-256 hash utils | AWS-002 |
| `src/main/auth/auth-middleware.ts` | Remove `x-orca-admin` bypass | AWS-004 |
| `src/main/repositories/dev-server-repository.ts` | Add token hash methods | BE-AWS-001 |
| `src/main/db/migrations/0008_agent_token_hash.ts` | NEW migration | BE-AWS-001 |
| `src/main/dev-server/agent-ws-server.ts` | Replace slot TTL với persistent DB token | AWS-003 |

---

## Verification Plan

```bash
# Type check:
pnpm tsc --noEmit -p config/tsconfig.node.json

# Security tests:
# 1. Request với x-orca-admin: true → expect 401 (không còn bypass)
# 2. Agent connect với invalid token → expect ws.close(4403)
# 3. Agent connect với valid raw token → verify SHA-256 hash lookup in DB
# 4. Server restart → agent reconnect với same token → verify still works

# Unit tests:
pnpm vitest run src/main/auth/__tests__/auth-middleware.test.ts
pnpm vitest run src/main/dev-server/__tests__/agent-ws-server.test.ts
```
