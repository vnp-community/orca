# TASK-FE-009 — Tạo `useAuthSession.ts` + `useLogout.ts` + Tests

**Phase:** 2 — User Identity
**Solution:** [SOL-FE-LG-002](../solutions/SOL-FE-LG-002-user-identity.md) §4.1, §4.2, §3.3
**Depends on:** TASK-FE-003 (AuthSlice phải đã có trong store)
**Blocks:** TASK-FE-011, TASK-FE-012, TASK-FE-014
**Effort:** S (~25 phút)
**Status:** ✅ Done

---

## Mô tả

Tạo 2 custom hooks cho auth:
- `useAuthSession()` — đọc `auth` state từ Zustand store
- `useAuthUser()` — shortcut trả về `AuthUser | null`
- `useLogout()` — action: POST /auth/logout → clear store → redirect

---

## Files cần tạo

### `src/renderer/src/hooks/useAuthSession.ts` [NEW]

```typescript
import { useAppStore } from '../store'
import { AuthState } from '../auth/auth-types'

/** Đọc toàn bộ auth state */
export function useAuthSession(): AuthState {
  return useAppStore(s => s.auth)
}

/** Shortcut: trả về user nếu authenticated, null nếu không */
export function useAuthUser() {
  const auth = useAuthSession()
  return auth.status === 'authenticated' ? auth.user : null
}
```

### `src/renderer/src/hooks/useLogout.ts` [NEW]

```typescript
import { useCallback } from 'react'
import { useAppStore } from '../store'
import { logoutUser } from '../auth/auth-api-client'

export function useLogout() {
  const clearAuth = useAppStore(s => s.clearAuth)

  return useCallback(async () => {
    await logoutUser()
    clearAuth()
    window.location.href = '/login'
  }, [clearAuth])
}
```

### `src/renderer/src/hooks/__tests__/useAuthSession.test.ts` [NEW]

Sao chép test spec từ [SOL-FE-LG-002 §3.3](../solutions/SOL-FE-LG-002-user-identity.md).

Test cases (3 tests):
- Returns authenticated state khi store có user
- Returns unknown state initially
- Returns unauthenticated khi không có session

### `src/renderer/src/hooks/__tests__/useLogout.test.ts` [NEW]

Test cases (2 tests):
- Calls logoutUser API + clearAuth khi invoke
- Redirects to /login sau logout

---

## Verify

```bash
npx vitest run src/renderer/src/hooks/__tests__/useAuthSession.test.ts
# Expected: 3 pass
npx vitest run src/renderer/src/hooks/__tests__/useLogout.test.ts
# Expected: 2 pass
```
