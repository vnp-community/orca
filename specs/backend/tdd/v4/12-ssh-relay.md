# TDD-BE-12: SSH & Relay

**Version:** 4.0  
**Date:** 2026-07-28  
**Source:** `src/main/ssh/`, `src/relay/`

---

## 1. SSH Connection Subsystem

### 1.1 SSH Connection Manager

```typescript
class SshConnectionManager {
  // Create connection from stored config
  async connect(id: string): Promise<void>
  async disconnect(id: string): Promise<void>

  // Resolve connection for a user (multi-user routing)
  getConnection(id: string, userId?: string): SshConnection | null
}
```

### 1.2 SSH User Resolver (CR-LOGIN-003)

```typescript
// src/main/ssh/ssh-user-resolver.ts
function toLinuxUsername(email: string, uid: string): string {
  // Format: 'orca-<prefix>'
  // prefix = lowercase(email.split('@')[0]).replace(/[^a-z0-9]/g, '')
  //          truncated to 20 chars max, deduplicated với uid hash nếu collision
  // e.g., 'binhnt@example.com' → 'orca-binhnt'
}
```

### 1.3 Dev Server Provisioner (CR-LOGIN-003)

```typescript
// src/main/ssh/dev-server-provisioner.ts
class DevServerProvisioner {
  // Idempotent provisioning: create linux user + deploy relay + authorize SSH key
  async provision(serverId: string, userId: string): Promise<ProvisionResult>
}
```

**Provisioning steps:**
1. `toLinuxUsername(email, userId)` → linux username
2. SSH: `useradd -m -s /bin/bash <linux-user>` (idempotent via `id <linux-user>` check)
3. Deploy relay binary to `~<linux-user>/.orca/relay/`
4. Authorize SSH public key tới `~<linux-user>/.ssh/authorized_keys`
5. Audit log: `ssh.provision`

---

## 2. Relay Binary

### 2.1 Relay Deploy

```typescript
// src/main/ssh/fleet-bootstrap-service.ts
class FleetBootstrapService {
  // 7-step bootstrap pipeline
  async bootstrap(serverId: string): Promise<BootstrapResult> {
    // Step 1: SSH connect
    // Step 2: Detect platform/arch
    // Step 3: Upload relay binary
    // Step 4: Set permissions (chmod +x)
    // Step 5: Install as systemd service
    // Step 6: Start service
    // Step 7: Health check
  }
}
```

### 2.2 Relay Binary Versioning

```
userData/relay/
├─ 1.2.3/
│   └─ orca-relay-linux-x64
├─ 1.2.4/
│   └─ orca-relay-linux-arm64
└─ current → 1.2.4/  (symlink)
```

GC old versions: giữ N=2 phiên bản gần nhất.

---

## 3. SshChannelMultiplexer

Binary frame protocol dùng để multiplex nhiều channels qua một WebSocket:

```
Frame format (13-byte header):
  [0]     TYPE (1 = Regular, 2 = Keepalive, 3 = Control)
  [1-4]   CHANNEL_ID (u32 LE)
  [5-8]   SEQ (u32 LE) — monotonic, per-channel
  [9-12]  LENGTH (u32 LE)
  [13+]   PAYLOAD (LENGTH bytes)
```

```typescript
class SshChannelMultiplexer {
  // Open new channel
  openChannel(channelId: number): SshChannel

  // Send on channel
  send(channelId: number, data: Buffer): void

  // Receive handler
  onData(channelId: number, handler: (data: Buffer) => void): void

  // Close channel
  closeChannel(channelId: number): void

  // ACK tracking (prevent timeout on keepalive)
  sendKeepalive(): void
}
```

**Timeout:** 20s (ACK must be received). Keepalive gửi mỗi 10s.

---

## 4. Fleet Config Parser

```typescript
// src/main/ssh/fleet-config-parser.ts (Zod schema)
export type FleetConfig = {
  version:  '1.0'
  servers:  FleetServer[]
  groups?:  FleetGroup[]
  defaults?: FleetDefaults
}

export type FleetServer = {
  id:       string
  name:     string
  host:     string
  port?:    number      // default 22
  user?:    string      // SSH user
  keyId?:   string      // SSH key reference
  group?:   string
  tags?:    string[]
}
```

---

## 5. Fleet Health Monitor

```typescript
class FleetHealthMonitor {
  // Periodic health check (default: 60s)
  start(): void
  stop(): void

  // Webhook notify on status change
  onStatusChange(callback: (serverId: string, status: HealthStatus) => void): void

  getStatus(serverId: string): HealthStatus
  getAllStatuses(): Map<string, HealthStatus>
}
```

---

## 6. Audit Log (SSH events)

```
ssh.connect      — user kết nối tới dev server
ssh.disconnect   — user disconnect
ssh.provision    — user được provision linux account
ssh.key_added    — SSH key được authorize
```
