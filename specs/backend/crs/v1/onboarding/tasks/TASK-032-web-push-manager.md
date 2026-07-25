# TASK-032: Tạo `src/main/notifications/web-push-manager.ts`

**Phase:** 3 — Web Push Notifications  
**Solution:** [SOL-007-008-009](../solutions/SOL-007-008-009-windows-notifications-checklist.md) §B.4  
**Depends on:** TASK-031, TASK-033  
**Blocks:** TASK-034, TASK-035

---

## Mục tiêu

Tạo class `WebPushManager` với VAPID key lifecycle, subscription CRUD, và send-to-all functionality.

---

## File cần tạo

**Path:** `src/main/notifications/web-push-manager.ts`

---

## Nội dung cần implement

```typescript
import webPush from 'web-push'
import { randomUUID } from 'node:crypto'
import type { Store } from '../persistence'

export type WebPushPayload = {
  title: string
  body: string
  icon?: string
  tag?: string
  url?: string
}

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

  saveSubscription(
    subscription: PushSubscriptionJSON,
    meta?: { userAgent?: string }
  ): WebPushSubscription {
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

  private async sendToSubscription(
    sub: WebPushSubscription,
    payload: WebPushPayload
  ): Promise<void> {
    try {
      await webPush.sendNotification(
        { endpoint: sub.endpoint, keys: sub.keys },
        JSON.stringify(payload),
        { TTL: 86400 }
      )
    } catch (err: unknown) {
      // 410 Gone: subscription expired — auto-remove
      if ((err as { statusCode?: number }).statusCode === 410) {
        this.removeSubscription(sub.endpoint)
      }
      // Other errors: log (không throw để không ảnh hưởng các subscriptions khác)
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

// Re-export type (import từ TASK-033 types.ts sau khi implement)
export type { WebPushSubscription } from '../../shared/types'
```

---

## Acceptance Criteria

- [x] File tồn tại tại `src/main/notifications/web-push-manager.ts`
- [x] `WebPushManager` class được export
- [x] `loadOrCreateVapidKeys()`: tạo keys mới nếu chưa có, persist vào store
- [x] `loadOrCreateVapidKeys()`: reuse keys đã có (không tạo lại)
- [x] `saveSubscription()`: deduplicate theo endpoint (upsert)
- [x] `sendToAll()`: dùng `Promise.allSettled` (1 lỗi không ảnh hưởng others)
- [x] `sendToSubscription()`: tự xóa subscription bị 410 Gone
- [x] `WebPushPayload` type được export
- [x] TypeScript compile thành công
