# SOL-PC-002 — Browser Session RPC: Kết Nối Không Cần Pair Code

**Fixes:** [BUG-PC-001](../BUG-PC-001-browser-requires-paircode.md), [BUG-PC-003](../BUG-PC-003-devserver-list-silent-fail.md)  
**TDD Ref:** TDD-11 §Addendum v4.0 "Multi-User WebSocket Routing"  
**Files:**
- `src/renderer/src/web/main-web-bootstrap.tsx`
- `src/renderer/src/web/web-preload-api.ts`
- `src/renderer/src/hooks/useDevServersSync.ts`

**Effort:** ~2 giờ  
**Status:** ✅ DONE — 2026-07-27  
**Prerequisite:** SOL-PC-001 phải được deploy trước  
**Implemented in:**
- `web-runtime-environment.ts` L134-164 — `createSessionWebRuntimeEnvironment()` mới
- `main-web-bootstrap.tsx` L15, L119-127 — import + set session env khi login
- `web-preload-api.ts` L108, L150, L3187-3213 — `WebSocketRpcClient` import + `getClientForEnvironment` phân nánh
- `web-preload-api.ts` L843-861 — `listWithStatus` try/catch có log rõ ràng
- `useDevServersSync.ts` L20-28 — `.catch()` handler cho initial load

---

## Phân Tích Chi Tiết

### Root Cause Thực Sự Của BUG-PC-001

Đọc `main-web-bootstrap.tsx` L115-124:

```tsx
// Khi user đã có session cookie (đã login)
if (sessionUser !== null) {
  installWebPreloadApi()   // ← set window.api (dùng activeEnvironment)
  return (
    <ConnectionStatusProvider client={client}>
      <App />              // ← App render, gọi devServer.list()
    </ConnectionStatusProvider>
  )
}
```

Vấn đề: `installWebPreloadApi()` gọi `readStoredWebRuntimeEnvironment()` — đọc từ localStorage. **Không set `activeEnvironment` từ session**.

Kết quả:
- `activeEnvironment = null` (localStorage trống vì chưa pair)
- `requireActiveEnvironment()` throw
- `devServer.list()` → `.catch(() => [])` → `[]` → UI trống

### `WebSocketRpcClient` — Đây Là Client Dùng Cho Session Mode

```tsx
// main-web-bootstrap.tsx L214
const client = new WebSocketRpcClient(wsUrl)
// → Kết nối đến wsUrl (mặc định: ws://same-host/ws)
// → Dùng session cookie, không cần pair code!
```

`client` này **đã hoạt động** (bootstrapWebApp kết nối thành công). Nhưng `installWebPreloadApi()` không dùng `client` này — nó tạo client riêng từ `activeEnvironment` (pair code path).

### Điểm Can Thiệp Tốt Nhất

Trong `main-web-bootstrap.tsx` L115, khi `sessionUser !== null`:
1. Tạo `StoredWebRuntimeEnvironment` đặc biệt cho session mode (không cần pair code)
2. Dùng `WebSocketRpcClient` với current origin URL để kết nối qua cookie

---

## Giải Pháp

### Thay đổi 1: `main-web-bootstrap.tsx` — Set session environment khi đã login

Thêm helper function để tạo "session environment" từ current origin, sau đó set `activeEnvironment` trước khi `installWebPreloadApi()`:

```tsx
// main-web-bootstrap.tsx — THÊM: helper tạo session environment

import { createSessionWebRuntimeEnvironment } from './web-runtime-environment'  // xem thay đổi 2

// Trong WebRoot component, khi sessionUser !== null:
if (sessionUser !== null) {
  // Tạo implicit environment dựa trên current origin (không cần pair code)
  // Why: WsSessionRouter (SOL-PC-001) route WS connections qua cookie session.
  //      Tạo environment pointing đến ws://same-host/ws để tận dụng session auth.
  const sessionEnv = createSessionWebRuntimeEnvironment(window.location)
  saveStoredWebRuntimeEnvironment(sessionEnv)  // persist để reload không mất
  
  installWebPreloadApi()  // đọc environment từ localStorage → sessionEnv
  
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

### Thay đổi 2: `web-runtime-environment.ts` — Thêm `createSessionWebRuntimeEnvironment`

```typescript
// src/renderer/src/web/web-runtime-environment.ts — THÊM function

/**
 * Tạo một WebRuntimeEnvironment cho session-based auth (không cần pair code).
 *
 * Khi ORCA_MULTI_USER=1 và user đã login qua /auth/local:
 * - WsSessionRouter route WS connections đến per-user process qua cookie
 * - Không cần E2EE pair code — session cookie là auth
 * - wsUrl trỏ đến ws(s)://same-host/ws
 *
 * Why: cần một StoredWebRuntimeEnvironment để `requireActiveEnvironment()` 
 * không throw, nhưng không dùng pair code / E2EE offer.
 */
export function createSessionWebRuntimeEnvironment(location: Location): StoredWebRuntimeEnvironment {
  // Tính WS URL từ current origin
  // http://host → ws://host/ws
  // https://host → wss://host/ws
  const wsProtocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const wsUrl = `${wsProtocol}//${location.host}/ws`
  
  return {
    id: 'session-auth',         // Stable ID (không random như pair code)
    name: 'Orca Session',       // Display name
    connectionType: 'session',  // Mới — phân biệt với 'pairing'
    wsUrl,                      // ws://b15.openledger.vn/ws
    deviceToken: null,          // Không có device token (dùng cookie)
    publicKeyB64: null,         // Không có E2EE
    createdAt: Date.now()
  }
}
```

### Thay đổi 3: `web-preload-api.ts` — Xử lý session environment type

`callRuntimeEnvelope` hiện tại dùng `activeEnvironment` để khởi tạo E2EE client. Cần xử lý trường hợp `connectionType === 'session'`:

```typescript
// web-preload-api.ts — SỬA callRuntimeEnvelope

function getClientForEnvironment(environment: StoredWebRuntimeEnvironment): IRpcClient {
  // Cache client theo environmentId
  if (activeClient && activeClientEnvironmentId === environment.id) {
    return activeClient
  }
  
  let client: IRpcClient
  
  if (environment.connectionType === 'session') {
    // Session auth: dùng WebSocketRpcClient với cookie (không E2EE)
    // WsSessionRouter sẽ validate cookie và proxy đến user process
    const { WebSocketRpcClient } = await import('../../../platform/adapters/web/rpc-client')
    client = new WebSocketRpcClient(environment.wsUrl)
  } else {
    // Pair code / E2EE path (existing behavior — backward compat)
    client = new WebRuntimeClient(environment)
  }
  
  activeClient = client
  activeClientEnvironmentId = environment.id
  return client
}
```

> **Note về async**: `getClientForEnvironment` hiện là sync. Nếu cần import dynamic cho `WebSocketRpcClient`, phải handle async. Thay vào đó, `WebSocketRpcClient` có thể được import statically vì nó đã là một module trong bundle.

### Thay đổi 4: `web-preload-api.ts` — Phân biệt lỗi trong `listWithStatus`

Fix BUG-PC-003 — thêm error logging và phân biệt loại lỗi:

```typescript
// web-preload-api.ts L842-843 — SỬA

const listWithStatus = async (): Promise<DevServer[]> => {
  try {
    return await callRuntimeResult<DevServer[]>('devServer.list', null)
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err)
    
    if (message.includes('No active runtime environment') || 
        message.includes('not authenticated') ||
        message.includes('Authentication required')) {
      // Lỗi auth — không phải lỗi thực sự, user cần login/pair
      // Không log spam mỗi 5s, chỉ trace level
      console.debug('[DevServer] RPC not authenticated — user needs to login or pair')
    } else {
      // Lỗi thực sự — log để debug
      console.warn('[DevServer] devServer.list failed:', message)
    }
    
    return []
  }
}
```

### Thay đổi 5: `useDevServersSync.ts` — Error state

Fix BUG-PC-003 — thêm error state để UI có thể hiển thị lỗi:

```typescript
// src/renderer/src/hooks/useDevServersSync.ts — SỬA

export function useDevServersSync(): void {
  const setDevServers = useSetAtom(devServersAtom)
  const setDevServersError = useSetAtom(devServersErrorAtom)  // ← THÊM atom

  useEffect(() => {
    // Initial load
    void window.api.devServer.list()
      .then((servers) => {
        setDevServers(servers)
        setDevServersError(null)   // ← clear error khi thành công
      })
      .catch((err: Error) => {
        console.warn('[useDevServersSync] devServer.list error:', err.message)
        setDevServersError(err.message)  // ← set error để UI hiện thị
      })

    // Subscribe to status changes
    const cleanup = window.api.devServer.onStatusChanged((update) => {
      // ...existing logic...
    })
    return cleanup
  }, [setDevServers, setDevServersError])
}
```

---

## Thứ Tự Implement

```
1. SOL-PC-001 (server-side) phải deploy trước
   └── Wire WsSessionRouter vào HTTP server upgrade

2. Thêm createSessionWebRuntimeEnvironment() vào web-runtime-environment.ts

3. Cập nhật main-web-bootstrap.tsx
   └── Khi sessionUser !== null → tạo session env → saveStoredWebRuntimeEnvironment

4. Cập nhật web-preload-api.ts
   ├── getClientForEnvironment: handle connectionType === 'session'
   └── listWithStatus: phân biệt lỗi auth vs lỗi thực

5. Cập nhật useDevServersSync.ts
   └── Thêm error state atom và setDevServersError

6. Cập nhật DevServerList.tsx (optional)
   └── Hiển thị error message khi devServersError !== null
```

---

## Verification

### Test 1: Frontend build ✅ CONFIRMED

```
pnpm build:frontend:web
✓ built in 3m 28s — no type errors
# No TypeScript errors in: web-runtime-environment.ts, main-web-bootstrap.tsx,
# web-preload-api.ts, useDevServersSync.ts
```

### Test 2: Implementation verified (code level) ✅ ALL CONFIRMED

| Thay đổi | File | Dòng | Verified |
|---------|------|------|---------|
| `createSessionWebRuntimeEnvironment()` | `web-runtime-environment.ts` | L134 | ✅ |
| Import + null-guard trong bootstrap | `main-web-bootstrap.tsx` | L15, L122 | ✅ |
| `WebSocketRpcClient` import + union type | `web-preload-api.ts` | L108, L150 | ✅ |
| `getClientForEnvironment` session path | `web-preload-api.ts` | L3205 | ✅ |
| `listWithStatus` try/catch logging | `web-preload-api.ts` | L843-861 | ✅ |
| `.catch()` handler initial load | `useDevServersSync.ts` | L24 | ✅ |

### Test 3: Server deployment ✅ CONFIRMED

```
# Server log sau deploy (2026-07-27):
[Orca Server] ✅ WsSessionRouter wired (port 6769) — browser login → per-user process
[DevServerManager] Daemon agent connected: id=dev-local platform=linux node=unknown

# Health check:
{"status":"ready","version":"1.4.138","uptime":13}

# Login test:
HTTP 200 — User: admin@b15.openledger.vn
```

> **Pending E2E UI verify:** Cần mở https://b15.openledger.vn → Login → Settings → Dev Servers để xác nhận `dev-local` hiển thị (cần browser access). Code và server deploy đã sẵn sàng.


---

## Files Liên Quan

| File | Dòng hiện tại | Thay đổi |
|------|--------------|---------|
| `src/renderer/src/web/main-web-bootstrap.tsx` | L115-124 | Thêm session env trước `installWebPreloadApi()` |
| `src/renderer/src/web/web-runtime-environment.ts` | — | Thêm `createSessionWebRuntimeEnvironment()` |
| `src/renderer/src/web/web-preload-api.ts` | L842, L3067 | Session client + error logging |
| `src/renderer/src/hooks/useDevServersSync.ts` | L18-22 | Error state |
| `src/renderer/src/store/slices/dev-servers.ts` | — | Thêm `devServersErrorAtom` |
| `src/renderer/src/components/dev-server/DevServerList.tsx` | L32-38 | Hiển thị error state |
