# TDD-AG-03: Connection Modes

**Version:** 4.0  
**Date:** 2026-07-28  
**Source:** `deploy/dev/agent/agent.js`

---

## 1. Direct WebSocket Mode (direct-ws)

### Flow

```
1. start.sh: GET /api/agent-token { devServerId, name }
   Response: { token, expiresIn: 300 }

2. agent.js: AGENT_TOKEN=<token> ORCA_URL=wss://orca-host:6769/agent

3. ws = new WebSocket(`${ORCA_URL}?token=${AGENT_TOKEN}`)
   HTTP Upgrade với header:
     Authorization: Bearer AGENT_TOKEN

4. Server side:
   AgentWebSocketServer.handleConnection(ws)
     → runOrcaReceiverHandshake(ws, tokenValidator, orcaVersion)
     → slot.callback(mux, info)
     → DevServerRelayBridge wires session

5. Session established → tool calls available
```

### Token Lifecycle

```
POST /api/agent-token → token (TTL 300s)
Agent connects → token consumed (one-time use)
Connection drop → exit(2)
systemd restart → start.sh → fresh token → reconnect
```

---

## 2. Relay WebSocket Mode (relay-ws)

### Flow

```
1. RELAY_URL=wss://relay.internal/relay
   RELAY_TOKEN=<secret>

2. ws = new WebSocket(RELAY_URL)
   Auth: { type: 'auth', token: RELAY_TOKEN }
   Server: { type: 'auth-ack' }

3. Relay proxies traffic:
   Agent ← relay → Orca Server
   (SshChannelMultiplexer over relay)

4. Session established
```

### Difference from direct-ws

| Aspect | direct-ws | relay-ws |
|--------|-----------|---------|
| Token source | POST /api/agent-token | Pre-configured |
| Server endpoint | AgentWebSocketServer /agent | Relay server |
| Firewall requirements | Port 6769 open | Only outbound to relay |
| Reconnect | exit(2) + systemd + fresh token | Reconnect with same RELAY_TOKEN |

---

## 3. Connection URL Construction

### direct-ws

```javascript
const url = new URL(process.env.ORCA_URL)
url.searchParams.set('token', process.env.AGENT_TOKEN)
// Result: wss://orca-host:6769/agent?token=<token>
```

### relay-ws

```javascript
const url = process.env.RELAY_URL  // direct, no modification
```

---

## 4. Reconnect Strategy

### direct-ws: No Reconnect (exit instead)

```javascript
ws.on('close', (code, reason) => {
  console.log(`[Agent] Disconnected (${code}): ${reason}`)
  // Token is one-time-use: MUST exit + let systemd restart
  // start.sh will GET a new token on next run
  process.exit(2)
})

ws.on('error', (err) => {
  console.error('[Agent] Error:', err.message)
  // 'close' will fire after 'error'
})
```

### relay-ws: Exponential Backoff Reconnect

```javascript
let retryDelay = 1000   // 1s initial
const maxDelay = 30_000  // 30s max

function reconnect() {
  setTimeout(() => {
    console.log('[Agent] Reconnecting...')
    connect()
  }, retryDelay)
  retryDelay = Math.min(retryDelay * 2, maxDelay)
}

ws.on('close', () => reconnect())
```

---

## 5. Connection Timeout

```javascript
const CONNECT_TIMEOUT_MS = 30_000  // 30s

const connectTimer = setTimeout(() => {
  console.error('[Agent] Connection timeout')
  ws.terminate()
  process.exit(2)
}, CONNECT_TIMEOUT_MS)

ws.on('open', () => clearTimeout(connectTimer))
```

---

## 6. TLS/WSS

```javascript
// WSS (TLS) = required cho production
// WS (plain) = dev only (localhost)

const url = process.env.ORCA_URL
if (!url.startsWith('wss://') && !url.startsWith('ws://')) {
  throw new Error('ORCA_URL must start with ws:// or wss://')
}

// Custom CA cert (self-signed):
const ws = new WebSocket(url, {
  ca: process.env.ORCA_CA_CERT ? [process.env.ORCA_CA_CERT] : undefined
})
```

---

## 7. Environment Variable Validation

```javascript
function validateEnv() {
  const mode = process.env.RELAY_URL ? 'relay-ws' : 'direct-ws'

  if (mode === 'direct-ws') {
    if (!process.env.ORCA_URL)   throw new Error('ORCA_URL is required')
    if (!process.env.AGENT_TOKEN) throw new Error('AGENT_TOKEN is required')
  } else {
    if (!process.env.RELAY_URL)  throw new Error('RELAY_URL is required')
    if (!process.env.RELAY_TOKEN) throw new Error('RELAY_TOKEN is required')
  }

  return mode
}
```
