# TDD-AG-02: Binary Wire Protocol

**Version:** 4.0  
**Date:** 2026-07-28  
**Source:** `deploy/dev/agent/agent.js`

---

## 1. Frame Format (13-byte header)

```
Offset  Size  Field       Description
------  ----  ---------   -----------
0       1     TYPE        0x01=Regular, 0x02=Keepalive, 0x03=Control
1       4     CHANNEL_ID  u32 LE — multiplexed channel identifier
5       4     SEQ         u32 LE — monotonic sequence number
9       4     LENGTH      u32 LE — payload length in bytes
13+     N     PAYLOAD     N bytes of JSON-encoded data
```

**Total header size:** 13 bytes  
**Max payload:** 16MB (u32 max)

---

## 2. Frame Types

| TYPE | Value | Usage |
|------|-------|-------|
| Regular | `0x01` | All normal data frames (handshake, JSON-RPC) |
| Keepalive | `0x02` | ~~Deprecated~~ — server ignores non-Regular |
| Control | `0x03` | ~~Deprecated~~ — server ignores non-Regular |

> **CRITICAL:** Server SILENTLY IGNORES frames where TYPE != 0x01.  
> This causes handshake timeout if agent sends wrong frame type.  
> **ALWAYS use TYPE=0x01 for ALL frames including keepalive.**

---

## 3. Frame Encoding

```javascript
function encodeFrame(channelId, seqNo, payload) {
  const payloadBuf = Buffer.from(payload)
  const frame = Buffer.allocUnsafe(13 + payloadBuf.length)

  frame[0] = 0x01                                    // TYPE = Regular
  frame.writeUInt32LE(channelId,        1)           // CHANNEL_ID
  frame.writeUInt32LE(seqNo,            5)           // SEQ
  frame.writeUInt32LE(payloadBuf.length, 9)          // LENGTH
  payloadBuf.copy(frame, 13)                          // PAYLOAD

  return frame
}
```

---

## 4. Frame Decoding

```javascript
function decodeFrame(data) {
  if (data.length < 13) throw new Error('Frame too short')

  const type      = data[0]
  const channelId = data.readUInt32LE(1)
  const seqNo     = data.readUInt32LE(5)
  const length    = data.readUInt32LE(9)

  if (data.length < 13 + length) throw new Error('Incomplete frame')

  const payload = data.slice(13, 13 + length)
  return { type, channelId, seqNo, length, payload }
}
```

---

## 5. SEQ/ACK Tracking

```javascript
// Agent tracks:
let outgoingSeq = 0   // Increment for each frame sent
let lastAckSeq  = 0   // Track last ACK received from server

// On send:
outgoingSeq++
const frame = encodeFrame(channelId, outgoingSeq, payload)

// On receive:
const { seqNo } = decodeFrame(data)
lastAckSeq = Math.max(lastAckSeq, seqNo)

// ACK in keepalive payload:
// { type: 'keepalive', ack: lastAckSeq }
```

**Why ACK tracking:**
- `SshChannelMultiplexer` has 20s timeout per channel
- If server doesn't receive ACK within 20s → close channel
- Keepalive every 10s with ACK prevents timeout

---

## 6. Keepalive Frames

```javascript
// Gửi mỗi 10 giây — TYPE=0x01 (Regular), không phải 0x02
const keepaliveInterval = setInterval(() => {
  const payload = JSON.stringify({ type: 'keepalive', ack: lastAckSeq })
  const frame   = encodeFrame(KEEPALIVE_CHANNEL_ID, 0, payload)
  ws.send(frame)
}, 10_000)

// Unref so timer doesn't prevent process exit
keepaliveInterval.unref()
```

---

## 7. WebSocket Binary Mode

```javascript
// Agent sends/receives binary WebSocket messages (not text)
ws.on('message', (data, isBinary) => {
  if (!isBinary) {
    console.warn('[Agent] Unexpected text frame — ignoring')
    return
  }
  const frame = decodeFrame(Buffer.from(data))
  handleFrame(frame)
})

// Send:
ws.send(frameBuffer)   // Buffer → binary WebSocket message
```

---

## 8. Channel ID Convention

| Channel ID | Usage |
|-----------|-------|
| 0 | Handshake channel |
| 1 | Keepalive channel |
| 2-N | Per-RPC-call channels (auto-incremented) |

---

## 9. Frame Size Limits

- Minimum valid frame: 13 bytes (empty payload)
- Maximum recommended payload: 1MB (larger → split into chunks)
- Streaming output: each chunk sent as separate frame
