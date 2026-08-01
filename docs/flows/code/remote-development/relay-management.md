# Relay Management — Orca Server

> **Scope**: SSH Relay — kết nối Orca Server với Dev Machines để cung cấp remote PTY và Git
> **Key files**:
> - [`src/main/ssh/ssh-relay-deploy.ts`](../../src/main/ssh/ssh-relay-deploy.ts) — Deploy & launch relay binary
> - [`src/main/ssh/ssh-relay-session.ts`](../../src/main/ssh/ssh-relay-session.ts) — `SshRelaySession` state machine
> - [`src/main/ssh/ssh-connection-store.ts`](../../src/main/ssh/ssh-connection-store.ts) — `SshConnectionStore`
> - [`src/main/providers/ssh-pty-provider.ts`](../../src/main/providers/ssh-pty-provider.ts) — PTY qua relay
> - [`src/main/providers/ssh-git-provider.ts`](../../src/main/providers/ssh-git-provider.ts) — Git qua relay

---

## 1. Tổng quan Relay Architecture

Relay là cơ chế cho phép Orca Server điều khiển terminal và git operations trên **remote Dev Machine** qua SSH. Thay vì chạy PTY trực tiếp, Orca deploy một Node.js binary nhỏ (`orca-relay`) lên remote machine, binary này lắng nghe lệnh từ Orca Server và thực thi local.

```
Orca Server (172.20.2.39)              Dev Machine (172.20.2.31)
     │                                        │
     │  SSH connection (ssh2 npm)             │
     │──────────────────────────────────────►│
     │                                        │
     │  deployAndLaunchRelay():               │
     │  1. detect OS/arch                     │
     │  2. upload relay binary (SFTP)         │ ~/.orca-relay/{version}/
     │  3. launch: node daemon-entry.js       │ (process đang chạy)
     │  4. wait for ORCA-RELAY sentinel       │ → print "ORCA-RELAY\n"
     │                                        │
     │  JSON-RPC over SSH channel             │
     │◄────── MultiplexerTransport ──────────►│
     │                                        │
     │  PTY commands → remote PTY             │
     │  Git commands → remote git             │
```

---

## 2. SshTarget — Cấu hình Remote Server

```typescript
// src/shared/ssh-types.ts
type SshTarget = {
  id:       string         // UUID hoặc "runtime:{runtimeId}"
  label?:   string         // display name: "dev-local"
  host:     string         // "172.20.2.31"
  port:     number         // 22
  username: string         // "ubuntu"
  // Auth:
  identityFile?:    string // path to private key
  password?:        string // (ít dùng)
  // Config:
  graceTimeSeconds?: number  // relay idle keepalive (default: 300s)
  remoteGrokHome?:   string  // custom ~/.orca-relay dir
  // State:
  source: 'user' | 'fleet' | 'runtime-owned'
  owner:  string
  lastRequiredPassphrase?: number
}
```

**SshConnectionStore** quản lý tất cả SshTargets và connections:
```typescript
class SshConnectionStore {
  listTargets(): SshTarget[]
  getTarget(id): SshTarget | undefined
  addTarget(target): SshTarget
  updateTarget(id, updates): SshTarget | null
  removeTarget(id): void
  exportToFleetConfig(): FleetConfig
}
```

---

## 3. Relay Deploy — `deployAndLaunchRelay()`

### 3.1 Overall Flow

```typescript
// src/main/ssh/ssh-relay-deploy.ts:104
async function deployAndLaunchRelay(
  conn: SshConnection,
  onProgress?: (status: string) => void,
  graceTimeSeconds?: number,
  relayInstanceId?: string
): Promise<RelayDeployResult>
// Timeout: DEFAULT_DEPLOY_TIMEOUT_MS (vài phút)
```

### 3.2 Bước 1 — Detect Remote Platform

```typescript
// detectRemoteHostPlatform(conn)
// → SSH exec: uname -sm (Linux/Darwin/Windows)
// → Supported: linux-x64, linux-arm64, darwin-x64, darwin-arm64, win32-x64, win32-arm64

type RemoteHostPlatform = {
  os:            'linux' | 'darwin' | 'windows'
  arch:          'x64' | 'arm64'
  relayPlatform: RelayPlatform  // e.g. "linux-x64"
  shell:         string
  pathSeparator: '/' | '\\'
}
```

### 3.3 Bước 2 — Check existing relay

```typescript
// resolveRelayBootstrapState(conn, hostPlatform, fullVersion)
// Kiểm tra đồng thời:
// - remoteHome (~/)
// - remoteRelayDir (~/.orca-relay/{version}/)
// - alreadyInstalled: tồn tại .install-complete
// - nodePath: đường dẫn đến node binary
```

**Version**: Content-hash dựa trên relay binary, đọc từ `.version` file local.

### 3.4 Bước 3 — Upload relay (nếu chưa có)

```typescript
// uploadRelay(conn, platform, remoteDir, fullVersion, hostPlatform)
// 1. SSH exec: mkdir -p remoteDir
// 2. SFTP upload: localRelayDir → remoteDir (toàn bộ directory)
//    hoặc conn.uploadDirectory() nếu có (optimized path)
// 3. SSH exec: chmod +x {remoteDir}/node (Linux/Darwin)
// 4. SFTP write: {remoteDir}/.version = fullVersion

// atomicInstallLock: mkdir lock để tránh concurrent uploads
// → atomic mkdir: chỉ 1 process thắng, các process khác chờ
```

**Local relay dir**: Được đóng gói trong Orca binary:
```
{app}/resources/app.asar.unpacked/relay/linux-x64/
  ├── node                    # Node.js binary (stripped)
  ├── daemon-entry.js         # Relay entry point (~84KB bundle)
  ├── node_modules/           # Native deps (node-pty, etc.)
  └── .version                # Content hash
```

### 3.5 Bước 4 — Launch relay

```typescript
// launchRelay(conn, remoteRelayDir, hostPlatform, nodePath, graceTimeSeconds, instanceId)

// 1. Xác định socket path:
const sockFile = relayEndpointForHost(hostPlatform, remoteDir, fallbackSockName)
// Linux/Darwin: ~/.orca-relay/{version}/orca.sock  (Unix domain socket)
// Windows:      \\.\pipe\orca-relay-{hash}          (Named pipe)

// 2. Kiểm tra relay đang chạy (có socket sẵn không):
const existingEndpoint = await checkForActiveRelay(conn, remoteDir, instanceId, hostPlatform)
if (existingEndpoint) {
  // Reuse existing relay (grace period chưa hết)
  return { transport: connectToExisting(existingEndpoint), ... }
}

// 3. Launch mới:
const channel = await conn.exec(
  loginShellCommand(shell, `${nodePath} ${quoteSh(daemonPath)}`)
)

// 4. Wait for ORCA-RELAY sentinel trên stdout:
// Relay print "ORCA-RELAY\n" khi ready
const transport = await waitForSentinel(channel)
// transport = MultiplexerTransport over SSH channel stdout/stdin
```

---

## 4. Wire Protocol — MultiplexerTransport

Sau khi relay ready, Orca Server giao tiếp qua **JSON-RPC multiplexed over SSH channel**:

```
Orca Server                         Relay (daemon-entry.js)
    │                                       │
    │  SSH exec channel (stdin/stdout)      │
    │◄─────────────────────────────────────►│
    │                                       │
    │  MultiplexerTransport                 │
    │  [13-byte header][payload]            │
    │                                       │
    │── JSON-RPC request ──────────────────►│
    │   { method: "pty.create", args: ... } │ → execve() → PTY
    │                                       │
    │◄─ JSON-RPC response ─────────────────│
    │   { result: { ptyId } }               │
    │                                       │
    │◄─ binary PTY output ─────────────────│
    │   [streamId][compressed bytes]        │
    │                                       │
```

**Frame format (VS Code PersistentProtocol compatible):**
```
[4 bytes: length][1 byte: type][8 bytes: id][payload...]
Total header: 13 bytes
```

---

## 5. SshRelaySession — State Machine

```typescript
// src/main/ssh/ssh-relay-session.ts
export type RelaySessionState = 'idle' | 'deploying' | 'ready' | 'reconnecting' | 'disposed'

export class SshRelaySession {
  private _state: RelaySessionState = 'idle'
  private muxDisposeCleanup: (() => void) | null = null
  private _onReady: ((targetId: string) => void) | null = null

  getState(): RelaySessionState
  onReady(cb: (targetId: string) => void): void
  async establish(conn: SshConnection, graceTimeSeconds?: number): Promise<void>
  async reconnect(conn: SshConnection): Promise<void>
  dispose(): void
}
```

### 5.1 State Transitions

```
                   ┌─────────────────────────────────────────────────────┐
                   │                                                       │
    initial ──────►│ idle                                                  │
                   └──────┬──────────────────────────────────────────────┘
                          │ establish(conn)
                   ┌──────▼────────┐
                   │  deploying    │ deployAndLaunchRelay()
                   │               │ detectPlatform, upload, launch, handshake
                   └──────┬────────┘
                          │                     ┌── error ──►│ idle (retry) hoặc throw
                          │ success             │
                   ┌──────▼────────┐            │
                   │    ready      │ registerProviders()
                   │  ◄─────────── │ SshPtyProvider, SshGitProvider active
                   └──────┬────────┘
                          │
                   ┌──────┴────────────────────────────────────────────────┐
                   │  Triggers:                                             │
                   │  - network blip → mux transport closes                 │
                   │  - SSH connection drops                                │
                   └──────┬────────────────────────────────────────────────┘
                          │ mux onStateChange: disconnect
                   ┌──────▼────────┐
                   │ reconnecting  │ unregisterProviders()
                   │               │ reconnect(conn) → establish() lại
                   └──────┬────────┘
                          │ success
                   ┌──────▼────────┐
                   │    ready      │ registerProviders() lại
                   └──────┬────────┘
                          │ dispose() / user disconnect
                   ┌──────▼────────┐
                   │   disposed    │ cleanup all resources
                   └───────────────┘
```

### 5.2 establish() — Chi tiết

```typescript
async establish(conn: SshConnection, graceTimeSeconds?: number): Promise<void> {
  this._state = 'deploying'

  // 1. Deploy và launch relay binary
  const { transport, remoteHome, remoteRelayDir, nodePath, sockPath, hostPlatform }
    = await deployAndLaunchRelay(conn, undefined, graceTimeSeconds, this.targetId)

  // 2. Lưu env cho provider registration
  this.remoteEnv = { remoteHome, binDir, nodePath, ... }

  // 3. Tạo mux (JSON-RPC multiplexer)
  const mux = new MultiplexerTransport(transport)

  // 4. Configure grace time
  this.configureRelayGraceTime(mux, graceTimeSeconds)

  // 5. Register providers (PTY + Git)
  const registered = await this.registerProviders(mux, ownsAttempt)

  // 6. Watch for disconnect → trigger reconnect
  mux.onStateChange((state) => {
    if (state === 'closed' && !this.isDisposed()) {
      this._state = 'reconnecting'
      this.reconnect(conn)
    }
  })

  this._state = 'ready'
  this._onReady?.(this.targetId)
}
```

### 5.3 registerProviders()

```typescript
private async registerProviders(mux, ownsAttempt): Promise<boolean> {
  // PTY Provider: cung cấp terminal sessions qua relay
  const ptyProvider = new SshPtyProvider(this.targetId, mux, this.remoteCliBridgeEnv)
  registerSshPtyProvider(this.targetId, ptyProvider)

  // Git Provider: cung cấp git operations qua relay
  const gitProvider = new SshGitProvider(this.targetId, mux, this.remoteEnv)
  registerSshGitProvider(this.targetId, gitProvider)

  // Khi relay disconnect: unregister providers để tránh stale calls
  return true
}
```

---

## 6. Provider Dispatch

Sau khi relay ready, terminal và git calls được route qua relay:

```
User trong web browser
  → terminal.create({ worktreeId, ... })
  → OrcaRuntimeService
  → getProviderForTarget(sshTargetId) → SshPtyProvider
  → mux.request('pty.create', args)
  → [SSH channel] → relay daemon-entry.js
  → spawn PTY process (node-pty)
  → PTY output → mux stream → SSH → OrcaServer → WebSocket → Browser
```

```typescript
// src/main/providers/ssh-pty-provider.ts
class SshPtyProvider {
  constructor(targetId, mux, remoteCliBridgeEnv) {}

  async createPty(args): Promise<{ ptyId }> {
    return mux.request('pty.create', args)
  }

  async writeToPty(ptyId, data): Promise<void> {
    return mux.notify('pty.write', { ptyId, data })
  }

  subscribeOutput(ptyId, onData): () => void {
    return mux.subscribe(`pty.output.${ptyId}`, onData)
  }
}
```

---

## 7. Grace Period — Relay Keepalive

**Grace period** là thời gian relay tự giữ sống sau khi Orca Server ngắt kết nối:

```typescript
const DEFAULT_SSH_RELAY_GRACE_PERIOD_SECONDS = 300  // 5 phút

// Gửi cho relay khi connect:
mux.notify(SSH_RELAY_CONFIGURE_GRACE_TIME_METHOD, { graceTimeSeconds: N })
// Relay sẽ tự exit sau N giây nếu không có connection mới

// Khi user disconnect (close browser):
mux.notify(SSH_RELAY_CONFIGURE_GRACE_TIME_METHOD, { graceTimeSeconds: 0 })
// → relay exit ngay lập tức (không grace)
```

**Tại sao cần grace period:**
- Network blip ngắn (vài giây) → relay không cần restart, reconnect dùng lại process cũ
- User đóng browser → sau N giây → relay tự exit → giải phóng resources trên dev server

---

## 8. Relay Version Management

```typescript
// Version = content hash của relay binary package
// Lưu ở: {remoteRelayDir}/.version

// Khi connect:
const fullVersion = readLocalFullVersion(localRelayDir)
// → tìm ~/.orca-relay/{version}/ trên remote
// → nếu không khớp → upload version mới vào new dir

// GC cũ:
gcOldRelayVersions(conn, remoteHome, currentRelayDir, ...)
// → xoá các dir ~/.orca-relay/{old-version}/ không còn dùng
// → Best-effort: error trong GC không block user
```

---

## 9. SSH Connection (Underlying Transport)

```typescript
// src/main/ssh/ssh-connection.ts — SshConnection
// Wrapper xung quanh ssh2 npm package

class SshConnection {
  // Capabilities
  exec(command): Promise<Channel>              // SSH exec channel
  sftp(): Promise<SFTPStream>                  // SFTP session
  uploadDirectory(local, remote, opts): Promise<void>  // SFTP batch upload
  writeFile(remotePath, content): Promise<void>
  readFile(remotePath): Promise<string>

  // Auth: prefer identity file (SSH key), fallback password
  // IdentityFile: /home/orca/.ssh/id_ed25519 (mounted trong container)
}
```

**SSH config trong container:**
```
# /home/orca/.ssh/config
Host dev-local
    HostName 172.20.2.31
    Port 22
    User ubuntu
    IdentityFile /home/orca/.ssh/id_ed25519
    UserKnownHostsFile /home/orca/.ssh/known_hosts
    StrictHostKeyChecking accept-new
    ServerAliveInterval 30
    ServerAliveCountMax 3
```

---

## 10. Fleet Integration

Orca hỗ trợ "fleet" — tập hợp nhiều remote servers được quản lý tập trung:

```typescript
// SshConnectionStore.exportToFleetConfig()
type FleetConfig = {
  version: 1
  servers: Array<{ id, host, port, username, label, ... }>
}

// Import từ fleet config file
SshConnectionStore.importFromFleetConfig(config)

// Fleet targets được đánh dấu: source = 'fleet'
// Runtime-owned targets: source = 'runtime-owned', id = 'runtime:{runtimeId}'
```

---

## 11. Relay Lifecycle Sequence đầy đủ

```
User clicks "Connect to server 172.20.2.31"
    │
    ▼
OrcaRuntimeService.ssh.connect({ targetId })
    │
    ▼
SshConnectionStore.createConnection(target)
    │── SshConnection → ssh2 → TCP connect → SSH handshake → auth
    │
    ▼
SshRelaySession.establish(conn)
    │── state = 'deploying'
    │── deployAndLaunchRelay(conn)
    │    ├── detectRemoteHostPlatform()       SSH exec: uname -sm
    │    ├── resolveRelayBootstrapState()     SSH exec: ls, node -v (concurrent)
    │    ├── [if !installed] uploadRelay()    SFTP: upload ~84KB bundle
    │    ├── [if !installed] installNativeDeps()  SSH exec: npm rebuild
    │    ├── [if !installed] finalizeInstall()    SFTP: write .install-complete
    │    └── launchRelay()
    │         ├── SSH exec: node daemon-entry.js --grace=300 --sock=orca.sock
    │         ├── waitForSentinel(): read stdout until "ORCA-RELAY\n"
    │         └── return MultiplexerTransport
    │── registerProviders(mux)
    │    ├── new SshPtyProvider(targetId, mux)  → registerSshPtyProvider
    │    └── new SshGitProvider(targetId, mux)  → registerSshGitProvider
    │── configureRelayGraceTime(mux, 300)
    │── state = 'ready'
    │── onReady(targetId) callback
    │
    ▼
Web client thấy: "Connected to dev-local"

    [Network blip]
    │
    ▼
mux.onStateChange('closed')
    │── state = 'reconnecting'
    │── unregisterProviders()
    │── reconnect(conn) → establish(conn) lại
    │
    ▼
state = 'ready' (sau ~2-5s reconnect)

    [User disconnects]
    │
    ▼
SshRelaySession.dispose()
    │── mux.notify(configureGraceTime, {graceTimeSeconds: 0})
    │── muxDisposeCleanup()
    │── unregisterProviders()
    │── state = 'disposed'
    │── SSH connection.end()

    [Relay exits sau 0s (hoặc graceTimeSeconds nếu blip)]
```

---

## 12. Hiện trạng và Kế hoạch

### Hiện tại

- ✅ SSH relay deploy: detect, upload (SFTP), launch, sentinel
- ✅ MultiplexerTransport: JSON-RPC over SSH channel
- ✅ SshPtyProvider + SshGitProvider
- ✅ Grace period keepalive
- ✅ Reconnect sau network blip
- ✅ Version management + GC cũ
- ✅ Atomic install lock (concurrent deploy safety)
- ✅ Multi-platform: linux-x64/arm64, darwin-x64/arm64, win32-x64/arm64

---

## 13. RelayConnectionPool (v5.0)

Thay vì tạo SSH relay mới cho mỗi user, v5.0 giới thiệu `RelayConnectionPool` để **reuse** kết nối relay theo `devServerId`:

```typescript
// src/main/dev-server/relay-connection-pool.ts
class RelayConnectionPool {
  // Shared pool: devServerId → DevServerRelayBridge
  private pool = new Map<string, DevServerRelayBridge>()

  async getOrConnect(devServerId: string): Promise<DevServerRelayBridge> {
    const existing = this.pool.get(devServerId)

    // ✅ REUSE: nếu relay đang ready
    if (existing?.state === 'ready') return existing

    // ⏳ WAIT: nếu đang reconnect
    if (existing?.state === 'reconnecting') return existing.waitReady()

    // 🆕 CREATE: establish new relay
    const conn = await SshConnectionStore.connect(devServerId)
    const bridge = new DevServerRelayBridge(devServerId, conn)
    await bridge.establish()
    this.pool.set(devServerId, bridge)
    return bridge
  }
}
```

**Lợi ích**:
- Nhiều user cùng project → dùng chung 1 SSH relay (không tạo N SSH connections)
- Khi user đầu tiên connect → relay warm up cho tất cả users sau
- Relay lifetime độc lập với user session lifecycle

---

## 14. AgentConnectionManager — Persistent WS Pool (v6.0)

v6.0 giới thiệu `AgentConnectionManager` quản lý **pool WS connections đến Dev Server Agents**:

```typescript
// src/main/dev-server/agent-connection-manager.ts
class AgentConnectionManager {
  // Persistent pool: devServerId → AgentWsConnection
  private pool = new Map<string, AgentWsConnection>()

  // Khi Agent (direct-ws mode) kết nối:
  onAgentConnected(agentId: string, devServerId: string, ws: WebSocket): void {
    const conn = new AgentWsConnection(agentId, devServerId, ws)
    this.pool.set(devServerId, conn)
    conn.onDisconnect(() => this.pool.delete(devServerId))
  }

  // Gửi RPC đến agent (persistent, không cần reconnect):
  async call(devServerId: string, method: string, params: any): Promise<any> {
    const conn = this.pool.get(devServerId)
    if (!conn?.isAlive) throw new Error(`No active agent for ${devServerId}`)
    return conn.rpc(method, params)
  }
}
```

---

## 15. HMAC-SHA256 Signed RpcExecutionContext (v6.0)

Mọi RPC từ Orca Server đến Dev Server Agent đều kèm **signed context** để verify:

```typescript
// Context được tạo khi user authenticated:
const ctx: RpcExecutionContext = {
  userId:    user.id,
  userName:  user.name,
  userEmail: user.email,
  devServerId,
  projectId,
  worktreeId,
  issuedAt:  Date.now(),
  expiresAt: Date.now() + 30_000,  // 30s TTL
}

// Sign bằng ORCA_RELAY_SECRET (shared secret Orca Server ↔ Agent)
const signature = createHmac('sha256', ORCA_RELAY_SECRET)
  .update(JSON.stringify(ctx))
  .digest('hex')

// Relay RPC request:
relay.call('git.status', {
  cwd: worktreePath,
  _ctx: { ...ctx, signature }   // kèm trong mỗi request
})

// Dev Server Agent: verify trước khi execute
context-verifier.ts:
  const expected = hmacSHA256(ORCA_RELAY_SECRET, JSON.stringify(ctx_without_sig))
  if (expected !== ctx.signature) throw new Error('Invalid context signature')
  if (Date.now() > ctx.expiresAt) throw new Error('Context expired')
  // → Proceed with trusted userId, userEmail
```

**Tác dụng**: Dev Server tin tưởng userId trong ctx để:
- Git commit: `GIT_AUTHOR_EMAIL = ctx.userEmail`
- AI spawn: inject correct credentials cho ctx.userId
- PTY session: lưu vào per-userId session store

---

## 16. Cross-References (v5/v6)

| Resource | Mô tả |
|---|---|
| [project-workspace-switch.md](./project-workspace-switch.md) | RelayConnectionPool.getOrConnect() usage |
| [agent-connection-modes.md](./agent-connection-modes.md) | AgentConnectionManager (direct-ws mode) |
| [authentication.md](./authentication.md) | HMAC context source từ session |
| **HLD C2 Container 13–16** | Profile/Project/AI/Workflow services (all via relay) |
| **HLD C4.4** | Fleet Management (DevServerRelayBridge) |
| **F34 Project Binding** | Project → Dev Server relay routing |
