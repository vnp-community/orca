# TASK-DS-007 — DevServerManager: direct-ws Status → 'connecting' Khi Startup

**Status:** ✅ DONE — 2026-08-01 (DevServerRelayBridge exponential backoff + reconnect status)

**Solution:** [SOL-DS-004 §1](../solutions/SOL-DS-004-reconnect-status.md)  
**Bug:** [BUG-DS-004](../BUG-DS-004-inmemory-state-lost-on-restart.md)  
**File:** `src/main/dev-server/dev-server-manager.ts`  
**Phụ thuộc:** Không  
**Estimated:** 20 phút  
**Status:** ✅ DONE — 2026-07-27

---

## Mục Tiêu

Khi Orca server khởi động lại (restart), `DevServerManager` reset tất cả runtime state về `'disconnected'`. Với `direct-websocket` servers, agent sẽ tự reconnect qua systemd — status nên là `'connecting'` thay vì `'disconnected'`. Thêm method `restoreConnections()` để emit đúng trạng thái ngay sau startup.

---

## Context

Đọc trước:
- `src/main/dev-server/dev-server-manager.ts` — constructor (dòng ~50-61), `initRuntimeState()`, `setRuntimeState()`
- `src/server/index.ts` hoặc `src/main/server-bootstrap.ts` — nơi khởi tạo `DevServerManager`

---

## Thay Đổi Cần Thực Hiện

### Thay đổi 1: `src/main/dev-server/dev-server-manager.ts`

**Sửa constructor** — TÌM:
```typescript
  constructor(
    persistStore: Store,
    private sshManager: SshConnectionManager,
    private agentWsServer: AgentWebSocketServer | null = null
  ) {
    super()
    this.store = new DevServerStore(persistStore)
    // Restore runtime state for all persisted servers (status = disconnected)
    for (const ds of this.store.list()) {
      this.initRuntimeState(ds.id)
    }
  }
```

**THAY BẰNG:**
```typescript
  constructor(
    persistStore: Store,
    private sshManager: SshConnectionManager,
    private agentWsServer: AgentWebSocketServer | null = null
  ) {
    super()
    this.store = new DevServerStore(persistStore)
    // Restore runtime state for all persisted servers.
    // direct-websocket: agent tự reconnect via systemd → 'connecting', không 'disconnected'
    // relay-ssh, relay-websocket: không có auto-reconnect → 'disconnected' (default)
    for (const ds of this.store.list()) {
      this.initRuntimeState(ds.id)
      if (ds.connectionType === 'direct-websocket') {
        this.setRuntimeState(ds.id, { status: 'connecting', lastError: null })
      }
    }
  }
```

**Thêm method `restoreConnections()`** — sau constructor (trước method `list()`):

```typescript
  /**
   * Emit 'devServer:statusChanged' → 'connecting' for all persisted
   * direct-websocket servers after server startup.
   *
   * Call once from server bootstrap AFTER the HTTP server is listening,
   * so WebSocket clients already connected to the UI receive the event.
   *
   * Background: DevServerManager runtime state is in-memory and lost on restart.
   * direct-websocket agents reconnect via systemd (exit(2) → start.sh → fresh token).
   * This method signals the UI to show "Connecting..." instead of "Disconnected"
   * while the agent reconnects.
   */
  restoreConnections(): void {
    for (const ds of this.store.list()) {
      if (ds.connectionType === 'direct-websocket') {
        this.emit('devServer:statusChanged', ds.id, 'connecting')
        console.log(
          `[DevServerManager] Startup restore: ${ds.id} (${ds.name}) → 'connecting' ` +
          `(daemon agent will reconnect via systemd)`
        )
      }
    }
  }
```

### Thay đổi 2: Gọi `restoreConnections()` trong server bootstrap

**File:** `src/server/index.ts` (hoặc file bootstrap tương ứng)

Tìm đoạn khởi tạo `devServerManager`. Sau dòng tạo instance, thêm:

```typescript
// Emit 'connecting' status for direct-websocket servers.
// Agents will reconnect via systemd within ~30s.
// Call after HTTP server is listening so IPC events reach UI clients.
server.on('listening', () => {
  devServerManager.restoreConnections()
})
```

Hoặc nếu không có event `'listening'`, gọi ngay sau `server.listen()`:
```typescript
devServerManager.restoreConnections()
```

---

## Verify

```bash
# 1. Server đang chạy, agent connected
# 2. docker restart orca-server (hoặc equivalent)
# 3. Mở Orca UI
# Expected: Dev server hiện "Connecting..." (thay vì "Disconnected")
# Sau ~30s khi agent reconnect: "Connected" ✅

# Kiểm tra server log:
grep "Startup restore\|restoreConnections" logs/server.log
# Expected: "[DevServerManager] Startup restore: dev-local (Dev Server Name) → 'connecting'"
```

---

## Definition of Done

- [x] Constructor set `direct-websocket` servers về `'connecting'` thay vì `'disconnected'`
- [x] Method `restoreConnections()` đã thêm với JSDoc
- [x] `relay-ssh` và `relay-websocket` servers vẫn là `'disconnected'` (default, đúng)
- [x] TypeScript compile OK (no errors)
