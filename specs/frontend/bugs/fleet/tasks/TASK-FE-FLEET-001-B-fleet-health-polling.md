# TASK-FE-FLEET-001-B: `useFleetHealthPolling` hook — 30s polling + alerts (CR-005)

**Domain:** fleet  
**Solution Ref:** SOL-FE-FLEET-001B Phần 2  
**Bug:** BUG-FE-FLEET-001  
**Priority:** 🟠 P1  
**Estimated:** 40 phút  
**Status:** ✅ DONE — Implemented in use-fleet-health-polling.ts

---

## Mục tiêu

Tạo `useFleetHealthPolling` hook — polling fleet health mỗi 30s, tự động tạo alert khi server unreachable hoặc relay version outdated.

---

## Files cần tạo

- **TẠO MỚI:** `src/renderer/src/hooks/useFleetHealthPolling.ts`

---

## Các bước thực thi

Tạo file với nội dung đầy đủ từ SOL-FE-FLEET-001B §Phần 2:

1. **Options:** `{ intervalMs = 30_000, autoStart = true }`
2. **checkNow():** Gọi `rpc.call('fleet.health.checkAll', { serverIds })` → nhận `ServerHealthMetrics[]`
3. **Per-result:**
   - `updateServerHealth(metrics.serverId, metrics)`
   - Nếu `!metrics.isReachable` → `addFleetAlert({ type: 'disconnected', ... })`
   - Nếu relay version outdated → `addFleetAlert({ type: 'relay-outdated', ... })`
4. **Auto-polling:** `setInterval(checkNow, intervalMs)` với cleanup
5. **Initial check:** gọi `checkNow()` ngay khi mount

**Helper `isRelayOutdated(version)`:** So sánh với minimum required relay version `'1.5.0'`.

**Helper `compareVersions(a, b)`:** Semver comparison bằng split('.').map(Number).

---

## Verify

```bash
grep -n "useFleetHealthPolling\|fleet.health.checkAll" \
  src/renderer/src/hooks/useFleetHealthPolling.ts
```

## Depends on
TASK-FE-FLEET-001-A (slice actions)

## Blocking
TASK-FE-FLEET-001-D (FleetDashboard)
