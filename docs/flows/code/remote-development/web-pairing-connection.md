# Web Pairing & Connection Flow

> **Scope:** Luồng kết nối từ khi user nhập PairCode trên web UI (`https://b15.openledger.vn`)
> đến khi app Orca hoạt động đầy đủ qua WebSocket E2EE.
>
> **Key files:**
> - [`src/renderer/src/web/web-pairing.ts`](../../src/renderer/src/web/web-pairing.ts) — parse/decode PairCode
> - [`src/renderer/src/web/WebConnect.tsx`](../../src/renderer/src/web/WebConnect.tsx) — UI + probe
> - [`src/renderer/src/web/web-runtime-client.ts`](../../src/renderer/src/web/web-runtime-client.ts) — WS state machine + E2EE
> - [`src/main/runtime/runtime-rpc.ts`](../../src/main/runtime/runtime-rpc.ts) — server-side handler

---

## Tổng quan

```
PairCode (base64url)
    ↓  parseWebPairingInput()
WebPairingOffer { endpoint, deviceToken, publicKeyB64 }
    ↓  WebRuntimeClient constructor
WebSocket → E2EE Handshake (4 frames)
    ↓  state = 'connected'
JSON-RPC calls (encrypted) ↔ Orca Runtime
    ↓  Heartbeat (10s interval)
Full app running
```

---

## Phase 0: Parse PairCode

**File:** [`web-pairing.ts:18`](../../src/renderer/src/web/web-pairing.ts#L18)

Field "Pairing URL or code" trong [`WebConnect.tsx`](../../src/renderer/src/web/WebConnect.tsx) chấp nhận 2 dạng input:

| Dạng | Ví dụ |
|------|-------|
| `orca://pair?code=<base64url>` | Deep link từ Orca Desktop |
| Raw base64url string | Khi copy từ Settings → Share |

```typescript
// web-pairing.ts
export function parseWebPairingInput(input: string): WebPairingOffer | null {
  if (input.startsWith('orca://')) {
    const code = extractPairingCodeFromUrl(input)   // lấy ?code= param
    return decodePairingPayload(code)
  }
  return decodePairingPayload(input)                // raw base64url
}

function decodePairingPayload(base64url: string): WebPairingOffer | null {
  const json = atob(base64url)                      // base64 → JSON string
  const parsed = JSON.parse(json)
  // validate: v === 2, endpoint, deviceToken, publicKeyB64 đều phải có
  return { v: 2, endpoint, deviceToken, publicKeyB64, scope? }
}
```

**Kết quả — `WebPairingOffer`:**
```json
{
  "v": 2,
  "endpoint":     "wss://b15.openledger.vn",
  "deviceToken":  "abc123...",
  "publicKeyB64": "Xyz456...",
  "scope":        "runtime"
}
```

> ⚠️ **Không thể nhập `wss://b15.openledger.vn` trực tiếp.**
> Field này KHÔNG nhận WebSocket URL — nó nhận **PairCode** (base64 JSON bundle).
> Khi `parsedOffer = null` → Connect button bị `disabled`.

---

## Phase 1: Probe Connection (WebConnect)

**File:** [`WebConnect.tsx:35`](../../src/renderer/src/web/WebConnect.tsx#L35)

Khi user click **Connect**:

```typescript
// WebConnect.tsx
const connect = async () => {
  if (!parsedOffer) {
    setError('Enter a valid Orca pairing URL or pairing code.')
    return
  }
  if (isMixedContentWebSocket(parsedOffer.endpoint)) {
    // HTTPS page không thể connect ws:// → phải dùng wss://
    setError('This HTTPS page cannot connect to a plain ws:// ...')
    return
  }

  setConnecting(true)
  const client = new WebRuntimeClient(parsedOffer)  // → mở WS ngay (Phase 2)

  // Probe: kiểm tra server alive + scope hợp lệ
  const response = await client.call('status.get', undefined, { timeoutMs: 15_000 })

  if (response.result?.deviceScope === 'mobile') {
    setError('Mobile-only QR code — use the browser access link instead.')
    return
  }

  saveStoredWebRuntimeEnvironment({ name, offer: parsedOffer, runtimeId })
  onConnected()   // → render app
}
```

---

## Phase 2: E2EE WebSocket Handshake

**File:** [`web-runtime-client.ts:348`](../../src/renderer/src/web/web-runtime-client.ts#L348), [`runtime-rpc.ts:740`](../../src/main/runtime/runtime-rpc.ts#L740)

Handshake dùng **ECDH Curve25519** để establish shared key, sau đó **ChaCha20-Poly1305** encrypt mọi frame.

```
CLIENT (browser)                          SERVER (orca-server container)
────────────────                          ──────────────────────────────
new WebSocket("wss://b15.openledger.vn")
state = 'connecting'
        ───────────── TCP/TLS connect ────────────────────→

◀─────────────────── onopen ──────────────────────────────

[1] generateKeyPair()           // Ephemeral Curve25519 keypair
    sharedKey = ECDH(            // Derive shared secret
      mySecretKey,
      serverPublicKey             // từ publicKeyB64 trong PairCode
    )
    state = 'handshaking'

──→ { type: "e2ee_hello",                                  // Plaintext JSON
      publicKeyB64: myPublicKey }

                                          serverSharedKey = ECDH(
                                            serverSecretKey,
                                            clientPublicKey
                                          )
                                          // sharedKey client == sharedKey server ✅

◀── { type: "e2ee_ready" }                                 // Plaintext JSON

[2] sendEncrypted({
      type: "e2ee_auth",
      deviceToken: "abc123..."   // Bearer token từ PairCode
    })                           // Encrypted với sharedKey

                                          decrypt → validateToken("abc123...")
                                          → deviceRegistry.lookup() ✅
                                          → updateLastSeen(deviceId)

◀── encrypt({ type: "e2ee_authenticated" })               // Encrypted

[3] clearHandshakeTimer()
    reconnectAttempt = 0
    state = 'connected' ✅
    startHeartbeat()
```

**Timeout guards:**
- `CONNECT_TIMEOUT_MS = 12s` — TCP connect timeout
- `HANDSHAKE_TIMEOUT_MS = 10s` — handshake completion timeout

**Auth failure path:**
```
◀── encrypt({ type: "e2ee_error" | error.code: "unauthorized" })
state = 'auth-failed'
intentionallyClosed = true      // không reconnect
setError("Unauthorized. Pair this web client again.")
```

---

## Phase 3: Normal RPC Calls

**File:** [`web-runtime-client.ts:111`](../../src/renderer/src/web/web-runtime-client.ts#L111)

Sau khi connected, mọi IPC call đều đi qua encrypted WebSocket:

```
CLIENT                                    SERVER
──────                                    ──────
client.call("worktrees.list")

encrypt({
  id: "web-rpc-1-1721234567",
  deviceToken: "abc123...",      // validate trên mỗi request
  method: "worktrees.list",
  params: {}
})  ─────────────────────────────────────→

                                          decrypt → validate deviceToken
                                          dispatch → handleRuntimeRpc()
                                          → runtime.listWorktrees()

◀───────────────── encrypt({
                    id: "web-rpc-1-...",
                    ok: true,
                    result: [...worktrees]
                   })

resolve(response) → UI update
```

**Request timeout:** `REQUEST_TIMEOUT_MS = 30s`

**Subscription (streaming):**
```typescript
// Mỗi subscription mở 1 WebSocket mới (child client)
// Ngoại trừ files.watch → shared connection
client.subscribe("terminal.stream", params, {
  onResponse: (frame) => { /* terminal output */ },
  onBinary:   (bytes) => { /* raw PTY bytes   */ },
  onClose:    ()      => { /* cleanup          */ }
})
```

---

## Phase 4: Heartbeat & Reconnect

**File:** [`web-runtime-client.ts:76`](../../src/renderer/src/web/web-runtime-client.ts#L76)

```
HEARTBEAT_INTERVAL_MS   = 10s   // tick interval
HEARTBEAT_IDLE_MS       = 25s   // sau 25s không có frame → send probe
HEARTBEAT_PROBE_GRACE_MS= 20s   // nếu probe không reply sau 20s → close
```

```
every 10s tick:
  ┌─ sinceLastTick > 20s? (tab was frozen/backgrounded)
  │    → reset clocks, clear probe (not evidence of death)
  ├─ tab hidden? → skip (no battery waste)
  ├─ probe in-flight + grace expired?
  │    → ws.close() → handleSocketClosed() → scheduleReconnect()
  └─ idle > 25s + no probe in-flight?
       → sendEncrypted({ method: "status.get" })  // liveness probe
         heartbeatProbeSentAt = now
```

**Reconnect backoff:**
```
attempt: 0    1     2     3     4      5+
delay:  500ms 1000ms 2000ms 4000ms 8000ms 15000ms
```

---

## Phase 5: Tạo PairCode (server-side)

**File:** [`runtime-rpc.ts:511`](../../src/main/runtime/runtime-rpc.ts#L511)

PairCode **phải được generate bởi server**, không thể tự tạo:

```typescript
// OrcaRuntimeRpcServer.createPairingOffer()
createPairingOffer({ address?, name?, rotate?, scope? }) {
  const endpoint   = resolvePairingEndpoint(wsEndpoint, address)
  // ORCA_DOMAIN env var: override hostname cho reverse proxy
  // → "wss://b15.openledger.vn" khi ORCA_DOMAIN=b15.openledger.vn

  const device     = deviceRegistry.getOrCreatePendingDevice(name, scope)
  const pairingUrl = encodePairingOffer({
    v: 2,
    endpoint,
    deviceToken:  device.token,    // unique Bearer token
    publicKeyB64: e2eeKeypair.publicKeyB64,
    scope                          // 'runtime' | 'mobile'
  })
  const webClientUrl = `https://b15.openledger.vn/#pairing=${encodeURIComponent(pairingUrl)}`
  return { pairingUrl, webClientUrl, endpoint, deviceId }
}
```

**Cách lấy PairCode trong thực tế:**

```
Orca Desktop → Settings → Runtime Environments
  → Share this Orca server → New Link
  → Copy URL: https://b15.openledger.vn/#pairing=eyJ2Ij...
                                                  ↑ PairCode ở đây
```

Hoặc mở trực tiếp URL đó trong browser → web UI tự động parse `#pairing=...` từ hash → auto-connect.

---

## Security Model

| Thành phần | Mô tả |
|------------|-------|
| `publicKeyB64` | Server's Curve25519 public key — embedded trong PairCode, dùng để verify server identity |
| `deviceToken` | UUID bearer token — define trong `deviceRegistry`, validate trên mỗi message |
| `sharedKey` | `ECDH(ephemeralClientSecret, serverPublic)` — unique mỗi session, không reuse |
| Encryption | ChaCha20-Poly1305 — authenticated encryption, tamper-proof |
| Replay protection | Ephemeral keypair mỗi lần connect → different `sharedKey` → old frames invalid |

> **PairCode không phải password.** Nó là credential bundle: biết PairCode = biết endpoint + có token hợp lệ.
> Token bị revoke ngay khi: `rotate=true`, device bị remove, hoặc server restart clear deviceRegistry.

---

## Error States

| State | Nguyên nhân | Hành động |
|-------|-------------|-----------|
| Button disabled | `parsedOffer = null` — input không phải valid base64 JSON | Nhập đúng PairCode |
| Mixed content error | `https://` page cố connect `ws://` | Dùng `wss://` endpoint |
| Connection timeout (12s) | Server không reachable | Kiểm tra network/firewall |
| Handshake timeout (10s) | Server reachable nhưng không respond đúng | Kiểm tra server logs |
| `unauthorized` | `deviceToken` invalid/expired | Generate PairCode mới |
| `mobile scope` | QR code mobile-only | Dùng "Browser access link" thay vì QR |
