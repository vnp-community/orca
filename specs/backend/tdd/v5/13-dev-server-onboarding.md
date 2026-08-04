# TDD-13: Dev Server Management & Onboarding

**Document:** TDD-13 (NEW — onboarding CRs)  
**Version:** 1.0  
**Date:** 2026-07-23  
**Domain:** Remote Dev Server — Registration, Connection, Agent Detection, Onboarding  
**Source files:**
- `src/main/dev-server/`
- `src/main/ipc/dev-server-ipc.ts`
- `src/main/ipc/onboarding-ipc.ts`
- `src/main/ipc/repo-remote-ipc.ts`
- `src/shared/dev-server-types.ts`

> **Status: ✅ IMPLEMENTED** — 9/9 solutions (Phase 1, 2, 3) | All handlers registered in server-bootstrap

---

## 1. Mục tiêu

Dev Server Management cho phép Orca Server quản lý **nhiều remote dev servers**:

```
Orca Server (Docker)
  ├── DevServerManager
  │     ├── devServer1 (macOS) ←─── SSH Relay ─────── MacBook Pro
  │     ├── devServer2 (Linux) ←─── SSH Relay ─────── Ubuntu Workstation
  │     └── devServer3 (Win32) ←─── SSH Relay ─────── Windows PC
  │
  └── Onboarding Wizard (web SPA)
        ├── Platform detection (per server)
        ├── Agent detection (gh, git, claude, cursor...)
        ├── Preflight checks (Node, Git version)
        └── Repo clone / folder open
```

---

## 2. Data Types (`src/shared/dev-server-types.ts`)

```typescript
type DevServerConnectionType =
  | 'relay-ssh'          // Orca SSH → deploy relay → stdin/stdout
  | 'relay-websocket'    // Dev server connects WS → Orca (reverse)
  | 'direct-websocket'   // Orca connects WS → dev server relay

type DevServerStatus =
  | 'connected'
  | 'disconnected'
  | 'connecting'
  | 'error'

type DevServer = {
  id: string                         // 'ds-<uuid>'
  name: string                       // "MacBook Pro M3"
  connectionType: DevServerConnectionType
  sshTargetId?: string               // → SshTarget for relay-ssh
  wsUrl?: string                     // ws://devserver.local:6799
  // Runtime (không persist)
  status: DevServerStatus
  platform: NodeJS.Platform | null   // populated after handshake
  arch: string | null
  nodeVersion: string | null
  lastConnectedAt: number | null
  lastError: string | null
  workspaceDir: string | null
  addedAt: number
}

type DevServerInput = {
  name: string
  connectionType: DevServerConnectionType
  sshTargetId?: string
  wsUrl?: string
}

type ConnectionTestResult =
  | { ok: true; platform: NodeJS.Platform; nodeVersion: string }
  | { ok: false; error: string; hint?: string }

type WindowsTerminalCapabilities = {
  wslAvailable: boolean
  wslDistros: string[]
  pwshAvailable: boolean
  pwshVersion?: string         // e.g. "PowerShell 7.4.1"
  gitBashAvailable: boolean
  gitBashPath?: string         // e.g. "C:\Program Files\Git\bin\bash.exe"
}
```

**Persistence trong `PersistedState`:**
```typescript
// src/shared/types.ts (MODIFY):
type PersistedState = {
  // ...existing fields...
  devServers: PersistedDevServer[]    // NEW — không persist runtime fields
}

// GlobalSettings (MODIFY):
terminalWindowsConfigByServer?: Record<string, {
  shell: string
  wslDistro: string | null
  rightClickToPaste: boolean
}>
```

---

## 3. DevServerManager (`src/main/dev-server/`)

```
src/main/dev-server/
├── dev-server-manager.ts        ← Lifecycle: add, connect, disconnect, remove
├── dev-server-store.ts          ← CRUD trên PersistedState.devServers
├── dev-server-relay-bridge.ts   ← Wrap SshRelaySession → DevServerRelay interface
└── dev-server-preflight.ts      ← Test connection, platform probe
```

### 3.1 DevServerManager API

```typescript
class DevServerManager {
  constructor(store: Store, sshManager: SshConnectionManager)

  // CRUD
  add(input: DevServerInput): DevServer
  get(id: string): DevServer | undefined
  getAll(): DevServer[]
  update(id: string, updates: Partial<DevServerInput>): DevServer
  remove(id: string): void

  // Connection
  connect(id: string): Promise<void>
  disconnect(id: string): void
  getRelay(id: string): DevServerRelay | null

  // Testing
  testConnection(id: string): Promise<ConnectionTestResult>
  detectAgents(id: string, commands: AgentDetectionCommand[]): Promise<{
    agents: string[]
    platform: NodeJS.Platform
  }>

  // Events
  on(event: 'statusChanged', cb: (id: string, status: DevServerStatus) => void): void
  on(event: 'platformDetected', cb: (id: string, server: DevServer) => void): void
}
```

### 3.2 DevServerRelay Interface

```typescript
interface DevServerRelay {
  session: RelaySession        // underlying SSH relay session
  detectAgents(commands: AgentDetectionCommand[]): Promise<AgentDetectionResult>
  runPreflight(): Promise<PreflightResult>
  detectWindowsCapabilities(): Promise<WindowsTerminalCapabilities>
  exec(command: string): Promise<{ stdout: string; stderr: string; exitCode: number }>
}
```

---

## 4. IPC Handlers

### 4.1 Dev Server IPC (`src/main/ipc/dev-server-ipc.ts`)

```typescript
export function registerDevServerIpcHandlers(
  devServerManager: DevServerManager,
  store: Store
): void

// Handlers registered:
ipc.handle('devServer.list', ...) → DevServer[]
ipc.handle('devServer.add', ...)  → DevServer
ipc.handle('devServer.update', ...) → DevServer
ipc.handle('devServer.remove', ...) → void
ipc.handle('devServer.connect', ...) → void
ipc.handle('devServer.disconnect', ...) → void
ipc.handle('devServer.testConnection', ...) → ConnectionTestResult
```

### 4.2 Onboarding IPC (`src/main/ipc/onboarding-ipc.ts`)

```typescript
export function registerOnboardingIpcHandlers(
  devServerManager: DevServerManager,
  store: Store
): void

// Handlers registered:
ipc.handle('onboarding.detectAgents', async (_, { devServerId }) → {
  agents: string[]
  platform: NodeJS.Platform | null
  devServerId: string | null
})

ipc.handle('onboarding.runPreflight', async (_, { devServerId }) → PreflightResult)

ipc.handle('onboarding.detectWindowsCapabilities', async (_, { devServerId }) → {
  WindowsTerminalCapabilities
  // Cached 60 seconds per devServerId
})

ipc.handle('onboarding.cloneRepo', async (_, {
  devServerId,
  repoUrl,
  targetDir,
  authToken?
}) → { localPath: string })

ipc.handle('onboarding.openFolder', async (_, {
  devServerId,
  folderPath
}) → { worktreeId: string })
```

**Windows capabilities cache (60s TTL):**
```typescript
const windowsCapsCache = new Map<string, {
  result: WindowsTerminalCapabilities
  cachedAt: number
}>()
// Key: `win-caps-${devServerId}`
```

### 4.3 Repo Remote IPC (`src/main/ipc/repo-remote-ipc.ts`)

```typescript
export function registerRepoRemoteIpcHandlers(
  devServerManager: DevServerManager,
  store: Store
): void

// Handlers registered:
ipc.handle('repoRemote.list', ...) → Repo[]
ipc.handle('repoRemote.clone', ...) → { repoId: string }
ipc.handle('repoRemote.syncStatus', ...) → { ahead: number; behind: number }
```

---

## 5. Remote Agent Detection (`src/relay/preflight-handler.ts`)

Agent detection chạy **trên relay process** (remote dev server):

```typescript
// preflight.detectAgents response (MODIFIED):
{
  agents: string[]         // e.g. ['github-copilot', 'claude', 'cursor']
  platform: NodeJS.Platform  // NEW — 'win32' | 'linux' | 'darwin'
}

// preflight.detectWindowsTerminalCapabilities (MODIFIED):
{
  wslAvailable: boolean
  wslDistros: string[]
  pwshAvailable: boolean
  pwshVersion?: string       // NEW — e.g. "PowerShell 7.4.1"
  gitBashAvailable: boolean
  gitBashPath?: string       // NEW — full path
}
```

**Call flow:**
```
Browser (web SPA)
  → ipc.invoke('onboarding.detectAgents', { devServerId })
  → NodeIpcBridge.invoke() → onboarding-ipc.ts handler
  → DevServerManager.getRelay(devServerId)
  → relay.session.call('preflight.detectAgents', { commands })
  → [SSH] → relay process on dev server
  → { agents, platform }
```

---

## 6. Registration in Server Bootstrap

```typescript
// src/main/server-bootstrap.ts:

// DevServerManager (Phase 1 — TASK-OB-002)
const sshManager = new SshConnectionManager({ ... })
const devServerManager = new DevServerManager(store, sshManager)
registerDevServerIpcHandlers(devServerManager, store)
registerOnboardingIpcHandlers(devServerManager, store)
registerRepoRemoteIpcHandlers(devServerManager, store)

// WebPushManager (Phase 3 — TASK-OB-035)
const pushManager = new WebPushManager(store)
// Wired to OrcaRuntimeService after it's created:
runtime.setPushManager(pushManager)

// Returns:
return {
  devServerManager,
  dbMonitor,
  pushManager,
  shutdown: async () => { ... }
}
```

---

## 7. Web Push Notifications (`src/main/notifications/`)

```typescript
// src/main/notifications/web-push-manager.ts
class WebPushManager {
  constructor(store: Store)  // persists VAPID keys in store

  // VAPID key management
  getVapidPublicKey(): string
  generateVapidKeys(): void   // on first init

  // Subscription management
  subscribe(subscription: PushSubscription): void
  unsubscribe(endpoint: string): void

  // Push
  send(notification: WebPushNotification): Promise<void>
  broadcast(notification: WebPushNotification): Promise<void>
}

// src/server/push-api-routes.ts (TASK-035)
export function registerPushApiRoutes(
  server: http.Server,
  pushManager: WebPushManager
): void
// Routes:
// POST /push/subscribe    ← Browser registers subscription
// DELETE /push/subscribe  ← Browser unregisters
// GET /push/vapid-key     ← Get server VAPID public key
```

**Service Worker (`src/renderer/service-worker.js`):**
```javascript
// Handles push events from server
self.addEventListener('push', event => {
  const data = event.data.json()
  event.waitUntil(
    self.registration.showNotification(data.title, {
      body: data.body,
      icon: '/assets/icon.png',
      tag: data.tag
    })
  )
})
```

---

## 8. Fleet Management (`src/main/ssh/fleet-*.ts`)

Từ remote-server CRs — extends SSH layer với fleet capabilities:

```
src/main/ssh/
├── fleet-config-parser.ts         ← YAML fleet config + Zod validation
├── fleet-remote-commands.ts       ← Node/Git install, repo clone via SSH
├── fleet-bootstrap-service.ts     ← 7-step bootstrap pipeline
├── fleet-health-store.ts          ← In-memory health history + uptime
├── fleet-health-monitor.ts        ← Periodic poll + webhook/IPC alerts
└── fleet-status-service.ts        ← getFleetStatus() standalone
src/shared/
├── fleet-types.ts                 ← FleetServerStatus, FleetStatusReport
└── rbac-types.ts                  ← OrcaUser, OrcaAccessPolicy, ScopedPairingToken
src/main/audit/
└── audit-log.ts                   ← NDJSON audit log (record + query)
```

### 8.1 SshTarget Extensions (`src/shared/ssh-types.ts`)

```typescript
// Extended SshTarget (CR-001, CR-002):
type SshTarget = {
  // ...existing fields...
  tags?: string[]                    // NEW — for grouping
  group?: string                     // NEW — group name
  metadata?: Record<string, string>  // NEW — custom key-value
  importedFrom?: 'ssh-config' | 'fleet-yaml' | 'manual'  // NEW
  fleetConfigPath?: string           // NEW — which fleet.yaml
}

// New type:
type SshTargetGroup = {
  name: string
  targets: SshTarget[]
  tags: string[]
}
```

### 8.2 Fleet Config YAML

```yaml
# orca-fleet.yaml
fleet:
  servers:
    - host: dev1.internal
      port: 22
      user: ubuntu
      identity: ~/.ssh/id_ed25519
      tags: [linux, backend, staging]
      group: staging-backend
    - host: 192.168.1.100
      user: ubuntu
      tags: [linux, devbox]
  defaults:
    user: ubuntu
    identity: ~/.ssh/id_ed25519
```

### 8.3 Fleet IPC Handlers (`src/main/ipc/ssh.ts` additions)

```typescript
// 12+ new IPC handlers added:
ipc.handle('ssh:fleet:import', ...)              // Import from YAML
ipc.handle('ssh:fleet:status', ...)             // Current fleet health
ipc.handle('ssh:fleet:bootstrap', ...)          // 7-step bootstrap
ipc.handle('ssh:fleet:listByGroup', ...)        // Grouped targets
ipc.handle('ssh:fleet:getHealthHistory', ...)   // Health time series
// + 7 more
```

### 8.4 Bootstrap Pipeline (`fleet-bootstrap-service.ts`)

7-step pipeline, each step idempotent:
```
Step 1: SSH connectivity test
Step 2: Remote platform detection
Step 3: Node.js version check / install
Step 4: Git version check / install
Step 5: Relay binary deploy
Step 6: Relay process start
Step 7: Agent detection + handshake
```

---

## 9. RBAC (`src/shared/rbac-types.ts`, CR-006)

Phase 1+2 completed (multi-instance isolation):
```typescript
type OrcaUser = {
  id: string
  email: string
  displayName: string
  role: 'admin' | 'developer' | 'viewer'
  sshTargetIds: string[]    // scoped access
}

type ScopedPairingToken = {
  token: string
  userId: string
  allowedSshTargetIds: string[]
  expiresAt: number
}
```

Phase 3 deferred: OIDC/SSO handler (`src/main/auth/oidc-handler.ts`)

---

## 10. Key Files Reference

| File | CR | Role |
|------|----|------|
| `src/shared/dev-server-types.ts` | OB-002 | DevServer, ConnectionTestResult types |
| `src/main/dev-server/dev-server-manager.ts` | OB-002 | Lifecycle management |
| `src/main/dev-server/dev-server-relay-bridge.ts` | OB-002 | SshRelaySession wrapper |
| `src/main/dev-server/dev-server-preflight.ts` | OB-002, OB-003 | Connection test + platform probe |
| `src/main/ipc/dev-server-ipc.ts` | OB-002 | IPC handlers for DevServer CRUD |
| `src/main/ipc/onboarding-ipc.ts` | OB-003..009 | Agent detection, preflight, clone |
| `src/main/ipc/repo-remote-ipc.ts` | OB-006 | Remote repo management |
| `src/main/notifications/web-push-manager.ts` | OB-008 | VAPID + push subscriptions |
| `src/server/push-api-routes.ts` | OB-008 | Push API HTTP routes |
| `src/renderer/service-worker.js` | OB-008 | Browser Service Worker |
| `src/main/ssh/fleet-config-parser.ts` | RS-001 | YAML fleet config |
| `src/main/ssh/fleet-bootstrap-service.ts` | RS-004 | 7-step bootstrap |
| `src/main/ssh/fleet-health-monitor.ts` | RS-005 | Periodic health + alerts |
| `src/main/audit/audit-log.ts` | RS-006 | NDJSON audit trail |
| `src/shared/rbac-types.ts` | RS-006 | RBAC types |

---

## 11. Provider Unification with SSH Registries (v5.0) — IMPLEMENTED ✅

> **Date:** 2026-08-02/03 | **TDD-05:** [05-ssh-relay.md § Addendum v5.0](./05-ssh-relay.md#addendum-v50-provider-registries-are-transport-agnostic)

### 11.1 Vấn đề

Trước v5.0, một Repo có thể gắn với remote host theo 2 cách **không liên quan gì nhau**:

1. Classic **SSH Targets/Hosts** — `Repo.connectionId` / `Repo.executionHostId`, mọi file/git/terminal op đi qua `orca-runtime.ts` lookup trong `ssh-filesystem-dispatch.ts`/`ssh-git-dispatch.ts`.
2. **Dev Servers** — `Repo.devServerId`, agent kết nối OUTBOUND qua WebSocket tới Orca server (§2-§3 ở trên), nhưng trước đây chỉ được wire vào flow onboarding/project hẹp — **vô hình** với machinery `Repo`/`orca-runtime.ts` cổ điển.

### 11.2 Giải pháp

Vì `ssh-filesystem-dispatch.ts`/`ssh-git-dispatch.ts` vốn đã keyed bằng 1 opaque connection-id string và transport-agnostic, Dev Server's agent connection sẵn có được đăng ký vào **CÙNG registry** — không mở connection mới, gần như zero-change cho ~40+ call site đọc từ registry. Chi tiết provider class + registry widening: xem [05-ssh-relay.md § Addendum v5.0](./05-ssh-relay.md#addendum-v50-provider-registries-are-transport-agnostic).

Helper hợp nhất `getRepoProviderConnectionKey(repo)` (`src/shared/execution-host.ts`):

```typescript
export function getRepoProviderConnectionKey(
  repo: Pick<Repo, 'connectionId' | 'devServerId'>
): string | null {
  return normalizeHostPart(repo.connectionId) ?? normalizeHostPart(repo.devServerId) ?? null
}
```

Đây là bare provider-registry lookup key — khác với `ExecutionHostId` dạng prefix (`ssh:<id>` / `devServer:<id>`) dùng ở UI. Áp dụng xuyên suốt `orca-runtime.ts` (choke-point `resolveRuntimeGitTarget`/`resolveRuntimeFileTarget`, fix ~45 call site downstream miễn phí), `worktree-remote.ts` (~24 hàm retype từ `SshGitProvider` sang `IGitProvider`), `worktrees.ts` (53 call site), `repos.ts`, `git-username.ts`, `pr-head-tracking-ref.ts`, `first-work-generation-target.ts`, `first-work-branch-rename.ts`, `workspace-cleanup-scan.ts`, `repo-worktrees.ts`, `workspace-space-analysis.ts`.

**Ngoài phạm vi (giữ nguyên SSH-only):** GitHub/GitLab/Gitea/Bitbucket/Azure DevOps-specific `connectionId` usages — muốn hỗ trợ Dev Server sẽ cần extension riêng.

### 11.3 Repo / Host-Setup IPC (`src/main/ipc/repos.ts`)

`addRemoteRepoFromPath` có thêm param `hostKind?: 'ssh' | 'devServer'` (default `'ssh'`); repo persistence ghi `connectionId` (ssh) hoặc `devServerId` (devServer) tuỳ `hostKind`. Handler `'projectHostSetups:setupExistingFolder'` branch theo `parseExecutionHostId(args.hostId)?.kind === 'devServer'` → gọi `addRemoteRepoFromPath({ hostKind: 'devServer', ... })` — full flow "set up existing folder as project on Dev Server" hoạt động end-to-end.

**Chưa làm:** Clone repo MỚI lên Dev Server (`cloneRemoteRepo`/`repos:cloneRemote`) — hàm này phụ thuộc khái niệm SSH-only (`getHostPlatform()`, remote home-path resolution, SSH multiplexer progress notify) chưa có tương đương ở Dev Server. Guard mới throw rõ ràng thay vì âm thầm clone lên local filesystem của chính Orca server (rủi ro bug thật trước khi fix).

### 11.4 Realtime Notification Relay (Multi-User / Web Mode)

**Vấn đề:** `GatewayDevServerManagerProxy` (per-user child-process proxy tới `DevServerManager` thật sống ở parent/gateway process) trước đây chỉ hỗ trợ request/response `call()` — không có đường cho agent PUSH notification (pty output, file-change event) vào child process của đúng user.

**Giải pháp:** Broadcast message type mới `devServer:proxyNotification`, song song với `devServer:event` đã có:

```
DevServerRelayBridge (parent, wrap agent connection — cả 3 mode relay-ssh/relay-websocket/direct-websocket)
  └── public onNotification(handler): unsubscribe
        // Subscriber lưu độc lập với session hiện tại, tự rewire vào mỗi
        // SshChannelMultiplexer session mới khi reconnect — sống sót qua
        // reconnect, khác với subscribe thẳng vào mux (chết theo mux)

DevServerManager.connect() / .connectDaemonAgent()
  └── bridge.onNotification((method, params) => this.emit('devServer:notification', id, method, params))

SessionManager (parent process)
  └── on('devServer:notification') → proc.process.send({
        type: 'devServer:proxyNotification', devServerId, method, params
      })  // tới mọi live user child process

GatewayDevServerManagerProxy (child process)
  └── process.on('message') nhận 'devServer:proxyNotification'
        → re-dispatch tới subscriber đăng ký qua getRelay(id).onNotification(handler)
```

### 11.5 Rollout Process (operational note)

Agent binary mới (`agent.js`) được deploy tới `test-01` trước qua `deploy/dev/scripts/deploy-agents.sh --server TEST01`, verify qua log (reconnect sạch, capability mới `fs.watch`/`pty.stream` advertise đúng, capability `git`/`worktrees` không đổi, 0 lỗi trong log orca-server) — rồi mới rollout tiếp `dev-01`/`dev-ai` (`--server DEV01`/`--server DEV02`). Pattern staged-rollout này là cách an toàn để ship thay đổi agent-wire-protocol mà không phá vỡ Dev Server đang connect.

### 11.6 Explicitly Deferred

`DevServerPtyProvider` (`IPtyProvider` cho Dev Server agent) **chưa xây**. `IPtyProvider` cần ~8 method (getCwd, hasChildProcesses, getForegroundProcess, serialize, revive, clearBuffer, acknowledgeDataEvent) không có equivalent trung thực ở phía agent nếu không lossy-approximate; đồng thời chưa Dev Server nào report `pty=true` ở handshake (node-pty native binary fail load ở deployment hiện tại) — nên terminal-over-Dev-Server vẫn unavailable end-to-end bất kể provider có hoàn chỉnh hay không. `pty.create` mới ở agent-side với push notification `pty.data`/`pty.exit` real-time, và fix `normalizeConnectionId()` trong `ssh-pty-id.ts` (giờ unwrap cả `kind === 'devServer'`, không chỉ `kind === 'ssh'`) là groundwork cho việc này, nhưng provider class là future work.
