**Version:** 5.0 (restructure_v1 + onboarding + remote-server + login + v5.0 workspace CRs)  
**Date:** 2026-07-24  
**Updated:** 2026-07-30 (v5.0 — cross-references with HLD v1 C3, C4 + web-server-architecture.md)  
**Source:** `src/renderer/src/` + `src/shared/` + `src/platform/adapters/web/`  
**Change Requests:** [restructure_v1](../../../docs/crs/v1/restructure_v1/) | [onboarding](../crs/v1/onboarding/) | [remote-server](../crs/v1/remote-server/) | [login](../crs/v1/login/)  
**Solutions:** [restructure_v1 ✅](../crs/v1/restructure_v1/solutions/) | [onboarding ✅](../crs/v1/onboarding/solutions/) | [remote-server ✅](../crs/v1/remote-server/solutions/) | [login ✅](../crs/v1/login/solutions/)  
**HLD Reference:** [web-server-architecture.md](../../../docs/hld/web-server-architecture.md) | [HLD v1 C3](../../../docs/hld/v1/C3-components.md) | [HLD v1 C4](../../../docs/hld/v1/C4-code.md)

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

---

## v5.0 — New Frontend TDDs

| TDD | Domain | Feature(s) | ADR | HLD Ref | Status |
|-----|--------|-----------|-----|---------|--------|
| [TDD-FE-11](./11-profile-ui.md) | Profile Hierarchy UI | F33 | ADR-007 | C3.10, C4.7 | 🚧 In-Progress |
| [TDD-FE-12](./12-project-workspace-ui.md) | Project Workspace Shell | F34, F38 | ADR-011 | C3.12, C4.10 | 🚧 In-Progress |
| [TDD-FE-13](./13-ai-provider-ui.md) | AI Provider Admin UI | F35 | ADR-008 | C3.11a | 🚧 In-Progress |
| [TDD-FE-14](./14-workflow-ui.md) | Workflow Builder & Monitor | F36 | ADR-009 | C3.11c | 🚧 In-Progress |
| [TDD-FE-15](./15-task-graph-ui.md) | Task Graph UI | F37 | ADR-010 | C3.11b | 🚧 In-Progress |
| [TDD-FE-16](./16-remote-git-ui.md) | Remote Git UI | F39 | ADR-012 | C3.12, C4.10 | 🚧 In-Progress |
| [TDD-FE-17](./17-file-explorer-ui.md) | Remote File Explorer | F38 | ADR-011 | C3.12 | 🚧 In-Progress |

---

## Addendum E: v5.0 HLD Cross-References (2026-07-30)

> **Nguồn:** [web-server-architecture.md](../../../docs/hld/web-server-architecture.md), [HLD v1 C3](../../../docs/hld/v1/C3-components.md), [HLD v1 C4](../../../docs/hld/v1/C4-code.md)

### E.1 Zustand Slices mới (v5.0) — Đã xác nhận từ HLD C4.7–C4.10

| Slice | State chính | Actions chính | HLD Section |
|-------|-------------|--------------|-------------|
| `profile` | `resolvedProfile`, `companyProfile`, `deptProfile`, `userProfile` | `setResolved`, `updateUser`, `invalidate` | C4.7 |
| `project` | `projects[]`, `activeProjectId` | `setProjects`, `setActive`, `addMember` | C4.8 |
| `ai-provider` | `accounts[]`, `usage` | `setAccounts`, `updateStatus`, `recordUsage` | C4.9 |
| `workflow` | `templates[]`, `executions[]` | `setTemplates`, `addExecution`, `updateStep` | C4.9 |
| `task` | `tasks{}`, `activeTaskId`, `expandedNodes` | `setTasks`, `updateTask`, `setActive` | C4.9 |

### E.2 WorkspaceContext — State Interface (từ HLD C4.10)

```typescript
interface WorkspaceContextValue {
  // Project
  project: OrcaProject | null
  devServer: SshHost | null
  isConnected: boolean
  isOffline: boolean

  // Connection (proxied via RPC, managed backend)
  relay: DevServerRelayBridge | null

  // Worktree
  currentWorktree: Worktree | null
  availableWorktrees: Worktree[]
  setCurrentWorktree: (wt: Worktree) => void

  // Git
  gitStatus: GitStatus | null
  refreshGitStatus: () => Promise<void>

  // Profile (resolved từ backend C4.7)
  resolvedProfile: ResolvedProfile | null

  // Agent
  activeAgentSessionId: string | null
  setActiveAgentSession: (id: string | null) => void

  // Event bus
  emit: (event: WorkspaceEvent) => void
  on: (event: string, handler: Function) => () => void

  // Actions
  switchProject: (projectId: string) => Promise<void>
}

type WorkspaceEvent =
  | { type: 'agent.complete'; filesChanged: number }
  | { type: 'git.commit'; hash: string; message: string }
  | { type: 'git.push'; branch: string }
  | { type: 'worktree.switched'; path: string; branch: string }
  | { type: 'workflow.step.complete'; stepId: string; executionId: string }
```

### E.3 switchProject() Data Flow (từ HLD C4.10)

```
switchProject('proj-abc')
    ├── RPC: projects.get('proj-abc') → { devServerId, repoPath }
    ├── Backend: FleetHealthMonitor.getCached(devServerId) → 'healthy'
    ├── Backend: DevServerRelayBridge.connect(devServerId)
    ├── Promise.all([
    │     RPC: git.status({ cwd: repoPath }),
    │     RPC: git.worktree.list({ repoPath }),
    │     RPC: fs.readDir({ path: repoPath, depth: 2 }),
    │     RPC: workflows.getActiveExecutions('proj-abc'),
    │   ])
    ├── WorkspaceContext state = { project, devServer, relay, resolvedProfile,
    │                              gitStatus, availableWorktrees, fileTree }
    ├── Start git status poll (5s interval)
    └── UI renders: Explorer + GitPanel + AgentPanel ready
```

### E.4 Cross-panel Event Bus (từ HLD C3.12b)

| Event | Panels nhận | Hành động |
|-------|------------|----------|
| `agent.complete` | GitPanel, ExplorerPanel, TasksPanel, Banner | refresh gitStatus, git decorations, advance task |
| `git.commit` | TasksPanel, GitPanel, ExplorerPanel | scan message #task-id, refresh ahead/behind |
| `worktree.switched` | GitPanel, ExplorerPanel, TerminalPanel | reload status/tree/cwd |
| `workflow.step.complete` | GitPanel, TasksPanel | auto-fetch, refresh tasks |

### E.5 git.* RPC Methods Full List (từ HLD C4.10)

```typescript
// src/main/runtime/rpc/methods/git.ts (backend) — relay đến Dev Server
'git.status'           → exec('git status --porcelain=v2 --branch', { cwd })
'git.diff'             → exec('git diff [--staged] [--] [file]', { cwd })
'git.add'              → exec('git add <files>', { cwd })
'git.restore'          → exec('git restore [--staged] <files>', { cwd })
'git.commit'           → exec('git commit -m <msg> --author=...', { cwd })
'git.push'             → execStream('git push origin <branch>', { cwd })
'git.pull'             → execStream('git pull origin <branch>', { cwd })
'git.fetch'            → exec('git fetch --all', { cwd })
'git.branch.list'      → exec('git branch -a -vv', { cwd })
'git.branch.create'    → exec('git checkout -b <name> [from]', { cwd })
'git.branch.delete'    → exec('git branch -d <name>', { cwd })
'git.merge'            → exec('git merge --no-ff <branch>', { cwd })
'git.stash'            → exec('git stash push -m <msg>', { cwd })
'git.stash.pop'        → exec('git stash pop', { cwd })
'git.log'              → exec('git log --oneline --graph --decorate -50', { cwd })
```

### E.6 fs.* RPC Methods (từ HLD C4.10)

```typescript
'fs.readDir'  → relay: fs.readdir + stat per entry
'fs.readFile' → relay: fs.readFile (max 5MB)
'fs.stat'     → relay: fs.stat
'fs.glob'     → relay: glob(pattern, { cwd, ignore })
'fs.grep'     → relay: grep -rn --include=<ext> pattern cwd (limit 30)
```

### E.7 AI Provider RPC Methods (từ HLD C4.9)

```typescript
'profile.getEffective'      // (userId) → ResolvedProfile (cached 60s)
'profile.updateUser'        // (fields) — personal only
'profile.getDepartment'     // (deptId) → OrcaProfile
'profile.updateDepartment'  // (deptId, fields) — lead/admin
'profile.getCompany'        // () → OrcaProfile — admin only
'profile.updateCompany'     // (fields) — admin only
'profile.listDepartments'   // () → Department[]
'profile.createDepartment'  // (name) — admin

'projects.list'         // (userId?) → OrcaProject[]
'projects.get'          // (projectId) → OrcaProject
'projects.create'       // (input) — lead/admin
'projects.updateBinding'// (projectId, devServerId) — admin
'projects.addMember'    // (projectId, userId, role)
'projects.removeMember' // (projectId, userId)

'ai-providers.list'          // (devServerId) → AIProviderAccount[]
'ai-providers.add'           // (input)
'ai-providers.testConnection'// (accountId)
'ai-providers.rotateKey'     // (accountId)
'ai-providers.getUsage'      // (accountId, date)

'workflows.listTemplates'    // → WorkflowTemplate[]
'workflows.run'              // (templateId, inputs)
'workflows.pause'            // (executionId)
'workflows.resume'           // (executionId)
'workflows.streamStepOutput' // (stepId) → AsyncIterable

'tasks.list'            // (projectId) → OrcaTask[]
'tasks.get'             // (taskId)
'tasks.create'          // (input) with parentId
'tasks.addDependency'   // (fromId, toId)
'tasks.aiPlan'          // (taskId) → suggested subtasks
'tasks.runAgent'        // (taskId, worktreeId?) → spawn agent
'tasks.grant'           // (taskId, userId, level)
'tasks.revokeGrant'     // (taskId, userId)
```

### E.8 Security — Credential Flow (từ HLD security.md + ADR-008)

```
Browser (CredentialInput component)
    ↓
SubtleCrypto.encrypt(sessionKey, apiKey)  ← client-side, NOT plaintext
    ↓
POST /rpc { method: 'ai-providers.rotateKey', params: { accountId, encryptedKey } }
    ↓
Orca Backend Server → relay.call('aiProvider.writeCredential', encryptedBlob)
    ↓
SSH tunnel → Dev Server decrypt (ORCA_AI_CREDENTIAL_KEY env)
    ↓
Write ~/.orca/ai-providers/<accountId>.enc (AES-256-GCM)

[Orca Server KHÔNG BAO GIỜ thấy plaintext API key]
```

### E.9 DB Migrations → Feature Dependencies (từ HLD C4.3)

| Migration | Tables | Frontend Feature |
|-----------|--------|------------------|
| 0006 | `orca_company`, `orca_departments` | TDD-FE-11 (F33) |
| 0007 | `orca_projects`, `orca_project_members` | TDD-FE-12 (F34, F38) |
| 0008 | `orca_ai_provider_accounts`, `orca_provider_usage` | TDD-FE-13 (F35) |
| 0009 | `orca_workflow_templates`, `orca_workflow_executions` | TDD-FE-14 (F36) |
| 0010 | `orca_tasks`, `orca_task_edges`, `orca_task_grants` | TDD-FE-15 (F37) |

### E.10 Web Server Boot Flow v5.0 (từ HLD C4 + web-server-architecture.md)

```
main-web-bootstrap.tsx
    ├── checkAuthSession() → GET /auth/me → setAuth(user) | renderLoginPage()
    ├── WebSocketRpcClient.connect() → ws://orca:6768/ (cookie auth)
    ├── loadProfileConfig()  → rpc.call('profile.getResolved')
    ├── loadProjects()       → rpc.call('project.list')
    └── mount <App />
              └── <WorkspaceProvider>
                        ├── <ProjectSwitcher> → WorkspaceContext.switchProject()
                        └── <WorkspaceLayout>
                                  ├── ExplorerPanel
                                  ├── GitPanel
                                  ├── AgentPanel
                                  ├── WorkflowBuilder
                                  ├── TaskGraph
                                  └── WorkspaceTerminal
```
