# TASK-PC-004 — Handle Session ConnectionType Trong `getClientForEnvironment`

**Solution:** [SOL-PC-002 §Thay đổi 3](../solutions/SOL-PC-002-browser-session-rpc.md)  
**Bug:** [BUG-PC-001](../BUG-PC-001-browser-requires-paircode.md)  
**File:** `src/renderer/src/web/web-preload-api.ts`  
**Phụ thuộc:** TASK-PC-002 (cần `createSessionWebRuntimeEnvironment`)  
**Estimated:** 30 phút  
**Status:** ✅ DONE — 2026-07-27

---

## Mục Tiêu

Hàm `getClientForEnvironment()` hiện tại luôn tạo `WebRuntimeClient` (E2EE pair code client). Cần thêm path cho `session-auth` environment: dùng `WebSocketRpcClient` kết nối bằng session cookie, không cần E2EE.

---

## Context

Đọc trước:
- `src/renderer/src/web/web-preload-api.ts` — L3169-3176 (`getClientForEnvironment`)
- `src/renderer/src/web/web-runtime-client.ts` — class `WebRuntimeClient` (E2EE client)
- `src/platform/adapters/web/rpc-client.ts` — class `WebSocketRpcClient` (plain WS client, dùng session cookie)

**Điểm mấu chốt**: `WebRuntimeClient` dùng E2EE Curve25519 key exchange (cần `publicKeyB64` và `deviceToken`). `WebSocketRpcClient` là plain JSON-RPC over WebSocket, auth qua cookie do browser gửi tự động.

**Vấn đề hiện tại:**
```typescript
// L3169-3176 — HIỆN TẠI
function getClientForEnvironment(environment: StoredWebRuntimeEnvironment): WebRuntimeClient {
  if (!activeClient || activeClientEnvironmentId !== environment.id) {
    activeClient?.close()
    activeClient = new WebRuntimeClient(getPreferredWebPairingOffer(environment))
    //            ↑ Luôn E2EE — nhưng session env có deviceToken='' và publicKeyB64=''
    //              → sẽ fail khi thử handshake E2EE với empty keys
    activeClientEnvironmentId = environment.id
  }
  return activeClient
}
```

---

## Thay Đổi Cần Thực Hiện

### File: `src/renderer/src/web/web-preload-api.ts`

**Bước 1: Thêm import `WebSocketRpcClient`** — tìm import block ở đầu file (gần L107):

```typescript
import { WebRuntimeClient } from './web-runtime-client'
import { WebSocketRpcClient } from '../../../platform/adapters/web/rpc-client'  // ← THÊM
```

**Bước 2: Đổi kiểu `activeClient`** để chứa cả hai loại client:

TÌM (L149):
```typescript
let activeClient: WebRuntimeClient | null = null
```

THAY BẰNG:
```typescript
let activeClient: WebRuntimeClient | WebSocketRpcClient | null = null
```

**Bước 3: Sửa `getClientForEnvironment`** (L3169-3176):

TÌM:
```typescript
function getClientForEnvironment(environment: StoredWebRuntimeEnvironment): WebRuntimeClient {
  if (!activeClient || activeClientEnvironmentId !== environment.id) {
    activeClient?.close()
    activeClient = new WebRuntimeClient(getPreferredWebPairingOffer(environment))
    activeClientEnvironmentId = environment.id
  }
  return activeClient
}
```

THAY BẰNG:
```typescript
function getClientForEnvironment(
  environment: StoredWebRuntimeEnvironment
): WebRuntimeClient | WebSocketRpcClient {
  if (!activeClient || activeClientEnvironmentId !== environment.id) {
    activeClient?.close()

    const preferredEndpoint = environment.endpoints.find(
      (ep) => ep.id === environment.preferredEndpointId
    ) ?? environment.endpoints[0]

    if (
      preferredEndpoint &&
      (!preferredEndpoint.deviceToken || !preferredEndpoint.publicKeyB64)
    ) {
      // Session-auth environment: no E2EE keys → use plain WebSocketRpcClient.
      // WsSessionRouter (server-side) validates session cookie and proxies to user process.
      activeClient = new WebSocketRpcClient(preferredEndpoint.endpoint)
    } else {
      // Pair code / E2EE environment: use WebRuntimeClient with full key exchange.
      activeClient = new WebRuntimeClient(getPreferredWebPairingOffer(environment))
    }

    activeClientEnvironmentId = environment.id
  }
  return activeClient
}
```

**Bước 4: Fix return type của `callRuntimeEnvelope`** — Hàm này gọi `getClientForEnvironment(...).call(...)`. Cả `WebRuntimeClient` và `WebSocketRpcClient` đều có method `call()` tương thích (cả hai implement `IRpcClient` hoặc có interface tương tự). Không cần thay đổi gì thêm.

---

## Verify

```bash
# TypeScript compile check
cd /Users/binhnt/Work/blockchain/vnp-blc/orca
npx tsc --noEmit 2>&1 | grep "web-preload-api" | head -20
# Expected: không có type error

# Build frontend
pnpm build:frontend:web 2>&1 | tail -20
# Expected: build thành công
```

**Runtime verify** (sau khi deploy + TASK-PC-001 và TASK-PC-003 đã done):
1. Login vào `https://b15.openledger.vn`
2. Browser DevTools → Network → WS connections
3. Expected: thấy WS connection đến `wss://b15.openledger.vn/ws` (hoặc tương tự) thành công
4. Settings → Dev Servers → thấy `dev-local` với status `connected`

---

## Definition of Done

- [x] Import `WebSocketRpcClient` đã thêm
- [x] `activeClient` type đã mở rộng để chứa cả hai client loại
- [x] `getClientForEnvironment` phân biệt `session-auth` (no E2EE keys) vs pair code path
- [x] Session path tạo `WebSocketRpcClient(endpoint.endpoint)` không phải `WebRuntimeClient`
- [x] TypeScript compile OK — không có `any` mới
- [x] `pnpm build:frontend:web` thành công
