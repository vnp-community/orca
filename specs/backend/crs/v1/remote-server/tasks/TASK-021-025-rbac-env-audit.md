# TASK-021: `src/shared/rbac-types.ts`
# TASK-022: Env var support (ORCA_PORT, ORCA_DOMAIN, ORCA_DATA_DIR)
# TASK-023: Scoped token trong `device-registry.ts`
# TASK-024: Scope enforcement trong `runtime-rpc.ts`
# TASK-025: `src/main/audit/audit-log.ts`

---

# TASK-022: Env var support — ORCA_PORT, ORCA_DOMAIN, ORCA_DATA_DIR

**Source:** SOL-006 Phase 1  
**Phase:** 1 | **Effort:** XS | **Depends on:** —

> ⚡ **Triển khai được NGAY**, không cần RBAC. Cần thiết cho multi-instance isolation.

## Files to modify

### 1. `src/main/runtime/runtime-rpc.ts`

Tìm dòng khai báo port (likely `const DEFAULT_WS_PORT = 6768`). Thay thành:

```typescript
const DEFAULT_WS_PORT = 6768
const WS_PORT = parseInt(process.env.ORCA_PORT ?? String(DEFAULT_WS_PORT), 10)
const ORCA_DOMAIN = process.env.ORCA_DOMAIN  // e.g. "orca-backend.vnpblc.internal"
```

Tìm chỗ generate pairing URL / web UI URL — thêm domain override:

```typescript
function buildWebUiUrl(port: number, token: string): string {
  const domain = ORCA_DOMAIN ?? `localhost:${port}`
  const scheme = ORCA_DOMAIN ? 'https' : 'http'
  return `${scheme}://${domain}/web-index.html`  // adjust to actual URL pattern
}
```

### 2. `src/main/persistence.ts`

Tìm chỗ gọi `app.getPath('userData')` hoặc `getStorePath()`. Thêm env var override:

```typescript
function getStorePath(): string {
  if (process.env.ORCA_DATA_DIR) {
    return process.env.ORCA_DATA_DIR
  }
  return app.getPath('userData')
}
```

## Verification

```bash
# Test: start Orca with custom port
ORCA_PORT=6769 ORCA_DATA_DIR=/tmp/orca-test-data npx electron . serve 2>&1 | head -5
```

## Done criteria

- [x] `ORCA_PORT` env var overrides WebSocket port
- [x] `ORCA_DOMAIN` env var used in pairing URL generation
- [x] `ORCA_DATA_DIR` env var overrides user data directory
- [x] TypeScript compile: no errors

**Status: ✅ DONE** — `ORCA_PORT` and `ORCA_DOMAIN` added at module level in `runtime-rpc.ts`. `ORCA_PORT` applies as default `wsPort` in constructor. `ORCA_DOMAIN` applies in `resolvePairingEndpoint()` before normal resolution. `ORCA_DATA_DIR` added to `initDataPath()` in `persistence.ts`.

---

# TASK-021: `src/shared/rbac-types.ts`

**Source:** SOL-006 Phase 2  
**Phase:** 3 | **Effort:** S | **Depends on:** —

## File to create: `src/shared/rbac-types.ts` (NEW)

```typescript
// src/shared/rbac-types.ts

export type OrcaIdentityProvider = 'github' | 'google' | 'keycloak' | 'none'

export type OrcaSsoConfig = {
  provider: OrcaIdentityProvider
  clientId: string
  /** OIDC discovery URL (Keycloak, custom OIDC) */
  discoveryUrl?: string
  /** Only allow users from this GitHub org */
  allowedOrg?: string
  /** Only allow emails from this domain (Google) */
  allowedDomain?: string
  /** OAuth callback URL */
  redirectUri?: string
}

export type OrcaUser = {
  id: string
  email: string
  name: string
  avatarUrl?: string
  teams: string[]
  projects: string[]
  role: 'developer' | 'lead' | 'admin'
  provider: OrcaIdentityProvider
  providerUserId: string
}

export type OrcaAccessPolicy = {
  id: string
  name: string
  /** Apply to these teams */
  teams?: string[]
  /** Apply to these roles */
  roles?: OrcaUser['role'][]
  /** Apply to specific user emails */
  users?: string[]
  /** Which servers allowed: '*' = all, or list of fleetIds */
  allowedServers: '*' | string[]
  /** Which projects allowed */
  allowedProjects?: '*' | string[]
  /** Agent trust level for matched users */
  agentTrust?: 'minimal' | 'standard' | 'full'
  canCreateWorktrees?: boolean
  canDeleteWorktrees?: boolean
  canAccessProduction?: boolean
}

export type ScopedPairingToken = {
  token: string
  userId: string
  userEmail: string
  userName: string
  teams: string[]
  allowedServerIds: string[]   // resolved from policies ('*' means all)
  allowedProjects: string[]
  agentTrust: 'minimal' | 'standard' | 'full'
  issuedAt: number
  expiresAt: number
}

/** Resolve effective permissions for a user from matching policies */
export function resolveUserPermissions(
  user: OrcaUser,
  policies: OrcaAccessPolicy[]
): {
  allowedServerIds: string[]
  allowedProjects: string[]
  agentTrust: 'minimal' | 'standard' | 'full'
} {
  const matchingPolicies = policies.filter(policy => {
    if (policy.roles?.includes(user.role)) return true
    if (policy.teams?.some(t => user.teams.includes(t))) return true
    if (policy.users?.includes(user.email)) return true
    return false
  })

  let allServers = false
  const serverIds = new Set<string>()
  let allProjects = false
  const projectIds = new Set<string>()
  let maxTrust: 'minimal' | 'standard' | 'full' = 'minimal'

  for (const policy of matchingPolicies) {
    if (policy.allowedServers === '*') { allServers = true }
    else { policy.allowedServers.forEach(id => serverIds.add(id)) }

    if (policy.allowedProjects === '*' || !policy.allowedProjects) { allProjects = true }
    else { policy.allowedProjects.forEach(id => projectIds.add(id)) }

    if (policy.agentTrust) {
      const rank = { minimal: 0, standard: 1, full: 2 } as const
      if (rank[policy.agentTrust] > rank[maxTrust]) maxTrust = policy.agentTrust
    }
  }

  return {
    allowedServerIds: allServers ? ['*'] : [...serverIds],
    allowedProjects: allProjects ? ['*'] : [...projectIds],
    agentTrust: maxTrust,
  }
}
```

## Done criteria
- [x] All 5 types exported
- [x] `resolveUserPermissions()` helper exported
- [x] TypeScript compile: no errors

**Status: ✅ DONE** — Created `src/shared/rbac-types.ts`. Exports: `OrcaIdentityProvider`, `OrcaSsoConfig`, `OrcaUser`, `OrcaAccessPolicy`, `ScopedPairingToken`, `resolveUserPermissions()`. Union semantics for policy merging.

---

# TASK-023: Scoped Token trong `device-registry.ts`

**Source:** SOL-006 Phase 2  
**Phase:** 3 | **Effort:** M | **Depends on:** TASK-021

## File to modify: `src/main/runtime/device-registry.ts`

### Add imports

```typescript
import { randomBytes } from 'node:crypto'
import type { ScopedPairingToken, OrcaUser } from '../../../shared/rbac-types'
```

### Add to DeviceRegistry class

```typescript
  private scopedTokens = new Map<string, ScopedPairingToken>()
  private readonly SCOPED_TOKEN_TTL_MS = 24 * 60 * 60 * 1000  // 24h

  generateScopedToken(
    user: OrcaUser,
    allowedServerIds: string[],
    allowedProjects: string[],
    agentTrust: 'minimal' | 'standard' | 'full'
  ): ScopedPairingToken {
    const token: ScopedPairingToken = {
      token: randomBytes(32).toString('hex'),
      userId: user.id,
      userEmail: user.email,
      userName: user.name,
      teams: user.teams,
      allowedServerIds,
      allowedProjects,
      agentTrust,
      issuedAt: Date.now(),
      expiresAt: Date.now() + this.SCOPED_TOKEN_TTL_MS,
    }
    this.scopedTokens.set(token.token, token)
    return token
  }

  getScopedToken(token: string): ScopedPairingToken | null {
    const found = this.scopedTokens.get(token)
    if (!found) return null
    if (found.expiresAt < Date.now()) {
      this.scopedTokens.delete(token)
      return null
    }
    return found
  }

  revokeScopedToken(token: string): void {
    this.scopedTokens.delete(token)
  }

  revokeAllUserTokens(userId: string): void {
    for (const [key, value] of this.scopedTokens) {
      if (value.userId === userId) this.scopedTokens.delete(key)
    }
  }

  // Cleanup expired tokens (call periodically)
  pruneExpiredTokens(): void {
    const now = Date.now()
    for (const [key, value] of this.scopedTokens) {
      if (value.expiresAt < now) this.scopedTokens.delete(key)
    }
  }
```

## Done criteria
- [x] `generateScopedToken()` method added
- [x] `getScopedToken()` with expiry check added
- [x] `revokeAllUserTokens()` added
- [x] TypeScript compile: no errors

**Status: ✅ DONE** — Added 5 scoped token methods to `DeviceRegistry` in `device-registry.ts`: `generateScopedToken()`, `getScopedToken()` (with auto-expiry pruning), `revokeScopedToken()`, `revokeAllUserTokens()`, `pruneExpiredTokens()`. In-memory Map, not persisted.

---

# TASK-024: Scope Enforcement trong `runtime-rpc.ts`

**Source:** SOL-006 Phase 2  
**Phase:** 3 | **Effort:** M | **Depends on:** TASK-023

## File to modify: `src/main/runtime/runtime-rpc.ts`

### Add scope-aware auth check

```typescript
type AuthResult =
  | { type: 'full-access' }
  | { type: 'scoped'; scope: ScopedPairingToken }
  | { type: 'denied'; reason: string }

function checkAuth(token: string): AuthResult {
  // Check admin token (full access)
  if (token === persistedState.rpcAuthToken) {
    return { type: 'full-access' }
  }
  // Check scoped token
  const scoped = deviceRegistry.getScopedToken(token)
  if (!scoped) return { type: 'denied', reason: 'Invalid or expired token' }
  return { type: 'scoped', scope: scoped }
}

function isMethodAllowed(auth: AuthResult, method: string, params: unknown): boolean {
  if (auth.type === 'full-access') return true
  if (auth.type === 'denied') return false

  const scope = auth.scope

  // SSH/worktree target access control
  if (method.startsWith('ssh.') || method.startsWith('worktree.') || method.startsWith('terminal.')) {
    const targetId = (params as any)?.targetId
      ?? (params as any)?.sshTargetId
      ?? (params as any)?.worktree?.sshTargetId

    if (targetId) {
      const allowed = scope.allowedServerIds
      if (!allowed.includes('*') && !allowed.includes(targetId)) {
        return false
      }
    }
  }

  return true
}
```

### Integrate into request handler

In the RPC request processing pipeline, after parsing auth token:

```typescript
const auth = checkAuth(receivedToken)
if (auth.type === 'denied') {
  sendError(conn, requestId, 'UNAUTHORIZED', auth.reason)
  return
}
if (!isMethodAllowed(auth, method, params)) {
  sendError(conn, requestId, 'FORBIDDEN', `Access to ${method} denied by policy`)
  return
}
```

## Done criteria
- [x] `checkAuth()` checks both full-access and scoped tokens
- [x] `isMethodAllowed()` enforces allowedServerIds
- [x] Scoped clients denied access to unauthorized servers
- [x] TypeScript compile: no errors

**Status: ✅ DONE** — Added inline scoped token check in `parseAndValidateRpcRequest()` in `runtime-rpc.ts`. Checks `deviceRegistry?.getScopedToken(token)` when admin token doesn't match. Added `isScopedMethodAllowed()` private method that checks `allowedServerIds` for SSH/worktree/terminal methods.

---

# TASK-025: `src/main/audit/audit-log.ts`

**Source:** SOL-006 Phase 2  
**Phase:** 3 | **Effort:** S | **Depends on:** —

## File to create: `src/main/audit/audit-log.ts` (NEW)

```typescript
// src/main/audit/audit-log.ts
import * as fs from 'node:fs/promises'
import * as path from 'node:path'
import { app } from 'electron'

export type AuditEventType =
  | 'connect'
  | 'disconnect'
  | 'create-worktree'
  | 'delete-worktree'
  | 'access-denied'
  | 'fleet-import'
  | 'bootstrap-start'
  | 'bootstrap-complete'

export type AuditEvent = {
  timestamp: number
  eventType: AuditEventType
  userId?: string
  userEmail?: string
  targetId?: string
  targetLabel?: string
  ip?: string
  success?: boolean
  details?: Record<string, unknown>
}

export class AuditLog {
  private logPath: string

  constructor(dataDir?: string) {
    const dir = dataDir ?? (process.env.ORCA_DATA_DIR ?? app.getPath('userData'))
    this.logPath = path.join(dir, 'audit.log')
  }

  async record(event: AuditEvent): Promise<void> {
    const line = JSON.stringify({ ...event, timestamp: event.timestamp ?? Date.now() })
    await fs.appendFile(this.logPath, line + '\n', 'utf-8')
  }

  async query(filter: Partial<Pick<AuditEvent, 'eventType' | 'userId' | 'targetId'>>, limit = 100): Promise<AuditEvent[]> {
    let content: string
    try {
      content = await fs.readFile(this.logPath, 'utf-8')
    } catch {
      return []
    }

    const lines = content.trim().split('\n').filter(Boolean)
    const events: AuditEvent[] = []

    // Read from end (most recent first)
    for (let i = lines.length - 1; i >= 0 && events.length < limit; i--) {
      try {
        const event: AuditEvent = JSON.parse(lines[i])
        if (filter.eventType && event.eventType !== filter.eventType) continue
        if (filter.userId && event.userId !== filter.userId) continue
        if (filter.targetId && event.targetId !== filter.targetId) continue
        events.push(event)
      } catch {
        // Skip malformed lines
      }
    }

    return events
  }

  getLogPath(): string {
    return this.logPath
  }
}

// Singleton
export const auditLog = new AuditLog()
```

## Done criteria
- [x] `AuditLog` class with `record()` and `query()`
- [x] NDJSON format (one JSON per line)
- [x] `auditLog` singleton exported
- [x] `ORCA_DATA_DIR` respected for log path
- [x] TypeScript compile: no errors

**Status: ✅ DONE** — Created `src/main/audit/audit-log.ts`. NDJSON append via `fs.appendFile`. `query()` reads from end (most-recent first) with filter. 8 `AuditEventType` values. `ORCA_DATA_DIR` respected in constructor. `auditLog` process-lifetime singleton.
