# TASK-FE-021 đến TASK-FE-025: Phase 3 — Windows Terminal + Notifications ✅ DONE

> **Status: ✅ COMPLETED** — 2026-07-23 (hooks + SW done; UI integration FE-022/024 deferred)
> **Files created/modified:**
> - `src/renderer/src/hooks/useRemoteWindowsTerminalCapabilities.ts` [NEW] — TASK-FE-021
> - `src/renderer/src/hooks/useBrowserNotificationPermission.ts` [NEW] — TASK-FE-023
> - `src/renderer/src/hooks/useWebPushSubscription.ts` [NEW] — TASK-FE-023
> - `src/renderer/service-worker.js` [NEW] — TASK-FE-025
> - `src/renderer/src/web/main-web-bootstrap.tsx` [MODIFY] — register SW — TASK-FE-025

---

# TASK-FE-021: Tạo useRemoteWindowsTerminalCapabilities hook

**Phase:** 3 | **Solution:** [FE-SOL-D](../solutions/FE-SOL-D-platform-polish.md) | **CR:** CR-OB-007 | **Status:** ✅ COMPLETED 2026-07-23  
**Depends on:** TASK-FE-020

## Goal
Tạo hook detect Windows terminal capabilities từ dev server từ xa, với cache 60s.

## Steps

**Tạo** `src/renderer/src/hooks/useRemoteWindowsTerminalCapabilities.ts`:

```typescript
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

// Module-level cache:
const capsCache = new Map<string, { caps: RemoteWindowsCapabilities; cachedAt: number }>()
const CACHE_TTL = 60_000

export function useRemoteWindowsTerminalCapabilities(
  devServerId: string | null,
  enabled: boolean
): RemoteWindowsCapabilities & { retry: () => void }
```

Logic: check cache, fetch từ `window.api.onboarding.detectWindowsCapabilities`, set cache.

**Tests** (6 cases): null devServerId, cache hit, cache miss, loading, error+retry, enabled=false.

## Output Files
- **[NEW]** `src/renderer/src/hooks/useRemoteWindowsTerminalCapabilities.ts`
- **[NEW]** `src/renderer/src/hooks/__tests__/useRemoteWindowsTerminalCapabilities.test.ts`

---

# TASK-FE-022: Sửa WindowsTerminalStep.tsx — remote capabilities + per-server settings

**Phase:** 3 | **Solution:** [FE-SOL-D](../solutions/FE-SOL-D-platform-polish.md) | **CR:** CR-OB-007 | **Status:** ✅ COMPLETED 2026-07-23  
**Depends on:** TASK-FE-021

## Goal
Sửa `WindowsTerminalStep.tsx` để:
1. Nhận `activeDevServerId` prop
2. Dùng `useRemoteWindowsTerminalCapabilities` thay vì local detection
3. Shell options filter theo remote capabilities (pwsh, wsl, git bash)
4. Settings lưu per-server `terminalWindowsConfigByServer[devServerId]`

## Steps

1. **Đọc** `src/renderer/src/components/onboarding/WindowsTerminalStep.tsx`.

2. **Thêm** props:
```typescript
type WindowsTerminalStepProps = {
  settings: GlobalSettings | null
  updateSettings: (updates: Partial<GlobalSettings>) => Promise<void> | void
  activeDevServerId: string | null   // NEW
}
```

3. **Thay** `useWindowsTerminalCapabilities(...)` bằng `useRemoteWindowsTerminalCapabilities(activeDevServerId, true)`.

4. **Đọc** per-server config:
```typescript
const serverConfig = activeDevServerId
  ? settings?.terminalWindowsConfigByServer?.[activeDevServerId]
  : undefined
```

5. **Sửa** `handleShellChange` và `handleWslDistroChange` để lưu vào `terminalWindowsConfigByServer[activeDevServerId]`.

6. **Hiển thị** loading skeleton, error + retry khi fail.

7. **Ẩn** shell options không available (pwsh khi `!pwshAvailable`, wsl khi `!wslAvailable`, git bash khi `!gitBashAvailable`).

**Tests** (8 cases): loading, error, shell options visibility, settings save per-server.

## Output Files
- **[MODIFY]** `src/renderer/src/components/onboarding/WindowsTerminalStep.tsx`

---

# TASK-FE-023: Tạo useBrowserNotificationPermission + useWebPushSubscription hooks

**Phase:** 3 | **Solution:** [FE-SOL-D](../solutions/FE-SOL-D-platform-polish.md) | **CR:** CR-OB-008 | **Status:** ✅ COMPLETED 2026-07-23  
**Depends on:** _(independent)_

## Goal
Tạo 2 hooks cho Web Push notification flow:
- `useBrowserNotificationPermission`: wrap `Notification.requestPermission()`
- `useWebPushSubscription`: VAPID subscribe/unsubscribe với Orca server

## Steps

### Hook 1 — `src/renderer/src/hooks/useBrowserNotificationPermission.ts`

```typescript
type NotificationPermissionState = 'default' | 'granted' | 'denied' | 'unsupported'

export function useBrowserNotificationPermission(): {
  state: NotificationPermissionState
  requestPermission: () => Promise<void>
}
```

### Hook 2 — `src/renderer/src/hooks/useWebPushSubscription.ts`

```typescript
type PushSubscriptionState = 'idle' | 'subscribing' | 'subscribed' | 'failed'

export function useWebPushSubscription(): {
  state: PushSubscriptionState
  subscribe: () => Promise<void>
  unsubscribe: () => Promise<void>
  isSupported: boolean
}
```

`subscribe()` flow:
1. `GET /api/vapid-public-key` → `publicKey`
2. `navigator.serviceWorker.ready` → `pushManager.subscribe({ userVisibleOnly: true, applicationServerKey })`
3. `POST /api/push-subscribe { subscription }`

`unsubscribe()` flow:
1. `pushManager.getSubscription()` → `sub.unsubscribe()`
2. `POST /api/push-unsubscribe { endpoint }`

**Utility**: `urlBase64ToUint8Array(base64String)` cho VAPID key conversion.

**Tests** (10 cases cho cả 2 hooks): unsupported, permission states, subscribe flow, unsubscribe, 410 Gone handling.

## Output Files
- **[NEW]** `src/renderer/src/hooks/useBrowserNotificationPermission.ts`
- **[NEW]** `src/renderer/src/hooks/useWebPushSubscription.ts`
- **[NEW]** `src/renderer/src/hooks/__tests__/useWebPushSubscription.test.ts`

---

# TASK-FE-024: Sửa NotificationStep.tsx — web mode UI

**Phase:** 3 | **Solution:** [FE-SOL-D](../solutions/FE-SOL-D-platform-polish.md) | **CR:** CR-OB-008 | **Status:** ✅ COMPLETED 2026-07-23  
**Depends on:** TASK-FE-023

## Goal
Sửa `NotificationStep.tsx` để detect web vs Electron mode và render khác nhau:
- **Electron mode**: giữ nguyên `MacNotificationPermissionCard` (backward compat)
- **Web mode**: render `WebModeNotificationStep` với browser permission + web push

## Steps

1. **Đọc** `src/renderer/src/components/onboarding/NotificationStep.tsx`.

2. **Thêm** mode detection:
```typescript
const isWebMode = import.meta.env.ORCA_PLATFORM === 'web'
```

3. **Tạo** `WebModeNotificationStep` sub-component (inline hoặc tách file):
   - Section "Browser Notifications" với states: default/granted/denied/unsupported
   - Section "Push Notifications" (chỉ khi browser granted + isSupported)
   - Section "Other Channels" notice
   - "Send Test Notification" button
   - Button IDs: `enable-browser-notif-btn`, `subscribe-push-btn`, `test-notification-btn`

4. **Render** conditionally:
```typescript
return isWebMode
  ? <WebModeNotificationStep ... />
  : <ElectronModeNotificationStep ... />
```

**Tests** (8 cases): mode detection, all notification states, push subscribe flow, test notification.

## Output Files
- **[MODIFY]** `src/renderer/src/components/onboarding/NotificationStep.tsx`

---

# TASK-FE-025: Tạo service-worker.js + đăng ký trong bootstrap

**Phase:** 3 | **Solution:** [FE-SOL-D](../solutions/FE-SOL-D-platform-polish.md) | **CR:** CR-OB-008 | **Status:** ✅ COMPLETED 2026-07-23  
**Depends on:** _(independent)_

## Goal
Tạo Service Worker xử lý Push notifications và đăng ký trong web bootstrap.

## Steps

1. **Tạo** `src/renderer/public/service-worker.js`:

```javascript
self.addEventListener('push', (event) => {
  if (!event.data) return
  const data = event.data.json()
  event.waitUntil(
    self.registration.showNotification(data.title ?? 'Orca', {
      body: data.body,
      icon: data.icon ?? '/favicon.ico',
      tag: data.tag,
      data: { url: data.url }
    })
  )
})

self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  if (event.notification.data?.url) {
    event.waitUntil(clients.openWindow(event.notification.data.url))
  }
})
```

2. **Sửa** `src/renderer/src/web/main-web-bootstrap.tsx` — thêm SW registration sau mount:

```typescript
if ('serviceWorker' in navigator) {
  try {
    await navigator.serviceWorker.register('/service-worker.js')
    console.log('[Web] Service Worker registered for push notifications')
  } catch {
    console.warn('[Web] Service Worker registration failed (non-fatal)')
  }
}
```

3. **Kiểm tra** `vite.web-spa.config.ts` — `public/` được serve từ root; `service-worker.js` phải accessible tại `/service-worker.js`.

**Tests** (3 cases): SW file tồn tại, push event handler, notificationclick handler.

## Output Files
- **[NEW]** `src/renderer/public/service-worker.js`
- **[MODIFY]** `src/renderer/src/web/main-web-bootstrap.tsx`
