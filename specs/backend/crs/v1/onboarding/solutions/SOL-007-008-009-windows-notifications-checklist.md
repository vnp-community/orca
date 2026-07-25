# SOL-007 + SOL-008 + SOL-009: Windows Terminal, Notifications & Checklist

**CRs:** [CR-OB-007](../../../../../docs/crs/v1/onboarding/CR-OB-007-windows-terminal-remote.md) | [CR-OB-008](../../../../../docs/crs/v1/onboarding/CR-OB-008-notification-server.md) | [CR-OB-009](../../../../../docs/crs/v1/onboarding/CR-OB-009-multi-devserver-checklist.md)  
**TDD refs:** TDD-06 (Persistence), TDD-09 (IPC), TDD-11 (Web Server)  
**Status:** ✅ Implemented | **Phase:** 3  
**Depends on:** SOL-002, SOL-004

> Gộp 3 CR thành 1 file vì cùng thuộc Phase 3 (polish layer).

---

## PART A — SOL-007: Remote Windows Terminal Detection

### A.1 Relay — Windows Capabilities (không thay đổi core)

```typescript
// src/relay/preflight-handler.ts — đã có detectWindowsTerminalCapabilities()
// CHỈ thêm pwshVersion và gitBashPath vào response:

private async detectWindowsTerminalCapabilities(): Promise<{
  wslAvailable: boolean
  wslDistros: string[]
  pwshAvailable: boolean
  pwshVersion?: string       // NEW
  gitBashAvailable: boolean
  gitBashPath?: string       // NEW
}> {
  const [wslResult, pwshResult, gitBashResult] = await Promise.all([
    this.checkWsl(),
    this.checkPwsh(),
    this.checkGitBash()
  ])
  return { ...wslResult, ...pwshResult, ...gitBashResult }
}

private async checkPwsh(): Promise<{ pwshAvailable: boolean; pwshVersion?: string }> {
  try {
    const { stdout } = await execFileAsync('pwsh', ['--version'])
    return { pwshAvailable: true, pwshVersion: stdout.trim() }
  } catch {
    return { pwshAvailable: false }
  }
}

private async checkGitBash(): Promise<{ gitBashAvailable: boolean; gitBashPath?: string }> {
  // Windows: check common Git Bash locations
  const candidates = [
    'C:\\Program Files\\Git\\bin\\bash.exe',
    'C:\\Program Files (x86)\\Git\\bin\\bash.exe'
  ]
  for (const candidate of candidates) {
    try {
      await stat(candidate)
      return { gitBashAvailable: true, gitBashPath: candidate }
    } catch { /* continue */ }
  }
  return { gitBashAvailable: false }
}
```

### A.2 IPC Handler

```typescript
// src/main/ipc/onboarding-ipc.ts (MODIFY — thêm):
ipc.handle('onboarding.detectWindowsCapabilities', async (_, params: {
  devServerId: string
}): Promise<WindowsTerminalCapabilities> => {
  const devServer = devServerManager.get(params.devServerId)
  if (!devServer) throw new Error('Dev server not found')
  if (devServer.platform !== 'win32') {
    throw new Error(`Dev server ${params.devServerId} is not Windows (platform: ${devServer.platform})`)
  }
  const relay = devServerManager.getRelay(params.devServerId)
  if (!relay) throw new Error('Dev server not connected')

  // Cache TTL 60s
  const cacheKey = `win-caps-${params.devServerId}`
  const cached = windowsCapsCache.get(cacheKey)
  if (cached && Date.now() - cached.cachedAt < 60_000) return cached.result

  const result = await relay.session.call('preflight.detectWindowsTerminalCapabilities', {})
  windowsCapsCache.set(cacheKey, { result, cachedAt: Date.now() })
  return result
})

const windowsCapsCache = new Map<string, { result: WindowsTerminalCapabilities; cachedAt: number }>()
```

### A.3 Settings per Dev Server

```typescript
// src/shared/types.ts (MODIFY):
type GlobalSettings = {
  // ...existing terminal settings giữ nguyên cho backward compat...
  terminalWindowsConfigByServer?: Record<string, {   // NEW
    shell: string                    // 'powershell.exe' | 'cmd.exe' | 'wsl.exe' | git-bash-path
    wslDistro: string | null
    rightClickToPaste: boolean
  }>
}
```

---

## PART B — SOL-008: Web Push Notifications (Backend)

### B.1 Architecture

```
browser.serviceWorker.subscribe(vapidPublicKey)
     → POST /api/push-subscribe { subscription: PushSubscriptionJSON }
     → WebPushManager.saveSubscription(userId, subscription)
     → stored in PersistedState.webPushSubscriptions[]

Agent task complete
     → NotificationService.send({ userId, payload })
     → WebPushManager.sendToUser(userId, payload)    ← web-push library
     → Browser ServiceWorker push event
     → self.registration.showNotification(...)
```

### B.2 New Files

```
src/main/notifications/
├── web-push-manager.ts          ← VAPID keys, subscription CRUD, send
└── notification-service.ts      ← Delivery channel router (existing → extend)

src/server/
└── push-api-routes.ts           ← HTTP endpoints: /api/vapid-key, /api/push-subscribe
```

### B.3 VAPID + PushSubscription Schema

```typescript
// src/shared/types.ts (MODIFY):
type PersistedState = {
  // ...existing...
  webPushSubscriptions?: WebPushSubscription[]   // NEW
  vapidKeys?: { publicKey: string; privateKey: string }  // NEW
}

type WebPushSubscription = {
  id: string
  endpoint: string
  keys: { auth: string; p256dh: string }
  addedAt: number
  userAgent?: string
}
```

### B.4 WebPushManager — `src/main/notifications/web-push-manager.ts`

```typescript
import webPush from 'web-push'
import { randomUUID } from 'node:crypto'
import type { Store } from '../persistence'

export class WebPushManager {
  private vapidKeys: { publicKey: string; privateKey: string }

  constructor(private store: Store) {
    this.vapidKeys = this.loadOrCreateVapidKeys()
    webPush.setVapidDetails(
      'mailto:admin@orca.local',
      this.vapidKeys.publicKey,
      this.vapidKeys.privateKey
    )
  }

  getPublicKey(): string {
    return this.vapidKeys.publicKey
  }

  saveSubscription(subscription: PushSubscriptionJSON, meta?: { userAgent?: string }): WebPushSubscription {
    const record: WebPushSubscription = {
      id: randomUUID(),
      endpoint: subscription.endpoint!,
      keys: {
        auth: subscription.keys!.auth,
        p256dh: subscription.keys!.p256dh
      },
      addedAt: Date.now(),
      userAgent: meta?.userAgent
    }
    this.store.mutate(state => {
      state.webPushSubscriptions = [
        ...(state.webPushSubscriptions ?? []).filter(s => s.endpoint !== record.endpoint),
        record
      ]
    })
    return record
  }

  removeSubscription(endpoint: string): void {
    this.store.mutate(state => {
      state.webPushSubscriptions = (state.webPushSubscriptions ?? [])
        .filter(s => s.endpoint !== endpoint)
    })
  }

  async sendToAll(payload: WebPushPayload): Promise<void> {
    const subscriptions = this.store.getState().webPushSubscriptions ?? []
    await Promise.allSettled(
      subscriptions.map(sub => this.sendToSubscription(sub, payload))
    )
  }

  private async sendToSubscription(sub: WebPushSubscription, payload: WebPushPayload): Promise<void> {
    try {
      await webPush.sendNotification(
        { endpoint: sub.endpoint, keys: sub.keys },
        JSON.stringify(payload),
        { TTL: 86400 }
      )
    } catch (err: unknown) {
      // 410 Gone: subscription expired — remove it
      if ((err as { statusCode?: number }).statusCode === 410) {
        this.removeSubscription(sub.endpoint)
      }
      // Other errors: log and continue
    }
  }

  private loadOrCreateVapidKeys(): { publicKey: string; privateKey: string } {
    const stored = this.store.getState().vapidKeys
    if (stored) return stored
    const keys = webPush.generateVAPIDKeys()
    this.store.mutate(state => { state.vapidKeys = keys })
    return keys
  }
}

type WebPushPayload = {
  title: string
  body: string
  icon?: string
  tag?: string
  url?: string
}
```

### B.5 HTTP Push API Endpoints — `src/server/push-api-routes.ts`

```typescript
import type { IncomingMessage, ServerResponse } from 'node:http'
import type { WebPushManager } from '../main/notifications/web-push-manager'

export function registerPushApiRoutes(
  server: import('node:http').Server,
  pushManager: WebPushManager
): void {
  server.on('request', (req: IncomingMessage, res: ServerResponse) => {
    const url = req.url ?? ''

    // GET /api/vapid-public-key
    if (req.method === 'GET' && url === '/api/vapid-public-key') {
      res.writeHead(200, { 'Content-Type': 'application/json' })
      res.end(JSON.stringify({ publicKey: pushManager.getPublicKey() }))
      return
    }

    // POST /api/push-subscribe
    if (req.method === 'POST' && url === '/api/push-subscribe') {
      readBody(req).then(body => {
        const { subscription } = JSON.parse(body)
        const record = pushManager.saveSubscription(subscription, {
          userAgent: req.headers['user-agent']
        })
        res.writeHead(201, { 'Content-Type': 'application/json' })
        res.end(JSON.stringify({ id: record.id }))
      }).catch(() => {
        res.writeHead(400)
        res.end('Invalid body')
      })
      return
    }

    // POST /api/push-unsubscribe
    if (req.method === 'POST' && url === '/api/push-unsubscribe') {
      readBody(req).then(body => {
        const { endpoint } = JSON.parse(body)
        pushManager.removeSubscription(endpoint)
        res.writeHead(204)
        res.end()
      }).catch(() => {
        res.writeHead(400)
        res.end()
      })
      return
    }
  })
}

async function readBody(req: IncomingMessage): Promise<string> {
  return new Promise((resolve, reject) => {
    let data = ''
    req.on('data', chunk => { data += chunk })
    req.on('end', () => resolve(data))
    req.on('error', reject)
  })
}
```

### B.6 Integrate trong server-bootstrap + http-server

```typescript
// src/main/server-bootstrap.ts (MODIFY):
import { WebPushManager } from './notifications/web-push-manager'

export async function initializeOrcaServices(options): Promise<ServerBootstrapResult> {
  // ...existing init...
  const pushManager = new WebPushManager(store)    // NEW
  // Expose qua rpcServer (IPC):
  rpcServer.setPushManager(pushManager)
  return { shutdown, pushManager }  // expose để http-server dùng
}
```

```typescript
// src/server/index.ts (MODIFY):
const { shutdown, pushManager } = await initializeOrcaServices({ platform: adapter, port: rpcPort })
if (existsSync(webRoot)) {
  const httpServer = await startHttpServer(httpPort, webRoot)
  registerPushApiRoutes(httpServer, pushManager)   // NEW
}
```

### B.7 Notification trigger khi Agent complete

```typescript
// src/main/runtime/orca-runtime.ts (MODIFY) — trong agent task complete handler:
private async onAgentTaskComplete(worktreeId: string, summary: string): Promise<void> {
  // ...existing logic...
  // NEW: push notification
  await this.pushManager?.sendToAll({
    title: 'Task complete',
    body: summary,
    tag: `worktree-${worktreeId}`,
    url: `/worktree/${worktreeId}`
  })
}
```

---

## PART C — SOL-009: Multi Dev-Server Checklist

### C.1 Schema — OnboardingChecklistState (MODIFY)

```typescript
// src/shared/types.ts:
type OnboardingChecklistState = {
  // Global items (không đổi — 1 lần cho toàn account):
  choseAgent?: boolean
  triedCmdJ?: boolean
  shapedSidebar?: boolean

  // Per-server items — keyed by devServerId:
  perServer?: Record<string, PerServerChecklistState>
}

type PerServerChecklistState = {
  addedRepo?: boolean
  ranFirstAgent?: boolean
  ranSecondAgentOnSameTask?: boolean
  reviewedDiff?: boolean
  openedPr?: boolean
  addedFolder?: boolean
  openedFile?: boolean
  ranAgentOnFile?: boolean
}
```

### C.2 Persistence Migration

```typescript
// src/main/persistence.ts — trong normalizeLoadedOnboardingState():
function migrateOnboardingChecklist(onboarding: OnboardingState): OnboardingState {
  const cl = onboarding.checklist
  if (!cl || cl.perServer !== undefined) return onboarding  // đã migrate

  // v1 → v2: move flat items sang perServer['local']
  const perServerItems: PerServerChecklistState = {}
  const PER_SERVER_KEYS: (keyof PerServerChecklistState)[] = [
    'addedRepo', 'ranFirstAgent', 'ranSecondAgentOnSameTask',
    'reviewedDiff', 'openedPr', 'addedFolder', 'openedFile', 'ranAgentOnFile'
  ]
  for (const key of PER_SERVER_KEYS) {
    const val = (cl as Record<string, unknown>)[key]
    if (val === true) {
      perServerItems[key] = true
    }
  }

  return {
    ...onboarding,
    checklist: {
      choseAgent: cl.choseAgent,
      triedCmdJ: cl.triedCmdJ,
      shapedSidebar: cl.shapedSidebar,
      perServer: Object.keys(perServerItems).length > 0
        ? { local: perServerItems }
        : {}
    }
  }
}
```

### C.3 Checklist Update IPC

```typescript
// src/main/ipc/onboarding-ipc.ts (MODIFY — thêm):
ipc.handle('onboarding.markChecklistItem', async (_, params: {
  item: keyof OnboardingChecklistState | keyof PerServerChecklistState
  devServerId?: string   // undefined = global item
  value?: boolean        // default: true
}): Promise<void> => {
  const { item, devServerId, value = true } = params
  store.mutate(state => {
    const cl = state.onboarding.checklist ?? {}
    if (devServerId) {
      // Per-server item
      cl.perServer = cl.perServer ?? {}
      cl.perServer[devServerId] = cl.perServer[devServerId] ?? {}
      ;(cl.perServer[devServerId] as Record<string, unknown>)[item] = value
    } else {
      // Global item
      ;(cl as Record<string, unknown>)[item] = value
    }
    state.onboarding.checklist = cl
  })
})
```

### C.4 FeatureWallSetupStepId — Thêm 2 steps mới

```typescript
// src/shared/feature-wall-setup-steps.ts (MODIFY):
export type FeatureWallSetupStepId =
  | 'default-agent'
  | 'add-two-repos'
  | 'notifications'
  | 'two-worktrees'
  | 'browser'
  | 'task-sources'
  | 'agent-capabilities'
  | 'setup-script'
  | 'connect-dev-server'     // NEW
  | 'add-dev-server-repo'    // NEW

// Completion check:
export function isConnectDevServerComplete(devServers: DevServer[]): boolean {
  return devServers.some(ds => ds.status === 'connected')
}

export function isAddDevServerRepoComplete(
  repos: Repo[],
  activeDevServerId: string | null
): boolean {
  if (!activeDevServerId) return false
  return repos.some(r => r.devServerId === activeDevServerId)
}

// Priority: 'connect-dev-server' phải là step ĐẦU TIÊN nếu chưa có dev server
export function getFirstIncompleteFeatureWallSetupStepId(
  steps: Record<FeatureWallSetupStepId, boolean>,
  devServers: DevServer[],
  repos: Repo[]
): FeatureWallSetupStepId | null {
  // Override: nếu chưa có dev server → 'connect-dev-server' là ưu tiên cao nhất
  if (!isConnectDevServerComplete(devServers)) {
    return 'connect-dev-server'
  }
  // Existing logic for other steps...
  const ORDER: FeatureWallSetupStepId[] = [
    'connect-dev-server',
    'add-dev-server-repo',
    'default-agent',
    'agent-capabilities',
    'task-sources',
    'add-two-repos',
    'setup-script',
    'notifications',
    'two-worktrees',
    'browser'
  ]
  return ORDER.find(id => !steps[id]) ?? null
}
```

---

## Tests tổng hợp Phase 3

```typescript
// SOL-007 tests:
describe('onboarding.detectWindowsCapabilities', () => {
  it('dev server không phải Windows → throw Error')
  it('dev server Windows connected → forward đến relay')
  it('cache hit 60s → không gọi relay')
  it('pwshVersion được include trong response')
  it('gitBashPath được include khi available')
})

// SOL-008 tests:
describe('WebPushManager', () => {
  it('loadOrCreateVapidKeys() tạo keys mới nếu chưa có')
  it('loadOrCreateVapidKeys() reuse keys đã có')
  it('saveSubscription() deduplicate theo endpoint')
  it('sendToAll() gửi đến tất cả subscriptions')
  it('sendToAll() tự xóa subscription 410 Gone')
  it('getPublicKey() trả về VAPID public key')
})

describe('Push API Routes', () => {
  it('GET /api/vapid-public-key → 200 { publicKey }')
  it('POST /api/push-subscribe → 201 { id }')
  it('POST /api/push-subscribe → deduplicate endpoint')
  it('POST /api/push-unsubscribe → 204')
  it('Unknown route → không xử lý (pass-through)')
})

// SOL-009 tests:
describe('onboarding.markChecklistItem', () => {
  it('global item: choseAgent = true → không cần devServerId')
  it('per-server item: addedRepo với devServerId → lưu vào perServer[dsId]')
  it('per-server item: không có devServerId → throw Error')
})

describe('migrateOnboardingChecklist', () => {
  it('flat checklist (v1) → migrate sang perServer["local"]')
  it('checklist đã có perServer → không migrate lại')
  it('global items (choseAgent, triedCmdJ) giữ nguyên sau migrate')
})

describe('getFirstIncompleteFeatureWallSetupStepId', () => {
  it('không có dev server → "connect-dev-server" ưu tiên đầu')
  it('có dev server nhưng chưa add repo → "add-dev-server-repo"')
  it('tất cả done → null')
})
```

---

## Checklist triển khai tổng hợp Phase 3

**SOL-007:**
- [x] Relay: thêm `pwshVersion` + `gitBashPath` vào `detectWindowsTerminalCapabilities`
- [x] IPC `onboarding.detectWindowsCapabilities` với platform guard + cache (TTL 60s)
- [x] Schema `GlobalSettings.terminalWindowsConfigByServer`

**SOL-008:**
- [x] `npm install web-push` + types `@types/web-push`
- [x] Tạo `WebPushManager` với VAPID key lifecycle
- [x] Schema `webPushSubscriptions[]` + `vapidKeys` trong `PersistedState`
- [x] Tạo `push-api-routes.ts` với 3 endpoints
- [x] Tích hợp trong `server-bootstrap.ts` + `src/server/index.ts`
- [x] Trigger push khi agent task complete (`dispatchMobileNotification` hook)

**SOL-009:**
- [x] Schema `OnboardingChecklistState.perServer`
- [x] Persistence migration v1 → v2 (flat → perServer)
- [x] IPC `onboarding.markChecklistItem` với devServerId
- [x] `FeatureWallSetupStepId` thêm `connect-dev-server` + `add-dev-server-repo`
- [x] `getFirstIncompleteDevServerStepId` ưu tiên dev server steps
