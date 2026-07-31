# TASK-01: Mở rộng `RpcContext` với `devServerManager` và `userId`

**Status:** ✅ DONE — 2026-07-25  
**Phase:** 1 — Foundation  
**Priority:** 🔴 Critical  
**Depends on:** Không có  
**Solution:** SOL-05-Context-Injection.md  
**CRs:** CR-GH-003, CR-GH-005  
**Estimated effort:** ~30 phút

---

## Mục tiêu

Thêm `devServerManager` và `userId` vào `RpcContext` (type được định nghĩa trong `src/main/runtime/rpc/core.ts`). Đây là **foundation** cho tất cả các task tiếp theo — nếu không có 2 field này, các handler không thể proxy sang Dev Server hoặc truy cập credential store per-user.

---

## Hiện trạng code

**File:** `src/main/runtime/rpc/core.ts` — `RpcContext` type (line 41–78):
```typescript
export type RpcContext = {
  runtime: OrcaRuntimeService
  signal?: AbortSignal
  connectionId?: string
  requestId?: string
  clientId?: string
  clientKind?: 'mobile' | 'runtime'
  sendBinary?: (bytes: Uint8Array<ArrayBufferLike>) => boolean | void
  registerBinaryStreamHandler?: (streamId: number, handler: ...) => () => void
}
```

**File:** `src/main/runtime/rpc/dispatcher.ts` — `dispatch()` (line 80–83):
```typescript
const result = await method.handler(parsedParams.value, {
  runtime: this.runtime,
  signal: options?.signal
})
```

---

## Các bước thực thi

### Bước 1: Sửa `RpcContext` type trong `core.ts`

Thêm 2 field mới vào cuối type `RpcContext`:

```typescript
// src/main/runtime/rpc/core.ts
// Thêm import ở đầu file:
import type { DevServerManager } from '../../dev-server/dev-server-manager'

// Trong type RpcContext, thêm 2 field vào sau field cuối cùng (registerBinaryStreamHandler):
  // Why: integration proxy methods (preflight.check, github.startAuthLogin, etc.)
  // need to reach the active relay for a given dev server without going through
  // the OrcaRuntimeService, which is not relay-aware.
  devServerManager?: DevServerManager
  // Why: credential-store reads are scoped per authenticated Orca user.
  // Each user-process has a distinct userId injected via ORCA_USER_ID env var.
  userId?: string
```

### Bước 2: Truyền `devServerManager` và `userId` vào `DispatcherOptions`

**File:** `src/main/runtime/rpc/dispatcher.ts`

```typescript
// Sửa DispatcherOptions (line 32–35):
export type DispatcherOptions = {
  runtime: OrcaRuntimeService
  methods?: readonly RpcAnyMethod[]
  devServerManager?: DevServerManager   // THÊM
}

// Sửa class field (line 37–39):
export class RpcDispatcher {
  private readonly runtime: OrcaRuntimeService
  private readonly registry: RpcRegistry
  private readonly devServerManager?: DevServerManager  // THÊM

// Sửa constructor (line 41–44):
  constructor({ runtime, methods = ALL_RPC_METHODS, devServerManager }: DispatcherOptions) {
    this.runtime = runtime
    this.registry = buildRegistry(methods)
    this.devServerManager = devServerManager  // THÊM
  }
```

### Bước 3: Tiêm vào context trong `dispatch()` và `dispatchStreaming()`

**File:** `src/main/runtime/rpc/dispatcher.ts`

```typescript
// Sửa dispatch() — line 80–83:
const result = await method.handler(parsedParams.value, {
  runtime: this.runtime,
  signal: options?.signal,
  devServerManager: this.devServerManager,  // THÊM
  userId: options?.userId                   // THÊM
})

// Tương tự, thêm userId vào options của dispatchStreaming():
async dispatchStreaming(
  request: RpcRequest,
  reply: (response: string) => void,
  options?: {
    connectionId?: string
    signal?: AbortSignal
    clientId?: string
    clientKind?: 'mobile' | 'runtime'
    sendBinary?: ...
    registerBinaryStreamHandler?: ...
    userId?: string   // THÊM
  }
)
```

### Bước 4: Tìm nơi khởi tạo `RpcDispatcher` và truyền `devServerManager`

```bash
grep -rn "new RpcDispatcher" /Users/binhnt/Work/blockchain/vnp-blc/orca/src/
```

Cần tìm xem `RpcDispatcher` được khởi tạo ở đâu (thường trong `runtime-rpc.ts` hoặc `server-bootstrap.ts`) và truyền thêm `devServerManager` vào options.

---

## Acceptance Criteria

1. `RpcContext` type có `devServerManager?: DevServerManager` và `userId?: string`
2. `DispatcherOptions` có `devServerManager?: DevServerManager`
3. Handler của `preflight.check` có thể gọi `ctx.devServerManager?.getRelay(...)` mà không bị TypeScript error
4. Test suite hiện tại vẫn pass (`pnpm test` — không có breaking changes vì các field là optional)

---

## Files cần sửa

- `src/main/runtime/rpc/core.ts` — type extension
- `src/main/runtime/rpc/dispatcher.ts` — DispatcherOptions + dispatch + dispatchStreaming

## Lưu ý

- Tất cả field mới là **optional** (`?:`) → không breaking bất kỳ existing test nào
- Import `DevServerManager` type phải dùng `import type` (không runtime import) để tránh circular dependency
