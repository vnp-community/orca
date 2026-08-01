# Authentication — Orca Server

> **Scope**: Cơ chế xác thực client kết nối đến Orca Server
> **Key files**:
> - [`src/shared/pairing.ts`](../../src/shared/pairing.ts) — PairingOffer encode/decode
> - [`src/main/runtime/rpc/e2ee-channel.ts`](../../src/main/runtime/rpc/e2ee-channel.ts) — E2EE handshake state machine
> - [`src/main/runtime/rpc/e2ee-crypto.ts`](../../src/main/runtime/rpc/e2ee-crypto.ts) — ECDH + XChaCha20 crypto
> - [`src/main/runtime/runtime-rpc.ts`](../../src/main/runtime/runtime-rpc.ts) — Auth enforcement, admission control
> - [`src/shared/rbac-types.ts`](../../src/shared/rbac-types.ts) — User/RBAC types (kế hoạch)

---

## 1. Tổng quan

Orca hiện tại dùng **PairCode + E2EE** là cơ chế xác thực duy nhất. Không có login/SSO. Mỗi kết nối WebSocket phải hoàn thành E2EE handshake trước khi gửi bất kỳ RPC nào.

```
Client (Browser/Mobile/CLI)         Orca Server (172.20.2.39)
             │                              │
             │── WS connect ───────────────►│ WS :6768
             │                              │ tạo E2EEChannel (state=awaiting_hello)
             │── e2ee_hello ───────────────►│ { type, publicKeyB64 }
             │                              │ ECDH: derive sharedKey
             │                              │ state → awaiting_auth
             │◄─ e2ee_ready ────────────────│ { type: 'e2ee_ready' } (plaintext)
             │                              │
             │── e2ee_auth (encrypted) ────►│ { type, deviceToken }
             │                              │ validateToken(deviceToken) → DeviceEntry
             │                              │ state → ready
             │                              │ onReady(channel)
             │                              │
             │  ── mọi RPC sau đây đều encrypted ──
             │── { authToken, method, ... }►│ decrypt → handle → encrypt reply
             │◄─ { result/error } ──────────│
```

---

## 2. PairCode — Cơ chế cấp phát credential

### 2.1 Cấu trúc PairingOffer

```typescript
// src/shared/pairing.ts
export const PAIRING_OFFER_VERSION = 2

export type PairingOffer = {
  v: 2                    // version
  endpoint: string        // "wss://b15.openledger.vn"
  deviceToken: string     // 48 ký tự hex (24 random bytes)
  publicKeyB64: string    // Curve25519 public key của server, base64
  scope?: 'mobile' | 'runtime'  // 'runtime' = CLI/web, 'mobile' = phone
}
```

### 2.2 Encoding

```typescript
export function encodePairingOffer(offer: PairingOffer): string {
  const json = JSON.stringify(offer)
  const base64url = Buffer.from(json).toString('base64')
    .replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
  return `orca://pair?code=${base64url}`
}
```

**Ví dụ output:**
```
orca://pair?code=eyJ2IjoyLCJlbmRwb2ludCI6IndzczovL2IxNS5vcGVubGVkZ2VyLnZuIiwiZGV2aWNlVG9rZW4iOiJhYmMxMjMuLi4iLCJwdWJsaWNLZXlCNjQiOiJvUDdRNy4uLiIsInNjb3BlIjoicnVudGltZSJ9
```

Web client cũng chấp nhận bare base64 (không có `orca://` prefix) và `https://b15.openledger.vn/#pairing=<base64>`.

### 2.3 Decoding & Validation (client side)

```typescript
export function parsePairingCode(input: string): PairingOffer | null {
  // Accept: orca://pair?code=..., bare base64, https://domain/#pairing=...
  if (input.startsWith('orca://')) return decodePairingOffer(input)
  return decodePairingBase64(input)  // bare base64
}
```

### 2.4 Tạo PairingOffer trên server

```typescript
// src/main/runtime/runtime-rpc.ts
createPairingOffer(args: { address?, name?, rotate?, scope? }) {
  const endpoint       = this.getWebSocketEndpoint()  // "wss://b15.openledger.vn"
  const publicKeyB64   = this.getE2EEPublicKey()       // server's Curve25519 pubkey
  const device         = args.rotate
    ? registry.rotatePendingDevice(args.name, scope)
    : registry.getOrCreatePendingDevice(args.name, scope)
  // device.token = 48-hex random string — credential duy nhất cho lần pair này
  const pairingUrl = encodePairingOffer({
    v: 2, endpoint, deviceToken: device.token, publicKeyB64, scope
  })
  return { available: true, pairingUrl, endpoint, deviceId: device.deviceId, webClientUrl }
}
```

---

## 3. E2EE Handshake — State Machine

### 3.1 Crypto primitives

```typescript
// src/shared/e2ee-crypto.ts (được re-export qua e2ee-crypto.ts)
generateKeyPair()    // Curve25519 keypair (server keypair, persistent)
deriveSharedKey(serverSecretKey, clientPublicKey)  // ECDH → shared 32-byte key
encrypt(sharedKey, plaintext: string): Uint8Array  // XChaCha20-Poly1305
decrypt(sharedKey, ciphertext: Uint8Array): string
encryptBytes(sharedKey, bytes): Uint8Array         // Binary (PTY frames)
decryptBytes(sharedKey, bytes): Uint8Array
```

**Thuật toán:**
- Key exchange: **Curve25519 ECDH** (X25519)
- Encryption: **XChaCha20-Poly1305** (AEAD, authenticated)
- Nonce: random 24 bytes per message (prepended to ciphertext)

### 3.2 State Machine: `E2EEChannel`

```
                  ┌───────────────┐
    WS connect ──►│awaiting_hello │
                  └──────┬────────┘
                         │ receive e2ee_hello { publicKeyB64 }
                         │ ECDH: sharedKey = deriveSharedKey(serverSecret, clientPub)
                         │ send e2ee_ready (plaintext)
                  ┌──────▼────────┐
                  │awaiting_auth  │ timeout: 10s → close(4002)
                  └──────┬────────┘
                         │ receive e2ee_auth (encrypted) { deviceToken }
                         │ decrypt với sharedKey
                         │ validateToken(deviceToken) → DeviceEntry?
                         │   FAIL → close(4003, 'auth failed')
                  ┌──────▼────────┐
                  │    ready      │◄─── mọi RPC đều encrypt/decrypt qua đây
                  └──────┬────────┘
                         │ disconnect / error
                         └── destroy() → cleanup timer, state
```

```typescript
// src/main/runtime/rpc/e2ee-channel.ts
type ChannelState = 'awaiting_hello' | 'awaiting_auth' | 'ready'

const HANDSHAKE_TIMEOUT_MS         = 10_000  // 10s để hoàn thành handshake
const MAX_CONSECUTIVE_DECRYPT_FAILURES = 5   // đóng kết nối nếu decrypt fail liên tục
```

### 3.3 Luồng chi tiết

**Step 1 — Client gửi e2ee_hello:**
```json
{ "type": "e2ee_hello", "publicKeyB64": "<client Curve25519 pubkey base64>" }
```
Server nhận → ECDH → `sharedKey` → state = `awaiting_auth`.

**Step 2 — Server gửi e2ee_ready (plaintext):**
```json
{ "type": "e2ee_ready" }
```

**Step 3 — Client gửi e2ee_auth (encrypted với sharedKey):**
```json
{ "type": "e2ee_auth", "deviceToken": "<48-hex token from PairingOffer>" }
```
Server decrypt → `validateToken(token)` → DeviceEntry tồn tại → state = `ready`.

**Step 4 — Tất cả message sau đều encrypted:**
```
plaintext: '{"authToken":"...","id":1,"method":"runtime.status",...}'
  ↓ encrypt(sharedKey, plaintext)
ciphertext: <binary, 24-byte nonce + encrypted payload>
```

---

## 4. Auth Token Enforcement (RPC layer)

Sau khi E2EE channel `ready`, mọi RPC request phải có `authToken`:

```typescript
// src/main/runtime/runtime-rpc.ts — parseAndAuthenticate()
if (typeof request.authToken !== 'string' || request.authToken.length === 0) {
  return { error: 'missing authToken' }
}

// Priority 1: shared runtime authToken (Unix socket, Electron IPC)
if (request.authToken === this.authToken) {
  return { authenticated: true, scope: 'runtime' }
}

// Priority 2: scoped token (in-memory, 24h TTL, RBAC-limited)
const scopedToken = this.deviceRegistry?.getScopedToken(request.authToken)
if (scopedToken && Date.now() < scopedToken.expiresAt) {
  return { authenticated: true, scope: scopedToken, ... }
}

return { error: 'invalid authToken' }
```

**Ba loại authToken:**

| Loại | Nguồn | Scope | TTL | Lưu trữ |
|------|-------|-------|-----|---------|
| `runtime authToken` | `randomBytes(24)` khi khởi động | Full (all methods) | Process lifetime | In-memory |
| `deviceToken` | `DeviceRegistry`, 48-hex | Per-device | Persistent | `orca-devices.json` |
| `ScopedPairingToken` | `DeviceRegistry.generateScopedToken()` | RBAC limited | 24h | In-memory |

---

## 5. Server E2EE Keypair — Persistent Identity

```typescript
// src/main/runtime/e2ee-keypair.ts
export function loadOrCreateE2EEKeypair(userDataPath: string): E2EEKeypair {
  const path = join(userDataPath, 'orca-e2ee-keypair.json')
  if (existsSync(path)) return JSON.parse(readFileSync(path))  // reuse existing
  // Tạo mới: Curve25519 keypair
  const { publicKey, secretKey } = generateKeyPair()
  const keypair = {
    publicKeyB64: Buffer.from(publicKey).toString('base64'),
    secretKeyB64: Buffer.from(secretKey).toString('base64'),
  }
  writeSecureJsonFile(path, keypair)  // chmod 600
  return keypair
}
```

- **Persistent**: keypair tồn tại qua container restart (mounted volume `/data/orca`)
- **Identity**: `publicKeyB64` là identity của server — client dùng để verify khi pair
- **Security**: `secretKey` lưu với permission 600, không expose qua API

---

## 6. Unix Socket Auth (Electron IPC / CLI)

```typescript
// runtime-rpc.ts — listenOnLocalSocket()
// Why: Unix socket transport uses the shared runtime auth token.
// This is intentionally weaker than E2EE — the socket is protected by
// OS file permissions (0o600), not application-layer encryption.
```

Trên local (Electron hoặc CLI tool), authToken được lấy từ `orca-runtime.json`:
```json
{ "socketPath": "/data/orca/o-1-07f4.sock", "authToken": "abc123..." }
```
File này cũng có permission 600, chỉ process owner đọc được.

---

## 7. Admission Control — Rate Limiting

```typescript
// runtime-rpc.ts — long-poll admission fence
// Why: prevent concurrent slow RPCs (agent runs, git clone) from
// blocking the WS event loop for other clients
const MAX_CONCURRENT_SLOW_RPCS = 3  // (giá trị thực trong source)

// Short RPCs bypass counter — only methods classified as 'slow' are gated
function isSlowMethod(method: string): boolean { ... }
```

---

## 8. Security Properties tóm tắt

| Property | Cơ chế | Mức độ |
|---------|--------|--------|
| **Confidentiality** | XChaCha20-Poly1305 AEAD | Mạnh |
| **Authentication** | deviceToken in E2EE auth message | Trung bình |
| **Integrity** | Poly1305 MAC per message | Mạnh |
| **Forward Secrecy** | ❌ Không (server keypair persistent) | Thiếu |
| **Token Revocation** | `DeviceRegistry.removeDevice()` | Có |
| **Replay Prevention** | ❌ Không có nonce tracking | Thiếu |
| **DoS Protection** | Handshake timeout 10s, decrypt failure limit 5 | Cơ bản |

---

## 9. Web Server Mode Auth (v5.0) — Triển khai

Auth đã được implement cho Web Server Mode (F23). Xem chi tiết đầy đủ tại [multi-user-session.md](./multi-user-session.md).

### 9.1 HTTP-based Auth (POST /auth/local)

```
Browser → POST /auth/local { email, password }
        → bcrypt.compare(pw, hash, 12r)
        → INSERT orca_sessions (token 64-hex, userId, expires_at+8h)
        → Set-Cookie: orca_session=<token>; HttpOnly; SameSite=Strict; Secure
```

### 9.2 WsSessionRouter — Per-User Sandbox

Sau khi có session cookie, WebSocket connections được route qua `WsSessionRouter`:

```
WS ws://:6768 + Cookie: orca_session=<token>
    │
    ▼ WsSessionRouter.onConnection()
    │  → validateSession(token) → OrcaUser
    │  → getOrCreateUserRuntime(userId)   [isolated per user]
    │
    └── OrcaRuntimeRpcServer (user-scoped)
        → PTY, worktrees, agents — all filtered by userId
```

### 9.3 Hai auth mode song song

| Mode | Transport | Auth mechanism | TTL |
|---|---|---|---|
| **Desktop / Web Pairing** | WebSocket E2EE | deviceToken (48-hex) trong e2ee_auth | Persistent |
| **Web Server Mode** | HTTP cookie + WS | `orca_session` cookie (64-hex) | 8h sliding |

### 9.4 HMAC-SHA256 Signed Context (v6.0)

Khi Orca Server relay RPC đến Dev Server Agent, mọi request đều kèm `RpcExecutionContext` được ký:

```typescript
// src/main/rpc/context-builder.ts
const ctx: RpcExecutionContext = {
  userId:    session.userId,
  userName:  session.user.name,
  userEmail: session.user.email,
  devServerId, projectId, worktreeId,
  issuedAt:  Date.now(),
  expiresAt: Date.now() + 30_000,  // 30s TTL
}
const signature = hmacSHA256(ORCA_RELAY_SECRET, JSON.stringify(ctx))
// → Agent verifies signature before executing any RPC
```

---

## 10. Cross-References (cập nhật v5/v6)

| Resource | Mô tả |
|---|---|
| [multi-user-session.md](./multi-user-session.md) | Chi tiết đầy đủ Web Server Mode auth (F22/23/24) |
| [account-management.md](./account-management.md) | Device registry và E2EE token management |
| [profile-resolution.md](./profile-resolution.md) | Profile resolve sau login |
| [relay-management.md](./relay-management.md) | SSH relay với signed context |
| **HLD C1 Flow 5** | Web Server Multi-User flow |
| **HLD C4.2** | Platform Abstraction + Web Server Bootstrap |
| **F22 Web Server Mode** | Feature spec |
| **F23 Multi-User Auth** | Feature spec |
| **BL-AUTH-01** | Local login business logic |
| **BL-AUTH-02** | Session management business logic |
