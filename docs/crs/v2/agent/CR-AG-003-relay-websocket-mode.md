# CR-AG-003 — relay-websocket: Orca kết nối vào Agent WS Server

**CR-ID:** CR-AG-003  
**Ngày:** 2026-07-26  
**Priority:** 🟠 High  
**Effort:** Large (3–5 ngày)  
**Status:** ✅ Implemented (2026-07-26)  

**Implementation Note:** relay-websocket bridge + UI đã implemented. AddDevServerDialog hiển thị URL input và setup guide.  
**Backend:** SOL-AG-003 IMPLEMENTED (TASK-006, TASK-007, TASK-008, TASK-011, TASK-012)  
**Frontend:** SOL-FE-AG-001, SOL-FE-AG-003 IMPLEMENTED  
**Depends on:** CR-AG-001, CR-AG-002  
**Parallel with:** CR-AG-004  

---

## 1. Mô tả mode

```
relay-websocket mode:

Dev Server / Agent                          Orca App (or Orca Server)
       │                                             │
       │  Agent khởi động                           │
       │  WS Server lắng nghe                       │
       │  ws://0.0.0.0:6799/orca-relay              │
       │                                             │
       │ ◄──── Orca WebSocket connect ──────────────│
       │ ◄──── Bearer token (HTTP header) ──────────│
       │                                             │
       │──── agent.handshake { platform, arch } ───►│
       │◄─── handshake-ok { orcaVersion, sessionId }│
       │                                             │
       │◄═══ JSON-RPC 2.0 (framed binary) ═════════►│
       │   (Orca gọi fs.readDir, pty.spawn, ...)     │
```

**Tính chất quan trọng:**
- **Agent tự khởi WS server**, lắng nghe ở một port
- **Orca là WebSocket client**, chủ động connect vào agent
- Agent **không cần biết địa chỉ của Orca**
- Phù hợp khi: agent chạy trong LAN, cloud VM, container có port public

---

## 2. Phân tích codebase cần thay đổi

### 2.1 `DevServerRelayBridge.connect()` — nơi cần fill Phase 2

```typescript
// Hiện tại — src/main/dev-server/dev-server-relay-bridge.ts:85-89
throw new Error(`Only 'relay-ssh' is supported in Phase 1.`)
```

Cần thêm branch `relay-websocket`:

```typescript
if (this.config.connectionType === 'relay-websocket') {
  const wsUrl = this.config.wsUrl
  if (!wsUrl) throw new Error(`DevServer '${this.config.name}' has no wsUrl`)
  return this.connectRelayWebSocket(wsUrl, opts)
}
```

### 2.2 `PersistedDevServer.wsUrl` — field đã có sẵn

```typescript
// src/shared/dev-server-types.ts:52
wsUrl?: string   // ws://devserver.local:6799?token=abc
```

Token được encode vào URL query string để Orca biết.

### 2.3 `DevServerManager.testConnection()` — cần hỗ trợ relay-websocket

```typescript
// src/main/dev-server/dev-server-manager.ts:88
async testConnection(input: DevServerInput): Promise<ConnectionTestResult> {
  // Hiện tại chỉ handle relay-ssh, cần thêm relay-websocket path
}
```

---

## 3. Giải pháp — Orca side

### 3.1 Thêm `connectRelayWebSocket()` vào `DevServerRelayBridge`

```typescript
// src/main/dev-server/dev-server-relay-bridge.ts (thêm method)

import WebSocket from 'ws'
import { SshChannelMultiplexer } from '../ssh/ssh-channel-multiplexer'
import { createWebSocketTransport } from './ws-transport'
import { runOrcaInitiatorHandshake } from './ws-handshake'
import { app } from 'electron'

private async connectRelayWebSocket(
  rawUrl: string,
  opts: { testOnly?: boolean }
): Promise<RelayHandshakeInfo> {
  // Parse token from URL: ws://host:port?token=<secret>
  const url = new URL(rawUrl)
  const token = url.searchParams.get('token') ?? ''
  url.searchParams.delete('token')
  const cleanUrl = url.toString()

  return new Promise<RelayHandshakeInfo>((resolve, reject) => {
    const ws = new WebSocket(cleanUrl, {
      headers: token ? { 'Authorization': `Bearer ${token}` } : {},
    })

    ws.binaryType = 'nodebuffer'

    const connectionTimeout = setTimeout(() => {
      ws.close()
      reject(new Error(`relay-websocket: connection timed out after 10s: ${cleanUrl}`))
    }, 10_000)

    ws.on('error', (err) => {
      clearTimeout(connectionTimeout)
      reject(new Error(`relay-websocket: WebSocket error: ${err.message}`))
    })

    ws.on('open', async () => {
      clearTimeout(connectionTimeout)
      try {
        const orcaVersion = app.getVersion()
        const info = await runOrcaInitiatorHandshake(ws, orcaVersion)

        const transport = createWebSocketTransport(ws)
        this.session = new SshChannelMultiplexer(transport)

        if (opts.testOnly) {
          await this.disconnect()
        }

        resolve({
          platform: info.platform as NodeJS.Platform,
          arch: info.arch,
          nodeVersion: info.nodeVersion,
          relayVersion: info.agentVersion,
        })
      } catch (err) {
        ws.close()
        reject(err)
      }
    })
  })
}
```

### 3.2 Agent Side — WS Server

Agent tự khởi động một HTTP server với WebSocket upgrade. Spec tối thiểu:

#### Endpoint
```
GET ws://<host>:<port>/orca-relay
Upgrade: websocket
Authorization: Bearer <secret-token>
```

#### Agent startup sequence
1. Đọc config (port, secret token)
2. Bind HTTP server, lắng nghe port (default: `6799`)
3. Accept WebSocket upgrade chỉ trên path `/orca-relay`
4. Validate `Authorization: Bearer <token>` header
5. Sau khi WS connected:
   a. **Nhận** `agent.handshake` request từ Orca
   b. **Reply** handshake-ok với `{ platform, arch, nodeVersion, agentVersion, sessionId }`
6. Agent bắt đầu handle JSON-RPC calls từ Orca

#### Agent handshake response
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "ok": true,
    "platform": "linux",
    "arch": "arm64",
    "nodeVersion": "v20.11.0",
    "agentVersion": "1.0.0",
    "sessionId": "sess-1722033600-abc123"
  }
}
```

#### Agent configuration (YAML / env)
```yaml
# agent.yaml
listen_port: 6799
secret_token: "my-super-secret-token-here"
workspace_dir: "/home/ubuntu/code"
capabilities:
  - pty
  - fs
  - git
  - preflight
```

### 3.3 URL format cho wsUrl

```
ws://host:port/orca-relay?token=<secret>
wss://host:port/orca-relay?token=<secret>   (TLS variant)
```

> **Security**: Token không nên đặt trong URL production (log exposure). Sau này có thể dùng `Authorization` header thông qua custom header extension trong UI. Phase 2 dùng URL query string cho đơn giản.

---

## 4. Agent Implementation Guide (multi-language)

### TypeScript (Node.js) — Minimal Agent

```typescript
// agent-ws-server.ts
import * as http from 'http'
import * as WebSocket from 'ws'
import { readFileSync } from 'fs'

const PORT = parseInt(process.env.AGENT_PORT ?? '6799')
const TOKEN = process.env.AGENT_TOKEN ?? ''

const httpServer = http.createServer()
const wss = new WebSocket.Server({ noServer: true })

httpServer.on('upgrade', (req, socket, head) => {
  if (req.url !== '/orca-relay') {
    socket.destroy()
    return
  }
  const auth = req.headers['authorization'] ?? ''
  if (!auth.startsWith('Bearer ') || auth.slice(7) !== TOKEN) {
    socket.write('HTTP/1.1 401 Unauthorized\r\n\r\n')
    socket.destroy()
    return
  }
  wss.handleUpgrade(req, socket, head, (ws) => {
    wss.emit('connection', ws, req)
  })
})

wss.on('connection', (ws) => {
  const agent = new OrcaAgent(ws)
  agent.start()
})

httpServer.listen(PORT, () => {
  console.log(`[agent] Listening on ws://0.0.0.0:${PORT}/orca-relay`)
})
```

### Python (asyncio + websockets)

```python
# agent_server.py
import asyncio, json, os, struct, platform
import websockets
from websockets.server import WebSocketServerProtocol

PORT = int(os.environ.get('AGENT_PORT', '6799'))
TOKEN = os.environ.get('AGENT_TOKEN', '')

async def handle_connection(websocket: WebSocketServerProtocol, path: str):
    if path != '/orca-relay':
        await websocket.close(1008, 'Invalid path')
        return

    # Auth check
    auth = websocket.request_headers.get('Authorization', '')
    if not auth.startswith('Bearer ') or auth[7:] != TOKEN:
        await websocket.close(1008, 'Unauthorized')
        return

    # Handshake
    await handle_handshake(websocket)

    # Main loop — handle Orca RPC calls
    async for message in websocket:
        frame_type, seq, ack, payload = decode_frame(message)
        if frame_type == 0x01:  # Regular
            msg = json.loads(payload)
            response = await dispatch(msg)
            if response:
                await websocket.send(encode_frame(0x01, seq+1, seq, response))

async def handle_handshake(websocket):
    # Wait for agent.handshake from Orca
    data = await websocket.recv()
    frame_type, seq, ack, payload = decode_frame(data)
    msg = json.loads(payload)
    assert msg['method'] == 'agent.handshake'

    response = {
        'jsonrpc': '2.0',
        'id': msg['id'],
        'result': {
            'ok': True,
            'platform': platform.system().lower(),
            'arch': platform.machine(),
            'nodeVersion': 'python-agent',
            'agentVersion': '1.0.0',
            'sessionId': f'sess-python-{id(websocket)}'
        }
    }
    await websocket.send(encode_frame(0x01, 1, seq, response))

def encode_frame(msg_type: int, seq: int, ack: int, payload: dict) -> bytes:
    body = json.dumps(payload).encode('utf-8')
    return struct.pack('>BIII', msg_type, seq, ack, len(body)) + body

def decode_frame(data: bytes) -> tuple:
    msg_type, seq, ack, length = struct.unpack('>BIII', data[:13])
    return msg_type, seq, ack, data[13:13+length]

async def main():
    print(f'[agent] Listening on ws://0.0.0.0:{PORT}/orca-relay')
    async with websockets.serve(handle_connection, '0.0.0.0', PORT):
        await asyncio.Future()

asyncio.run(main())
```

### Go

```go
// main.go
package main

import (
    "encoding/binary"
    "encoding/json"
    "log"
    "net/http"
    "os"
    "runtime"
    "strings"
    "github.com/gorilla/websocket"
)

var (
    port    = getenv("AGENT_PORT", "6799")
    token   = getenv("AGENT_TOKEN", "")
    upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
)

func main() {
    http.HandleFunc("/orca-relay", handleRelay)
    log.Printf("[agent] Listening on ws://0.0.0.0:%s/orca-relay", port)
    log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleRelay(w http.ResponseWriter, r *http.Request) {
    auth := r.Header.Get("Authorization")
    if !strings.HasPrefix(auth, "Bearer ") || strings.TrimPrefix(auth, "Bearer ") != token {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil { return }
    defer conn.Close()

    // Wait for agent.handshake
    _, data, err := conn.ReadMessage()
    if err != nil { return }
    frameType, seq, _, payload := decodeFrame(data)
    if frameType != 0x01 { return }

    var req map[string]interface{}
    json.Unmarshal(payload, &req)

    resp := map[string]interface{}{
        "jsonrpc": "2.0", "id": req["id"],
        "result": map[string]interface{}{
            "ok": true, "platform": runtime.GOOS, "arch": runtime.GOARCH,
            "agentVersion": "1.0.0", "sessionId": "sess-go",
        },
    }
    conn.WriteMessage(websocket.BinaryMessage, encodeFrame(0x01, seq+1, seq, resp))

    // Main dispatch loop
    for {
        _, data, err := conn.ReadMessage()
        if err != nil { break }
        handleMessage(conn, data)
    }
}
```

---

## 5. Files cần thay đổi

### [MODIFY] `src/main/dev-server/dev-server-relay-bridge.ts`
- Thêm `connectRelayWebSocket()` private method
- Fill `relay-websocket` branch trong `connect()`
- Import `WebSocket` from `ws` (check package.json đã có chưa)

### [MODIFY] `src/main/dev-server/dev-server-manager.ts`
- `testConnection()`: handle `relay-websocket` path (không cần SSH setup)

### [NEW] `src/main/dev-server/ws-transport.ts`
(từ CR-AG-002)

### [NEW] `src/main/dev-server/ws-handshake.ts`
(từ CR-AG-002)

### [NEW] `src/shared/agent-wire-protocol.ts`
(từ CR-AG-001)

### [NEW] `docs/specs/agent-relay-websocket-server.md`
Agent implementation guide — reference spec cho external agent developers.

### [MODIFY] `src/shared/dev-server-types.ts`
Không cần thay đổi — `wsUrl` đã có sẵn.

---

## 6. Tiêu chí hoàn thành

- [ ] `DevServerRelayBridge.connect()` không còn throw error cho `relay-websocket`
- [ ] Orca kết nối thành công tới TypeScript agent WS server (integration test)
- [ ] Orca kết nối thành công tới Python agent WS server (manual test)
- [ ] Token auth: invalid token → WS close + error message rõ ràng
- [ ] `DevServerManager.testConnection()` trả về `ok: true` cho relay-websocket config hợp lệ
- [ ] Settings UI (`DevServerPane`) hiển thị WebSocket URL input khi chọn `relay-websocket`
- [ ] Disconnect: `DevServerRelayBridge.disconnect()` close WS gracefully
