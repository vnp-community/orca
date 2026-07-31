# TDD-FE-06: Web Client Mode

**Document:** TDD-FE-06  
**Domain:** Web Client — Pairing, WebConnect, web-preload-api  
**Source files:** `src/renderer/src/web/`

---

## 1. Tổng quan

Orca có **Web Client mode**: chạy trong browser thông thường, kết nối tới `orca serve` server qua WebSocket + E2EE.

```
Browser truy cập https://your-server/
  ↓
web-index.html
  ↓
web/main.tsx
  ├─ installWebPreloadApi()   ← tạo window.api proxy
  └─ Mount React app:
      ├─ Nếu chưa paired → <WebConnect />
      └─ Nếu đã paired → <App /> (full Orca UI)
```

---

## 2. Web Entry Point (`web/main.tsx`)

```typescript
// src/renderer/src/web/main.tsx (~3KB)

import { installWebPreloadApi } from './web-preload-api'

// Bước 1: setup window.api proxy
installWebPreloadApi()

// Bước 2: detect nếu đã có saved environment
const savedEnv = readStoredWebRuntimeEnvironment()
const hasPairingInput = new URLSearchParams(location.search).get('pair')

// Bước 3: render
if (savedEnv || hasPairingInput) {
  renderApp()   // → App component
} else {
  renderWebConnect()   // → WebConnect + pairing UI
}

function renderApp(): void {
  createRoot(document.getElementById('root')).render(
    <StrictMode>
      <I18nProvider>
        <RecoverableRenderErrorBoundary ...>
          <App />
        </RecoverableRenderErrorBoundary>
      </I18nProvider>
    </StrictMode>
  )
}
```

---

## 3. WebConnect UI (`WebConnect.tsx`)

```typescript
// src/renderer/src/web/WebConnect.tsx

// UI: form nhập pairing URL
// Validate → connect → probe status.get → save environment

// Pairing URL formats:
// - orca://pair?code=<base64-offer>   (QR code from desktop)
// - https://domain/web-index.html?pair=<code>  (web link)
// - wss://server:6768?token=<token>   (direct WebSocket)

// Scope check:
// - 'web' scope → allowed (full access)
// - 'mobile' scope → rejected với error message

// Success path:
// 1. Parse offer → extract endpoint + publicKey
// 2. WebRuntimeClient.call('status.get')
// 3. Check deviceScope === 'mobile' → reject
// 4. saveStoredWebRuntimeEnvironment()
// 5. onConnected() → navigate to full app
```

---

## 4. Web Pairing (`web-pairing.ts`)

```typescript
// src/renderer/src/web/web-pairing.ts (~5KB)
// Parse pairing input → PairingOffer

function parseWebPairingInput(input: string): PairingOffer | null {
  // Hỗ trợ formats:
  // 1. orca://pair?code=<base64url>
  //    → decode base64url → { endpoint: string, publicKey: Uint8Array }
  // 2. https://...?pair=<base64url>
  //    → same decode
  // 3. Raw WebSocket URL: wss://host:6768?token=xxx
  //    → { endpoint: url, noPairing: true }
  // 4. Short code: "ABCD-1234"
  //    → lookup via HTTP (not yet implemented)
}

type PairingOffer = {
  endpoint: string          // WebSocket URL
  publicKey?: Uint8Array    // Server E2EE public key (Curve25519)
  noPairing?: boolean       // True for direct token URLs
  scope?: 'web' | 'mobile'
}

function isMixedContentWebSocket(endpoint: string): boolean {
  // HTTPS page cannot use ws:// (non-TLS) WebSocket
  return location.protocol === 'https:' && endpoint.startsWith('ws://')
}
```

---

## 5. Web E2EE (`web-e2ee.ts`)

```typescript
// src/renderer/src/web/web-e2ee.ts (~3KB)
// Client-side E2EE key exchange

import * as nacl from 'tweetnacl'

function performWebE2EEPairing(
  serverPublicKey: Uint8Array
): { clientKeypair: nacl.BoxKeyPair; sharedKey: Uint8Array } {
  // 1. Generate ephemeral client keypair
  const clientKeypair = nacl.box.keyPair()

  // 2. Compute shared secret
  const sharedKey = nacl.box.before(serverPublicKey, clientKeypair.secretKey)

  return { clientKeypair, sharedKey }
}

function encryptWebMessage(plaintext: Uint8Array, sharedKey: Uint8Array): Uint8Array
function decryptWebMessage(ciphertext: Uint8Array, sharedKey: Uint8Array): Uint8Array | null
```

---

## 6. Web Runtime Client (`web-runtime-client.ts`)

~27KB — WebSocket client cho browser-to-server communication:

```typescript
// src/renderer/src/web/web-runtime-client.ts

class WebRuntimeClient {
  private ws: WebSocket
  private sharedKey: Uint8Array | null   // E2EE session key
  private pendingRequests: Map<string, RequestState>

  constructor(offer: PairingOffer) {
    this.ws = new WebSocket(offer.endpoint)
  }

  async connect(): Promise<void> {
    await this.waitForOpen()

    if (offer.publicKey) {
      // E2EE pairing handshake
      const { clientKeypair, sharedKey } = performWebE2EEPairing(offer.publicKey)
      this.sharedKey = sharedKey

      // Send client public key as first message
      await this.sendRaw({ type: 'pair', clientPublicKey: toBase64(clientKeypair.publicKey) })

      // Server responds with confirmation
      await this.waitForPairAck()
    } else {
      // No E2EE (direct token URL)
      await this.sendRaw({ type: 'auth', token: extractToken(offer.endpoint) })
    }
  }

  async call<T>(method: string, params?: unknown, options?: { timeoutMs?: number }): Promise<T> {
    const id = generateRequestId()
    const request = { id, method, params }

    // Encrypt nếu có E2EE
    const frame = this.sharedKey
      ? encryptWebMessage(JSON.stringify(request), this.sharedKey)
      : JSON.stringify(request)

    this.ws.send(frame)

    // Wait for response
    return this.waitForResponse<T>(id, options?.timeoutMs)
  }

  close(): void {
    this.ws.close()
  }
}
```

---

## 7. web-preload-api (`web-preload-api.ts`)

~135KB — **Tương đương hoàn toàn với Electron preload** nhưng chạy trong browser:

```typescript
// src/renderer/src/web/web-preload-api.ts
// ~135KB — mirrors every IPC handler từ src/main/ipc/

export function installWebPreloadApi(): void {
  // Tạo window.api với cùng interface như Electron preload:
  ;(window as any).api = {
    filesystem: {
      readFile: (path) => callRuntimeRpc('local', 'file.read', { path }),
      writeFile: (path, data) => callRuntimeRpc('local', 'file.write', { path, data }),
      listDir: (path, opts) => callRuntimeRpc('local', 'file.list', { path, ...opts }),
      search: (args) => callRuntimeRpc('local', 'file.search', args),
      watch: (path) => callRuntimeRpc('local', 'file.watch', { path }),
      unwatch: (watcherId) => callRuntimeRpc('local', 'file.unwatch', { watcherId }),
    },
    pty: {
      create: (args) => callRuntimeRpc('local', 'terminal.create', args),
      write: (ptyId, data) => callRuntimeRpc('local', 'terminal.write', { ptyId, data }),
      resize: (ptyId, cols, rows) => callRuntimeRpc('local', 'terminal.resize', { ptyId, cols, rows }),
      kill: (ptyId, signal) => callRuntimeRpc('local', 'terminal.kill', { ptyId, signal }),
      subscribe: (ptyId, onData) => { /* WebSocket stream subscription */ },
    },
    ssh: {
      listTargets: () => callRuntimeRpc('local', 'ssh.list', {}),
      connect: (targetId) => callRuntimeRpc('local', 'ssh.connect', { targetId }),
      // ...
    },
    worktrees: { ... },
    repos: { ... },
    settings: { ... },
    runtimeEnvironments: {
      call: ({ selector, method, params }) =>
        callRuntimeRpc({ kind: 'environment', environmentId: selector }, method, params)
    },
    // ...
  }
}
```

---

## 8. Web Runtime Environment (`web-runtime-environment.ts`)

```typescript
// src/renderer/src/web/web-runtime-environment.ts
// Quản lý saved server connections trong localStorage

type StoredWebRuntimeEnvironment = {
  id: string
  name: string                    // "Orca Server" (user-set)
  endpoint: string                // wss://server:6768
  publicKey: string               // base64 Curve25519 public key
  lastUsedAt: number
  runtimeId?: string              // Server identity (từ status.get)
}

function readStoredWebRuntimeEnvironment(): StoredWebRuntimeEnvironment | null
function saveStoredWebRuntimeEnvironment(env: StoredWebRuntimeEnvironment): void
function clearStoredWebRuntimeEnvironment(): void

// Lưu trong: localStorage['orca-web-runtime-environment']
```

---

## 9. Web Session Close/Reorder/Focus

```typescript
// web-session-close-intent.ts   — track close intent (pending confirmation)
// web-session-focus-intent.ts   — track focus intent (which tab user wanted)
// web-session-reorder-intent.ts — track reorder intent (drag-and-drop)

// Tại sao cần "intent" pattern?
// Web client gửi intent tới server, server xử lý và push state update.
// Frontend optimistically show UI nhưng không commit cho đến khi server confirm.
// Nếu server reject → rollback.
```

---

## 10. Web Workspace Session (`web-workspace-session.ts`)

```typescript
// src/renderer/src/web/web-workspace-session.ts (~800 bytes)
// Đơn giản: persist + restore active worktreeId trong sessionStorage

function readWebWorkspaceSession(): { activeWorktreeId?: string }
function saveWebWorkspaceSession(session: { activeWorktreeId?: string }): void
```

---

## 11. Web Client Heartbeat

```typescript
// web-runtime-client-heartbeat.test.ts
// WebRuntimeClient gửi heartbeat mỗi 30s để keep WebSocket alive
// Nếu không có heartbeat ack trong 60s → disconnect + reconnect attempt
```

---

## 12. Sự khác biệt Desktop vs Web

| Feature | Desktop (Electron) | Web (Browser) |
|---------|-------------------|---------------|
| `window.api` | Electron preload | `web-preload-api.ts` |
| PTY transport | `LocalPtyTransport` (IPC) | `RemoteRuntimePtyTransport` (WS) |
| Pairing | Không cần (same process) | `WebConnect.tsx` + E2EE |
| Auth | Electron IPC (implicit) | Auth token + E2EE |
| File system | Direct IPC | Via RPC |
| Notifications | Native OS notifications | Browser notifications |
| Updates | `electron-updater` | Không có auto-update |
| Offline | Có (Electron cache) | Không (requires WS) |
| Multiple windows | Có (Electron multi-window) | Một tab per session |

---

## 13. IRpcClient Interface & WebSocketRpcClient — restructure_v1

Từ restructure_v1, có thêm lớp abstraction nhẹ hơn `WebRuntimeClient` (E2EE) cho việc track connection status:

```typescript
// src/platform/rpc-client-interface.ts [MỚI]
export interface IRpcClient {
  connect(): Promise<void>
  disconnect(): void
  isConnected(): boolean
  invoke(channel: string, ...args: unknown[]): Promise<unknown>
  send(channel: string, data?: unknown): void
  on(channel: string, handler: (...args: unknown[]) => void): () => void
  off(channel: string, handler: (...args: unknown[]) => void): void
  once(channel: string, handler: (...args: unknown[]) => void): void
}
```

```typescript
// src/platform/adapters/web/rpc-client.ts [MỚI]
// JSON-RPC over WebSocket — lightweight, không E2EE
// Dùng cho: ConnectionStatusProvider polling + bootstrapWebApp() initial connect

export class WebSocketRpcClient implements IRpcClient {
  constructor(url?: string)   // auto-detect từ window.location nếu không có url

  // Wire format:
  // invoke → { type: 'invoke', id, channel, args }
  // push   ← { type: 'push', channel, args }
  // result ← { type: 'result', id, result }
  // error  ← { type: 'error', id, message }
  // send   → { type: 'send', channel, data }
}
```

### So sánh WebRuntimeClient vs WebSocketRpcClient

| Feature | `WebRuntimeClient` | `WebSocketRpcClient` |
|---------|-------------------|---------------------|
| E2EE (NaCl box) | ✅ | ❌ (plain JSON) |
| Pairing handshake | ✅ | ❌ |
| Target | Full app RPC | Connection status polling |
| Size | ~27KB | ~5KB |
| Location | `src/renderer/src/web/` | `src/platform/adapters/web/` |
| Tests | — | 15/15 ✅ |

---

## 14. ConnectionStatusProvider & Banner — restructure_v1

```typescript
// src/renderer/src/web/ConnectionStatusProvider.tsx [MỚI]

export type ConnectionStatus = 'connecting' | 'connected' | 'disconnected' | 'error'

// Component
export function ConnectionStatusProvider({
  children,
  client,          // IRpcClient — dùng để poll isConnected()
  pollIntervalMs   // default: 2000ms
}: ConnectionStatusProviderProps): JSX.Element

// Hooks
export function useConnectionStatus(): ConnectionStatus
export function useConnectionClient(): IRpcClient | null
export function useConnectionRetry(): () => void   // trigger reconnect
```

```typescript
// src/renderer/src/web/ConnectionStatusBanner.tsx [MỚI]
// Fixed-position bottom-right banner
// Visible khi: status === 'disconnected' | 'connecting'
// Props: status, onRetry

// Accessibility: role="alert" aria-live="polite"
// States:
// - disconnected: ⚠ "Connection lost" + Retry button (red bg)
// - connecting:   ↻ "Connecting..." (amber bg, spinner)
// - connected:    null (không render)
```

### Test coverage

```bash
# 11/11 tests pass
vitest run --config config/vitest.config.ts \
  "src/renderer/src/web/__tests__/ConnectionStatusProvider.test.tsx" \
  "src/renderer/src/web/__tests__/ConnectionStatusBanner.test.tsx"
```

---

## Addendum — login CRs: Auth-Aware Web Entry (v4.0)

### Updated `bootstrapWebApp()` flow (v4.0)

```
bootstrapWebApp()
  ├── registerServiceWorker()            ← đã có từ v3.0
  ├── installWebPreloadApi()             ← đã có
  │
  ├── [NEW v4.0] checkAuthSession()
  │     ├── GET /auth/me (fetchCurrentUser)
  │     └── GET /auth/config (fetchAuthConfig) — SSO providers
  │
  └── Route based on auth + pairing state:
        ├── currentUser → renderApp({ authUser })         ← authenticated
        ├── savedEnv || hasPairingInput → renderApp({})   ← backward compat
        └── else → renderLoginPage({ authConfig })        ← show Login page
```

### `renderLoginPage()` (NEW v4.0)

```typescript
function renderLoginPage(props: { authConfig: { providers: string[]; localEnabled: boolean } }) {
  // Mount LoginPage vào #root
  // onLoginSuccess → window.location.href = '/'
  // bootstrapWebApp() sẽ detect authenticated session và vào App
}
```

### Auth-aware Web Routing sơ đồ

```
web-index.html / admin-index.html
         │
         ├─ /         → web entry → bootstrapWebApp()
         │                ├─ authenticated → App.tsx
         │                ├─ unauthenticated (no pairing) → LoginPage
         │                └─ unauthenticated (has pairing) → WebConnect (compat)
         │
         └─ /admin/  → admin-index.html → AdminApp (React Router SPA)
                          └─ guarded by session cookie (backend 401 redirect)
```

### Admin SPA entry points

```typescript
// src/renderer/admin-index.html       ← HTML entry
// src/renderer/src/admin/admin-main.tsx ← TSX entry → mounts AdminApp
// src/renderer/src/components/admin/AdminApp.tsx ← SPA root

// Build config — cả 3 vite configs đều thêm:
rollupOptions: {
  input: {
    'main':         resolve('src/renderer/web-index.html'),   // hoặc web-index
    'admin-index':  resolve('src/renderer/admin-index.html')  // NEW
  }
}
```

### auth-api-client.ts (NEW v4.0)

```typescript
// src/renderer/src/auth/auth-api-client.ts

export async function fetchCurrentUser(): Promise<AuthUser | null>
// GET /auth/me — returns null on 401

export async function loginLocal(email: string, password: string): Promise<AuthUser>
// POST /auth/local — throws AuthError on 401

export async function logoutUser(): Promise<void>
// POST /auth/logout

export async function fetchAuthConfig(): Promise<{ providers: string[]; localEnabled: boolean }>
// GET /auth/config — list enabled SSO providers
```

### auth-utils.ts (NEW v4.0 — CR-LOGIN-003)

```typescript
// src/renderer/src/auth/auth-utils.ts

// Mirror của backend toLinuxUsername() trong src/main/ssh/ssh-user-resolver.ts
export function toLinuxUsername(email: string): string
// 'alice@company.com' → 'orca-alice'
// 'alice.smith@co.com' → 'orca-alice-smith'
// Truncate local part tại 20 chars
// Sanitize: lowercase, replace non-alphanumeric với '-', strip leading/trailing dashes
```

### Test coverage (v4.0 — 104/104 tests ✅)

```bash
# Run tất cả login CR tests
npx vitest run \
  src/renderer/src/auth/__tests__/ \
  src/renderer/src/web/login/__tests__/ \
  src/renderer/src/components/auth/__tests__/ \
  src/renderer/src/components/admin/__tests__/ \
  src/renderer/src/components/ssh/__tests__/ \
  src/renderer/src/hooks/__tests__/useAuthSession.test.ts \
  src/renderer/src/hooks/__tests__/useLogout.test.ts \
  src/renderer/src/hooks/__tests__/useSshUserAccount.test.ts
```
