# BUG-BE-AUTH-002: `auth-router.ts` dùng `SameSite: 'lax'` thay vì `SameSite: 'strict'` như HLD quy định

**Status:** ✅ FIXED — 2026-08-01  
**Fixed by:** TASK-AUTH-001  
**Implementation:** auth-router.ts: SameSite=strict  

## Mức độ: 🟡 MEDIUM

## Tóm tắt

HLD (BL-AUTH-01) quy định session cookie:
```
Set-Cookie: orca_session=<token>; HttpOnly; Secure; SameSite=Strict
```

Nhưng code thực tế dùng `SameSite: 'lax'`:

```typescript
// auth-router.ts:21-27
const COOKIE_OPTIONS = {
  httpOnly: true,
  sameSite: 'lax' as const,   // ← HLD yêu cầu 'strict'
  secure: process.env['NODE_ENV'] === 'production',
  path: '/',
  maxAge: 8 * 60 * 60 * 1000
}
```

## File liên quan

- [`src/main/auth/auth-router.ts`](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/src/main/auth/auth-router.ts) — Line 23

## Ảnh hưởng

1. **Security downgrade**: `SameSite=Lax` cho phép cookie được gửi khi user navigate đến Orca từ external link (top-level GET navigation) — `SameSite=Strict` không cho phép điều này.
2. CSRF risk tăng khi dùng `lax` thay vì `strict` — attacker có thể craft link dẫn user navigate đến một GET endpoint của Orca.
3. **Low severity vì** `SameSite=Lax` vẫn protect POST/PUT/DELETE requests — nhưng diverges từ HLD security model.

## Cách fix đề xuất

```typescript
const COOKIE_OPTIONS = {
  httpOnly: true,
  sameSite: 'strict' as const,  // Đúng theo HLD
  secure: process.env['NODE_ENV'] === 'production',
  path: '/',
  maxAge: 8 * 60 * 60 * 1000
}
```

**Lưu ý**: `SameSite=Strict` có thể break OAuth redirect flows nếu Orca dùng external OAuth (GitHub/GitLab login). Cần test kỹ trước khi thay đổi.

## Liên quan đến luồng

- **BL-AUTH-01**: Set-Cookie security attributes.
