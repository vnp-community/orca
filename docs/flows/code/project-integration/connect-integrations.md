# Luồng: Connect Integrations (Kiểm tra kết nối tích hợp)

**Ngày ghi:** 2026-07-25  
**Trạng thái:** RECHECK — cần xác nhận lại với code thực tế trước khi promote lên `flows/`

## Tổng quan

"Connect Integrations" là tính năng cho phép Orca kết nối với các dịch vụ bên ngoài:

| Nhóm | Providers | Mục đích |
|------|-----------|---------|
| **Review providers** (Code Hosts) | GitHub, GitLab, Bitbucket, Azure DevOps, Gitea | Xem PR/MR status, checks, review |
| **Task providers** (Trackers) | Linear, Jira | Browse & link issues vào workspace |

Giao diện xuất hiện ở **2 nơi**:
- `Settings → Integrations` pane (full settings)
- `Feature Wall` / Onboarding — progressive 2-step setup guide

---

## Kiến trúc tổng thể

```
Browser (React UI)
    ↓ Zustand store actions
Renderer Store (preflight.ts / linear.ts / jira.ts)
    ↓ Routing theo runtime target
        ┌─── LOCAL (Electron) ──────────────────────────────────────┐
        │   window.api.preflight.check()  → IPC → Main Process     │
        │   window.api.linear.status()   → IPC → Main Process     │
        │   window.api.jira.status()     → IPC → Main Process     │
        │   Main Process → execFile('gh', 'glab', ...) → filesystem│
        └───────────────────────────────────────────────────────────┘
        ┌─── ENVIRONMENT (Remote/SSH/Web) ──────────────────────────┐
        │   callRuntimeRpc(target, 'preflight.check', ...)          │
        │   callRuntimeRpc(target, 'linear.status', ...)            │
        │   → window.api.runtime.call() hoặc                       │
        │     window.api.runtimeEnvironments.call()                 │
        │   → WebSocket → Orca Server → OrcaRuntimeRpcServer       │
        │   → PREFLIGHT_METHODS / LINEAR_METHODS handlers          │
        └───────────────────────────────────────────────────────────┘
```

---

## LUỒNG 1: Kiểm tra Code Host (GitHub / GitLab / Bitbucket / Azure DevOps / Gitea)

### 1.1 — UI Mount (Component lifecycle)

```
IntegrationsPane.tsx hoặc ConnectIntegrationsList.tsx
  └── useIntegrationProviderStatusRefresh()
        useEffect → kiểm tra context keys:
          - preflightStatusContextKey != expectedPreflightContextKey ?
          - preflightStatusChecked == false ?
          → refreshPreflightStatus()    ← store action
```

### 1.2 — Store: `refreshPreflightStatus()` [preflight.ts:78]

```typescript
refreshPreflightStatus: async (options) => {
  const force = options?.force === true
  const context = getLocalPreflightContext(get())   // lấy WSL distro, projectRuntime...
  const contextKey = localPreflightContextKey(context)

  // Dedup: nếu đang có request cùng contextKey → trả về promise cũ
  if (!force && nonForcedPreflightRequest?.key === contextKey) return nonForcedPreflightRequest.promise

  // Routing theo runtime target
  const runtimeTarget = getActiveRuntimeTarget(get().settings)

  const request =
    runtimeTarget.kind === 'environment'
      ? callRuntimeRpc<PreflightStatus>(runtimeTarget, 'preflight.check', force ? {force} : {})
      : window.api.preflight.check(preflightArgs)    // ← Electron IPC

  // Update store state khi resolve
  set({ preflightStatus: status, preflightStatusChecked: true, ... })
}
```

### 1.3 — Luồng Electron: `window.api.preflight.check()`

```
preload/index.ts:2029
  preflight.check: (args) => ipcRenderer.invoke('preflight:check', args)
        ↓ [IPC channel: 'preflight:check']
main/ipc/preflight.ts:278
  ipcMain.handle('preflight:check', async (_event, args) =>
    runPreflightCheck(args?.force, args)
  )
        ↓
runPreflightCheck(force, context):
  // Cache hit nếu !force && cached != null
  const [gitProbe, ghProbe, glabProbe] = await Promise.all([
    detectCommandRuntime('git', context),    // which git / where.exe
    detectCommandRuntime('gh', context),     // which gh
    detectCommandRuntime('glab', context)    // which glab
  ])
  // Nếu installed → kiểm tra auth:
  const [ghAuthenticated, glabAuthenticated, bitbucket, azureDevOps, gitea] = await Promise.all([
    ghProbe.installed  ? isGhAuthenticated()  : false,
      //  → execFile('gh', ['auth', 'status']) → exit 0 = authenticated
    glabProbe.installed ? isGlabAuthenticated() : false,
      //  → execFile('glab', ['auth', 'status'])
    getBitbucketAuthStatus(),    // → đọc stored credentials
    getAzureDevOpsAuthStatus(),  // → đọc stored token
    getGiteaAuthStatus()         // → đọc stored token
  ])
  // Trả về: { git, gh, glab, bitbucket, azureDevOps, gitea }
```

### 1.4 — Luồng Web/Remote: `callRuntimeRpc(target, 'preflight.check')`

```
runtime-rpc-client.ts:68
callRuntimeRpc(target, 'preflight.check', ...)
    ↓
    target.kind === 'local'
      → window.api.runtime.call({ method: 'preflight.check', params })
    target.kind === 'environment'
      → window.api.runtimeEnvironments.call({ selector: envId, method, params })
        ↓ [IPC → Main Process]
        → OrcaRuntimeRpcServer.dispatch('preflight.check')
              ↓
              PREFLIGHT_METHODS handler [main/runtime/rpc/methods/preflight.ts:22]:
              handler: async (params) => runPreflightCheck(params.force)
              → Same runPreflightCheck() như Electron
              ← return PreflightStatus

    // Trường hợp WebSocket (Web browser mode → Orca Server):
    → window.api.runtime.call via web-preload-api.ts
      → callRuntimeResult('preflight.check', args)
      → WebSocket JSON-RPC → Orca Server container
      → OrcaRuntimeRpcServer.dispatch() → PREFLIGHT_METHODS handler
      ← PreflightStatus via WebSocket response
```

### 1.5 — `Re-check` button khi user click

```
GitHubIntegrationCard.tsx (hoặc GitLabIntegrationCard.tsx)
  const { statuses, unavailable, refresh } = usePreflightCardStatuses('gh')

  refresh() → [source-control-preflight-card-status.ts:81]
    setRefreshing(true)
    refreshPreflightStatus({ force: true })  // force = true, bỏ cache
      ← PreflightStatus
    setRefreshing(false)
    UI re-renders với status mới
```

### 1.6 — State flow qua Zustand store

```
AppStore.preflightStatus:     PreflightStatus | null
AppStore.preflightStatusChecked: boolean
AppStore.preflightStatusContextKey: string | null  (context fingerprint)
AppStore.preflightStatusLoading: boolean
AppStore.preflightStatusError: string | null

UI reads: usePreflightCardStatuses('gh')
  → deriveCliProviderCardState({
       cliStatus: preflightStatus?.gh,
       preflightStatusChecked,
       preflightStatusCurrent: contextKey === expectedContextKey,
       preflightStatusError,
       preflightStatusLoading
     })
  → 'checking' | 'connected' | 'not-installed' | 'not-authenticated' | 'unavailable'
```

---

## LUỒNG 2: Kiểm tra Linear Connection

### 2.1 — UI Mount

```
ConnectIntegrationsList.tsx hoặc IntegrationsPane.tsx
  └── useIntegrationProviderStatusRefresh()
        useEffect → kiểm tra:
          - linearStatusContextKey != providerRuntimeContextKey ?
          - linearStatusChecked == false ?
          → checkLinearConnection()
```

### 2.2 — Store: `checkLinearConnection(force?)` [linear.ts:601]

```typescript
checkLinearConnection: async (force = false) => {
  const contextKey = getProviderRuntimeContextKey(get().settings)

  // Dedup: một request đang chạy + cùng context → reuse promise
  if (inflightStatusRequest && !force && inflightStatusRequest.contextKey === contextKey)
    return inflightStatusRequest.promise

  // Gọi linearStatus() — routing đến đúng runtime
  const status = await linearStatus(get().settings)
    //  runtime-linear-client.ts:113:
    //    target.kind === 'environment'
    //      → callRuntimeRpc(target, 'linear.status', ...)
    //    else
    //      → window.api.linear.status()
    //         → IPC → linear IPC handler → Linear credentials

  // Update store + invalidate cache nếu scope thay đổi
  set({ linearStatus: status, linearStatusChecked: true, linearStatusContextKey: contextKey })
}
```

### 2.3 — Kết nối Linear (user nhập API key)

```
LinearIntegrationCard.tsx
  → <LinearApiKeyDialog open> → user nhập API key
  → linearConnect(settings, apiKey)   [runtime-linear-client.ts]
      target.kind === 'environment'
        → callRuntimeRpc(target, 'linear.connect', { apiKey })
      else
        → window.api.linear.connect({ apiKey })
          → IPC → Main → lưu encrypted API key vào keychain
          ← LinearConnectResult { ok, viewer }
  → checkLinearConnection(true)  // force refresh status
```

### 2.4 — Test Linear workspace

```
LinearIntegrationCard.tsx
  handleTest(workspaceId)
    → testLinearConnection(workspaceId)   [runtime-linear-client.ts]
        target: callRuntimeRpc(target, 'linear.testConnection', { workspaceId })
        local:  window.api.linear.testConnection({ workspaceId })
          → IPC → Main → gọi Linear API với stored key
          ← { ok: true } hoặc { ok: false, error: '...' }
    → UI hiển thị "Verified" hoặc error message
```

---

## LUỒNG 3: Kiểm tra Jira Connection

### 3.1 — Store: `checkJiraConnection()` [jira.ts:193]

```typescript
checkJiraConnection: async () => {
  const contextKey = getProviderRuntimeContextKey(get().settings)
  const status = await jiraStatus(get().settings)
    // runtime-jira-client.ts:
    //   target.kind === 'environment'
    //     → callRuntimeRpc(target, 'jira.status', ...)
    //   else
    //     → window.api.jira.status()
    //        → IPC → Main → kiểm tra stored Jira token
  set({ jiraStatus: status, jiraStatusChecked: true, jiraStatusContextKey: contextKey })
}
```

### 3.2 — Kết nối Jira (user nhập credentials)

```
JiraIntegrationCard.tsx
  → user nhập { siteUrl, email, apiToken }
  → connectJira({ siteUrl, email, apiToken })
      → callRuntimeRpc(target, 'jira.connect', { siteUrl, email, apiToken })
        hoặc window.api.jira.connect(...)
          → IPC → Main → lưu Jira API token
          ← JiraConnectResult { ok, viewer }
  → checkJiraConnection()  // refresh status
```

---

## LUỒNG 4: Derive integration connection status (UI logic)

```
useIntegrationConnectionStatus()   [use-integration-connection-status.ts:221]
  Reads from store:
    preflightStatus, linearStatus, jiraStatus, ...
  
  deriveIntegrationConnectionStatus({...}) → {
    reviewConnected: githubConnected || gitlabConnected || bitbucketConnected || ...
    reviewProviderName: 'GitHub' | 'GitLab' | ...
    trackerProviderName: 'Linear' | 'Jira' | null
    checking: boolean  // true nếu chưa có kết quả
  }
  
  → ConnectIntegrationsList.tsx dùng để:
    - Hiển thị step 1 (Review) hoặc step 2 (Task) theo trạng thái
    - Toggle "done"/"active"/"upcoming" cho từng step
    - Hiển thị summary text khi đã connected
```

---

## Recheck Server — Cơ chế hỗ trợ server

### Remote Preflight qua Onboarding IPC

Khi Orca Server muốn kiểm tra trạng thái integrations trên Dev Machine:

```
main/ipc/onboarding-ipc.ts:166
ipcMain.handle('onboarding.getPreflightStatus', async (_event, { devServerId, force }) => {
  // Cache hit nếu !force && < PREFLIGHT_CACHE_TTL_MS
  const cached = preflightCache.get(devServerId)
  if (!force && cached && Date.now() - cached.cachedAt < TTL) return cached.result

  // SSH relay call đến Dev Machine
  const relay = devServerManager.getRelay(devServerId)
  const raw = await relay.call<{ gh, git, platform }>('preflight.check', {}, 30_000)
              ↓
  // relay binary trên Dev Machine (172.20.2.31)
  // src/relay/preflight-handler.ts:57
  PreflightHandler.checkFullPreflight()
    → execFile('gh', ['--version'])   → installed?
    → execFile('gh', ['auth', 'status'])   → authenticated?
    → execFile('git', ['--version'])   → installed?
    → execFile('git', ['config', '--global', 'user.name'])  → hasUserName?
    ← { platform, gh: { installed, authenticated, version }, git: { installed, hasUserName, hasUserEmail } }

  // Cache kết quả
  preflightCache.set(devServerId, { result, cachedAt: Date.now() })
  return result
})
```

### Force Recheck (Re-check button)

```
User click "Re-check" trong GitHub card:
  refresh() → refreshPreflightStatus({ force: true })
    ↓
    force = true → bỏ qua cache (cached = null chỉ reset với _resetPreflightCache())
    _resetKnownHostsCache()   // reset GitLab known-hosts cache (cần thiết sau glab auth login)
    ↓
    runPreflightCheck(true, context)
      → Chạy lại toàn bộ: git, gh, glab, bitbucket, azureDevOps, gitea checks
      → Cập nhật cached (nếu không phải WSL context)
      ← PreflightStatus mới nhất
```

### Cache Strategy

| Provider | Cache location | TTL | Reset condition |
|----------|---------------|-----|-----------------|
| `gh`/`glab`/`git`/`bitbucket`/etc. | `cached` module-level variable (`preflight.ts`) | Một lần per session | `force: true` hoặc restart |
| Remote preflight (dev server) | `preflightCache` Map trong `onboarding-ipc.ts` | `PREFLIGHT_CACHE_TTL_MS` | `force: true` |
| Linear status | Dedup in-flight request per contextKey | N/A (real-time) | Scope/context change |
| Jira status | Per contextKey | N/A (real-time) | Context change |

---

## File-by-File Reference

### UI Layer
| File | Vai trò |
|------|---------|
| [`ConnectIntegrationsList.tsx`](../../src/renderer/src/components/feature-wall/ConnectIntegrationsList.tsx) | Progressive 2-step setup (Feature Wall) |
| [`IntegrationsPane.tsx`](../../src/renderer/src/components/settings/IntegrationsPane.tsx) | Full settings pane |
| [`cli-source-control-integration-cards.tsx`](../../src/renderer/src/components/settings/cli-source-control-integration-cards.tsx) | GitHub + GitLab cards |
| [`token-source-control-integration-cards.tsx`](../../src/renderer/src/components/settings/token-source-control-integration-cards.tsx) | Bitbucket, Azure DevOps, Gitea cards |
| [`task-tracker-integration-cards.tsx`](../../src/renderer/src/components/settings/task-tracker-integration-cards.tsx) | Linear + Jira cards |

### Hooks & Logic
| File | Vai trò |
|------|---------|
| [`use-integration-provider-status-refresh.ts`](../../src/renderer/src/components/settings/use-integration-provider-status-refresh.ts) | Auto-refresh on mount + context change |
| [`use-integration-connection-status.ts`](../../src/renderer/src/components/feature-wall/use-integration-connection-status.ts) | Derive connected/checking state từ store |
| [`source-control-preflight-card-status.ts`](../../src/renderer/src/components/settings/source-control-preflight-card-status.ts) | Per-card status derivation + refresh() |
| [`integrations-pane-status.ts`](../../src/renderer/src/components/settings/integrations-pane-status.ts) | Mapping PreflightStatus → UI states |

### Store Slices
| File | Vai trò |
|------|---------|
| [`store/slices/preflight.ts`](../../src/renderer/src/store/slices/preflight.ts) | `refreshPreflightStatus` — routing + state |
| [`store/slices/linear.ts`](../../src/renderer/src/store/slices/linear.ts) | `checkLinearConnection`, `linearStatus`, connect/disconnect |
| [`store/slices/jira.ts`](../../src/renderer/src/store/slices/jira.ts) | `checkJiraConnection`, `jiraStatus`, connect/disconnect |

### Runtime Client (Routing Layer)
| File | Vai trò |
|------|---------|
| [`runtime/runtime-rpc-client.ts`](../../src/renderer/src/runtime/runtime-rpc-client.ts) | `callRuntimeRpc` — local vs environment routing |
| [`runtime/runtime-linear-client.ts`](../../src/renderer/src/runtime/runtime-linear-client.ts) | `linearStatus` + all Linear RPC calls |
| [`runtime/runtime-jira-client.ts`](../../src/renderer/src/runtime/runtime-jira-client.ts) | `jiraStatus` + all Jira RPC calls |

### Main Process (Electron IPC)
| File | Vai trò |
|------|---------|
| [`main/ipc/preflight.ts`](../../src/main/ipc/preflight.ts) | `runPreflightCheck` — execFile gh/glab/git |
| [`main/ipc/onboarding-ipc.ts`](../../src/main/ipc/onboarding-ipc.ts) | Remote preflight qua relay (dev server) |
| [`main/runtime/rpc/methods/preflight.ts`](../../src/main/runtime/rpc/methods/preflight.ts) | `PREFLIGHT_METHODS` — RPC handler registration |

### Relay (Dev Machine)
| File | Vai trò |
|------|---------|
| [`relay/preflight-handler.ts`](../../src/relay/preflight-handler.ts) | `PreflightHandler.checkFullPreflight()` trên dev machine |

---

## Sơ đồ tổng thể

```
┌──────────────────────────────────────────────────────────────────────┐
│ BROWSER (React UI)                                                   │
│  IntegrationsPane / ConnectIntegrationsList                          │
│  └── useIntegrationProviderStatusRefresh() → mount trigger          │
│  └── GitHubCard → "Re-check" button                                 │
│  └── LinearCard → "Add Linear access" → API Key Dialog              │
│  └── JiraCard   → "Connect" → credentials form                      │
└─────────────────────────┬────────────────────────────────────────────┘
                          │ Zustand store action calls
                          ▼
┌──────────────────────────────────────────────────────────────────────┐
│ RENDERER STORE                                                       │
│  refreshPreflightStatus() → routing theo activeRuntimeEnvironmentId │
│  checkLinearConnection()  → linearStatus() → routing                │
│  checkJiraConnection()    → jiraStatus() → routing                  │
└─────────┬───────────────────────────┬────────────────────────────────┘
          │ LOCAL (Electron)          │ ENVIRONMENT (Remote/Web)
          ▼                           ▼
┌─────────────────────┐   ┌────────────────────────────────────────────┐
│ Electron IPC        │   │ WebSocket JSON-RPC                         │
│ preflight:check →   │   │ callRuntimeRpc → window.api.runtime.call   │
│ runPreflightCheck() │   │  → ws://orca-server:6768                   │
│  execFile(gh,glab)  │   │  → OrcaRuntimeRpcServer                   │
│  getBitbucketAuth() │   │  → PREFLIGHT_METHODS / LINEAR_METHODS      │
│  getAzureDevOps()   │   │  → runPreflightCheck() / linearStatus()    │
│  getGiteaAuth()     │   └────────────────────────────────────────────┘
│                     │
│ linear:status →     │   ┌────────────────────────────────────────────┐
│ → stored keys       │   │ DEV MACHINE SSH RELAY (172.20.2.31)        │
│                     │   │ onboarding.getPreflightStatus              │
│ jira:status →       │   │  → relay.call('preflight.check', {})      │
│ → stored token      │   │  → PreflightHandler.checkFullPreflight()  │
└─────────────────────┘   │  → execFile('gh', 'git', ...) trên remote │
                          └────────────────────────────────────────────┘
```

---

## Điểm cần recheck

- [x] `PREFLIGHT_CACHE_TTL_MS` = `30_000` ms (30 giây) — confirmed trong `onboarding-ipc.ts:61`
- [x] Cache strategy: module-level `cached` (per-session, reset với `force:true`) — confirmed `preflight.ts:63`
- [ ] Xác nhận `getProviderRuntimeContextKey()` — logic tạo context key để dedup checks
- [ ] Xác nhận `getLocalPreflightContext()` — khi nào WSL context được dùng
- [ ] Check: Linear/Jira trong web mode — credential storage (keychain trên server?)
- [ ] Check: Bitbucket, Azure DevOps, Gitea credentials được store ở đâu?
- [ ] Verify WSL preflight flow — `detectWslCommandsOnPath` thực sự hoạt động thế nào

---

## Fix 2026-07-25 — Web Server Mode

### Vấn đề ban đầu
Trên `https://b15.openledger.vn`:
- GitHub card hiển thị "**Unavailable**" thay vì "Not authenticated"
- "**Re-check**" không có tác dụng
- "**Open Remote Servers**" click không phản hồi gì

### Nguyên nhân root
1. `gh` CLI **không được cài** trong Docker image → `runPreflightCheck()` → `gh.installed=false`
2. Nếu WebSocket RPC error → `preflightStatusError !== null` → `unavailable=true` → "Unavailable"
3. `ProviderHostScopeControl.tsx` luôn hiển thị "Open Remote Servers" button dù trong web mode pane `servers` không render

### Fix áp dụng

**[P0] Thêm `gh` v2.96.0 vào Dockerfile:**
```dockerfile
# deploy/dev/docker/orca/Dockerfile — Stage 2: Runtime
RUN apt-get install -y git openssh-client wget curl ca-certificates \
  && curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg \
       | dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg \
  && apt-get update && apt-get install -y gh
# → gh version 2.96.0 ✅ → ghStatus = 'not-authenticated' thay vì 'unavailable'
```

**[P1] Ẩn "Open Remote Servers" trong web mode:**
```typescript
// ProviderHostScopeControl.tsx
useEffect(() => {
  window.api.cli.getInstallStatus().then(status => {
    if (status.unsupportedReason === 'launch_mode_unavailable')
      setIsWebServerMode(true)
  })
}, [])
// → button bị ẩn trong web browser mode
```

**[P2] Auto-done `task-sources` step trong web server mode:**
```typescript
// feature-wall-setup-progress.ts
'task-sources': input.taskSourcesUnavailable === true || input.hasConnectedTaskSource,

// use-setup-guide-progress.ts
taskSourcesUnavailable: agentCapabilitiesUnavailable,  // same condition as CLI step
// → trong web mode: task-sources auto ✅ (không block Getting Started)
```

### Kết quả sau fix
- GitHub: "**Not authenticated**" + hướng dẫn `gh auth login` ✅
- "Re-check": hoạt động bình thường ✅
- "Open Remote Servers": ẩn trong web mode ✅
- `task-sources` step: auto-done trong web server mode ✅
