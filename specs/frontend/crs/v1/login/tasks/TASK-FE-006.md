# TASK-FE-006 — Tạo `LoginPage.tsx` + Tests

**Phase:** 1 — Auth Foundation
**Solution:** [SOL-FE-LG-001](../solutions/SOL-FE-LG-001-login-page.md) §4.5, §3.2
**Depends on:** TASK-FE-003, TASK-FE-004, TASK-FE-005
**Blocks:** TASK-FE-007
**Effort:** M (~40 phút)
**Status:** ✅ Done

---

## Mô tả

Tạo component trang Login chính — compose LoginForm + SsoButton + PairCodeFallback.
Đây là root component hiển thị cho user chưa login.

---

## Files cần tạo

### `src/renderer/src/web/login/LoginPage.tsx` [NEW]

Implement theo spec đầy đủ tại [SOL-FE-LG-001 §4.5](../solutions/SOL-FE-LG-001-login-page.md).

Props:
```typescript
type Props = {
  availableProviders: SsoProvider[]
  onLoginSuccess: (user: AuthUser) => void
}
```

Behavior:
- Render `<LoginForm>` với handler gọi `loginLocal()`
- Render `<SsoButton>` cho mỗi provider trong `availableProviders`
- Render `<PairCodeFallback>` luôn luôn (backward compat)
- `isLoading` state khi đang gọi API
- Hiển thị error khi `loginLocal()` throw

### `src/renderer/src/web/login/__tests__/LoginPage.test.tsx` [NEW]

Sao chép test spec từ [SOL-FE-LG-001 §3.2](../solutions/SOL-FE-LG-001-login-page.md).

Test cases (7 tests):
- Renders email, password, Sign In button
- Calls loginLocal on submit → onLoginSuccess
- Shows error on invalid credentials
- Disables button during loading
- Renders SSO buttons for providers
- SSO href = /auth/sso/{provider}
- No SSO section when no providers

---

## Verify

```bash
npx vitest run src/renderer/src/web/login/__tests__/LoginPage.test.tsx
# Expected: 7 pass
```
