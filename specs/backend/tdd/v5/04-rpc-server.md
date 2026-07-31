# TDD-04: WebSocket / Unix RPC Server

**Document:** TDD-04  
**Domain:** RPC Server — API surface, Auth, E2EE, Transport  
**Source files:** `src/main/runtime/runtime-rpc.ts`, `src/main/runtime/rpc/`

---

## 1. Tổng quan

**`OrcaRuntimeRpcServer`** là **security boundary** duy nhất của Orca backend:
- Nhận requests từ Browser/App/Mobile/CLI
- Enforce authentication (auth token)
- Manage E2EE sessions (mobile/web)
- Dispatch tới `RpcDispatcher` → method handlers
- Quản lý connection pool (max 32)
- Keepalive cho long-poll operations

```typescript
// src/main/runtime/runtime-rpc.ts
const DEFAULT_WS_PORT = 6768
const KEEPALIVE_INTERVAL_MS = 10_000   // 10s
const LONG_POLL_CAP = 16               // max concurrent long-poll RPCs
```

---

## 2. Transport Layers

### 2.1 WebSocket Transport (`rpc/ws-transport.ts`)

```typescript
// Dùng cho: Browser, Orca Mobile, Orca Desktop (web serve mode)
class WebSocketTransport implements RpcTransport {
  private server: WebSocketServer   // ws package

  start(port: number): Promise<void>
  // Lắng nghe connections
  // Cho mỗi connection: tạo E2EEChannel nếu cần

  send(ws: WebSocket, response: RpcResponse): void
  // Serialize JSON + send

  handleMessage(ws: WebSocket, data: Buffer): void
  // Parse JSON → RpcRequest
  // Hoặc: binary TerminalStreamFrame → terminal data
}
```

**Serve static web bundle:**
```typescript
// src/main/runtime/rpc/static-web-client-handler.ts
// HTTP GET → serve files từ out/web/ directory
// Tích hợp vào cùng WebSocket server (HTTP upgrade)
```

### 2.2 Unix Socket Transport (`rpc/unix-socket-transport.ts`)

```typescript
// Dùng cho: Orca CLI (local machine)
class UnixSocketTransport implements RpcTransport {
  private server: net.Server

  start(socketPath: string): Promise<void>
  // Lắng nghe trên Unix socket file

  // One-shot: mỗi connection = 1 request + 1 response
  // Không support streaming methods
  // Auth: token trong request body
}
```

---

## 3. Authentication

### 3.1 Auth Token

```typescript
// Khi khởi động, tạo random token:
const authToken = randomBytes(32).toString('hex')

// Ghi ra file: ~/.config/orca/runtime/runtime.json
// CLI đọc file này để lấy token
writeRuntimeMetadata({ authToken, wsPort, socketPath, pid })
```

### 3.2 Token Enforcement

```typescript
// src/main/runtime/runtime-rpc.ts
function handleConnection(ws: WebSocket) {
  const firstMessage = await waitForFirstMessage(ws)

  // Verify auth token
  if (firstMessage.token !== this.authToken) {
    ws.close(4001, 'Unauthorized')
    return
  }

  // Hoặc E2EE pairing (nếu là web/mobile)
  if (firstMessage.type === 'pair') {
    const channel = new E2EEChannel(...)
    // Thực hiện Curve25519 key exchange
    // Token được derive từ pairing session key
  }
}
```

---

## 4. E2EE (End-to-End Encryption)

### 4.1 Pairing Flow

```typescript
// src/shared/pairing.ts
function encodePairingOffer(keypair: E2EEKeypair): string {
  // Base64url encode:
  // { publicKey: Uint8Array(32), version: number }
}

// Pairing URL:
// orca://pair?code=<base64url-encoded-offer>
// Web UI: https://domain/web-index.html?pair=<code>
```

### 4.2 Key Exchange

```typescript
// src/main/runtime/rpc/e2ee-channel.ts
class E2EEChannel {
  // Curve25519 key exchange via TweetNaCl
  // Server: static keypair (persisted in ~/.config/orca/e2ee-keypair.json)
  // Client: ephemeral keypair mỗi session

  async pair(clientPublicKey: Uint8Array): Promise<void> {
    // 1. Compute shared secret: box.before(clientPublicKey, serverPrivateKey)
    // 2. Derive session key: HKDF(sharedSecret, nonce)
    // 3. From this point: all messages encrypted with NaCl secretbox

    this.sessionKey = computeSharedKey(clientPublicKey, this.serverKeypair.secretKey)
  }

  encrypt(plaintext: Uint8Array): Uint8Array {
    return nacl.secretbox(plaintext, nonce, this.sessionKey)
  }

  decrypt(ciphertext: Uint8Array): Uint8Array {
    return nacl.secretbox.open(ciphertext, nonce, this.sessionKey)
  }
}
```

### 4.3 Server Keypair Storage

```typescript
// src/main/runtime/e2ee-keypair.ts
async function loadOrCreateE2EEKeypair(userDataPath: string): Promise<E2EEKeypair> {
  const keypairPath = join(userDataPath, 'e2ee-keypair.json')
  if (existsSync(keypairPath)) {
    return loadKeypair(keypairPath)
  }
  // Tạo mới Curve25519 keypair
  const keypair = nacl.box.keyPair()
  // Persist (private key ở local only)
  saveKeypair(keypairPath, keypair)
  return keypair
}
```

---

## 5. RPC Protocol

### 5.1 Request Format

```typescript
// src/main/runtime/rpc/core.ts
type RpcRequest = {
  id: string           // UUID
  method: string       // e.g. 'worktree.create'
  params: unknown      // method-specific params
  token?: string       // auth token (first message hoặc Unix socket)
}
```

### 5.2 Response Format

```typescript
type RpcResponse =
  | { id: string; result: unknown; meta: RpcEnvelopeMeta }
  | { id: string; error: { code: string; message: string }; meta: RpcEnvelopeMeta }

type RpcEnvelopeMeta = {
  serverVersion: string    // e.g. '1.4.138'
  serverPlatform: string   // 'darwin' | 'linux' | 'win32'
}
```

### 5.3 Streaming (Terminal output)

```typescript
// Terminal data không dùng JSON request/response
// Dùng binary frame protocol:

// src/shared/terminal-stream-protocol.ts
type TerminalStreamFrame = {
  ptyId: string
  data: Uint8Array    // raw terminal bytes
  seq: number         // sequence number (flow control)
}

// Binary encoding: [ptyId-length(2)] [ptyId-utf8] [seq(4)] [data]
```

---

## 6. RPC Dispatcher

```typescript
// src/main/runtime/rpc/dispatcher.ts
class RpcDispatcher {
  private registry: Map<string, RpcAnyMethod>

  async dispatch(request: RpcRequest): Promise<RpcResponse> {
    // 1. Lookup method
    const method = this.registry.get(request.method)
    if (!method) return errorResponse('method_not_found')

    // 2. Zod validation
    const parsed = method.schema.safeParse(request.params)
    if (!parsed.success) return errorResponse('invalid_params', formatZodError(parsed.error))

    // 3. Execute
    try {
      const result = await method.handler(parsed.data, { runtime: this.runtime })
      return successResponse(request.id, result)
    } catch (e) {
      return mapRuntimeError(request.id, e)
    }
  }
}
```

---

## 7. Method Categories (All 36 groups)

```typescript
// src/main/runtime/rpc/methods/index.ts
// ALL_RPC_METHODS = [
STATUS_METHODS,          // status.check
AI_VAULT_METHODS,        // ai-vault.get, ai-vault.set
AUTOMATION_METHODS,      // automation.create, .run, .list, .update, .delete
REPO_METHODS,            // repo.list, .create, .update, .delete
WORKTREE_METHODS,        // worktree.create, .list, .delete, .update
TERMINAL_METHODS,        // terminal.create, .write, .resize, .kill, .subscribe
BROWSER_CORE_METHODS,    // browser.open, .navigate, .click
BROWSER_EXTRA_METHODS,   // browser.screenshot, .pdf
BROWSER_SCREENCAST_METHODS, // browser.screencast.start/stop
ORCHESTRATION_METHODS,   // orchestration.run, .check, .cancel
NOTIFICATION_METHODS,    // notification.send
STATS_METHODS,           // stats.get
DIAGNOSTICS_METHODS,     // diagnostics.get
ACCOUNT_METHODS,         // accounts.list (Claude, Codex, etc.)
PREFLIGHT_METHODS,       // preflight.check
COMPUTER_METHODS,        // computer.screenshot, .click, .type
SESSION_TAB_METHODS,     // session-tabs.list, .create, .move
NATIVE_CHAT_METHODS,     // native-chat.send
FILE_METHODS,            // file.read, .write, .list, .search
GIT_METHODS,             // git.status, .log, .diff, .commit, .push
GITHUB_METHODS,          // github.pr.list, .create
GITLAB_METHODS,          // gitlab.mr.list, .create
HOSTED_REVIEW_METHODS,   // hosted-review.get
LINEAR_METHODS,          // linear.issue.list, .create
LINEAR_AGENT_ACCESS_METHODS, // linear.agent.*
JIRA_METHODS,            // jira.issue.list, .create
SSH_METHODS,             // ssh.list, .connect
SPEECH_METHODS,          // speech.transcribe
WORKSPACE_PORT_METHODS,  // workspace-ports.list
SKILL_METHODS,           // skills.list
CLIPBOARD_METHODS,       // clipboard.read, .write
HOST_CAPABILITY_METHODS, // host-capabilities.get
CLIENT_EVENT_METHODS,    // client-events.emit
CLIENT_UI_METHODS,       // client-ui.navigate
EMULATOR_METHODS,        // emulator.* (mobile emulator)
// ]
```

---

## 8. Connection Management

```typescript
const MAX_RUNTIME_RPC_CONNECTIONS = 32

class OrcaRuntimeRpcServer {
  private connections = new Map<WebSocket, ConnectionState>()

  onConnection(ws: WebSocket) {
    if (this.connections.size >= MAX_RUNTIME_RPC_CONNECTIONS) {
      ws.close(4002, 'Too many connections')
      return
    }
    this.connections.set(ws, { authenticated: false, deviceId: null })
    ws.on('close', () => this.connections.delete(ws))
  }
}
```

---

## 9. Runtime Metadata

```typescript
// src/main/runtime/runtime-metadata.ts
// Ghi file: ~/.config/orca/runtime/runtime.json
type RuntimeMetadata = {
  pid: number
  wsPort: number
  socketPath: string
  authToken: string           // CLI dùng để authenticate
  webClientRoot?: string      // path to out/web/
  transport: RuntimeTransportMetadata
}

// CLI đọc file này để biết:
// - Nơi kết nối (port/socket)
// - Auth token
```

---

## 10. Long-poll và Keepalive

```typescript
// Cho operations dài (orchestration.check --wait):
// - Server gửi keepalive JSON mỗi 10s
// - Max 16 concurrent long-poll slots (LONG_POLL_CAP)
// - Nếu vượt cap → trả về lỗi 'runtime_busy'

const KEEPALIVE_INTERVAL_MS = 10_000
const LONG_POLL_CAP = 16

// Keepalive frame:
{ "_keepalive": true }  // minimal JSON để reset idle timers
```

---

## Addendum v2.0: Web Server Mode Enhancements (restructure_v1) — IMPLEMENTED ✅

> **Date:** 2026-07-23

### WebIpcBridge — New Component

`OrcaRuntimeRpcServer` xử lý JSON-RPC protocol (`method`, `params`).  
`WebIpcBridge` xử lý IPC-style protocol (`type: 'invoke' | 'send'`):

```typescript
// Phân biệt trong ws-transport message handling:
function isIpcStyleMessage(msg: any): boolean {
  return msg.type === 'invoke' || msg.type === 'send'
}

// invoke: renderer calls ipcMain.handle() handler
// { id, type: 'invoke', channel: 'filesystem:readFile', args: [path] }
// → { id, type: 'result', result: <file content> }
// → { id, type: 'error', message: 'File not found' }

// send: fire-and-forget ipcMain.on() event
// { type: 'send', channel: 'pty:write', args: [ptyId, data] }
// → no reply

// push: server → client (window.send equivalents)
// { type: 'push', channel: 'pty:data', args: [{ ptyId, data }] }
```

### enableWebSocket = true (Server Mode)

```typescript
// OrcaRuntimeRpcServer constructor options:
{
  runtime: OrcaRuntimeService,
  userDataPath: string,
  enableWebSocket: true,     // ← kích hoạt WS transport
  wsPort: 6768               // ← port cho WebSocket
}
```

### HTTP Server Integration (Port :6769)

Server mode thêm HTTP server riêng biệt (port 6769) phục vụ web SPA:

```
Port 6768: WebSocket (OrcaRuntimeRpcServer + WebIpcBridge)
Port 6769: HTTP (startHttpServer → serve out/web/)
```

HTTP server và WebSocket server chạy trên 2 port **tách biệt** (không phải cùng server).

### Tham khảo

- [TDD-10: Platform Layer](./10-platform-layer.md) — WebIpcBridge protocol
- [TDD-11: Web Server Mode](./11-web-server-mode.md) — Full server mode docs
- `src/platform/adapters/node/web-ipc-bridge.ts`
- `src/server/http-server.ts`

---

## Addendum v4.0: Auth Layer & Session Management (login CRs) — IMPLEMENTED ✅

> **Date:** 2026-07-24 | **Status:** Complete | **Tests:** 40 (auth) + 21 (session)

### Auth Architecture

Thêm một lớp auth riêng bên ngoài RPC server hiện tại. PairCode flow và E2EE giữ nguyên hoàn toàn.

```
HTTP :6769  (Express)
  ├── cookie-parser
  ├── AuthMiddleware        ← populates req.orcaSession from cookie
  ├── POST /auth/local      ← AuthLocalHandler → bcrypt → AuthSessionStore → Set-Cookie
  ├── POST /auth/logout     ← clear cookie + revokeSession
  ├── GET  /auth/me         ← return OrcaSessionUser if authenticated
  ├── GET  /auth/sso/:p     ← 501 (Phase 2 deferred)
  └── /admin/api/*          ← requireAdmin guard → AdminRouter

WebSocket :6768 (OrcaRuntimeRpcServer — UNCHANGED)
  └── PairCode + E2EE flow (unchanged — backward compat)
```

### Session Model

```typescript
// OrcaSession (8h TTL, HTTP-only cookie)
{
  sessionId:  string   // 64-hex (randomBytes(32))
  userId:     string
  userEmail:  string
  role:       'admin' | 'developer'
  createdAt:  number
  expiresAt:  number   // createdAt + 8h
  lastSeenAt: number | null
  ipAddress:  string | null
  userAgent:  string | null
}
```

Session được persist trong `orca_sessions` (SQLite). `AuthManager` chạy cleanup mỗi 30 phút.

### AuthManager Facade

```typescript
class AuthManager {
  readonly sessionStore: AuthSessionStore
  readonly userStore:    AuthUserStore
  readonly localHandler: AuthLocalHandler

  validateRequest(cookieHeader): Promise<OrcaSession | null>
  login(input, ip, ua): Promise<LocalLoginResult>
  logout(sessionId): Promise<void>
  destroy(): void  // clear cleanup interval on shutdown
}
```

### Per-User Process Isolation (ORCA_MULTI_USER=1)

```
ORCA_MULTI_USER=0 (default) → single process, PairCode flow (backward compat)
ORCA_MULTI_USER=1           → WsSessionRouter proxies WS per userId
  └── SessionManager.getOrSpawn(userId)
        ├── fork() user-process-entry.ts
        ├── env: ORCA_USER_ID, ORCA_SOCKET_PATH
        ├── timeout: 30s (spawn), idle: 4h (shutdown)
        └── OrcaRuntimeRpcServer (Unix socket only)
```

### Request Flow (ORCA_MULTI_USER=1)

```
Browser WS → WsSessionRouter
  ├── read req.orcaSession.userId
  ├── SessionManager.getOrSpawn(userId)
  └── proxy WS ↔ Unix socket(userData/users/<userId>/orca.sock)
```

Tham khảo:
- [TDD-11: Web Server Mode](./11-web-server-mode.md) — HTTP server + auth mounting
- `src/main/auth/auth-manager.ts`, `auth-session-store.ts`, `auth-user-store.ts`
- `src/main/session/session-manager.ts`, `ws-session-router.ts`
