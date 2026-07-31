# TASK-PC-003 — Set Session Environment Trong Bootstrap Khi User Đã Login

**Solution:** [SOL-PC-002 §Thay đổi 1](../solutions/SOL-PC-002-browser-session-rpc.md)  
**Bug:** [BUG-PC-001](../BUG-PC-001-browser-requires-paircode.md)  
**File:** `src/renderer/src/web/main-web-bootstrap.tsx`  
**Phụ thuộc:** TASK-PC-002 (phải có `createSessionWebRuntimeEnvironment` trước)  
**Estimated:** 20 phút  
**Status:** ✅ DONE — 2026-07-27

---

## Mục Tiêu

Khi user đã login (session cookie hợp lệ, `sessionUser !== null`), tạo và lưu `StoredWebRuntimeEnvironment` từ session context trước khi gọi `installWebPreloadApi()`. Điều này giúp `requireActiveEnvironment()` không throw và toàn bộ RPC calls hoạt động mà không cần Pair Code.

---

## Context

Đọc trước:
- `src/renderer/src/web/main-web-bootstrap.tsx` — L83-148 (WebRoot component)
- `src/renderer/src/web/web-runtime-environment.ts` — `createSessionWebRuntimeEnvironment`, `saveStoredWebRuntimeEnvironment`, `readStoredWebRuntimeEnvironment`

**Vấn đề hiện tại** (L115-124):
```tsx
// Khi sessionUser !== null (đã login):
if (sessionUser !== null) {
  installWebPreloadApi()   // ← THIẾU: không set activeEnvironment từ session
  return (
    <ConnectionStatusProvider client={client}>
      <App />
    </ConnectionStatusProvider>
  )
}
// → requireActiveEnvironment() throw → devServer.list() → catch → []
```

---

## Thay Đổi Cần Thực Hiện

### File: `src/renderer/src/web/main-web-bootstrap.tsx`

**Bước 1: Thêm import** (thêm vào block imports của web-runtime-environment, L14-17):

```tsx
import {
  createStoredWebRuntimeEnvironment,
  createSessionWebRuntimeEnvironment,   // ← THÊM
  readStoredWebRuntimeEnvironment,
  saveStoredWebRuntimeEnvironment
} from './web-runtime-environment'
```

**Bước 2: TÌM** đoạn `if (sessionUser !== null)` trong `WebRoot` component:

```tsx
  // CR-LOGIN-001: if the user is already authenticated via session cookie,
  // skip the WebConnect / pairing flow entirely and render the App directly.
  if (sessionUser !== null) {
    installWebPreloadApi()
    return (
      <ConnectionStatusProvider client={client}>
        <WebConnectionBannerWrapper />
        <Suspense fallback={<div className="min-h-dvh bg-background" />}>
          <App />
        </Suspense>
      </ConnectionStatusProvider>
    )
  }
```

**THAY BẰNG:**

```tsx
  // CR-LOGIN-001: if the user is already authenticated via session cookie,
  // skip the WebConnect / pairing flow entirely and render the App directly.
  if (sessionUser !== null) {
    // Create an implicit session-based environment so requireActiveEnvironment()
    // in web-preload-api.ts does not throw when the user has a valid login session.
    // Why: WsSessionRouter (SOL-PC-001) routes WS via cookie — no Pair Code needed.
    // Only create if no existing environment (don't overwrite a paired environment).
    if (readStoredWebRuntimeEnvironment() === null) {
      saveStoredWebRuntimeEnvironment(
        createSessionWebRuntimeEnvironment(window.location)
      )
    }
    installWebPreloadApi()
    return (
      <ConnectionStatusProvider client={client}>
        <WebConnectionBannerWrapper />
        <Suspense fallback={<div className="min-h-dvh bg-background" />}>
          <App />
        </Suspense>
      </ConnectionStatusProvider>
    )
  }
```

---

## Verify

```bash
# TypeScript compile
cd /Users/binhnt/Work/blockchain/vnp-blc/orca
npx tsc --noEmit 2>&1 | grep "main-web-bootstrap" | head -10
# Expected: không có lỗi

# Build frontend
pnpm build:frontend:web 2>&1 | tail -10
# Expected: build thành công
```

**Manual verify** (sau khi deploy):
1. Clear localStorage: `localStorage.clear()` trong Browser DevTools
2. Mở `https://b15.openledger.vn` → Login
3. Kiểm tra localStorage: `localStorage.getItem('orca.web.runtimeEnvironment.v1')`
4. Expected: JSON có `id: 'session-auth'`, `endpoints[0].endpoint: 'wss://b15.openledger.vn/ws'`

---

## Definition of Done

- [x] Import `createSessionWebRuntimeEnvironment` đã được thêm vào imports block
- [x] Khi `sessionUser !== null` và `readStoredWebRuntimeEnvironment() === null`: gọi `saveStoredWebRuntimeEnvironment(createSessionWebRuntimeEnvironment(window.location))`
- [x] Không overwrite environment đã có (pair code user không bị reset)
- [x] `installWebPreloadApi()` vẫn được gọi ngay sau
- [x] TypeScript compile OK
