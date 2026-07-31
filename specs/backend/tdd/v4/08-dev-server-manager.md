# TDD-BE-08: Dev Server Manager & Agent WebSocket Server

**Version:** 4.0  
**Date:** 2026-07-28  
**Source:** `src/main/dev-server/`

---

## 1. Module Map

| File | Role |
|------|------|
| `dev-server-manager.ts` | CRUD + connection lifecycle cho DevServer |
| `dev-server-store.ts` | Persist DevServer records (via Store) |
| `dev-server-relay-bridge.ts` | Kết nối đến remote dev server (relay/ssh) |
| `agent-ws-server.ts` | Accept direct-WebSocket agent connections |
| `ws-handshake.ts` | Binary handshake protocol (agent ↔ server) |
| `ws-transport.ts` | WebSocket → duplex stream adapter |
| `gateway-proxy.ts` | HTTP proxy gateway cho dev server |

---

## 2. DevServer Types

```typescript
export type DevServer = {
  id:             string
  name:           string
  host:           string
  connectionType: 'relay-ssh' | 'relay-websocket' | 'direct-websocket'
  status:         DevServerStatus
  platform:       NodeJS.Platform | null
  arch:           string | null
  nodeVersion:    string | null
  lastConnectedAt: number | null
  lastError:      string | null
}

export type DevServerStatus = 'connecting' | 'connected' | 'disconnected' | 'error'
```

---

## 3. DevServerManager

```typescript
class DevServerManager extends EventEmitter {
  // CRUD
  add(input: DevServerInput): DevServer
  get(id: string): DevServer | undefined
  list(): DevServer[]
  remove(id: string): void

  // Connection
  async connect(id: string): Promise<void>
  async disconnect(id: string): Promise<void>
  async testConnection(id: string): Promise<ConnectionTestResult>

  // Direct-WS daemon agent registration
  async connectDaemonAgent(params: {
    devServerId: string
    name:        string
    token:       string
    ttlMs:       number
  }): Promise<{ created: boolean }>

  // Emit 'connecting' for all direct-ws servers (gọi sau HTTP server listen)
  restoreConnections(): void
}
```

**Events emitted:**
- `devServer:added` (id)
- `devServer:removed` (id)
- `devServer:statusChanged` (id, status, error?)

---

## 4. AgentWebSocketServer

```typescript
class AgentWebSocketServer {
  // Attach to httpServer — intercepts WS upgrades on '/agent' path
  attach(httpServer: HttpServer): void

  // Register a one-time slot for agent token
  // Slot auto-expires after AGENT_CONNECT_TIMEOUT_MS (60s)
  registerSlot(
    agentToken: string,
    onConnected: (mux: SshChannelMultiplexer, info: AgentConnectedInfo) => void,
    onExpired: (reason: string) => void
  ): () => void  // disposer

  // Stop server + cancel all pending slots
  stop(): void
}
```

**Connection flow:**
```
Agent (remote)
  → ws://orca-server:6769/agent?token=<agentToken>
  → AgentWebSocketServer.handleConnection()
  → runOrcaReceiverHandshake() — validate token + version
  → slot.callback(SshChannelMultiplexer, info)
  → DevServerRelayBridge wires mux into channel map
```

---

## 5. Connection Types

### direct-websocket
```
Agent → ws://<orca-host>:6769/agent (POST /api/agent-token trước)
Server → AgentWebSocketServer
```

### relay-ssh
```
Server → SSH tunnel → remote agent
SSH Channel Multiplexer (binary frames)
```

### relay-websocket
```
Server → ws://<relay-server>/ws
Relay proxies to local agent
```

---

## 6. Runtime State (In-Memory Only)

Runtime state KHÔNG được persist (mất khi restart):
```typescript
type RuntimeDevServerState = {
  status:          DevServerStatus
  platform:        NodeJS.Platform | null
  arch:            string | null
  nodeVersion:     string | null
  lastConnectedAt: number | null
  lastError:       string | null
}
```

**direct-websocket behavior on restart:**
- Khởi động với status='connecting' (agent auto-reconnect via systemd)
- `restoreConnections()` emit 'devServer:statusChanged' → UI hiển thị "Connecting..."

---

## 7. Handshake Protocol

```
Client → Server: {
  type:         'handshake',
  agentToken:   string,     // từ POST /api/agent-token
  agentVersion: string,
  platform:     string,     // os.platform()
  arch:         string,     // os.arch()
  nodeVersion:  string      // process.version
}

Server → Client: {
  type:           'handshake-ack',
  orcaVersion:    string,
  sessionId:      string    // UUID per connection
}
```

Timeout: 10s (nếu không nhận handshake → close với 1008)
