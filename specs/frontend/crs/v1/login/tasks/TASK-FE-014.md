# TASK-FE-014 — Tạo `AdminLayout.tsx` + `AdminApp.tsx`

**Phase:** 3 — Admin Panel
**Solution:** [SOL-FE-LG-003](../solutions/SOL-FE-LG-003-admin-panel.md) §4.2, §4.3
**Depends on:** TASK-FE-009 (useAuthUser), TASK-FE-013 (admin-api-client)
**Blocks:** TASK-FE-015..TASK-FE-020
**Effort:** M (~35 phút)
**Status:** ✅ Done

---

## Mô tả

Tạo layout và router root cho Admin SPA. Admin SPA là separate entry point từ main App.

---

## Files cần tạo

### `src/renderer/src/components/admin/AdminLayout.tsx` [NEW]

```typescript
// Layout với:
// - Header: "🔧 Orca Admin" + user email + Logout button
// - Nav sidebar: Dashboard | Users | Policies | Sessions | Audit Log
// - <Outlet /> cho page content

// Nav items với NavLink (active class khi matched)
const NAV_ITEMS = [
  { to: '/admin/', label: '📊 Dashboard', exact: true },
  { to: '/admin/users', label: '👥 Users' },
  { to: '/admin/policies', label: '🔐 Policies' },
  { to: '/admin/sessions', label: '📡 Sessions' },
  { to: '/admin/audit', label: '📋 Audit Log' },
]
```

Dùng `useAuthUser()` và `useLogout()` từ TASK-FE-009.

### `src/renderer/src/admin/AdminApp.tsx` [NEW]

```typescript
// Root component của Admin SPA
// Dùng React Router BrowserRouter với basename="/admin"

import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { AdminLayout } from '../components/admin/AdminLayout'
// lazy imports cho tất cả pages:
const AdminDashboard = lazy(() => import('../components/admin/AdminDashboard'))
const UsersPage      = lazy(() => import('../components/admin/UsersPage'))
const UserForm       = lazy(() => import('../components/admin/UserForm'))
const PoliciesPage   = lazy(() => import('../components/admin/PoliciesPage'))
const PolicyForm     = lazy(() => import('../components/admin/PolicyForm'))
const SessionsPage   = lazy(() => import('../components/admin/SessionsPage'))
const AuditPage      = lazy(() => import('../components/admin/AuditPage'))
```

Routes:
```
/ → AdminDashboard
/users → UsersPage
/users/new → UserForm (create mode)
/users/:id/edit → UserForm (edit mode)
/policies → PoliciesPage
/policies/new → PolicyForm
/policies/:id/edit → PolicyForm
/sessions → SessionsPage
/audit → AuditPage
```

---

## Verify

```bash
npx tsc --noEmit
# Check không có TypeScript errors với React Router types
```
