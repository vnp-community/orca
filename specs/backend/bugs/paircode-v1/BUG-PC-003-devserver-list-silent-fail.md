# BUG-PC-003 — `devServer.list` Fail Silently — UI Trống Không Có Lỗi

**Status:** ✅ FIXED — 2026-08-01  
**Task:** TASK-PC-001~005  
**Note:** paircode-v1 domain fixed 2026-07-27  

**ID:** BUG-PC-003  
**Mức độ:** 🟠 High  
**Module:** `src/renderer/src/web/web-preload-api.ts`, `src/renderer/src/hooks/useDevServersSync.ts`  
**Phát hiện:** 2026-07-27  
**Status:** 🔴 Open

---

## Mô Tả

Khi `devServer.list()` thất bại (do BUG-PC-001: không có pair code), lỗi bị nuốt hoàn toàn bởi `.catch(() => [])`. UI hiển thị "No dev servers configured" thay vì thông báo lỗi rõ ràng. User không biết có sự cố, không biết cần pair lại, không có cách debug.

Đây là bug **UX + debuggability** nghiêm trọng — làm cho BUG-PC-001 và BUG-PC-002 cực kỳ khó phát hiện trong production.

---

## Root Cause

### 1. Silent catch trong `createDevServerApi()` — `web-preload-api.ts` L842-843

```typescript
const listWithStatus = async () =>
  callRuntimeResult<DevServer[]>('devServer.list', null).catch(() => [] as DevServer[])
  //                                                    ↑ Nuốt MỌI lỗi, kể cả "No active environment"
```

Khi `callRuntimeResult` throw vì `requireActiveEnvironment()` → error bị catch → trả `[]` → store set `devServers = []` → UI "No dev servers configured".

### 2. Không có error state trong `useDevServersSync.ts`

```typescript
// useDevServersSync.ts L18-22
useEffect(() => {
  // ── Initial load ────────────────────────────────────────────────────────────
  void window.api.devServer.list().then((servers) => {
    setDevServers(servers)
  })
  // ← Không có .catch() — errors silently dropped
```

Nếu `list()` throw (edge case), không có handler nào → React không show lỗi.

### 3. Polling `onStatusChanged` cũng fail silently

```typescript
// web-preload-api.ts L870-885
const interval = setInterval(() => {
  void listWithStatus().then((servers) => {  // ← listWithStatus() dùng .catch(() => [])
    // Mọi 5s: cập nhật status dựa trên list() result
    // Nhưng nếu list() luôn trả [] → không có status change → polling vô nghĩa
  })
}, 5000)
```

Vòng poll chạy mỗi 5s nhưng luôn nhận `[]` → không fire status change event → UI không update kể cả khi agent vừa reconnect.

---

## Tái Hiện

1. Mở `https://b15.openledger.vn` mà không có Pair Code
2. Login (hoặc không login)
3. Vào Settings → Dev Servers
4. Mở Browser DevTools → Network tab

**Kết quả**:
- UI: "No dev servers configured" (không có error)
- Console: không có error log
- Network: không có failed request (RPC không được gửi vì throw trước khi tạo WS)

**Diagnose thực tế**: Phải grep log server-side mới thấy `[DevServerManager] Daemon agent connected`.

---

## Hậu Quả

| Vấn đề | Tác động |
|--------|---------|
| User không biết có lỗi | ❌ Tưởng chưa setup dev server |
| Không có hướng dẫn fix | ❌ Không biết cần pair lại |
| Dev khó debug | ❌ Mất thời gian tìm root cause |
| Server-side state bị ẩn | ❌ Agent connected nhưng UI trống |

---

## Fix Đề Xuất

### Phương án A — Phân biệt lỗi "no session" vs lỗi khác

```typescript
// web-preload-api.ts
const listWithStatus = async () => {
  try {
    return await callRuntimeResult<DevServer[]>('devServer.list', null)
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err)
    
    if (message.includes('No active runtime environment')) {
      // Không có pair code — đây là trạng thái bình thường khi chưa pair
      // Có thể emit event để UI show "Connect to server" prompt
      notifyNeedsPairing()
      return [] as DevServer[]
    }
    
    // Lỗi thực sự — log để debug
    console.error('[DevServerSync] Failed to load dev servers:', message)
    return [] as DevServer[]
  }
}
```

### Phương án B — Thêm error state vào store

```typescript
// store/slices/dev-servers.ts — thêm field
export type DevServerSlice = {
  devServers: DevServer[]
  devServersLoadError: string | null  // ← thêm field này
  // ...
}

// useDevServersSync.ts — set error state
void window.api.devServer.list()
  .then((servers) => {
    setDevServers(servers)
    setDevServersLoadError(null)
  })
  .catch((err: Error) => {
    setDevServersLoadError(err.message)
  })

// DevServerList.tsx — hiển thị lỗi
{devServersLoadError && (
  <div className="dev-server-list__error">
    <p>Cannot load dev servers: {devServersLoadError}</p>
    <Button onClick={() => openPairingDialog()}>Connect to Server</Button>
  </div>
)}
```

### Phương án C — Toast / notification khi `requireActiveEnvironment` throw

Thêm một global error boundary bắt lỗi từ RPC calls và show notification với action "Pair to server".

---

## Files Liên Quan

| File | Dòng | Vai trò |
|------|------|---------|
| `src/renderer/src/web/web-preload-api.ts` | L842-843 | `.catch(() => [])` bug location |
| `src/renderer/src/hooks/useDevServersSync.ts` | L18-22 | Không handle error từ `list()` |
| `src/renderer/src/components/dev-server/DevServerList.tsx` | L32-38 | UI "No dev servers" — không check error state |
| `src/renderer/src/store/slices/dev-servers.ts` | All | Không có `loadError` field |

---

## Quan Hệ với Bugs Khác

- **Symtom của BUG-PC-001**: PC-003 là lý do BUG-PC-001 khó phát hiện
- **Độc lập**: Có thể fix PC-003 ngay cả khi PC-001 và PC-002 chưa fix — sẽ giúp user biết cần pair
