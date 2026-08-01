# Security Architecture

**Tài liệu:** Security Model của Orca  
**Tham chiếu:** SRS Section 4.3, logic/agent-orchestration/, logic/mobile-companion/  
**Cập nhật:** 2026-07-28

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

### relay-websocket (Orca → Agent)

```
Orca → HTTP Upgrade: ws://agent:6799/orca-relay
      Header: Authorization: Bearer <agentToken>
Agent validates token → accept/reject
```

### direct-websocket (Agent → Orca)

```
Agent → HTTP Upgrade: ws://orca:6768/agent
Orca sends: { type: 'handshake-request' }
Agent sends: { type: 'agent.handshake', agentToken, name, version }
Orca validates agentToken (lookup trong DevServer config)
  - Valid → { type: 'handshake-ok', sessionId }
  - Invalid → WS close (4001 Unauthorized)
```

**Token management:**
- Tokens lưu trong DevServer config (NOT in main DB)
- Token hiển thị một lần khi tạo (sau đó hashed)
- Revoke: xoá token khỏi DevServer config

---

## 11. WebCredentialStore

**Mục đích:** Lưu API tokens cho integrations (Bitbucket, Linear, Jira...) mà không expose sang user khác.

```
Encryption: AES-256-GCM
Key derivation: Per-user key từ userId + server secret (ORCA_CREDENTIAL_KEY env)
Storage: File `~/.orca/users/<userId>/credentials.enc`
IV: Random 12 bytes per encryption op
Auth tag: 16 bytes (GCM)
```

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

**Threat model:**
```
Threat: Orca Server compromise → attacker reads AI API keys
Mitigation: Keys NEVER stored on Orca Server. Only on Dev Server.

Threat: Man-in-the-middle relay interception
Mitigation: SSH tunnel encrypts relay traffic

Threat: Dev Server filesystem read by unauthorized user
Mitigation: Files at ~/.orca/ai-providers/<id>.enc (AES-256-GCM)
            Key derived from ORCA_AI_CREDENTIAL_KEY (env var, not on disk)

Threat: Quota abuse (user uses company AI key for personal)
Mitigation: quota tracking per accountId per day, 80% alert, hard limit
```

**Key rotation procedure:**
1. Admin tạo new account với key mới
2. 30s grace period: cả old và new account active
3. Old account deactivated
4. Background health check cron verify new key status

**`ORCA_AI_CREDENTIAL_KEY` management:**
- Không được commit vào git
- Phải set trên mỗi Dev Server (khác với Orca Server secret)
- Rotation: re-encrypt tất cả `.enc` files với key mới (migration script cần thiết)

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
| AI keys NEVER on Orca Server | 🚧 | ADR-008 |
| Workflow shell injection prevention | 🚧 | ADR-009 |
| Task grant no escalation | 🚧 | ADR-010 |
| Relay file path traversal prevention | 🚧 | ADR-011 |
| Git command injection prevention | 🚧 | ADR-012 |

---

## Trust Boundary 5: Gateway ↔ Dev Server Agent (v6.0 NEW)

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

### 12. Dev Server Agent Security Model (v6.0)

#### 12.1 Agent Authentication

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

