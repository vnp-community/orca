# Dev Server Connection Types

> **Scope:** Mô tả chi tiết 3 cơ chế kết nối giữa Orca App và Dev Server.
>
> **Key files:**
> - [`src/main/dev-server/dev-server-relay-bridge.ts`](../../src/main/dev-server/dev-server-relay-bridge.ts) — Bridge layer cho DevServerManager
> - [`src/main/dev-server/dev-server-manager.ts`](../../src/main/dev-server/dev-server-manager.ts) — Lifecycle quản lý dev server
> - [`src/main/ssh/ssh-relay-deploy.ts`](../../src/main/ssh/ssh-relay-deploy.ts) — Deploy + launch relay qua SSH
> - [`src/main/ssh/ssh-channel-multiplexer.ts`](../../src/main/ssh/ssh-channel-multiplexer.ts) — JSON-RPC multiplexer trên SSH transport
> - [`src/relay/relay.ts`](../../src/relay/relay.ts) — Relay daemon entry point (chạy trên remote host)

---

## Tổng quan

Orca hỗ trợ 3 loại kết nối từ App đến Dev Server, được định nghĩa trong `connectionType` của `PersistedDevServer`:

```
┌─────────────────────────────────────────────────────────────────────────┐
│                          Orca App (Local / Server)                       │
│                                                                          │
│   DevServerManager                                                       │
│       └─► DevServerRelayBridge                                           │
│               │                                                          │
│               ├─── 1. relay-ssh ────────── SSH exec channel ───────────►│
│               │         (stdin/stdout, JSON-RPC framed)                  │
│               │                                                          │
│               ├─── 2. relay-websocket ─── ws:// ──────────────────────►│
│               │         (planned Phase 2)                                │
│               │                                                          │
│               └─── 3. direct-websocket ── ws:// ──────────────────────►│
│                         (planned Phase 2)                                │
└──────────────────────────────────────────────────────────────────────────┘
                                          ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                            Dev Server (Remote Host)                      │
│                                                                          │
│   relay.js daemon                                                        │
│       ├── PtyHandler   (terminal sessions)                               │
│       ├── FsHandler    (filesystem operations)                           │
│       ├── GitHandler   (git operations)                                  │
│       └── relay.sock  ◄── Unix domain socket (reconnect bridge)         │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 1. SSH — `relay-ssh` ✅ Phase 1 (Đang hoạt động)

### Mô tả

Là transport **chính thức và duy nhất được implement**. Orca deploy một Node.js script (`relay.js`) lên remote host qua SFTP, khởi động nó như **daemon nền**, rồi giao tiếp qua `stdin/stdout` của SSH exec channel.

### Luồng thiết lập kết nối

```
Orca App                                              Dev Server (Remote Host)
    │                                                       │
    │──(1) SSH connect (ssh2 npm / system ssh) ────────────►│
    │                                                       │
    │──(2) Detect platform: `uname -sm` ───────────────────►│
    │◄──── "Linux arm64" / "Darwin x86_64" etc. ────────────│
    │                                                       │
    │──(3) Check version:                                    │
    │      read ~/.orca-remote/v<hash>/.version ───────────►│
    │◄──── match → skip upload / mismatch → upload ─────────│
    │                                                       │
    │──(4a) SFTP upload relay package ─────────────────────►│
    │       (uploadDirectory qua sftp sub-channel)          │
    │──(4b) npm install node-pty @parcel/watcher ──────────►│
    │       (native addons biên dịch trên remote)            │
    │                                                       │
    │──(5) Launch daemon (fire-and-forget):                  │
    │      nohup node relay.js --detached                   │
    │        --grace-time 300                               │
    │        --sock-path ~/.orca-remote/.../relay.sock      │
    │        > relay.log 2>&1 </dev/null & ────────────────►│
    │                                                       │  relay.js khởi động
    │                                                       │  bind relay.sock
    │──(6) Poll socket ready (mỗi 200ms, tối đa 10s):       │
    │      node -e 'net.connect("relay.sock")...' ─────────►│
    │◄──── READY ────────────────────────────────────────────│
    │                                                       │
    │──(7) SSH exec (connect bridge):                        │
    │      node relay.js --connect                          │
    │        --sock-path relay.sock ───────────────────────►│
    │◄──── "ORCA-RELAY v0.1.0 READY\n" (RELAY_SENTINEL) ────│
    │                                                       │
    │──(8) Handshake frame (version check) ────────────────►│
    │◄──── handshake-ok + {platform, arch, nodeVersion} ────│
    │                                                       │
    │      SshChannelMultiplexer attached                   │
    │      ←──── JSON-RPC 2.0 (framed) ────────────────────►│
```

### Wire Protocol

```
Frame Header (13 bytes — VS Code PersistentProtocol format):
 [0]    : MessageType  (1=Regular, 9=KeepAlive)
 [1-4]  : Outgoing sequence number (uint32 BE)
 [5-8]  : Ack number (uint32 BE)
 [9-12] : Payload length (uint32 BE)

Payload: JSON-RPC 2.0
```

- **Keepalive**: gửi mỗi 5 giây (`KEEPALIVE_SEND_MS = 5_000`)
- **Timeout**: không nhận dữ liệu sau 20 giây → connection lost (`TIMEOUT_MS = 20_000`)
- **Max message size**: 16 MB

### Ví dụ JSON-RPC calls

```json
// Request từ Orca → relay
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "fs.readDir",
  "params": { "path": "/home/ubuntu/code" }
}

// Response từ relay → Orca
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": { "entries": [...] }
}

// Notification (không có id) — relay → Orca
{
  "jsonrpc": "2.0",
  "method": "fs.changed",
  "params": { "path": "/home/ubuntu/code", "type": "modified" }
}
```

### Versioned install dirs

Mỗi relay version có thư mục riêng (content-hashed), tránh conflict khi upgrade:

```bash
~/.orca-remote/
  relay-0.1.0+abc123def/    # version cũ → GC tự dọn sau 24h
  relay-0.1.0+9f8e7d6c5/    # version hiện tại
    relay.js                 # ~84KB compiled bundle
    node/                    # bundled node binary (một số platform)
    node_modules/
      node-pty/              # native addon: terminal emulation
      @parcel/watcher/       # native addon: filesystem watch
    .version                 # "0.1.0+9f8e7d6c5"
    .install-complete        # marker: npm install hoàn tất
    relay.sock               # Unix domain socket
    relay.log                # rotated logs
```

### Grace period & Reconnect

```
Orca App restart / SSH channel đóng:
    relay.js nhận stdin.end()
    ──► stdoutAlive = false
    ──► startGrace("stdin ended")
    ──► graceTimer bắt đầu (mặc định 5 phút)
    relay.sock vẫn listening, PTY sessions sống

Orca reconnect:
    ──► probe: test -S relay.sock
    ──► ALIVE → ssh exec node relay.js --connect --sock-path relay.sock
                 (bridge SSH stdio ↔ relay.sock)
    ──► DEAD  → launch relay mới (step 5-7 ở trên)
```

**Grace time cấu hình**: `DEFAULT_SSH_RELAY_GRACE_PERIOD_SECONDS` (trong `src/shared/ssh-types.ts`)

### Files triển khai

| File | Vai trò |
|------|---------|
| [`ssh-relay-deploy.ts`](../../src/main/ssh/ssh-relay-deploy.ts) | Deploy pipeline: detect, upload, npm install, launch |
| [`ssh-relay-session.ts`](../../src/main/ssh/ssh-relay-session.ts) | Lifecycle relay session: connect, reconnect, dispose |
| [`ssh-channel-multiplexer.ts`](../../src/main/ssh/ssh-channel-multiplexer.ts) | JSON-RPC framing, keepalive, timeout, pending requests |
| [`dev-server-relay-bridge.ts`](../../src/main/dev-server/dev-server-relay-bridge.ts) | Adapter cho DevServerManager (`relay-ssh` mode) |
| [`relay.ts`](../../src/relay/relay.ts) | Relay daemon (chạy trên remote): handlers, socket server |
| [`relay-handshake.ts`](../../src/relay/relay-handshake.ts) | Version handshake (daemon + --connect sides) |

---

## 2. WebSocket — `relay-websocket` / `direct-websocket` ❌ Phase 2 (Chưa implement)

### Mô tả

Hai sub-type kết nối WebSocket, dự kiến implement sau Phase 1. Hiện tại code throw error nếu dùng:

```typescript
// dev-server-relay-bridge.ts, line 85-89
throw new Error(
  `Connection type '${this.config.connectionType}' is not yet implemented. ` +
    `Only 'relay-ssh' is supported in Phase 1.`
)
```

### 2a. `relay-websocket`

- Dev server có relay daemon chạy và lắng nghe WebSocket
- Orca App kết nối `ws://dev-server:<port>` tới relay WebSocket server
- **Hướng**: Orca App → Dev Server relay (pull-based, Orca chủ động connect)
- Phù hợp khi không có SSH access nhưng có network access

### 2b. `direct-websocket`

- Dev server **là** một Orca Server instance (`node out/server/index.js`)
- Orca App kết nối trực tiếp `ws://dev-server:6768`
- **Hướng**: Orca App → Orca Server on Dev Server
- Phù hợp cho môi trường cloud / containerized

### Orca Server WebSocket endpoint

Khi chạy ở server mode (`src/server/index.ts`):

```
ws://0.0.0.0:6768   ← WebSocket/RPC endpoint  (ORCA_PORT)
http://0.0.0.0:6769 ← HTTP: web UI + push API  (ORCA_HTTP_PORT)
```

Cấu hình qua biến môi trường:

| Env var | Default | Mô tả |
|---------|---------|-------|
| `ORCA_PORT` | `6768` | WebSocket/RPC port |
| `ORCA_HTTP_PORT` | `ORCA_PORT + 1` | HTTP port |
| `ORCA_USER_DATA_PATH` | `~/.orca` | Data directory |
| `ORCA_MULTI_USER` | unset | `'1'` để enable per-user process isolation |
| `ORCA_DB_URL` | SQLite | Database DSN |

---

## 3. Unix Domain Socket — `relay.sock` 🔄 Nội bộ relay daemon

### Mô tả

Đây **không phải loại kết nối người dùng cấu hình**. Đây là cơ chế **nội bộ của relay daemon** cho phép sống sót qua nhiều SSH session mà không mất PTY state.

Mục đích: sau khi app restart, SSH channel mới cần "attach" vào relay process đang chạy — thay vì spawn relay mới và mất toàn bộ terminal sessions.

### Cách relay.sock được tạo

```typescript
// relay.ts, hàm startSocketServer()

// 1. Set umask 0o177 trước khi listen → socket tạo với mode 0o600
const prevUmask = process.umask(0o177)
// 2. Bind Unix domain socket
server.listen(sockPath)  // ~/.orca-remote/.../relay.sock
// 3. Restore umask
process.umask(prevUmask)
```

### Luồng reconnect qua socket

```
Lần kết nối đầu (SSH exec channel #1):
    relay.js main()
    ├── bind relay.sock
    ├── ghi RELAY_SENTINEL ra stdout
    └── dispatcher ←──── stdin/stdout (SSH #1) ──────────►

SSH channel #1 đóng (app restart):
    stdin.end() event
    ├── stdoutAlive = false
    └── startGrace("stdin ended")
        graceTimer: 5 phút (PTYs vẫn sống)

Lần kết nối lại (SSH exec channel #2):
    Orca: probe test -S relay.sock → ALIVE
    Orca: ssh exec "node relay.js --connect --sock-path relay.sock"
    
    --connect mode (runConnectMode):
    ├── createConnection(relay.sock)  ← kết nối tới daemon
    ├── gửi handshake frame {version}
    ├── nhận handshake-ok
    ├── ghi RELAY_SENTINEL ra stdout (SSH #2)
    ├── stdin.pipe(sock)   ── SSH stdin → relay socket
    └── sock.pipe(stdout)  ── relay socket → SSH stdout
    (bridge hoàn toàn 2 chiều)
```

### Socket server trong relay daemon

```
relay.ts (line 676-942): socket server
├── createServer() → bind relay.sock
├── Mỗi client kết nối:
│   ├── setupDaemonHandshake() → verify version match
│   ├── attachAcceptedSocket():
│   │   ├── process.stdin.pause() (dừng nhận từ SSH channel cũ)
│   │   ├── dispatcher.attachClient(sock.write) → socket thay stdin
│   │   └── sock.on('data') → dispatcher.feedClient()
│   └── sock.on('close'):
│       ├── dispatcher.detachClient()
│       └── nếu không còn client → startGrace()
└── Stale socket detection:
    ├── EADDRINUSE → probe connect (timeout 500ms)
    ├── ECONNREFUSED → socket stale → unlink → retry listen
    └── connect OK → live relay → không ghi đè
```

### Các trường hợp sử dụng socket

| Trường hợp | Mô tả |
|-----------|-------|
| App restart bình thường | `--connect` bridge SSH stdio ↔ relay.sock |
| Nhiều Orca windows | Nhiều `--connect` → nhiều socket clients đồng thời |
| Remote CLI (`orca` lệnh) | `--orca-cli` mode: kết nối relay.sock, gọi `orca.cli` method |
| Relay crash | ECONNREFUSED → stale socket → unlink → fresh launch |

---

## So sánh ba loại

| Tiêu chí | SSH (`relay-ssh`) | WebSocket (Phase 2) | Unix Socket (nội bộ) |
|---------|-------------------|---------------------|----------------------|
| **Transport** | SSH exec channel stdin/stdout | TCP WebSocket | Unix domain socket file |
| **Phạm vi** | Cross-host (Internet/LAN) | Cross-host | Chỉ trong remote host |
| **Mục đích** | Kết nối chính Orca↔relay | Kết nối trực tiếp | Reconnect bridge nội bộ |
| **Deploy** | SCP + npm install | Không cần | Không cần |
| **Auth** | SSH credentials/key | Token/Auth | Unix file permissions (0o600) |
| **Implemented** | ✅ Phase 1 | ❌ Phase 2 (planned) | ✅ Built-in relay daemon |
| **Protocol** | JSON-RPC 2.0 + 13-byte frame | JSON-RPC 2.0 (planned) | JSON-RPC 2.0 + 13-byte frame |
| **Key files** | `ssh-relay-deploy.ts`, `ssh-relay-session.ts` | `dev-server-relay-bridge.ts` (stub) | `relay.ts` lines 670-942 |

---

## Trạng thái relay daemon

```
  [START]
     │
     ▼
  bind relay.sock
     │
     ▼
  write RELAY_SENTINEL → stdout (SSH channel)
     │
     ▼
  [ACTIVE: stdin/stdout transport]
     │
     │ stdin.end() or stdout error
     ▼
  [GRACE PERIOD: graceTimer]
     │   ▲
     │   └── cancelGrace() ← socket client connects (--connect)
     │
     │ grace expired (no client reconnected)
     ▼
  shutdown() → process.exit(0)
```
