# FE-SOL-D: Windows Terminal, Web Push Notifications & Multi-Server Checklist

**CRs:** [CR-OB-007](../../../../../docs/crs/v1/onboarding/CR-OB-007-windows-terminal-remote.md) | [CR-OB-008](../../../../../docs/crs/v1/onboarding/CR-OB-008-notification-server.md) | [CR-OB-009](../../../../../docs/crs/v1/onboarding/CR-OB-009-multi-devserver-checklist.md)  
**TDD refs:** TDD-FE-02 (State), TDD-FE-05 (Components), TDD-FE-06 (Web Client), TDD-FE-07 (Hooks)  
**Status:** ✅ COMPLETED (2026-07-23) | **Phase:** 3

---

## PART A — Windows Terminal Remote (CR-OB-007)

### A.1 New Hook — `useRemoteWindowsTerminalCapabilities.ts`

```typescript
// src/renderer/src/hooks/useRemoteWindowsTerminalCapabilities.ts
import { useState, useEffect } from 'react'

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

const DEFAULT: RemoteWindowsCapabilities = {
  wslAvailable: false,
  wslDistros: [],
  pwshAvailable: false,
  gitBashAvailable: false,
  loading: false,
  error: null
}

const capsCache = new Map<string, { caps: RemoteWindowsCapabilities; cachedAt: number }>()
const CACHE_TTL = 60_000

export function useRemoteWindowsTerminalCapabilities(
  devServerId: string | null,
  enabled: boolean
): RemoteWindowsCapabilities & { retry: () => void } {
  const cached = devServerId ? capsCache.get(devServerId) : undefined
  const isCacheValid = cached && Date.now() - cached.cachedAt < CACHE_TTL

  const [caps, setCaps] = useState<RemoteWindowsCapabilities>(
    isCacheValid ? cached.caps : DEFAULT
  )

  const fetch = async () => {
    if (!devServerId || !enabled) return
    setCaps(prev => ({ ...prev, loading: true, error: null }))
    try {
      const result = await window.api.onboarding.detectWindowsCapabilities({ devServerId })
      const next = { ...result, loading: false, error: null }
      capsCache.set(devServerId, { caps: next, cachedAt: Date.now() })
      setCaps(next)
    } catch (err) {
      setCaps(prev => ({ ...prev, loading: false, error: (err as Error).message }))
    }
  }

  useEffect(() => {
    if (!enabled || !devServerId) return
    if (isCacheValid) return  // use cached
    void fetch()
  }, [devServerId, enabled])

  return { ...caps, retry: fetch }
}
```

### A.2 WindowsTerminalStep — MODIFY

```tsx
// src/renderer/src/components/onboarding/WindowsTerminalStep.tsx (MODIFY)
import { useRemoteWindowsTerminalCapabilities } from '../../hooks/useRemoteWindowsTerminalCapabilities'

type WindowsTerminalStepProps = {
  settings: GlobalSettings | null
  updateSettings: (updates: Partial<GlobalSettings>) => Promise<void> | void
  activeDevServerId: string | null       // NEW (replaces local detection)
  activeDevServerPlatform: 'win32'       // Guaranteed — step chỉ render khi win32
}

export function WindowsTerminalStep({
  settings,
  updateSettings,
  activeDevServerId,
  activeDevServerPlatform: _platform
}: WindowsTerminalStepProps) {
  const { wslAvailable, wslDistros, pwshAvailable, pwshVersion, gitBashAvailable, gitBashPath,
          loading, error, retry } =
    useRemoteWindowsTerminalCapabilities(activeDevServerId, true)

  // Đọc per-server config:
  const serverConfig = activeDevServerId
    ? settings?.terminalWindowsConfigByServer?.[activeDevServerId]
    : undefined

  const handleShellChange = async (shell: string) => {
    if (!activeDevServerId) return
    await updateSettings({
      terminalWindowsConfigByServer: {
        ...settings?.terminalWindowsConfigByServer,
        [activeDevServerId]: { ...serverConfig, shell }
      }
    })
  }

  const handleWslDistroChange = async (distro: string) => {
    if (!activeDevServerId) return
    await updateSettings({
      terminalWindowsConfigByServer: {
        ...settings?.terminalWindowsConfigByServer,
        [activeDevServerId]: { ...serverConfig, wslDistro: distro }
      }
    })
  }

  if (loading) return <div className="step-loading"><Spinner />Detecting Windows capabilities…</div>

  if (error) {
    return (
      <div className="step-error">
        <p>Could not detect Windows capabilities: {error}</p>
        <Button onClick={retry}>Retry</Button>
      </div>
    )
  }

  return (
    <div className="onboarding-step windows-terminal-step">
      <h2>Windows Terminal Setup</h2>

      {/* Shell selector */}
      <div className="field">
        <label>Shell</label>
        <Select
          value={serverConfig?.shell ?? 'powershell.exe'}
          onValueChange={handleShellChange}
        >
          <SelectItem value="powershell.exe">
            Windows PowerShell
          </SelectItem>
          {pwshAvailable && (
            <SelectItem value="pwsh.exe">
              PowerShell {pwshVersion ?? '7+'}
            </SelectItem>
          )}
          {wslAvailable && (
            <SelectItem value="wsl.exe">
              WSL (Windows Subsystem for Linux)
            </SelectItem>
          )}
          {gitBashAvailable && (
            <SelectItem value={gitBashPath ?? 'git-bash'}>
              Git Bash
            </SelectItem>
          )}
        </Select>
      </div>

      {/* WSL Distro selector — chỉ khi chọn wsl.exe */}
      {serverConfig?.shell === 'wsl.exe' && wslDistros.length > 0 && (
        <div className="field">
          <label>WSL Distro</label>
          <Select
            value={serverConfig?.wslDistro ?? wslDistros[0]}
            onValueChange={handleWslDistroChange}
          >
            {wslDistros.map(d => (
              <SelectItem key={d} value={d}>{d}</SelectItem>
            ))}
          </Select>
        </div>
      )}

      {/* Right-click to paste */}
      <div className="field field--toggle">
        <label>Right-click to paste</label>
        <Switch
          id="right-click-paste"
          checked={serverConfig?.rightClickToPaste ?? true}
          onCheckedChange={v => {
            if (!activeDevServerId) return
            void updateSettings({
              terminalWindowsConfigByServer: {
                ...settings?.terminalWindowsConfigByServer,
                [activeDevServerId]: { ...serverConfig, rightClickToPaste: v }
              }
            })
          }}
        />
      </div>
    </div>
  )
}
```

---

## PART B — Web Push Notifications (CR-OB-008)

### B.1 New Hook — `useBrowserNotificationPermission.ts`

```typescript
// src/renderer/src/hooks/useBrowserNotificationPermission.ts
import { useState, useCallback } from 'react'

type NotificationPermissionState = 'default' | 'granted' | 'denied' | 'unsupported'

export function useBrowserNotificationPermission(): {
  state: NotificationPermissionState
  requestPermission: () => Promise<void>
} {
  const [state, setState] = useState<NotificationPermissionState>(() => {
    if (typeof window === 'undefined' || !('Notification' in window)) return 'unsupported'
    return Notification.permission as NotificationPermissionState
  })

  const requestPermission = useCallback(async () => {
    if (state === 'unsupported') return
    const result = await Notification.requestPermission()
    setState(result as NotificationPermissionState)
  }, [state])

  return { state, requestPermission }
}
```

### B.2 New Hook — `useWebPushSubscription.ts`

```typescript
// src/renderer/src/hooks/useWebPushSubscription.ts
import { useState, useCallback } from 'react'

type PushSubscriptionState = 'idle' | 'subscribing' | 'subscribed' | 'failed'

export function useWebPushSubscription(): {
  state: PushSubscriptionState
  subscribe: () => Promise<void>
  unsubscribe: () => Promise<void>
  isSupported: boolean
} {
  const isSupported = 'serviceWorker' in navigator && 'PushManager' in window
  const [state, setState] = useState<PushSubscriptionState>(() => {
    // Check initial state asynchronously after mount
    return 'idle'
  })

  // Check existing subscription on mount:
  useEffect(() => {
    if (!isSupported) return
    navigator.serviceWorker.ready.then(reg => reg.pushManager.getSubscription()).then(sub => {
      if (sub) setState('subscribed')
    })
  }, [isSupported])

  const subscribe = useCallback(async () => {
    if (!isSupported) return
    setState('subscribing')
    try {
      // Fetch VAPID public key from Orca server:
      const { publicKey } = await fetch('/api/vapid-public-key').then(r => r.json())

      const reg = await navigator.serviceWorker.ready
      const subscription = await reg.pushManager.subscribe({
        userVisibleOnly: true,
        applicationServerKey: urlBase64ToUint8Array(publicKey)
      })

      // Register on server:
      await fetch('/api/push-subscribe', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ subscription: subscription.toJSON() })
      })

      setState('subscribed')
    } catch (err) {
      setState('failed')
    }
  }, [isSupported])

  const unsubscribe = useCallback(async () => {
    if (!isSupported) return
    const reg = await navigator.serviceWorker.ready
    const sub = await reg.pushManager.getSubscription()
    if (sub) {
      await fetch('/api/push-unsubscribe', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ endpoint: sub.endpoint })
      })
      await sub.unsubscribe()
    }
    setState('idle')
  }, [isSupported])

  return { state, subscribe, unsubscribe, isSupported }
}

// Utility: VAPID key → Uint8Array
function urlBase64ToUint8Array(base64String: string): Uint8Array {
  const padding = '='.repeat((4 - base64String.length % 4) % 4)
  const base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/')
  const rawData = atob(base64)
  return Uint8Array.from(rawData.split(''), c => c.charCodeAt(0))
}
```

### B.3 NotificationStep — MODIFY

```tsx
// src/renderer/src/components/onboarding/NotificationStep.tsx (MODIFY)
import { useBrowserNotificationPermission } from '../../hooks/useBrowserNotificationPermission'
import { useWebPushSubscription } from '../../hooks/useWebPushSubscription'

const isWebMode = import.meta.env.ORCA_PLATFORM === 'web'

export function NotificationStep({ settings, updateSettings }: NotificationStepProps) {
  // Web mode: dùng Web APIs
  const browserNotif = useBrowserNotificationPermission()
  const webPush = useWebPushSubscription()

  if (isWebMode) {
    return <WebModeNotificationStep
      browserNotif={browserNotif}
      webPush={webPush}
      onTestNotification={handleTestNotification}
    />
  }

  // Electron mode: existing implementation
  return <ElectronModeNotificationStep settings={settings} updateSettings={updateSettings} />
}

function WebModeNotificationStep({ browserNotif, webPush, onTestNotification }) {
  return (
    <div className="onboarding-step notification-step notification-step--web">
      <h2>Stay informed</h2>

      {/* Browser notifications */}
      <div className="notification-section">
        <h3>Browser Notifications</h3>
        <p>Get notified when you're on this page</p>
        {browserNotif.state === 'unsupported' && (
          <p className="hint">Browser notifications not supported in this browser</p>
        )}
        {browserNotif.state === 'default' && (
          <Button
            id="enable-browser-notif-btn"
            onClick={browserNotif.requestPermission}
          >
            Enable Browser Notifications
          </Button>
        )}
        {browserNotif.state === 'granted' && (
          <div className="status-ok">
            <span>✓ Browser notifications enabled</span>
          </div>
        )}
        {browserNotif.state === 'denied' && (
          <div className="status-blocked">
            <span>⚠ Notifications blocked</span>
            <span className="hint">Open browser Settings → Site permissions → Allow notifications</span>
          </div>
        )}
      </div>

      {/* Push notifications */}
      {browserNotif.state === 'granted' && webPush.isSupported && (
        <div className="notification-section">
          <h3>Push Notifications</h3>
          <p>Get notified even when the tab is closed</p>
          {webPush.state === 'idle' && (
            <Button
              id="subscribe-push-btn"
              variant="secondary"
              onClick={webPush.subscribe}
            >
              Subscribe to Push Notifications
            </Button>
          )}
          {webPush.state === 'subscribing' && <Spinner />}
          {webPush.state === 'subscribed' && (
            <div className="status-ok">
              <span>✓ Push notifications enabled</span>
              <Button variant="ghost" size="xs" onClick={webPush.unsubscribe}>Disable</Button>
            </div>
          )}
          {webPush.state === 'failed' && (
            <p className="error">Failed to subscribe — please try again</p>
          )}
        </div>
      )}

      {/* Additional channels notice */}
      <div className="notification-section notification-section--secondary">
        <h3>Other Channels (optional)</h3>
        <p>Set up email or Slack notifications in <strong>Settings → Notifications</strong> after onboarding.</p>
      </div>

      <Button id="test-notification-btn" variant="secondary" onClick={onTestNotification}>
        Send Test Notification
      </Button>
    </div>
  )
}
```

### B.4 Service Worker Registration

```typescript
// src/renderer/src/web/main-web-bootstrap.tsx (MODIFY — thêm SW registration)
export async function bootstrapWebApp() {
  // ...existing bootstrap...

  // Register service worker for push notifications (web mode only):
  if ('serviceWorker' in navigator) {
    try {
      await navigator.serviceWorker.register('/service-worker.js')
      console.log('[Web] Service Worker registered')
    } catch {
      console.warn('[Web] Service Worker registration failed (non-fatal)')
    }
  }

  // ...mount React app...
}
```

---

## PART C — Multi-Server Checklist (CR-OB-009)

### C.1 Checklist Slice (MODIFY)

```typescript
// src/renderer/src/store/slices/onboarding.ts (MODIFY)
import type { OnboardingChecklistState, PerServerChecklistState } from '../../../../shared/types'

type OnboardingSlice = {
  // ...existing
  checklistState: OnboardingChecklistState

  // Actions:
  markGlobalChecklistItem: (item: keyof OnboardingChecklistState, value?: boolean) => void
  markServerChecklistItem: (
    devServerId: string,
    item: keyof PerServerChecklistState,
    value?: boolean
  ) => void
}

export const createOnboardingSlice = (set: SetState<AppState>) => ({
  // ...existing
  checklistState: {},

  markGlobalChecklistItem: (item, value = true) => {
    set(state => ({
      checklistState: { ...state.checklistState, [item]: value }
    }))
    // Persist:
    void window.api.onboarding.markChecklistItem({ item, value })
  },

  markServerChecklistItem: (devServerId, item, value = true) => {
    set(state => ({
      checklistState: {
        ...state.checklistState,
        perServer: {
          ...state.checklistState.perServer,
          [devServerId]: {
            ...(state.checklistState.perServer?.[devServerId] ?? {}),
            [item]: value
          }
        }
      }
    }))
    void window.api.onboarding.markChecklistItem({ item, devServerId, value })
  }
})

// Selector:
export function useServerChecklist(devServerId: string | null) {
  return useAppStore(
    useShallow(s => devServerId ? s.checklistState.perServer?.[devServerId] ?? {} : {})
  )
}
```

### C.2 SetupGuideModal — Grouped per-server (MODIFY)

```tsx
// src/renderer/src/components/setup-guide/SetupGuideModal.tsx (MODIFY)
import { useDevServers, useActiveDevServer } from '../../store/slices/dev-servers'
import { useServerChecklist } from '../../store/slices/onboarding'

export function SetupGuideModal() {
  const devServers = useDevServers()
  const checklist = useAppStore(s => s.checklistState)
  const connectedServers = devServers.filter(ds => ds.status === 'connected')

  return (
    <div className="setup-guide">
      {/* Global section */}
      <SetupSection title="General">
        <ChecklistItem
          id="choose-agent"
          done={!!checklist.choseAgent}
          label="Choose your default agent"
        />
        <ChecklistItem
          id="notifications"
          done={isNotificationsComplete(checklist)}
          label="Turn on notifications"
        />
        <ChecklistItem
          id="two-worktrees"
          done={isFeatureWallStepDone('two-worktrees')}
          label="Run two agents in parallel"
        />
        <ChecklistItem
          id="browser"
          done={isFeatureWallStepDone('browser')}
          label="Use Orca's browser"
        />
      </SetupSection>

      {/* Per-server sections */}
      {connectedServers.map(ds => (
        <ServerChecklistSection
          key={ds.id}
          devServer={ds}
          checklist={checklist.perServer?.[ds.id] ?? {}}
        />
      ))}

      {/* Feature Wall steps (cross-server) */}
      <SetupSection title="Setup">
        <ChecklistItem
          id="connect-dev-server"
          done={connectedServers.length > 0}
          label="Connect a dev server"
        />
        {connectedServers.map(ds => (
          <ChecklistItem
            key={`cli-${ds.id}`}
            id={`orca-cli-${ds.id}`}
            done={isFeatureWallStepDone(`agent-capabilities-${ds.id}`)}
            label={`Enable Orca CLI — ${ds.name}`}
            action={
              !isFeatureWallStepDone(`agent-capabilities-${ds.id}`) && (
                <InstallCliButton devServerId={ds.id} />
              )
            }
          />
        ))}
        <ChecklistItem
          id="setup-script"
          done={isFeatureWallStepDone('setup-script')}
          label="Automate workspace setup"
        />
      </SetupSection>

      {/* Progress bar */}
      <OverallProgressBar
        checklist={checklist}
        devServers={connectedServers}
      />
    </div>
  )
}

// Per-server section component:
function ServerChecklistSection({ devServer, checklist }: {
  devServer: DevServer
  checklist: PerServerChecklistState
}) {
  return (
    <SetupSection
      title={`${devServer.name} (${devServer.platform ?? 'Unknown OS'})`}
      headerRight={<DevServerStatusBadge status={devServer.status} platform={devServer.platform} />}
    >
      <ChecklistItem
        id={`add-repo-${devServer.id}`}
        done={!!checklist.addedRepo}
        label="Add a repository"
        action={!checklist.addedRepo && (
          <Button size="xs" onClick={() => {/* open add repo for this server */}}>
            Add repo
          </Button>
        )}
      />
      <ChecklistItem
        id={`first-agent-${devServer.id}`}
        done={!!checklist.ranFirstAgent}
        label="Run your first agent"
      />
      <ChecklistItem
        id={`review-diff-${devServer.id}`}
        done={!!checklist.reviewedDiff}
        label="Review a diff"
      />
      <ChecklistItem
        id={`open-pr-${devServer.id}`}
        done={!!checklist.openedPr}
        label="Open a Pull Request"
      />
    </SetupSection>
  )
}

// Progress bar component:
function OverallProgressBar({ checklist, devServers }: {
  checklist: OnboardingChecklistState
  devServers: DevServer[]
}) {
  const total = useMemo(() => {
    let done = 0
    let total = 0
    // Global items:
    const globalItems: (keyof OnboardingChecklistState)[] = ['choseAgent', 'triedCmdJ', 'shapedSidebar']
    globalItems.forEach(key => {
      total++
      if (checklist[key]) done++
    })
    // Per-server items:
    const serverItems: (keyof PerServerChecklistState)[] = [
      'addedRepo', 'ranFirstAgent', 'reviewedDiff', 'openedPr'
    ]
    devServers.forEach(ds => {
      serverItems.forEach(key => {
        total++
        if (checklist.perServer?.[ds.id]?.[key]) done++
      })
    })
    return { done, total }
  }, [checklist, devServers])

  const pct = total.total === 0 ? 0 : Math.round((total.done / total.total) * 100)
  return (
    <div className="overall-progress">
      <Progress value={pct} />
      <span>{total.done}/{total.total} complete</span>
    </div>
  )
}
```

### C.3 Trigger checklist items từ existing actions

```typescript
// Trong các component/hooks liên quan, thêm markServerChecklistItem calls:

// Khi repo được add thành công:
// src/renderer/src/components/onboarding/AddRepoStep.tsx
const onRepoAdded = (repoId: string) => {
  const activeDevServerId = useAppStore.getState().activeDevServerId
  if (activeDevServerId) {
    useAppStore.getState().markServerChecklistItem(activeDevServerId, 'addedRepo')
  }
}

// Khi agent chạy lần đầu:
// src/renderer/src/hooks/useIpcEvents.ts (MODIFY)
window.api.onAgentStatusUpdate((event) => {
  store.updateAgentStatus(event)
  // NEW: mark checklist nếu agent started:
  if (event.status === 'running') {
    const devServerId = getDevServerForWorktree(event.worktreeId)
    if (devServerId && !store.checklistState.perServer?.[devServerId]?.ranFirstAgent) {
      store.markServerChecklistItem(devServerId, 'ranFirstAgent')
    }
  }
})
```

---

## Tests (Phase 3)

```tsx
// CR-OB-007:
describe('useRemoteWindowsTerminalCapabilities', () => {
  it('devServerId = null → DEFAULT state, không gọi API')
  it('gọi api.onboarding.detectWindowsCapabilities khi mount')
  it('cache hit (< 60s) → không gọi API lần 2')
  it('loading = true khi đang fetch')
  it('error state + retry function khi fetch fail')
})

describe('WindowsTerminalStep', () => {
  it('hiển thị PowerShell luôn luôn')
  it('hiển thị PowerShell 7+ khi pwshAvailable = true, với version')
  it('hiển thị WSL khi wslAvailable = true')
  it('ẩn WSL khi wslAvailable = false')
  it('WSL Distro select hiển thị khi shell = wsl.exe')
  it('handleShellChange lưu vào terminalWindowsConfigByServer[devServerId]')
})

// CR-OB-008:
describe('useBrowserNotificationPermission', () => {
  it('state = "unsupported" khi Notification không có trong window')
  it('state = Notification.permission khi init')
  it('requestPermission() gọi Notification.requestPermission()')
  it('state cập nhật sau khi requestPermission')
})

describe('useWebPushSubscription', () => {
  it('isSupported = false khi serviceWorker hoặc PushManager không có')
  it('subscribe() fetch VAPID key, tạo subscription, POST /api/push-subscribe')
  it('state = subscribed sau khi subscribe thành công')
  it('unsubscribe() DELETE subscription, POST /api/push-unsubscribe')
})

describe('NotificationStep (web mode)', () => {
  it('render BrowserNotification section')
  it('Enable button hiển thị khi permission = "default"')
  it('✓ message hiển thị khi permission = "granted"')
  it('Warning hiển thị khi permission = "denied"')
  it('Push section hiển thị khi browser permission granted + isSupported = true')
  it('Test Notification button gọi handleTestNotification')
})

// CR-OB-009:
describe('markServerChecklistItem', () => {
  it('cập nhật state.checklistState.perServer[dsId][item]')
  it('gọi api.onboarding.markChecklistItem với devServerId')
})

describe('SetupGuideModal — multi-server', () => {
  it('render Global section với global items')
  it('render 1 ServerChecklistSection per connected server')
  it('ServerChecklistSection hiển thị items cho đúng server')
  it('OverallProgressBar tính đúng done/total')
  it('connect-dev-server item done khi có ít nhất 1 connected server')
})
```

---

## Checklist triển khai

**CR-OB-007:**
- [x] Tạo `useRemoteWindowsTerminalCapabilities` hook với cache
- [ ] Sửa `WindowsTerminalStep.tsx`: nhận `activeDevServerId`, dùng remote hook (UI deferred)
- [ ] Shell options filter theo remote capabilities (UI deferred)
- [ ] WSL distro list từ remote (UI deferred)
- [ ] Settings lưu per-server `terminalWindowsConfigByServer` (UI deferred)
- [x] Extend `window.api.onboarding.detectWindowsCapabilities` (preload bridge)

**CR-OB-008:**
- [x] Tạo `useBrowserNotificationPermission` hook
- [x] Tạo `useWebPushSubscription` hook
- [ ] Sửa `NotificationStep.tsx`: detect web mode, render `WebModeNotificationStep` (UI deferred)
- [x] Service Worker registration trong `bootstrapWebApp()` (`main-web-bootstrap.tsx`)
- [x] Tạo `service-worker.js` (push handler + navigation)

**CR-OB-009:**
- [x] Tạo `onboarding-checklist` slice: `perServer`, `markServerChecklistItem`, `markGlobalChecklistItem`
- [x] Sửa `SetupGuideModal.tsx`: grouped UI per dev server (`PerServerChecklistPanel`, `OverallProgressBar`)
- [x] `ServerChecklistSection` component
- [x] `OverallProgressBar` component  
- [x] Trigger checklist marks từ agent events (`useIpcEvents.ts`) + repo add (`AddRepoStep.tsx`)
- [x] Unit tests: `onboarding-checklist.test.ts` (7 tests)
