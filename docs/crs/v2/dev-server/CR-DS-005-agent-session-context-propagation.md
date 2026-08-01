# CR-DS-005 — Agent Session Context Propagation

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-DS-005 |
| **Tên** | Session Context Propagation — Gateway to Agent |
| **Loại** | Security & Data Design |
| **Priority** | P0 — Critical |
| **Phiên bản** | v6.0 |
| **Ngày tạo** | 2026-07-30 |
| **Trạng thái** | Proposed |
| **Phụ thuộc** | CR-DS-001, CR-DS-002, CR-DS-003 |
| **Tác động HLD** | security.md (Trust Boundary 5) |

---

## Vấn đề

Orca Backend Server là authority về:
- Ai là user (userId, role, email)
- User thuộc team/project nào
- User có quyền gì (RBAC)
- Profile settings đã resolved (F33)
- AI Provider account nào được dùng (F35)

Dev Server Agent **không có** auth layer riêng. Agent phải:
1. **Tin tưởng** context từ Gateway (sau khi verify Gateway identity)
2. **Không độc lập authenticate** users (không có local user DB)
3. **Enforce** path restrictions và resource limits từ context đó

---

## Context Propagation Model

### Thiết kế: Signed Execution Context

```typescript
// Gateway tạo context này cho mỗi RPC call
interface SignedExecutionContext {
  // Payload
  ctx: RpcExecutionContext      // user, project, profile, provider...
  
  // Security
  issuedAt: number              // unix timestamp ms
  expiresAt: number             // issuedAt + 30s (short-lived)
  gatewayId: string             // which gateway instance issued this
  signature: string             // HMAC-SHA256(JSON(ctx + times), sharedSecret)
}

interface RpcExecutionContext {
  // User identity
  userId: string
  userEmail: string             // used as git author email
  userName: string              // used as git author name
  userRole: 'admin' | 'lead' | 'developer'
  
  // Scope
  projectId: string
  projectRoot: string           // absolute path on THIS dev server (validated)
  teamId: string
  
  // Resolved profile (from F33 ProfileResolver)
  agentSettings: {
    trustPreset: 'minimal' | 'standard' | 'permissive'
    approvedModels: string[]     // agent cannot use models not in this list
    maxTokensPerSession: number
    disallowedCommands: string[] // shell step blacklist
  }
  shellSettings: {
    defaultShell: string
    pathAdditions: string[]
    envVars: Record<string, string>
  }
  integrationSettings: {
    githubOrg: string
    prTemplate: string
    ghConfigDir: string          // per-user ~/.orca/users/{userId}/.gh
  }
  
  // AI Provider (from F35 resolver)
  providerAccountId?: string
  
  // Audit
  requestId: string             // UUID, for end-to-end tracing
  gatewaySessionId: string      // user's gateway session
}
```

### Verification on Agent side

```typescript
class ContextVerifier {
  verify(signed: SignedExecutionContext): RpcExecutionContext {
    // 1. Check signature
    const expectedSig = HMAC_SHA256(
      JSON.stringify({ ctx: signed.ctx, issuedAt: signed.issuedAt, expiresAt: signed.expiresAt }),
      this.sharedSecret
    )
    if (expectedSig !== signed.signature) {
      throw new AgentError(4001, 'Context signature invalid')
    }
    
    // 2. Check expiry (short window to prevent replay)
    if (Date.now() > signed.expiresAt) {
      throw new AgentError(4001, 'Context expired')
    }
    
    // 3. Validate projectRoot is in allowedRoots
    const allowed = this.config.allowedRoots.some(root =>
      signed.ctx.projectRoot.startsWith(root)
    )
    if (!allowed) {
      throw new AgentError(4010, 'projectRoot not in allowed paths')
    }
    
    return signed.ctx
  }
}
```

---

## Per-User Isolation on Agent

Dù Agent không fork() như Gateway (F24), Agent vẫn enforce isolation:

### PTY Namespace

```typescript
// PTY session luôn được tạo với userId binding
interface PtySession {
  ptyId: string
  userId: string              // owner
  projectId: string
  workdir: string
  createdAt: number
  lastActiveAt: number
}

// PTY router: chỉ cho phép truy cập PTY của chính user
class AgentPtyRouter {
  write(ptyId: string, data: string, ctx: RpcExecutionContext) {
    const pty = this.sessions.get(ptyId)
    if (!pty) throw new AgentError(4004, 'PTY not found')
    
    // Admin có thể access all; developer chỉ access own
    if (ctx.userRole !== 'admin' && pty.userId !== ctx.userId) {
      throw new AgentError(4001, 'Access denied: PTY belongs to another user')
    }
    
    pty.write(data)
  }
}
```

### File Path Enforcement

```typescript
class SecureFs {
  validatePath(requestedPath: string, ctx: RpcExecutionContext): string {
    const resolved = path.resolve(requestedPath)
    const projectRoot = path.resolve(ctx.projectRoot)
    
    // Must be under projectRoot
    const relative = path.relative(projectRoot, resolved)
    if (relative.startsWith('..')) {
      throw new AgentError(4010, 'Path traversal attempt blocked')
    }
    
    // Must be under one of agent's allowedRoots
    const inAllowed = this.allowedRoots.some(r => resolved.startsWith(r))
    if (!inAllowed) {
      throw new AgentError(4010, 'Path not in allowed workspace roots')
    }
    
    return resolved
  }
}
```

### Git Author Injection

```typescript
// Every git commit uses ctx.userEmail + ctx.userName
// Prevents impersonation
async function gitCommit(repoPath: string, message: string, ctx: RpcExecutionContext) {
  await execFile('git', ['commit', '-m', message], {
    env: {
      ...process.env,
      GIT_AUTHOR_NAME: ctx.userName,
      GIT_AUTHOR_EMAIL: ctx.userEmail,
      GIT_COMMITTER_NAME: ctx.userName,
      GIT_COMMITTER_EMAIL: ctx.userEmail,
    },
    cwd: repoPath
  })
}
```

### AI Agent Environment

```typescript
// Agent spawn injects resolved profile as env vars
// Approved models check BEFORE spawning
function buildAgentEnv(ctx: RpcExecutionContext, accountId: string): NodeJS.ProcessEnv {
  const credentials = loadCredential(accountId)  // decrypt local .enc file
  
  // Validate model is approved
  if (ctx.agentSettings.approvedModels.length > 0) {
    const requestedModel = resolveModel(accountId)
    if (!ctx.agentSettings.approvedModels.includes(requestedModel)) {
      throw new AgentError(4001, `Model ${requestedModel} not in approved list`)
    }
  }
  
  return {
    ...process.env,
    // PATH additions from profile
    PATH: [...ctx.shellSettings.pathAdditions, process.env.PATH].join(':'),
    // Custom env vars from profile
    ...ctx.shellSettings.envVars,
    // Trust preset enforcement
    ...(ctx.agentSettings.trustPreset === 'minimal' ? {
      DISABLE_WRITE: '1', DISABLE_BASH: '1'
    } : {}),
    // AI Provider credentials
    ANTHROPIC_API_KEY: credentials.apiKey,   // or provider-specific
    // GitHub config dir (per-user isolation)
    GH_CONFIG_DIR: path.join(AGENT_DATA_DIR, 'users', ctx.userId, '.gh'),
  }
}
```

---

## Audit Log on Agent

Agent duy trì audit log riêng (append-only):

```typescript
interface AgentAuditEntry {
  requestId: string           // from ctx
  userId: string
  userEmail: string
  gatewaySessionId: string
  method: string              // RPC method called
  params: Record<string, any> // sanitized (no credentials)
  outcome: 'success' | 'error' | 'denied'
  errorCode?: number
  durationMs: number
  timestamp: number
}

// Sync audit entries về Gateway
// Gateway writes to orca_audit_log (Server DB)
// Agent keeps local copy for offline periods
```

---

## Trust Boundary 5: Gateway ↔ Agent

```
┌──────────────── TRUST BOUNDARY 5: Gateway-Agent ──────────────────┐
│                                                                     │
│  Orca Backend Server          →WS/SSH→    Dev Server Agent         │
│  - Authenticates users                    - Trusts signed ctx      │
│  - Resolves RBAC                          - Verifies HMAC          │
│  - Issues signed context                  - Enforces path limits   │
│  - Validates providerAccountId            - Runs actual operations │
│                                           - Emits audit events     │
│                                                                     │
│  Shared secret: scrypt(ORCA_GATEWAY_SECRET + agentId)              │
│  Context TTL: 30 seconds (prevents replay attacks)                 │
│  Transport: TLS 1.3 mandatory                                       │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Multi-Gateway Support (Cluster)

Khi có nhiều Gateway instances (horizontal scaling):

```typescript
// Context verification không cần shared session state
// vì context là self-contained + signed

// Shared secret phải sync giữa gateway instances:
// - Kubernetes Secret
// - HashiCorp Vault
// - AWS Secrets Manager

// Agent trusts bất kỳ gateway nào có đúng shared secret
// → stateless verification, không cần sticky sessions với agent
```

---

## Acceptance Criteria

- [ ] HMAC-SHA256 signature verification hoạt động < 1ms
- [ ] Expired context (> 30s) bị reject với error 4001
- [ ] PTY access denied khi userId không match (non-admin)
- [ ] Path traversal attempt blocked (error 4010)
- [ ] Git commits luôn dùng ctx.userEmail (không thể fake)
- [ ] Agent env từ resolvedProfile inject đúng
- [ ] approvedModels check trước khi spawn agent
- [ ] Audit log đầy đủ cho mọi RPC call
- [ ] Multi-gateway: agent accept context từ bất kỳ gateway có đúng secret
- [ ] Context replay prevention (expiresAt 30s window)
