# F29 — Agent WebSocket Protocol

| Trường | Giá trị |
|--------|---------|
| **ID** | F29 |
| **Tên** | Agent WebSocket Protocol |
| **Ưu tiên** | P1 |
| **Trạng thái** | ✅ Phát hành |
| **CRs** | [agent/CR-AG-001~004](../crs/v2/agent/) |
| **Phiên bản** | v4.1+ |
| **ADR References** | ADR-005 |
| **HLD References** | C3.8, C4.5 |

---

## Mô tả

Orca hỗ trợ kết nối với **AI agents tùy chỉnh** qua WebSocket — cho phép agent viết bằng bất kỳ ngôn ngữ nào (TypeScript, Python, Go, Java...) giao tiếp với Orca qua một **giao thức chuẩn ngôn ngữ-agnostic**. Có 2 mode kết nối tùy theo topology mạng.

---

## Vấn đề cần giải quyết

Phase 1 chỉ hỗ trợ `relay-ssh`. Phase 2 mở rộng để:
- Agent ở container/VM không cần SSH access
- Developer bên ngoài có thể viết agent tùy chỉnh bằng ngôn ngữ yêu thích
- Orca hoạt động như WS server nhận agent kết nối vào (direct mode)
- Agent tự expose WS server để Orca kết nối vào (relay mode)

---

## Tính năng chi tiết

### Mode 1: relay-websocket (Orca → Agent)

```
Agent WS Server                     Orca App
ws://agent-host:6799/orca-relay     │
        │ ◄── Orca WebSocket ────────┘
        │ ◄── Authorization: Bearer <agentToken>
        │ agent.handshake ──────────►
        │ ◄── handshake-ok ──────────
        │◄══ JSON-RPC 2.0 (framed) ═►│
```

Config: `connectionType: 'relay-websocket'`, `wsUrl: 'ws://agent-host:6799/orca-relay'`

Phù hợp khi: Agent chạy trong LAN/VM có port public.

---

### Mode 2: direct-websocket (Agent → Orca)

```
Agent                               Orca WS Server
        │                           ws://orca:6768/agent
        ├─── WebSocket connect ─────►
        ├─── agent.handshake ───────► (agentToken validated)
        │◄── handshake-ok ───────────
        │◄══ JSON-RPC 2.0 (framed) ═►│
```

Config: `connectionType: 'direct-websocket'`

Phù hợp khi: Agent ở container không có port public; Orca expose trên internet.

---

### Wire Protocol — Frame Format (CR-AG-001)

```
Byte  0     : TYPE (uint8)  0x01=Regular | 0x09=KeepAlive
Bytes 1–4   : SEQ  (uint32 big-endian)  — monotonically increasing
Bytes 5–8   : ACK  (uint32 big-endian)  — highest received SEQ from peer
Bytes 9–12  : LEN  (uint32 big-endian)  — payload byte length
Bytes 13+   : PAYLOAD (UTF-8 JSON-RPC 2.0)
Total header: 13 bytes
```

JSON-RPC methods Orca gọi agent: `preflight.check`, `pty.spawn`, `fs.readDir`, `git.exec`

---

### Agent Token Management

```typescript
// Generate (crypto.randomBytes(32).hex() = 64 hex chars)
rawToken → stored as SHA-256(rawToken)  // không lưu plain text

// Validate khi kết nối
SHA-256(received) === storedHash ? OK : close(4001)
```

- UI: `AgentTokenPanel` — hiển thị token 1 lần, copy-to-clipboard, regenerate
- Close codes: 4001=Unauthorized, 4002=HandshakeTimeout, 4003=VersionMismatch

---

### Language-agnostic SDK examples

**TypeScript:**
```typescript
const body = Buffer.from(JSON.stringify(payload))
const header = Buffer.allocUnsafe(13)
header[0] = 0x01
header.writeUInt32BE(seq, 1); header.writeUInt32BE(ack, 5); header.writeUInt32BE(body.length, 9)
ws.send(Buffer.concat([header, body]))
```

**Python:** `struct.pack('>BIII', 0x01, seq, ack, len(body)) + body`

**Go:** `binary.BigEndian.PutUint32(...)` + `conn.WriteMessage(websocket.BinaryMessage, ...)`

---

## Tiêu chí chấp nhận

- [x] `relay-websocket` mode hoạt động (Orca → ws://agent:PORT/orca-relay)
- [x] `direct-websocket` mode hoạt động (Agent → ws://orca:6768/agent)
- [x] Wire protocol 13-byte header đúng spec
- [x] `agent.handshake` + `handshake-ok` cả 2 mode
- [x] KeepAlive 0x09 gửi mỗi 30s, timeout 90s
- [x] Token validate via SHA-256 (không lưu plain text)
- [x] AgentWebSocketServer tại `/agent` (tách biệt `/` dành cho browser)
- [x] UI: AddDevServerDialog 3 modes, AgentTokenPanel
- [x] 66/66 backend tests pass

---

## Yêu cầu kỹ thuật

| Component | File |
|-----------|------|
| Wire protocol types | `src/shared/agent-wire-protocol.ts` |
| WsTransport adapter | `src/main/dev-server/ws-transport.ts` |
| WsHandshake logic | `src/main/dev-server/ws-handshake.ts` |
| DevServerRelayBridge | `src/main/dev-server/dev-server-relay-bridge.ts` |
| AgentWebSocketServer | `src/main/dev-server/agent-ws-server.ts` |
| Token management | `src/main/dev-server/dev-server-manager.ts` |
| Server bootstrap | `src/main/server-bootstrap.ts` |
| AddDevServerDialog UI | `src/renderer/src/components/dev-server/AddDevServerDialog.tsx` |
| AgentTokenPanel UI | `src/renderer/src/components/dev-server/AgentTokenPanel.tsx` |
| Public protocol spec | `docs/specs/agent-wire-protocol-v1.md` |
| relay-ws guide | `docs/specs/agent-relay-websocket-server.md` |
| direct-ws guide | `docs/specs/agent-direct-websocket-client.md` |

**Tests:** 66 backend tests | **Frontend:** 0 TS errors

---

## Metrics

| KPI | Mục tiêu |
|-----|----------|
| Handshake round-trip | < 500ms |
| KeepAlive interval | 30s |
| KeepAlive timeout | 90s (3 missed) |
| Token entropy | 32 bytes (64 hex chars) |
| Max frame payload | 16 MB |
