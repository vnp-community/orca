# BUG-PC-001 — Browser Yêu Cầu Pair Code Dù Đã Login Email/Password

**ID:** BUG-PC-001  
**Mức độ:** 🔴 Critical  
**Module:** `src/renderer/src/web/web-preload-api.ts`  
**Phát hiện:** 2026-07-27  
**Status:** 🔴 Open

---

## Mô Tả

Orca server chạy ở web/server mode với `ORCA_AUTH_MODE=local`. User có thể login thành công bằng email/password tại `POST /auth/local`. Tuy nhiên sau khi login, browser **không thể gọi bất kỳ RPC nào** (bao gồm `devServer.list`) vì toàn bộ RPC calls đều đi qua `requireActiveEnvironment()` — hàm này yêu cầu một **Pair Code session** (E2EE device token) chứ không phải session cookie từ login.

Kết quả: Settings → Dev Servers luôn trống dù agent đã connected.

---

## Root Cause

### Flow thực tế khi gọi `devServer.list`

```
useDevServersSync.ts
  └── window.api.devServer.list()
       └── createDevServerApi().list()
            └── callRuntimeResult<DevServer[]>('devServer.list', null)
                 └── callRuntimeEnvelope(method, params)
                      └── requireActiveEnvironment()  ← THROW nếu chưa pair!
                           ↓
                      catch(() => [])  ← nuốt lỗi, trả [] về store
```

### Code location — `web-preload-api.ts` L3062-3073

```typescript
async function callRuntimeEnvelope<TResult = unknown>(
  method: string,
  params?: unknown,
  timeoutMs?: number
): Promise<RuntimeRpcResponse<TResult>> {
  const environment = requireActiveEnvironment()  // ← Throws khi không có pair code
  const response = await runtimeCallQueuePool.enqueue(environment.id, method, () =>
    getClientForEnvironment(environment).call(method, params, { timeoutMs })
  )
  return response as RuntimeRpcResponse<TResult>
}
```

### `requireActiveEnvironment()` — `web-preload-api.ts` L~463

```typescript
let activeEnvironment: StoredWebRuntimeEnvironment | null = readStoredWebRuntimeEnvironment()
// Đọc từ localStorage — null nếu chưa có Pair Code

function requireActiveEnvironment(): StoredWebRuntimeEnvironment {
  if (!activeEnvironment) {
    throw new Error('No active runtime environment')  // ← Throw ở đây
  }
  return activeEnvironment
}
```

`activeEnvironment` chỉ được set khi user mở URL `#pairing=<base64>` và hoàn thành E2EE handshake. Login qua email/password **không set** `activeEnvironment`.

---

## Tái Hiện

1. Deploy Orca server với `ORCA_MULTI_USER=1`, `ORCA_AUTH_MODE=local`
2. Mở `https://b15.openledger.vn` → login với `admin@b15.openledger.vn`
3. Vào Settings → Dev Servers

**Kết quả**: "No dev servers configured" — dù agent đã connected và `orca-data.json` có 2 DevServer records.

**Confirm thêm**: Mở Browser DevTools → Console → gọi `window.api.devServer.list()` → throw `Error: No active runtime environment`

---

## Hậu Quả

| Tính năng | Bị ảnh hưởng |
|-----------|-------------|
| Settings → Dev Servers | ❌ Luôn trống |
| Onboarding wizard | ❌ Không load được dev server step |
| Preflight checks | ❌ Fail silently |
| Git operations qua relay | ❌ Fail |
| Terminal qua relay | ❌ Fail |

---

## Fix Đề Xuất

### Phương án A — Thêm session-cookie path vào `callRuntimeEnvelope` (Khuyến nghị)

Khi `activeEnvironment === null` nhưng user đã login (có session cookie), tạo một "implicit environment" kết nối đến server qua session-authenticated WebSocket:

```typescript
async function callRuntimeEnvelope<TResult = unknown>(
  method: string,
  params?: unknown,
  timeoutMs?: number
): Promise<RuntimeRpcResponse<TResult>> {
  // Ưu tiên pair-code environment nếu có
  if (activeEnvironment) {
    const response = await runtimeCallQueuePool.enqueue(activeEnvironment.id, method, () =>
      getClientForEnvironment(activeEnvironment!).call(method, params, { timeoutMs })
    )
    return response as RuntimeRpcResponse<TResult>
  }
  
  // Fallback: dùng session-cookie WebSocket (web/server mode)
  // Requires BUG-PC-002 to be fixed first (WsSessionRouter wired)
  return callViaSessionWebSocket<TResult>(method, params, timeoutMs)
}
```

### Phương án B — Set `activeEnvironment` tự động sau khi login

Sau khi `POST /auth/local` thành công, auto-create `StoredWebRuntimeEnvironment` dựa trên current origin và session cookie.

---

## Files Liên Quan

| File | Dòng | Vai trò |
|------|------|---------|
| `src/renderer/src/web/web-preload-api.ts` | L148, L3067 | `activeEnvironment`, `requireActiveEnvironment()` |
| `src/renderer/src/web/web-runtime-environment.ts` | All | `StoredWebRuntimeEnvironment` type |
| `src/renderer/src/hooks/useDevServersSync.ts` | L20 | Caller của `devServer.list()` |

---

## Quan Hệ với Bugs Khác

- **Phụ thuộc BUG-PC-002**: Ngay cả khi fix PC-001, browser vẫn cần `WsSessionRouter` được wired để có session-authenticated WS channel
- **Liên quan BUG-PC-003**: `catch(() => [])` ẩn lỗi này khỏi user
