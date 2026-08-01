# SSH Relay Connection: Orca Server → Dev Machine

> **Scope:** Cơ chế Orca Server (172.20.2.39) kết nối vào Dev Machine (172.20.2.31)
> để relay filesystem, terminal, git operations.
>
> **Key files:**
> - [`src/main/ssh/ssh-connection.ts`](../../src/main/ssh/ssh-connection.ts) — SSH connection lifecycle (dùng `ssh2` npm)
> - [`src/main/ssh/ssh-relay-deploy.ts`](../../src/main/ssh/ssh-relay-deploy.ts) — deploy relay binary qua SFTP
> - [`src/main/ssh/relay-protocol.ts`](../../src/main/ssh/relay-protocol.ts) — wire format (13-byte framing)
> - [`src/shared/ssh-types.ts`](../../src/shared/ssh-types.ts) — `SshTarget` type definition

---

## Tổng quan kiến trúc

```
[Browser]
    ↓ wss://b15.openledger.vn (WSS/E2EE)
[Orca Server: 172.20.2.39]       ← container Node.js
    ↓ SSH port 22 (ssh2 npm library)
[Dev Machine: 172.20.2.31]       ← code để ở đây
    ~/.orca-remote/v1.4.138/
    node daemon-entry.js          ← relay process
    ↕ JSON-RPC qua SSH channel
[Orca Server]                     ← relay mọi request
```

---

## Giao thức: SSH + Relay Node.js

Orca **không** dùng HTTP hay WebSocket để kết nối vào Dev Machine.
Toàn bộ dùng **SSH** (thư viện `ssh2` npm) với các sub-protocol:

| Layer | Protocol | Dùng cho |
|-------|----------|---------|
| Transport | SSH (port 22) | Kết nối bảo mật |
| File transfer | SFTP (sub-channel) | Upload relay binary lần đầu |
| Command exec | SSH exec channel | Launch relay process |
| RPC | JSON-RPC over SSH channel | fs, terminal, git operations |
| Binary | Custom 13-byte framing | PTY output, file streaming |

---

## Phase 1: SSH Connect

**File:** [`ssh-connection.ts:86`](../../src/main/ssh/ssh-connection.ts#L86)

```typescript
// SshTarget — lưu trong Orca state store
const target: SshTarget = {
  id:           "dev-local-31",
  label:        "Dev Local — 172.20.2.31",
  host:         "172.20.2.31",
  port:         22,
  username:     "ubuntu",
  identityFile: "/data/orca/.ssh/id_ed25519",  // key trong Docker volume
  project:      "vnp-blc",
  environment:  "development"
}

// Orca dùng npm `ssh2` — không dùng hệ thống ssh binary
import { Client as SshClient } from 'ssh2'

const conn = new SshConnection(target, callbacks)
conn.connect()
// → ssh2 kết nối TCP đến 172.20.2.31:22
// → handshake + key-based auth với identityFile
```

**Auth flow:**
```
Orca Server                     Dev Machine (172.20.2.31)
     │                                │
     │── TCP connect :22 ────────────→│
     │← SSH banner ──────────────────│
     │── publickey auth ─────────────→│  /data/orca/.ssh/id_ed25519
     │                                │  check ~/.ssh/authorized_keys
     │←── auth OK ───────────────────│
     │   SSH session established      │
```

**Retry policy:**
```typescript
const INITIAL_RETRY_ATTEMPTS = 3
const INITIAL_RETRY_DELAY_MS = 500
const RECONNECT_BACKOFF_MS   = [1000, 2000, 4000, 8000, 15000]
```

---

## Phase 2: Deploy Relay (lần đầu connect)

**File:** [`ssh-relay-deploy.ts:368`](../../src/main/ssh/ssh-relay-deploy.ts#L368)

Relay là **Node.js bundle** (~84KB, đã compiled) — upload 1 lần, tái sử dụng các lần sau.

```
[Deploy pipeline]

1. Detect remote platform
   → SSH exec: `uname -s`, `uname -m`
   → Result: linux-x64 (172.20.2.31)

2. Check relay version
   → SFTP: read ~/.orca-remote/v1.4.138/.version
   → If match: skip upload ✅
   → If mismatch: upload

3. Upload relay bundle qua SFTP
   → local:  out/relay/  (~84KB)
   → remote: ~/.orca-remote/v1.4.138/
   → Files:  daemon-entry.js, các deps native

4. Resolve Node.js path trên remote
   → SSH exec: `which node` hoặc `~/.nvm/...`
   → 172.20.2.31 có: node v20.20.2

5. Launch relay process
   → SSH exec:
     node ~/.orca-remote/v1.4.138/daemon-entry.js \
       --grace-time=86400

6. Wait for sentinel
   → Relay prints to stdout: "ORCA-RELAY v0.1.0 READY\n"
   → Orca reads this → connection ready ✅
```

**Timeout guards:**
```typescript
const RELAY_DEPLOY_TIMEOUT_MS        = 300_000  // 5 phút toàn pipeline
const NATIVE_DEPS_INSTALL_TIMEOUT_MS = 240_000  // 4 phút npm install (nếu cần)
const RELAY_SENTINEL_TIMEOUT_MS      = 10_000   // 10s chờ READY sentinel
```

---

## Phase 3: JSON-RPC qua SSH Channel

**File:** [`relay-protocol.ts`](../../src/main/ssh/relay-protocol.ts)

Sau khi relay sẵn sàng, mọi operation đều là **JSON-RPC** qua SSH multiplexed channels.

### Wire Format (VS Code PersistentProtocol)

```
┌─────────────────────────────────────────────┐
│              13-byte Header                  │
├──────┬──────────┬──────────────────────────┤
│ Type │  Length  │   (reserved padding)      │
│  1B  │   4B     │         8B               │
└──────┴──────────┴──────────────────────────┘
│              JSON Payload                    │
│           (variable length)                  │
└─────────────────────────────────────────────┘
```

```typescript
export const HEADER_LENGTH    = 13
export const MAX_MESSAGE_SIZE = 16 * 1024 * 1024  // 16MB

export const MessageType = {
  Regular:   1,  // JSON-RPC request/response
  KeepAlive: 9   // Liveness probe (mỗi 5s)
} as const

export const KEEPALIVE_SEND_MS = 5_000   // probe interval
export const TIMEOUT_MS        = 20_000  // timeout nếu không nhận reply
```

### Ví dụ RPC calls

```
Orca Server (172.20.2.39)               Relay (172.20.2.31)
      │                                        │
      │── [header: type=1, len=52]            │
      │   { id:1, method:"fs.readDir",        │
      │     params:{ path:"/home/ubuntu/code"}}│
      │────────────────────────────────────────→
      │                                        │ read dir
      │←── { id:1, ok:true, result:[...] } ───│
      │                                        │
      │── { id:2, method:"terminal.spawn",    │
      │    params:{ cwd:"/home/ubuntu/code",  │
      │             cols:80, rows:24 } }      │
      │────────────────────────────────────────→
      │                                        │ fork PTY
      │←── { id:2, ok:true,                   │
      │      result:{ pid:1234 } }            │
      │                                        │
      │ [header: type=1]  binary PTY frames   │
      │←───────────────────────────────────────│  PTY output stream
      │                                        │
      │── [type=9] KeepAlive (5s interval) ───→│
      │←── [type=9] KeepAlive ────────────────│
```

### File streaming (lớn hơn 256KB)

```typescript
// Chunk files thành 256KB pieces
export const STREAM_CHUNK_SIZE = 256 * 1024

// Git diff lớn → stream theo chunks
export const GIT_RESPONSE_STREAM_THRESHOLD = 256 * 1024
export const GIT_RESPONSE_CHUNK_SIZE       = 128 * 1024
```

---

## Phase 4: Grace Period & Reconnect

```typescript
// Relay không tắt ngay khi Orca Server disconnect
// Giữ process sống để reconnect nhanh
export const DEFAULT_GRACE_TIME_MS = 0           // default: tắt ngay
// Khuyến nghị cho dev: 86400s (1 ngày)
// Max: 604800s (7 ngày)

// Deploy: --grace-time=86400
```

**Reconnect flow:**
```
Orca Server mất kết nối SSH
    ↓
scheduleReconnect()
    ↓
SSH connect lại đến 172.20.2.31:22
    ↓
Relay vẫn còn sống (grace period)? → connect lại socket
Relay đã tắt? → redeploy + launch relay mới
```

---

## Setup thực tế (đã hoàn thành)

### SSH Key trong Orca Container

```bash
# Key đã được tạo tự động
/data/orca/.ssh/id_ed25519       ← private key (trong Docker volume)
/data/orca/.ssh/id_ed25519.pub   ← public key
/data/orca/.ssh/known_hosts      ← fingerprint của 172.20.2.31

# Public key:
ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIPCWDC9fmup9a48mxh/aaxxM7XaTeLmls23viMZW3IfN orca-server@172.20.2.39
```

### Authorized trên Dev Machine

```bash
# 172.20.2.31: /home/ubuntu/.ssh/authorized_keys
ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIPCWDC9fmup9a48mxh/aaxxM7XaTeLmls23viMZW3IfN orca-server@172.20.2.39
```

### Kiểm tra kết nối

```bash
# Từ máy local — test SSH container → dev machine
ssh ubuntu@172.20.2.39 "
  docker exec orca-server sh -c '
    ssh -i /data/orca/.ssh/id_ed25519 \
        -o UserKnownHostsFile=/data/orca/.ssh/known_hosts \
        ubuntu@172.20.2.31 \"echo OK && node --version\"
  '
"
# Output: OK / v20.20.2
```

### SshTarget cần add vào Orca

```json
{
  "host":         "172.20.2.31",
  "port":         22,
  "username":     "ubuntu",
  "identityFile": "/data/orca/.ssh/id_ed25519",
  "label":        "Dev Local — 172.20.2.31",
  "project":      "vnp-blc",
  "environment":  "development"
}
```

---

## Relay trên Dev Machine sau khi connect

```bash
# Thư mục được tạo tự động trên 172.20.2.31:
~/.orca-remote/
  v1.4.138/
    daemon-entry.js     # relay binary (~84KB)
    orca-relay.sock     # Unix socket (nếu dùng socket mode)
    .version            # "1.4.138" — dùng để check xem cần upload lại không
```

---

## Điều kiện Dev Machine cần đáp ứng

| Yêu cầu | Lý do | Kiểm tra |
|---------|-------|---------|
| SSH server port 22 | Orca SSH vào | `sshd -t` |
| Node.js ≥ 18 | Chạy relay process | `node --version` |
| `~/.ssh/authorized_keys` có Orca key | Auth không cần password | `cat ~/.ssh/authorized_keys` |
| Disk space ≥ 10MB | Upload relay + native deps | `df -h ~` |
| `/tmp` writable | Relay socket | `ls -la /tmp` |
