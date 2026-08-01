# BUG-AUTH-002: `auth-router.ts` — Cookie options dùng `sameSite: 'lax'` thay vì `'strict'` như HLD mô tả

**Status:** ✅ FIXED — 2026-08-01  
**Fixed by:** TASK-AUTH-001  
**Implementation:** auth-router.ts: SameSite=strict  

## Mức độ: 🟡 MEDIUM

## Tóm tắt

HLD BL-AUTH-01:
```
Set-Cookie: orca_session=<token>; HttpOnly; Secure; SameSite=Strict
```

Thực tế `src/main/auth/auth-router.ts:21-27`:
```typescript
const COOKIE_OPTIONS = {
  httpOnly: true,
  sameSite: 'lax' as const,    ← 'lax' chứ không phải 'strict'
  secure: process.env['NODE_ENV'] === 'production',
  path: '/',
  maxAge: 8 * 60 * 60 * 1000
}
```

`SameSite=Lax` cho phép cookie được gửi khi người dùng navigate từ external site đến Orca (top-level GET navigation). Với `SameSite=Strict`, cookie chỉ được gửi khi request xuất phát từ cùng site.

Mức độ CSRF risk: `SameSite=Lax` giảm CSRF risk nhưng không bằng `Strict`. Với Orca là admin tool, `Strict` là phù hợp hơn.

## Thêm: `secure: process.env['NODE_ENV'] === 'production'`

Nếu deployment không set `NODE_ENV=production` (ví dụ staging, docker dev) → cookie **không có Secure flag** → có thể bị gửi qua HTTP.

## Fix đề xuất

```typescript
const COOKIE_OPTIONS = {
  httpOnly: true,
  sameSite: 'strict' as const,  // stricter CSRF protection
  secure: true,                  // always require HTTPS (hoặc check ORCA_FORCE_HTTPS env)
  path: '/',
  maxAge: 8 * 60 * 60 * 1000
}
```

## Files liên quan

- `src/main/auth/auth-router.ts:21-27`: cookie options
