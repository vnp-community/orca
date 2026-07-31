# TDD-AG-04: Handshake & Session Management

**Version:** 4.0  
**Date:** 2026-07-28  
**Source:** `deploy/dev/agent/agent.js`

---

## 1. Handshake Flow

```
Agent                                    Server
  │                                        │
  │──── WS open ──────────────────────────▶│
  │                                        │
  │ Send frame (TYPE=0x01, ch=0, seq=1):   │
  │   {                                    │
  │     type:         'handshake',         │
  │     agentToken:   process.env.AGENT_TOKEN,
  │     agentVersion: '1.0.0',            │
  │     platform:     os.platform(),       │
  │     arch:         os.arch(),           │
  │     nodeVersion:  process.version      │
  │   }                                    │
  │──────────────────────────────────────▶│
  │                                        │ Validate token (slot lookup)
  │ Recv frame:                            │
  │   {                                    │
  │     type:       'handshake-ack',       │
  │     orcaVersion: '4.x.x',             │
  │     sessionId:  'uuid-v4'             │
  │   }                                    │
  │◀──────────────────────────────────────│
  │                                        │
  │ Handshake complete ✅                   │
  │ Begin tool dispatch loop               │
```

---

## 2. Handshake Timeout

```javascript
const HANDSHAKE_TIMEOUT_MS = 10_000   // 10 seconds

const handshakeTimer = setTimeout(() => {
  console.error('[Agent] Handshake timeout — no ack received')
  ws.close(1008, 'Handshake timeout')
  process.exit(2)
}, HANDSHAKE_TIMEOUT_MS)

// On handshake-ack received:
clearTimeout(handshakeTimer)
```

---

## 3. Session Info

```javascript
// Sau khi handshake-ack:
const session = {
  sessionId:   ackData.sessionId,
  orcaVersion: ackData.orcaVersion,
  agentToken:  process.env.AGENT_TOKEN,
  connectedAt: Date.now()
}

// Log (no token):
console.log('[Agent] Session:', {
  sessionId:   session.sessionId,
  orcaVersion: session.orcaVersion,
  platform:    os.platform(),
  arch:        os.arch()
})
```

---

## 4. Keepalive

```javascript
const KEEPALIVE_INTERVAL_MS = 10_000   // 10s
const TIMEOUT_MS            = 20_000   // server timeout

function startKeepalive(ws, channelId, getLastAck) {
  const interval = setInterval(() => {
    if (ws.readyState !== WebSocket.OPEN) {
      clearInterval(interval)
      return
    }

    const payload = JSON.stringify({
      type: 'keepalive',
      ack:  getLastAck()   // Highest SEQ received from server
    })
    const frame = encodeFrame(channelId, 0, payload)  // SEQ=0 for keepalive
    ws.send(frame)

  }, KEEPALIVE_INTERVAL_MS)

  interval.unref()   // Don't prevent process exit
  return interval
}
```

**Why 10s interval with 20s timeout:**
- Keepalive interval = timeout/2 → safe margin
- If agent hangs (busy tool call) → still sends keepalive in time

---

## 5. Message Dispatch

```javascript
function handleMessage(data) {
  const frame = decodeFrame(data)

  // Skip non-Regular frames
  if (frame.type !== 0x01) return

  const msg = JSON.parse(frame.payload.toString('utf8'))

  switch (msg.type) {
    case 'handshake-ack': handleHandshakeAck(msg);   break
    case 'rpc':           handleRpc(frame, msg);     break
    case 'keepalive-ack': updateLastAck(frame.seqNo); break
    default:
      console.warn('[Agent] Unknown message type:', msg.type)
  }
}
```

---

## 6. Session Termination

```javascript
// Normal close (server initiated):
ws.on('close', (code, reason) => {
  clearInterval(keepaliveInterval)
  clearTimeout(handshakeTimer)

  console.log(`[Agent] Closed: code=${code} reason=${reason.toString()}`)

  // Exit 2 → systemd restart
  process.exit(2)
})

// Error:
ws.on('error', (err) => {
  console.error('[Agent] Error:', err.message)
  // 'close' will fire automatically
})
```

---

## 7. Session Metrics

```javascript
// Tracked in-memory (logged on disconnect):
const metrics = {
  connectedAt:  Date.now(),
  framesRecv:   0,
  framesSent:   0,
  rpcCallCount: 0,
  rpcErrors:    0
}

ws.on('close', () => {
  const uptime = Date.now() - metrics.connectedAt
  console.log('[Agent] Session metrics:', {
    uptimeMs: uptime,
    ...metrics
  })
})
```
