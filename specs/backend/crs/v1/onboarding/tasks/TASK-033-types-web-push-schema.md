# TASK-033: Sửa `src/shared/types.ts` — Thêm `webPushSubscriptions` + `vapidKeys`

**Phase:** 3 — Web Push Notifications  
**Solution:** [SOL-007-008-009](../solutions/SOL-007-008-009-windows-notifications-checklist.md) §B.3  
**Depends on:** TASK-002  
**Blocks:** TASK-032

---

## Mục tiêu

Thêm `webPushSubscriptions[]` và `vapidKeys` vào `PersistedState`, và thêm `WebPushSubscription` type.

---

## File cần sửa

**Path:** `src/shared/types.ts`

---

## Thay đổi cần thực hiện

### 1. Thêm `WebPushSubscription` type

```typescript
export type WebPushSubscription = {
  id: string
  endpoint: string
  keys: { auth: string; p256dh: string }
  addedAt: number
  userAgent?: string
}
```

### 2. Thêm vào `PersistedState`

```typescript
type PersistedState = {
  // ... existing fields giữ nguyên ...
  webPushSubscriptions?: WebPushSubscription[]       // NEW
  vapidKeys?: { publicKey: string; privateKey: string }  // NEW
}
```

---

## Acceptance Criteria

- [x] `WebPushSubscription` type được export
- [x] `PersistedState` có field `webPushSubscriptions?: WebPushSubscription[]`
- [x] `PersistedState` có field `vapidKeys?: { publicKey: string; privateKey: string }`
- [x] Cả 2 fields là optional (backward compatible)
- [x] TypeScript compile thành công
