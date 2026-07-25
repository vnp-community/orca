# TASK-FE-007 — Modify `main-web-bootstrap.tsx` — Auth Routing

**Phase:** 1 — Auth Foundation
**Solution:** [SOL-FE-LG-001](../solutions/SOL-FE-LG-001-login-page.md) §4.4
**Depends on:** TASK-FE-006
**Blocks:** —
**Effort:** M (~30 phút)
**Status:** ✅ Done

---

## Mô tả

Sửa `bootstrapWebApp()` để:
1. Check auth session trước khi render
2. Nếu authenticated → render App (bỏ qua Login)
3. Nếu không có session và không có pairing data → render LoginPage

**Nguyên tắc:** Không sửa `main.tsx` (desktop entry). Chỉ sửa `main-web-bootstrap.tsx`.

---

## File cần sửa

### `src/renderer/src/web/main-web-bootstrap.tsx` [MODIFY]

**Tìm:** hàm `bootstrapWebApp()`

**Thêm vào đầu hàm (sau các imports hiện tại):**

```typescript
import { fetchCurrentUser, fetchAuthConfig } from '../auth/auth-api-client'
import { LoginPage } from './login/LoginPage'
import type { SsoProvider } from '../auth/auth-types'
```

**Thêm logic auth check vào `bootstrapWebApp()` trước bước render:**

```typescript
// [NEW] Bước 3: Check auth session
const [currentUser, authConfig] = await Promise.all([
  fetchCurrentUser().catch(() => null),
  fetchAuthConfig().catch(() => ({ providers: [], localEnabled: true }))
])

// [NEW] Bước 4: Route based on auth + pairing state
const savedEnv = readStoredWebRuntimeEnvironment()
const hasPairingInput = new URLSearchParams(location.search).get('pair')

if (currentUser) {
  renderApp({})
} else if (savedEnv || hasPairingInput) {
  renderApp({})   // backward compat: pairing without login
} else {
  renderLoginPage({ authConfig })
  return   // early return — don't proceed to renderApp
}
```

**Thêm helper function `renderLoginPage()`:**

```typescript
function renderLoginPage(props: { authConfig: { providers: string[]; localEnabled: boolean } }): void {
  const root = createRoot(document.getElementById('root')!)
  root.render(
    <StrictMode>
      <LoginPage
        availableProviders={props.authConfig.providers as SsoProvider[]}
        onLoginSuccess={() => { window.location.href = '/' }}
      />
    </StrictMode>
  )
}
```

---

## Constraints

- KHÔNG sửa `renderApp()` function signature nếu đã có
- KHÔNG sửa `src/renderer/src/web/main.tsx`
- KHÔNG sửa `src/renderer/src/App.tsx`

---

## Verify

```bash
# TypeScript check
npx tsc --noEmit
# Manual: mở browser → truy cập / mà không có session → phải thấy LoginPage
```
