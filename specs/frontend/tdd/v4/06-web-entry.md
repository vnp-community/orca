# TDD-FE-06: Web Entry Point

**Version:** 4.0  
**Date:** 2026-07-28  
**Source:** `src/renderer/src/web/main-web-bootstrap.tsx`

---

## 1. bootstrapWebApp()

**Cập nhật 2026-08-09:** mục này trước đây mô tả `bootstrapWebApp()` tự quyết định "no-auth mode" bên trong nó (`checkNoAuthMode()` → `renderPairCodeFallback()`). **Không khớp code thật** — quyết định đó xảy ra ở tầng NGOÀI `bootstrapWebApp()`, trong `main.tsx`, TRƯỚC KHI hàm này được gọi (xem §6, §7 đã viết lại). Phát hiện qua audit `specs/frontend/crs/frontend-e2ee/solutions/SOL-FE2E-001-scope-and-discovery-audit.md` mục 4.

`bootstrapWebApp()` thật (chỉ chạy trong nhánh `/auth/config` → 200) không có nhánh no-auth — nó luôn kết thúc bằng `<LoginPage>` (không có `PairCodeFallback` bên trong nữa, xem [CR-FE2E-002](../../../../docs/crs/v2/frontend-e2ee/CR-FE2E-002-remove-paircode-fallback-from-login.md)) hoặc `<App>`:

```typescript
// src/renderer/src/web/main-web-bootstrap.tsx
export async function bootstrapWebApp(options: BootstrapOptions = {}): Promise<void> {
  // 1. Tạo WebSocketRpcClient nhẹ (chỉ cho ConnectionStatusProvider), retry connect
  // 2. installAuthFailedRedirect() — lắng nghe 'orca:auth-failed' (WS đóng mã 4401)
  // 3. Mount <WebRootBoundary client={client} /> — component này tự fetch
  //    /auth/me + /auth/config để quyết định LoginPage hay App, KHÔNG có
  //    nhánh no-auth/pair-code nào (đã bỏ ở CR-FE2E-002)
}
```

---

## 2. checkAuthSession()

```typescript
async function checkAuthSession(): Promise<AuthUser | null> {
  try {
    const user = await fetchCurrentUser()  // GET /auth/me
    if (user) {
      store.setAuth(user)
      return user
    }
    return null
  } catch (err) {
    // Network error → null (will show LoginPage)
    // 404 (endpoint doesn't exist = no-auth mode) → null + flag
    return null
  }
}
```

---

## 3. renderLoginPage()

```typescript
function renderLoginPage(): void {
  const root = createRoot(document.getElementById('root')!)
  root.render(
    <StrictMode>
      <LoginPage
        onLoginSuccess={(user) => {
          // After successful login → unmount LoginPage, render App
          root.unmount()
          renderApp(user)
        }}
      />
    </StrictMode>
  )
}
```

---

## 4. renderApp()

```typescript
function renderApp(user: AuthUser): void {
  const root = createRoot(document.getElementById('root')!)
  root.render(
    <StrictMode>
      <WebSocketRpcClientProvider url={getRpcUrl()}>
        <ConnectionStatusProvider>
          <App />
          <ConnectionStatusBanner />
        </ConnectionStatusProvider>
      </WebSocketRpcClientProvider>
    </StrictMode>
  )
}
```

---

## 5. getRpcUrl()

```typescript
function getRpcUrl(): string {
  // Web mode: ws(s)://<current-host>:<rpcPort>
  const protocol = location.protocol === 'https:' ? 'wss' : 'ws'
  const host = location.hostname
  const port = parseInt(location.port) || 6769  // default httpPort
  return `${protocol}://${host}:${port}`
}
```

---

## 6. main.tsx (Web) — cập nhật 2026-08-09 ([CR-FE2E-003](../../../../docs/crs/v2/frontend-e2ee/CR-FE2E-003-lazy-split-pairing-bundle.md))

`main.tsx` **KHÔNG** chỉ delegate sang `bootstrapWebApp()` — nó là nơi thật sự quyết định nhánh nào chạy, bằng cách probe `/auth/config` (không phải `/auth/me` như §7 cũ mô tả):

```typescript
// src/renderer/src/web/main.tsx (thật)
import '../assets/main.css'
import { bootstrapWebApp } from './main-web-bootstrap'

fetch('/auth/config')
  .then((res) => {
    if (res.ok) void bootstrapWebApp()        // multi-user backend (backend/) — session-auth
    else renderOriginalPairCodeApp()           // Desktop Pair Code sharing — không có backend
  })
  .catch(() => renderOriginalPairCodeApp())    // lỗi mạng → coi như Desktop mode

// Chỉ /auth/config 404 mới tới đây — dynamic import giữ TweetNaCl/E2EE pairing UI
// ngoài bundle mà mọi browser multi-user tải (CR-FE2E-003).
function renderOriginalPairCodeApp(): void {
  void import('./pair-code-app-entry').then(({ mountPairCodeApp }) => mountPairCodeApp())
}
```

`pair-code-app-entry.tsx` (file mới, CR-FE2E-003) chứa `WebRoot`/`WebRootBoundary` — logic pairing đầy đủ (`WebConnect`, `web-pairing.ts`, `web-runtime-environment.ts`) — di chuyển nguyên vẹn từ `main.tsx` cũ.

---

## 7. Phân biệt 2 nhánh (thay cho "No-Auth Mode Detection" cũ — hàm `checkNoAuthMode()` không tồn tại trong code)

Doc trước đây mô tả 1 hàm `checkNoAuthMode()` kiểm tra `/auth/me` trả 404 để suy ra "no-auth mode". **Không có hàm này trong code.** Cơ chế thật (xem §6): `main.tsx` probe **`/auth/config`** (không phải `/auth/me`), và route thẳng tới 1 trong 2 hàm mount hoàn toàn tách biệt (`bootstrapWebApp()` vs `renderOriginalPairCodeApp()`) — không có khái niệm "no-auth mode" như 1 cờ boolean truyền vào 1 hàm dùng chung.

Backend (`backend/src/server/http-server.ts`) chỉ mount `/auth/*` (gồm cả `/auth/config` lẫn `/auth/local`) khi `options.authManager` tồn tại — 2 route này **luôn cùng có hoặc cùng không có**, không có deployment nào tách rời được (xác nhận: `specs/frontend/crs/frontend-e2ee/solutions/SOL-FE2E-001-scope-and-discovery-audit.md` mục 1).

---

## 8. Error Boundary

```tsx
// bootstrapWebApp() wrapped trong RecoverableErrorBoundary
// Nếu App.tsx crash → show error message + "Reload" button
// Không crash toàn bộ browser tab
```

---

## 9. ORCA_PLATFORM Flag

```typescript
// Set at build time bởi Vite define config
declare global {
  const ORCA_PLATFORM: 'desktop' | 'web'
}

// Web SPA build:
// vite.web-spa.config.ts: define: { ORCA_PLATFORM: '"web"' }

// Dùng để ẩn/hiện web-only UI:
if (ORCA_PLATFORM === 'web') {
  // Show UserAvatarMenu, LoginPage, ConnectionStatusBanner
}
```
