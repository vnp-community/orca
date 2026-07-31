# Orca Frontend — Technical Design Document v4
## Index & Overview

**Version:** 4.0 (as-implemented — login CRs complete)  
**Date:** 2026-07-28  
**Source:** `src/renderer/src/` + `src/shared/` + `src/platform/adapters/web/`  
**Baseline TDD:** [`../00-index.md`](../00-index.md) (v1–v4 history)

---

## Tài liệu trong bộ TDD v4

| # | File | Nội dung | Status |
|---|------|---------|--------|
| 1 | [01-architecture-overview.md](./01-architecture-overview.md) | Render targets, entry points, build config | ✅ |
| 2 | [02-auth-flow.md](./02-auth-flow.md) | Auth boot, LoginPage, AuthSlice, session check | ✅ |
| 3 | [03-admin-spa.md](./03-admin-spa.md) | Admin SPA — AdminApp, Users, Sessions, Audit | ✅ |
| 4 | [04-state-management.md](./04-state-management.md) | Zustand store, AuthSlice, DevServerSlice, SSH extensions | ✅ |
| 5 | [05-runtime-client.md](./05-runtime-client.md) | WebSocketRpcClient, WebRuntimeClient, ConnectionStatus | ✅ |
| 6 | [06-web-entry.md](./06-web-entry.md) | bootstrapWebApp(), auth check, LoginPage render | ✅ |
| 7 | [07-onboarding-devserver.md](./07-onboarding-devserver.md) | Dev Server UI, onboarding wizard, agent detection | ✅ |
| 8 | [08-ssh-provisioning-ui.md](./08-ssh-provisioning-ui.md) | SSH user provisioning, progress UI, user indicator | ✅ |
| 9 | [09-fleet-management-ui.md](./09-fleet-management-ui.md) | Fleet health, bootstrap UI, RBAC, bulk provisioning | ✅ |
| 10 | [10-web-push-ui.md](./10-web-push-ui.md) | Web Push subscription, service worker, notifications | ✅ |

---

## Kiến trúc tổng thể v4

```
src/renderer/
├─ index.html              ← Desktop Electron entry
├─ web-index.html          ← Web SPA entry
├─ admin-index.html        ← Admin SPA entry (NEW v4.0)
│
└─ src/
    ├─ main.tsx            ← Desktop: RecoverableErrorBoundary → App.tsx
    ├─ App.tsx             ← Core app (~127KB) — KHÔNG SỬA
    │
    ├─ web/
    │   ├─ main.tsx        ← Web: delegates → bootstrapWebApp()
    │   ├─ main-web-bootstrap.tsx  ← checkAuthSession() → LoginPage | App.tsx
    │   ├─ login/
    │   │   ├─ LoginPage.tsx
    │   │   ├─ LoginForm.tsx
    │   │   ├─ SsoButton.tsx
    │   │   └─ PairCodeFallback.tsx
    │   ├─ ConnectionStatusProvider.tsx
    │   └─ ConnectionStatusBanner.tsx
    │
    ├─ admin/
    │   └─ admin-main.tsx  ← Admin SPA: AdminApp.tsx
    │
    ├─ auth/
    │   ├─ auth-types.ts   ← AuthUser, AuthState, AuthError
    │   ├─ auth-api-client.ts ← fetchCurrentUser(), loginLocal(), etc.
    │   └─ auth-utils.ts   ← toLinuxUsername() (mirror backend)
    │
    ├─ store/
    │   ├─ index.ts        ← useAppStore + createAuthSlice registration
    │   └─ slices/
    │       ├─ auth.ts     ← AuthSlice (NEW v4.0)
    │       ├─ dev-servers.ts  ← DevServerSlice
    │       └─ ssh.ts      ← Extended: ProvisioningStatus, SshUserAccount
    │
    ├─ hooks/
    │   ├─ useAuthSession.ts    ← useAuthUser(), useIsAuthenticated()
    │   ├─ useLogout.ts
    │   ├─ useSshUserAccount.ts
    │   ├─ useSshProvisioning.ts
    │   ├─ useFleetHealthPolling.ts
    │   └─ useWebPushSubscription.ts
    │
    └─ components/
        ├─ auth/
        │   ├─ UserAvatarMenu.tsx   ← Avatar dropdown (web-only)
        │   └─ UserRoleBadge.tsx    ← Role indicator
        ├─ admin/
        │   ├─ admin-api-client.ts
        │   ├─ AdminApp.tsx
        │   ├─ AdminDashboard.tsx
        │   ├─ UsersPage.tsx + UserForm.tsx
        │   ├─ SessionsPage.tsx
        │   ├─ PoliciesPage.tsx + PolicyForm.tsx
        │   └─ AuditPage.tsx
        └─ ssh/
            ├─ SshProvisioningProgress.tsx
            └─ SshUserIndicator.tsx
```

---

## Build Targets v4

| Script | Output | Entry points |
|--------|--------|-------------|
| `pnpm build` | `out/renderer/` | `index.html` (Electron) |
| `pnpm build:frontend:web` | `out/web/` | `web-index.html` + `admin-index.html` |
| `pnpm dev:web-spa` | Dev server :5174 | same |

---

## Nguyên tắc thiết kế v4

1. **Auth-first boot** — `bootstrapWebApp()` check `/auth/me` TRƯỚC khi render bất kỳ UI nào
2. **Additive-only** — Không sửa `App.tsx`, `main.tsx`, `web-preload-api.ts`
3. **AuthSlice tập trung** — Toàn bộ auth state trong 1 Zustand slice
4. **Admin SPA riêng** — `admin-index.html` → `AdminApp.tsx` — không thuộc App.tsx
5. **Web-only UI** — Login/SSO/Avatar chỉ xuất hiện khi `ORCA_PLATFORM === 'web'`
6. **Two render targets** — Desktop + Web dùng cùng `App.tsx` — tách qua `window.api`
7. **IRpcClient interface** — Transport abstraction cho WebSocketRpcClient
8. **SSH provisioning UI** — Inject vào SshTargetRow (additive, không sửa core)
9. **toLinuxUsername() sync** — Frontend mirror chính xác backend impl

---

## Test Coverage v4

| Module | Tests |
|--------|-------|
| `auth/__tests__/` | 14 tests |
| `web/login/__tests__/` | 16 tests |
| `hooks/__tests__/useAuthSession` | 6 tests |
| `hooks/__tests__/useLogout` | 3 tests |
| `components/auth/__tests__/` | 12 tests |
| `components/admin/__tests__/` | 18 tests |
| `components/ssh/__tests__/` | 7 tests |
| `hooks/__tests__/useSsh*` | 6 tests |
| Platform/web RPC client | 15 tests |
| ConnectionStatusProvider | 11 tests |
| **Total (v4 additions)** | **108 tests** |
