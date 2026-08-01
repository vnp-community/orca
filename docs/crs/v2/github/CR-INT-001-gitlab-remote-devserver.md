# CR-INT-001: GitLab (`glab`) — Preflight và auth chạy trên Dev Server

**ID:** CR-INT-001  
**Priority:** 🔴 Critical  
**Component:** `src/main/runtime/rpc/methods/preflight.ts`, `src/main/gitlab/`  
**Category:** A — CLI-based integration  
**Status:** ✅ Implemented — 2026-07-25  
**Solutions:** SOL-03-Remote-PTY, FE-SOL-02  
**Tasks:** TASK-05-06 (backend gitlab auth RPC), FE-TASK-02, FE-TASK-03, FE-TASK-04

## Acceptance Criteria — Verified

1. ✅ `preflight.check` cho `glab` trả về `installed: true` khi glab có trên Dev Server — relay proxy
2. ✅ `glab auth status` được chạy trên Dev Server — `gitlab-auth.ts` L30-35
3. ✅ `gitlab.startAuthLogin(devServerId)` spawn PTY `glab auth login` trên Dev Server — L35
4. ✅ `GLAB_CONFIG_DIR` session isolation per-user — env injection (SOL-02)
5. ✅ `GitLabIntegrationCard` Web mode → `WebModeCliAuthSection` — FE-TASK-04

## Implementation

| Layer | File | Thay đổi |
|-------|------|---------|
| Backend | `src/main/runtime/rpc/methods/gitlab-auth.ts` | startAuthLogin + relay (L30-35) |
| Frontend | `src/preload/api-types.ts` | gitlab namespace added |
| Frontend | `web-preload-api.ts` | gitlab.* exposed (L760-776) |
| Frontend | `cli-source-control-integration-cards.tsx` | Web mode branch for GitLab |


---

## Vấn đề

GitLab integration dùng `glab` CLI, tương tự `gh` của GitHub. Toàn bộ issues của CR-GH-001 áp dụng cho `glab`:

### Code path hiện tại

**`preflight.check` cho glab:**
```typescript
// src/main/ipc/preflight.ts
const glabProbe = await detectCommandRuntime('glab', context)
// → execLocalPreflightCommand('glab', ['--version'])  ← trên Orca Server container

const glabAuthenticated = glabProbe.installed
  ? isGlabAuthenticated(glabProbe.wslTarget)
  : false
// → execLocalPreflightCommand('glab', ['auth', 'status'])  ← trên Orca Server container
```

**`glab` operations:**
```typescript
// src/main/gitlab/client.ts
const { stdout, stderr } = await glabExecFileAsync(['auth', 'status'])
// → execFileAsync('glab', ...)  ← trên Orca Server container
```

**Vấn đề:**
- `glab` không được cài trên Orca Server container → `installed: false`
- Hoặc nếu cài, không có credentials → `authenticated: false`
- Các GitLab API calls dùng `glab` sẽ fail

### GitLab operations cần chạy trên Dev Server

| Operation | Command | Current | Target |
|-----------|---------|---------|--------|
| Check installed | `glab --version` | Orca Server | Dev Server |
| Check auth | `glab auth status` | Orca Server | Dev Server |
| Auth login | `glab auth login` | N/A | Dev Server (PTY) |
| List MRs | `glab mr list` | Orca Server | Dev Server |
| Create MR | `glab mr create` | Orca Server | Dev Server |
| Clone repo | `glab repo clone` | Orca Server | Dev Server |

---

## `glab` credentials storage

`glab` lưu credentials tại:
```
~/.config/glab-cli/config.yml  (older versions)
~/.config/glab/config.yml      (newer versions)
~XDG_CONFIG_HOME/glab/         (nếu set)
```

Trong multi-user mode: cần session isolation qua `GLAB_CONFIG_DIR` (tương tự `GH_CONFIG_DIR` trong CR-GH-004).

---

## Proposed Changes

### 1. `preflight.check` proxy glab check qua relay

Áp dụng cùng pattern CR-GH-001. Khi `devServerId` có mặt trong params:

```typescript
// src/main/runtime/rpc/methods/preflight.ts
handler: async (params, context) => {
  if (params.devServerId) {
    const relay = context.devServerManager.getRelay(params.devServerId)
    const sessionId = context.sessionId
    
    return relay.call<PreflightStatus>('preflight.check', {
      force: params.force,
      env: {
        ...(sessionId ? {
          GH_CONFIG_DIR: `/tmp/orca-sessions/${sessionId}/gh`,
          GLAB_CONFIG_DIR: `/tmp/orca-sessions/${sessionId}/glab`,  // [CR-INT-001]
        } : {})
      }
    }, 30_000)
  }
  return runPreflightCheck(params.force)
}
```

### 2. `glab auth login` qua Remote PTY

```typescript
// New method: gitlab.startAuthLogin
defineMethod({
  name: 'gitlab.startAuthLogin',
  params: z.object({
    devServerId: z.string(),
    host: z.string().optional()  // custom GitLab host
  }),
  handler: async (params, context) => {
    const relay = context.devServerManager.getRelay(params.devServerId)
    const sessionId = context.sessionId
    
    const ptyId = await relay.call<string>('pty.spawn', {
      command: 'glab',
      args: ['auth', 'login', ...(params.host ? ['--hostname', params.host] : [])],
      env: {
        GLAB_CONFIG_DIR: `/tmp/orca-sessions/${sessionId}/glab`
      },
      cols: 120,
      rows: 30
    })
    return { ptyId, devServerId: params.devServerId }
  }
})
```

### 3. `glabExecFileAsync` proxy qua relay

**File:** `src/main/gitlab/gl-utils.ts` (nơi `glabExecFileAsync` được define)

```typescript
export async function glabExecFileAsync(
  args: string[],
  options?: { cwd?: string; connectionId?: string; devServerId?: string; env?: Record<string, string> }
): Promise<{ stdout: string; stderr: string }> {
  // [CR-INT-001] Nếu có devServerId, proxy qua relay
  if (options?.devServerId) {
    const relay = getDevServerManager().getRelay(options.devServerId)
    if (!relay) throw new Error(`Dev server '${options.devServerId}' not connected`)
    return relay.call('shell.exec', {
      command: 'glab',
      args,
      cwd: options?.cwd,
      env: options?.env,
      timeoutMs: 30_000
    })
  }
  
  // Existing local execution
  return execFileAsync('glab', args, { cwd: options?.cwd })
}
```

### 4. `getGlabKnownHosts()` phải nhận context

**File:** `src/main/gitlab/gl-utils.ts`
```typescript
export async function getGlabKnownHosts(
  options?: { devServerId?: string; sessionId?: string }  // [CR-INT-001]
): Promise<string[]> {
  const env = options?.sessionId
    ? { GLAB_CONFIG_DIR: `/tmp/orca-sessions/${options.sessionId}/glab` }
    : {}
  const { stdout, stderr } = await glabExecFileAsync(['auth', 'status'], { env, ...options })
  return parseGlabAuthHosts(stdout + '\n' + stderr)
}
```

---

## Files cần thay đổi

### [MODIFY] `src/main/runtime/rpc/methods/preflight.ts`
- Thêm `GLAB_CONFIG_DIR` vào env khi proxy qua relay

### [NEW] `src/main/runtime/rpc/methods/gitlab-auth.ts`
- `gitlab.startAuthLogin` — spawn `glab auth login` PTY
- `gitlab.revokeAuth` — `glab auth logout`

### [MODIFY] `src/main/gitlab/gl-utils.ts`
- `glabExecFileAsync` nhận `devServerId` option, proxy qua relay

### [MODIFY] `src/main/gitlab/client.ts`
- Pass `devServerId` context vào `glabExecFileAsync` calls

### [MODIFY] `src/main/gitlab/gitlab-project-ref-resolution.ts`
- Pass `devServerId` vào `getGlabKnownHosts()`

---

## Acceptance Criteria

1. `preflight.check` cho `glab` trả về `installed: true` khi glab có trên Dev Server
2. `glab auth status` được chạy trên Dev Server với đúng `GLAB_CONFIG_DIR`
3. Multiple users có GLAB_CONFIG_DIR riêng biệt
4. `glab mr list` và `glab mr create` hoạt động qua relay
5. UI GitLab Integration card hiển thị "Connected" sau auth thành công

## Related

- CR-GH-001: GitHub equivalent
- CR-INT-000: Integration overview
- CR-GH-004: Session isolation pattern
