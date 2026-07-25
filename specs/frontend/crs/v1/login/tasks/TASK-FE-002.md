# TASK-FE-002 — Tạo `auth-api-client.ts` + Tests

**Phase:** 1 — Auth Foundation
**Solution:** [SOL-FE-LG-001](../solutions/SOL-FE-LG-001-login-page.md) §4.2, §3.1
**Depends on:** TASK-FE-001
**Blocks:** TASK-FE-003, TASK-FE-009
**Effort:** S (~30 phút)
**Status:** ✅ Done

---

## Mô tả

Tạo HTTP client wrapper cho các auth endpoints của backend. Sử dụng `fetch()` với `credentials: 'include'`.

---

## Files cần tạo

### `src/renderer/src/auth/auth-api-client.ts` [NEW]

```typescript
import { AuthError, AuthUser } from './auth-types'

/** GET /auth/me — returns null if no session (401) */
export async function fetchCurrentUser(): Promise<AuthUser | null> {
  const res = await fetch('/auth/me', { credentials: 'include' })
  if (res.status === 401) return null
  if (!res.ok) throw new Error(`Server error: ${res.status}`)
  return res.json() as Promise<AuthUser>
}

/** POST /auth/local — email+password login */
export async function loginLocal(email: string, password: string): Promise<AuthUser> {
  const res = await fetch('/auth/local', {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password })
  })
  const body = await res.json()
  if (!res.ok) {
    throw new AuthError(body.error ?? 'Login failed', 'invalid_credentials')
  }
  return body as AuthUser
}

/** POST /auth/logout */
export async function logoutUser(): Promise<void> {
  await fetch('/auth/logout', { method: 'POST', credentials: 'include' })
}

/** GET /auth/config — list of enabled SSO providers */
export async function fetchAuthConfig(): Promise<{ providers: string[]; localEnabled: boolean }> {
  const res = await fetch('/auth/config', { credentials: 'include' })
  if (!res.ok) return { providers: [], localEnabled: true }
  return res.json()
}
```

### `src/renderer/src/auth/__tests__/auth-api-client.test.ts` [NEW]

Sao chép nội dung test spec từ [SOL-FE-LG-001 §3.1](../solutions/SOL-FE-LG-001-login-page.md).

Test cases (6 tests):
- `fetchCurrentUser`: returns AuthUser khi 200, null khi 401, throw khi network error
- `loginLocal`: returns AuthUser khi thành công, throw AuthError khi 401, throw khi 500
- `logoutUser`: call POST /auth/logout với credentials

---

## Verify

```bash
npx vitest run src/renderer/src/auth/__tests__/auth-api-client.test.ts
# Expected: 6 pass
```
