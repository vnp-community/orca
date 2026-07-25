# SOL-006: Team-based Access Control (RBAC) — Backend Solution

**CR:** [CR-006](../../../../../../../docs/crs/v1/remote-server/CR-006-team-rbac.md)  
**Backend TDD refs:** `04-rpc-server.md`, `07-runtime-service.md`, `06-persistence.md`  
**Depends on:** SOL-001, SOL-002  
**Effort:** Large (5–7 ngày full RBAC) | Small (1–2 ngày immediate workaround)  
**Phase:** 3 (full) | Immediate (workaround)

---

## 1. Phân tích security model hiện tại

Từ `TDD-04 (RPC Server)`:

```typescript
// src/main/runtime/runtime-rpc.ts
// Auth model hiện tại:
// - Single auth token per Orca instance
// - Token sinh ra khi start → stored in PersistedState
// - Mọi client với token đúng → full access

// Token type: random 32-byte hex string
// Stored in: PersistedState.rpcAuthToken
// Scope: ALL methods, ALL SSH targets, ALL repos
```

Từ `TDD-07 (Runtime Service)`:
- `device-registry.ts`: track connected devices (deviceId + token)
- **Không có**: user identity, team membership, per-target access control

---

## 2. Giải pháp: Hai giai đoạn

### Phase 1 — Immediate: Multi-Instance Isolation

**Không cần code change**. Chạy được ngay với infrastructure hiện có.

#### 2.1 Nguyên tắc

```
Mỗi Orca instance = isolated namespace:
  - Pairing URL riêng (token riêng)
  - SSH targets riêng (SQLite riêng)
  - DevOps phân phối URL riêng cho từng team
```

#### 2.2 Backend changes cho multi-instance support

Hiện tại Orca dùng hardcoded port `6768`. Cần cho phép override qua env:

```typescript
// src/main/runtime/runtime-rpc.ts — MODIFY (nếu chưa có)

const WS_PORT = parseInt(process.env.ORCA_PORT ?? '6768', 10)
const ORCA_DOMAIN = process.env.ORCA_DOMAIN ?? undefined  // for pairing URL generation

// Pairing URL generation: dùng ORCA_DOMAIN nếu set
function buildPairingUrl(token: string): string {
  const domain = ORCA_DOMAIN ?? `localhost:${WS_PORT}`
  return `orca://pair?code=${encodeBase64UrlPairingOffer({ endpoint: `wss://${domain}`, token })}`
}
```

#### 2.3 Per-instance SSH target isolation

Mỗi Orca instance cần data directory riêng:

```typescript
// src/main/persistence.ts — VERIFY
// getStorePath() đọc từ: process.env.ORCA_DATA_DIR ?? app.getPath('userData')
// → Nếu đặt ORCA_DATA_DIR khác nhau cho mỗi instance → data hoàn toàn isolated

// Hiện tại app.getPath('userData') là fixed per-OS path
// Cần: respect ORCA_DATA_DIR env var
```

---

### Phase 2 — Long-term: Native RBAC Layer

#### 2.4 New types

```typescript
// src/shared/rbac-types.ts — NEW FILE

export type OrcaIdentityProvider = 'github' | 'google' | 'keycloak' | 'none'

export type OrcaSsoConfig = {
  provider: OrcaIdentityProvider
  clientId: string
  clientSecret?: string       // từ env var
  discoveryUrl?: string       // OIDC discovery endpoint (Keycloak, etc.)
  allowedOrg?: string         // GitHub: only allow users from this org
  allowedDomain?: string      // Google: only allow emails from domain
  redirectUri?: string        // OAuth callback URL
}

export type OrcaUser = {
  id: string                  // unique per provider
  email: string
  name: string
  avatarUrl?: string
  teams: string[]             // from OIDC claims hoặc GitHub teams
  projects: string[]          // derived từ teams
  role: 'developer' | 'lead' | 'admin'
  provider: OrcaIdentityProvider
  providerUserId: string      // original provider user ID
  accessToken?: string        // short-lived (không persist)
}

export type OrcaAccessPolicy = {
  id: string
  name: string
  // Who this policy applies to:
  teams?: string[]            // e.g. ["backend"]
  roles?: string[]            // e.g. ["admin"]
  users?: string[]            // specific email addresses
  // What they can access:
  allowedServers: '*' | string[]  // fleetId list or '*' for all
  allowedProjects: '*' | string[]
  // Permissions:
  agentTrust?: 'minimal' | 'standard' | 'full'
  canCreateWorktrees?: boolean
  canDeleteWorktrees?: boolean
  canAccessProduction?: boolean
}

export type ScopedPairingToken = {
  token: string
  userId: string
  userEmail: string
  teams: string[]
  allowedServerIds: string[]  // resolved from policies
  agentTrust: 'minimal' | 'standard' | 'full'
  expiresAt: number           // Unix ms
}
```

#### 2.5 RPC Server: Token Scope Enforcement

```typescript
// src/main/runtime/runtime-rpc.ts — MODIFY

// Hiện tại: single authToken check
// Sau: check ScopedPairingToken, enforce allowed servers

class OrcaRuntimeRpcServer {
  // Existing auth check:
  private isAuthenticated(token: string): boolean {
    return token === this.persistedState.rpcAuthToken
  }

  // NEW: scope-aware auth
  private getTokenScope(token: string): ScopedPairingToken | 'full-access' | null {
    // Check full-access token (admin/backward compat)
    if (token === this.persistedState.rpcAuthToken) return 'full-access'

    // Check scoped tokens
    const scoped = this.deviceRegistry.getScopedToken(token)
    if (!scoped || scoped.expiresAt < Date.now()) return null

    return scoped
  }

  // Method authorization:
  private isMethodAllowed(scope: ScopedPairingToken | 'full-access', method: string, params: unknown): boolean {
    if (scope === 'full-access') return true

    // SSH target access control:
    if (method.startsWith('ssh.') || method.startsWith('worktree.')) {
      const targetId = (params as any)?.targetId ?? (params as any)?.worktree?.sshTargetId
      if (targetId && !scope.allowedServerIds.includes(targetId)) {
        return false
      }
    }

    // Agent trust check (for methods that launch agents):
    if (method === 'session.tabs.createTerminal') {
      const requestedTrust = (params as any)?.agentTrust
      if (requestedTrust && !isAllowedTrust(scope.agentTrust, requestedTrust)) {
        return false
      }
    }

    return true
  }
}
```

#### 2.6 OIDC/SSO Handler

```typescript
// src/main/auth/oidc-handler.ts — NEW FILE

import { randomBytes } from 'crypto'

class OidcAuthHandler {
  async startAuthFlow(config: OrcaSsoConfig): Promise<{ authUrl: string; state: string }> {
    const state = randomBytes(16).toString('hex')
    const nonce = randomBytes(16).toString('hex')

    let authUrl: string
    switch (config.provider) {
      case 'github':
        authUrl = `https://github.com/login/oauth/authorize?` +
          `client_id=${config.clientId}&` +
          `scope=read:user,read:org&` +
          `state=${state}`
        break

      case 'google':
        authUrl = `https://accounts.google.com/o/oauth2/v2/auth?` +
          `client_id=${config.clientId}&` +
          `response_type=code&` +
          `scope=openid+email+profile&` +
          `redirect_uri=${encodeURIComponent(config.redirectUri!)}&` +
          `state=${state}&nonce=${nonce}`
        break

      case 'keycloak':
        // Discovery: fetch ${discoveryUrl}/.well-known/openid-configuration
        const discovery = await fetchOidcDiscovery(config.discoveryUrl!)
        authUrl = `${discovery.authorization_endpoint}?` +
          `client_id=${config.clientId}&` +
          `response_type=code&scope=openid+email+profile+groups&` +
          `redirect_uri=${encodeURIComponent(config.redirectUri!)}&` +
          `state=${state}`
        break

      default:
        throw new Error(`Unsupported SSO provider: ${config.provider}`)
    }

    // Store pending state
    this.pendingStates.set(state, { nonce, startedAt: Date.now() })
    return { authUrl, state }
  }

  async handleCallback(code: string, state: string): Promise<OrcaUser> {
    // Verify state
    if (!this.pendingStates.has(state)) throw new Error('Invalid OAuth state')
    this.pendingStates.delete(state)

    // Exchange code for tokens
    const tokens = await this.exchangeCode(code)

    // Fetch user info + team membership
    const user = await this.fetchUserInfo(tokens.access_token)

    return user
  }

  async resolveUserPolicies(user: OrcaUser, policies: OrcaAccessPolicy[]): Promise<{
    allowedServerIds: string[]
    agentTrust: 'minimal' | 'standard' | 'full'
  }> {
    const matchingPolicies = policies.filter(policy => {
      if (policy.roles?.includes(user.role)) return true
      if (policy.teams?.some(t => user.teams.includes(t))) return true
      if (policy.users?.includes(user.email)) return true
      return false
    })

    // Collect allowed servers (union across all matching policies)
    let allowedServerIds: string[] = []
    let maxTrust: 'minimal' | 'standard' | 'full' = 'minimal'

    for (const policy of matchingPolicies) {
      if (policy.allowedServers === '*') {
        allowedServerIds = ['*']  // all servers
      } else if (!allowedServerIds.includes('*')) {
        allowedServerIds.push(...policy.allowedServers)
      }

      if (policy.agentTrust) {
        maxTrust = maxAgentTrust(maxTrust, policy.agentTrust)
      }
    }

    return {
      allowedServerIds: [...new Set(allowedServerIds)],
      agentTrust: maxTrust,
    }
  }
}
```

#### 2.7 Scoped Token Generation

```typescript
// src/main/runtime/device-registry.ts — MODIFY

class DeviceRegistry {
  // Existing: track connected devices
  // Add: scoped token management

  generateScopedToken(user: OrcaUser, allowedServerIds: string[], agentTrust: 'minimal' | 'standard' | 'full'): ScopedPairingToken {
    const token: ScopedPairingToken = {
      token: randomBytes(32).toString('hex'),
      userId: user.id,
      userEmail: user.email,
      teams: user.teams,
      allowedServerIds,
      agentTrust,
      expiresAt: Date.now() + 24 * 60 * 60 * 1000,  // 24h
    }
    this.scopedTokens.set(token.token, token)
    return token
  }

  getScopedToken(token: string): ScopedPairingToken | null {
    const found = this.scopedTokens.get(token)
    if (!found || found.expiresAt < Date.now()) {
      this.scopedTokens.delete(token)
      return null
    }
    return found
  }

  revokeScopedToken(token: string): void {
    this.scopedTokens.delete(token)
  }

  revokeAllUserTokens(userId: string): void {
    for (const [key, value] of this.scopedTokens.entries()) {
      if (value.userId === userId) {
        this.scopedTokens.delete(key)
      }
    }
  }
}
```

#### 2.8 Fleet Access Config trong `orca-fleet.yaml`

```typescript
// Được parse trong fleet-config-parser.ts (đã có schema slot)
// FleetConfig.access section:

type FleetAccess = {
  sso?: OrcaSsoConfig
  policies?: OrcaAccessPolicy[]
}
```

#### 2.9 Settings extension

```typescript
// src/shared/types.ts — ADD to GlobalSettings
type GlobalSettings = {
  // ... existing ...
  ssoConfig?: OrcaSsoConfig         // null = no SSO
  accessPolicies?: OrcaAccessPolicy[]
  requireAuthentication?: boolean    // if true, block unauthenticated connections
}
```

---

## 3. Audit Logging

```typescript
// src/main/audit/audit-log.ts — NEW FILE

type AuditEvent = {
  timestamp: number
  userId?: string
  userEmail?: string
  action: 'connect' | 'disconnect' | 'create-worktree' | 'delete-worktree' | 'access-denied'
  targetId?: string
  targetLabel?: string
  ip?: string
  details?: Record<string, unknown>
}

class AuditLog {
  // Append to: ~/.config/orca/audit.log (NDJSON format)
  async record(event: AuditEvent): Promise<void> {
    const line = JSON.stringify({ ...event, timestamp: Date.now() })
    await fs.appendFile(this.logPath, line + '\n')
  }

  async query(filter: Partial<AuditEvent>, limit = 100): Promise<AuditEvent[]> {
    // Read + filter last N lines
  }
}
```

---

## 4. Implementation Roadmap

```
PHASE 1 — Immediate (1–2 ngày, không cần code):
  ✅ Multiple docker-compose services (orca-backend, orca-ai, orca-claw)
  ✅ Separate ORCA_DATA_DIR per instance (if not already supported)
  ✅ Nginx vhosts per team
  ✅ Distribute pairing URLs per team via DevOps

PHASE 2 — Short-term (3–5 ngày backend):
  [1] Add ORCA_PORT + ORCA_DOMAIN env var support in runtime-rpc.ts
  [2] Add ORCA_DATA_DIR env var support in persistence.ts
  [3] Add rbac-types.ts
  [4] Add fleet access config parsing in fleet-config-parser.ts
  [5] Add scoped token in device-registry.ts
  [6] Add scope enforcement in runtime-rpc.ts
  [7] Add audit-log.ts

PHASE 3 — Enterprise (5–7 ngày):
  [8] Add oidc-handler.ts (GitHub OAuth first)
  [9] Add SSO login flow in web renderer
  [10] Admin UI: manage users, policies
  [11] OIDC for Keycloak, Google
```

---

## 5. Files cần thay đổi

| File | Action | Phase |
|------|--------|-------|
| `src/main/runtime/runtime-rpc.ts` | MODIFY (ORCA_PORT env) | 1 |
| `src/main/persistence.ts` | MODIFY (ORCA_DATA_DIR env) | 1 |
| `src/shared/rbac-types.ts` | **NEW** | 2 |
| `src/main/auth/oidc-handler.ts` | **NEW** | 3 |
| `src/main/runtime/device-registry.ts` | MODIFY (scoped tokens) | 2 |
| `src/main/audit/audit-log.ts` | **NEW** | 2 |
| `src/main/ssh/fleet-config-parser.ts` | MODIFY (access section) | 2 |
| `src/shared/types.ts` | MODIFY (GlobalSettings) | 2 |

---

## 6. Implementation Status

> **⚡ PARTIAL — Phase 1 & 2 Complete, Phase 3 Pending**  
> Ngày: 2026-07-22

### Phase 1 — Multi-instance Isolation ✅ Done

| File | Status | Chi tiết |
|------|--------|---------|
| [`src/main/runtime/runtime-rpc.ts`](../../../../../src/main/runtime/runtime-rpc.ts) | ✅ Done | `ORCA_PORT` env var overrides WebSocket port; `ORCA_DOMAIN` overrides pairing endpoint hostname |
| [`src/main/persistence.ts`](../../../../../src/main/persistence.ts) | ✅ Done | `ORCA_DATA_DIR` env var overrides userData path in `initDataPath()` |

### Phase 2 — Scoped Access Control ✅ Done

| File | Status | Chi tiết |
|------|--------|---------|
| [`src/shared/rbac-types.ts`](../../../../../src/shared/rbac-types.ts) | ✅ Done | **NEW** — `OrcaIdentityProvider`, `OrcaSsoConfig`, `OrcaUser`, `OrcaAccessPolicy`, `ScopedPairingToken`, `resolveUserPermissions()` |
| [`src/main/runtime/device-registry.ts`](../../../../../src/main/runtime/device-registry.ts) | ✅ Done | `generateScopedToken()`, `getScopedToken()` (expiry), `revokeScopedToken()`, `revokeAllUserTokens()`, `pruneExpiredTokens()` |
| [`src/main/runtime/runtime-rpc.ts`](../../../../../src/main/runtime/runtime-rpc.ts) | ✅ Done | Scoped token auth check trong `parseAndValidateRpcRequest()` + `isScopedMethodAllowed()` guard |
| [`src/main/audit/audit-log.ts`](../../../../../src/main/audit/audit-log.ts) | ✅ Done | **NEW** — NDJSON audit log, `record()` + `query()`, `ORCA_DATA_DIR` support |

### Phase 3 — OIDC/SSO ⏳ Pending (Enterprise)

| File | Status | Chi tiết |
|------|--------|---------|
| `src/main/auth/oidc-handler.ts` | ⏳ Pending | GitHub OAuth first, then Google/Keycloak |
| SSO login flow in web renderer | ⏳ Pending | Cần OIDC handler trước |
| Admin UI: users & policies | ⏳ Pending | Cần SSO flow trước |

### Deviation từ design gốc

> **Phase 2 access section trong fleet-config-parser** chưa implement — `orca-fleet.yaml` chưa support `access:` block. Scoped tokens hiện được issue manually (programmatic API), chưa có UI.
>
> **GlobalSettings** chưa được extend với fleet RBAC fields.

### Notes

- **TASK-022** (env var support ORCA_PORT/DOMAIN/DATA_DIR): ✅ Done  
- **TASK-021** (rbac-types.ts): ✅ Done  
- **TASK-023** (scoped tokens in device-registry): ✅ Done  
- **TASK-024** (scope enforcement in runtime-rpc): ✅ Done  
- **TASK-025** (audit-log.ts): ✅ Done
