# TASK-FE-001 — Tạo `auth-types.ts`

**Phase:** 1 — Auth Foundation
**Solution:** [SOL-FE-LG-001](../solutions/SOL-FE-LG-001-login-page.md) §4.1
**Depends on:** —
**Blocks:** TASK-FE-002, TASK-FE-004, TASK-FE-005, TASK-FE-021
**Effort:** XS (~15 phút)
**Status:** ✅ Done

---

## Mô tả

Tạo file type definitions cho auth layer frontend. Đây là foundation cho toàn bộ Phase 1–4.

---

## File cần tạo

### `src/renderer/src/auth/auth-types.ts` [NEW]

```typescript
export type SsoProvider = 'github' | 'google' | 'keycloak'

export type AuthUser = {
  id:        string
  email:     string
  name:      string
  role:      'developer' | 'lead' | 'admin'
  provider:  'none' | SsoProvider
  avatarUrl?: string
}

export type AuthState =
  | { status: 'unknown' }
  | { status: 'unauthenticated' }
  | { status: 'authenticated'; user: AuthUser }
  | { status: 'error'; message: string }

export class AuthError extends Error {
  constructor(message: string, public code: string) {
    super(message)
    this.name = 'AuthError'
  }
}
```

---

## Verify

```bash
# TypeScript compile check
npx tsc --noEmit src/renderer/src/auth/auth-types.ts
```

Không có test riêng — types được verify qua TASK-FE-002 tests.
