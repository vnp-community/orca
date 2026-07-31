# BUG-DS-004 — In-Memory State Mất Sau Orca Server Restart

**ID:** BUG-DS-004  
**Mức độ:** 🟠 High  
**Module:** `DevServerManager` — runtime state management  
**Phát hiện:** 2026-07-26  
**Status:** 🔴 Open

---

## Mô Tả

`DevServerManager` lưu connection state (status, platform, nodeVersion, lastConnectedAt) trong bộ nhớ (`Map<string, RuntimeDevServerState>`). Khi Orca server restart (Docker container restart), toàn bộ state này mất. Agent (systemd) sẽ tự reconnect sau ~5-30s, nhưng trong khoảng thời gian đó UI hiển thị sai trạng thái và mọi relay calls fail.

---

## Root Cause

**`dev-server-manager.ts` L27-43**:

```typescript
type RuntimeDevServerState = {
  status: DevServerStatus     // ← in-memory ONLY
  platform: NodeJS.Platform | null
  arch: string | null
  nodeVersion: string | null
  lastConnectedAt: number | null
  lastError: string | null
}

// Restore on startup: tất cả = 'disconnected'
for (const ds of this.store.list()) {
  this.initRuntimeState(ds.id)  // status = 'disconnected'
}
```

Persisted state (tên server, connectionType, wsUrl) được restore từ disk. Runtime state (status, platform) không persist → reset về `disconnected` mỗi restart.

---

## Tái Hiện

1. Agent đang connected (status = 'connected' trong UI)
2. `docker restart orca-server` (hoặc `sync-to-server.sh`)
3. Server restart mất 10-20s
4. Trong 20s: agent ws close → exit(2) → systemd restart → start.sh → token mới → agent reconnect
5. Trong quá trình này: UI hiển thị "Disconnected" ngay cả khi agent sắp reconnect

**Kịch bản nghiêm trọng hơn**:
- User mở UI ngay sau server restart
- Gọi `onboarding.detectAgents` → `relay = null` (chưa reconnect) → throw `'not connected'`
- UI hiển thị error, user nghĩ agent bị lỗi

---

## Hậu Quả

| Thời điểm | Trạng thái | Hậu quả |
|-----------|-----------|---------|
| T=0: Server restart | All state = disconnected | UI flash "Disconnected" |
| T=0 → T=20: Agent reconnecting | relays map empty | Mọi relay calls fail |
| T=20: Agent reconnected | status = 'connected' | UI restored |
| Bất kỳ lúc nào trong T=0-20 | relay = null | `getRelay()` returns null → throw |

---

## Fix

**Phương án A — Hiển thị "Reconnecting" thay vì "Disconnected"**:

Khi server restart, emit `'reconnecting'` status thay vì `'disconnected'` cho các server có `connectionType === 'direct-websocket'` (vì agent sẽ tự reconnect).

```typescript
// dev-server-manager.ts
private handleServerStartup(): void {
  for (const ds of this.store.list()) {
    this.initRuntimeState(ds.id)
    if (ds.connectionType === 'direct-websocket') {
      // Agent sẽ tự reconnect — hiển thị pending thay vì disconnected
      this.setRuntimeState(ds.id, { status: 'connecting' })
    }
  }
}
```

**Phương án B — Persist lastConnectedAt**:

Lưu `lastConnectedAt` vào store. Nếu `now - lastConnectedAt < 5 phút` → giả định "reconnecting" thay vì "disconnected".

**Phương án C — Grace period**:

Không emit `status = 'error'` khi relay call fail trong 30s đầu sau startup — buffer time cho agent reconnect.

---

## Files Liên Quan

| File | Vai trò |
|------|---------|
| `src/main/dev-server/dev-server-manager.ts` | Runtime state management |
| `src/main/dev-server/dev-server-store.ts` | Persisted state (không lưu runtime) |
