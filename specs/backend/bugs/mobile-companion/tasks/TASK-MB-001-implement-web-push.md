# TASK-MB-001: Implement Web Push notification cho Mobile Companion

**Priority:** 🟡 MEDIUM — Mobile companion không nhận notifications  
**Effort:** ~90 phút  
**Status:** ✅ DONE  
**Bug refs:** BUG-BE-MB-001  
**Solution ref:** [SOLUTION-mobile-companion.md](../solutions/SOLUTION-mobile-companion.md)

## Mục tiêu

Implement Web Push (RFC 8030) để gửi notifications tới mobile browsers.

## Bước 1 — Cài web-push library

```bash
# Kiểm tra đã có chưa:
grep "web-push" package.json
# Nếu chưa có:
pnpm add web-push
pnpm add -D @types/web-push
```

## Bước 2 — Tạo file

`src/main/mobile/MobileCompanionService.ts` (NEW)

```typescript
import webpush from 'web-push'
import type { IConnectionPool } from '../db/pool'

export class MobileCompanionService {
  constructor(private readonly pool: IConnectionPool) {
    // VAPID keys từ env:
    webpush.setVapidDetails(
      `mailto:${process.env['ORCA_ADMIN_EMAIL'] ?? 'admin@example.com'}`,
      process.env['VAPID_PUBLIC_KEY']!,
      process.env['VAPID_PRIVATE_KEY']!
    )
  }

  async registerDevice(userId: string, subscription: webpush.PushSubscription): Promise<void> {
    await this.pool.withConnection((db) =>
      db.query(
        `INSERT OR REPLACE INTO orca_push_subscriptions (user_id, endpoint, auth, p256dh, updated_at)
         VALUES (?, ?, ?, ?, ?)`,
        [userId, subscription.endpoint, subscription.keys.auth, subscription.keys.p256dh, Date.now()]
      )
    )
  }

  async notify(userId: string, payload: { title: string; body: string; url?: string }): Promise<void> {
    const subs = await this.pool.withConnection((db) =>
      db.query<{ endpoint: string; auth: string; p256dh: string }>(
        'SELECT endpoint, auth, p256dh FROM orca_push_subscriptions WHERE user_id = ?',
        [userId]
      )
    )

    await Promise.allSettled(
      subs.map(sub =>
        webpush.sendNotification(
          { endpoint: sub.endpoint, keys: { auth: sub.auth, p256dh: sub.p256dh } },
          JSON.stringify(payload)
        ).catch(err => console.warn('[Push] Send failed:', err))
      )
    )
  }
}
```

## Bước 3 — Migration

```sql
CREATE TABLE IF NOT EXISTS orca_push_subscriptions (
  user_id    TEXT NOT NULL,
  endpoint   TEXT NOT NULL PRIMARY KEY,
  auth       TEXT NOT NULL,
  p256dh     TEXT NOT NULL,
  updated_at INTEGER NOT NULL
)
```

## Verification

```bash
pnpm tsc --noEmit
# Setup: generate VAPID keys: npx web-push generate-vapid-keys
# Test: register device → trigger event → mobile browser receives notification
```
