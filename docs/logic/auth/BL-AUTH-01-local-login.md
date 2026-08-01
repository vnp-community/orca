# BL-AUTH-01: Local Login (email + password)

**Domain:** Authentication & User Management  
**Priority:** P0  
**Actor chính:** Mọi user khi dùng Orca Web Server  
**Tham chiếu:** FR-11.1, UR-110, F23

---

## Mô tả

Người dùng đăng nhập vào Orca Web Server bằng email và password. Sau khi xác thực thành công, hệ thống tạo session và trả về HTTP-only cookie.

## Preconditions

- `ORCA_MULTI_USER=1` trong environment
- Database đã apply migration 0005 (`orca_users` table tồn tại)
- User account tồn tại và không bị deactivated (`is_active=true`)

## Flow chính

```
1. User nhập email + password vào LoginForm tại /login
2. POST /auth/local { email, password }
3. Validate input (Zod schema: email format, password min 8 chars)
4. Lookup user trong orca_users WHERE email = ? AND is_active = 1
5. Verify password: bcrypt.compare(password, user.password_hash)
6. Tạo session: INSERT INTO orca_sessions (id, userId, expires_at)
7. Set-Cookie: orca_session=<token>; HttpOnly; SameSite=Lax; Path=/
8. Return 200 { id, email, name, role }
```

## Điều kiện lỗi

| Tình huống | HTTP Code | Response |
|-----------|-----------|----------|
| Email không tồn tại | 401 | `{ error: "invalid_credentials" }` |
| Password sai | 401 | `{ error: "invalid_credentials" }` |
| Account deactivated | 403 | `{ error: "account_inactive" }` |
| ORCA_MULTI_USER=0 | 404 | — |
| Rate limit exceeded | 429 | `{ error: "too_many_attempts" }` |

## Postconditions

- Session record trong `orca_sessions`
- Cookie set trên browser
- `orca_audit_log` entry: `{ action: "login.success", actorId: userId }`

## Security Notes

- Không tiết lộ "email không tồn tại" vs "password sai" (cùng 401)
- Rate limiting: tối đa 10 lần/phút per IP
- bcrypt: 12 rounds, ~300ms/verify (timing-safe)

## Source References

- `src/main/auth/auth-router.ts` — route handler
- `src/main/auth/auth-manager.ts` — verify logic
- `src/main/auth/auth-session-store.ts` — session CRUD
- `src/renderer/src/web/LoginPage.tsx` — frontend form
