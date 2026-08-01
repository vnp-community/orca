# CR-AG-001 — Agent Wire Protocol Specification

**CR-ID:** CR-AG-001  
**Ngày:** 2026-07-26  
**Priority:** 🔴 Critical  
**Effort:** Medium (2–3 ngày)  
**Status:** ✅ Implemented (2026-07-26)  

**Implementation Note:** Wire protocol 13-byte header implemented trong agent-wire-protocol.ts. Tests: 100% pass.  
**Backend:** SOL-AG-001 IMPLEMENTED (TASK-001, TASK-002)  
**Frontend:** N/A (shared types)  
**Depends on:** —  
**Blocks:** CR-AG-002, CR-AG-003, CR-AG-004  

---

## 1. Vấn đề

Orca hiện giao tiếp với relay qua protocol nội bộ TypeScript (`relay-protocol.ts`). Protocol này chưa được document theo chuẩn ngôn ngữ-agnostic, khiến agent bên ngoài (Python, Go, Java, Rust...) không thể implement được.

---

## 2. Mục tiêu

Định nghĩa **Agent Wire Protocol v1** — một spec đầy đủ, ngôn ngữ-agnostic cho phép bất kỳ agent nào implement để nói chuyện với Orca qua WebSocket.

---

## 3. Thiết kế Protocol

### 3.1 Transport Layer: WebSocket

- **Protocol**: WebSocket (RFC 6455) over TCP
- **Message encoding**: Binary frames (không phải text frames)
- **Each WebSocket message** = 1 framed Orca message (không split/merge)

### 3.2 Frame Format

Mỗi binary WebSocket message là một **Orca Frame**:

```
┌─────────────────────────────────────────────────┐
│                  ORCA FRAME                       │
├───────┬──────────┬──────────┬────────────────────┤
│  [0]  │  [1–4]   │  [5–8]   │      [9–12]        │
│ TYPE  │ SEQ (u32)│ ACK (u32)│   LENGTH (u32 BE)  │
├───────┴──────────┴──────────┴────────────────────┤
│              PAYLOAD (LENGTH bytes)               │
│           UTF-8 encoded JSON-RPC 2.0             │
└───────────────────────────────────────────────────┘
Total header: 13 bytes
```

#### TYPE values

| Value | Name | Meaning |
|-------|------|---------|
| `0x01` | `Regular` | JSON-RPC payload (request / response / notification) |
| `0x09` | `KeepAlive` | Liveness probe. Payload is empty (LENGTH=0). |

#### SEQ / ACK fields

- **SEQ**: Monotonically increasing sequence number of this frame (starts at 1, uint32 big-endian)
- **ACK**: Highest SEQ received from the peer so far (uint32 big-endian)
- Both sides track the highest received SEQ and echo it as ACK in the next outgoing frame

### 3.3 JSON-RPC 2.0 Messages

Payload của `Regular` frames là JSON-RPC 2.0:

#### Request (Orca → Agent hoặc Agent → Orca)
```json
{
  "jsonrpc": "2.0",
  "id": 42,
  "method": "preflight.check",
  "params": { "path": "/home/user/code" }
}
```

#### Response
```json
{
  "jsonrpc": "2.0",
  "id": 42,
  "result": { "ok": true, "platform": "linux" }
}
```

#### Error Response
```json
{
  "jsonrpc": "2.0",
  "id": 42,
  "error": {
    "code": -32601,
    "message": "Method not found: unknown.method"
  }
}
```

#### Notification (no `id`, no response expected)
```json
{
  "jsonrpc": "2.0",
  "method": "fs.changed",
  "params": { "path": "/home/user/code/main.go", "type": "modified" }
}
```

### 3.4 Keepalive & Timeout

- **Sender**: Gửi KeepAlive frame mỗi **5000ms** nếu không có Regular frame nào gửi đi
- **Receiver**: Nếu không nhận bất kỳ frame nào (Regular hoặc KeepAlive) trong **20000ms** → coi connection là dead → đóng WebSocket
- Cả hai phía đều phải implement keepalive

### 3.5 Handshake

Ngay sau khi WebSocket connection được thiết lập, **agent phải gửi handshake request đầu tiên**:

#### Agent → Orca (sau khi WS connected)
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "agent.handshake",
  "params": {
    "agentVersion": "1.0.0",
    "platform": "linux",
    "arch": "arm64",
    "nodeVersion": "v20.11.0",
    "capabilities": ["pty", "fs", "git", "preflight"]
  }
}
```

#### Orca → Agent (response)
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "ok": true,
    "orcaVersion": "1.4.138",
    "sessionId": "sess-uuid-here"
  }
}
```

#### Handshake failure (version mismatch, auth fail...)
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -33100,
    "message": "Version mismatch: agent=0.9.0 is too old, minimum=1.0.0"
  }
}
```

> Agent phải **đóng WebSocket** ngay sau khi nhận error response cho handshake.

### 3.6 Authentication

#### Mode: `relay-websocket` (Orca kết nối vào agent)
Agent WS server phải check **Bearer token** trong HTTP Upgrade header:

```
GET /orca-relay HTTP/1.1
Host: agent.local:6799
Upgrade: websocket
Authorization: Bearer <secret-token>
```

Secret token được cấu hình trước (qua config file hoặc env var trên agent side, và Orca biết qua `wsUrl` field format: `ws://host:port?token=<secret>`).

#### Mode: `direct-websocket` (Agent kết nối vào Orca)
Agent gửi token trong `agent.handshake` params:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "agent.handshake",
  "params": {
    "agentToken": "agt-<uuid>",
    ...
  }
}
```

Orca validate token trước khi accept.

### 3.7 Error Codes

| Code | Name | Meaning |
|------|------|---------|
| `-32700` | `ParseError` | Invalid JSON |
| `-32600` | `InvalidRequest` | Not a valid JSON-RPC request |
| `-32601` | `MethodNotFound` | Method does not exist |
| `-32602` | `InvalidParams` | Invalid method parameters |
| `-32000` | `ServerError` | Generic server error |
| `-33001` | `CommandNotFound` | Shell command not found on remote |
| `-33002` | `PermissionDenied` | Filesystem permission denied |
| `-33003` | `PathNotFound` | Path does not exist |
| `-33004` | `PtyAllocationFailed` | Cannot allocate PTY |
| `-33005` | `DiskFull` | Disk full |
| `-33006` | `TooManyStreams` | Max concurrent file streams exceeded |
| `-33007` | `StreamProtocolError` | File stream protocol error |
| `-33100` | `HandshakeFailed` | Handshake rejected (version, auth, etc.) |
| `-33101` | `AuthFailed` | Invalid or missing auth token |

---

## 4. Pseudo-code triển khai agent

### TypeScript
```typescript
import WebSocket from 'ws'

const ws = new WebSocket('ws://orca-server:6768/agent', {
  headers: { 'Authorization': `Bearer ${token}` }
})

ws.on('open', () => {
  // Send handshake as first message
  sendFrame(ws, 1, 0, {
    jsonrpc: '2.0', id: 1,
    method: 'agent.handshake',
    params: { agentVersion: '1.0.0', platform: 'linux', arch: 'arm64', capabilities: ['fs', 'git'] }
  })
})

ws.on('message', (data: Buffer) => {
  const frame = decodeFrame(data)
  if (frame.type === 0x01) {
    const msg = JSON.parse(frame.payload.toString('utf-8'))
    handleMessage(msg)
  }
})

function encodeFrame(type: number, seq: number, ack: number, payload: object): Buffer {
  const json = Buffer.from(JSON.stringify(payload), 'utf-8')
  const header = Buffer.alloc(13)
  header[0] = type
  header.writeUInt32BE(seq, 1)
  header.writeUInt32BE(ack, 5)
  header.writeUInt32BE(json.length, 9)
  return Buffer.concat([header, json])
}
```

### Python
```python
import asyncio, json, struct
import websockets

async def connect_agent(url: str, token: str):
    headers = {'Authorization': f'Bearer {token}'}
    async with websockets.connect(url, extra_headers=headers) as ws:
        seq, ack = 1, 0
        # Handshake
        frame = encode_frame(0x01, seq, ack, {
            'jsonrpc': '2.0', 'id': 1,
            'method': 'agent.handshake',
            'params': {'agentVersion': '1.0.0', 'platform': 'linux', 'arch': 'x64', 'capabilities': []}
        })
        await ws.send(frame)
        seq += 1
        # Receive loop
        async for message in ws:
            frame_type, frame_seq, frame_ack, payload = decode_frame(message)
            if frame_type == 0x01:
                msg = json.loads(payload)
                handle_message(msg)

def encode_frame(msg_type: int, seq: int, ack: int, payload: dict) -> bytes:
    body = json.dumps(payload).encode('utf-8')
    header = struct.pack('>BIII', msg_type, seq, ack, len(body))  # 13 bytes
    return header + body

def decode_frame(data: bytes) -> tuple:
    msg_type, seq, ack, length = struct.unpack('>BIII', data[:13])
    payload = data[13:13 + length]
    return msg_type, seq, ack, payload
```

### Go
```go
package agent

import (
    "encoding/binary"
    "encoding/json"
    "github.com/gorilla/websocket"
)

func encodeFrame(msgType byte, seq, ack uint32, payload interface{}) ([]byte, error) {
    body, err := json.Marshal(payload)
    if err != nil { return nil, err }
    frame := make([]byte, 13+len(body))
    frame[0] = msgType
    binary.BigEndian.PutUint32(frame[1:5], seq)
    binary.BigEndian.PutUint32(frame[5:9], ack)
    binary.BigEndian.PutUint32(frame[9:13], uint32(len(body)))
    copy(frame[13:], body)
    return frame, nil
}

func decodeFrame(data []byte) (msgType byte, seq, ack uint32, payload []byte) {
    msgType = data[0]
    seq = binary.BigEndian.Uint32(data[1:5])
    ack = binary.BigEndian.Uint32(data[5:9])
    length := binary.BigEndian.Uint32(data[9:13])
    payload = data[13 : 13+length]
    return
}
```

### Java
```java
import java.nio.ByteBuffer;
import java.nio.ByteOrder;
import com.fasterxml.jackson.databind.ObjectMapper;

public class OrcaFrameCodec {
    private static final ObjectMapper mapper = new ObjectMapper();

    public static byte[] encodeFrame(int type, int seq, int ack, Object payload) throws Exception {
        byte[] body = mapper.writeValueAsBytes(payload);
        ByteBuffer buf = ByteBuffer.allocate(13 + body.length).order(ByteOrder.BIG_ENDIAN);
        buf.put((byte) type);
        buf.putInt(seq);
        buf.putInt(ack);
        buf.putInt(body.length);
        buf.put(body);
        return buf.array();
    }

    public static DecodedFrame decodeFrame(byte[] data) {
        ByteBuffer buf = ByteBuffer.wrap(data).order(ByteOrder.BIG_ENDIAN);
        int type = buf.get() & 0xFF;
        int seq = buf.getInt();
        int ack = buf.getInt();
        int length = buf.getInt();
        byte[] payload = new byte[length];
        buf.get(payload);
        return new DecodedFrame(type, seq, ack, payload);
    }
}
```

---

## 5. File cần tạo

### [NEW] `docs/specs/agent-wire-protocol-v1.md`

File spec chính thức, bản tiếng Anh, bao gồm:
- Frame format (byte layout với diagram)
- JSON-RPC message types
- Handshake flow
- Auth mechanism (cả 2 mode)
- Error codes
- Keepalive contract
- Pseudo-code cho TypeScript, Python, Go, Java

### [NEW] `src/shared/agent-wire-protocol.ts`

TypeScript constants shared giữa Orca server và relay:

```typescript
// src/shared/agent-wire-protocol.ts
export const AGENT_PROTOCOL_VERSION = '1'
export const AGENT_HANDSHAKE_METHOD = 'agent.handshake'
export const AGENT_KEEPALIVE_INTERVAL_MS = 5_000
export const AGENT_TIMEOUT_MS = 20_000

export const AgentErrorCode = {
  HandshakeFailed: -33100,
  AuthFailed: -33101,
} as const

export type AgentCapability = 'pty' | 'fs' | 'git' | 'preflight'

export type AgentHandshakeParams = {
  agentVersion: string
  platform: string
  arch: string
  nodeVersion?: string
  agentToken?: string          // for direct-websocket mode
  capabilities: AgentCapability[]
}

export type AgentHandshakeResult = {
  ok: true
  orcaVersion: string
  sessionId: string
}
```

---

## 6. Tiêu chí hoàn thành

- [ ] `docs/specs/agent-wire-protocol-v1.md` được viết đầy đủ, review bởi ít nhất 1 người
- [ ] `src/shared/agent-wire-protocol.ts` được tạo với constants và types
- [ ] Pseudo-code cho 4 ngôn ngữ (TS, Python, Go, Java) được test thủ công encode/decode đúng frame
- [ ] Spec được reference từ `docs/flows/dev-server-connection-types.md`
