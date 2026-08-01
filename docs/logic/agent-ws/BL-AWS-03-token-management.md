# BL-AWS-03: Agent Token Management

**Domain:** Agent WebSocket  
**Priority:** P1  
**Actor chính:** Agent Developer, Admin  
**Tham chiếu:** FR-17.1, UR-131, F29

---

## Mô tả

Quản lý agent tokens để xác thực WebSocket connections. Tokens được tạo, hiển thị, và revoke từ UI trong DevServer settings.

## Token Lifecycle

```
CREATE: Admin/user nhấn "Generate Token"
  → crypto.randomBytes(32).toString('hex') → 64-char hex token
  → Hash: SHA-256(token) → store hashed version
  → Display raw token ONCE (copy now, không xem lại được)
  → Store { hash, name, createdAt } trong DevServer config

USE: Agent gửi raw token trong handshake
  → Orca computes SHA-256(token)
  → Compare với stored hash
  → Match → auth OK

REVOKE: Admin/user nhấn "Revoke Token"
  → Delete token entry từ DevServer config
  → Existing WS connections với token đó bị close(4001)
```

## Token Storage

```typescript
// Trong DevServer record (per DevServer)
interface DevServerToken {
  id: string;          // UUID
  name: string;        // Human-readable (e.g., "Python agent prod")
  hash: string;        // SHA-256 hex của raw token
  createdAt: string;   // ISO 8601
  lastUsedAt?: string; // Updated on successful WS auth
}
```

Tokens KHÔNG lưu trong database — lưu trong `DevServer` config (để tránh DB leak = token leak).

## UI (DevServerPane)

```
DevServer Settings → Agent Tokens tab

[ + Generate Token ]

┌────────────────────────────────────────────────┐
│ Name           │ Created     │ Last Used │ Action│
├────────────────────────────────────────────────┤
│ Python agent   │ 2026-07-28  │ 1h ago    │ Revoke│
│ Go agent prod  │ 2026-07-27  │ 3d ago    │ Revoke│
└────────────────────────────────────────────────┘
```

Token generation dialog:
```
"New Agent Token"
Token: [a3f9b2c1...d8e7f6] [Copy]
⚠ Store this token now. It won't be shown again.
[ OK ]
```

## Multiple Tokens per DevServer

- Mỗi DevServer có thể có nhiều tokens (tối đa 10)
- Mỗi token có thể thuộc về agent khác nhau
- Token name là label để dễ quản lý (không ảnh hưởng auth)

## Source References

- `src/main/dev-server/agent-token-manager.ts`
- `src/renderer/src/components/DevServerPane.tsx` — AgentTokensTab
- `src/main/agent-ws/agent-ws-server.ts` — validateAgentToken()
