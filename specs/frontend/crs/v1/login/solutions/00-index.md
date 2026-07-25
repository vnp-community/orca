# Frontend Solutions — Login (Multi-User Auth, Sandbox, SSH Isolation, Admin)
## Index

**Version:** 1.0
**Date:** 2026-07-24
**CRs:** [docs/crs/v1/login/](../../../../../docs/crs/v1/login/)
**TDD Reference:** [specs/frontend/tdd/](../../../tdd/)
**Backend Solutions:** [specs/backend/crs/v1/login/solutions/](../../../../backend/crs/v1/login/solutions/)
**Based on TDD:** TDD-FE-02 (State Management), TDD-FE-03 (Runtime Client), TDD-FE-05 (UI Components), TDD-FE-06 (Web Client), TDD-FE-09 (Onboarding)

---

## Mục tiêu

Bộ solutions này cung cấp **hướng dẫn triển khai chi tiết** (test-driven) cho 4 Change Requests trong `login` ở phía **frontend (renderer process)**:

- Login page + SSO buttons tích hợp vào web entry point
- Session-aware web client: bỏ qua pairing form nếu đã login
- User identity display trong App shell (avatar, role, logout)
- Admin Panel SPA: quản lý users, sessions, policies, audit log

### Nguyên tắc thiết kế

1. **Additive Only** — Không thay đổi `App.tsx`, `web-preload-api.ts`, `main.tsx` (desktop)
2. **Backward Compat** — PairCode/WebConnect vẫn hoạt động song song với login
3. **Web-only** — Login/SSO UI chỉ xuất hiện trong web mode (`ORCA_PLATFORM === 'web'`)
4. **Zustand + Auth Slice** — Auth state tập trung vào 1 Zustand slice mới `auth.ts`
5. **Interface-driven** — HTTP client (fetch) calls qua `auth-api-client.ts` (dễ mock)
6. **Test-driven** — Viết test spec trước implementation, mỗi component/hook ≥ 3 test cases
7. **No App.tsx changes** — Admin SPA là **separate route/entry** (`/admin/`), không nằm trong App.tsx

---

## Danh sách Solutions

| Solution | CR tương ứng | Domain | TDD Reference | Status |
|----------|-------------|--------|--------------|--------|
| [SOL-FE-LG-001](./SOL-FE-LG-001-login-page.md) | CR-LOGIN-001 | Login Page + SSO UI + Session-aware routing | TDD-FE-06, TDD-FE-02, TDD-FE-05 | ✅ Done |
| [SOL-FE-LG-002](./SOL-FE-LG-002-user-identity.md) | CR-LOGIN-001, CR-LOGIN-002 | User Identity Display (avatar, role, logout) | TDD-FE-05, TDD-FE-02 | ✅ Done |
| [SOL-FE-LG-003](./SOL-FE-LG-003-admin-panel.md) | CR-LOGIN-004 | Admin Panel SPA (Users, Sessions, Policies, Audit) | TDD-FE-06, TDD-FE-05 | ✅ Done |
| [SOL-FE-LG-004](./SOL-FE-LG-004-ssh-ui.md) | CR-LOGIN-003 | SSH User Indicator + Provisioning Progress | TDD-FE-05, TDD-FE-09 | ✅ Done |

---

## Mapping CR → Frontend Solution

```
CR-LOGIN-001 (Auth: Login + SSO)     → SOL-FE-LG-001 (Login Page)
                                      → SOL-FE-LG-002 (User Identity in App Shell)
CR-LOGIN-002 (Per-User Sandbox)      → SOL-FE-LG-002 (session context aware routing)
CR-LOGIN-003 (SSH Dev Isolation)     → SOL-FE-LG-004 (SSH User Indicator)
CR-LOGIN-004 (Admin UI)              → SOL-FE-LG-003 (Admin Panel SPA)
```

---

## Dependency thực hiện

```
SOL-FE-LG-001 (Login Page + Auth Slice) — phải xong trước
    │
    ├──► SOL-FE-LG-002 (User Identity) — cần session state từ auth slice
    │         │
    │         └──► SOL-FE-LG-004 (SSH UI) — cần userId từ auth slice
    │
    └──► SOL-FE-LG-003 (Admin Panel) — cần auth session cookie + admin API client
```

---

## File Structure Mục tiêu

```
src/renderer/src/
│
├── auth/                                        ← [NEW] Auth HTTP client layer
│   ├── auth-api-client.ts                       ← fetch() wrapper cho /auth/* endpoints
│   ├── auth-types.ts                            ← AuthSession, AuthUser, SsoProvider types
│   └── __tests__/
│       └── auth-api-client.test.ts
│
├── store/slices/
│   └── auth.ts                                  ← [NEW] AuthSlice — Zustand slice
│
├── web/                                         ← [MODIFY] Web entry point
│   ├── login/                                   ← [NEW] Login page module
│   │   ├── LoginPage.tsx                        ← Login form + SSO buttons
│   │   ├── LoginForm.tsx                        ← Email/password form
│   │   ├── SsoButton.tsx                        ← GitHub/Google/Keycloak button
│   │   ├── PairCodeFallback.tsx                 ← PairCode section (backward compat)
│   │   └── __tests__/
│   │       ├── LoginPage.test.tsx
│   │       ├── LoginForm.test.tsx
│   │       └── SsoButton.test.tsx
│   │
│   ├── main-web-bootstrap.tsx                   ← [MODIFY] Thêm auth session check
│   └── WebConnect.tsx                           ← [MODIFY] Bỏ qua nếu đã có session
│
├── components/
│   ├── auth/                                    ← [NEW] Auth UI components
│   │   ├── UserAvatarMenu.tsx                   ← Avatar dropdown (logout, profile)
│   │   ├── UserRoleBadge.tsx                    ← role indicator badge
│   │   └── __tests__/
│   │       └── UserAvatarMenu.test.tsx
│   │
│   └── admin/                                   ← [NEW] Admin SPA components
│       ├── AdminLayout.tsx                      ← Layout với sidebar navigation
│       ├── AdminDashboard.tsx                   ← Overview stats + active sessions
│       ├── UsersPage.tsx                        ← User list với search/filter
│       ├── UserForm.tsx                         ← Create/Edit user form
│       ├── PoliciesPage.tsx                     ← Access policies CRUD
│       ├── PolicyForm.tsx                       ← Create/Edit policy form
│       ├── SessionsPage.tsx                     ← Active sessions + kill controls
│       ├── AuditPage.tsx                        ← Audit log với date filter
│       ├── admin-api-client.ts                  ← fetch() wrapper cho /admin/api/*
│       └── __tests__/
│           ├── AdminDashboard.test.tsx
│           ├── UsersPage.test.tsx
│           └── admin-api-client.test.ts
│
└── hooks/
    ├── useAuthSession.ts                        ← [NEW] Hook lấy auth state từ store
    ├── useAdminStats.ts                         ← [NEW] Poll /admin/api/stats
    └── useAdminUsers.ts                         ← [NEW] Fetch + mutate users
```

---

## Schema mới trong Frontend (Auth Slice)

```typescript
// src/renderer/src/store/slices/auth.ts

type AuthUser = {
  id:       string
  email:    string
  name:     string
  role:     'developer' | 'lead' | 'admin'
  provider: 'none' | 'github' | 'google' | 'keycloak'
  avatarUrl?: string
}

type AuthState =
  | { status: 'unknown' }           // initial — đang check session
  | { status: 'unauthenticated' }   // không có session
  | { status: 'authenticated'; user: AuthUser }  // đã login
  | { status: 'error'; message: string }         // lỗi khi check

type AuthSlice = {
  auth: AuthState
  setAuth: (state: AuthState) => void
  clearAuth: () => void
  checkSession: () => Promise<void>  // GET /auth/me → update auth state
}
```

---

## Test Setup

```typescript
// Dùng happy-dom per test file
// @vitest-environment happy-dom

// Mock auth API client:
vi.mock('../../../auth/auth-api-client', () => ({
  fetchCurrentUser: vi.fn(),
  loginLocal: vi.fn(),
  logout: vi.fn(),
}))

// Mock fetch globally:
global.fetch = vi.fn()
```
