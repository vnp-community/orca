# ADR-014 — Gateway–Agent JSON-RPC 2.0 Protocol v3

| Trường | Giá trị |
|--------|---------|
| **ID** | ADR-014 |
| **Trạng thái** | 🚧 Proposed |
| **Ngày** | 2026-07-30 |
| **HLD Ref** | C3.13, C4.11 |
| **CR Ref** | CR-DS-002 |
| **Code Ref** | `src/agent/rpc/agent-rpc-server.ts`, `src/main/dev-server/agent-connection-manager.ts` |
| **Feature Ref** | F29 (agent WS protocol) |
| **Supersedes** | [ADR-005](./ADR-005-agent-websocket-binary-wire-protocol.md) |

---

## Bối cảnh

### Vấn đề với binary wire protocol (ADR-005)

ADR-005 dùng **13-byte binary header** (TYPE + SEQ + ACK + LEN) cho communication giữa AI Agents và Orca Server. Protocol này được thiết kế cho use case AI Agent ↔ Orca (một chiều: agent kết nối, nhận tools).

Trong v6.0, communication pattern thay đổi căn bản:
- **Cũ:** AI agent (Python/TypeScript) ↔ Orca Server (tool execution)
- **Mới:** Dev Server Agent (Node.js) ↔ Orca Gateway (full bi-directional RPC)

**Vấn đề với 13-byte binary header trong context mới:**

| Vấn đề | Chi tiết |
|--------|---------|
| **Không debuggable** | Binary frames không thể đọc với wireshark/curl |
| **Không có context propagation** | Không có cơ chế carry signed execution context per-call |
| **Protocol mismatch** | Designed for agent tools (simple req/resp) — không phù hợp với bi-directional event streaming phức tạp |
| **SDK complexity** | Phải implement 13-byte encoder/decoder trước khi làm bất cứ điều gì |
| **No method namespacing** | Không có chuẩn cho method routing |

---

## Quyết định

### JSON-RPC 2.0 over WebSocket — v3 Protocol

**Lý do chọn JSON-RPC 2.0:**
- Standard protocol, widely understood
- Built-in method routing, error codes, request IDs
- Human-readable → debuggable
- Extensible via `params` object

**Transport: Persistent WebSocket (outbound từ Agent)**

```
Agent kết nối ra ngoài:
  wss://orca-backend.company.com/agent/connect
  Header: Authorization: Bearer <agentToken>

Không cần binary framing vì:
  - WebSocket đã có frame boundary (không cần LEN field)
  - WebSocket đã có ordered delivery (không cần SEQ/ACK)
  - WebSocket text frames đủ cho JSON
  - Binary frames chỉ dùng cho PTY output (raw bytes)
```

### Message Format

```typescript
// Request: Gateway → Agent
{
  jsonrpc: '2.0',
  id: string,             // UUID v4
  method: string,         // 'pty.create', 'git.status', etc.
  params: {
    ...methodParams,      // method-specific params
    _ctx: SignedExecutionContext  // ALWAYS present (security)
  }
}

// Response: Agent → Gateway
{
  jsonrpc: '2.0',
  id: string,             // matches request id
  result: any             // method-specific result
}

// Error Response
{
  jsonrpc: '2.0',
  id: string,
  error: {
    code: number,
    message: string,
    data?: any
  }
}

// Event: Agent → Gateway (no id — unidirectional)
{
  jsonrpc: '2.0',
  method: 'event',
  params: {
    type: 'event.stream' | 'event.agentStatus' | 'event.health' | ...,
    ...eventData
  }
}
```

### Handshake Sequence

```
Agent → wss://gateway/agent/connect
  Authorization: Bearer <agentToken>

Gateway → { type: 'handshake.challenge', nonce: '<32-byte-random>' }

Agent → {
  type: 'handshake.response',
  nonce: '<echo>',
  agentId: '<uuid>',
  capabilities: { agentVersion, protocolVersion: 3, os, arch, features, resources },
  signature: HMAC_SHA256(nonce + agentId, agentSecret)
}

Gateway → {
  type: 'handshake.ok',
  sessionKey: '<session-key>',
  config: { heartbeatIntervalMs: 30000, maxConcurrentRpcs: 20 }
}

[Connection established — Agent ready to receive RPC, Gateway ready to receive events]
```

### Keepalive (thay KEEPALIVE frame)

```typescript
// WebSocket ping/pong (native WS protocol)
// Gateway sends ping every 30s
// Agent responds with pong
// No heartbeat → Gateway marks agent OFFLINE after 90s
```

### Binary PTY Data (exception to JSON)

PTY output là binary → dùng WS binary frames:

```typescript
// Binary frame for PTY/step output
// Format: [PTY_MAGIC: 4 bytes][ptyId or stepRunId: 36 bytes UTF-8][seq: 4 bytes BE][data: rest]
// Lý do: JSON encoding của binary PTY output tốn gấp đôi bandwidth
const PTY_FRAME_MAGIC = 0x4f524341 // 'ORCA' in hex
```

### Method Categories

```
pty.*         — PTY lifecycle operations
agent.*       — AI agent operations
worktree.*    — Worktree management
git.*         — Git operations
github.*      — GitHub operations
fs.*          — File system operations
aiProvider.*  — AI provider credential management
step.*        — Workflow step execution
health.*      — Health and diagnostics
```

### Error Codes

```typescript
const AgentErrorCodes = {
  UNAUTHORIZED: 4001,
  CAPABILITY_NOT_SUPPORTED: 4002,
  RESOURCE_NOT_FOUND: 4004,
  PROJECT_ROOT_VIOLATION: 4010,
  CONCURRENT_LIMIT: 4029,
  EXECUTION_TIMEOUT: 4408,
  AGENT_BINARY_NOT_FOUND: 5001,
  CREDENTIAL_NOT_FOUND: 5002,
  GIT_ERROR: 5003,
  PTY_ERROR: 5004,
}
```

---

## So sánh với ADR-005 (binary protocol)

| Khía cạnh | ADR-005 (binary) | ADR-014 (JSON-RPC) |
|-----------|-----------------|-------------------|
| Debugging | Khó (cần custom parser) | Dễ (bất kỳ WS client) |
| Sequencing | SEQ/ACK fields | WS native ordering |
| Frame boundary | LEN field | WS native frames |
| Context propagation | Không có | `_ctx` field trong params |
| Method routing | Custom dispatcher | JSON-RPC method field |
| Error handling | Custom error types | JSON-RPC error object (standard) |
| Binary data | Toàn bộ trong binary frame | Mixed: JSON + WS binary frames cho PTY |
| SDK complexity | 13-byte encoder required | Any JSON + WS library |
| Protocol version | Implicit | `protocolVersion: 3` trong handshake |

---

## Lý do chọn

| Lựa chọn | Đánh giá |
|----------|---------| 
| **JSON-RPC 2.0 over WebSocket** ✅ | Standard, debuggable, extensible, no custom framing |
| Giữ 13-byte binary header (ADR-005) | Không có context propagation, khó debug, SDK complexity |
| gRPC over WS | Protobuf overhead, code-gen complexity, overkill cho single-language use |
| GraphQL subscriptions | Over-engineered, mutation/query mismatch |
| SSE (Server-Sent Events) | Uni-directional, không phù hợp bi-directional RPC |
| NATS / message broker | External dependency, added complexity |

---

## Hậu quả

**Tích cực:**
- Protocol dễ debug (bất kỳ WS client đều có thể connect)
- Không cần custom binary encoder/decoder
- `_ctx` field cho phép per-call signed context (security, ADR-015)
- `protocolVersion` trong handshake → clean versioning
- Standard JSON-RPC error codes
- PTY binary frames giải quyết bandwidth concern

**Tiêu cực:**
- JSON text > binary cho high-frequency ops (PTY output → giải quyết bằng WS binary frames)
- `_ctx` field thêm ~500 bytes per request → overhead nhỏ (justified by security)
- Cần migrate `agent.js` (deploy/dev/agent/) từ binary protocol sang JSON-RPC

---

## Trạng thái Implementation

❌ Chưa implement (v6.0 proposed)  
🎯 `src/agent/rpc/agent-rpc-server.ts` — JSON-RPC server  
🎯 `src/agent/rpc/method-router.ts` — method dispatcher  
🎯 `src/agent/rpc/event-emitter.ts` — event streaming  
🎯 `src/main/dev-server/agent-connection-manager.ts` — Gateway side  
🎯 Update `deploy/dev/agent/agent.js` — migrate từ binary protocol
