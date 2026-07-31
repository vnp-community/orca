# TDD-FE-10: Web Push UI

**Version:** 4.0  
**Date:** 2026-07-28  
**Source:** `src/renderer/src/hooks/`, `src/renderer/service-worker.js`

---

## 1. Mục tiêu

Nhận browser notifications khi dev server events xảy ra (agent connected, error, etc.) — ngay cả khi Orca tab không active.

---

## 2. Service Worker

```javascript
// src/renderer/service-worker.js

// Push event handler
self.addEventListener('push', (event) => {
  const payload = event.data?.json()
  event.waitUntil(
    self.registration.showNotification(payload.title, {
      body:  payload.body,
      icon:  '/icon.png',
      tag:   payload.tag,
      data:  payload.data
    })
  )
})

// Click handler — open Orca tab on notification click
self.addEventListener('notificationclick', (event) => {
  event.notification.close()
  event.waitUntil(
    clients.matchAll({ type: 'window' }).then(clientList => {
      for (const client of clientList) {
        if (client.url.includes(self.location.origin)) {
          return client.focus()
        }
      }
      return clients.openWindow('/')
    })
  )
})
```

---

## 3. useBrowserNotificationPermission Hook

```typescript
// src/renderer/src/hooks/useBrowserNotificationPermission.ts

function useBrowserNotificationPermission(): {
  permission: NotificationPermission   // 'default' | 'granted' | 'denied'
  request: () => Promise<NotificationPermission>
}
```

---

## 4. useWebPushSubscription Hook

```typescript
// src/renderer/src/hooks/useWebPushSubscription.ts

function useWebPushSubscription(): {
  subscribed:    boolean
  subscribing:   boolean
  error:         string | null
  subscribe:     () => Promise<void>
  unsubscribe:   () => Promise<void>
}

// subscribe() flow:
// 1. Get VAPID public key: GET /push/vapid-public-key
// 2. navigator.serviceWorker.ready.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey })
// 3. POST /push/subscribe { subscription }
```

---

## 5. SW Registration (bootstrapWebApp)

```typescript
// Trong bootstrapWebApp() — STEP 1
if ('serviceWorker' in navigator) {
  const reg = await navigator.serviceWorker.register('/service-worker.js')
  // Store registration cho push subscription
  window.__SW_REGISTRATION__ = reg
}
```

---

## 6. Push Notification Examples

| Event | Title | Body |
|-------|-------|------|
| Dev server connected | "Dev Server Online" | "prod-server is now connected" |
| Dev server error | "Dev Server Error" | "staging-server: Connection refused" |
| Agent timeout | "Agent Disconnected" | "Agent token expired — restart agent" |
| Provisioning done | "SSH Setup Complete" | "Linux user orca-binhnt provisioned on prod" |

---

## 7. Notification Permissions Flow

```
First visit:
  → useBrowserNotificationPermission: permission = 'default'
  → Show banner: "Enable notifications for dev server alerts"
  → User clicks Allow → permission = 'granted'
  → useWebPushSubscription.subscribe()

Already granted:
  → bootstrapWebApp detects 'granted' → auto-subscribe

Denied:
  → Show info: "Notifications disabled. Enable in browser settings."
```
