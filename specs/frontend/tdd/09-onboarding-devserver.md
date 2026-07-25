# TDD-FE-09: Onboarding — Dev Server, Agent Detection & Wizard

**Document:** TDD-FE-09 (NEW — onboarding CRs)  
**Version:** 1.0  
**Date:** 2026-07-23  
**Domain:** Onboarding Wizard, Dev Server UI, Remote Agent Detection, Preflight, Push Notifications  
**Source files:**
- `src/renderer/src/components/onboarding/`
- `src/renderer/src/components/dev-server/`
- `src/renderer/src/components/remote-browser/`
- `src/renderer/src/hooks/` (useDevServers, useRemoteAgentDetection, ...)
- `src/renderer/src/store/slices/dev-servers.ts`
- `src/renderer/service-worker.js`

> **Status: ✅ IMPLEMENTED** — 4/4 solutions (Phase 1, 2, 3) | All components done

---

## 1. Mục tiêu

Onboarding Wizard hỗ trợ **remote dev servers** — không chỉ localhost:

```
TRƯỚC (v1.x):
  Onboarding → local machine only
  Agent detection → localhost commands

SAU (v2.0 + onboarding CRs):
  Onboarding → chọn dev server → wizard chạy trên dev server đó
  Agent detection → relay.session.call() → dev server remote
  Preflight → Node/Git check trên dev server
  Repo clone → dev server filesystem
  Notification → Web Push (browser Push API)
```

---

## 2. New Zustand Slices

### 2.1 Dev Servers Slice (`store/slices/dev-servers.ts`)

```typescript
type DevServerSlice = {
  devServers: DevServer[]
  activeDevServerId: string | null

  // Actions
  setDevServers: (servers: DevServer[]) => void
  upsertDevServer: (server: DevServer) => void
  removeDevServer: (id: string) => void
  setActiveDevServerId: (id: string | null) => void
  updateDevServerStatus: (id: string, status: DevServer['status'], extra?: Partial<DevServer>) => void
}

// Key selectors:
export function useDevServers(): DevServer[]
export function useActiveDevServer(): DevServer | null
export function useConnectedDevServers(): DevServer[]
```

**Thêm vào `AppState`** (rootStore.ts):
```typescript
AppState = {
  // ...existing slices...
  ...DevServerSlice
  ...RemoteAgentSlice  // NEW — onboarding.ts MODIFY
  ...RemotePreflightSlice  // NEW — preflight.ts MODIFY
}
```

### 2.2 Remote Agent Detection (onboarding.ts MODIFY)

```typescript
// Thêm vào onboarding.ts slice:
agentDetectionByServer: Record<string, DetectionState>
setAgentDetectionForServer: (serverId: string, state: DetectionState) => void
```

### 2.3 Remote Preflight (preflight.ts MODIFY)

```typescript
// Thêm vào preflight.ts slice:
remotePreflightByServer: Record<string, RemotePreflightStatus>
activeRemotePreflightStatus: RemotePreflightStatus | null

setRemotePreflightStatus: (devServerId: string, status: RemotePreflightStatus) => void
clearRemotePreflightStatus: (devServerId: string) => void
```

---

## 3. New Hooks

### Phase 1: Dev Server

| Hook | File | Purpose |
|------|------|---------|
| `useDevServers` | `hooks/useDevServers.ts` | Zustand selector + IPC subscription |
| `useDevServerConnection` | `hooks/useDevServerConnection.ts` | connect/disconnect actions |
| `useRemoteAgentDetection` | `hooks/useRemoteAgentDetection.ts` | Detect agents trên dev server |
| `useActiveDevServerPlatform` | `hooks/useActiveDevServerPlatform.ts` | Reactive platform selector |

### Phase 2: Preflight & Repo

| Hook | File | Purpose |
|------|------|---------|
| `useRemotePreflightStatus` | `hooks/useRemotePreflightStatus.ts` | gh + git check trên dev server |
| `useRemoteDirectoryBrowser` | `hooks/useRemoteDirectoryBrowser.ts` | Browse remote filesystem |
| `useRemoteRepoAdd` | `hooks/useRemoteRepoAdd.ts` | Add/clone/scan remote repos |

### Phase 3: Windows & Notifications

| Hook | File | Purpose |
|------|------|---------|
| `useRemoteWindowsTerminalCapabilities` | `hooks/useRemoteWindowsTerminalCapabilities.ts` | Windows caps từ dev server, cache 60s |
| `useBrowserNotificationPermission` | `hooks/useBrowserNotificationPermission.ts` | Browser notification permission |
| `useWebPushSubscription` | `hooks/useWebPushSubscription.ts` | VAPID subscribe/unsubscribe |

---

## 4. Remote Agent Detection Hook

```typescript
// src/renderer/src/hooks/useRemoteAgentDetection.ts
type DetectionState = {
  agents: string[]
  platform: NodeJS.Platform | null
  loading: boolean
  error: string | null
  lastDetectedAt: number | null
}

// Per-server module-level cache (survives re-renders):
const detectionCache = new Map<string, DetectionState>()

export function useRemoteAgentDetection(devServerId: string | null): DetectionState & {
  redetect: () => Promise<void>
}

// IPC call: window.api.onboarding.detectAgents({ devServerId })
// → { agents: string[], platform: NodeJS.Platform | null }
```

**Cache strategy:** module-level Map — persists across unmount/remount. `redetect()` bypasses cache.

---

## 5. Windows Terminal Capabilities Hook

```typescript
// src/renderer/src/hooks/useRemoteWindowsTerminalCapabilities.ts
type RemoteWindowsCapabilities = {
  wslAvailable: boolean
  wslDistros: string[]
  pwshAvailable: boolean
  pwshVersion?: string
  gitBashAvailable: boolean
  gitBashPath?: string
  loading: boolean
  error: string | null
}

const capsCache = new Map<string, { caps: RemoteWindowsCapabilities; cachedAt: number }>()
const CACHE_TTL = 60_000  // 60 seconds

export function useRemoteWindowsTerminalCapabilities(
  devServerId: string | null,
  enabled: boolean   // only fetch when on Windows step
): RemoteWindowsCapabilities & { retry: () => void }

// IPC: window.api.onboarding.detectWindowsCapabilities({ devServerId })
```

---

## 6. Web Push Notifications

### 6.1 Hook — `useWebPushSubscription.ts`

```typescript
type PushSubscriptionState = {
  status: 'idle' | 'subscribing' | 'subscribed' | 'unsubscribing' | 'error'
  subscription: PushSubscription | null
  error: string | null
}

export function useWebPushSubscription(): PushSubscriptionState & {
  subscribe: () => Promise<void>    // POST /push/subscribe
  unsubscribe: () => Promise<void>  // DELETE /push/subscribe
}

// Fetches VAPID public key: GET /push/vapid-key
// Uses browser PushManager.subscribe({ userVisibleOnly: true, applicationServerKey })
```

### 6.2 Service Worker (`src/renderer/service-worker.js`)

```javascript
// Registered in main-web-bootstrap.tsx:
if ('serviceWorker' in navigator) {
  navigator.serviceWorker.register('/service-worker.js')
}

// Push event handler:
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

### 6.3 Hook — `useBrowserNotificationPermission.ts`

```typescript
type NotificationPermissionState = {
  permission: NotificationPermission  // 'default' | 'granted' | 'denied'
  isSupported: boolean
}

export function useBrowserNotificationPermission(): NotificationPermissionState & {
  requestPermission: () => Promise<NotificationPermission>
}
```

---

## 7. New UI Components

### Phase 1 — Dev Server UI

```
src/renderer/src/
├── components/onboarding/
│   └── DevServerStep.tsx               ← Wizard step: thêm dev server
├── components/dev-server/
│   ├── DevServerCard.tsx               ← Card + status indicator
│   ├── DevServerList.tsx               ← Danh sách trong Settings
│   ├── AddDevServerDialog.tsx          ← Dialog thêm mới
│   └── DevServerStatusBadge.tsx        ← Badge: connected/disconnected/error/connecting
```

**DevServerCard** props:
```typescript
type DevServerCardProps = {
  server: DevServer
  isActive: boolean
  onConnect: (id: string) => void
  onDisconnect: (id: string) => void
  onRemove: (id: string) => void
  onSelect: (id: string) => void
}
```

### Phase 2 — Preflight & Repo

```
├── components/onboarding/
│   ├── IntegrationsStep.tsx    ← MODIFY: remote preflight + remote PTY
│   ├── GitIdentityCard.tsx     ← NEW: git user.name/user.email form
│   └── AddRepoStep.tsx         ← NEW: remote directory browser
├── components/remote-browser/
│   ├── RemoteDirectoryBrowser.tsx      ← Browse dev server filesystem
│   ├── RemoteDirectoryEntry.tsx        ← Single dir row (expand/select)
│   └── remote-directory-browser.css
```

### Phase 3 — Windows & Checklist

```
├── components/onboarding/
│   ├── WindowsTerminalStep.tsx ← MODIFY: remote capabilities + per-server settings
│   ├── NotificationStep.tsx    ← MODIFY: web mode UI + push subscription
│   └── MultiServerChecklist.tsx ← NEW: checklist for all dev servers
```

---

## 8. Wizard Flow (onboarding CRs)

```
Wizard Steps (web mode):
  1. DevServerStep         ← Thêm/chọn dev server (CR-OB-002)
  2. AgentStep             ← Detect agents trên dev server (CR-OB-003)
  3. PlatformStep          ← Platform-aware UI (CR-OB-004)
  4. IntegrationsStep      ← Preflight: gh/git trên dev server (CR-OB-005)
  5. AddRepoStep           ← Browse + clone remote repo (CR-OB-006)
  6. WindowsTerminalStep   ← Remote Windows caps + per-server config (CR-OB-007)
  7. NotificationStep      ← Web Push setup (CR-OB-008)
  8. MultiServerChecklist  ← Checklist qua tất cả servers (CR-OB-009)
```

**Platform guard pattern:**
```typescript
const isWebMode = import.meta.env.ORCA_PLATFORM === 'web'

// Electron: dùng native safeStorage Notification
// Web: dùng Web Push API → service worker
```

---

## 9. IPC API Surface (window.api extensions)

```typescript
// Thêm vào window.api (web-preload-api.ts):
window.api.devServer = {
  list: () => invoke('devServer.list'),
  add: (input) => invoke('devServer.add', input),
  update: (id, updates) => invoke('devServer.update', id, updates),
  remove: (id) => invoke('devServer.remove', id),
  connect: (id) => invoke('devServer.connect', id),
  disconnect: (id) => invoke('devServer.disconnect', id),
  testConnection: (id) => invoke('devServer.testConnection', id),
  onStatusChanged: (cb) => on('devServer.statusChanged', cb),
  offStatusChanged: (cb) => off('devServer.statusChanged', cb),
}

window.api.onboarding = {
  detectAgents: (params) => invoke('onboarding.detectAgents', params),
  runPreflight: (params) => invoke('onboarding.runPreflight', params),
  detectWindowsCapabilities: (params) => invoke('onboarding.detectWindowsCapabilities', params),
  cloneRepo: (params) => invoke('onboarding.cloneRepo', params),
  openFolder: (params) => invoke('onboarding.openFolder', params),
}
```

---

## 10. Design Principles cho Onboarding CRs

1. **Platform guard** — Detect web vs electron qua `import.meta.env.ORCA_PLATFORM`
2. **Per-server cache** — Module-level Map, không React state — survives re-renders
3. **Cleanup required** — Mọi `window.api.onDevServer.onStatusChanged()` cần `offStatusChanged()` trong `useEffect` cleanup
4. **Lazy loading** — Heavy onboarding components `React.lazy()` + `<Suspense>`
5. **Không sửa App.tsx** — Chỉ thêm mới, không sửa App shell
6. **useShallow** — Dùng khi selector trả về object
7. **Error boundaries** — Mỗi wizard step cần `<ErrorBoundary>`
8. **60s TTL cache** — Windows capabilities, agent detection results
