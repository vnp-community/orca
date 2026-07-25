# TASK-002: Tạo file `src/main/auth/auth-types.ts`

> **Status:** ✅ DONE (2026-07-24)

**Phase:** 1 — Auth Foundation
**Solution:** [SOL-LG-001](../solutions/SOL-LG-001-auth-session.md) §4.1
**Depends on:** TASK-001
**Blocks:** TASK-003, TASK-004, TASK-007, TASK-008

---

## Mục tiêu

Tạo mới file `src/main/auth/auth-types.ts` chứa toàn bộ TypeScript types cho Auth subsystem.

---

## File cần tạo

**Path:** `src/main/auth/auth-types.ts`

---

## Nội dung

```typescript
// src/main/auth/auth-types.ts
import type { OrcaUser } from '../../shared/rbac-types'

export type OrcaSessionUser = Pick<OrcaUser, 'id' | 'email' | 'name' | 'role' | 'provider'>

export type OrcaSession = {
  sessionId:   string    // 64-hex (32 random bytes)
  userId:      string
  userEmail:   string
  role:        OrcaUser['role']
  createdAt:   number
  expiresAt:   number    // createdAt + SESSION_TTL_MS
  lastSeenAt:  number | null
  ipAddress:   string | null
  userAgent:   string | null
}

export type CreateSessionInput = {
  userId:    string
  userEmail: string
  role:      OrcaUser['role']
  ipAddress: string
  userAgent: string
}

export type LocalUserInput = {
  email:    string
  name:     string
  password: string
  role:     OrcaUser['role']
}

export type SsoUserInput = {
  email:          string
  name:           string
  provider:       'github' | 'google' | 'keycloak'
  providerUserId: string
  avatarUrl?:     string
}

export type LocalLoginInput  = { email: string; password: string }
export type LocalLoginResult =
  | { success: true;  sessionId: string; user: OrcaSessionUser }
  | { success: false; error: 'invalid_credentials' | 'account_disabled' | 'validation_error'; detail?: string }

export const SESSION_TTL_MS = 8 * 60 * 60 * 1000  // 8 giờ
```

---

## Acceptance Criteria

- [x] File `src/main/auth/auth-types.ts` tồn tại
- [x] Export đủ: `OrcaSessionUser`, `OrcaSession`, `CreateSessionInput`, `LocalUserInput`, `SsoUserInput`, `LocalLoginInput`, `LocalLoginResult`, `SESSION_TTL_MS`
- [x] TypeScript compile không lỗi
- [x] `OrcaUser` được import từ `../../shared/rbac-types` (không duplicate type)
