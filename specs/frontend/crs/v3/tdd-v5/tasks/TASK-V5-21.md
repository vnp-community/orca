# TASK-V5-21: Admin SPA Integration (Mount All Features)

**Order:** 21 | **Prerequisite:** TASK-V5-05, TASK-V5-08 | **Tests:** 0 (manual verification)

---

## Mô tả

Mount tất cả v5.0 features vào Admin SPA và wrap App với WorkspaceProvider. Đây là task cuối — chỉ sửa files hiện có, additive-only.

---

## Files Cần Sửa

### 1. `src/renderer/src/components/admin/AdminApp.tsx` — Add Routes

```typescript
// Tìm phần <Routes> và thêm (lazy imports):

// Thêm ở đầu file (lazy imports):
const CompanyProfileAdmin = lazy(() =>
  import('../profile/CompanyProfileAdmin').then(m => ({ default: m.CompanyProfileAdmin }))
)
const ProviderList = lazy(() =>
  import('../ai-provider/ProviderList').then(m => ({ default: m.ProviderList }))
)

// Trong <Routes>:
<Route path="/profile"      element={<CompanyProfileAdmin />} />
<Route path="/ai-providers" element={<ProviderList />} />
```

### 2. `src/renderer/src/components/admin/AdminSidebar.tsx` (hoặc tương đương nav file)

```typescript
// Thêm nav links (additive):
const NAV_ITEMS = [
  // ... existing items ...
  { path: '/profile',       label: 'Company Profile', icon: <User size={16} /> },
  { path: '/ai-providers',  label: 'AI Providers',    icon: <Bot  size={16} /> },
]
```

### 3. `src/renderer/src/web/main-web-bootstrap.tsx` hoặc `src/renderer/src/main.tsx`

```typescript
// Wrap root với WorkspaceProvider (additive, around existing App):
import { WorkspaceProvider } from '../context/WorkspaceContext'

// Tìm <App /> và wrap:
<WorkspaceProvider>
  <App />
</WorkspaceProvider>
```

---

## Verification Steps (Manual)

```bash
# 1. Start dev server
npm run dev

# 2. Navigate to Admin SPA:
# → /admin/profile        ← CompanyProfileAdmin
# → /admin/ai-providers   ← ProviderList

# 3. Check workspace context:
# → Open DevTools console
# → window.__workspaceContext  // should exist after TASK-02
```

---

## Acceptance Criteria

- [x] `/admin/profile` route renders `CompanyProfileAdmin` without errors
- [x] `/admin/ai-providers` route renders `ProviderList` without errors
- [x] `WorkspaceProvider` wraps root app
- [x] No regression in existing Admin pages (auth, settings, etc.)
- [x] `npm run build` completes without TypeScript errors
