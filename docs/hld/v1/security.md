# Security Architecture

**Tài liệu:** Security Model của Orca  
**Tham chiếu:** SRS Section 4.3, logic/agent-orchestration/, logic/mobile-companion/  
**Cập nhật:** 2026-07-28 (corrections 2026-08-14, xem ghi chú dưới)

> **Ghi chú đối chiếu code (2026-08-14):** Phần lớn tài liệu này mô tả đúng ý định thiết kế và khớp code hiện tại. Các mục có gắn nhãn ✅ (implemented) / 🚧 (proposed, chưa implement) / ❌ (đã xác nhận sai/không tồn tại trong code) đã được đối chiếu trực tiếp `file:line` với `backend/`, `agent/` qua 4 audit: `audit/backend/backend-vs-design-review.md`, `audit/agent/connection-wire-protocol-vs-design-review.md`, `audit/agent/credential-fswatch-telemetry-vs-design-review.md`, `audit/agent/rpc-dispatch-lifecycle-vs-design-review.md`. Một số gap mà các audit đó phát hiện (2026-08-08) đã được fix trong code kể từ đó — nơi nào đã fix, ghi chú "Fixed" kèm bug ref thay vì mô tả như một lỗ hổng đang mở.

---

## 1. Security Boundaries

```
┌──────────────── TRUST BOUNDARY 1: Electron App ──────────────────┐
│                                                                    │
│  Renderer (SANDBOXED)    ←→    Main Process (NODE.JS TRUSTED)    │
│  - No nodeIntegration              - Full filesystem access        │
│  - contextBridge whitelist         - Shell exec                    │
│  - No direct shell access          - Network access                │
│                                    - Secret management             │
└───────────────────────────────────────────────────────────────────┘

┌──────────────── TRUST BOUNDARY 2: SSH ────────────────────────────┐
│                                                                    │
│  Desktop (LOCAL)          →SSH→   Remote Host                     │
│  - All sensitive data               - Relay runs as user (no root) │
│  - Credentials in keychain          - Token-authenticated relay    │
│  - Session tokens ephemeral         - Hash-verified binaries       │
└───────────────────────────────────────────────────────────────────┘

┌──────────────── TRUST BOUNDARY 3: Mobile ─────────────────────────┐
│                                                                    │
│  Desktop          ←→WebSocket E2E→   Mobile App                  │
│  TweetNaCl keypair                   TweetNaCl keypair            │
│  QR one-time token                   QR scan                      │
└───────────────────────────────────────────────────────────────────┘
```

---

## 2. Agent Trust Presets

Orca định nghĩa 3 trust levels cho AI agents:

| Tier | Tên | Mô tả | Env Vars |
|------|-----|-------|---------|
| 0 | **Minimal** | Chỉ read, không write, không exec | `DISABLE_WRITE=1`, `DISABLE_BASH=1` |
| 1 | **Standard** | Read + write trong worktree, exec hạn chế | Mặc định |
| 2 | **Full** | Toàn quyền (network, system calls) | `CLAUDE_TRUST_FULL=1` |

**Source:** `src/main/agent-trust-presets.ts`

---

## 3. Credential Management

| Credential Type | Storage | Access |
|----------------|---------|--------|
| SSH Private Keys | ~/.ssh/ (user filesystem) | Resolved per-connection |
| API Tokens (GitHub, Linear) | OS Keychain (Keytar) | Loaded at API call time |
| Mobile shared secret | In-memory only | Cleared on unpair |
| Relay session token | In-memory, ephemeral | Invalidated after session |
| Agent API keys | OS Keychain | Injected via env vars |

**Rules:**
- ❌ Không bao giờ log credentials
- ❌ Không bao giờ serialize credentials sang disk
- ✅ OS Keychain cho tất cả long-lived secrets
- ✅ In-memory cho ephemeral session tokens

---

## 4. Mobile E2E Encryption

```
Pairing (one-time):
  Desktop generates: (d_pub, d_priv) keypair
  Desktop generates: token (random 32 bytes, expire 5min)
  Desktop → QR code: { d_pub, host, port, token }

  Mobile scans QR
  Mobile generates: (m_pub, m_priv) keypair
  Mobile sends: { m_pub, token }

  Desktop verifies token
  Desktop derives: shared_secret = nacl.box(m_pub, d_priv)
  Mobile derives:  shared_secret = nacl.box(d_pub, m_priv)

Communication (ongoing):
  All messages: nacl.box.seal(message, nonce, shared_secret)
  Nonce: random 24 bytes per message (never reused)
```

**Source:** `src/main/ipc/mobile.ts`

---

## 5. Relay Binary Security

| Check | Method | Fail action |
|-------|--------|------------|
| Binary authenticity | SHA256 hash comparison | Refuse to start |
| Version compatibility | Version string check | Re-upload |
| Token auth | Session token per connection | Reject connection |
| Privilege | Runs as non-root user | Install fails gracefully |
| Network exposure | localhost only (127.0.0.1) | No remote binding |

---

## 6. Renderer Sandbox

Electron renderer sử dụng strict sandbox:

```typescript
// Renderer: CANNOT do this (blocked by sandbox)
const fs = require('fs');  // ❌ No Node.js in renderer

// Renderer: MUST use contextBridge
const data = await window.orcaAPI.filesystem.readFile(path);  // ✅

// Preload: whitelist only safe operations
contextBridge.exposeInMainWorld('orcaAPI', {
  filesystem: {
    readFile: (path: string) => ipcRenderer.invoke('fs:read', path),
    // writeFile: NOT exposed (too dangerous from renderer)
  }
});
```

---

## 7. IPC Security

| Threat | Mitigation |
|--------|-----------|
| Renderer XSS → shell exec | contextBridge whitelist, no `shell.openExternal` from untrusted input |
| Path traversal | Input validation in main process handlers |
| IPC injection | Type-safe IPC with Zod validation |
| Renderer → main privilege escalation | Main process validates all IPC inputs |

---

## 8. Multi-User Auth Security (Web Server Mode)

### 8.1 Password Storage

| Aspect | Implementation |
|--------|----------------|
| Algorithm | bcrypt 12 rounds (resistant to brute force) |
| Storage | `orca_users.password_hash` (hash only, never plaintext) |
| Reset | Admin-only, generates temp password, forces change on login |

### 8.2 Session Management

| Aspect | Implementation |
|--------|----------------|
| Token format | Secure random UUID v4 (128-bit entropy) |
| Storage | `orca_sessions` table (server-side, not JWT) |
| Cookie flags | `HttpOnly; SameSite=Lax; Secure` (when TLS) |
| TTL | 8h (configurable); `last_seen_at` updated per request |
| Revocation | Immediate: delete row from `orca_sessions` |

### 8.3 Authorization Guards

```
requireAuth()  → validate orca_session cookie → inject userId, role
requireAdmin() → requireAuth() + check role == 'admin' → 403 if not
```

**Protected routes:**
- `requireAuth`: WebSocket upgrade, all API routes
- `requireAdmin`: `/admin/api/*`

**HTTP-layer `requireAdmin` (above) is correctly enforced and always has been.** The RPC-layer admin checks are a separate code path and had a real gap:

**Fixed 2026-08-09 (was a real permission-bypass bug — BUG-BE-HLD-001/002):** `requireAdmin(ctx)` in `backend/src/main/profile/profile-rpc-handler.ts` and `requireOwnerOrAdmin(...)` in `backend/src/main/project/project-rpc-handler.ts` previously only checked that the caller was authenticated (`ctx.userId` present), **not** that their role was `'admin'` — any logged-in user, including role `'developer'`, could call `profile.updateCompany`, `profile.updateDept`, `profile.createCompany`, `profile.createDept`, `profile.setUserDept`, and override project ownership checks. This has been fixed: both guards now resolve the caller's real org-level role via `AuthUserStore` (`getUserRole(ctx.userId)`, wired in `server-bootstrap.ts`) and throw `FORBIDDEN` unless `role === 'admin'` — see `backend/src/main/profile/profile-rpc-handler.ts:301-308`.

**Still an open gap:** RBAC remains fragmented across ~4 independent mechanisms with no single policy table — HTTP `requireAdmin` middleware, RPC `requireAdmin`, RPC `requireOwnerOrAdmin`, and fleet-level `resolveUserPermissions()` (`shared/rbac-types.ts`) are separate, not-fully-consistent implementations. There is no `hasPermission(role, resource, action)` function as a single source of truth (BUG-BE-HLD-003, still open).

---

## 9. Admin Audit Log

| Field | Mô tả |
|-------|---------|
| `id` | UUID |
| `actor_id` | userId của người thực hiện |
| `action` | `login.success`, `user.create`, `session.kill`, v.v. |
| `target_type` | `user`, `session`, `ssh_host` |
| `target_id` | ID của target |
| `metadata` | JSON extra data (vd: IP address, `linuxUser`) |
| `created_at` | Timestamp UTC |

**Rule:** Audit log là append-only. Không có DELETE API trên audit log.

---

## 10. Agent WebSocket Auth

### relay-websocket (Orca → Agent) — ✅ implemented

```
Orca → HTTP Upgrade: ws://agent:6799/orca-relay   (agent is the WS server)
      Header: Authorization: Bearer <agentToken>  (or ?token= query param)
Agent validates token → accept/reject
```
`agent/src/relay/agent-connection-relay.ts:44-45,96-129`. Token is mandatory — if `ORCA_AGENT_TOKEN` is unset the agent process refuses to start (`process.exit(1)`), no insecure default fallback.

### direct-websocket (Agent → Orca) — ⚠️ port corrected, flow corrected

```
Agent → HTTP Upgrade: ws://orca:6769/agent        ← was documented as :6768, code uses httpPort (default 6769)
Agent sends first: { jsonrpc: '2.0', id: 1, method: 'agent.handshake', params: { agentToken, capabilities } }
Orca validates agentToken (SHA-256 hash lookup against pending slot, in-memory)
  - Valid   → normal JSON-RPC response { result: { ok: true } } (no separate "handshake-ok" message type)
  - Invalid → JSON-RPC error -33101 AuthFailed, then WS close(1008)  ← NOT custom code 4001
```
Evidence: `backend/src/server/index.ts:46-47,106,108` (`AgentWebSocketServer` attaches to `httpPort`, not `rpcPort`); `backend/src/main/dev-server/ws-handshake.ts:180-203` (auth-failure error + `close(1008,...)`); `agent/src/relay/agent-session.ts:196-219` (agent sends the handshake request, Orca does not send `handshake-request` first as previously documented).

**Token lifecycle/management — 🚧 the model below is PROPOSED, not implemented (ADR-015/ADR-019 territory). Real mechanism, as shipped today:**

| Aspect | Real behavior |
|---|---|
| Issuance | Self-service: agent (or an operator script) calls `POST /api/agent-token` with header `Authorization: Bearer <ORCA_AGENT_API_SECRET>` — a single static shared secret set once on the Orca Server, **not** an admin-UI-generated per-connection credential (`backend/src/server/agent-token-routes.ts`) |
| Token format | **Predictable**: `agt-<devServerId>-<timestamp>` (`generateAgentToken()`, `agent/src/shared/agent-wire-protocol.ts:89-91`) — not `crypto.randomBytes(32)` |
| Storage | **No DB table.** In-memory `Map` (`pendingMeta`/`pendingSlots` in `agent-ws-server.ts` / `agent-token-routes.ts`) — token is hashed (SHA-256) before being kept in the map, so no plaintext sits in memory/heap dumps long-term |
| Renewal | Agent-side `AgentTokenManager` proactively renews at 80% of TTL (`TOKEN_RENEW_RATIO=0.8`, default TTL 24h) and pre-fetches the next token, so reconnects rarely hit an expired-token path (`agent/src/relay/agent-token-manager.ts`) — this is more sophisticated than the docs below assume, but it is still all keyed off the one static `ORCA_AGENT_API_SECRET` |
| Revocation | **None.** There is no `DELETE /admin/api/agent-tokens/:id` or equivalent — the only way to invalidate access is to rotate `ORCA_AGENT_API_SECRET` server-wide, which invalidates every dev server's ability to mint a *new* token but does not kill already-issued live connections |
| Admin UI | None — no token is ever shown once in an Admin Panel; there is no per-dev-server token registry to browse or revoke from |

**Known weaknesses of the real mechanism:** the token format is guessable given a known `devServerId`; there is no way to revoke a single compromised dev server's access without rotating the shared secret for the whole fleet; and issuance auth is a single long-lived shared secret rather than a scoped, per-connection credential. Treat `ORCA_AGENT_API_SECRET` with the same care as a root credential.

---

## 11. WebCredentialStore

**Mục đích:** Lưu API tokens cho integrations (Bitbucket, Linear, Jira, Azure DevOps, Gitea) mà không expose sang user khác.

```
Encryption: AES-256-GCM
Key derivation: Per-user key từ userId + server secret (ORCA_SERVER_SECRET env)  ← corrected: real env var
Storage: File `~/.orca/users/<userId>/credentials.enc`
IV: Random 12 bytes per encryption op
Auth tag: 16 bytes (GCM)
```

`backend/src/main/credentials/index.ts:11,16` — the real env var is `ORCA_SERVER_SECRET`; `ORCA_CREDENTIAL_KEY` (as previously documented here) does not exist anywhere in code.

**Scope — does NOT cover GitHub/GitLab.** `CredentialService` only handles `'bitbucket'|'azure-devops'|'gitea'|'linear'|'jira'`. GitHub and GitLab auth instead rely on the OS keychain used by the `gh`/`glab` CLIs — a different trust mechanism, not AES-256-GCM-encrypted through this store.

**API:**
- `credentials.set(service, token)` → encrypt + write file
- `credentials.get(service)` → read file + decrypt
- `credentials.delete(service)` → update file
- Tất cả RPC calls được scope theo `userId` từ session context

---

## Trust Boundary 4: Web Mode

```
┌─────────────────── TRUST BOUNDARY 4: Web Server ───────────────────────────┐
│                                                                    │
│  Browser/Agent → Nginx (TLS term) → Orca Web Server              │
│                                                                    │
│  Layer 1: TLS (Nginx)                                              │
│  Layer 2: Auth middleware (requireAuth cookie check)               │
│  Layer 3: WsSessionRouter (route to userId process)                │
│  Layer 4: Per-user process (full isolation)                        │
│                                                                    │
│  Admin escalation: requireAdmin() check truớc tiếp admin API    │
└────────────────────────────────────────────────────────────────────┘
```

---

## v5.0 — Security Model Extensions

### 5.1 Profile Hierarchy Security

**Lock mechanism — Company controls security policy:**
```
Company profile: security section is LOCKED
  approvedModels: ['claude-opus-4-5', 'gpt-4o']  ← locked
  disallowedCommands: ['rm -rf /', 'curl | bash']  ← locked

Dept/User profiles: security section IGNORED (deep-merge discards it)
ProfileResolver: lockedSections = ['security'] → always use company values
```

**Profile API permissions:**
| API | Requires |
|-----|---------|
| `profile.company.update` | `admin` role |
| `profile.dept.update` | `admin` or `team-lead` role |
| `profile.user.update` | own user, NOT locked fields |
| `profile.resolve(userId)` | session of `userId` or admin |

**Cache invalidation security:** Profile TTL = 60s. Admin có thể force-invalidate tức thì qua `profile.invalidate(userId)`. Nếu user bị kick (suspended), session vẫn có thể dùng cached profile trong ≤60s — acceptable trade-off.

---

### 5.2 AI Provider Credential Security (ADR-008)

**Threat model — corrected against real code (`agent/src/relay/agent-credential-store.ts`, `agent/src/relay/agent-spawner.ts`):**
```
Threat: Orca Server compromise → attacker reads AI API keys
Mitigation: Keys NEVER written to disk on Orca Server. Only the .enc file lives on Dev Server.
⚠️ Caveat (not just doc drift — a real architectural fact): this mitigation covers storage
   only. On the SPAWN/USE path, Orca Server must resolve the credential and forward it as a
   PLAINTEXT `resolvedApiKey` param in the `agent.spawn` RPC call — the Dev Server agent has
   no way to decrypt the Layer-1 (browser-encrypted) blob itself. If Orca Server omits
   `resolvedApiKey`, agent spawn now fails fast with a clear error (fixed — previously it
   silently injected undecryptable ciphertext as the API key, BUG-AG-HLD-002) rather than
   succeeding. So: "Orca Server never sees plaintext" is true for the credential-write path,
   FALSE for the spawn/use path. See `agent/src/relay/agent-spawner.ts:227-243`.

Threat: Man-in-the-middle relay interception
Mitigation: SSH tunnel encrypts relay traffic

Threat: Dev Server filesystem read by unauthorized user
Mitigation: Files at ~/.orca/credentials/<accountId>.enc (AES-256-GCM)   ← corrected path
            (previously documented as ~/.orca/ai-providers/<id>.enc — that path only exists
            in `ai-provider-handler.ts`, which is dead code with 0 callers)
            Key derived via scrypt(ORCA_AI_CREDENTIAL_KEY, salt) — salt is a random 16 bytes
            generated per write and stored alongside the ciphertext, not accountId-derived

Threat: Quota abuse (user uses company AI key for personal)
Mitigation: quota tracking per accountId per day, 80% alert (implemented, see below), hard limit
```

**Key rotation procedure — ✅ implemented (`backend/src/main/ai-providers/AIProviderService.ts`):**
1. Admin tạo new account với key mới → `aiProvider.rotateKey` RPC
2. 30s grace period (`DEFAULT_ROTATION_GRACE_PERIOD_MS = 30_000`): cả old (status `'rotating'`) và new account active
3. Old account deactivated once grace period elapses (`completeRotation()`, also triggered by crash-recovery health check)
4. Audit log entry written on create/update/delete/credential-write

**80% quota alert — ✅ implemented** (`ProviderHealthChecker.ts`, `QUOTA_ALERT_THRESHOLD_RATIO = 0.8`), debounced to once per account per day.

**`ORCA_AI_CREDENTIAL_KEY` management:**
- Không được commit vào git
- Phải set trên mỗi Dev Server (khác với Orca Server secret `ORCA_SERVER_SECRET`)
- Rotation: re-encrypt tất cả `.enc` files với key mới (migration script cần thiết) — this specific key-rotation-for-the-master-key-itself is not the same as the account key rotation described above, and remains a manual/undocumented procedure

---

### 5.3 Workflow Orchestration Security

**Step execution authorization:**
```
Workflow step chạy agent → dùng project AI provider account
  → Account phải thuộc cùng devServer với step's serverSpec
  → User phải là project member để trigger workflow

Workflow step: shell type
  → BLOCKED từ dùng disallowedCommands (từ Company profile)
  → No shell expansion: args validated whitelist (no &&, |, ;, $)
  → Timeout bắt buộc: default 30min, max 2h
```

**Template inheritance security:**
- `company` scope templates: chỉ admin publish
- `team` scope templates: team-lead approve
- `public` templates: admin review queue (prevent prompt injection via shared templates)

---

### 5.4 Task Grant Security

**Grant model bảo mật:**
```
Permission levels (từ thấp đến cao):
  view → comment → edit → execute → manage

'execute' permission = có thể spawn AI agent từ task
  → Phải có project membership để execute
  → Agent spawned inherits task.promptTemplate (admin-controlled)

'manage' permission = có thể grant/revoke cho người khác
  → Không thể grant higher than own permission
  → apply_tree=true: phải check tất cả descendants trước khi grant
```

**Public share link:**
- Token-based, view-only, no login required
- No PII exposed: ẩn assignee email, chỉ show display name
- Admin có thể revoke tất cả public links của task
- Rate-limited: max 100 req/min per share token

---

### 5.5 Remote Git UI Security

**Git command injection prevention (ADR-012):**
```typescript
const ALLOWED_GIT_SUBCOMMANDS = new Set([
  'status', 'diff', 'add', 'restore', 'commit', 'push', 'pull',
  'fetch', 'branch', 'checkout', 'merge', 'rebase', 'stash',
  'log', 'worktree', 'remote', 'tag'
])

// No shell expansion
// All args validated: /[&|;$`]/.test(arg) → reject
// Git exec runs via execFile() (not shell spawn) → no injection
```

**PR creation security:**
- GitHub CLI: dùng per-user `GH_CONFIG_DIR` isolation (không share token)
- API token: encrypted via WebCredentialStore per userId
- PR branch: validated không chứa `../` hay absolute paths

**Commit author:**
- Always uses Dev Server git config (`user.name`, `user.email`)
- Cannot impersonate other users' git identity
- Audit log records: userId → commitHash

---

### 5.6 Project Workspace Security

**RelayConnectionPool isolation:**
```
Mỗi relay.call() phải kèm userId context
  → Relay dispatcher validate: user is project member
  → File paths validated: không truy cập ngoài project.repoPath
     (path.relative() phải không bắt đầu bằng '../')

Worktree access:
  → User chỉ thấy worktrees trong project.repoPath
  → Cannot access other projects' worktrees via same relay connection
```

**Offline mode security:**
- Cached file tree: không cache file contents (chỉ metadata)
- Cached git status: stale data warning sau 5 phút
- Write operations disabled khi offline: không thể commit/push

---

## Security Checklist v5.0

| Concern | Status | ADR |
|---------|--------|-----|
| Platform abstraction (no Electron in server) | ✅ | ADR-001 |
| Multi-DB dialect injection prevention | ✅ | ADR-002 |
| Per-user process isolation | ✅ | ADR-003 |
| SSH relay auto-deploy security | ✅ | ADR-004 |
| Agent WS token auth | ✅ | ADR-005 |
| Credential AES-256-GCM per-user | ✅ | ADR-006 |
| Profile lock (security section) | 🚧 | ADR-007 |
| AI keys NEVER on Orca Server | ⚠️ Partial — true for storage, false for spawn path (Orca Server forwards plaintext `resolvedApiKey`; see §5.2) | ADR-008 |
| Workflow shell injection prevention | 🚧 | ADR-009 |
| Task grant no escalation | 🚧 | ADR-010 |
| Relay file path traversal prevention | 🚧 | ADR-011 |
| Git command injection prevention | 🚧 | ADR-012 |

---

## Trust Boundary 5: Gateway ↔ Dev Server Agent (v6.0 NEW)

> 🚧 **PROPOSED — not implemented.** Everything from here through the end of §12 describes the
> target architecture of `docs/adrs/v1/ADR-015-signed-execution-context-gateway-agent.md` and
> `docs/adrs/v2/ADR-017/018/019`, all of which self-declare `❌ Chưa implement` / `🚧 Proposed`.
> Confirmed by direct code inspection (`agent/src/relay/context.ts:20-34`): there is no
> `ContextVerifier`, `SignedExecutionContext`, `RpcExecutionContext`, or `_ctx` field anywhere in
> `agent/src`. `RelayContext.registerRoot()` is an intentionally-empty no-op — the team explicitly
> **removed** the equivalent agent-side path/FS allowlist (see `docs/relay-fs-allowlist-removal.md`,
> cited directly in the code comment) and instead trusts the renderer/Orca-Server layer entirely,
> because "a compromised renderer can already weaponize `pty.spawn` and `git.exec` to reach any
> path the SSH user can reach." That is the REAL current trust model for the Gateway↔Agent
> channel — not per-request HMAC-signed, TTL-bounded contexts. For the real agent authentication
> mechanism in production today, see **§10 Agent WebSocket Auth** above.

```
┌──────────────── TRUST BOUNDARY 5: Gateway–Agent ──────────────────────┐
│                                                                         │
│  Orca Backend Server (Gateway)    →wss://→    Dev Server Agent         │
│  - Authenticates all users                    - No local user DB       │
│  - Resolves RBAC policies                     - Trusts signed context  │
│  - Issues signed RpcExecutionContext           - Verifies HMAC-SHA256  │
│  - Enforces approved model whitelist           - Enforces path limits  │
│  - Tracks quota per account                    - Runs actual work      │
│                                                - Emits audit events    │
│                                                                         │
│  Shared secret: scrypt(ORCA_GATEWAY_SECRET + agentId)                  │
│  Context TTL: 30 seconds (prevents replay attacks)                     │
│  Transport: TLS 1.3 mandatory (wss://)                                 │
│  Agent authentication: HMAC-signed handshake                           │
└─────────────────────────────────────────────────────────────────────────┘
```

### 12. Dev Server Agent Security Model (v6.0) — 🚧 Proposed, not implemented (see banner above)

#### 12.1 Agent Authentication — 🚧 Proposed. Real mechanism is documented in §10 above (self-service `POST /api/agent-token` + static `ORCA_AGENT_API_SECRET`, predictable token format, no DB, no revoke endpoint).

| Aspect | Implementation |
|--------|---------------|
| Initial registration | Admin generates `agentToken` (32 bytes random) in Admin Panel |
| First connection | Agent sends token in `Authorization: Bearer` header |
| Ongoing auth | Gateway issues `agentSecret` after first verified connection |
| Handshake | HMAC-SHA256(nonce + agentId, agentSecret) — prevents replay |
| Token revocation | Admin deletes agent registration → all connections rejected |

#### 12.2 Signed Execution Context

Every RPC call from Gateway carries a **signed execution context** that Agent verifies:

```typescript
// Context signed by Gateway (HMAC-SHA256)
{
  ctx: { userId, userEmail, projectId, projectRoot, resolvedProfile, providerAccountId },
  issuedAt: number,
  expiresAt: issuedAt + 30000,    // 30 second TTL — prevents replay
  gatewayId: string,
  signature: HMAC_SHA256(JSON(ctx + times), sharedSecret)
}
```

**Why 30-second TTL:**
- Prevents replay attacks (captured context cannot be reused)
- Short enough that clock skew (< 5s) is acceptable
- Long enough for multi-hop operations (Gateway → Agent)

#### 12.3 Path Traversal Prevention (Agent-side)

```typescript
// SecureFs enforces ALL file operations
class SecureFs {
  validate(path: string, ctx: RpcExecutionContext) {
    const resolved = path.resolve(path)
    
    // Must be under project root
    if (!resolved.startsWith(ctx.projectRoot)) {
      throw AgentError(4010, 'Path outside project root')
    }
    
    // Must be under configured allowed roots
    const inAllowed = this.allowedRoots.some(r => resolved.startsWith(r))
    if (!inAllowed) {
      throw AgentError(4010, 'Path not in allowed roots')
    }
  }
}
```

#### 12.4 Shell Command Injection Prevention (Agent-side)

```typescript
// ShellStepExecutor — never uses shell interpolation
class ShellStepExecutor {
  execute(command: string, args: string[], ctx: RpcExecutionContext) {
    // Validate command is in whitelist
    const [cmd] = command.split(' ')
    if (!this.config.allowedCommands.includes(cmd)) {
      throw AgentError(4001, `Command '${cmd}' not in allowedCommands`)
    }
    
    // Validate no shell operators in args
    for (const arg of args) {
      if (/[&|;$`]/.test(arg)) {
        throw AgentError(4001, 'Shell operator in argument')
      }
    }
    
    // execFile — NOT exec (no shell expansion)
    return execFile(cmd, args, { env: buildEnv(ctx) })
  }
}
```

#### 12.5 AI Provider Credential Security (Agent-side)

| Threat | Mitigation |
|--------|-----------|
| Credential theft via API | Credentials NEVER returned over RPC (write-only from Gateway) |
| Filesystem read by other processes | Files at `~/.orca/ai-providers/*.enc` (AES-256-GCM, 0600 permissions) |
| Key compromise | Key `ORCA_AI_CREDENTIAL_KEY` stored as env var, not on disk |
| Account misuse | Model validation via `ctx.agentSettings.approvedModels` before spawn |
| Cross-user access | Credentials bound to `accountId`, not per-user (provider account is shared resource) |

#### 12.6 Audit Trail (Agent-side)

Agent maintains append-only local audit log (`agent_audit_log` SQLite table):

| Field | Content |
|-------|---------|
| `requestId` | From Gateway context (end-to-end trace) |
| `userId` | From signed context |
| `userEmail` | From signed context |
| `method` | RPC method called |
| `params` | Sanitized (no credentials, no file contents) |
| `outcome` | `success` / `error` / `denied` |
| `errorCode` | Agent error code if denied |
| `durationMs` | Execution time |
| `timestamp` | Unix ms |

Agent syncs audit entries to Gateway (which writes to `orca_audit_log` Server DB).

---

## Security Checklist v6.0 (additions)

| Concern | Status | CR |
|---------|--------|----|
| Gateway-Agent HMAC-signed context | ❌ TODO | CR-DS-005 |
| Context replay prevention (30s TTL) | ❌ TODO | CR-DS-005 |
| Agent path traversal prevention (SecureFs) | ❌ TODO | CR-DS-003 |
| Agent shell injection prevention (execFile) | ❌ TODO | CR-DS-003 |
| Agent credential write-only over RPC | ❌ TODO | CR-DS-002 |
| Agent audit log sync to Gateway | ❌ TODO | CR-DS-005 |
| Agent handshake HMAC | ❌ TODO | CR-DS-002 |
| Agent token revocation | ❌ TODO | CR-DS-004 |
| PTY per-userId ownership enforcement | ❌ TODO | CR-DS-005 |
| Git author injection (no impersonation) | ❌ TODO | CR-DS-003 |
| Per-user GH_CONFIG_DIR isolation | ❌ TODO | CR-DS-003 |
| approvedModels check before spawn | ❌ TODO | CR-DS-005 |

