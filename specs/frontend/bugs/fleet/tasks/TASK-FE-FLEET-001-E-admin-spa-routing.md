# TASK-FE-FLEET-001-E: Route `FleetDashboard` vào Admin SPA (CR-001)

**Domain:** fleet  
**Solution Ref:** SOL-FE-FLEET-001 §Files  
**Bug:** BUG-FE-FLEET-001  
**Priority:** 🟡 P2  
**Estimated:** 15 phút  
**Status:** ✅ DONE — Implemented

---

## Mục tiêu

Thêm route `/admin/fleet` vào Admin SPA (`AdminApp.tsx`) để navigate đến `FleetDashboard`.

---

## Files cần sửa

- `src/renderer/src/components/admin/AdminApp.tsx`

---

## Các bước thực thi

### Bước 1: Import FleetDashboard

```typescript
import { FleetDashboard } from './fleet/fleet-dashboard'
```

### Bước 2: Thêm route trong React Router

```typescript
<Routes>
  {/* existing routes */}
  <Route path="/admin/fleet" element={<FleetDashboard />} />
</Routes>
```

### Bước 3: Thêm nav link trong sidebar/menu

```typescript
{ path: '/admin/fleet', label: 'Fleet', icon: Server }
```

---

## Verify

```bash
grep -n "fleet\|FleetDashboard" \
  src/renderer/src/components/admin/AdminApp.tsx
```

## Depends on
TASK-FE-FLEET-001-D
