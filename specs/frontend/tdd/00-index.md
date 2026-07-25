**Version:** 4.0 (restructure_v1 + onboarding + remote-server + login CRs)  
**Date:** 2026-07-24  
**Updated:** 2026-07-24 (v4.0 — login CRs: Auth, Admin Panel, SSH UI, Per-User Sandbox)  
**Source:** `src/renderer/src/` + `src/shared/` + `src/platform/adapters/web/`  
**Change Requests:** [restructure_v1](../../../docs/crs/v1/restructure_v1/) | [onboarding](../crs/v1/onboarding/) | [remote-server](../crs/v1/remote-server/) | [login](../crs/v1/login/)  
**Solutions:** [restructure_v1 ✅](../crs/v1/restructure_v1/solutions/) | [onboarding ✅](../crs/v1/onboarding/solutions/) | [remote-server ✅](../crs/v1/remote-server/solutions/) | [login ✅](../crs/v1/login/solutions/)

---

## Tài liệu trong bộ TDD này

| # | File | Nội dung | Status |
|---|------|---------|----|
| 1 | [01-architecture-overview.md](./01-architecture-overview.md) | Kiến trúc tổng thể, render targets, build | ✅ v2.0 |
| 2 | [02-state-management.md](./02-state-management.md) | Zustand store, slices, selectors | ✅ v4.0 |
| 3 | [03-runtime-client-layer.md](./03-runtime-client-layer.md) | WebSocket RPC client, sync graph, web runtime session | ✅ v2.0 |
| 4 | [04-terminal-subsystem.md](./04-terminal-subsystem.md) | xterm.js integration, PTY transport, pane layout | v1.x |
| 5 | [05-ui-components.md](./05-ui-components.md) | Component tree, App shell, major screens | ✅ v4.0 |
| 6 | [06-web-client.md](./06-web-client.md) | Web client mode, pairing, WebConnect, auth routing | ✅ v4.0 |
| 7 | [07-hooks-and-ipc.md](./07-hooks-and-ipc.md) | Custom hooks, IPC events, useIpcEvents | v1.x |
| 8 | [08-editor-and-files.md](./08-editor-and-files.md) | Editor slice, code editor, file explorer | v1.x |
| 9 | [09-onboarding-devserver.md](./09-onboarding-devserver.md) | **[NEW]** Onboarding, Dev Server UI, Agent Detection, Web Push | ✅ v3.0 NEW |
| 10 | [10-fleet-management.md](./10-fleet-management.md) | **[NEW]** Fleet Inventory, Health Monitoring, Bootstrap, RBAC | ✅ v3.0 NEW |

---

## Tóm tắt kiến trúc — restructure_v1

```
┌──────────────────────────────────────────────────────────────────────────┐
│              RENDERER PROCESS (Browser/Electron)                       │
│                    React + Zustand + Vite                              │
│                                                                        │
│  Desktop entry:          │  Web entry:                                │
│  src/renderer/main.tsx   │  src/renderer/web/main.tsx                 │
│        ↓                 │        ↓                                   │
│  RecoverableErrorBoundary│  bootstrapWebApp() [MỚI]                   │
│        ↓                 │   + SW registration [MỚI v3.0]             │
│      App.tsx (~127KB)    │   + checkAuthSession() [MỚI v4.0]          │
│  (same App for both!)    │  WebSocketRpcClient [MỚI]                  │
│                          │        ↓                                   │
│                          │  ↓ authenticated?                           │
│                          │  ├─ YES → App.tsx (full Orca UI)           │
│                          │  ├─ NO  → LoginPage [MỚI v4.0]            │
│                          │  └─ PAIR→ WebConnect (backward compat)      │
│                          │                                            │
│  Admin SPA (riêng):       │  /admin/                                   │
│  admin-index.html → AdminApp.tsx [MỚI v4.0]                           │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │         Zustand Store (useAppStore)                             │   │
│  │  slices: repos, worktrees, terminals, tabs, ui, ssh,           │   │
│  │          devServers [NEW v3.0], remoteAgents [NEW],            │   │
│  │          fleetHealth [NEW], preflight [EXT],                   │   │
│  │          auth [NEW v4.0] — AuthSlice                           │   │
│  │          ssh.sshUserAccounts [EXT v4.0]                        │   │
│  └─────────────────────────────────┴─────────────────────────────────┘   │
│                              │                                         │
│  ┌──────────────────────────┴──────────────────────────────────┐   │
│  │         Runtime Client Layer (src/runtime/)                     │   │
│  │  - sync-runtime-graph.ts  (state sync)                          │   │
│  │  - runtime-rpc-client.ts  (callRuntimeRpc)                      │   │
│  │  - web-runtime-session.ts (web terminal ops)                    │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                                                        │
│  Platform adapters (src/platform/):                                    │
│  - rpc-client-interface.ts (IRpcClient) [restructure_v1]              │
│  - adapters/web/rpc-client.ts (WebSocketRpcClient) [restructure_v1]   │
│                                                                        │
│  Onboarding & Fleet (v3.0):                                            │
│  - components/onboarding/ (DevServerStep, AgentStep, ...)             │
│  - components/fleet/ (FleetHealthDashboard, BootstrapStatusPanel, ...) │
│  - service-worker.js (Web Push) [onboarding CRs]                      │
│                                                                        │
│  Auth & Admin (v4.0) [login CRs]:                                      │
│  - auth/ (auth-types, auth-api-client, auth-utils)                    │
│  - web/login/ (LoginPage, LoginForm, SsoButton, PairCodeFallback)     │
│  - components/auth/ (UserAvatarMenu, UserRoleBadge)                   │
│  - components/admin/ (AdminApp, Dashboard, Users, Sessions, ...)      │
│  - components/ssh/ (SshProvisioningProgress, SshUserIndicator)        │
└──────────────────────────────────────────────────────────────────────────┘
                    │                         │
           Electron IPC (Desktop)     WebSocket (Web)
           window.api.*              ws://orca-server:6768
                    │                         │
              Main Process (Backend) / OrcaRuntimeRpcServer
```

---

## Nguyên tắc thiết kế

1. **Two render targets**: Desktop (Electron) và Web Browser dùng **cùng `App.tsx`** — tách biệt qua `window.api` abstraction
2. **Zustand + slices**: 40+ slices composable, không Redux
3. **Sync graph**: State từ backend sync vào Zustand qua `scheduleRuntimeGraphSync()`
4. **xterm.js**: Terminal rendering qua WebGL; custom PTY transport layer
5. **Lazy loading**: Tất cả heavy components lazy-loaded để tránh blocking
6. **i18n first**: Mọi user-facing text qua `translate()` / `useTranslation()`

**Nguyên tắc bổ sung — restructure_v1:**

7. **`bootstrapWebApp()` testable**: Web entry bootstrap tách khỏi side-effects của `main.tsx`
8. **`IRpcClient` interface**: Tách transport khỏi consumer — `ConnectionStatusProvider` không biết WebRuntimeClient hay WebSocketRpcClient
9. **`ConnectionStatus` as React context**: Không global variable, polling-based, web-only
10. **`web-preload-api.ts` immutable**: File 135KB không bị thay đổi — chỉ thêm adapters ở lớp platform
11. **App.tsx không thay đổi**: Không sửa App.tsx — chỉ entry point và web-specific wrappers
12. **Cleanup required**: Tất cả `on*()` methods phải có `off*()` tương ứng

**Nguyên tắc bổ sung — login CRs (v4.0):**

18. **Auth-first boot** — `bootstrapWebApp()` check `/auth/me` trước khi render bất kỳ UI nào
19. **Additive-only** — Không sửa `App.tsx`, `main.tsx`, `web-preload-api.ts`
20. **AuthSlice tập trung** — Toàn bộ auth state trong 1 Zustand slice `auth.ts`
21. **Admin SPA riêng** — `admin-index.html` → `AdminApp.tsx` — không thuộc App.tsx
22. **Web-only UI** — Login/SSO/Avatar chỉ xuất hiện khi `ORCA_PLATFORM === 'web'`
23. **SSH provisioning UI** — `SshUserIndicator` inject vào `SshTargetRow` (additive, không sửa core sidebar)
24. **toLinuxUsername() sync** — Frontend mirror chính xác backend impl (lowercase, regex, truncate 20)

---

## Addendum A: restructure_v1 — COMPLETE ✅

> **Tests:** 34/34 pass | **Solutions:** 4/4 done

### Files mới tạo

| File | Layer | Vai trò |
|------|-------|---------|
| `src/platform/rpc-client-interface.ts` | Platform | `IRpcClient` — transport abstraction |
| `src/platform/adapters/web/rpc-client.ts` | Adapter | `WebSocketRpcClient` — JSON-RPC over WS |
| `src/renderer/src/web/main-web-bootstrap.tsx` | Entry | `bootstrapWebApp()` — testable init |
| `src/renderer/src/web/ConnectionStatusProvider.tsx` | UI | React context + 3 hooks |
| `src/renderer/src/web/ConnectionStatusBanner.tsx` | UI | Fixed-position disconnect overlay |
| `scripts/audit-window-api-coverage.ts` | Tooling | API surface coverage check |

### Test files (34/34 pass ✅)

| File | Tests |
|------|-------|
| `src/platform/adapters/web/__tests__/rpc-client.test.ts` | 15 ✅ |
| `src/renderer/src/web/__tests__/ConnectionStatusProvider.test.tsx` | 5 ✅ |
| `src/renderer/src/web/__tests__/ConnectionStatusBanner.test.tsx` | 6 ✅ |
| `src/renderer/src/web/__tests__/web-index-html.test.ts` | 5 ✅ |
| `src/renderer/src/web/__tests__/preload-no-change.test.ts` | 3 ✅ |

---

### Không thay đổi (backward compat)

| File | Lý do giữ nguyên |
|------|----------------|
| `src/renderer/src/main.tsx` | Desktop entry |
| `src/renderer/src/web/main.tsx` | Đã delegate sang `bootstrapWebApp()` |
| `src/renderer/src/App.tsx` | Same App cho cả Desktop và Web |
| `src/preload/index.ts` | Electron preload |
| `src/renderer/src/web/web-preload-api.ts` | 135KB complete — không rewrite |

---

## Addendum B: onboarding CRs (CR-OB-001~009) — COMPLETE ✅

> **TDD:** [TDD-FE-09: Onboarding](./09-onboarding-devserver.md) | **Solutions:** 4/4 done

Key new files:
- `src/shared/dev-server-types.ts` — DevServer, WindowsTerminalCapabilities, ConnectionTestResult
- `src/renderer/src/store/slices/dev-servers.ts` — DevServerSlice
- `src/renderer/src/components/onboarding/DevServerStep.tsx` — Wizard step
- `src/renderer/src/components/dev-server/` — DevServerCard, List, Dialog, StatusBadge
- `src/renderer/src/components/remote-browser/` — RemoteDirectoryBrowser
- `src/renderer/src/hooks/useRemoteAgentDetection.ts` — module cache, 60s TTL
- `src/renderer/src/hooks/useRemoteWindowsTerminalCapabilities.ts` — capsCache, 60s TTL
- `src/renderer/src/hooks/useBrowserNotificationPermission.ts` — Web Push
- `src/renderer/src/hooks/useWebPushSubscription.ts` — VAPID subscribe
- `src/renderer/service-worker.js` — Push event handler
- `src/renderer/src/web/main-web-bootstrap.tsx` — MODIFY: SW registration

Store extensions: `onboarding.ts` (agentDetectionByServer), `preflight.ts` (remotePreflightByServer).

---

## Addendum D: login CRs (CR-LOGIN-001~004) — COMPLETE ✅

> **TDD refs:** TDD-FE-06 (Web Client), TDD-FE-02 (State Management), TDD-FE-05 (UI Components), TDD-FE-09 (Onboarding)  
> **Solutions:** [4/4 done](../crs/v1/login/solutions/) | **Tests:** 104/104 pass

### CR-LOGIN-001 — Auth (SOL-FE-LG-001 + SOL-FE-LG-002)

Key new files:
- `src/renderer/src/auth/auth-types.ts` — `AuthUser`, `AuthState`, `AuthError`, `SsoProvider`
- `src/renderer/src/auth/auth-api-client.ts` — `fetchCurrentUser()`, `loginLocal()`, `logoutUser()`, `fetchAuthConfig()`
- `src/renderer/src/store/slices/auth.ts` — `AuthSlice` (Zustand: `auth`, `setAuth`, `clearAuth`, `checkSession`)
- `src/renderer/src/web/login/LoginPage.tsx` — Login page + SSO routing
- `src/renderer/src/web/login/LoginForm.tsx` — Email/password form
- `src/renderer/src/web/login/SsoButton.tsx` — GitHub/Google/Keycloak link
- `src/renderer/src/web/login/PairCodeFallback.tsx` — Backward compat pairing
- `src/renderer/src/hooks/useAuthSession.ts` — `useAuthUser()`, `useIsAuthenticated()`, `useAuthStatus()`
- `src/renderer/src/hooks/useLogout.ts` — `useLogout()` hook
- `src/renderer/src/components/auth/UserAvatarMenu.tsx` — Avatar dropdown (web-only)
- `src/renderer/src/components/auth/UserRoleBadge.tsx` — Role indicator badge

File được sửa:
- `src/renderer/src/web/main-web-bootstrap.tsx` — Thêm `checkAuthSession()` + `renderLoginPage()`
- `src/renderer/src/store/index.ts` — Register `createAuthSlice`
- `src/renderer/src/components/orca-profile-switcher/OrcaProfileSwitcher.tsx` — Thêm `UserAvatarMenu` (web + authenticated)

### CR-LOGIN-003 — SSH Dev Isolation (SOL-FE-LG-004)

Key new files:
- `src/renderer/src/auth/auth-utils.ts` — `toLinuxUsername(email)` (mirror backend)
- `src/renderer/src/components/ssh/SshProvisioningProgress.tsx` — Progress bar component
- `src/renderer/src/components/ssh/SshUserIndicator.tsx` — Username + status badge
- `src/renderer/src/hooks/useSshUserAccount.ts` — Fetch SSH user per server
- `src/renderer/src/hooks/useSshProvisioning.ts` — Subscribe provisioning events

Store extension:
- `src/renderer/src/store/slices/ssh.ts` — EXTENDED: `ProvisioningStatus`, `SshUserAccount`, `sshUserAccounts: Map<string, SshUserAccount>`

### CR-LOGIN-004 — Admin UI (SOL-FE-LG-003)

Key new files:
- `src/renderer/src/components/admin/admin-api-client.ts` — fetch wrapper `/admin/api/*`
- `src/renderer/src/components/admin/AdminApp.tsx` — Admin SPA root (React Router)
- `src/renderer/src/components/admin/AdminDashboard.tsx` — Stats + active sessions
- `src/renderer/src/components/admin/UsersPage.tsx` + `UserForm.tsx` — CRUD
- `src/renderer/src/components/admin/SessionsPage.tsx` — Kill sessions
- `src/renderer/src/components/admin/PoliciesPage.tsx` + `PolicyForm.tsx` — Access policies
- `src/renderer/src/components/admin/AuditPage.tsx` — Audit log + date filter
- `src/renderer/src/admin/admin-main.tsx` — Admin SPA entry
- `src/renderer/admin-index.html` — Admin SPA HTML entry

Build config:
- `vite.web.config.ts`, `vite.web-spa.config.ts`, `electron.vite.config.ts` — `admin-index.html` added as separate entry

### Tests (104/104 ✅)

| File | Tests |
|------|-------|
| `auth/__tests__/auth-api-client.test.ts` | 10 ✅ |
| `web/login/__tests__/LoginPage.test.tsx` | 8 ✅ |
| `web/login/__tests__/LoginForm.test.tsx` | 4 ✅ |
| `web/login/__tests__/SsoButton.test.tsx` | 4 ✅ |
| `hooks/__tests__/useAuthSession.test.ts` | 6 ✅ |
| `hooks/__tests__/useLogout.test.ts` | 3 ✅ |
| `components/auth/__tests__/UserAvatarMenu.test.tsx` | 8 ✅ |
| `components/auth/__tests__/UserRoleBadge.test.tsx` | 4 ✅ |
| `components/admin/__tests__/admin-api-client.test.ts` | 7 ✅ |
| `components/admin/__tests__/AdminDashboard.test.tsx` | 3 ✅ |
| `components/admin/__tests__/UsersPage.test.tsx` | 5 ✅ |
| `components/admin/__tests__/SessionsPage.test.tsx` | 3 ✅ |
| `components/admin/__tests__/AuditPage.test.tsx` | 3 ✅ |
| `auth/__tests__/auth-utils.test.ts` | 4 ✅ |
| `components/ssh/__tests__/SshProvisioningProgress.test.tsx` | 3 ✅ |
| `components/ssh/__tests__/SshUserIndicator.test.tsx` | 4 ✅ |
| `hooks/__tests__/useSshUserAccount.test.ts` | 3 ✅ |

---

## Addendum C: remote-server CRs (CR-001~006) — COMPLETE ✅ (Phase 1+2)

> **TDD:** [TDD-FE-10: Fleet Management](./10-fleet-management.md) | **Solutions:** 6/6 done

Key new files:
- `src/renderer/src/store/slices/ssh.ts` — EXTENDED: fleetImportStatus, serverHealthMetrics, fleetAlerts, groups, bootstrap, RBAC
- `src/renderer/src/components/fleet/` — FleetImportDialog, FleetHealthDashboard, BulkProvisioningWizard, BootstrapStatusPanel, UserProfileBadge
- `src/renderer/src/hooks/useFleetHealthPolling.ts` — 30s polling, IPC events
- `src/renderer/src/hooks/useFleetImport.ts` — YAML import with progress
- `src/renderer/src/hooks/useBootstrapAutomation.ts` — 7-step bootstrap tracking
- `src/renderer/src/hooks/useServerGroups.ts` — grouping + filter
- `src/renderer/src/hooks/useBulkProvisioning.ts` — parallel provision

Phase 3 (OIDC/SSO UI) deferred.

---

## Test Setup cho Frontend

```typescript
// config/vitest.config.ts — node environment với per-file overrides
{
  test: {
    environment: 'node',   // default
    include: [
      'src/**/*.test.ts',
      'src/**/*.test.tsx'
    ]
  }
}

// Với React tests — dùng happy-dom per file:
// @vitest-environment happy-dom
// import '@testing-library/jest-dom/vitest'
// afterEach(() => cleanup())
```

### Vitest mock patterns

```typescript
// MockWebSocket (Node env — phải dùng onX callbacks, KHÔNG EventTarget)
const MockWsConstructor = vi.fn(function(this: unknown, url: string) {
  mockWs = new MockWebSocket(url)
  return mockWs
}) as unknown as typeof WebSocket
vi.stubGlobal('WebSocket', MockWsConstructor)

// connectClient helper — timing-safe
async function connectClient(client: WebSocketRpcClient): Promise<void> {
  const connectPromise = client.connect()
  await Promise.resolve()   // flush microtask — ensure onopen is assigned
  mockWs.simulateOpen()
  await connectPromise
}
```
