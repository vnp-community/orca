# TASK-FE-FLEET-001-D: `FleetDashboard` + `FleetAlertStrip` components (CR-005)

**Domain:** fleet  
**Solution Ref:** SOL-FE-FLEET-001 §Component 1 + SOL-FE-FLEET-001B §Phần 4  
**Bug:** BUG-FE-FLEET-001  
**Priority:** 🟠 P1  
**Estimated:** 60 phút  
**Status:** ✅ DONE — Implemented

---

## Mục tiêu

Tạo `FleetDashboard` — trang chính của fleet management. Tạo `FleetAlertStrip` — hiển thị alerts unreachable/outdated.

---

## Files cần tạo

- **TẠO MỚI:** `src/renderer/src/components/admin/fleet/fleet-dashboard.tsx`
- **TẠO MỚI:** `src/renderer/src/components/admin/fleet/fleet-alert-strip.tsx`

---

## Bước 1: `fleet-alert-strip.tsx`

```typescript
// Props: alerts: FleetAlert[]
// Per alert: AlertTriangle icon (yellow) + serverLabel bold + message + X dismiss button
// Max 3 visible + "+N more" text
// dismissFleetAlert(alertId) từ store
```

## Bước 2: `fleet-dashboard.tsx`

Layout:
```
[Header: Fleet Configuration] [Import YAML] [Check Health Now]
[FleetAlertStrip] — chỉ hiện khi có alert chưa dismissed
[Tabs: All Servers | Groups | Bulk Actions | Bootstrap | RBAC]
  Tab "All Servers":
    [Filter: Group | Status | Search]
    [ServerHealthTable: server | status | cpu | mem | disk | relay | actions]
```

Hooks cần dùng:
```typescript
const { isPolling, checkNow } = useFleetHealthPolling({ autoStart: true })
```

Tích hợp `FleetImportDialog` khi click "Import YAML" button.

---

## Verify

```bash
grep -n "FleetDashboard\|FleetAlertStrip" \
  src/renderer/src/components/admin/fleet/fleet-dashboard.tsx
```

## Depends on
TASK-FE-FLEET-001-A, TASK-FE-FLEET-001-B, TASK-FE-FLEET-001-C

## Blocking
TASK-FE-FLEET-001-E (Admin SPA routing)
