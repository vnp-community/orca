# Solution cho CR-GH-003 & CR-GH-005: Context Injection cho Web Mode

## Bối cảnh & TDD Specs liên kết
- Theo **TDD-11 (Web Server Mode)**, ứng dụng chạy trong chế độ nhiều người dùng (Multi-User) sẽ định tuyến các request thông qua `OrcaRuntimeRpcServer`.
- Để proxy các tác vụ xuống Dev Server tương ứng, chúng ta cần xác định được `devServerId` từ client UI gửi lên.

## Đánh giá vấn đề
- RPC Handler của `preflight.check` hiện tại bị thiếu quyền truy cập vào `devServerManager` để proxy sang các SSH Relay.
- Trình duyệt UI (`src/renderer/src/store/slices/preflight.ts`) cần gửi `devServerId` nhưng tham số này chưa được đưa vào payload của JSON-RPC `preflight.check`.

## Thiết kế giải pháp

### 1. Bổ sung `devServerManager` vào `RpcMethodContext`
Trong `src/main/server-bootstrap.ts`, đối tượng `devServerManager` đã được khởi tạo và trả về (theo **TDD-11 Addendum v3.0**).
Chúng ta chỉ cần tiêm nó vào context khi `RpcDispatcher` gọi handler.

```typescript
// src/main/runtime/rpc/dispatcher.ts
export type RpcMethodContext = {
  runtime: OrcaRuntimeService;
  devServerManager?: DevServerManager; // Inject từ server-bootstrap
  userId?: string; // Sẵn có từ WsSessionRouter environment
};
```

### 2. Frontend gửi `devServerId` trong Context
Khi `preflight.check` được kích hoạt từ giao diện:
- Nếu đang ở màn hình Settings > Dev Servers (chỉ định 1 Dev Server), `devServerId` sẽ có giá trị.
- UI slice sẽ chèn thêm ID này vào params:

```typescript
// src/renderer/src/store/slices/preflight.ts
const context = getLocalPreflightContext(get());

const rpcParams = {
  force: force || undefined,
  devServerId: context?.devServerId // Kèm ID vào để Backend biết đường proxy
};

callRuntimeRpc(runtimeTarget, 'preflight.check', rpcParams);
```

## Ưu điểm
- Cơ chế linh hoạt: Khi không có `devServerId` (ví dụ, kiểm tra Local trên app Electron), logic tự động fallback về kiểm tra binary nội bộ của máy trạm.
- Tận dụng hệ thống Dependency Injection sẵn có của RPC server, không cần sửa đổi lớn kiến trúc.

---

## ✅ Implementation Status — COMPLETED (2026-07-25)

### Files đã implement

#### `src/main/runtime/rpc/core.ts` [MODIFIED]
- ✅ `RpcMethodContext` type mở rộng với 2 optional fields:
  ```typescript
  devServerManager?: DevServerManager  // Inject từ RpcDispatcher → server-bootstrap
  userId?: string                      // Inject từ WsSessionRouter (ORCA_USER_ID env)
  ```
- ✅ Comment giải thích rõ: `Injected by RpcDispatcher from ServerBootstrapResult.devServerManager`

#### `src/main/runtime/rpc/dispatcher.ts` [MODIFIED]
- ✅ `DispatcherOptions` nhận `devServerManager?: DevServerManager`
- ✅ `RpcDispatcher` store `devServerManager` và propagate vào mọi handler context
- ✅ `dispatch()` truyền `userId` từ `options.userId` vào context
- ✅ `dispatch()` overload: `dispatch(request, { signal?, userId? })`

#### `src/main/runtime/runtime-rpc.ts` [MODIFIED]
- ✅ `OrcaRuntimeRpcServer` nhận `devServerManager` trong constructor options
- ✅ Truyền `devServerManager` khi tạo `new RpcDispatcher({ runtime, devServerManager })`

#### `src/main/server-bootstrap.ts` [MODIFIED]
- ✅ Truyền `devServerManager` khi tạo `OrcaRuntimeRpcServer`:
  ```typescript
  new OrcaRuntimeRpcServer({ runtime, userDataPath, enableWebSocket: true, wsPort, devServerManager })
  ```

#### `src/renderer/src/store/slices/preflight.ts` [MODIFIED]
- ✅ Gửi `devServerId` trong `preflight.check` params:
  ```typescript
  ...(get().activeDevServerId ? { devServerId: get().activeDevServerId } : {})
  ```
- ✅ State management: `setRemotePreflightStatus(devServerId, status)` / `clearRemotePreflightStatus(devServerId)`
- ✅ `remotePreflightByServer: Record<string, RemotePreflightStatus>` — per-server status map

#### `src/main/runtime/rpc/methods/preflight.ts` [MODIFIED]
- ✅ Schema bổ sung `devServerId?: z.string()`
- ✅ Handler kiểm tra `params.devServerId && ctx.devServerManager` → proxy sang relay
- ✅ Fallback `runPreflightCheck(params.force)` khi không có devServerId

### Kết quả test
```
✅ preflight.test.ts — 21/21 tests pass bao gồm:
   - proxies preflight.check to relay when devServerId is provided
   - falls back to runPreflightCheck() when no devServerId provided
   - throws when devServerId is given but relay is not connected
✅ TypeScript — 0 new errors trong các files đã sửa
```

### Thay đổi so với thiết kế gốc
| Thiết kế gốc | Implementation thực tế |
|---|---|
| `RpcMethodContext` thêm `devServerManager?` và `userId?` | Đúng như thiết kế |
| Frontend slice gửi `devServerId` trong params | Đúng như thiết kế: `get().activeDevServerId` |
| Fallback local khi không có devServerId | Đúng như thiết kế |
| `getLocalPreflightContext()` để lấy context | Function tồn tại — slice dùng trực tiếp `get().activeDevServerId` |
