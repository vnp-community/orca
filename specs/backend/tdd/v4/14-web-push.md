# TDD-BE-14: Web Push

**Version:** 4.0  
**Date:** 2026-07-28  
**Source:** `src/main/notifications/web-push-manager.ts`, `src/server/push-api-routes.ts`

---

## 1. Mục tiêu

Gửi Web Push notifications tới browser khi dev server events xảy ra (connected, error, etc.).

---

## 2. WebPushManager

```typescript
class WebPushManager {
  constructor(store: Store) {
    // Load VAPID keys từ store hoặc generate mới
    // VAPID keys: { publicKey, privateKey } stored tại 'vapid.keys'
  }

  // Register push subscription từ browser
  async subscribe(subscription: PushSubscription, userId: string): Promise<void>

  // Remove push subscription
  async unsubscribe(endpoint: string): Promise<void>

  // Send notification tới user (tất cả subscriptions của user)
  async notifyUser(userId: string, payload: PushPayload): Promise<void>

  // Broadcast tới tất cả subscriptions
  async broadcast(payload: PushPayload): Promise<void>

  // Get VAPID public key (frontend cần để subscribe)
  getVapidPublicKey(): string
}
```

---

## 3. Push API Routes

```
GET  /push/vapid-public-key
  → { publicKey: string }  (không cần auth)

POST /push/subscribe
  Auth: session cookie required
  Body: PushSubscription (Web Push standard format)
  → 201 Created

DELETE /push/subscribe
  Auth: session cookie required
  Body: { endpoint: string }
  → 200 OK

POST /push/test
  Auth: admin only
  → Send test notification tới caller
```

---

## 4. PushPayload

```typescript
type PushPayload = {
  title:   string
  body:    string
  icon?:   string    // URL to app icon
  tag?:    string    // Notification grouping key
  data?:   unknown   // Custom data for service worker
  url?:    string    // URL to open on click
}
```

---

## 5. Service Worker Integration

```javascript
// src/renderer/service-worker.js
self.addEventListener('push', (event) => {
  const payload = event.data?.json()
  event.waitUntil(
    self.registration.showNotification(payload.title, {
      body:  payload.body,
      icon:  payload.icon,
      tag:   payload.tag,
      data:  payload.data
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

---

## 6. VAPID Key Storage

- VAPID keys được generate 1 lần và persist trong `Store` (key: `vapid.keys`)
- Nếu key không tồn tại → generate bằng `webpush.generateVAPIDKeys()`
- Public key được expose qua `GET /push/vapid-public-key`

---

## 7. Push Subscriptions Storage

Subscriptions stored trong `Store` (key: `push.subscriptions`):
```typescript
type StoredSubscription = {
  endpoint:   string
  userId:     string
  keys: {
    p256dh: string
    auth:   string
  }
  createdAt:  number
}
```

TTL: Subscriptions không tự expire. Nếu push fail với 410 Gone → auto-remove.
