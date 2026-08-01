# Orca Agent — Luồng Kết Nối Chi Tiết

> **Tài liệu này mô tả 2 loại agent**, luồng kết nối đầy đủ, wire protocol,
> và hướng dẫn triển khai thực tế qua các scripts trong `deploy/dev/scripts/`.

---

## Tổng quan: 2 Mode kết nối

| Đặc điểm | Mode 1: `direct-websocket` | Mode 2: `relay-websocket` |
|-----------|---------------------------|--------------------------|
| **Ai khởi tạo WS** | Agent → Orca Server | Orca Server → Agent |
| **Agent vai trò** | WebSocket **client** | WebSocket **server** |
| **Agent cần mở port** | ❌ Không cần | ✅ Port `6799` phải accessible |
| **Token** | One-time (từ `/api/agent-token`, TTL 600s) | Static secret (`AGENT_RELAY_TOKEN`) |
| **Daemon** | ✅ systemd (`orca-agent-direct.service`) | `nohup` hoặc systemd riêng |
| **Phù hợp khi** | Agent sau firewall/NAT | Dev server trong LAN, port accessible |
| **Script chính** | `start-agent-direct.sh` | `connect-agent.sh --mode relay-ws` |
| **Orca UI config** | Không cần (API tự đăng ký) | Thêm URL + token trong Dev Servers |

---

## Hạ tầng thực tế

```
┌─────────────────────────────────────────────────────────────────────┐
│  Dev Machine (MacBook — nơi chạy scripts)                           │
│  bash deploy/dev/scripts/*.sh                                       │
└──────────────┬──────────────────────────────────────┬──────────────┘
               │ SSH                                  │ SSH
               ▼                                      ▼
┌──────────────────────────┐          ┌───────────────────────────────┐
│  Dev Server (172.20.2.31)│          │  Orca Server (172.20.2.39)    │
│  ubuntu@for-dev          │          │  Docker: orca-server          │
│                          │          │  Port 6769 (HTTP + WS /agent) │
│  ~/orca-agent/           │          │                               │
│    agent.js              │          │  Gateway (172.20.2.16)        │
│    start.sh (direct)     │          │  Nginx reverse proxy:         │
│    logs/agent-direct.log │          │  wss://b15.openledger.vn      │
└──────────────────────────┘          └───────────────────────────────┘
```

---

## Mode 1: `direct-websocket` — Agent chủ động kết nối

### Topology

```
Dev Machine                Dev Server (172.20.2.31)   Orca Server (172.20.2.39)
     │                              │                          │
     │ bash start-agent-direct.sh   │                          │
     │──────SSH: deploy────────────►│                          │
     │   1. Copy start.sh           │                          │
     │   2. Install systemd service │                          │
     │   3. systemctl start         │                          │
     │                              │                          │
     │                    ┌─────────┘                          │
     │                    │ systemd starts start.sh            │
     │                    │                                    │
     │                    │── POST /api/agent-token ──────────►│
     │                    │   {devServerId, name, ttl:600}     │
     │                    │                                    │
     │                    │                         connectDaemonAgent()
     │                    │                         bridge = RelayBridge
     │                    │                         registerSlot(token, cb)
     │                    │                         pendingSlots[token] = slot
     │                    │                                    │
     │                    │◄── {token, expiresIn:600} ────────│
     │                    │                                    │
     │                    │── exec node agent.js ─────────────────────────────►
     │                    │   AGENT_TOKEN=agt-dev-local-xxx   │              │
     │                    │   ORCA_URL=wss://b15.../agent     │              │
     │                    │                                    │              │
     │                    │                              WebSocket connect    │
     │                    │                              ◄──────────────────│
     │                    │                              Nginx → :6769/agent │
     │                    │                                    │              │
     │                    │                              HANDSHAKE frame     │
     │                    │                              ◄──────────────────│
     │                    │                    type=0x01 "agent.handshake"   │
     │                    │                    {agentToken, tools, version}  │
     │                    │                                    │              │
     │                    │                    validateToken() → slot found  │
     │                    │                    removeSlot(token)             │
     │                    │                    slot.callback(mux, info)      │
     │                    │                    bridge.session = mux ✅       │
     │                    │                                    │              │
     │                    │                              HANDSHAKE_OK        │
     │                    │                              ──────────────────►│
     │                    │                    type=0x01 {ok:true,sessionId} │
     │                    │                                    │              │
     │                    │              ◄═══════════════════════════════════►
     │                    │              CONNECTED: bidirectional JSON-RPC   │
```

### Triển khai qua Scripts

#### Script 1: `start-agent-direct.sh` ← **Khuyến nghị cho production daemon**

```bash
# Chạy từ thư mục gốc repo trên Dev Machine:
bash deploy/dev/scripts/start-agent-direct.sh
```

**Những gì script làm:**

```
start-agent-direct.sh
  │
  ├─ [1] Tạo start.sh wrapper (từ heredoc) tại /tmp/orca-agent-start.sh
  │      Nội dung start.sh:
  │        #!/usr/bin/env bash
  │        curl -sf POST http://172.20.2.39:6769/api/agent-token → TOKEN
  │        exec env ORCA_URL=... AGENT_TOKEN=$TOKEN node agent.js
  │
  ├─ [2] Tạo systemd service file tại /tmp/orca-agent-direct.service
  │      [Service]
  │      ExecStart=/bin/bash ~/orca-agent/start.sh
  │      Restart=always
  │      RestartSec=5
  │      StandardOutput=append:~/orca-agent/logs/agent-direct.log
  │
  ├─ [3] SCP cả 2 files lên Dev Server (172.20.2.31)
  │
  └─ [4] SSH vào Dev Server, setup systemd:
         mkdir -p ~/orca-agent/logs
         cp start.sh → ~/orca-agent/start.sh
         sudo mv service → /etc/systemd/system/orca-agent-direct.service
         sudo systemctl daemon-reload
         sudo systemctl enable orca-agent-direct
         sudo systemctl restart orca-agent-direct
```

**Sau khi chạy — kiểm tra:**

```bash
# Kiểm tra service status
ssh ubuntu@172.20.2.31 "sudo systemctl status orca-agent-direct"

# Xem logs agent
ssh ubuntu@172.20.2.31 "tail -f ~/orca-agent/logs/agent-direct.log"

# Restart thủ công
ssh ubuntu@172.20.2.31 "sudo systemctl restart orca-agent-direct"
```

#### Script 2: `connect-agent.sh` (direct-ws) ← **Deploy một lần, chạy nohup**

```bash
# Chỉ in hướng dẫn (sinh token, user tự chạy thủ công):
bash deploy/dev/scripts/connect-agent.sh

# Deploy agent files + sinh token + start nohup:
bash deploy/dev/scripts/connect-agent.sh --deploy --start

# Chỉ sinh token + start (nếu đã deploy):
bash deploy/dev/scripts/connect-agent.sh --start

# Kiểm tra / dừng / xem logs:
bash deploy/dev/scripts/connect-agent.sh --status
bash deploy/dev/scripts/connect-agent.sh --stop
bash deploy/dev/scripts/connect-agent.sh --logs
```

**Những gì `--deploy --start` làm:**

```
connect-agent.sh --deploy --start
  │
  ├─ deploy_agent():
  │    rsync deploy/dev/agent/ → ubuntu@172.20.2.31:~/orca-agent/
  │    ssh: npm install --production
  │
  ├─ generate_agent_token():
  │    ssh ubuntu@172.20.2.39 "curl POST http://172.20.2.39:6769/api/agent-token"
  │    → {token: "agt-dev-local-xxx", expiresIn: 300}
  │
  └─ start_agent_direct(token):
       ssh ubuntu@172.20.2.31:
         AGENT_TOKEN='agt-dev-local-xxx' \
         ORCA_URL='wss://b15.openledger.vn/agent' \
         MODE='direct-websocket' \
         nohup node agent.js > logs/agent.log 2>&1 &
         echo $! > logs/agent.pid
```

> [!WARNING]
> `connect-agent.sh --start` dùng **nohup** (không persistent qua reboot và không auto-restart). Dùng `start-agent-direct.sh` để có systemd daemon với `Restart=always`.

### Daemon Lifecycle (systemd)

```
                systemd orca-agent-direct.service
                Restart=always, RestartSec=5
                         │
          ┌──────────────┼────────────────────────┐
          │              │                         │
     Boot/Start   Connection drop          Token rejected
          │              │                  (code=1005)
          ▼              │                         │
    start.sh runs        │                         │
    curl /api/token      │            lastConnHandshakeOk=false
    exec node agent.js   │            ws.on('close') → exit(2)
          │              │                         │
          │    lastConnHandshakeOk=true             │
          │    ws.on('close') → exit(2)            │
          │              │                         │
          └──────────────┴─────────────────────────┘
                         │
                   systemd detects exit(2)
                   waits RestartSec=5
                   → start.sh runs again
                   → fresh token from API
                   → new WS connection ✅
```

**Trạng thái module-level trong agent.js:**

```javascript
// Module-level (persist qua các lần connectDirect() call)
let lastConnHandshakeOk = false;

function connectDirect() {
  const ws = new WebSocket(ORCA_URL, ...);
  handleSession(ws, true, null);  // reset lastConnHandshakeOk = false

  ws.on('close', (code) => {
    if (code === 1000) process.exit(0);     // Clean shutdown
    // Tất cả close khác → exit(2) → systemd restart → fresh token
    if (lastConnHandshakeOk) {
      log.warn('Connection dropped after handshake — exiting for fresh token');
    } else {
      log.error('Closed before handshake — token rejected/expired');
    }
    setTimeout(() => process.exit(2), 200);
  });
}
```

---

## Mode 2: `relay-websocket` — Orca chủ động kết nối

### Topology

```
Dev Machine              Dev Server (172.20.2.31)   Orca Server (172.20.2.39)
     │                            │                          │
     │ bash connect-agent.sh      │                          │
     │   --mode relay-ws          │                          │
     │   --deploy --start         │                          │
     │──── SSH: rsync agent ─────►│                          │
     │──── SSH: npm install ──────►│                          │
     │──── SSH: start nohup ──────►│                          │
     │                            │                          │
     │                   node agent.js                       │
     │                   MODE=relay-websocket                │
     │                   WebSocketServer :6799/orca-relay    │
     │                   Waiting for Orca to connect...      │
     │                            │                          │
     │                            │                          │
     │  User thêm Dev Server      │                          │
     │  trong Orca UI:            │                          │
     │  Type: relay-websocket     │                          │
     │  URL: ws://172.20.2.31     │                          │
     │  :6799/orca-relay          │                          │
     │  ?token=relay-secret       │                          │──────────────────►│
     │                            │              DevServerManager.connect(id)    │
     │                            │              bridge.connectRelayWebSocket()  │
     │                            │◄────── WebSocket connect ──────────────────│
     │                            │   ws://172.20.2.31:6799/orca-relay          │
     │                            │   ?token=relay-secret                       │
     │                            │                                             │
     │                    validateToken(relay-secret) → OK                     │
     │                            │                                             │
     │                            │───── HANDSHAKE_OK ─────────────────────────►│
     │                            │  type=0x01 {ok:true, tools:[...]}           │
     │                            │                                             │
     │                            │              runOrcaInitiatorHandshake()    │
     │                            │              bridge.session = mux ✅        │
     │                            │              status = "connected"           │
     │                            │◄═══════ bidirectional JSON-RPC ═════════════►
```

### Triển khai qua Script

#### `connect-agent.sh --mode relay-ws`

```bash
# In hướng dẫn (không deploy/start):
bash deploy/dev/scripts/connect-agent.sh --mode relay-ws

# Deploy agent + start nohup relay mode:
bash deploy/dev/scripts/connect-agent.sh --mode relay-ws --deploy --start

# Kiểm tra:
bash deploy/dev/scripts/connect-agent.sh --status
bash deploy/dev/scripts/connect-agent.sh --logs
```

**Những gì `--mode relay-ws --deploy --start` làm:**

```
connect-agent.sh --mode relay-ws --deploy --start
  │
  ├─ deploy_agent():
  │    rsync deploy/dev/agent/ → ubuntu@172.20.2.31:~/orca-agent/
  │    ssh: npm install --production
  │
  └─ start_agent_relay():
       ssh ubuntu@172.20.2.31:
         AGENT_PORT='6799' \
         AGENT_TOKEN='relay-secret' \
         MODE='relay-websocket' \
         DEV_SERVER_ID='dev-local' \
         nohup node agent.js > logs/agent.log 2>&1 &
         echo $! > logs/agent.pid
```

**Output hướng dẫn (khi chạy không `--start`):**

```
═══════════════════════════════════════════════════════════
 🔗 Orca Agent — relay-websocket mode
═══════════════════════════════════════════════════════════

Dev Server: 172.20.2.31:6799
Token:      relay-secret

1. Chạy agent trên dev server:
  AGENT_PORT=6799 \
  AGENT_TOKEN=relay-secret \
  MODE=relay-websocket \
  node agent.js

2. Trong Orca UI, thêm Dev Server:
   Connection Type: relay-websocket
   WebSocket URL:   ws://172.20.2.31:6799/orca-relay?token=relay-secret
```

**Cấu hình trong Orca UI:**

```
Settings → Dev Servers → Add Dev Server
  Name:            dev-local
  Connection Type: relay-websocket
  WebSocket URL:   ws://172.20.2.31:6799/orca-relay?token=relay-secret
```

### Code: agent.js listenRelay()

```javascript
// agent.js — MODE=relay-websocket
function listenRelay() {
  const token = AGENT_TOKEN || 'relay-secret';
  const wss = new WebSocketServer({ port: AGENT_PORT, path: '/orca-relay' });

  wss.on('listening', () => {
    log.info(`✅ Ready: ws://0.0.0.0:${AGENT_PORT}/orca-relay`);
    log.info(`In Orca UI: Type=relay-websocket  URL=ws://${DEV_ID}:${AGENT_PORT}/orca-relay?token=${token}`);
  });

  wss.on('connection', (ws, req) => {
    const url    = new URL(`ws://localhost${req.url}`);
    const qToken = url.searchParams.get('token');
    const bToken = (req.headers['authorization'] || '').replace(/^Bearer\s+/i, '');
    const inToken = qToken || bToken;

    if (inToken !== token) {
      ws.close(1008, 'Unauthorized');  // Token không khớp
      return;
    }

    // Orca connected → handle như relay session (isInitiator=false)
    // Orca sẽ gửi HANDSHAKE frame đến agent
    handleSession(ws, false, token);
  });
}
```

### Code: server-side (Orca → Agent)

```typescript
// src/main/dev-server/dev-server-relay-bridge.ts
connectRelayWebSocket(opts): Promise<RelayHandshakeInfo> {
  const ws = new WebSocket(this.config.wsUrl)  // ws://172.20.2.31:6799/orca-relay?token=xxx

  return runOrcaInitiatorHandshake(ws, this.config.id, orcaVersion)
    .then((info) => {
      const transport = createWebSocketTransport(ws)
      this.session = new SshChannelMultiplexer(transport)
      return info
    })
}

// src/main/dev-server/ws-handshake.ts — runOrcaInitiatorHandshake
// Orca là WS client, gửi handshake trước:
ws.send(encodeJsonRpcFrame({
  method: 'agent.handshake',
  params: { agentToken, orcaVersion }
}, seq, ack))
// Chờ agent reply { result: { ok: true } }
```

---

## Wire Protocol — Binary Frame Format

Cả 2 mode dùng chung binary framing (13-byte header):

```
┌──────────┬────────────────┬────────────────┬─────────────────┬──────────────────┐
│  TYPE    │  SEQ (u32 BE)  │  ACK (u32 BE)  │  LENGTH (u32 BE)│   PAYLOAD        │
│  (1 byte)│  (4 bytes)     │  (4 bytes)     │  (4 bytes)      │  (LENGTH bytes)  │
└──────────┴────────────────┴────────────────┴─────────────────┴──────────────────┘
  Total header: 13 bytes

Frame Types (relay-protocol.ts MessageType):
  0x01  Regular   — JSON-RPC payload: handshake, data, tools/call, tools/list
  0x09  KeepAlive — Keepalive ping/pong (empty payload)
```

> [!IMPORTANT]
> **Server chỉ process frame type `0x01` (Regular)**. Mọi type khác bị `FrameDecoder` bỏ qua.
> Agent.js phải gửi **tất cả frames với type=0x01** — kể cả HANDSHAKE và DATA.
> Frame type `0x09` chỉ dùng cho keepalive.

**Trong agent.js (sau fix):**

```javascript
const FRAME_TYPE = {
  DATA:         0x01,  // Regular — JSON-RPC requests/responses
  HANDSHAKE:    0x01,  // Regular — agent.handshake method (type phải = 0x01!)
  HANDSHAKE_OK: 0x01,  // Regular — { result: { ok: true } }
  PING:         0x09,  // KeepAlive
  PONG:         0x09,  // KeepAlive
  DISCONNECT:   0x01,  // Regular
};
```

> [!WARNING]
> Vì tất cả FRAME_TYPE đều = `0x01` hoặc `0x09`, agent.js không dùng `switch(frame.type)`
> để phân biệt HANDSHAKE vs DATA. Thay vào đó phân biệt qua JSON-RPC message:
> - `msg.method === 'agent.handshake'` → handshake request (relay-ws server)
> - `msg.result?.ok === true` → handshake OK reply (direct-ws)
> - `msg.method` khác → tools/call, tools/list, ping... → dispatchRpc()

### Handshake Payload (direct-ws: agent → Orca)

```json
// Agent → Orca (frame type=0x01, method=agent.handshake):
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "agent.handshake",
  "params": {
    "agentToken":   "agt-dev-local-1785058753742",
    "devServerId":  "dev-local",
    "capabilities": ["rpc", "tools"],
    "tools":        ["claude_code", "claude_code_file", "gh", "git",
                     "gitnexus", "codegraph", "docker", "shell",
                     "read_file", "list_dir"],
    "version":      "1.0.0"
  }
}

// Orca → Agent (frame type=0x01, result.ok=true):
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "ok":          true,
    "orcaVersion": "1.4.138",
    "sessionId":   "sess-1785058753742-abc123"
  }
}
```

### Handshake Payload (relay-ws: Orca → Agent)

```json
// Orca → Agent (frame type=0x01, Orca gửi trước):
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "agent.handshake",
  "params": {
    "agentToken": "relay-secret",
    "orcaVersion": "1.4.138"
  }
}

// Agent → Orca (frame type=0x01):
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "ok":        true,
    "devServerId": "dev-local",
    "tools":     ["claude_code", "shell", "docker", ...],
    "version":   "1.0.0"
  }
}
```

### Tool Invocation (sau connected — giống nhau ở cả 2 mode)

```json
// Orca → Agent (frame=0x01):
{
  "jsonrpc": "2.0", "id": 42,
  "method": "tools/call",
  "params": {
    "name": "shell",
    "arguments": { "command": "git log --oneline -5", "cwd": "/home/ubuntu/vnp-blc" }
  }
}

// Agent → Orca (frame=0x01):
{
  "jsonrpc": "2.0", "id": 42,
  "result": {
    "content": [{ "type": "text", "text": "abc1234 feat: add feature\n..." }],
    "isError": false,
    "exitCode": 0
  }
}
```

---

## Điểm chung sau khi Connected

Sau handshake, **cả 2 mode hoạt động giống nhau**:

```
Browser UI
    │  JSON-RPC (ws://:6768 → OrcaRuntimeRpcServer)
    ▼
Orca Runtime (SshChannelMultiplexer)
    │  bridge.session = SshChannelMultiplexer ← đây là "tunnel" chung cả 2 mode
    │  JSON-RPC over framed binary WebSocket
    ▼
agent.js (dispatchRpc)
    │
    ├── tools/call: shell   → spawn("bash", ["-c", cmd])
    ├── tools/call: docker  → execFile("docker", args)
    ├── tools/call: git     → execFile("git", args)
    ├── tools/call: claude_code → execFile("claude", ["--print", prompt])
    └── tools/list          → return discoveredTools[]
```

**Server-side**: `bridge.session` = `SshChannelMultiplexer` trong cả 2 mode — Orca code phía trên không phân biệt direct hay relay sau khi connected.

---

## So sánh chi tiết & Khi nào dùng mode nào

```
                    Mode 1: direct-ws           Mode 2: relay-ws
                    ─────────────────           ────────────────
Agent vai trò       WebSocket CLIENT            WebSocket SERVER
Orca vai trò        WebSocket SERVER            WebSocket CLIENT
Firewall agent      ✅ Agent sau NAT OK          ❌ Cần port 6799 mở ra ngoài
Token               One-time TTL 600s           Static secret (file .env)
Daemon              systemd Restart=always      nohup hoặc systemd riêng
Reconnect           Auto (systemd restart)      Agent luôn listen, Orca retry
UI config           Không cần (auto đăng ký)   Thêm URL + token trong UI
Deploy script       start-agent-direct.sh       connect-agent.sh --mode relay-ws
Persistent          ✅ Survive reboot            ⚠️ nohup mất sau reboot
Production          ✅ Recommended              ⚠️ Dev/LAN only
```

### Khuyến nghị

- **Dev server sau firewall/corporate NAT** → `start-agent-direct.sh` (Mode 1, systemd)
- **Dev server trong LAN với port accessible** → `connect-agent.sh --mode relay-ws` (Mode 2)
- **Production/persistent daemon** → Mode 1 với systemd + `Restart=always`
- **Test nhanh, không cần persistent** → `connect-agent.sh --start` (Mode 1, nohup)

---

## Script Reference

### Deploy Orca Server (172.20.2.39)

```bash
# Sync source code + rebuild Docker container:
bash deploy/dev/scripts/sync-to-server.sh
# → rsync src/ → SERVER:~/orca-deploy/
# → docker compose up -d --build
# → health check :6769/health/ready
```

### Deploy Agent — Mode 1: direct-websocket

```bash
# Full daemon setup (systemd, khuyến nghị):
bash deploy/dev/scripts/start-agent-direct.sh

# Hoặc qua connect-agent.sh:
bash deploy/dev/scripts/connect-agent.sh --deploy --start   # nohup
bash deploy/dev/scripts/connect-agent.sh                    # chỉ in token
```

### Deploy Agent — Mode 2: relay-websocket

```bash
# Deploy + start relay agent:
bash deploy/dev/scripts/connect-agent.sh --mode relay-ws --deploy --start

# Chỉ in hướng dẫn:
bash deploy/dev/scripts/connect-agent.sh --mode relay-ws

# Thêm vào Orca UI sau khi agent đang chạy:
# ws://172.20.2.31:6799/orca-relay?token=relay-secret
```

### Operations

```bash
# Xem agent status
bash deploy/dev/scripts/connect-agent.sh --status
ssh ubuntu@172.20.2.31 "sudo systemctl status orca-agent-direct"

# Xem logs
bash deploy/dev/scripts/connect-agent.sh --logs
ssh ubuntu@172.20.2.31 "tail -f ~/orca-agent/logs/agent-direct.log"

# Restart
ssh ubuntu@172.20.2.31 "sudo systemctl restart orca-agent-direct"

# Dừng
bash deploy/dev/scripts/connect-agent.sh --stop
```

## 7. HMAC-SHA256 Signed Context (v6.0)

Sau khi handshake thành công, mọi tool call từ Orca Server đều kèm signed `RpcExecutionContext`:

```typescript
// Orca Server gửi:
{
  "jsonrpc": "2.0", "id": 42,
  "method": "tools/call",
  "params": {
    "name": "shell",
    "arguments": { "command": "git log -5" },
    "_ctx": {
      "userId":    "user-alice-uuid",
      "userName":  "Alice Nguyen",
      "userEmail": "alice@company.com",
      "devServerId": "svr-01",
      "projectId": "proj-vnp-blc",
      "issuedAt":  1722312345000,
      "expiresAt": 1722312375000,   // +30s
      "signature": "hmac-sha256-hex..."
    }
  }
}

// Agent: src/agent/rpc/context-verifier.ts
// 1. Verify HMAC: hmacSHA256(ORCA_RELAY_SECRET, JSON.stringify(ctx_without_sig)) === signature
// 2. Check expiry: Date.now() < ctx.expiresAt
// 3. Attach ctx to request → dùng trong mọi handler
```

---

## 8. Per-userId PTY Isolation (v6.0)

Dev Server Agent dùng `pty-session-store.ts` để cô lập PTY sessions theo userId:

```typescript
// src/agent/pty/pty-session-store.ts
class PtySessionStore {
  // Map: userId → { ptyId → PtySession }
  private sessions = new Map<string, Map<string, PtySession>>()

  create(userId: string, ptyId: string, session: PtySession): void {
    if (!this.sessions.has(userId)) this.sessions.set(userId, new Map())
    this.sessions.get(userId)!.set(ptyId, session)
  }

  getForUser(userId: string, ptyId: string): PtySession | undefined {
    return this.sessions.get(userId)?.get(ptyId)
  }

  // Cleanup khi user disconnect: kill all user's PTY sessions
  cleanupUser(userId: string): void {
    const userSessions = this.sessions.get(userId)
    if (userSessions) {
      for (const session of userSessions.values()) session.kill()
      this.sessions.delete(userId)
    }
  }
}
```

**Isolation đảm bảo**:
- User A không thể access PTY của User B
- Khi user disconnect → tất cả PTY sessions của user bị cleanup
- PTY state persist trong SQLite (`pty-state-persistence.ts`) để resume sau restart

---

## 9. ProfileAwareAgentSpawner (v6.0)

Khi Orca Server gọi `pty.spawn` với agent binary, Dev Server Agent apply profile:

```typescript
// src/agent/agent-spawn/profile-aware-agent-spawner.ts
// (runs ON Dev Server, receives resolved profile via relay ctx)

async spawn(params: AgentSpawnParams): Promise<{ ptyId: string, sessionId: string }> {
  const { binary, args, cwd, env, userId, projectId, resolvedProfile } = params

  // Validate model against approvedModels whitelist
  if (resolvedProfile.agent?.approvedModels) {
    const model = resolvedProfile.agent.preferredModel
    if (!resolvedProfile.agent.approvedModels.includes(model)) {
      throw new Error(`Model ${model} not approved for this organization`)
    }
  }

  // Build full env
  const spawnEnv = {
    ...process.env,
    ...resolvedProfile.shell?.envVars,
    PATH: [
      ...(resolvedProfile.shell?.pathAdditions ?? []),
      process.env.PATH,
    ].join(':'),
    GH_CONFIG_DIR:   path.join(AGENT_DATA_PATH, 'users', userId, 'gh-config'),
    GLAB_CONFIG_DIR: path.join(AGENT_DATA_PATH, 'users', userId, 'glab-config'),
    ...env,  // explicit overrides (e.g., ANTHROPIC_API_KEY from ProviderResolver)
  }

  // Spawn PTY
  const ptyId = randomUUID()
  const pty = nodePty.spawn(binary, args, { cwd, env: spawnEnv, cols: 220, rows: 50 })

  // Store + track
  ptySessionStore.create(userId, ptyId, pty)
  agentStateDetector.watch(pty, ptyId)  // OSC: idle→running→waiting→completed

  return { ptyId, sessionId: generateSessionId() }
}
```

---

## Files Liên Quan

| File | Mô tả |
|------|-------|
| [`deploy/dev/scripts/start-agent-direct.sh`](../scripts/start-agent-direct.sh) | **Deploy daemon Mode 1** — systemd + auto-token mỗi restart |
| [`deploy/dev/scripts/connect-agent.sh`](../scripts/connect-agent.sh) | **CLI tool cả 2 mode** — deploy, start, status, stop, logs |
| [`deploy/dev/scripts/sync-to-server.sh`](../scripts/sync-to-server.sh) | Deploy Orca Server (rsync + Docker rebuild) |
| [`deploy/dev/agent/agent.js`](../agent/agent.js) | Agent runtime — Node.js, cả 2 mode, tool registry |
| [`deploy/dev/agent/package.json`](../agent/package.json) | Dependencies (`ws ^8.18`) |
| [`deploy/dev/.env.example`](../.env.example) | Biến môi trường: host, port, token, SSH key |
| [`src/server/agent-token-routes.ts`](../../src/server/agent-token-routes.ts) | HTTP API: `POST /api/agent-token` |
| [`src/main/dev-server/agent-ws-server.ts`](../../src/main/dev-server/agent-ws-server.ts) | WS server nhận agent connections (Mode 1) |
| [`src/main/dev-server/ws-handshake.ts`](../../src/main/dev-server/ws-handshake.ts) | Handshake protocol (initiator + receiver) |
| [`src/main/dev-server/ws-transport.ts`](../../src/main/dev-server/ws-transport.ts) | WebSocket → MultiplexerTransport adapter |
| [`src/main/dev-server/dev-server-relay-bridge.ts`](../../src/main/dev-server/dev-server-relay-bridge.ts) | Bridge: connect/session/relay logic cả 2 mode |
| [`src/main/dev-server/dev-server-manager.ts`](../../src/main/dev-server/dev-server-manager.ts) | Orchestrator: manage bridge lifecycle + daemon agent |
| [`src/main/dev-server/relay-connection-pool.ts`](../../src/main/dev-server/relay-connection-pool.ts) | **[v5.0]** Shared relay pool per devServerId |
| [`src/agent/rpc/context-verifier.ts`](../../src/agent/rpc/context-verifier.ts) | **[v6.0]** HMAC-SHA256 signed context verification |
| [`src/agent/pty/pty-session-store.ts`](../../src/agent/pty/pty-session-store.ts) | **[v6.0]** Per-userId PTY session registry |
| [`src/agent/agent-spawn/profile-aware-agent-spawner.ts`](../../src/agent/agent-spawn/profile-aware-agent-spawner.ts) | **[v6.0]** Profile-aware agent spawn |
| [`src/shared/agent-wire-protocol.ts`](../../src/shared/agent-wire-protocol.ts) | Constants: timeouts, paths, error codes |
| [`src/main/ssh/relay-protocol.ts`](../../src/main/ssh/relay-protocol.ts) | Frame encoder/decoder, FrameDecoder, MessageType |
