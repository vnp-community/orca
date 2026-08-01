# CR-AG-004 — direct-websocket: Agent kết nối vào Orca WS Server

**CR-ID:** CR-AG-004  
**Ngày:** 2026-07-26  
**Priority:** 🟠 High  
**Effort:** Large (3–5 ngày)  
**Status:** ✅ Implemented (2026-07-26)  

**Implementation Note:** direct-websocket AgentWebSocketServer + bridge + UI token panel đã implemented.  
**Backend:** SOL-AG-004 IMPLEMENTED (TASK-009, TASK-010, TASK-011, TASK-012)  
**Frontend:** SOL-FE-AG-002, SOL-FE-AG-003, SOL-FE-AG-004 IMPLEMENTED  
**Depends on:** CR-AG-001, CR-AG-002  
**Parallel with:** CR-AG-003  

---

## 1. Mô tả mode

```
direct-websocket mode:

Dev Server / Agent                          Orca App (or Orca Server)
       │                                             │
       │                                    WS server:ws://orca:6768/agent
       │                                    chờ incoming agent connections
       │                                             │
       │──── WebSocket connect ──────────────────────►│
       │──── Authorization: Bearer <agent-token> ────►│
       │                                             │
       │──── agent.handshake { agentToken, ... } ───►│
       │◄─── handshake-ok { orcaVersion, sessionId } │
       │                                             │
       │◄══ JSON-RPC 2.0 (framed binary) ═══════════►│
       │   (Orca gọi fs.readDir, pty.spawn, ...)     │
```

**Tính chất quan trọng:**
- **Agent là WebSocket client**, chủ động connect vào Orca
- **Orca là WebSocket server**, chờ agent kết nối
- Agent **cần biết địa chỉ Orca** (`ws://orca-server:6768/agent`)
- Phù hợp khi: agent ở trong container/VM không có port public, Orca ở trên internet

**Sự khác biệt vs relay-websocket:**

| | `relay-websocket` | `direct-websocket` |
|---|---|---|
| Ai là WS server | Agent | Orca |
| Ai connect trước | Orca | Agent |
| Agent cần port public | Có | Không |
| Orca cần port public | Không | Có |
| Config trên Orca | `wsUrl` = URL của agent | `wsUrl` = Orca URL (để agent biết đâu mà connect) |

---

## 2. Phân tích codebase cần thay đổi

### 2.1 Orca Server — cần thêm `/agent` WebSocket endpoint

`src/server/index.ts` khởi động HTTP server và WS RPC server, nhưng **chỉ phục vụ browser clients**. Cần thêm một WS endpoint `/agent` riêng cho agent connections.

```typescript
// src/server/index.ts (hiện tại)
console.log(`[Orca Server] RPC:     ws://0.0.0.0:${rpcPort}`)  // chỉ cho browser
```

Cần thêm:
```
ws://0.0.0.0:6768/agent   ← agent connections endpoint (NEW)
ws://0.0.0.0:6768/        ← browser/web client (existing)
```

### 2.2 Electron main process — cần thêm `/agent` WebSocket endpoint

`src/main/index.ts` (`initializeOrcaServices`) thiết lập WS server cho renderer. Cần accept agent connections từ một path riêng.

### 2.3 `DevServerRelayBridge` — cần thêm `direct-websocket` branch

```typescript
// Hiện tại — cần fill
if (this.config.connectionType === 'direct-websocket') {
  return this.registerDirectWebSocket(opts)
}
```

Khác với relay-websocket: ở mode này Orca **không connect out** — thay vào đó Orca **đăng ký một "slot"** để khi agent kết nối vào, bridge này được wired up.

---

## 3. Giải pháp — Orca side

### 3.1 Tạo `AgentWebSocketServer`

**File mới:** `src/main/dev-server/agent-ws-server.ts`

```typescript
// src/main/dev-server/agent-ws-server.ts
//
// WebSocket server that accepts incoming connections from external agents.
// Agents connect to ws://<orca-host>:<port>/agent and authenticate
// via the agent.handshake JSON-RPC method.
//
// Each accepted agent connection is wrapped in an SshChannelMultiplexer
// and handed off to a registered DevServerRelayBridge via a callback.

import { WebSocketServer, WebSocket } from 'ws'
import type { IncomingMessage } from 'node:http'
import type { Server as HttpServer } from 'node:http'
import { SshChannelMultiplexer } from '../ssh/ssh-channel-multiplexer'
import { createWebSocketTransport } from './ws-transport'
import { runOrcaReceiverHandshake } from './ws-handshake'

export type AgentConnectionCallback = (
  mux: SshChannelMultiplexer,
  info: {
    platform: string
    arch: string
    nodeVersion: string
    agentVersion: string
    sessionId: string
  }
) => void

export class AgentWebSocketServer {
  private wss: WebSocketServer | null = null
  // Map<agentToken, callback>
  private pendingSlots = new Map<string, AgentConnectionCallback>()
  private orcaVersion: string

  constructor(orcaVersion: string) {
    this.orcaVersion = orcaVersion
  }

  /**
   * Attach this server to an existing HTTP server on path /agent.
   * Call once during Orca startup.
   */
  attach(httpServer: HttpServer): void {
    this.wss = new WebSocketServer({ noServer: true })

    httpServer.on('upgrade', (req: IncomingMessage, socket, head) => {
      const url = new URL(req.url ?? '/', `http://${req.headers.host}`)
      if (url.pathname !== '/agent') return  // not for us

      this.wss!.handleUpgrade(req, socket, head, (ws) => {
        this.handleConnection(ws)
      })
    })
  }

  /**
   * Register a "slot" for a specific agent token.
   * When an agent with this token connects, the callback is called
   * with the established multiplexer.
   *
   * Returns a disposer that removes the slot (call on DevServer removal).
   */
  registerSlot(agentToken: string, onConnected: AgentConnectionCallback): () => void {
    this.pendingSlots.set(agentToken, onConnected)
    return () => {
      this.pendingSlots.delete(agentToken)
    }
  }

  private handleConnection(ws: WebSocket): void {
    // Why: runOrcaReceiverHandshake validates the token embedded in
    // agent.handshake params. We pass a validator that checks our slot map.
    runOrcaReceiverHandshake(
      ws,
      (token) => this.pendingSlots.has(token),
      this.orcaVersion
    )
      .then((info) => {
        const slot = this.pendingSlots.get(info.sessionId)
        // Find slot by matching the token from handshake params
        // (info.sessionId carries the token after our custom receiver impl)
        const callback = this.findAndConsumeSlot(ws, info)
        if (!callback) {
          // No registered slot — close
          ws.close(1008, 'No registered slot for this agent token')
          return
        }
        const transport = createWebSocketTransport(ws)
        const mux = new SshChannelMultiplexer(transport)
        callback(mux, info)
      })
      .catch((err) => {
        console.warn('[AgentWsServer] Handshake failed:', err.message)
        ws.close(1008, err.message)
      })
  }

  private findAndConsumeSlot(
    ws: WebSocket,
    info: { agentToken?: string }
  ): AgentConnectionCallback | null {
    // Token is carried in handshake params — ws-handshake sets it
    const token = (info as { agentToken?: string }).agentToken ?? ''
    const cb = this.pendingSlots.get(token)
    if (!cb) return null
    // Why: consume slot on first connect. If agent reconnects,
    // it must register a new slot (or we keep it — TBD by reconnect policy).
    this.pendingSlots.delete(token)
    return cb
  }

  stop(): void {
    this.wss?.close()
    this.wss = null
    this.pendingSlots.clear()
  }
}
```

### 3.2 `DevServerRelayBridge` — `direct-websocket` mode

```typescript
// src/main/dev-server/dev-server-relay-bridge.ts (thêm)

// Nhận từ server-bootstrap qua constructor injection
private agentWsServer: AgentWebSocketServer | null

private async connectDirectWebSocket(
  opts: { testOnly?: boolean }
): Promise<RelayHandshakeInfo> {
  if (!this.agentWsServer) {
    throw new Error('AgentWebSocketServer not initialized — cannot use direct-websocket mode')
  }

  // Generate a unique token for this dev server instance
  const agentToken = `agt-${this.config.id}-${Date.now()}`

  return new Promise<RelayHandshakeInfo>((resolve, reject) => {
    const timeout = setTimeout(() => {
      disposer()
      reject(new Error(`direct-websocket: agent did not connect within 60s. ` +
        `Ensure the agent is configured with: ORCA_URL=ws://<orca-host>:6768/agent ` +
        `AGENT_TOKEN=${agentToken}`))
    }, 60_000)

    const disposer = this.agentWsServer!.registerSlot(agentToken, (mux, info) => {
      clearTimeout(timeout)

      this.session = mux

      if (opts.testOnly) {
        this.disconnect()
      }

      resolve({
        platform: info.platform as NodeJS.Platform,
        arch: info.arch,
        nodeVersion: info.nodeVersion,
        relayVersion: info.agentVersion,
      })
    })

    // Emit the token to the UI so the user/agent can configure it
    this.emit('agentTokenGenerated', {
      devServerId: this.config.id,
      agentToken,
      orcaAgentUrl: `ws://<orca-host>:6768/agent`,
    })
  })
}
```

### 3.3 Server bootstrap — khởi tạo `AgentWebSocketServer`

```typescript
// src/main/server-bootstrap.ts (thêm vào initializeOrcaServices)

import { AgentWebSocketServer } from './dev-server/agent-ws-server'

// Trong initializeOrcaServices():
const agentWsServer = new AgentWebSocketServer(platform.app.getVersion())

// Attach to HTTP server sau khi khởi động
if (httpServer) {
  agentWsServer.attach(httpServer)
  console.log('[Orca Server] Agent WS endpoint: ws://0.0.0.0:<port>/agent')
}
```

### 3.4 Token display trong UI

Khi user chọn `direct-websocket` mode và click Connect:
1. Orca generate `agentToken`
2. UI hiển thị token + command cho user copy-paste vào agent machine:

```
Connect your agent:
  ORCA_URL=ws://b15.openledger.vn:6768/agent \
  AGENT_TOKEN=agt-ds-123-1722033600 \
  node agent.js
```

---

## 4. Agent Side — Client Implementation

### 4.1 Agent startup sequence

1. Đọc `ORCA_URL` và `AGENT_TOKEN` từ env/config
2. Connect WebSocket đến Orca: `ws://orca-host:6768/agent`
3. Gửi `agent.handshake` với `agentToken` trong params
4. Nhận `handshake-ok` từ Orca
5. Bắt đầu handle JSON-RPC calls từ Orca

### 4.2 Reconnect logic

Agent nên implement exponential backoff reconnect:

```
Initial delay: 1s
Max delay: 30s
Max attempts: unlimited (hoặc configurable)
```

Khi reconnect, agent **gửi lại cùng `AGENT_TOKEN`** — Orca phải accept (slot được recreate sau mỗi disconnect).

### 4.3 TypeScript Agent Client

```typescript
// agent-client.ts
import WebSocket from 'ws'
import os from 'os'

const ORCA_URL = process.env.ORCA_URL ?? 'ws://localhost:6768/agent'
const AGENT_TOKEN = process.env.AGENT_TOKEN ?? ''
const AGENT_VERSION = '1.0.0'

async function connectToOrca(): Promise<void> {
  let retryDelay = 1000

  while (true) {
    try {
      await connectOnce()
      retryDelay = 1000  // reset on success
    } catch (err) {
      console.error(`[agent] Connection failed: ${(err as Error).message}`)
      console.log(`[agent] Retrying in ${retryDelay}ms...`)
      await sleep(retryDelay)
      retryDelay = Math.min(retryDelay * 2, 30_000)
    }
  }
}

async function connectOnce(): Promise<void> {
  return new Promise((resolve, reject) => {
    const ws = new WebSocket(ORCA_URL)
    ws.binaryType = 'nodebuffer'
    let seq = 0, ack = 0

    ws.on('open', () => {
      seq++
      // Send handshake
      ws.send(encodeFrame(0x01, seq, ack, {
        jsonrpc: '2.0', id: 1,
        method: 'agent.handshake',
        params: {
          agentVersion: AGENT_VERSION,
          agentToken: AGENT_TOKEN,
          platform: os.platform(),
          arch: os.arch(),
          nodeVersion: process.version,
          capabilities: ['pty', 'fs', 'git', 'preflight'],
        }
      }))
    })

    ws.on('message', (data: Buffer) => {
      const [type, frameSeq, frameAck, payload] = decodeFrame(data)
      ack = Math.max(ack, frameSeq)
      if (type === 0x01) {
        const msg = JSON.parse(payload.toString('utf-8'))
        handleMessage(ws, msg, () => { seq++ })
      }
    })

    ws.on('close', () => resolve())
    ws.on('error', reject)
  })
}
```

### 4.4 Python Agent Client

```python
# agent_client.py
import asyncio, json, os, platform, struct
import websockets

ORCA_URL = os.environ.get('ORCA_URL', 'ws://localhost:6768/agent')
AGENT_TOKEN = os.environ.get('AGENT_TOKEN', '')

async def connect_to_orca():
    retry_delay = 1.0
    while True:
        try:
            await connect_once()
            retry_delay = 1.0
        except Exception as e:
            print(f'[agent] Connection failed: {e}. Retrying in {retry_delay}s...')
            await asyncio.sleep(retry_delay)
            retry_delay = min(retry_delay * 2, 30.0)

async def connect_once():
    async with websockets.connect(ORCA_URL) as ws:
        seq, ack = 0, 0
        seq += 1
        await ws.send(encode_frame(0x01, seq, ack, {
            'jsonrpc': '2.0', 'id': 1,
            'method': 'agent.handshake',
            'params': {
                'agentVersion': '1.0.0',
                'agentToken': AGENT_TOKEN,
                'platform': platform.system().lower(),
                'arch': platform.machine(),
                'capabilities': ['pty', 'fs', 'git', 'preflight'],
            }
        }))

        async for message in ws:
            frame_type, frame_seq, frame_ack, payload = decode_frame(message)
            ack = max(ack, frame_seq)
            if frame_type == 0x01:
                msg = json.loads(payload)
                response = await dispatch(msg)
                if response:
                    seq += 1
                    await ws.send(encode_frame(0x01, seq, frame_seq, response))
```

### 4.5 Go Agent Client

```go
package main

import (
    "encoding/binary"
    "encoding/json"
    "log"
    "os"
    "runtime"
    "time"
    "github.com/gorilla/websocket"
)

func connectToOrca() {
    orcaURL := getenv("ORCA_URL", "ws://localhost:6768/agent")
    retryDelay := time.Second

    for {
        err := connectOnce(orcaURL)
        if err != nil {
            log.Printf("[agent] Connection failed: %v. Retrying in %s", err, retryDelay)
            time.Sleep(retryDelay)
            if retryDelay < 30*time.Second {
                retryDelay *= 2
            }
        } else {
            retryDelay = time.Second
        }
    }
}

func connectOnce(url string) error {
    conn, _, err := websocket.DefaultDialer.Dial(url, nil)
    if err != nil { return err }
    defer conn.Close()

    var seq, ack uint32

    // Handshake
    seq++
    handshake := map[string]interface{}{
        "jsonrpc": "2.0", "id": 1,
        "method": "agent.handshake",
        "params": map[string]interface{}{
            "agentVersion": "1.0.0",
            "agentToken": os.Getenv("AGENT_TOKEN"),
            "platform": runtime.GOOS,
            "arch": runtime.GOARCH,
            "capabilities": []string{"pty", "fs", "git", "preflight"},
        },
    }
    conn.WriteMessage(websocket.BinaryMessage, encodeFrame(0x01, seq, ack, handshake))

    // Message loop
    for {
        _, data, err := conn.ReadMessage()
        if err != nil { return err }
        msgType, frameSeq, _, payload := decodeFrame(data)
        if frameSeq > ack { ack = frameSeq }
        if msgType == 0x01 {
            handleMessage(conn, payload, &seq)
        }
    }
}
```

---

## 5. Files cần thay đổi

### [NEW] `src/main/dev-server/agent-ws-server.ts`
WebSocket server nhận incoming agent connections.

### [MODIFY] `src/main/dev-server/dev-server-relay-bridge.ts`
- Thêm `AgentWebSocketServer` dependency qua constructor
- Fill `direct-websocket` branch trong `connect()`
- Emit `agentTokenGenerated` event

### [MODIFY] `src/main/server-bootstrap.ts`
- Khởi tạo `AgentWebSocketServer`
- Attach vào HTTP server
- Pass vào `DevServerManager` / `DevServerRelayBridge`

### [MODIFY] `src/main/dev-server/dev-server-manager.ts`
- Inject `AgentWebSocketServer` qua constructor

### [MODIFY] `src/renderer/src/components/settings/DevServerPane.tsx`
- Khi mode = `direct-websocket`: hiển thị generated token + command để copy
- Thêm state `agentToken: string | null`

### [NEW] `src/shared/agent-wire-protocol.ts`
(từ CR-AG-001)

### [NEW] `src/main/dev-server/ws-transport.ts`
(từ CR-AG-002)

### [NEW] `src/main/dev-server/ws-handshake.ts`
(từ CR-AG-002)

### [NEW] `docs/specs/agent-direct-websocket-client.md`
Agent client implementation guide — reference spec cho external agent developers.

---

## 6. Security considerations

| Concern | Mitigation |
|---------|-----------|
| Token exposure in logs | Token không log ở Orca side. Agent phải handle env var, không hardcode |
| Token reuse after disconnect | Orca generate new token per-session. Old token bị revoke |
| Man-in-the-middle | Dùng `wss://` (TLS) cho production |
| Multiple agents cùng token | Slot bị consume ngay lần đầu → second agent bị reject |
| Unauthenticated `/agent` requests | `AgentWebSocketServer` reject ngay nếu không có slot match |

---

## 7. Tiêu chí hoàn thành

- [ ] `AgentWebSocketServer` attach thành công vào HTTP server port 6768 path `/agent`
- [ ] `DevServerRelayBridge.connect()` không throw error cho `direct-websocket`
- [ ] Agent (TypeScript) connect thành công vào Orca, handshake pass
- [ ] Agent (Python) connect thành công (manual test)
- [ ] Token timeout 60s: nếu agent không connect → error rõ ràng trong UI
- [ ] Token display trong UI: user có thể copy lệnh khởi động agent
- [ ] Disconnect: Orca đóng multiplexer → agent detect close → reconnect loop
- [ ] Reconnect: agent reconnect → Orca re-register slot → session restored
