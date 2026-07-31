# BUG-DS-005 — relay-websocket Không Có Auto-Reconnect

**ID:** BUG-DS-005  
**Mức độ:** 🟠 High  
**Module:** `DevServerRelayBridge.connectRelayWebSocket()`  
**Phát hiện:** 2026-07-26  
**Status:** 🔴 Open

---

## Mô Tả

relay-websocket mode không có cơ chế tự reconnect. Khi Orca server restart, toàn bộ WebSocket connections (Orca là WS client kết nối đến agent) bị drop. Agent vẫn listen trên port 6799, nhưng Orca không tự reconnect. User phải thao tác thủ công trong UI để restore connection.

---

## Root Cause

**`dev-server-relay-bridge.ts` — `connectRelayWebSocket()`**:

```typescript
private connectRelayWebSocket(rawUrl, opts): Promise<RelayHandshakeInfo> {
  return new Promise((resolve, reject) => {
    const ws = new WebSocket(cleanUrl, { headers: {...} })

    ws.on('open', () => {
      runOrcaInitiatorHandshake(ws, orcaVersion)
        .then((info) => {
          const transport = createWebSocketTransport(ws)
          this.session = new SshChannelMultiplexer(transport)
          resolve(...)
        })
    })

    // ← KHÔNG có ws.on('close') → reconnect logic
    // ← KHÔNG có ws.on('error') sau khi connected
    // Sau khi resolve(), không ai theo dõi ws state nữa
  })
}
```

Sau khi `resolve()` được gọi, Promise hoàn tất và không còn xử lý gì khi `ws` bị đóng.

---

## Tái Hiện

1. agent.js đang chạy relay-ws mode trên 172.20.2.31:6799
2. Orca kết nối → connected ✅
3. `docker restart orca-server` (10-20s restart time)
4. Sau restart: Orca UI hiển thị "Disconnected"
5. Không có tự động reconnect — phải bấm Connect lại trong UI

---

## So Sánh với direct-websocket

| | direct-websocket | relay-websocket |
|--|--|--|
| **Reconnect** | ✅ Auto (systemd exit(2) → fresh token) | ❌ Manual |
| **Agent side** | Agent exit và restart | Agent vẫn running, vẫn listen |
| **Orca side** | Không cần làm gì | Cần manually re-initiate |

---

## Hậu Quả

- Mọi Orca server restart đều yêu cầu thao tác thủ công
- Nếu restart xảy ra đêm/ngoài giờ → dev server offline cho đến khi ai đó reconnect
- relay-ws mode kém reliable hơn direct-ws

---

## Fix

Thêm reconnect loop vào `connectRelayWebSocket()`:

```typescript
private connectRelayWebSocket(rawUrl, opts): Promise<RelayHandshakeInfo> {
  return new Promise((resolve, reject) => {
    let resolved = false;

    const attempt = () => {
      const ws = new WebSocket(cleanUrl, { headers: {...} })

      ws.on('open', () => {
        runOrcaInitiatorHandshake(ws, orcaVersion)
          .then((info) => {
            const transport = createWebSocketTransport(ws)
            this.session = new SshChannelMultiplexer(transport)

            // Monitor disconnect
            ws.on('close', () => {
              if (this.session) {
                this.session = null
                console.log('[RelayBridge] relay-ws disconnected, reconnecting in 10s...')
                setTimeout(attempt, 10_000)
              }
            })

            if (!resolved) {
              resolved = true
              resolve({ platform: ..., ... })
            }
          })
          .catch(reject)
      })

      ws.on('error', (err) => {
        if (!resolved) reject(err)
        else {
          console.warn('[RelayBridge] relay-ws error, retry in 10s:', err.message)
          setTimeout(attempt, 10_000)
        }
      })
    }

    attempt()
  })
}
```

---

## Files Liên Quan

| File | Dòng | Vai trò |
|------|------|---------|
| `src/main/dev-server/dev-server-relay-bridge.ts` | L270-331 | Bug location |
