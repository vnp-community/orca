# Luồng: GitHub Integration — Cơ chế hoạt động đầy đủ

**Ngày ghi:** 2026-07-25  
**Trạng thái:** VERIFIED — đã xác nhận với code thực tế và server production  
**Server:** `172.20.2.39` · **Public:** `https://b15.openledger.vn`

---

## Tổng quan

GitHub Integration trong Orca sử dụng **GitHub CLI (`gh`)** làm công cụ xác thực và kiểm tra trạng thái — không dùng OAuth flow hay API key trực tiếp.

| Trạng thái | Điều kiện | Hiển thị UI |
|-----------|----------|------------|
| `checking` | Đang fetch preflight status | Spinner |
| `connected` | `gh` installed **và** authenticated | ✅ Connected |
| `not-installed` | `gh` binary không tồn tại trong PATH | ❌ Not installed |
| `not-authenticated` | `gh` installed nhưng chưa login | ⚠️ Not authenticated |
| `unavailable` | `preflightStatusError !== null` hoặc không có `cliStatus` | ⚠️ Unavailable |

---

## Kiến trúc

```
Browser (React UI)
  GitHubIntegrationCard.tsx
    usePreflightCardStatuses('gh')
      ↓ reads from Zustand store
  AppStore (preflight slice)
    refreshPreflightStatus()
      ↓ routing theo activeRuntimeEnvironmentId
      ┌─── kind:'local' (Electron) ────────────────────────────────┐
      │  window.api.preflight.check(preflightArgs)                  │
      │  → Electron IPC → Main process                              │
      │  → runPreflightCheck() → execFile('gh', ...) locally       │
      └─────────────────────────────────────────────────────────────┘
      ┌─── kind:'environment' (Remote/Web) ─────────────────────────┐
      │  callRuntimeRpc(target, 'preflight.check', ...)              │
      │  → WebSocket JSON-RPC → Orca Server container               │
      │  → PREFLIGHT_METHODS handler → runPreflightCheck()          │
      └─────────────────────────────────────────────────────────────┘
```

---

## LUỒNG 1: `refreshPreflightStatus()` — Store action

**File:** `src/renderer/src/store/slices/preflight.ts:78`

```typescript
refreshPreflightStatus: async (options) => {
  const force = options?.force === true
  const context = getLocalPreflightContext(get())     // { wslDistro?, projectRuntime?, ... }
  const contextKey = localPreflightContextKey(context) // fingerprint string

  // ① Dedup: nếu đã có request đang chạy với cùng contextKey → reuse
  if (!force && nonForcedPreflightRequest?.key === contextKey)
    return nonForcedPreflightRequest.promise

  // ② Routing
  const runtimeTarget = getActiveRuntimeTarget(get().settings)
  //   → settings.activeRuntimeEnvironmentId?.trim()
  //   → truthy: { kind: 'environment', environmentId }
  //   → falsy:  { kind: 'local' }

  const request =
    runtimeTarget.kind === 'environment'
      ? callRuntimeRpc<PreflightStatus>(runtimeTarget, 'preflight.check', force ? {force} : {})
      : window.api.preflight.check(preflightArgs)    // Electron IPC

  .then(status => set({
    preflightStatus: status,
    preflightStatusChecked: true,
    preflightStatusContextKey: contextKey,
    preflightStatusLoading: false,
    preflightStatusError: null
  }))
  .catch(error => set({
    preflightStatusChecked: true,
    preflightStatusContextKey: contextKey,
    preflightStatusLoading: false,
    preflightStatusError: getErrorMessage(error)  // ← "Unavailable" xảy ra ở đây
  }))
}
```

---

## LUỒNG 2: `runPreflightCheck()` — Core logic

**File:** `src/main/ipc/preflight.ts:227`

```typescript
async function runPreflightCheck(force?: boolean, context?: LocalPreflightContext) {
  // Cache: module-level `cached`, reset khi force=true
  if (!force && cached !== null) return cached

  // ① Detect binary presence (parallel)
  const [gitProbe, ghProbe, glabProbe] = await Promise.all([
    detectCommandRuntime('git', context),   // which git / where.exe
    detectCommandRuntime('gh',  context),   // which gh
    detectCommandRuntime('glab', context)   // which glab
  ])

  // ② Auth check nếu installed
  const [ghAuthenticated, ...] = await Promise.all([
    ghProbe.installed
      ? isGhAuthenticated(ghProbe.wslTarget)   // execFile('gh', ['auth', 'status'])
      : Promise.resolve(false),                // exit 0 = authenticated
    // glab, bitbucket, azureDevOps, gitea...
  ])

  cached = {
    git:  { installed: gitProbe.installed },
    gh:   { installed: ghProbe.installed, authenticated: ghAuthenticated },
    glab: { installed: glabProbe.installed, authenticated: glabAuthenticated },
    bitbucket, azureDevOps, gitea
  }
  return cached
}

// isGhAuthenticated:
async function isGhAuthenticated(wslTarget?) {
  const [,, exitCode] = await execLocalPreflightCommand('gh', ['auth', 'status'])
  return exitCode === 0   // exit 0 = authenticated, exit 1 = not
}
```

---

## LUỒNG 3: Status derivation — UI layer

**File:** `src/renderer/src/components/settings/source-control-preflight-card-status.ts`

```typescript
// Hook trong GitHubIntegrationCard:
const { statuses, unavailable, refresh } = usePreflightCardStatuses('gh')
const status = unavailable ? 'unavailable' : statuses.ghStatus

// unavailable = true khi:
const unavailable =
  !preflightStatusLoading &&
  preflightStatusChecked &&
  (preflightStatusContextKey === expectedContextKey) &&  // preflightCurrent
  preflightStatusError !== null                          // ← WS/RPC error

// deriveCliProviderCardState → CliProviderCardState:
// 'checking'          nếu loading || !checked || !current
// 'unavailable'       nếu error !== null || !statusAvailable || !cliStatus
// 'not-installed'     nếu !cliStatus.installed
// 'not-authenticated' nếu installed && !authenticated
// 'connected'         nếu installed && authenticated
```

---

## LUỒNG 4: "Re-check" button

```typescript
// usePreflightCardStatuses.refresh():
const refresh = (): void => {
  setRefreshing(true)
  void refreshPreflightStatus({ force: true })   // force=true → bỏ cache
    .finally(() => setRefreshing(false))
}
// → runPreflightCheck(true) → reset `cached` → chạy lại toàn bộ execFile
```

**Khi nào Re-check hiện:**
- `status === 'unavailable'` → button "Re-check"
- `status === 'not-installed'` → button "Install GitHub CLI" + "Re-check"
- `status === 'not-authenticated'` → hướng dẫn `gh auth login` + "Re-check"

---

## LUỒNG 5: Web/Server mode (`b15.openledger.vn`)

### Routing decision

```
WebRoot (sessionUser !== null) → installWebPreloadApi()
  activeEnvironment = readStoredWebRuntimeEnvironment()
    → localStorage key: 'orca.web.runtimeEnvironment.v1'
```

**Case A — Không có stored environment:**
```
→ runtimeTarget = { kind: 'local' }
→ window.api.preflight.check()
→ createPreflightApi().check():
    if (!requireActiveEnvironmentOrNull()) return fallbackStatus
    // fallbackStatus.gh = { installed: false, authenticated: false }
→ ghStatus = 'not-installed'
```

**Case B — Có stored environment (đã pair):**
```
→ runtimeTarget = { kind: 'environment', environmentId }
→ callRuntimeRpc → WebSocket → ws://172.20.2.39:6768
→ PREFLIGHT_METHODS → runPreflightCheck()
  → which gh → /usr/bin/gh (v2.96.0) ✅
  → gh auth status → exit 1 (chưa login)
← { gh: { installed: true, authenticated: false } }
→ ghStatus = 'not-authenticated' ✅
```

---

## Docker container — `gh` CLI

### Fix 2026-07-25 (Dockerfile stage runtime)

```dockerfile
# Trước: gh NOT installed → ghStatus = 'not-installed' hoặc 'unavailable'
# Sau:
RUN apt-get install -y git openssh-client wget curl ca-certificates \
  && curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg \
       | dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg \
  && echo "deb [arch=$(dpkg --print-architecture) ...] https://cli.github.com/packages stable main" \
       > /etc/apt/sources.list.d/github-cli.list \
  && apt-get update && apt-get install -y gh

# Verified trên server:
# $ docker exec orca-server gh --version
# gh version 2.96.0 (2026-07-02)
```

---

## Cache strategy

| Tầng | Mechanism | TTL | Reset |
|------|-----------|-----|-------|
| Electron/Server local | Module-level `cached` variable | Per-session | `force: true` hoặc `_resetPreflightCache()` |
| Remote dev server relay | `preflightCache` Map per `devServerId` | `30_000 ms` | `force: true` |
| Request dedup | `nonForcedPreflightRequest` per contextKey | Single request | Auto-clear on settle |

---

## GitHub auth flow — User action

```bash
# Ngoài Orca (terminal)
$ gh auth login
  → chọn GitHub.com → HTTPS → Browser
  → Authenticate via github.com
  ✓ Logged in to github.com as <username>

# Trong Orca — Getting Started → GitHub → "Re-check"
→ refreshPreflightStatus({ force: true })
→ gh auth status → exit 0
← { gh: { installed: true, authenticated: true } }
→ ghStatus = 'connected' ✅
```

---

## File reference

| File | Vai trò |
|------|---------|
| `src/renderer/src/components/settings/cli-source-control-integration-cards.tsx` | `GitHubIntegrationCard` — UI + Re-check |
| `src/renderer/src/components/settings/source-control-preflight-card-status.ts` | `usePreflightCardStatuses` + `deriveCliProviderCardState` |
| `src/renderer/src/store/slices/preflight.ts` | `refreshPreflightStatus` — routing + state |
| `src/renderer/src/runtime/runtime-rpc-client.ts` | `getActiveRuntimeTarget` + `callRuntimeRpc` |
| `src/main/ipc/preflight.ts` | `runPreflightCheck` + `isGhAuthenticated` |
| `src/main/runtime/rpc/methods/preflight.ts` | `PREFLIGHT_METHODS` — RPC registration |
| `src/renderer/src/web/web-preload-api.ts` | Web stub: `createPreflightApi()` |
| `deploy/dev/docker/orca/Dockerfile` | Container — `gh` v2.96.0 installation |
