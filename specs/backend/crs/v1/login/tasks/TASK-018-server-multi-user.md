# TASK-018: Sửa `src/server/index.ts` — Tích hợp ORCA_MULTI_USER + WsSessionRouter

> **Status:** ✅ DONE (2026-07-24)


**Phase:** 2 — User Sandbox
**Solution:** [SOL-LG-002](../solutions/SOL-LG-002-user-sandbox.md) §5
**Depends on:** TASK-014, TASK-016, TASK-017
**Blocks:** (Phase 2 complete)

---

## Mục tiêu

Thêm feature flag `ORCA_MULTI_USER=1` vào server entry point. Khi bật: WsSessionRouter intercept WS connections thay vì shared runtime.

---

## File cần sửa

**Path:** `src/server/index.ts`

---

## Thay đổi cần thực hiện

### 1. Thêm imports (đặt trong block `if (MULTI_USER)`)

```typescript
// Chỉ import khi MULTI_USER = true (tránh load code không cần thiết)
// Dùng dynamic import bên trong if block
```

### 2. Đọc feature flag

```typescript
const MULTI_USER = process.env.ORCA_MULTI_USER === '1'
```

### 3. Thêm SessionManager + WsSessionRouter nếu MULTI_USER

Tìm chỗ WebSocket server được khởi tạo, thêm routing logic sau:

```typescript
if (MULTI_USER) {
  const { SessionManager }   = await import('../main/session/session-manager')
  const { WsSessionRouter }  = await import('../main/session/ws-session-router')
  const { join }             = await import('node:path')

  const sessionManager = new SessionManager({
    baseDataPath:        userDataPath,
    userProcessEntry:    join(__dirname, '../main/session/user-process-entry.js'),
    idleTimeoutMs:       4 * 60 * 60 * 1000,   // 4h
    maxRespawnAttempts:  3
  })

  const wsRouter = new WsSessionRouter({
    sessionManager,
    authManager   // từ ServerBootstrapResult
  })

  // Intercept WS 'connection' event — TRƯỚC shared runtime handler
  wss.on('connection', (ws, req) => {
    void wsRouter.handleConnection(ws, req)
  })

  console.log('[Server] Multi-user mode ENABLED — per-user process isolation active')

  // Shutdown sequence
  const originalShutdown = shutdown
  shutdown = async () => {
    await sessionManager.shutdown()
    await originalShutdown()
  }
} else {
  console.log('[Server] Single-user mode (ORCA_MULTI_USER not set)')
}
```

---

## Env vars

| Env | Default | Mô tả |
|-----|---------|-------|
| `ORCA_MULTI_USER` | `0` | Enable per-user process isolation |

---

## Acceptance Criteria

- [x] `ORCA_MULTI_USER=0` (default): behavior KHÔNG thay đổi — PairCode, E2EE, tất cả hoạt động như cũ
- [x] `ORCA_MULTI_USER=1`: SessionManager và WsSessionRouter được khởi tạo
- [x] `ORCA_MULTI_USER=1`: WS connections không có session cookie → close 4401
- [x] `ORCA_MULTI_USER=1`: WS connections có valid session → proxy đến user process socket
- [x] TypeScript compile không có lỗi mới
- [x] `sessionManager.shutdown()` được gọi trong shutdown sequence
