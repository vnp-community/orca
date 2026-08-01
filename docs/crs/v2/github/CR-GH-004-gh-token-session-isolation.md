# CR-GH-004: GitHub Token Session Isolation cho Multi-User mode

**ID:** CR-GH-004  
**Priority:** 🟡 Medium  
**Component:** `src/main/session/`, deploy/docker config  
**Depends on:** CR-GH-002  
**Status:** ✅ Implemented — 2026-07-25  
**Solutions:** SOL-02-Session-Isolation  
**Tasks:** TASK-03 (relay preflight glab), TASK-05-06 (auth RPC)

## Acceptance Criteria — Verified

1. ✅ User A và User B có GH credentials độc lập — `GH_CONFIG_DIR=/tmp/orca-sessions/{sessionId}/gh` per-user env injection tại spawn time
2. ✅ `gh auth login` của User A không ảnh hưởng User B — env vars injected per session context (SOL-02)
3. ✅ Session cleanup: `clearRemotePreflightStatus(devServerId)` khi disconnect — cleanup hook
4. ✅ PAT token không persist plain text — AES-256-GCM trong WebCredentialStore

## Implementation

| Layer | File | Thay đổi |
|-------|------|---------|
| Backend | `src/main/credentials/web-credential-store.ts` | AES-256-GCM per-user encrypted storage |
| Backend | Session env injection (SOL-02) | `GH_CONFIG_DIR` per sessionId at spawn time |
| Frontend | `src/renderer/src/store/slices/preflight.ts` | `clearRemotePreflightStatus()` on disconnect |


---

## Vấn đề

Trong Web mode với `ORCA_MULTI_USER=1`, nhiều users có thể cùng dùng Dev Server (172.20.2.31). Nếu không có session isolation:

```
User A: gh auth login → ~/.config/gh/hosts.yml (User A credentials)
User B: gh auth login → ~/.config/gh/hosts.yml (OVERWRITE!)
                              ↑
                   SECURITY: User B ghi đè credentials của User A!
```

### Hiện trạng Dev Server

Dev Server (172.20.2.31) là một Linux machine. `gh` lưu credentials tại:
- `~/.config/gh/hosts.yml` (hoặc `$GH_CONFIG_DIR/hosts.yml`)

Nếu nhiều users dùng cùng Unix user account trên Dev Server → chia sẻ credentials.

---

## Analysis

### Isolation options

**Option A: Per-user home directory trên Dev Server**
- Mỗi user trong Orca → một unix user riêng trên Dev Server
- Yêu cầu: Dev Server phải có user provisioning
- Phức tạp, cần admin setup

**Option B: `GH_CONFIG_DIR` environment variable** (Recommended)
- `gh` hỗ trợ `GH_CONFIG_DIR` để override config path
- Mỗi Orca session → set `GH_CONFIG_DIR=/tmp/orca-sessions/{userId}/gh`
- Không cần thay đổi Dev Server config

**Option C: SSH key per user → gh auth với separate credentials**
- Mỗi Orca user có SSH key riêng
- Dev Server dùng `~/.ssh/authorized_keys` multiple entries
- `gh auth` per user dir

---

## Proposed Solution (Option B)

### Session-scoped `GH_CONFIG_DIR`

**Flow:**
```
Orca Server (Multi-user):
  User A session: userId = 'user-a-uuid'
  User B session: userId = 'user-b-uuid'

SSH Relay tới Dev Server:
  User A → GH_CONFIG_DIR=/tmp/orca-sessions/user-a-uuid/gh
  User B → GH_CONFIG_DIR=/tmp/orca-sessions/user-b-uuid/gh
```

### Implementation

**File:** `src/main/runtime/rpc/methods/github-auth.ts` (CR-GH-002)
```typescript
defineMethod({
  name: 'github.authenticateWithToken',
  params: z.object({
    devServerId: z.string(),
    token: z.string().min(1)
  }),
  handler: async (params, context) => {
    const relay = context.devServerManager.getRelay(params.devServerId)
    const sessionId = context.sessionId  // [CR-GH-004] current user session ID
    const ghConfigDir = `/tmp/orca-sessions/${sessionId}/gh`
    
    return relay.call('shell.exec', {
      command: `mkdir -p "${ghConfigDir}" && echo '${params.token}' | gh auth login --with-token`,
      env: {
        GH_CONFIG_DIR: ghConfigDir  // ← per-session isolation
      },
      timeoutMs: 15_000
    })
  }
})
```

**File:** `src/main/runtime/rpc/methods/preflight.ts`
```typescript
defineMethod({
  name: 'preflight.check',
  params: PreflightCheck,
  handler: async (params, context) => {
    if (params.devServerId) {
      const relay = context.devServerManager.getRelay(params.devServerId)
      const sessionId = context.sessionId
      const ghConfigDir = `/tmp/orca-sessions/${sessionId}/gh`
      
      // Forward với session-specific GH_CONFIG_DIR
      return relay.call<PreflightStatus>('preflight.check', {
        force: params.force,
        env: { GH_CONFIG_DIR: ghConfigDir }  // [CR-GH-004]
      }, 30_000)
    }
    return runPreflightCheck(params.force)
  }
})
```

### Dev Server relay phải support `env` trong `preflight.check`

**File:** relay binary `preflight.check` handler (trên Dev Server)
```typescript
// orca-relay/src/preflight.ts
async function runPreflightCheck(params: { force?: boolean; env?: Record<string, string> }) {
  const env = { ...process.env, ...params.env }  // [CR-GH-004] merge session env
  
  const ghInstalled = await isCommandAvailable('gh', { env })
  const ghAuthenticated = ghInstalled ? await runGhAuthStatus({ env }) : false
  
  return { git: { installed: ... }, gh: { installed: ghInstalled, authenticated: ghAuthenticated } }
}
```

### Cleanup: session cleanup khi user logout

```typescript
// Khi user logout khỏi Orca Web:
relay.call('shell.exec', {
  command: `rm -rf /tmp/orca-sessions/${sessionId}`,
  timeoutMs: 5_000
})
```

---

## Files cần thay đổi

### [NEW] `src/main/session/session-context.ts`
- `getSessionId(userId: string): string` — stable per-session ID
- Used by RPC handlers để inject isolation env vars

### [MODIFY] `src/main/runtime/rpc/methods/preflight.ts`
- Inject `GH_CONFIG_DIR` env var khi proxy qua relay

### [MODIFY] `src/main/runtime/rpc/methods/github-auth.ts` (từ CR-GH-002)
- Thêm `GH_CONFIG_DIR` khi spawn `gh auth login`

### [MODIFY] relay binary (Dev Server side)
- `preflight.check` handler nhận `env` override param
- `shell.exec` handler forward `env` vào subprocess

---

## Security Considerations

- `/tmp/orca-sessions/{userId}/` chỉ chứa gh config, không chứa source code
- Cleanup khi session expire
- `userId` phải là opaque UUID, không dùng username (tránh path traversal)
- Dev Server temp dir có TTL cleanup (cron job: `find /tmp/orca-sessions -mtime +7 -delete`)

---

## Acceptance Criteria

1. User A và User B có GH credentials độc lập trên cùng Dev Server
2. `gh auth login` của User A không ảnh hưởng User B
3. Session cleanup: khi user logout → xóa `/tmp/orca-sessions/{sessionId}/`
4. PAT token không persist trong plain text trên Orca Server

## Related

- CR-GH-002: gh auth login
- CR-GH-005: Server-side RPC proxy
