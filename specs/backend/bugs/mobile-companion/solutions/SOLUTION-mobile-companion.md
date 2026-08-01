# SOLUTION: Mobile Companion Domain — Fix tất cả Bugs

**Domain:** mobile-companion  
**TDD Reference:** TDD-11 (Web Server Mode), TDD-04 (RPC Server), TDD-13 (Dev Server Onboarding §web-push)  
**Files cần thay đổi:** `src/main/mobile/MobileCompanionService.ts` (NEW), `src/server/mobile-api-routes.ts` (NEW)  
**Tổng số bugs:** 1 (BE-MB-001)

---

## BUG-BE-MB-001 — Fix MobileCompanionService not implemented

**Mức độ:** 🟡 MEDIUM  
**Root cause:** Mobile companion feature (push notifications, remote control từ phone) chưa được implement.

### Fix — Implement MobileCompanionService

Theo TDD v5 §onboarding CRs:
> "Key new files: `src/main/notifications/web-push-manager.ts` — VAPID + push subscriptions"
> "Key new files: `src/server/push-api-routes.ts` — Push API HTTP routes"

```typescript
// src/main/mobile/MobileCompanionService.ts (NEW)

export interface MobileDevice {
  id:           string
  userId:       string
  deviceName:   string
  platform:     'ios' | 'android' | 'web'
  pushEndpoint: string    // Web Push endpoint
  pushKeys: {
    p256dh: string
    auth:   string
  }
  registeredAt: number
  lastSeenAt:   number
}

export class MobileCompanionService {
  constructor(
    private readonly repository: IMobileDeviceRepository,
    private readonly pushManager: WebPushManager,
    private readonly log: Logger,
  ) {}

  /**
   * Register thiết bị mobile để nhận push notifications.
   */
  async registerDevice(params: {
    userId:       string
    deviceName:   string
    platform:     MobileDevice['platform']
    subscription: PushSubscription  // Web Push API subscription object
  }): Promise<MobileDevice> {
    const device: MobileDevice = {
      id:           generateId(),
      userId:       params.userId,
      deviceName:   params.deviceName,
      platform:     params.platform,
      pushEndpoint: params.subscription.endpoint,
      pushKeys:     {
        p256dh: params.subscription.keys.p256dh,
        auth:   params.subscription.keys.auth,
      },
      registeredAt: Date.now(),
      lastSeenAt:   Date.now(),
    }

    await this.repository.create(device)
    this.log.info(`[Mobile] Device registered: ${device.id} for user ${params.userId}`)
    return device
  }

  /**
   * Send push notification đến thiết bị của user.
   */
  async sendNotification(
    userId: string,
    notification: {
      title:   string
      body:    string
      data?:   Record<string, unknown>
      actions?: Array<{ action: string; title: string }>
    }
  ): Promise<void> {
    const devices = await this.repository.listByUser(userId)
    
    await Promise.allSettled(devices.map(async (device) => {
      try {
        await this.pushManager.sendNotification(
          {
            endpoint: device.pushEndpoint,
            keys:     device.pushKeys,
          },
          {
            title:  notification.title,
            body:   notification.body,
            data:   notification.data,
            badge:  '/icons/badge.png',
            icon:   '/icons/icon-192.png',
            actions: notification.actions,
          }
        )
        // Update lastSeenAt
        await this.repository.updateLastSeen(device.id)
      } catch (err: unknown) {
        // Remove expired subscriptions
        const isExpired = (err as any)?.statusCode === 410  // Gone
        if (isExpired) {
          await this.repository.delete(device.id)
          this.log.info(`[Mobile] Removed expired subscription: ${device.id}`)
        }
      }
    }))
  }

  /**
   * Unregister thiết bị mobile.
   */
  async unregisterDevice(deviceId: string, userId: string): Promise<void> {
    const device = await this.repository.findById(deviceId)
    if (!device || device.userId !== userId) return
    await this.repository.delete(deviceId)
    this.log.info(`[Mobile] Device unregistered: ${deviceId}`)
  }

  /**
   * List thiết bị của user.
   */
  async listDevices(userId: string): Promise<MobileDevice[]> {
    return await this.repository.listByUser(userId)
  }
}

// src/server/mobile-api-routes.ts (NEW)
import { Router } from 'express'

export function createMobileApiRouter(mobileService: MobileCompanionService): Router {
  const router = Router()

  // Register device
  router.post('/devices', requireAuth, async (req, res) => {
    try {
      const device = await mobileService.registerDevice({
        userId:       req.orcaSession!.userId,
        deviceName:   req.body.deviceName,
        platform:     req.body.platform,
        subscription: req.body.subscription,
      })
      res.json({ device })
    } catch (err) {
      res.status(500).json({ error: String(err) })
    }
  })

  // List devices
  router.get('/devices', requireAuth, async (req, res) => {
    const devices = await mobileService.listDevices(req.orcaSession!.userId)
    res.json({ devices })
  })

  // Unregister device
  router.delete('/devices/:id', requireAuth, async (req, res) => {
    await mobileService.unregisterDevice(req.params.id, req.orcaSession!.userId)
    res.json({ ok: true })
  })

  // VAPID public key (browser needs this to subscribe)
  router.get('/vapid-public-key', (req, res) => {
    res.json({ publicKey: process.env.VAPID_PUBLIC_KEY ?? '' })
  })

  return router
}
```

---

## Tóm tắt file changes

| File | Action | Bug |
|------|--------|-----|
| `src/main/mobile/MobileCompanionService.ts` | NEW — implement mobile companion | BE-MB-001 |
| `src/server/mobile-api-routes.ts` | NEW — HTTP routes for mobile | BE-MB-001 |
| `src/main/repositories/mobile-device-repository.ts` | NEW — repository interface + SQL impl | BE-MB-001 |
| `src/main/db/migrations/0011_mobile_devices.ts` | NEW migration | BE-MB-001 |
| `src/server/http-server.ts` | Mount /mobile/api router | BE-MB-001 |
| `src/main/server-bootstrap.ts` | Init MobileCompanionService | BE-MB-001 |

---

## Verification Plan

```bash
# Environment setup:
# VAPID_PUBLIC_KEY=... VAPID_PRIVATE_KEY=... pnpm dev:web

# Manual test:
# 1. GET /mobile/api/vapid-public-key → verify public key returned
# 2. POST /mobile/api/devices { subscription, deviceName, platform } → verify device registered
# 3. Trigger agent event → verify push notification received on device
# 4. DELETE /mobile/api/devices/:id → verify device unregistered
# 5. Subscription expired → verify auto-cleanup

pnpm vitest run src/main/mobile/__tests__/
```
