# BUG-TRM-AG-001 — Relay Session Null: Dev Server Agent Chưa Kết Nối

**ID:** BUG-TRM-AG-001
**Mức độ:** 🔴 Critical
**Module:** Dev Server Agent connection (direct-websocket)
**Phát hiện:** 2026-07-31
**Status:** 🔴 Open

---

## Mô Tả

Khi user mở Terminal trong project `test-repo`, browser gửi `terminal.create` RPC. Orca Server route request xuống User Process → `ptyController.spawn({ connectionId })`. Tại đây, `DevServerRelayBridge.callWithTimeout('pty.spawn', ...)` được gọi nhưng `this.session` là `null` vì **Dev Server Agent chưa connect inbound vào Orca Server**.

Kết quả: terminal.create fail với lỗi `"Not connected"`.

---

## Root Cause

**Kiến trúc direct-websocket** yêu cầu Dev Server Agent **chủ động kết nối INBOUND** vào Orca Server:

```
Orca Server (AgentWebSocketServer)     Dev Server Agent
  registerSlot(agentToken)          ←─────────────────
  GET /agent WS endpoint
         ▲
         │  ws://b15.openledger.vn/agent?token=<token>
         └─────────── Agent connects inbound ──────────
```

**Vấn đề:** `DevServerRelayBridge.callWithTimeout()` tại [`dev-server-relay-bridge.ts:544-548`](../../../../src/main/dev-server/dev-server-relay-bridge.ts):

```typescript
if (!session) {
  const span = relayCallTracer.start({ devServerId: this.config.id, method })
  span.fail('Not connected', { method, devServerId: this.config.id })
  throw new Error('Not connected')
}
```

`this.session` là `null` khi:
1. Agent chưa bao giờ kết nối (chưa start service trên remote host)
2. Agent connect thất bại do `agentToken` expired (slot TTL 60s)
3. Agent WS bị disconnect và chưa reconnect lại (`_reconnecting = false`)

---

## Tái Hiện

1. Dev Server `test-repo` được thêm vào Orca với `connectionType: 'direct-websocket'`
2. Agent process trên remote host chưa start (hoặc đã crash)
3. User mở Terminal pane trong project `test-repo`
4. `callRuntime('terminal.create', ...)` được gửi từ browser

**Kết quả:** Terminal pane hiện lỗi — `runtimeTerminalErrorMessage(error)` được gọi với `"Not connected"`.

**Trace log:**
```
[TRACE] relay:agentCall devServerId=<id> method=pty.spawn → FAIL 'Not connected'
```

---

## Hậu Quả

- Terminal **hoàn toàn không mở được** cho bất kỳ project nào dùng dev server đó
- User không có thông báo rõ ràng về nguyên nhân (chỉ thấy terminal error)
- Không có cơ chế retry hay fallback

---

## Fix Đề Xuất

### Phương án A — Hiển thị trạng thái kết nối agent rõ ràng (UX)

Trả về error code cụ thể thay vì generic `"Not connected"`:

```typescript
// dev-server-relay-bridge.ts
span.fail('agent_not_connected', { method, devServerId: this.config.id })
throw Object.assign(new Error('agent_not_connected'), {
  devServerId: this.config.id,
  hint: 'Start the Orca agent on the dev server and reconnect'
})
```

### Phương án B — Health check định kỳ + auto-reconnect slot

`AgentWebSocketServer` tự động re-emit `agentTokenGenerated` event khi slot expired, cho phép agent tự reconnect.

### Phương án C — Persistent agent connection (không dùng slot TTL)

Đăng ký slot không có timeout, agent giữ kết nối WebSocket dài hạn với Orca Server.

---

## Files Liên Quan

| File | Vị trí | Vai trò |
|------|--------|---------|
| [`dev-server-relay-bridge.ts`](../../../../src/main/dev-server/dev-server-relay-bridge.ts) | `callWithTimeout()` L513-590 | Throw `Not connected` khi session null |
| [`agent-ws-server.ts`](../../../../src/main/dev-server/agent-ws-server.ts) | `registerSlot()` | Slot registration với 60s TTL |
| [`ws-handshake.ts`](../../../../src/main/dev-server/ws-handshake.ts) | `runOrcaInitiatorHandshake` | Handshake khi agent connect |
| [`relay/pty-handler.ts`](../../../../src/relay/pty-handler.ts) | `pty.spawn` handler | PTY spawn trên remote host |

---

## ✅ Fix Status: RESOLVED (2026-08-01)

**Fix:** dev-server-relay-bridge.ts: Session null check returns AGENT_NOT_CONNECTED error. mux.onDispose() clears session reference.
