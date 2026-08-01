# ADR-015 — Signed Execution Context for Gateway–Agent Trust

| Trường | Giá trị |
|--------|---------|
| **ID** | ADR-015 |
| **Trạng thái** | 🚧 Proposed |
| **Ngày** | 2026-07-30 |
| **HLD Ref** | security.md (Trust Boundary 5), C3.13, C4.11 |
| **CR Ref** | CR-DS-005 |
| **Code Ref** | `src/agent/rpc/context-verifier.ts`, `src/main/dev-server/signed-context-issuer.ts` |
| **Feature Ref** | F24 (per-user isolation), F32 (RBAC), F33 (profile), F35 (AI credentials) |
| **Liên quan** | ADR-013 (Dev Server Agent), ADR-014 (JSON-RPC Protocol), ADR-003 (per-user isolation) |

---

## Bối cảnh

### Vấn đề trust giữa Gateway và Agent

Trong kiến trúc v6.0 (ADR-013), Dev Server Agent là một **separate process** không có auth layer riêng — Agent không có user database, không có session management. Tất cả identity và permission information đến từ Gateway.

**Câu hỏi thiết kế cốt lõi:**

> *Làm sao Agent biết rằng một RPC call đến từ Gateway hợp lệ, với đúng userId và permissions, và không phải là replay attack hoặc MITM?*

**Các lựa chọn:**

| Option | Vấn đề |
|--------|--------|
| Trust mọi RPC từ Gateway connection | Nếu Gateway bị compromise → toàn bộ agent bị compromise |
| JWT token per-call | JWT requires shared key hoặc public key infrastructure; không carry full profile |
| Session-based (stateful) | Agent phải maintain session state → phức tạp, không phù hợp stateless dispatch |
| **Signed Execution Context (HMAC-SHA256)** ✅ | Short-lived, self-contained, verifiable, không cần session state |

---

## Quyết định

### Signed Execution Context

Mỗi RPC call từ Gateway xuống Agent phải kèm một **Signed Execution Context** — một data structure mang đầy đủ user identity, scope, và policy, được ký bởi Gateway.

```typescript
// Signed bởi Gateway, verified bởi Agent
interface SignedExecutionContext {
  // Payload — full execution context
  ctx: RpcExecutionContext

  // Timing — replay prevention
  issuedAt: number      // unix timestamp ms
  expiresAt: number     // issuedAt + 30_000 (30 seconds)

  // Issuer
  gatewayId: string     // which Gateway instance issued

  // Integrity
  signature: string     // HMAC-SHA256(canonical(ctx) + issuedAt + expiresAt, sharedSecret)
}

interface RpcExecutionContext {
  // User identity
  userId: string
  userEmail: string           // used as git author email
  userName: string            // used as git author name
  userRole: 'admin' | 'lead' | 'developer'

  // Scope
  projectId: string
  projectRoot: string         // absolute path on this dev server
  teamId: string

  // Policy — resolved from F33 ProfileResolver (on Gateway)
  agentSettings: {
    trustPreset: 'minimal' | 'standard' | 'permissive'
    approvedModels: string[]
    maxTokensPerSession: number
    disallowedCommands: string[]
  }
  shellSettings: {
    defaultShell: string
    pathAdditions: string[]
    envVars: Record<string, string>
  }
  integrationSettings: {
    githubOrg: string
    prTemplate: string
    ghConfigDir: string       // per-user: ~/.orca/users/{userId}/.gh
  }

  // AI Provider (from F35 ProviderResolver)
  providerAccountId?: string

  // Traceability
  requestId: string           // UUID v4 (end-to-end trace)
  gatewaySessionId: string    // user's gateway session ID
}
```

### HMAC-SHA256 Signing

```typescript
// Gateway — SignedContextIssuer
class SignedContextIssuer {
  issue(ctx: RpcExecutionContext): SignedExecutionContext {
    const issuedAt = Date.now()
    const expiresAt = issuedAt + 30_000  // 30s TTL

    // Canonical JSON (sorted keys) — prevents signature malleability
    const payload = JSON.stringify({
      ctx,
      issuedAt,
      expiresAt,
      gatewayId: this.gatewayId
    }, sortedKeys)

    const signature = createHmac('sha256', this.sharedSecret)
      .update(payload)
      .digest('hex')

    return { ctx, issuedAt, expiresAt, gatewayId: this.gatewayId, signature }
  }
}

// Agent — ContextVerifier
class ContextVerifier {
  verify(signed: SignedExecutionContext): RpcExecutionContext {
    // Step 1: Check expiry
    if (Date.now() > signed.expiresAt) {
      throw new AgentError(4001, 'Context expired')
    }

    // Step 2: Verify signature
    const payload = JSON.stringify({
      ctx: signed.ctx,
      issuedAt: signed.issuedAt,
      expiresAt: signed.expiresAt,
      gatewayId: signed.gatewayId
    }, sortedKeys)

    const expected = createHmac('sha256', this.sharedSecret)
      .update(payload)
      .digest('hex')

    if (!timingSafeEqual(Buffer.from(expected), Buffer.from(signed.signature))) {
      throw new AgentError(4001, 'Context signature invalid')
    }

    // Step 3: Validate projectRoot is in allowedRoots
    const inAllowed = this.config.allowedRoots.some(root =>
      signed.ctx.projectRoot.startsWith(root)
    )
    if (!inAllowed) {
      throw new AgentError(4010, 'projectRoot not in allowed paths')
    }

    return signed.ctx
  }
}
```

### Shared Secret Management

```
Secret derivation:
  sharedSecret = scrypt(ORCA_GATEWAY_SECRET + agentId, salt, 32)

Lý do scrypt (not raw secret):
  - Mỗi agent có secret riêng (agentId khác nhau)
  - Nếu 1 agent bị compromise → không ảnh hưởng agent khác
  - ORCA_GATEWAY_SECRET chỉ trên Gateway, không truyền cho agent
  - Agent nhận pre-derived secret trong registration

Storage on Agent:
  /etc/orca-agent/agent-secret   (0600, root only)
  Không được commit vào git
  Không được log

Storage on Gateway:
  DB: agent_registrations.secret_hash (bcrypt hash)
  Memory: derived secret per agentId (in-process, not logged)
```

### 30-Second TTL — Rationale

```
Tại sao 30 giây?

1. Ngắn đủ: Nếu context bị intercept, attacker chỉ có 30s window
2. Dài đủ: Multi-hop operations (Gateway → Agent → PTY → spawn → respond) vẫn OK
3. Clock skew: Accepted drift < 5s (tổng 25s thực tế)
4. Performance: Không cần nonce/replay DB lookup (stateless verify)

Không dùng nonce (một lần):
  - Cần Agent maintain nonce DB → stateful → phức tạp
  - 30s window đủ nhỏ để attack không practical
```

### Per-Operation Context

Context được issue **per-operation** (không phải per-session):

```typescript
// Mỗi RPC call có context riêng:
async function dispatchToAgent(agentId, method, params, user, project) {
  // 1. Resolve fresh profile (TTL 60s, may be cached)
  const profile = await profileResolver.resolve(user.id)

  // 2. Resolve provider for this project
  const providerAccountId = await providerResolver.resolve(user.id, project.id, method)

  // 3. Issue fresh signed context (30s TTL)
  const ctx = contextIssuer.issue({
    userId: user.id,
    userEmail: user.email,
    ...profile.resolved,
    projectRoot: project.repoPath,
    providerAccountId,
    requestId: uuid()
  })

  // 4. Dispatch with context
  return agentConnectionManager.dispatch(agentId, method, { ...params, _ctx: ctx })
}
```

### Multi-Gateway Support

```
Multiple Gateway instances có thể issue context cho cùng Agent:
  - Mỗi Gateway có ORCA_GATEWAY_SECRET giống nhau (shared secret)
  - Agent verify bất kỳ context nào có đúng signature
  - Stateless verification → không cần sticky sessions
  - Kubernetes: secret từ K8s Secret object

Rotation:
  - New ORCA_GATEWAY_SECRET → update tất cả Gateway instances
  - Old contexts (issued với old secret) expire trong 30s
  - Zero downtime rotation (30s overlap window)
```

---

## Lý do chọn

| Lựa chọn | Đánh giá |
|----------|---------| 
| **HMAC-SHA256 signed context (30s TTL)** ✅ | Stateless, self-contained, replay-safe, no external dependency |
| JWT (RS256) | Public key infra; JWT libraries; overkill; không carry custom fields tốt bằng |
| mTLS (mutual TLS) | Transport-level (không per-call); không carry execution context |
| API key per-user | Không expire → long-lived risk; không carry profile/projectRoot |
| Session ID only | Stateful on agent → phức tạp; không carry profile |
| No auth (trust connection) | Insecure: Gateway compromise = full agent compromise |

---

## Hậu quả

**Tích cực:**
- Stateless verification on Agent (no DB lookup per call)
- Self-contained: context carries all needed info (no agent→gateway roundtrip to verify)
- Replay-safe: 30s TTL
- Multi-gateway: no sticky sessions needed
- Per-agent secret: blast radius limited per agent

**Tiêu cực:**
- ~500 bytes overhead per RPC call (ctx size) — acceptable at 1-10 calls/sec
- Clock synchronization: Agent và Gateway clocks phải sync (NTP, ≤5s drift)
- Secret rotation: cần coordinate giữa Gateway instances (Kubernetes Secret)
- `projectRoot` validation: phải match agent's `allowedRoots` config — setup step required

---

## Security Properties

| Property | Guaranteed by |
|----------|-------------|
| Authenticity | HMAC-SHA256 signature |
| Integrity | Canonical JSON + signature covers all fields |
| Freshness | 30s TTL `expiresAt` |
| Replay prevention | Short TTL (no nonce store needed) |
| Path safety | Agent validates `projectRoot` in `allowedRoots` |
| Identity binding | `userId` in signed ctx → enforced by Agent (PTY ownership, git author) |
| Model enforcement | `approvedModels` in ctx → checked before agent spawn |

---

## Trạng thái Implementation

❌ Chưa implement (v6.0 proposed)  
🎯 `src/agent/rpc/context-verifier.ts`  
🎯 `src/main/dev-server/signed-context-issuer.ts`  
🎯 `src/main/dev-server/agent-dispatcher.ts` (issues context per-call)
