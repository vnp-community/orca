# TDD-FE-02: Auth Flow

**Version:** 4.0  
**Date:** 2026-07-28  
**Source:** `src/renderer/src/auth/`, `src/renderer/src/web/login/`, `src/renderer/src/store/slices/auth.ts`

---

## 1. Boot Sequence (Auth-first)

```
bootstrapWebApp()
  │
  ├─ 1. Register Service Worker (Push)
  │
  ├─ 2. checkAuthSession()
  │   ├─ GET /auth/me (with credentials: 'include')
  │   ├─ OK (200) → { user } → store.auth.setAuth(user)
  │   │   └─ Render App.tsx (full UI)
  │   └─ Error (401/403)
  │       ├─ Render LoginPage
  │       └─ After login success → Re-render App.tsx
  │
  └─ 3. Mount ConnectionStatusProvider
      └─ ConnectionStatusBanner (fixed overlay khi mất kết nối)
```

---

## 2. Auth Types

```typescript
// src/renderer/src/auth/auth-types.ts

export type AuthUser = {
  id:       string
  email:    string
  name:     string
  role:     'admin' | 'user' | 'viewer'
  provider: 'local' | 'github' | 'google' | 'keycloak'
}

export type AuthState = {
  user:   AuthUser | null
  status: 'loading' | 'authenticated' | 'unauthenticated' | 'error'
  error?: string
}

export type SsoProvider = 'github' | 'google' | 'keycloak'
```

---

## 3. Auth API Client

```typescript
// src/renderer/src/auth/auth-api-client.ts

export async function fetchCurrentUser(): Promise<AuthUser | null>
// GET /auth/me → AuthUser | null (nếu 401 → null)

export async function loginLocal(email: string, password: string): Promise<{ success: true; user: AuthUser } | { success: false; error: string }>
// POST /auth/local

export async function logoutUser(): Promise<void>
// POST /auth/logout

export async function fetchAuthConfig(): Promise<{ ssoProviders: SsoProvider[] }>
// GET /auth/config (trả về SSO providers configured)
```

---

## 4. AuthSlice (Zustand)

```typescript
// src/renderer/src/store/slices/auth.ts

export type AuthSlice = {
  auth: AuthState

  // Actions
  setAuth(user: AuthUser): void
  clearAuth(): void
  checkSession(): Promise<void>   // calls fetchCurrentUser() → setAuth/clearAuth
}

export function createAuthSlice(set, get): AuthSlice {
  return {
    auth: { user: null, status: 'loading' },

    setAuth: (user) => set((s) => ({ auth: { user, status: 'authenticated' } })),
    clearAuth: () => set((s) => ({ auth: { user: null, status: 'unauthenticated' } })),
    checkSession: async () => {
      const user = await fetchCurrentUser()
      if (user) get().setAuth(user)
      else      get().clearAuth()
    }
  }
}
```

---

## 5. LoginPage

```tsx
// src/renderer/src/web/login/LoginPage.tsx

function LoginPage({ onLoginSuccess }: { onLoginSuccess: (user: AuthUser) => void }) {
  // Hiển thị:
  // 1. LoginForm (email/password)
  // 2. SsoButton(s) (nếu có SSO providers)
  // 3. PairCodeFallback (backward compat — pair code UI)

  // Sau login thành công → gọi onLoginSuccess(user)
}
```

---

## 6. LoginForm

```tsx
function LoginForm({ onSuccess }: { onSuccess: (user: AuthUser) => void }) {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)

  const handleSubmit = async (e: FormEvent) => {
    const result = await loginLocal(email, password)
    if (result.success) {
      store.setAuth(result.user)
      onSuccess(result.user)
    } else {
      setError(result.error)
    }
  }
}
```

---

## 7. SsoButton

```tsx
// src/renderer/src/web/login/SsoButton.tsx
function SsoButton({ provider }: { provider: SsoProvider }) {
  // href → /auth/sso/:provider (redirect-based OAuth2)
  // Supported: 'github' | 'google' | 'keycloak'
}
```

---

## 8. Auth Hooks

```typescript
// src/renderer/src/hooks/useAuthSession.ts

export function useAuthUser(): AuthUser | null
export function useIsAuthenticated(): boolean
export function useAuthStatus(): AuthState['status']

// src/renderer/src/hooks/useLogout.ts
export function useLogout(): () => Promise<void>
// Calls logoutUser() → clearAuth() → redirect to LoginPage
```

---

## 9. UserAvatarMenu (Web-only)

```tsx
// src/renderer/src/components/auth/UserAvatarMenu.tsx
// Hiển thị trong OrcaProfileSwitcher (additive — không sửa core)
// Chỉ render khi ORCA_PLATFORM === 'web' && isAuthenticated

function UserAvatarMenu() {
  const user = useAuthUser()
  // Dropdown:
  //   - Avatar + email display
  //   - Role badge
  //   - Admin Panel link (nếu role === 'admin')
  //   - Logout button
}
```

---

## 10. PairCodeFallback (Backward Compat)

```tsx
// src/renderer/src/web/login/PairCodeFallback.tsx
// Khi server không có auth (single-user mode không cần login)
// Hiển thị pair code form (phiên bản cũ)
// Chỉ show nếu /auth/me trả về 404 (endpoint không tồn tại = no-auth mode)
```

---

## 11. Tests (47 tests)

| File | Tests |
|------|-------|
| `auth/__tests__/auth-api-client.test.ts` | 10 |
| `auth/__tests__/auth-utils.test.ts` | 4 |
| `web/login/__tests__/LoginPage.test.tsx` | 8 |
| `web/login/__tests__/LoginForm.test.tsx` | 4 |
| `web/login/__tests__/SsoButton.test.tsx` | 4 |
| `hooks/__tests__/useAuthSession.test.ts` | 6 |
| `hooks/__tests__/useLogout.test.ts` | 3 |
| `components/auth/__tests__/UserAvatarMenu.test.tsx` | 8 |
