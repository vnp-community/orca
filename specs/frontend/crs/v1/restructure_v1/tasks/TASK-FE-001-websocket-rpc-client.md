# TASK-FE-001 — Implement WebSocketRpcClient

**Source Solution:** [SOL-FE-002](../solutions/SOL-FE-002-rpc-client-bridge.md)  
**Priority:** P0 — Được các tasks khác phụ thuộc vào  
**Loại:** Tạo file mới  
**Ước tính:** ~150 lines TypeScript

---

## Context

`WebSocketRpcClient` là lớp transport layer cho web mode. Nó thay thế Electron IPC (`ipcRenderer`) bằng JSON-RPC qua WebSocket. Tất cả `window.api` calls trong web mode đều đi qua class này.

---

## Input

Đọc trước khi implement:
- `src/renderer/src/web/web-preload-api.ts` — xem cách client được dùng
- `src/preload/index.ts` — hiểu Electron IPC pattern để mirror

---

## Output — Files cần tạo

### File: `src/platform/adapters/web/rpc-client.ts` [TẠO MỚI]

Implement `WebSocketRpcClient` class với interface sau:

```typescript
export interface IRpcClient {
  connect(): Promise<void>
  disconnect(): void
  isConnected(): boolean
  invoke(channel: string, ...args: any[]): Promise<any>
  send(channel: string, data?: any): void
  on(channel: string, handler: (...args: any[]) => void): () => void
  off(channel: string, handler: (...args: any[]) => void): void
  once(channel: string, handler: (...args: any[]) => void): void
}
```

### Giao thức JSON-RPC (Wire Format)

**Client → Server (invoke):**
```json
{ "id": "uuid", "type": "invoke", "channel": "repos:list", "args": [] }
```

**Server → Client (result):**
```json
{ "id": "uuid", "type": "result", "result": [...] }
```

**Server → Client (error):**
```json
{ "id": "uuid", "type": "error", "message": "Not found" }
```

**Server → Client (push event):**
```json
{ "type": "push", "channel": "pty:data", "args": [{ "ptyId": "...", "data": "..." }] }
```

**Client → Server (fire-and-forget):**
```json
{ "type": "send", "channel": "client:event", "data": {...} }
```

### Yêu cầu implementation

1. **Constructor**: `new WebSocketRpcClient(url?: string)` — nếu không có url, auto-detect từ `window.location.host` với path `/ws/runtime/api`
2. **connect()**: Tạo WebSocket, resolve khi `onopen`, reject khi `onerror`
3. **invoke()**: Gửi message có `id` (uuid/random), await reply theo `id`, timeout sau **30 giây**
4. **on()**: Subscribe push events, return unsubscribe function
5. **off()**: Unsubscribe theo handler reference
6. **once()**: Subscribe 1 lần rồi tự unsubscribe
7. **send()**: Fire-and-forget, silent nếu chưa kết nối
8. **disconnect()**: Đóng WebSocket, clear tất cả pending invocations với error

### ID generation (không dùng `crypto.randomUUID` — không available everywhere)

```typescript
function generateId(): string {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 9)}`
}
```

### URL auto-detection

```typescript
function getDefaultWsUrl(): string {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const host = window.location.host || 'localhost:6768'
  return `${protocol}//${host}/ws/runtime/api`
}
```

---

## Interface file — cần tạo cùng lúc

### File: `src/platform/rpc-client-interface.ts` [TẠO MỚI]

```typescript
// src/platform/rpc-client-interface.ts
// Shared interface — dùng bởi cả web-preload-api.ts và ConnectionStatusProvider.tsx

export interface IRpcClient {
  connect(): Promise<void>
  disconnect(): void
  isConnected(): boolean
  invoke(channel: string, ...args: any[]): Promise<any>
  send(channel: string, data?: any): void
  on(channel: string, handler: (...args: any[]) => void): () => void
  off(channel: string, handler: (...args: any[]) => void): void
  once(channel: string, handler: (...args: any[]) => void): void
}
```

`WebSocketRpcClient` phải `implements IRpcClient`.

---

## Acceptance Criteria

| # | Criteria | Verify bằng |
|---|----------|-------------|
| AC-1 | `connect()` resolves khi WS opens | unit test |
| AC-2 | `connect()` rejects khi WS errors | unit test |
| AC-3 | `invoke()` gửi đúng format JSON-RPC | unit test |
| AC-4 | `invoke()` resolve với result từ server | unit test |
| AC-5 | `invoke()` reject khi server trả error | unit test |
| AC-6 | `invoke()` timeout sau 30s | unit test (fake timers) |
| AC-7 | `invoke()` throw "Not connected" nếu chưa connect | unit test |
| AC-8 | `on()` nhận push events | unit test |
| AC-9 | `on()` trả về unsubscribe function hoạt động | unit test |
| AC-10 | `on()` hỗ trợ nhiều listeners cùng channel | unit test |
| AC-11 | `once()` chỉ nhận event 1 lần | unit test |
| AC-12 | `send()` không throw khi disconnected | unit test |
| AC-13 | `disconnect()` đóng WS và reject pending invocations | unit test |
| AC-14 | `IRpcClient` interface được export từ `rpc-client-interface.ts` | compile check |

---

## Constraints

- **KHÔNG** import `electron` hoặc bất kỳ Electron module nào
- **KHÔNG** dùng Node.js built-ins (`fs`, `path`, v.v.) — chỉ Web APIs
- **KHÔNG** dùng external libraries — chỉ native WebSocket API
- File phải compile clean với TypeScript strict mode

---

## Test file location

Test sẽ được tạo ở TASK-FE-005, nhưng bạn có thể tham khảo test spec tại:
`specs/frontend/crs/v1/restructure_v1/solutions/SOL-FE-002-rpc-client-bridge.md` §3.1

---

## Execution Status

**Status:** ✅ DONE  
**Date:** 2026-07-23  
**Files Created:**
- `src/platform/rpc-client-interface.ts` — IRpcClient shared interface
- `src/platform/adapters/web/rpc-client.ts` — WebSocketRpcClient implementation
- `src/platform/adapters/web/__tests__/rpc-client.test.ts` — 15 test cases
