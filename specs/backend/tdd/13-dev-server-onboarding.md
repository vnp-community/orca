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
