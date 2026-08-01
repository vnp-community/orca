# CR-GH-002: `gh auth login` qua Remote PTY trên Dev Server (Web mode)

**ID:** CR-GH-002  
**Priority:** 🔴 Critical  
**Component:** `src/renderer/src/web/web-preload-api.ts`, `src/main/ipc/onboarding-ipc.ts`  
**Depends on:** CR-GH-001  
**Status:** ✅ Implemented — 2026-07-25  
**Solutions:** SOL-03-Remote-PTY, FE-SOL-02  
**Tasks:** TASK-05-06 (backend), FE-TASK-02, FE-TASK-03, FE-TASK-04

## Acceptance Criteria — Verified

1. ✅ `github.startAuthLogin(devServerId)` spawn PTY `gh auth login` trên Dev Server — `src/main/runtime/rpc/methods/github-auth.ts` + relay
2. ✅ `gitlab.startAuthLogin(devServerId)` spawn PTY `glab auth login` trên Dev Server — `src/main/runtime/rpc/methods/gitlab-auth.ts` (L30-35)
3. ✅ `window.api.github.startAuthLogin` và `window.api.gitlab.startAuthLogin` available trong Web mode — `web-preload-api.ts` L747-776
4. ✅ `WebModeCliAuthSection` component hiển thị PTY launch panel với status tracking — `WebModeCliAuthSection.tsx`
5. ✅ `cli-source-control-integration-cards.tsx` phân nhánh Web mode → `<WebModeCliAuthSection>` khi `not-authenticated`
6. ⬜ Inline xterm.js terminal trong Settings panel (Phase 2 — deferred)

## Implementation

| Layer | File | Thay đổi |
|-------|------|---------|
| Backend | `src/main/runtime/rpc/methods/github-auth.ts` | `github.startAuthLogin` → relay PTY spawn |
| Backend | `src/main/runtime/rpc/methods/gitlab-auth.ts` | `gitlab.startAuthLogin` → relay PTY spawn (L30-35) |
| Preload | `src/preload/api-types.ts` | `github.*` + `gitlab.*` namespaces added |
| Frontend | `src/renderer/src/web/web-preload-api.ts` | `github.*` + `gitlab.*` exposed (L747-776) |
| Frontend | `WebModeCliAuthSection.tsx` | NEW — PTY launch UI component |
| Frontend | `cli-source-control-integration-cards.tsx` | Web mode branch (L131-136, L276-281) |

---

## Vấn đề

Khi user click "Connect GitHub" trong Web mode, không có cơ chế nào để:
1. Chạy `gh auth login` interactive trên **Dev Server** (172.20.2.31)
2. Stream output PTY back cho browser
3. Handle browser-based OAuth callback

### Onboarding flow (Electron) đã implement:

```typescript
// src/main/ipc/onboarding-ipc.ts
ipcMain.handle('onboarding.openGhAuthTerminal', async (_event, params) => {
  const relay = devServerManager.getRelay(params.devServerId)
  const ptyId = await relay.call<string>('pty.spawn', {
    command: 'gh',
    args: ['auth', 'login'],
    env: {},
    cols: 120,
    rows: 30
  })
  return { ptyId, devServerId: params.devServerId }
})
```

**Vấn đề:** Handler này chỉ có trong Electron mode (qua `ipcMain`). Web mode cần RPC equivalent.

---

## Analysis

### `gh auth login` flow trên Dev Server

```
gh auth login --web
    │
    ├─ Print OAuth URL: https://github.com/login/oauth/authorize?...
    │
    ├─ Wait for browser callback (localhost:XXXX)
    │         ↑ 
    │   [PROBLEM] Dev Server không reach được browser
    │
    └─ Sau khi auth: lưu token vào ~/.config/gh/hosts.yml
```

### Options

**Option A: `gh auth login --with-token`** (Recommended)
- User tạo GitHub PAT trên github.com
- Paste token vào Orca Settings UI
- Server forward token đến Dev Server qua SSH relay
- `echo TOKEN | gh auth login --with-token`

**Option B: `gh auth login --web` qua PTY**
- Spawn PTY trên Dev Server
- Stream output về browser
- User copy OAuth URL từ PTY output
- Hoặc: implement localhost callback bridge

**Option C: SSH tunnel cho OAuth callback**
- Dev Server `gh auth login --web`
- Tạo SSH tunnel: `localhost:XXXX (Dev Server) → browser`
- Phức tạp nhưng UX tốt nhất

---

## Proposed Solution (Option A — PAT-based)

### Phase 1: PAT Token Input (Quick Win)

**UI Change:** Thêm "GitHub Personal Access Token" input field trong Settings > Integrations > GitHub

```typescript
// New RPC method: github.authenticateWithToken
defineMethod({
  name: 'github.authenticateWithToken',
  params: z.object({
    devServerId: z.string(),
    token: z.string().min(1)
  }),
  handler: async (params, context) => {
    const relay = context.devServerManager.getRelay(params.devServerId)
    if (!relay) throw new Error('Dev server not connected')
    
    // Pipe token to gh auth login --with-token on dev server
    return relay.call('shell.exec', {
      command: `echo '${params.token}' | gh auth login --with-token`,
      timeoutMs: 15_000
    })
  }
})
```

### Phase 2: PTY-based Interactive Auth

**New RPC method:** `github.startAuthLogin` → spawn PTY trên dev server

```typescript
// src/main/runtime/rpc/methods/github-auth.ts [NEW]
defineMethod({
  name: 'github.startAuthLogin',
  params: z.object({
    devServerId: z.string()
  }),
  handler: async (params, context) => {
    const relay = context.devServerManager.getRelay(params.devServerId)
    if (!relay) throw new Error('Dev server not connected')
    
    // Spawn interactive PTY on dev server
    const ptyId = await relay.call<string>('pty.spawn', {
      command: 'gh',
      args: ['auth', 'login', '--web'],
      env: { GH_NO_UPDATE_NOTIFIER: '1' },
      cols: 120,
      rows: 30
    })
    return { ptyId, devServerId: params.devServerId }
  }
})
```

**Browser renderer:**
```typescript
// web-preload-api.ts — github.startAuthLogin
async startAuthLogin(devServerId: string) {
  return callRuntimeResult<{ ptyId: string; devServerId: string }>(
    'github.startAuthLogin',
    { devServerId }
  )
}
```

---

## Files cần thay đổi

### [NEW] `src/main/runtime/rpc/methods/github-auth.ts`
- `github.startAuthLogin` → spawn `gh auth login` PTY trên relay
- `github.authenticateWithToken` → `echo TOKEN | gh auth login --with-token`
- `github.revokeAuth` → `gh auth logout` trên relay

### [MODIFY] `src/main/runtime/rpc/methods/index.ts`
- Register `GITHUB_AUTH_METHODS` 

### [MODIFY] `src/renderer/src/components/settings/integrations-pane/`
- Thêm UI flow "Connect GitHub" cho web mode:
  - Detect web mode
  - Show PAT input field hoặc "Open terminal for gh auth login"

### [MODIFY] `src/renderer/src/web/web-preload-api.ts`
- Thêm `github.startAuthLogin()` và `github.authenticateWithToken()` vào API

---

## Acceptance Criteria

1. User có thể nhập GitHub PAT và server forward đến Dev Server
2. Dev Server xác thực với GitHub bằng PAT
3. Sau khi auth thành công, `preflight.check` trả về `gh.authenticated = true`
4. UI Settings > GitHub hiển thị đúng trạng thái
5. Token không được log hoặc persist trên Orca Server (chỉ pipe qua relay)

---

## Security Considerations

- PAT token chỉ được forward qua SSH relay (mã hóa)
- Không persist token trên Orca Server
- Dev Server lưu token tại `~/.config/gh/hosts.yml` (standard gh behavior)
- Session-scoped: mỗi user session có dev server riêng (xem CR-GH-004)

## Related

- CR-GH-001: preflight.check → Dev Server
- CR-GH-004: Session isolation cho multi-user
