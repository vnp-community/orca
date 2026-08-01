# Luồng Dữ liệu — Agent WebSocket Protocol

**Domain:** Agent WebSocket (AgentWS)  
**Nghiệp vụ:** BL-AWS-01 → BL-AWS-03  
**Kiến trúc tham chiếu:** HLD v1 — C3.8, C4.5, ADR-005, F29 Agent WebSocket

---

## Thành phần tham gia

| Thành phần | Layer | Vai trò |
|------------|-------|---------|
| Custom AI Agent | External Client | Agent tự viết kết nối WebSocket |
| Dev Server Agent | Remote | Node.js agent binary |
| AgentConnectionManager | Backend | Persistent WS pool quản lý kết nối |
| Orca Web Server | Backend | ws://orca:6768/agent endpoint |
| HMAC-SHA256 Signer | Security | Sign RpcExecutionContext (30s TTL) |
| SSH Relay | Transport | relay-websocket mode |
| Server Database | Persistence | orca_agent_tokens |

---

## BL-AWS-01 — relay-websocket Mode (Orca → Agent)

```
[Orca Web Server] muốn kết nối đến Agent đang chạy trên Dev Server
    │
    ▼
[AgentConnectionManager.connect(agentEndpoint)]
    ├─ agentEndpoint: ws://dev-server-01:6799/orca-relay
    ├─ Build Bearer token: HMAC-SHA256(ORCA_AGENT_SECRET, agentId + timestamp)
    ├─ Kết nối WebSocket:
    │   ws://dev-server-01:6799/orca-relay
    │   Headers: Authorization: Bearer <token>
    ├─ Handshake: { type: 'hello', orcaVersion: '5.0', capabilities: [...] }
    ├─ Agent response: { type: 'hello-ok', agentId, sessionId }
    └─ Connection established → add to pool
    │
    ▼
[JSON-RPC 2.0 over Binary WebSocket frames]
    ├─ Orca → Agent: { jsonrpc: '2.0', method: 'task.execute', params: {...}, id: 1 }
    ├─ Agent → Orca: { jsonrpc: '2.0', result: {...}, id: 1 }
    └─ Agent → Orca: { jsonrpc: '2.0', method: 'event.statusChanged', params: {...} }
    │
    ▼
RpcExecutionContext (signed, 30s TTL):
    { userId, sessionId, worktreeId, permissions[], issuedAt, signature: HMAC }

SSH Relay variant (agent behind firewall):
    Orca → SSH Tunnel → Dev Server port → relay ws://<agent>

Luồng:
Orca Web Server → WebSocket connect (Bearer token)
               → ws://dev-server:6799/orca-relay
               → Handshake → JSON-RPC session
               → bidirectional task dispatch
```

---

## BL-AWS-02 — direct-websocket Mode (Agent → Orca)

```
Custom AI Agent (tự viết)
    │
    ▼
[Custom Agent Code]
    ├─ WebSocket connect: ws://orca:6768/agent
    ├─ Handshake message:
    │   { type: 'agent.handshake', agentToken: 'tok_xxx', capabilities: ['execute', 'stream'] }
    │
    ▼
[Orca Web Server — AgentWsRouter]
    ├─ Verify agentToken:
    │   SELECT from orca_agent_tokens WHERE token_hash=SHA256(agentToken) AND is_active=1
    │   ← Server DB
    ├─ IF valid: { type: 'handshake-ok', sessionId: uuid(), serverId }
    ├─ IF invalid: { type: 'handshake-error', code: 'INVALID_TOKEN' } → close
    ├─ Register connection: AgentConnectionManager.register(sessionId, ws)
    └─ Start keepalive ping (30s interval)
    │
    ▼
[Full JSON-RPC 2.0 session over binary frames]
    Agent ← Orca: task assignments, prompt dispatch
    Agent → Orca: tool call results, status updates, completion events
    │
    ▼
[RpcExecutionContext injection per call]
    Orca signs context: HMAC-SHA256 { userId, worktreeId, issuedAt }
    Agent verifies signature before executing

Luồng:
Custom Agent → WebSocket ws://orca:6768/agent
            → Handshake (agentToken) → Orca (verify token in DB)
            → handshake-ok (sessionId)
            → JSON-RPC bidirectional (task ↔ result ↔ events)
```

---

## BL-AWS-03 — Agent Token Management

```
Admin hoặc Agent Developer
    │
    ▼
[Admin SPA] Settings → Agent Tokens → "Generate New Token"
    │ POST /admin/api/agent-tokens
    Body: { name: "my-custom-agent", permissions: ['execute', 'git.read'] }
    ▼
[Orca Web Server — AgentTokenRouter]
    ├─ requireAdmin() guard
    ├─ Generate raw token: 'tok_' + crypto.randomBytes(32).toString('hex')
    ├─ Hash: token_hash = SHA256(rawToken)
    ├─ INSERT orca_agent_tokens {
    │     id, name, token_hash, permissions,
    │     is_active: true, created_by: adminId, created_at
    │   }  ← Server DB
    └─ Return { token: rawToken, id }
        [RAW TOKEN CHỈ HIỂN THỊ 1 LẦN — không lưu plaintext]

REVOKE TOKEN:
    DELETE /admin/api/agent-tokens/:id
    ├─ UPDATE orca_agent_tokens SET is_active=false  ← DB
    └─ AgentConnectionManager.disconnectByTokenId(id)
        → close WebSocket gracefully

LIST TOKENS:
    GET /admin/api/agent-tokens
    └─ SELECT id, name, last_used_at, is_active FROM orca_agent_tokens
       [KHÔNG trả về token_hash]

Luồng:
Admin → POST /admin/api/agent-tokens → TokenService
      → Server DB (INSERT hash only)
      → Return raw token (display once)

Agent → WebSocket + token → Orca (SHA256 verify against DB)
      → Server DB (UPDATE last_used_at)
```

---

## Sơ đồ tổng quan — Agent WebSocket

```
Mode 1: relay-websocket (Orca kết nối đến Agent)
┌─────────────────┐  WS (Bearer)   ┌──────────────────────────────┐
│  Orca Web Server│───────────────►│  Dev Server Agent            │
│  AgentConn.     │                │  ws://dev-server:6799/       │
│  Manager        │◄───────────────│  orca-relay endpoint         │
└─────────────────┘  JSON-RPC 2.0  └──────────────────────────────┘

Mode 2: direct-websocket (Agent kết nối vào Orca)
┌─────────────────┐  WS (agentToken)  ┌─────────────────────────┐
│  Custom Agent   │──────────────────►│  Orca Web Server         │
│  (self-written) │                   │  ws://orca:6768/agent    │
│                 │◄──────────────────│  AgentWsRouter           │
└─────────────────┘  JSON-RPC 2.0     └──────────┬──────────────┘
                                                  │
                                       ┌──────────▼─────────────┐
                                       │  Server Database        │
                                       │  orca_agent_tokens     │
                                       │  (token_hash only)     │
                                       └────────────────────────┘

Security:
- Token: SHA-256 hash stored, plaintext never persisted
- RpcExecutionContext: HMAC-SHA256, 30s TTL
- All frames: binary WebSocket (not text)
```
