# TASK-PC-005 — Error Logging + Silent Fail Fix Cho `devServer.list`

**Solution:** [SOL-PC-002 §Thay đổi 4, 5](../solutions/SOL-PC-002-browser-session-rpc.md)  
**Bug:** [BUG-PC-003](../BUG-PC-003-devserver-list-silent-fail.md)  
**Files:**
- `src/renderer/src/web/web-preload-api.ts`
- `src/renderer/src/hooks/useDevServersSync.ts`

**Phụ thuộc:** Không (độc lập, có thể implement song song)  
**Estimated:** 25 phút  
**Status:** ✅ DONE — 2026-07-27

---

## Mục Tiêu

Thay `.catch(() => [])` ẩn danh bằng error handler có logging rõ ràng. Phân biệt lỗi auth (expected khi chưa login) vs lỗi thực sự (cần log warn). Thêm error handling trong `useDevServersSync` để hook không silently drop lỗi.

---

## Context

Đọc trước:
- `src/renderer/src/web/web-preload-api.ts` — L842-843 (`listWithStatus`)
- `src/renderer/src/hooks/useDevServersSync.ts` — L18-22 (initial load)

---

## Thay Đổi Cần Thực Hiện

### Thay đổi 1: `src/renderer/src/web/web-preload-api.ts`

**TÌM** (L842-843):
```typescript
  const listWithStatus = async () =>
    callRuntimeResult<DevServer[]>('devServer.list', null).catch(() => [] as DevServer[])
```

**THAY BẰNG:**
```typescript
  const listWithStatus = async (): Promise<DevServer[]> => {
    try {
      return await callRuntimeResult<DevServer[]>('devServer.list', null)
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err)
      // Auth/session errors are expected before login or pairing — debug level only
      if (
        message.includes('No active runtime environment') ||
        message.includes('not authenticated') ||
        message.includes('Authentication required') ||
        message.includes('auth')
      ) {
        console.debug('[DevServer] RPC not ready — user needs to login or pair')
      } else {
        // Unexpected error — log at warn level for debugging
        console.warn('[DevServer] devServer.list failed:', message)
      }
      return []
    }
  }
```

---

### Thay đổi 2: `src/renderer/src/hooks/useDevServersSync.ts`

**TÌM** (L19-22):
```typescript
    // ── Initial load ──────────────────────────────────────────────────────────
    void window.api.devServer.list().then((servers) => {
      setDevServers(servers)
    })
```

**THAY BẰNG:**
```typescript
    // ── Initial load ──────────────────────────────────────────────────────────
    void window.api.devServer.list()
      .then((servers) => {
        setDevServers(servers)
      })
      .catch((err: Error) => {
        // list() in web-preload-api already catches and returns [] for auth errors.
        // This catch handles any unexpected throw from the api layer itself.
        console.warn('[useDevServersSync] Unexpected error loading dev servers:', err.message)
      })
```

---

## Verify

```bash
# TypeScript compile
cd /Users/binhnt/Work/blockchain/vnp-blc/orca
npx tsc --noEmit 2>&1 | grep -E "useDevServersSync|web-preload-api" | head -10
# Expected: không có lỗi

# Build frontend
pnpm build:frontend:web 2>&1 | tail -10
# Expected: build thành công
```

**Manual verify** (trước khi login):
1. Mở `https://b15.openledger.vn` (chưa login, chưa pair)
2. Mở Browser DevTools → Console
3. Expected: thấy `[DevServer] RPC not ready — user needs to login or pair` ở debug level
4. KHÔNG thấy `No active runtime environment` error ở warn level

**Manual verify** (sau khi login):
1. Login thành công
2. Console: KHÔNG còn thấy debug message (request thành công)
3. Settings → Dev Servers → thấy `dev-local`

---

## Definition of Done

- [x] `listWithStatus` thay `.catch(() => [])` bằng try/catch với phân loại lỗi
- [x] Lỗi auth (`No active runtime environment`, `not authenticated`, `Authentication required`) → `console.debug` (không spam warn)
- [x] Lỗi khác → `console.warn` với message
- [x] `useDevServersSync` thêm `.catch()` cho initial load
- [x] TypeScript compile OK — `listWithStatus` return type `Promise<DevServer[]>` explicit
- [x] `pnpm build:frontend:web` thành công
