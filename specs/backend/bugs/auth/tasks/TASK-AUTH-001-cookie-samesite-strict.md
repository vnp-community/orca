# TASK-AUTH-001: Fix Cookie SameSite từ Lax → Strict

**Priority:** 🟠 HIGH — CSRF vulnerability  
**Effort:** ~5 phút  
**Status:** ✅ DONE  
**Bug refs:** BUG-AUTH-002, BUG-BE-AUTH-002  
**Solution ref:** [SOLUTION-auth-exact.md](../solutions/SOLUTION-auth-exact.md)

---

## Mục tiêu

Thay `sameSite: 'lax'` thành `sameSite: 'strict'` trong session cookie options để ngăn CSRF.

## File cần sửa

```
src/main/auth/auth-router.ts
```

## Thay đổi cụ thể

### Line 23 — Một thay đổi duy nhất:

```diff
 const COOKIE_OPTIONS = {
   httpOnly: true,
-  sameSite: 'lax' as const,
+  sameSite: 'strict' as const,
   secure: process.env['NODE_ENV'] === 'production',
   path: '/',
   maxAge: 8 * 60 * 60 * 1000  // 8 hours in ms (matches SESSION_TTL_MS)
 }
```

## Note về OAuth compatibility

`SameSite: strict` có thể break OAuth callback redirect trong tương lai (browser không gửi cookie trong cross-site redirect từ OAuth provider). Nếu OAuth được thêm sau này:
- Session cookie giữ `strict`
- Thêm riêng PKCE state cookie với `SameSite: lax` cho OAuth flow

## Verification

```bash
pnpm tsc --noEmit

# Verify:
grep -n "sameSite" src/main/auth/auth-router.ts
# Expected: sameSite: 'strict' as const

# Test: login → check Set-Cookie header trong response
curl -v -X POST http://localhost:6768/auth/local \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"password"}' 2>&1 | grep "Set-Cookie"
# Expected: Set-Cookie: orca_session=...; SameSite=Strict; HttpOnly; ...
```
