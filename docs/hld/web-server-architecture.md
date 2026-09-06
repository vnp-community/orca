# Orca Web Frontend — Kiến trúc Client-Side (Browser)

**Nguồn:** TDD v5 (`specs/frontend/tdd/v5/`), HLD v1 (C2, C3, C4)  
**Cập nhật:** 2026-08-14 — correction pass against `audit/frontend/03-hld-doc-drift.md` (§5.1 wire protocol, §9.1/§9.6/§9.8 paths, §10.2/§10.7 GitPanel, §11 Admin SPA routes, §12 routing)  
**Scope:** Chỉ bao gồm những gì chạy trong **browser** (React + Zustand + Vite) — mọi xử lý nghiệp vụ thuộc [backend-server-architecture.md](./backend-server-architecture.md)

---

## 1. Phân biệt rõ ràng

| Thành phần | Chạy ở đâu | Tài liệu |
|-----------|-----------|----------|
| **Orca Web Frontend** ← **File này** | Browser (React 18 + Zustand + Vite) | — |
| **Orca Backend Server** | Node.js server | [backend-server-architecture.md](./backend-server-architecture.md) |
| **Dev Server Agent** | Remote machine | [dev-server-architecture.md](./dev-server-architecture.md) |

> **Nguyên tắc cốt lõi:** Frontend KHÔNG tự xử lý nghiệp vụ. Mọi mutation đều là RPC call qua WebSocket đến Backend, sau đó Backend relay tiếp đến Dev Server nếu cần.

> **2026-09-06:** Frontend gọi cùng 1 giao thức WS-RPC (`invoke`/`on`, §5.1) bất kể phía sau là TS
> `backend/` hay backend Go mới (`backend-go/`, 17 microservices) — frontend **không biết và không cần
> biết** domain nào đang được xử lý bởi bên nào. Với domain `workflow.*`, Web-mode hiện đã đi qua
> `api-gateway`'s wscompat tới `workflow-service` (Go) trong khi Electron/local-runtime vẫn đi thẳng vào
> TS `WorkflowOrchestrator` — 2 engine không tương thích, xem
> [backend-go-architecture.md §5](./backend-go-architecture.md#5-hai-workflow-engine-song-song-không-tương-thích-nhau)
> và [ADR-024](../adrs/v2/ADR-024-dual-workflow-engines-migration.md). Domain khác có thể đang/sẽ được
> cắt tương tự — chưa audit lại toàn bộ RPC namespace trong tài liệu này để xác nhận domain nào đã
> chuyển sang backend-go.

---

## 2. Tech Stack (Browser-side)

| Layer | Công nghệ | Ghi chú |
|-------|-----------|---------|
| UI Framework | **React 18** (StrictMode) | — |
| State Management | **Zustand** (không Redux) | Single store + 40+ slices |
| Build tool | **Vite** (via electron-vite) | Code splitting + lazy loading |
| Terminal emulator | **xterm.js** (WebGL renderer) | Custom PTY transport layer |
| Code editor | **Monaco Editor** | Read-only diff + file viewer |
| UI Components | **shadcn/ui** (Radix primitives) | — |
| Styling | **Tailwind CSS** + CSS Variables | Dark/light/system theme |
| i18n | **react-i18next** | `translate()` + `useTranslation()` |
| Icons | **lucide-react** | — |
| Toasts | **Sonner** | — |
| HTTP/WS client | Custom (`WebSocket` + `fetch`) | `WebSocketRpcClient` + `WebRuntimeClient` |
| E2E encryption | **TweetNaCl** (Curve25519 + NaCl box) | Desktop-to-Web pairing + Mobile |
| Type system | **TypeScript strict** | — |
| Test | **Vitest** (unit) + **Playwright** (e2e) | `happy-dom` per React test file |
| Graph Rendering | **@xyflow/react** (React Flow) | Task DAG visualization (v5.0) |
| Markdown | **react-markdown** + **remark-gfm** | Task descriptions, AI output |

---

## 3. Hai Render Targets — Cùng codebase

Orca dùng **cùng `App.tsx`** cho cả Desktop và Web, tách biệt qua `window.api` abstraction:

```
src/renderer/
├── index.html          ← Electron Desktop renderer entry
├── web-index.html      ← Web browser entry
├── admin-index.html    ← Admin SPA entry [v4.0]
└── src/
    ├── main.tsx        ← Desktop entry → mount <App/>
    └── web/
        ├── main.tsx    ← Web entry → gọi bootstrapWebApp()
        └── main-web-bootstrap.tsx  ← Testable bootstrap
```

### 3.1 Desktop Mode (Electron)

```typescript
// src/renderer/src/main.tsx
createRoot(document.getElementById('root')).render(
  <StrictMode>
    <I18nProvider>
      <RecoverableRenderErrorBoundary>
        <App />   // ← same App.tsx
      </RecoverableRenderErrorBoundary>
    </I18nProvider>
  </StrictMode>
)
// window.api comes from Electron preload (contextBridge)
```

### 3.2 Web Mode (Browser) — bootstrapWebApp()

```typescript
// src/renderer/src/web/main.tsx — tối giản
import { bootstrapWebApp } from './main-web-bootstrap'
bootstrapWebApp().catch(console.error)
```

```typescript
// src/renderer/src/web/main-web-bootstrap.tsx
export async function bootstrapWebApp(): Promise<void> {
  // 1. Kiểm tra session auth: GET /auth/me
  const user = await checkAuthSession()

  // 2a. Không có session → render LoginPage
  if (!user) { renderLoginPage(); return }

  // 2b. Có session → tạo WebSocketRpcClient
  const client = new WebSocketRpcClient(wsUrl)
  await client.connect()  // reconnect backoff: 500ms→1s→2s→5s→10s→30s (no attempt limit)

  // 3. Load initial data
  await Promise.all([
    loadProfileConfig(),   // rpc.call('profile.getResolved')
    loadProjects(),        // rpc.call('project.list')
  ])

  // 4. Render full app
  ReactDOM.createRoot(root).render(
    <StrictMode>
      <I18nProvider>
        <ConnectionStatusProvider client={client}>
          <ConnectionStatusBanner />
          <App />
        </ConnectionStatusProvider>
      </I18nProvider>
    </StrictMode>
  )
}
```

**Decision tree:**
```
Browser truy cập https://orca-server/
    │
    ├── GET /auth/me → 200 → user session tồn tại
    │         ├── isPaired? → WebConnect (legacy pairing)
    │         └── → App.tsx (full Orca UI)
    │
    └── GET /auth/me → 401 → chưa login
              └── → LoginPage
```

### 3.3 `window.api` Abstraction

Cùng interface cho cả Desktop và Web:

```typescript
// Desktop: src/preload/index.ts (Electron contextBridge)
// Web:     src/renderer/src/web/web-preload-api.ts (~135KB)

interface OrcaApi {
  filesystem: { readFile, writeFile, listDir, search, watch, ... }
  pty:        { create, write, resize, kill, subscribe, ... }
  ssh:        { listTargets, connect, disconnect, ... }
  worktrees:  { detect, create, delete, ... }
  repos:      { list, create, update, delete, ... }
  settings:   { getGlobal, updateGlobal, ... }
  github:     { listPRs, createPR, ... }
  runtimeEnvironments: { call, ... }
}
```

---

## 4. App Initialization Flow Chi Tiết

### Desktop:

```
Electron mở Orca
    │
    ├── recordRendererCrashBreadcrumb('renderer_bootstrap_started')
    ├── installRendererCrashDiagnostics()   ← Sentry setup
    ├── applyDocumentTheme('system', ...)   ← CSS vars (light/dark)
    └── mount App.tsx
              ├── useIpcEvents()              ← subscribe Electron IPC events
              ├── scheduleRuntimeGraphSync()  ← sync store từ backend
              └── Layout: Sidebar + Content + RightSidebar
```

### Web (v4.0 + v5.0):

```
Browser → https://orca-server/
    │
    ├── web/main.tsx → bootstrapWebApp()
    │
    ├── checkAuthSession() → GET /auth/me
    │       ├── 401 → renderLoginPage() → LoginPage.tsx
    │       └── 200 → AuthSlice.setAuth(user)
    │
    ├── WebSocketRpcClient.connect() → ws://orca:6768/
    │       ├── WS upgrade + orca_session cookie
    │       └── ConnectionStatusProvider wraps app
    │
    ├── loadProfileConfig() → rpc.call('profile.getResolved')
    ├── loadProjects()      → rpc.call('project.list')
    │
    └── mount <App /> → <WorkspaceProvider>
                              ├── <ProjectSwitcher>
                              └── <WorkspaceLayout>
```

---

## 5. Transport Layer (Client-side RPC)

### 5.1 WebSocketRpcClient

**File:** `frontend/src/platform/adapters/web/rpc-client.ts`  
**Interface:** `IRpcClient` (`frontend/src/platform/rpc-client-interface.ts`) — decouples `ConnectionStatusProvider` and `web-preload-api` from the concrete transport, and is the *same* surface Electron's `ipcRenderer` preload exposes on desktop, so `web-preload-api.ts` can implement `window.api` identically on both targets.

```typescript
// IRpcClient — real shape, not JSON-RPC 2.0 call()/callStream()
export type IRpcClient = {
  connect(): Promise<void>
  disconnect(): void
  isConnected(): boolean
  invoke(channel: string, ...args: unknown[]): Promise<unknown>
  send(channel: string, data?: unknown): void
  on(channel: string, handler: (...args: unknown[]) => void): () => void
  off(channel: string, handler: (...args: unknown[]) => void): void
  once(channel: string, handler: (...args: unknown[]) => void): void
}

class WebSocketRpcClient implements IRpcClient {
  private ws: WebSocket | null
  private readonly pending: Map<string, PendingInvocation>   // request id → resolve/reject/timeout
  private readonly listeners: Map<string, Set<Handler>>       // push-channel → handlers

  connect(): Promise<void>       // sets intentionallyClosed=false, opens WS (10s connect timeout)
  disconnect(): void             // sets intentionallyClosed=true — stops the reconnect loop
  isConnected(): boolean
  invoke(channel, ...args): Promise<unknown>   // request/response, 30s timeout
  send(channel, data?): void                   // fire-and-forget
  on/off/once(channel, handler)                // subscribe to server-push messages
}
```

**Wire protocol — plain JSON text, not binary framing:**

```
WebSocket message = JSON.stringify(envelope), no ArrayBuffer/binary frame, no SEQ/ACK.

Client → Server:
  invoke: { id, type: 'invoke', channel, args }
  send:   { type: 'send', channel, data }

Server → Client:
  result: { id, type: 'result', result }
  error:  { id, type: 'error', message }
  push:   { type: 'push', channel, args }
```

This is an ipcRenderer-style envelope (`invoke`/`on`/`once`), **not** JSON-RPC 2.0 — there is no `method`/`params`/`jsonrpc` field. `rpc-client.ts` itself has **no keepalive/ping-pong logic at all**.

**Reconnect:** unexpected `ws.onclose` (network blip, proxy timeout, server restart) schedules a reconnect via `RECONNECT_DELAYS_MS = [500, 1000, 2000, 5000, 10000, 30000]` (ms) — backoff caps at 30s, **no limit on attempt count** (not "maxRetries=3, delay=2s"). `ConnectionStatusProvider` polls `isConnected()` every 2s so the banner clears automatically once a reconnect succeeds. Explicit `disconnect()` (logout/unmount) sets `intentionallyClosed` and skips the reconnect loop entirely.

> **Do not confuse with the SSH relay's binary protocol.** A 13-byte `TYPE[1B]|SEQ[4B BE]|ACK[4B BE]|LEN[4B BE]|PAYLOAD` frame format *does* exist in the codebase, but it belongs to a completely different subsystem: `frontend/src/main/ssh/relay-protocol.ts` (`HEADER_LENGTH = 13`, `MessageType.Regular = 1`, `MessageType.KeepAlive = 9`), used by the SSH remote-relay multiplexer (`ssh-channel-multiplexer.ts`, modeled on VS Code's `PersistentProtocol`) for the F23/F24 remote dev-server connection — it never touches the browser's `WebSocketRpcClient`. Its real keepalive interval is `KEEPALIVE_SEND_MS = 5_000` (5s), not 30s.

### 5.2 WebRuntimeClient (E2EE pairing — Desktop Pair Code sharing only)

**File:** `src/renderer/src/web/web-runtime-client.ts` (~27KB)

**Cập nhật 2026-08-09 ([CR-FE2E series](../../../docs/crs/v2/frontend-e2ee/)):** `main.tsx` probe `GET /auth/config` lúc khởi động để chọn 1 trong 2 nhánh hoàn toàn tách biệt:
- **`/auth/config` → 200** (đang chạy sau Orca Web Server multi-user, `backend/`) → `bootstrapWebApp()` — SSO/local login, session cookie luôn có. `WebRuntimeClient`/E2EE pairing **không còn reachable** từ nhánh này (CR-FE2E-002 đã bỏ `PairCodeFallback` khỏi `LoginPage`; CR-FE2E-003 code-split để bundle nhánh này không tải `WebConnect.tsx` nữa — dù `web-e2ee.ts`/TweetNaCl vẫn còn trong entry chunk qua `web-preload-api.ts`, xem ghi chú giới hạn ở `specs/frontend/crs/frontend-e2ee/tasks/TASK-FE2E-008-tests-and-bundle-measurement.md`).
- **`/auth/config` → 404** ("Desktop Pair Code sharing mode" — browser trỏ thẳng vào 1 Desktop app/bare relay, không qua `backend`) → `pair-code-app-entry.tsx` (dynamic import) — **đây là nơi duy nhất** `WebRuntimeClient` còn được dùng từ browser. Mobile Companion (F03) pair vào `backend` qua cùng cơ chế E2EE ở tầng backend, không đi qua code này.

```typescript
class WebRuntimeClient {
  private sharedKey: Uint8Array | null   // TweetNaCl session key

  async connect(offer: PairingOffer): Promise<void>
  // 1. WebSocket → send clientPublicKey
  // 2. Server responds with confirmation
  // 3. Derive sharedKey via nacl.box.before()

  async call<T>(method: string, params?: unknown): Promise<T>
  // encrypt frame với nacl.box.seal() nếu có sharedKey
}
```

**Pairing formats:**
```
orca://pair?code=<base64url>     ← QR code từ Desktop
https://domain/?pair=<base64url> ← Web share link
wss://server:6768?token=<token>  ← Direct (no E2EE)
```

### 5.3 ConnectionStatusProvider (restructure_v1)

**File:** `src/renderer/src/web/ConnectionStatusProvider.tsx`

```typescript
// React Context cung cấp 3 hooks:
const { status } = useConnectionStatus()
// status: 'connecting' | 'connected' | 'disconnected' | 'error'

const { isConnected } = useIsConnected()
const { reconnect } = useReconnect()
```

**ConnectionStatusBanner:** Overlay cố định hiển thị khi `status !== 'connected'`.

### 5.4 HTTP Client (auth + admin REST)

**File:** `src/renderer/src/auth/auth-api-client.ts`

```typescript
fetchCurrentUser()   → GET /auth/me
loginLocal(email, password) → POST /auth/local → Set-Cookie
logoutUser()         → POST /auth/logout
fetchAuthConfig()    → GET /auth/config (SSO providers)

// Admin REST:
// src/renderer/src/components/admin/admin-api-client.ts
fetchUsers()         → GET /admin/api/users
createUser(input)    → POST /admin/api/users
killSession(id)      → DELETE /admin/api/sessions/:id
```

---

## 6. State Management — Zustand Store

**File:** `src/renderer/src/store/index.ts`  
**Pattern:** Single store, composed từ 40+ slices

```typescript
export const useAppStore = create<AppState>()((...a) => ({
  ...createRepoSlice(...a),
  ...createWorktreeSlice(...a),
  ...createTerminalSlice(...a),
  ...createAuthSlice(...a),      // [v4.0]
  ...createProfileSlice(...a),   // [v5.0]
  ...createProjectSlice(...a),   // [v5.0]
  ...createTaskSlice(...a),      // [v5.0]
  ...createWorkflowSlice(...a),  // [v5.0]
  // ... 30+ more
}))
```

### 6.1 Danh sách tất cả Slices

| Slice | File | Mô tả |
|-------|------|-------|
| `repos` | `slices/repos.ts` (~115K) | Repositories, project groups, SSH repos |
| `worktrees` | `slices/worktrees.ts` (~192K) | Worktree lifecycle, metadata |
| `terminals` | `slices/terminals.ts` (~146K) | Terminal sessions, PTY states |
| `tabs` | `slices/tabs.ts` (~78K) | Tab groups, tab order, active tabs |
| `ui` | `slices/ui.ts` (~103K) | Sidebar, modals, layout, zoom, view state |
| `editor` | `slices/editor.ts` (~176K) | Open files, editor tabs, diff views |
| `github` | `slices/github.ts` (~156K) | GitHub PRs, issues, checks cache |
| `linear` | `slices/linear.ts` (~75K) | Linear issues, teams, projects |
| `jira` | `slices/jira.ts` (~22K) | Jira issues |
| `browser` | `slices/browser.ts` (~75K) | Browser pane state, webview |
| `agent-status` | `slices/agent-status.ts` (~105K) | AI agent status per terminal |
| `ssh` | `slices/ssh.ts` | SSH connection states + provisioning [v4.0] |
| `settings` | `slices/settings.ts` | User preferences |
| `keybindings` | `slices/keybindings.ts` | Keyboard shortcuts |
| `preflight` | `slices/preflight.ts` | Startup checks |
| `diff-comments` | `slices/diffComments.ts` | PR diff inline comments |
| `detected-agents` | `slices/detected-agents.ts` | Detected AI agents in terminals |
| `runtime-status` | `slices/runtime-status.ts` | Runtime connection health |
| `claude-usage` | `slices/claude-usage.ts` | Claude API usage meter |
| `codex-usage` | `slices/codex-usage.ts` | Codex usage meter |
| `rate-limits` | `slices/rate-limits.ts` | AI rate limiting |
| `pull-request-generation` | `slices/pull-request-generation.ts` | AI PR description gen |
| `commit-message-generation` | `slices/commit-message-generation.ts` | AI commit msg gen |
| `devServers` | `slices/dev-servers.ts` [v3.0] | Dev server registry + status |
| `remoteAgents` | `slices/remote-agents.ts` [v3.0] | Detected remote agents |
| `fleetHealth` | (trong ssh.ts) [v3.0] | Fleet health metrics + alerts |
| `auth` | `slices/auth.ts` [v4.0] | AuthUser, AuthState, session |
| `profile` | `slices/profile.ts` [v5.0] | Resolved/company/dept/user profile |
| `project` | `slices/project.ts` [v5.0] | Projects list + active project |
| `ai-provider` | `slices/ai-provider.ts` [v5.0] | Provider accounts + usage |
| `workflow` | `slices/workflow.ts` [v5.0] | Templates + executions |
| `task` | `slices/task.ts` [v5.0] | Task graph + grants + active task |

### 6.2 Slice Pattern

```typescript
// Mỗi slice theo pattern:
type WorktreeSlice = {
  worktrees: Record<string, Worktree>
  // actions:
  addWorktree: (wt: Worktree) => void
  removeWorktree: (id: string) => void
  updateWorktree: (id: string, updates: Partial<Worktree>) => void
}

export const createWorktreeSlice: StateCreator<AppState, [], [], WorktreeSlice> = (set) => ({
  worktrees: {},
  addWorktree: (wt) => set(state => { state.worktrees[wt.id] = wt }),
  removeWorktree: (id) => set(state => { delete state.worktrees[id] }),
})
```

### 6.3 Sync Runtime Graph (Backend → Store)

```typescript
// src/renderer/src/runtime/sync-runtime-graph.ts (~56KB)
// Core sync: backend canonical graph → Zustand store

scheduleRuntimeGraphSync()
// → debounce 16ms → callRuntimeRpc('status.get') → applyRuntimeGraph()
// Merge vào store: repos, worktrees, terminals, tabs
// Prune removed entities: removedRepoIds, removedWorktreeIds, ...
// Trigger: sau mỗi mutating RPC response
```

---

## 7. Auth & Session UI

### 7.1 Auth Flow (Web Mode)

**Files:** `src/renderer/src/auth/`, `src/renderer/src/web/login/`

```
Auth check → GET /auth/me
    ├── 200 → AuthSlice.setAuth({ userId, role, name, email })
    │         → render App.tsx
    └── 401 → render LoginPage

LoginPage:
    ├── LoginForm (email + password)
    │       → loginLocal(email, pwd) → POST /auth/local
    │       → Set-Cookie: orca_session
    │       → AuthSlice.setAuth(user)
    │       → window.location.href = '/'   (hard reload, not router navigation — §12)
    │
    └── SsoButton (GitHub/Google/Keycloak)
            → redirect to SSO provider OAuth flow
```

**CR-FE2E-002 removed `PairCodeFallback` from `LoginPage`** (`LoginPage.tsx:3`: *"PairCodeFallback removed"*) — the multi-user login branch always has session-cookie login available, so the fallback UI no longer applies there. `PairCodeFallback.tsx` still exists as a file, but only the separate legacy pair-code entry point (`web/pair-code-app-entry.tsx`, §12 item 3) can reach it now.

### 7.2 Auth State (AuthSlice)

```typescript
type AuthSlice = {
  auth: AuthState                        // { user, status, error }
  setAuth: (user: AuthUser) => void
  clearAuth: () => void
  checkSession: () => Promise<void>      // GET /auth/me
}

type AuthUser = {
  id: string
  email: string
  name: string
  role: 'admin' | 'developer' | 'lead'
}
```

### 7.3 Auth Hooks

```typescript
const user = useAuthUser()        // AuthUser | null
const isAuth = useIsAuthenticated()  // boolean
const status = useAuthStatus()    // 'loading' | 'authenticated' | 'unauthenticated'
const logout = useLogout()        // () => Promise<void>
```

### 7.4 Web-only Auth UI Components

| Component | File | Chức năng |
|-----------|------|-----------|
| `LoginPage` | `web/login/LoginPage.tsx` | Login container + SSO routing |
| `LoginForm` | `web/login/LoginForm.tsx` | Email + password form |
| `SsoButton` | `web/login/SsoButton.tsx` | GitHub/Google/Keycloak link |
| `PairCodeFallback` | `web/login/PairCodeFallback.tsx` | Legacy pairing UI — file still exists, but **no longer referenced by `LoginPage`** (CR-FE2E-002); only reachable via the separate `pair-code-app-entry.tsx` |
| `UserAvatarMenu` | `components/auth/UserAvatarMenu.tsx` | Avatar dropdown (web-only) |
| `UserRoleBadge` | `components/auth/UserRoleBadge.tsx` | Role indicator badge |

---

## 8. App Shell & Component Tree

### 8.1 App.tsx (~127KB) — Layout Root

```
App.tsx
├── <TooltipProvider>           (Radix tooltip context)
├── <ConfirmationDialogProvider>
│
├── Layout Frame
│   ├── Titlebar
│   │   ├── Traffic lights (macOS)
│   │   ├── <ActivityTitlebarControls>
│   │   ├── <OrcaProfileSwitcher>     ← + UserAvatarMenu khi web + auth
│   │   └── Nav buttons
│   │
│   ├── <Sidebar>  (left)
│   │   └── WorkspaceBoard, repo list, SSH status
│   │   └── <SshUserIndicator> [v4.0] ← SSH username per server
│   │
│   ├── Main Content Area
│   │   ├── <TaskPage>           (~542KB — lớn nhất) [F06]
│   │   ├── <PullRequestPage>    (~259KB)             [F06]
│   │   ├── <GitHubItemDialog>   (~285KB)             [F06]
│   │   ├── <LinearItemDrawer>   (~59KB)              [F06]
│   │   ├── <WorkspaceArea>
│   │   │   ├── <TabBar>
│   │   │   └── Tab content:
│   │   │       ├── <TerminalPane>   (terminal tab)  [F02]
│   │   │       ├── Editor tab        (file viewer)   [F12]
│   │   │       └── <BrowserPane>    (headless browser) [F05]
│   │   └── <FloatingTerminalToggleButton>
│   │
│   └── <RightSidebar>
│       ├── Git status / diff   [F01]
│       ├── Agent status        [F04]
│       └── Source control
│
└── Global Overlays
    ├── <QuickOpen>               (Cmd+P)     [F10]
    ├── <WorktreeJumpPalette>     (Cmd+K)     [F01]
    ├── <NewWorkspaceComposerModal>            [F01]
    ├── <UpdateCard>                           [F21]
    ├── <CrashReportDialog>
    ├── <ConnectionStatusBanner>  [web-only]  [F22]
    └── <Toaster> (Sonner)                    [F11]
```

**Lazy loading:** Tất cả heavy components dùng `React.lazy()`:
```typescript
const TaskPage = lazy(() => import('./components/TaskPage'))
const PullRequestPage = lazy(() => import('./components/PullRequestPage'))
// Wrap trong <Suspense fallback={null}>
```

---

## 9. Các Thành phần UI — Chi tiết

### 9.1 Worktree Sidebar (F01)

**File:** `src/renderer/src/components/sidebar/` (not `worktree-sidebar/`)

| Element | Mô tả |
|---------|-------|
| Worktree cards | Branch name + agent status badge (idle/running/waiting/done) |
| Status badge | Màu theo trạng thái: OSC 133 detection từ backend |
| Drag-drop reorder | Sắp xếp thứ tự hiển thị |
| Context menu | New / Delete / Fan-out / Open in terminal |
| Fan-out UI | Tạo N worktrees song song (cùng base ref) |

**RPC:** `worktree.list()`, `worktree.create(opts)`, `worktree.remove(id)`  
**Events:** `agent:status:changed` → update badge

---

### 9.2 TerminalPane (F02)

**File:** `src/renderer/src/components/terminal-pane/TerminalPane.tsx` (~127KB)

```typescript
type TerminalPaneProps = {
  tabId: string
  worktreeId: string
  environmentId: string | null  // null = local, string = remote env
  isActive: boolean
  initialPaneLayout?: TerminalPaneLayoutNode
}
```

| Element | Mô tả |
|---------|-------|
| xterm.js instance | WebGL-accelerated terminal rendering |
| PaneManager | Split layout (horizontal/vertical) |
| PtyConnection[] | Một PTY session per leaf pane |
| ANSI/OSC | Full ANSI + OSC 133 shell integration |
| Scrollback | Restore từ SQLite snapshot (session resume) |
| TerminalPaneOverlayLayer | SSH reconnect overlay, agent status |

**Hooks:** `useTerminalPaneLifecycle()`, `useTerminalPaneGlobalEffects()`  
**RPC:** `pty.create({cwd, shell})`, `pty.write({ptyId, data})`, `pty.resize({...})`  
**Events:** server-push `pty:data { ptyId, data }` → xterm.js write

---

### 9.3 TaskPage (~542KB) (F06, F37)

**File:** `src/renderer/src/components/TaskPage.tsx`

Multi-provider issue tracker:
- **GitHub Issues + PRs** — filter, search, link worktree
- **Linear Issues** — status, priority, assignee
- **Jira Issues** — project browse
- **Custom tasks** (Task Graph — v5.0)

---

### 9.4 PullRequestPage (~259KB) (F06, F08)

**File:** `src/renderer/src/components/PullRequestPage.tsx`

| Sub-component | Chức năng |
|--------------|-----------|
| Diff viewer | File-by-file diff |
| DiffComments | Inline review comments |
| PR checks status | GitHub Actions status |
| Review submission | Approve/Request Changes |
| Merge controls | Merge strategies |
| AI Annotation | Request AI explanation cho diff (F08) |

---

### 9.5 BrowserPane (F05, F15)

**File:** `src/renderer/src/components/browser-pane/`

| Element | Mô tả |
|---------|-------|
| Electron webview / iframe | Render local dev server URL |
| Viewport controls | Mobile/tablet/desktop breakpoints |
| Design overlay | Click-to-annotate elements (F05) |
| Screenshot | Capture viewport cho AI context (F15) |

**Events:** `port:detected` → auto-navigate browser pane

---

### 9.6 QuickOpen (F10)

**File:** `src/renderer/src/components/QuickOpen.tsx` (a flat file, not a `QuickOpen/` folder)

Trigger: **Cmd+P**

```typescript
// Fuzzy search qua:
// - Files trong current repo (local)
// - RPC: fs.glob(pattern) cho remote repos (relay)
// - Worktrees list
// - GitHub issues / Linear tasks

// Kết quả: open file tab | switch worktree | jump to issue
```

---

### 9.7 NewWorkspaceComposerCard (F01, F07)

**File:** `src/renderer/src/components/NewWorkspaceComposerCard.tsx` (~60KB)

Wizard tạo worktree mới:
1. Select repo
2. Select branch (hoặc tạo mới)
3. Configure execution host (local / SSH remote)
4. Run setup script
5. Start agent session (optional)

---

### 9.8 Fleet Management UI (F27, F28, F31) [v3.0]

**There is no `components/fleet/` folder.** The fleet UI is split across two real locations — a settings-panel copy and an admin-console copy:

**Files:** `src/renderer/src/components/settings/ssh/`

| Component | Chức năng |
|-----------|-----------|
| `FleetHealthDashboard.tsx` | Status grid tất cả servers: CPU/RAM/disk/latency |
| `FleetImportProgress.tsx`, `FleetSummaryCard.tsx`, `FleetFilterBar.tsx`, `FleetHealthTable.tsx`, `FleetAlertStrip.tsx` | Fleet health list, filtering, alert banner |
| `FleetProvisionWizard.tsx` (not `BulkProvisioningWizard`) | Provision nhiều servers cùng lúc |
| `ServerBootstrapPanel.tsx` / `BootstrapStepList.tsx` (not `BootstrapStatusPanel`) | Bootstrap step tracker per server |
| `ProvisionServerSelector.tsx`, `ProvisionConfirmStep.tsx`, `ProvisionProgressPanel.tsx`, `ProvisionDoneSummary.tsx` | Provisioning wizard steps |
| `SshTargetGroupRow.tsx`, `SshTargetGroupedList.tsx`, `SshTargetGroup.tsx` | Server list grouping |

**Files:** `src/renderer/src/components/admin/fleet/`

| Component | Chức năng |
|-----------|-----------|
| `fleet-dashboard.tsx` (exports `FleetDashboard`) | Admin console's `/fleet` route page |
| `fleet-import-dialog.tsx` | YAML import dialog with progress |

`UserProfileBadge` exists but lives in `components/activity/UserProfileBadge.tsx`, not the fleet UI.

**Hooks (real):**
```typescript
useFleetHealthPolling()   // hooks/useFleetHealthPolling.ts (30s polling)
useSshProvisioning()      // hooks/useSshProvisioning.ts
```

`useFleetImport()`, `useBootstrapAutomation()`, `useServerGroups()`, `useBulkProvisioning()` do **not exist anywhere** in `hooks/` — remove from any future doc unless implemented.

---

### 9.9 Onboarding & Dev Server UI (F28) [v3.0]

**Files:** `src/renderer/src/components/onboarding/`, `components/dev-server/`

| Component | Chức năng |
|-----------|-----------|
| `DevServerStep` | Onboarding wizard step cho dev server |
| `DevServerCard` | Card hiển thị server info + status |
| `DevServerList` | List tất cả configured servers |
| `DevServerDialog` | Add/edit server dialog |
| `DevServerStatusBadge` | Online/offline/degraded indicator |
| `RemoteDirectoryBrowser` | Browse remote filesystem để chọn repo path |

**Hooks:**
```typescript
useRemoteAgentDetection(serverId)   // module cache, 60s TTL
useRemoteWindowsTerminalCapabilities(serverId)  // capsCache, 60s TTL
```

---

### 9.10 SSH Provisioning UI (F23, F24) [v4.0]

**Files:** `src/renderer/src/components/ssh/`

| Component | Chức năng |
|-----------|-----------|
| `SshProvisioningProgress` | Progress bar khi provision SSH user |
| `SshUserIndicator` | Username + status badge injected vào SshTargetRow |

```typescript
// SSH User isolation:
toLinuxUsername(email: string): string
// → lowercase, regex clean, truncate 20
// Mirror chính xác backend implementation
```

**Store extension (ssh.ts):**
```typescript
// slices/ssh.ts [v4.0] extensions:
sshUserAccounts: Map<string, SshUserAccount>
provisioningStatus: Map<string, ProvisioningStatus>
```

---

### 9.11 Web Push Notifications (F11, F03) [v3.0]

**Files:**
- `src/renderer/service-worker.js` — Push event handler
- `src/renderer/src/hooks/useBrowserNotificationPermission.ts`
- `src/renderer/src/hooks/useWebPushSubscription.ts`

```typescript
// useBrowserNotificationPermission():
// Request permission → subscribe VAPID → send subscription to backend
// Nhận: agent complete, task update, fleet alert

// service-worker.js handles:
// push event → showNotification({ title, body, data })
// notificationclick → navigate to relevant panel
```

---

## 10. v5.0 — Workspace Components (F33–F39)

### 10.1 WorkspaceContext — Central State

**File:** `src/renderer/src/context/WorkspaceContext.tsx`

```typescript
interface WorkspaceContextValue {
  project: OrcaProject | null
  devServer: SshHost | null
  isConnected: boolean
  isOffline: boolean
  currentWorktree: Worktree | null
  availableWorktrees: Worktree[]
  gitStatus: GitStatus | null
  resolvedProfile: ResolvedProfile | null
  activeAgentSessionId: string | null
  relay: DevServerRelayBridge | null   // proxied via RPC, managed backend

  setCurrentWorktree: (wt: Worktree) => void
  setActiveAgentSession: (id: string | null) => void
  refreshGitStatus: () => Promise<void>
  switchProject: (projectId: string) => Promise<void>

  // Cross-panel event bus:
  emit: (event: WorkspaceEvent) => void
  on: (event: string, handler: Function) => () => void  // unsubscribe
}
```

**Cross-panel Events:**

| Event | Panels nhận | Hành động |
|-------|------------|-----------|
| `agent.complete` | GitPanel, ExplorerPanel, TasksPanel, Banner | Refresh git status + decorations + advance task |
| `git.commit` | TasksPanel, GitPanel, ExplorerPanel | Scan msg cho #task-id, refresh ahead/behind |
| `worktree.switched` | GitPanel, ExplorerPanel, TerminalPanel | Reload status/tree/cwd |
| `workflow.step.complete` | GitPanel, TasksPanel | Auto-fetch, refresh tasks |

**switchProject() flow:**
```typescript
switchProject('proj-abc'):
    → RPC: projects.get('proj-abc')    → { devServerId, repoPath }
    → Backend checks fleet health, establishes relay
    → RPC: git.status({ cwd })
    → RPC: git.worktree.list({ repoPath })
    → RPC: fs.readDir({ path, depth: 2 })
    → WorkspaceContext state ready → UI renders
    → Start git status poll (every 5s)
```

---

### 10.2 Workspace Components (F38, F39)

**Files:** `src/renderer/src/components/workspace/`

| Component | Feature | Chức năng |
|-----------|---------|-----------|
| `WorkspaceLayout` | F38 | Sidebar tabs + main area + bottom terminal |
| `ProjectSelector` | F34 | Project dropdown + server status dot |
| `ServerStatusBar` | F27 | Online/degraded/offline banner + retry |
| `ExplorerPanel` | F38 | Remote file tree (lazy expand), git badges |
| `FileTreeNode` | F38 | Single node: icon + name + git badge |
| `RemoteFileViewer` | F38 | Read-only Monaco tab (syntax highlight) |
| `FileSearchPanel` | F13 | Glob + grep results stream |
| `GitPanel` (`workspace/git/GitPanel.tsx`) | F39 | Full Git UI: Changes/History/Branches/Pull Requests tabs |
| `DiffViewer` | F39 | Unified diff với syntax highlight |
| `StagingArea` | F39 | Stage/unstage file list (Changes tab, doc trước đây không nhắc) |
| `CommitForm` | F39 | Message input + AI generate + commit + push |
| `BranchManager` | F39 | list/create/switch/delete/merge |
| `WorktreeSwitcher` | F01 | Dropdown + new worktree |
| `GitHistory` (not `GitLog`) | F39 | Last 50 commits + ASCII branch graph |
| `PullRequestForm` | F39 | Title + AI body + reviewers → gh CLI |
| `PullRequestList` | F39 | Pull Requests tab — **not wired to a backend RPC yet**, renders "not available" (doc trước đây không nhắc) |
| ~~`ConflictPanel`~~ | F39 | 🚧 documented, not implemented — no `ConflictPanel` component exists in code (0 grep hits repo-wide); likely an F39 AI-assisted conflict resolution feature that was never built, not just a doc typo |
| `AgentPanel` | F04 | Provider display + prompt + live output stream |
| `WorkspaceTerminal` | F02 | Bottom panel PTY sessions (multi-tab) |

---

### 10.3 Profile UI (F33)

**Files:** `src/renderer/src/components/profile/`

| Component | Chức năng |
|-----------|-----------|
| `ProfileEditor` | Edit user profile: model prefs, env vars, shell |
| `ProfileSourceBadge` | Hiển thị nguồn mỗi setting: company/dept/user |

**Store (profile slice):**
```typescript
{ resolvedProfile, companyProfile, deptProfile, userProfile }
actions: setResolved(), updateUser(), invalidate()
```

---

### 10.4 AI Provider UI (F35)

**Files:** `src/renderer/src/components/ai-provider/`

| Component | Chức năng |
|-----------|-----------|
| `ProviderList` | Grid providers: name + model + health status |
| `ProviderForm` | Add/edit provider: type, endpoint, model list |
| `CredentialInput` | Key input → client-side encrypt → relay to Dev Server |
| `UsageChart` | Token usage per day chart |

**Security:** Credentials KHÔNG gửi plaintext qua backend — Browser `SubtleCrypto.encrypt()` → relay encrypted blob.

---

### 10.5 Task Graph UI (F37)

**Files:** `src/renderer/src/components/task/`

| Component | Chức năng |
|-----------|-----------|
| `TaskGraph` | DAG visualization dùng **@xyflow/react** |
| `TaskCard` | Node trong graph: title + status + assignee |
| `TaskDetail` | Slide-in panel: full task info + comments + grants |
| `TaskAIDecompose` | AI decompose UI: show suggested subtasks + approve |
| `TaskPromptEditor` | Prompt template editor với variable highlighting |

**Store (task slice):**
```typescript
{ tasks: Map<id, OrcaTask>, activeTaskId, expandedNodes }
actions: setTasks(), updateTask(), setActive()
```

---

### 10.6 Workflow Builder UI (F36)

**Files:** `src/renderer/src/components/workflow/`

| Component | Chức năng |
|-----------|-----------|
| `WorkflowBuilder` | Template editor: step list + config |
| `StepEditor` | Edit step: type, serverSpec, command, timeout |
| `DAGPreview` | Preview execution order với dependency arrows |
| `ExecutionMonitor` | Live execution: step status + output stream |

---

### 10.7 Remote Git UI (F39) — GitPanel Chi tiết

**File:** `src/renderer/src/components/workspace/git/GitPanel.tsx` (thư mục tên `git/`, không phải `GitPanel/`)

Sub-components thật trong `workspace/git/`: `DiffViewer.tsx`, `CommitForm.tsx`, `BranchManager.tsx`, `PullRequestForm.tsx`, `GitHistory.tsx` (không phải `GitLog`), `StagingArea.tsx`, `PullRequestList.tsx`. **Không có `ConflictPanel.tsx`** — xem cảnh báo 🚧 ở §10.2.

**RPC calls từ GitPanel:**

| Action | RPC Method | Mô tả |
|--------|-----------|-------|
| Get status | `git.status({ cwd })` | Modified/staged/untracked list |
| View diff | `git.diff({ cwd, file? })` | Raw diff string |
| Stage | `git.add({ cwd, files })` | Stage files |
| Unstage | `git.restore({ cwd, staged: true, files })` | Unstage |
| Commit | `git.commit({ cwd, message, author })` | Commit |
| Push (stream) | `git.push({ cwd, remote, branch })` | Progress events |
| Pull (stream) | `git.pull({ cwd, remote, branch })` | Progress events |
| List branches | `git.branch.list({ cwd })` | — |
| Create branch | `git.branch.create({ cwd, name, from? })` | — |
| Switch branch | `git.branch.checkout({ cwd, name })` | — |
| Merge | `git.merge({ cwd, branch })` | — |
| Stash | `git.stash({ cwd, message? })` | — |
| Git log | `git.log({ cwd, limit: 50 })` | ASCII graph |
| Create PR | `git.pr.create({ title, body, base })` | via gh CLI |
| AI commit msg | `ai.complete({ prompt: diff })` | LLM generation |

---

### 10.8 Remote File Explorer (F38)

**File:** `src/renderer/src/components/workspace/ExplorerPanel.tsx`

```
User expand 📁 src/
    → RPC: fs.readDir({ path: '/srv/vnp/src', depth: 1 })
    → Backend → relay → Dev Server fs.readdir()
    → Return: [{ name: 'auth', isDir: true }, ...]
    → Overlay gitStatusMap (pre-fetched từ WorkspaceContext)
    → Render với git badges: [M] [A] [?]

User click 📄 auth-manager.ts
    → RPC: fs.readFile({ path, encoding: 'utf-8' })
    → Detect language từ extension → TypeScript
    → Open Monaco tab (read-only, syntax highlight)

User search
    → RPC: fs.grep({ pattern, cwd, include: '*.ts' })
    → Stream kết quả → FileSearchPanel
```

---

## 11. Admin SPA (F25)

**URL:** `https://orca-server/admin`  
**Entry:** `src/renderer/admin-index.html` → `src/renderer/src/admin/admin-main.tsx`  
**Root component:** `AdminApp.tsx` — **prop-driven state routing, NOT React Router.** The code comments say this explicitly:
- `AdminApp.tsx:1-2`: *"Uses prop-driven state routing (no react-router-dom). Pages are loaded lazily."*
- `AdminLayout.tsx:1-2`: *"Uses simple hash-based routing (no react-router-dom dependency required)."*

`react-router` / `react-router-dom` is **not a dependency in any `package.json`** in the repo. Navigation is `useState<AdminRoute>('/')` in `AdminApp`, compared by string in `PageContent()`; `AdminLayout`'s left nav calls `onNavigate(route)` to update it. Each page component is `React.lazy()`-loaded.  
**Guard:** `AdminApp` renders "Not authenticated. Redirecting…" if `useAuthUser()` returns null; the actual admin-role check happens server-side.

**Transport:** REST `/admin/api/*` qua `admin-api-client.ts`

**Real `AdminRoute` union** (`AdminLayout.tsx:5-16`) — 11 values, not the 10-route table this doc previously listed (which included a nonexistent "Departments" page and wrong SSH Hosts / Company Profile paths):

| Page | `AdminRoute` value | Component | Chức năng |
|------|--------------------|-----------|-----------|
| Dashboard | `/` | `AdminDashboard` | Stats: users, sessions, fleet health |
| Users | `/users` | `UsersPage` | List/CRUD users |
| New User | `/users/new` | `UserForm` (mode=create) | Create user form |
| Edit User | `/users/:id/edit` *(matched via `startsWith`, not in the type union)* | `UserForm` (mode=edit) | Edit user form |
| Policies | `/policies` | `PoliciesPage` | RBAC access policies |
| New Policy | `/policies/new` | `PolicyForm` (mode=create) | Create policy form |
| Edit Policy | `/policies/:id/edit` *(same `startsWith` pattern)* | `PolicyForm` (mode=edit) | Edit policy form |
| Sessions | `/sessions` | `SessionsPage` | Xem active sessions, kill session |
| Audit Log | `/audit` | `AuditPage` | Append-only log viewer, date filter |
| Profile | `/profile` | `ProfileAdminPage` (in-page tabs: Company → `CompanyProfileAdmin`, Departments → `DeptProfileAdmin`) | Company Profile **và** Department profile admin — this is where "Company Profile" and department management actually live, not a separate `/admin/company` or `/admin/departments` route |
| AI Providers | `/ai-providers` | `ProviderList` | CRUD accounts, rotate credentials |
| Teams | `/teams` | `TeamAdmin` | Team management (added since the previous audit pass — not in the 8/10-route lists earlier docs cited) |
| Fleet | `/fleet` | `FleetDashboard` (`components/admin/fleet/fleet-dashboard.tsx`) | Health dashboard tất cả servers — there is no separate "SSH Hosts" page; SSH host config is folded into Fleet |

Left-nav (`AdminLayout.tsx` `NAV_ITEMS`) shows 9 top-level entries — Dashboard, Users, Policies, Sessions, Audit Log, AI Providers, Profile, Teams, Fleet — the `/new` and `/:id/edit` routes are reached via in-page buttons, not the nav.

No **Departments** page exists as its own route (grep for `DepartmentsPage` under `components/admin/` — 0 hits); department management is a tab inside `/profile`.

---

## 12. "Routing" (Web Mode) — không có router thật nào

**Không tồn tại client-side router nào cho app chính.** Không `react-router`, không `useParams`, không path pattern `/workspace/:id` ở bất kỳ đâu trong `src/renderer/src`. Bảng route trước đây trong mục này mô tả 1 hệ thống chưa từng được xây.

**Cơ chế thật — 3 entry point riêng biệt, quyết định bởi `web/main.tsx` lúc load:**

1. **Desktop app** (`main.tsx`) — Electron renderer, mount thẳng `<App />`.
2. **Multi-user web app** (`web/main-web-bootstrap.tsx`) — dùng khi server expose `/auth/config` (`ORCA_MULTI_USER=1`). `WebRoot()` branch bằng **boolean/state thuần**, không match URL:
   - `sessionUser !== null` → mount `<App />` trực tiếp (bỏ qua pairing flow)
   - `sessionUser === null` && chưa có stored environment → render `<LoginPage />`
   - có stored/paired environment → mount `<App />`
   - Sau login thành công: **`window.location.href = '/'` cứng** — reload trang, không phải router navigation.
3. **Legacy pair-code web app** (`web/pair-code-app-entry.tsx`) — fallback khi `/auth/config` 404 (server single-user cũ hơn). E2EE pairing UI (`WebConnect`) thay cho login form.
4. **Admin console** (`components/admin/AdminApp.tsx`) — bundle HTML hoàn toàn riêng (`admin-index.html`), không nằm chung route tree với 3 entry point trên. Routing nội bộ của nó là state-branching (§11), cũng không phải router.

**Bên trong `<App />` chính** — không có URL routing: đúng 1 `activeView: TopLevelView` field trong Zustand `ui` slice (`store/slices/ui.ts`), đổi qua `setActiveView()` từ sidebar (`components/sidebar/SidebarNav.tsx`). 9 top-level view: `terminal` (default), `workspace`, `settings`, `tasks`, `activity`, `automations`, `space`, `skills`, `mobile`. Xem [`docs/ui/page-tree.md`](../ui/page-tree.md) và [`docs/ui/README.md`](../ui/README.md) để có bảng đầy đủ view ↔ component.

**Nếu router hoá thực sự là dự định tương lai**, hãy đánh dấu 🚧 Planned trong 1 mục riêng thay vì trình bày như một bảng route đã tồn tại.

---

## 13. Build Configuration

```typescript
// electron.vite.config.ts + vite.web.config.ts + vite.web-spa.config.ts
{
  renderer: {
    build: {
      rollupOptions: {
        input: {
          index: 'src/renderer/index.html',       // Desktop
          web: 'src/renderer/web-index.html',     // Web SPA
          admin: 'src/renderer/admin-index.html', // Admin SPA [v4.0]
        }
      }
    },
    plugins: [react(), tailwindcss(), tsconfigPaths()]
  }
}

// Code splitting:
const TaskPage = lazy(() => import('./components/TaskPage'))
// lazyWithRetry(): retry 3 lần + exponential backoff + Sentry breadcrumb
```

---

## 14. Feature → Frontend Component Mapping

| Feature | Status | Frontend Components | Transport |
|---------|--------|---------------------|-----------|
| **F01** Parallel Worktrees | ✅ | WorktreeSidebar, NewWorkspaceComposerCard | `worktree.*` RPC |
| **F02** Terminal Splits | ✅ | TerminalPane (PaneManager), WorkspaceTerminal | `pty.*` RPC + `pty:data` events |
| **F03** Mobile Companion | ✅ | NotificationsUI (QR), Settings | `mobile.generateQR` RPC |
| **F04** AI Agent Support | ✅ | AgentPanel, TerminalPane (agent output) | `agent.spawn`, `pty.*` RPC |
| **F05** Design Mode | ✅ | BrowserPane (design overlay) | `port:detected` event |
| **F06** GitHub/Linear | ✅ | TaskPage, PullRequestPage, GitHubItemDialog | `github.*`, `linear.*` RPC |
| **F07** SSH Worktrees | ✅ | NewWorkspaceComposerCard (host select), Settings | `ssh.*` RPC |
| **F08** Annotate AI Diffs | ✅ | PullRequestPage (annotation layer) | `git.diff`, `ai.annotate` RPC |
| **F09** Orca CLI | ✅ | — (CLI-only, no UI component) | Unix socket → Daemon |
| **F10** Quick Open | ✅ | QuickOpen modal (Cmd+P) | `fs.glob` RPC |
| **F11** Notifications | ✅ | NotificationsUI (toasts + history), Sonner | `notification.*` events |
| **F12** File Explorer | ✅ | FileExplorer (local), ExplorerPanel (remote) | `fs.readDir`, `fs.readFile` RPC |
| **F13** Text Search | ✅ | FileSearchPanel (trong Explorer) | `fs.grep` RPC (stream) |
| **F14** Automations | 🚧 | AutomationsUI (CRUD + run history) | `automation.*` RPC |
| **F15** Computer Use | 🚧 | BrowserPane (screenshot + AI) | `screenshot.*` RPC |
| **F16** Rich Repo Previews | ✅ | GitHubItemDialog (PR preview cards) | `github.pr.list` RPC |
| **F17** Memory & AI Vault | 🚧 | (future: memory panel) | `memory.*` RPC |
| **F18** Ephemeral VM | 🚧 | (future: VM panel) | `vm.*` RPC |
| **F19** Localization | ✅ | I18nProvider (toàn app), `translate()` | — |
| **F20** Speech Input | 📋 | Prompt editor (mic button) | `speech.*` RPC |
| **F21** Auto Update | ✅ | UpdateCard (Settings tab) | `update.*` (Electron IPC) |
| **F22** Web Server Mode | ✅ | Toàn bộ React SPA (web-index.html) | served từ Backend `:6769` |
| **F23** Multi-User Auth | ✅ | LoginPage, LoginForm, SsoButton, AuthContext | `POST /auth/local` REST |
| **F24** Per-User Sandbox | ✅ | — (transparent, handled backend) | — |
| **F25** Admin Panel | ✅ | AdminApp + 11-route table, §11 (admin-index.html) | `GET/POST /admin/api/*` REST |
| **F26** Multi-Database | ✅ | — (transparent to frontend) | — |
| **F27** Fleet Health | ✅ | FleetHealthDashboard (`settings/ssh/`), ServerStatusBar | `fleet.getStatus` RPC |
| **F28** Dev Server Onboarding | ✅ | DevServerCard/List/Dialog, ServerBootstrapPanel | `fleet.provision` RPC |
| **F29** Agent WebSocket Protocol | ✅ | — (WebSocketRpcClient handles transparently) | plain JSON WS messages (§5.1) — not binary frames |
| **F30** Remote Integrations | ✅ | Settings (Integrations tab), GitPanel | `credentials.*`, `preflight.check` RPC |
| **F31** Fleet Provisioning | ✅ | FleetProvisionWizard (`settings/ssh/`), fleet-import-dialog (`admin/fleet/`) | `fleet.provision` RPC |
| **F32** Team RBAC | 📋 | UsersPage + TeamAdmin (Admin `/teams`, not a "DepartmentsPage") | `admin.users.*` REST |
| **F33** Profile Hierarchy | 🚧 | ProfileEditor, ProfileSourceBadge, Settings (Profile tab) | `profile.*` RPC |
| **F34** Project-Dev Server Binding | 🚧 | ProjectSelector, Admin SSH Hosts | `projects.*` RPC |
| **F35** AI Provider Mgmt | 🚧 | AIProvidersPage (Admin), AgentPanel (provider badge) | `ai-providers.*` RPC |
| **F36** Workflow Orchestration | 🚧 | WorkflowBuilder, ExecutionMonitor, DAGPreview | `workflows.*` RPC |
| **F37** Task Graph | 🚧 | TaskGraph (React Flow), TaskCard, TaskDetail, TaskAIDecompose | `tasks.*` RPC |
| **F38** Project Workspace | 🚧 | WorkspaceLayout + ExplorerPanel + AgentPanel + Terminal | `projects.*`, `fs.*`, `git.*` RPC |
| **F39** Remote Git UI | 🚧 | GitPanel (full: diff/commit/push/PR) | `git.*`, `ai.complete` RPC |

---

## 15. Sơ đồ tổng quan (Frontend)

```
BROWSER
══════════════════════════════════════════════════════════════════
│
│  React App (Vite build)
│  │
│  ├── AuthContext ──── POST /auth/local ─────────────────────────→ Backend :6769
│  ├── Admin SPA  ──── GET/POST /admin/api/* ─────────────────────→ Backend :6769
│  │
│  ├── WebSocketRpcClient ──── WSS /:6768/ ───────────────────────→ Backend :6768
│  │   └── JSON invoke/send envelopes (plain text, §5.1) ─────────→ WsSessionRouter
│  │   └── server-push events ←────────────────────────────────── (pty:data, agent:status, ...)
│  │
│  ├── Zustand Store (40+ slices)
│  │   ├── sync-runtime-graph.ts ← scheduleRuntimeGraphSync() ← mutating RPC responses
│  │   └── slices: repos, worktrees, terminals, auth, profile, project, task, workflow, ...
│  │
│  ├── App.tsx (App Shell)
│  │   ├── Sidebar ──────── worktree.* RPC
│  │   ├── TerminalPane ─── pty.* RPC + pty:data events
│  │   ├── PullRequestPage ─ github.* RPC
│  │   ├── TaskPage ──────── github.*,linear.*,jira.* RPC
│  │   ├── FileExplorer ──── fs.* RPC (local) / fs.* relay (remote)
│  │   ├── BrowserPane ───── port:detected events
│  │   └── QuickOpen ─────── fs.glob RPC
│  │
│  └── WorkspaceContext (v5.0)
│      ├── ProjectSelector ────── projects.* RPC
│      ├── ExplorerPanel ─────── fs.* RPC (remote file tree)
│      ├── GitPanel ──────────── git.* RPC (remote git ops)
│      ├── AgentPanel ────────── tasks.runAgent / pty.* RPC
│      ├── WorkflowBuilder ───── workflows.* RPC
│      ├── TaskGraph ─────────── tasks.* RPC
│      └── WorkspaceTerminal ─── pty.* RPC
│
══════════════════════════════════════════════════════════════════
(tất cả xử lý nghiệp vụ → Backend Server → Dev Server Agent)
```
