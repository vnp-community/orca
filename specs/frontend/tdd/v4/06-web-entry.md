# TDD-FE-06: Web Entry Point

**Version:** 4.0  
**Date:** 2026-07-28  
**Source:** `src/renderer/src/web/main-web-bootstrap.tsx`

---

## 1. bootstrapWebApp()

```typescript
// src/renderer/src/web/main-web-bootstrap.tsx

export async function bootstrapWebApp(): Promise<void> {
  // Step 1: Service Worker registration (Web Push)
  if ('serviceWorker' in navigator) {
    await navigator.serviceWorker.register('/service-worker.js')
  }

  // Step 2: Auth check
  const user = await checkAuthSession()

  if (user) {
    // Authenticated → render App.tsx
    renderApp(user)
  } else {
    // Check if no-auth mode (pair code legacy)
    const isNoAuth = await checkNoAuthMode()
    if (isNoAuth) {
      renderPairCodeFallback()
    } else {
      renderLoginPage()
    }
  }
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

## 6. main.tsx (Web)

```typescript
// src/renderer/src/web/main.tsx — CHỈ ĐỌC, KHÔNG SỬA
// Delegates tất cả logic sang bootstrapWebApp()

import { bootstrapWebApp } from './main-web-bootstrap'
bootstrapWebApp().catch(console.error)
```

---

## 7. No-Auth Mode Detection

```typescript
// checkNoAuthMode() — kiểm tra xem server có auth endpoint không
async function checkNoAuthMode(): Promise<boolean> {
  const res = await fetch('/auth/me', { credentials: 'include' })
  if (res.status === 404) return true    // no-auth mode (old server)
  return false
}
```

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
