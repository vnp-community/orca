# ADR-004 — SSH Relay Binary for Remote Dev Server Execution

| Trường | Giá trị |
|--------|---------|
| **ID** | ADR-004 |
| **Trạng thái** | 🔄 Superseded by [ADR-013](./ADR-013-dev-server-agent-replaces-relay.md) |
| **Ngày** | 2026-07-28 |
| **Cập nhật** | 2026-07-30 (v6.0 superseded) |
| **HLD Ref** | C2 (Relay container), C3.3 |
| **Code Ref** | `src/relay/`, `src/main/ssh/`, `src/main/dev-server/dev-server-relay-bridge.ts`, `deploy/dev/agent/agent.js` |

---

> **⚠️ SUPERSEDED (v6.0 — 2026-07-30)**  
> ADR-004 bị thay thế bởi **[ADR-013 — Dev Server Agent Replaces Thin Relay](./ADR-013-dev-server-agent-replaces-relay.md)**.  
> Kiến trúc Thin Relay (auto-deploy binary via SFTP) không đáp ứng yêu cầu enterprise: không có business logic, không stateful, không autonomous, không scale với multi-user.  
> ADR-004 vẫn được giữ lại để lưu thông tin lịch sử thiết kế và lý do thay đổi.

---

## Bối cảnh

Orca cần thực thi các operations trên **remote dev servers** (PTY sessions, file operations, git commands, agent spawning) — mà không yêu cầu user install phần mềm phức tạp hoặc mở firewall inbound.

**Constraints:**
- Dev servers có thể ở behind NAT/firewall (chỉ SSH outbound)
- Không muốn yêu cầu user setup phức tạp
- Cần multiplex nhiều streams (PTY, file, git, AI hooks) qua một SSH connection
- Phải hoạt động trên Linux/macOS/Windows (WSL)

---

## Quyết định

### Kiến trúc 3-layer

```
Orca Server (Main Process)
    │
    │ SSH connection (ssh2 lib)
    │
Dev Server Host
    │
    └── Relay binary (Node.js) ← auto-deployed via SFTP + chmod
        ├── Dispatcher: routes JSON-RPC requests
        ├── fs-handler-*: file operations
        ├── agent-exec-handler: spawn PTY agents
        ├── agent-hook-server: loopback HTTP for agent hooks
        └── git handlers: git operations
```

### Wire protocol

**Relay Protocol** (từ `src/relay/protocol.ts` và `deploy/dev/agent/agent.js`):
```
[TYPE: u8][SEQ: u32 BE][ACK: u32 BE][LENGTH: u32 BE][PAYLOAD: JSON-RPC]
```

- `TYPE`: 0=DATA, 1=ACK, 2=KEEPALIVE, 3=CLOSE
- `SshChannelMultiplexer`: multiplexes nhiều virtual channels qua một SSH channel
- KEEPALIVE mỗi `KEEPALIVE_SEND_MS` ms để tránh TCP idle timeout

### Auto-deploy

```typescript
// src/main/ssh/ssh-relay-deploy.ts
async function deployAndLaunchRelay(sshConn, serverInfo): Promise<RelayProcess> {
  // 1. Detect platform (linux-arm64, darwin-x64, win32-x64...)
  // 2. SFTP upload relay binary if version mismatch
  // 3. chmod +x
  // 4. exec relay binary → get stdin/stdout as SSH channel
}
```

### 3 Connection modes

| Mode | Khi dùng | Code |
|------|---------|------|
| `relay-ssh` | Default — SSH tunnel, relay binary | `DevServerRelayBridge` (Phase 1) |
| `relay-websocket` | Relay có public WS endpoint | `DevServerRelayBridge` (Phase 2) |
| `direct-websocket` | Agent connects tới Orca WS Server | `AgentWebSocketServer`, `agent.js` |

### Relay dispatcher

```typescript
// src/relay/dispatcher.ts
// Routes JSON-RPC method to correct handler
const handlers: Record<string, MethodHandler> = {
  'fs.list':       fsHandlerListFiles,
  'fs.read':       fsHandlerFileRead,
  'fs.search':     fsHandlerGitSearch,
  'agent.exec':    agentExecHandler,
  'git.*':         gitHandlers,
  // ...
}
```

### deploy/dev/agent/agent.js

Agent chạy trên dev server, kết nối Orca qua 2 modes:
- **direct-websocket** (default): `ORCA_URL=wss://...`, `AGENT_TOKEN=agt-xxx`
- **relay-websocket**: `AGENT_PORT=6799`, `MODE=relay-websocket`

Expose tools: `claude`, `gh`, `gitnexus`, `codegraph`, `git`, `docker`...

---

## Lý do chọn

| Lựa chọn | Đánh giá |
|----------|---------|
| **SSH relay binary (auto-deploy)** ✅ | Không yêu cầu inbound firewall; auto-deploy; multiplexed; cross-platform |
| OpenSSH port forwarding | Cần cấu hình per-operation, không multiplex |
| gRPC direct | Yêu cầu TLS cert setup, inbound firewall |
| Agent pulls commands | Polling latency, không stream |
| Custom daemon (systemd) | User phải setup thủ công, không auto-deploy |

---

## Hậu quả

**Tích cực:**
- Một SSH connection → N operations (PTY, file, git, hooks)
- Auto-deploy: user chỉ cần SSH access
- Platform-aware binary (arm64/x64 Linux/Mac/Win)

**Tiêu cực:**
- SSH dependency: server phải có SSH daemon
- Binary version management: relay version phải match Orca version
- Keepalive overhead trên slow networks
- `deploy/dev/agent/agent.js` là separate codebase (Node.js CommonJS) — cần sync với relay protocol

---

## Relay RPC Methods (hiện tại)

```
fs.list, fs.read, fs.search, fs.readdir, fs.stream
agent.exec (PTY spawn)
git.* (future — hiện qua shell exec trong agent)
automations.run (external automation handler)
```

---

## Trạng thái Implementation

✅ relay-ssh mode (Phase 1) — DevServerRelayBridge  
✅ SshChannelMultiplexer — multi-channel over single SSH  
✅ Auto-deploy (SFTP + chmod)  
✅ fs-handler (list, read, search)  
✅ agent-exec-handler (PTY)  
✅ agent-hook-server (loopback HTTP)  
✅ direct-websocket mode (agent.js)  
🚧 relay-websocket mode (Phase 2)  
🚧 git.* dedicated RPC methods (hiện dùng shell exec)
