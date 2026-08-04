# TDD-05: SSH & SSH Relay

**Document:** TDD-05  
**Domain:** SSH Connection & Relay Protocol  
**Source files:** `src/main/ssh/`  

---

## 1. Tổng quan

Orca có 2 layer SSH:

```
Layer 1: SSH Transport
  - Kết nối tới remote server (ssh2 library + system SSH)
  - Thực hiện: auth, port forward, SFTP, channel multiplexer

Layer 2: SSH Relay
  - Sau khi kết nối, deploy orca-relay binary lên remote
  - relay chạy như Node.js process trên remote
  - Tạo PTY trên remote, pipe qua relay → SSH → Orca main
```

### Module map

```
src/main/ssh/
├── Connection layer
│   ├── ssh-connection.ts            (~41K) — core SSH connection
│   ├── ssh-connection-store.ts      — target CRUD + SQLite
│   ├── ssh-connection-manager.ts    — connection pool
│   ├── ssh-channel-multiplexer.ts   (~17K) — multiplex streams
│   ├── ssh-auth-resolution.ts       — key/agent/password auth
│   ├── ssh-config-parser.ts         — ~/.ssh/config reader
│   ├── ssh-config-loader.ts         — config file loading
│   └── ssh-port-forward.ts          — port forwarding
│
├── Relay layer
│   ├── ssh-relay-deploy.ts          (~43K) — relay install + start
│   ├── ssh-relay-session.ts         (~51K) — relay session lifecycle
│   ├── ssh-relay-versioned-install.ts — versioned deploy logic
│   ├── ssh-relay-deploy-helpers.ts  — SFTP upload, exec helpers
│   ├── ssh-relay-endpoints.ts       — socket paths
│   ├── ssh-relay-protocol.ts        — relay protocol types
│   └── relay-protocol.ts            — protocol constants
│
└── Remote utilities
    ├── ssh-remote-node-resolution.ts — detect Node.js on remote
    ├── ssh-remote-platform.ts        — detect OS on remote
    ├── ssh-remote-commands.ts        — remote command builders
    └── system-ssh-command.ts         — system ssh binary exec
```

---

## 2. SshConnection (`ssh-connection.ts`)

```typescript
// ~41K — core SSH connection abstraction
class SshConnection {
  private client: ssh2.Client       // ssh2 library
  private config: SshConnectionConfig

  async connect(): Promise<void> {
    // Resolve auth method:
    // 1. SSH agent (ORCA_SSH_AGENT_SOCK)
    // 2. Identity file (~/.ssh/id_ed25519, id_rsa)
    // 3. Password (prompt user)
    const authMethod = await resolveAuthMethod(this.config)
    await this.client.connect({ ...this.config, ...authMethod })
  }

  async exec(command: string): Promise<ExecResult>
  async sftp(): Promise<SFTPSession>
  async openChannel(): Promise<ssh2.Channel>
  async portForward(localPort, remoteHost, remotePort): Promise<PortForward>
}
```

### Auth Resolution

```typescript
// src/main/ssh/ssh-auth-resolution.ts
async function resolveAuthMethod(config: SshTarget): Promise<AuthConfig> {
  // Priority:
  // 1. IdentityAgent (SSH_AUTH_SOCK)
  if (config.identityAgent) {
    return { agent: config.identityAgent }
  }

  // 2. IdentityFile với passphrase (nếu có)
  if (config.identityFile) {
    const key = await readPrivateKey(config.identityFile)
    if (isEncrypted(key)) {
      const passphrase = await promptPassphrase(config.label)
      return { privateKey: key, passphrase }
    }
    return { privateKey: key }
  }

  // 3. Fallback: system SSH agent
  return { agent: process.env['SSH_AUTH_SOCK'] }
}
```

---

## 3. SSH Channel Multiplexer

```typescript
// src/main/ssh/ssh-channel-multiplexer.ts (~17K)
// Multiplexes nhiều logical channels qua 1 SSH connection

class SshChannelMultiplexer {
  // Protocol: prefix mỗi frame với channel ID + length
  // Cho phép: PTY, SFTP, exec channels chạy song song
  // Backpressure: flow control để không overwhelm SSH socket

  openChannel(type: 'pty' | 'exec' | 'sftp'): MultiplexerChannel
  closeChannel(channelId: string): void
}
```

---

## 4. SSH Relay Deploy (`ssh-relay-deploy.ts`)

Đây là module quan trọng nhất — tự động cài relay lên remote server:

```typescript
// src/main/ssh/ssh-relay-deploy.ts (~43K)
export async function deployRelay(
  connection: SshConnection,
  options: RelayDeployOptions
): Promise<RelayDeployResult> {

  // Step 1: Detect remote platform (Linux/Darwin/Windows)
  const platform = await detectRemoteHostPlatform(connection)

  // Step 2: Detect/install Node.js
  const nodePath = await resolveRemoteNodePath(connection)
  // Nếu không có Node → throw với hướng dẫn install

  // Step 3: Check nếu relay version đã đúng
  const remoteDir = computeRemoteRelayDir(platform, home)
  if (await isRelayAlreadyInstalled(connection, remoteDir, localVersion)) {
    return connectToExistingRelay(connection, remoteDir, nodePath)
  }

  // Step 4: Acquire install lock (prevent concurrent installs)
  await acquireInstallLock(connection, remoteDir)

  // Step 5: Upload relay bundle via SFTP
  await uploadDirectory(
    connection,
    localRelayDir,    // từ app resources
    remoteRelayDir
  )

  // Step 6: Start relay process
  const relay = await startRelayProcess(connection, nodePath, remoteRelayDir)

  // Step 7: Finalize + GC old versions
  await finalizeInstall(connection, remoteDir)
  await gcOldRelayVersions(connection, home)

  return { transport: relay.transport, platform }
}
```

### Versioned Install

```typescript
// src/main/ssh/ssh-relay-versioned-install.ts
// Relay versions trong thư mục: ~/.orca-relay/<version>/
// Ví dụ: ~/.orca-relay/1.4.138/

computeRemoteRelayDir(platform, home): string
isRelayAlreadyInstalled(conn, dir, version): Promise<boolean>
acquireInstallLock(conn, dir): Promise<void>
finalizeInstall(conn, dir): Promise<void>
gcOldRelayVersions(conn, home): Promise<void>  // xóa versions cũ
```

---

## 5. SSH Relay Session (`ssh-relay-session.ts`)

```typescript
// src/main/ssh/ssh-relay-session.ts (~51K)
// Manages live session với relay process

class SshRelaySession {
  // Transport: Unix socket trên remote (qua SSH forward)
  // Protocol: relay-protocol.ts (binary framing)

  async createPty(opts: PtyCreateOptions): Promise<string>
  async writePty(ptyId: string, data: Uint8Array): Promise<void>
  async resizePty(ptyId: string, cols: number, rows: number): Promise<void>
  async killPty(ptyId: string, signal?: string): Promise<void>

  // File operations qua relay:
  async readFile(path: string): Promise<Uint8Array>
  async writeFile(path: string, data: Uint8Array): Promise<void>
  async listDir(path: string): Promise<DirEntry[]>

  // Git qua relay:
  async gitExec(cwd: string, args: string[]): Promise<ExecResult>
}
```

---

## 6. Relay Protocol (`relay-protocol.ts`)

```typescript
// src/main/ssh/relay-protocol.ts
// Binary frame protocol: Main ↔ Relay

// Frame format: [type(1)] [length(4)] [payload(length)]
type RelayMessageType =
  | 0x01   // PTY_CREATE
  | 0x02   // PTY_WRITE
  | 0x03   // PTY_RESIZE
  | 0x04   // PTY_KILL
  | 0x05   // PTY_DATA (event)
  | 0x06   // PTY_EXIT (event)
  | 0x10   // FILE_READ
  | 0x11   // FILE_WRITE
  | 0x12   // DIR_LIST
  | 0x20   // GIT_EXEC
  | 0x30   // PORT_SCAN_RESULT (event)
```

---

## 7. SSH Connection Store

```typescript
// src/main/ssh/ssh-connection-store.ts
class SshConnectionStore {
  constructor(private store: Store) {}

  listTargets(): SshTarget[]                        // filter runtime-owned
  addTarget(target): SshTarget                      // CRUD
  updateTarget(id, updates): SshTarget | null
  removeTarget(id): void                            // + tombstone
  importFromSshConfig(options?): SshTarget[]        // parse ~/.ssh/config
}
```

### SshTarget type (từ `src/shared/ssh-types.ts`)

```typescript
type SshTarget = {
  id: string                    // 'ssh-<timestamp>-<random>'
  label: string                 // display name
  host: string                  // IP or hostname
  port: number                  // default: 22
  username: string
  identityFile?: string         // path to private key
  identityAgent?: string        // SSH agent socket
  identitiesOnly?: boolean      // OpenSSH IdentitiesOnly
  proxyCommand?: string         // ProxyCommand
  jumpHost?: string             // ProxyJump
  configHost?: string           // alias in ~/.ssh/config
  source?: 'ssh-config' | 'manual'
  relayGracePeriodSeconds?: number  // 0 = no expiry; default 0
  lastRequiredPassphrase?: boolean  // để optimize startup reconnect
  portForwards?: SavedPortForward[]
  systemSshConnectionReuse?: boolean
}
```

---

## 8. Node.js Detection (`ssh-remote-node-resolution.ts`)

```typescript
// Thứ tự tìm Node.js trên remote:
const NODE_LOOKUP_PATHS = [
  'node',                              // PATH
  'nodejs',
  '$HOME/.nvm/versions/node/*/bin/node',
  '$HOME/.fnm/node-versions/*/installation/bin/node',
  '$HOME/.volta/bin/node',
  '/usr/local/bin/node',
  '/opt/homebrew/bin/node',
  '/usr/bin/node',
]

// Minimum version requirement: Node 18+
```

---

## 9. Port Forwarding (`ssh-port-forward.ts`)

```typescript
// Tự động detect ports trên remote (qua relay port scan)
// Tạo local forward khi port detected

class SshPortForward {
  async create(
    targetId: string,
    remoteHost: string,
    remotePort: number,
    localPort: number
  ): Promise<PortForward>

  // Auto-restore: portForwards persisted trong SshTarget.portForwards
  async restoreForTargetForwards(targetId: string): Promise<void>
}
```

---

## 10. Grace Period (Relay Keepalive)

```typescript
// src/shared/ssh-types.ts
// relayGracePeriodSeconds: thời gian relay sống sau khi client disconnect
// 0 = relay tắt ngay khi disconnect (default)
// 86400 = relay sống 24h (dùng cho long-running sessions)
// Max: 604800 (7 ngày)

// Configured per SSH target:
const SSH_RELAY_CONFIGURE_GRACE_TIME_METHOD = 'relay.configureGraceTime'
// → Gửi tới relay để update grace timer
```

---

## Addendum v3.0: Fleet Management Extensions (remote-server CRs) — IMPLEMENTED ✅

> **Date:** 2026-07-23 | **TDD-13:** [13-dev-server-onboarding.md](./13-dev-server-onboarding.md)

### SshTarget Extensions (CR-001, CR-002)

```typescript
// src/shared/ssh-types.ts — MODIFIED:
type SshTarget = {
  // ...existing fields unchanged...
  tags?: string[]                    // NEW — for fleet grouping
  group?: string                     // NEW — group name
  metadata?: Record<string, string>  // NEW — custom key-value
  importedFrom?: 'ssh-config' | 'fleet-yaml' | 'manual'  // NEW
  fleetConfigPath?: string           // NEW — source fleet.yaml path
}

type SshTargetGroup = {             // NEW TYPE
  name: string
  targets: SshTarget[]
  tags: string[]
}
```

### Fleet Files (`src/main/ssh/fleet-*.ts`)

| File | CR | Purpose |
|------|----|---------|
| `fleet-config-parser.ts` | CR-001 | YAML fleet config + Zod validation |
| `fleet-remote-commands.ts` | CR-004 | Node/Git install, repo clone via SSH |
| `fleet-bootstrap-service.ts` | CR-004 | 7-step bootstrap pipeline |
| `fleet-health-store.ts` | CR-005 | In-memory health history + uptime |
| `fleet-health-monitor.ts` | CR-005 | Periodic poll + webhook/IPC alerts |
| `fleet-status-service.ts` | CR-005 | Standalone fleet status query |

### Fleet Config YAML (`orca-fleet.yaml`)

```yaml
fleet:
  servers:
    - host: dev1.internal
      user: ubuntu
      tags: [linux, backend]
      group: staging
  defaults:
    user: ubuntu
    identity: ~/.ssh/id_ed25519
```

### Bootstrap Pipeline (7 steps, idempotent)

```
Step 1: SSH connectivity test
Step 2: Remote platform detection
Step 3: Node.js version check / install
Step 4: Git version check / install
Step 5: Relay binary deploy
Step 6: Relay process start
Step 7: Agent detection + handshake
```

### Relay Preflight Extensions (CR-OB-003, CR-OB-007)

```typescript
// src/relay/preflight-handler.ts — MODIFIED:
// detectAgents() now includes:
{
  agents: string[]
  platform: NodeJS.Platform   // NEW
}

// detectWindowsTerminalCapabilities() — EXTENDED:
{
  wslAvailable: boolean
  wslDistros: string[]
  pwshAvailable: boolean
  pwshVersion?: string        // NEW
  gitBashAvailable: boolean
  gitBashPath?: string        // NEW
}
```

### RBAC (`src/shared/rbac-types.ts`, CR-006)

Phase 1+2 (multi-instance isolation) implemented:
```typescript
type ScopedPairingToken = {
  token: string
  userId: string
  allowedSshTargetIds: string[]  // scoped access
  expiresAt: number
}
```

Phase 3 (OIDC/SSO) deferred: `src/main/auth/oidc-handler.ts`

---

## Addendum v4.0: SSH User Isolation (login CRs CR-LOGIN-003) — IMPLEMENTED ✅

> **Date:** 2026-07-24 | **Status:** Complete | **Tests:** 29

### Per-User Linux Account Isolation

Mỗi user Orca khi relay SSH sang dev server được map sang 1 linux username riêng, tạo sandbox cô lập hoàn toàn trên dev server.

```
OrcaUser (userId, email)
  └── SshUserResolver.toLinuxUsername(email, userId)
        ├── prefix = email local part, sanitized ≤8 chars
        ├── suffix = sha256(userId).hex.slice(0,6)
        └── → 'orca-<prefix>-<suffix>'  (e.g. 'orca-alice-a1b2c3')
```

### DevServerProvisioner

```typescript
class DevServerProvisioner {
  async provision(userId: string, sshTarget: SshTarget): Promise<ProvisionResult> {
    // 1. Resolve linux username
    // 2. Check if user exists (idempotent)
    // 3. Create linux user if needed (useradd -m)
    // 4. Deploy relay binary to /home/<username>/.orca-relay/
    // 5. Authorize SSH public key
    // 6. Log to audit: ssh.connect { userId, linuxUser, targetId }
  }
}
```

Idempotent — safe to call multiple times. Backward compat: khi `ORCA_MULTI_USER=0`, resolver không được gọi.

### SSH Connection Store — Per-User Resolution

`SshConnectionStore` hiện hỗ trợ per-user username resolution:

```typescript
store.resolveConnectionUsername(targetId, userId)
// → 'orca-alice-a1b2c3'  (user-specific)
// → original username    (ORCA_MULTI_USER=0)
```

### Files

| File | Mô tả |
|------|-------|
| `src/main/ssh/ssh-user-resolver.ts` | `toLinuxUsername()`, collision-safe hashing |
| `src/main/ssh/dev-server-provisioner.ts` | Idempotent provision, relay deploy, key auth |
| `src/main/ssh/__tests__/` | 29 tests covering resolver + provisioner |

Tham khảo:
- [TDD-13: Dev Server Onboarding §5](./13-dev-server-onboarding.md) — relay deploy mechanics
- `src/main/ssh/ssh-user-resolver.ts`
- `src/main/ssh/dev-server-provisioner.ts`

---

## Addendum v5.0: Provider Registries Are Transport-Agnostic — IMPLEMENTED ✅

> **Date:** 2026-08-02/03 | **TDD-13:** [13-dev-server-onboarding.md §11](./13-dev-server-onboarding.md#11-provider-unification-with-ssh-registries-v50)

Trước đây, mọi file/git operation trên remote host đi qua `orca-runtime.ts` đều giả định host là một **SSH Target** — lookup bằng `connectionId` vào 2 registry:

```typescript
// src/main/providers/ssh-filesystem-dispatch.ts
// src/main/providers/ssh-git-dispatch.ts
const sshProviders = new Map<string, IGitProvider>()  // key: opaque connection-id string

registerSshGitProvider(connectionId: string, provider: IGitProvider): void
getSshGitProvider(connectionId: string): IGitProvider | undefined
requireSshGitProvider(connectionId: string): IGitProvider
```

Nhận thấy 2 registry này **đã transport-agnostic** — key chỉ là 1 string bất kỳ, provider chỉ cần implement `IGitProvider`/`IFilesystemProvider` — nên một **Dev Server** (agent kết nối OUTBOUND qua WebSocket, xem TDD-13) giờ đăng ký vào **CÙNG registry** mà SSH dùng, keyed bằng `devServerId` thay vì SSH `targetId`. Không mở thêm connection nào mới; ~40+ call site đọc từ registry (qua `getSshGitProvider`/`getSshFilesystemProvider`) không đổi.

```typescript
// src/main/providers/ssh-git-dispatch.ts — WIDENED:
// Map và các hàm export (registerSshGitProvider/getSshGitProvider/requireSshGitProvider)
// được retype từ concrete `SshGitProvider` sang interface `IGitProvider`,
// để registry chứa được cả provider SSH-backed lẫn Dev-Server-backed.
// ssh-filesystem-dispatch.ts đã dùng interface type từ trước — không cần đổi.
```

Provider mới cho Dev Server (`src/main/providers/`):

| File | Vai trò |
|------|---------|
| `dev-server-relay-connection.ts` | `DevServerRelayConnection` type — shape tối thiểu `{ call(), onNotification?() }`, không type theo concrete `DevServerRelayBridge` (multi-user child process chỉ thấy `GatewayDevServerManagerProxy.getRelay()`, một plain object khác forward qua IPC — cả 2 đều thoả interface) |
| `dev-server-filesystem-provider.ts` | `DevServerFilesystemProvider implements IFilesystemProvider` — compose từ agent RPC surface hẹp (fs.stat/readDir/readFile/writeFile/mkdir/rmdir/glob/grep, fs.watch/unwatch); method không có agent equivalent (rename, copy, realpath) throw "not supported for Dev Server hosts yet" |
| `dev-server-git-provider.ts` | `DevServerGitProvider implements IGitProvider` — compose từ 1 method generic `git.exec({args, cwd})` (agent whitelist subcommand, không shell) + `git.worktree.list/add/remove`; tái dùng `StatusPorcelainParser` không đổi |
| `dev-server-provider-lifecycle.ts` | `wireDevServerProviders(devServerManager)` — lắng nghe `devServer:statusChanged`/`devServer:removed`, register/unregister 2 provider trên vào registry theo `devServerId`. Gọi 1 lần trong `server-bootstrap.ts` ngay sau khi tạo `devServerManager` — giống nhau cho single-user main process và multi-user parent/gateway process |

`IGitProvider` (`src/main/providers/types.ts`) có thêm vài method optional (executeCommitMessagePlan, cancelGenerateCommitMessage, execNonInteractive, clone, fetchRemoteTrackingRef, fetchGitLabMergeRequestHead, refreshLocalBaseRefForWorktreeCreate, getHostPlatform) để các call site SSH-only dùng interface đã widen vẫn type-check — Dev Server provider đơn giản là omit các method này.

**File watch:** `.watch()` dùng push thật (subscribe qua `relay.onNotification` + gọi `fs.watch`) khi agent hỗ trợ, fallback về poll readDir-diff mỗi 3s cho agent binary cũ hơn — fallback này giúp rollout an toàn khi chỉ 1 phần Dev Server đã update agent.

**Chưa làm:** `DevServerPtyProvider` (`IPtyProvider` cho Dev Server) chưa được xây — xem TDD-13 §11 để biết lý do (thiếu ~8 method equivalent + chưa Dev Server nào report `pty=true` ở handshake).

Tham khảo:
- [TDD-13: Dev Server Onboarding §11](./13-dev-server-onboarding.md#11-provider-unification-with-ssh-registries-v50)
- `src/main/providers/ssh-git-dispatch.ts`, `ssh-filesystem-dispatch.ts`
- `src/main/providers/dev-server-*.ts`
