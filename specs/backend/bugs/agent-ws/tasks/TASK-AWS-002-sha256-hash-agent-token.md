# TASK-AWS-002: SHA-256 hash agent token trước khi lưu vào memory

**Priority:** 🟡 MEDIUM — memory dump leak prevention  
**Effort:** ~30 phút  
**Status:** ✅ DONE  
**Bug refs:** BUG-AWS-002  
**Solution ref:** [SOLUTION-agent-ws-exact.md](../solutions/SOLUTION-agent-ws-exact.md)

---

## Mục tiêu

Hash agent token bằng SHA-256 trước khi lưu vào `pendingSlots` Map. Agentgửi raw token khi connect, server verify bằng `SHA256(received) === stored_hash`.

## Files cần sửa

1. `src/server/agent-token-routes.ts`
2. `src/main/dev-server/dev-server-relay-bridge.ts`
3. `src/main/dev-server/agent-ws-server.ts`

---

## Bước 1 — agent-token-routes.ts

Thêm import `createHash` và hash token trước khi lưu vào `pendingMeta`:

```typescript
// Thêm import ở đầu file:
import { createHash } from 'node:crypto'

// Helper function (thêm sau imports):
function sha256hex(input: string): string {
  return createHash('sha256').update(input).digest('hex')
}

// Trong POST handler (khoảng line 113–121), thay thế:
// TRƯỚC:
pendingMeta.set(token, { devServerId, createdAt: Date.now(), expiresAt })

// SAU:
const tokenHash = sha256hex(token)
pendingMeta.set(tokenHash, { devServerId, createdAt: Date.now(), expiresAt })
// Response trả về raw token (agent cần raw token để connect):
// (response body không thay đổi — token vẫn là raw)
```

---

## Bước 2 — dev-server-relay-bridge.ts

Trong `connectDirectWebSocket()`, hash agentToken khi đăng ký slot:

```typescript
// Thêm import ở đầu file (nếu chưa có):
import { createHash } from 'node:crypto'

function sha256hex(input: string): string {
  return createHash('sha256').update(input).digest('hex')
}

// Trong connectDirectWebSocket() khoảng line 214–240:
const agentToken = generateAgentToken(this.config.id)
const agentTokenHash = sha256hex(agentToken)  // ← NEW: hash

const disposer = this.agentWsServer!.registerSlot(
  agentTokenHash,  // ← pass hash (không phải raw token)
  (mux, info) => { ... },
  (reason) => { ... }
)

// Emit raw token cho UI (agent cần raw token để connect):
this.emit('agentTokenGenerated', {
  devServerId: this.config.id,
  agentToken,   // ← vẫn emit raw token
  orcaUrl: orcaWsUrl,
})
```

---

## Bước 3 — agent-ws-server.ts

Validator trong `handleConnection()` phải hash incoming token trước khi lookup:

```typescript
// Thêm import:
import { createHash } from 'node:crypto'

// Trong handleConnection() khoảng line 114–118:
runOrcaReceiverHandshake(
  ws,
  (rawToken) => {
    const hash = createHash('sha256').update(rawToken).digest('hex')
    return this.pendingSlots.has(hash)  // ← check hash
  },
  this.orcaVersion
)
  .then((info) => {
    const agentToken = info.agentToken ?? ''
    const agentHash  = createHash('sha256').update(agentToken).digest('hex')
    const slot = this.pendingSlots.get(agentHash)  // ← lookup by hash
    // ... rest unchanged
  })
```

---

## Verification

```bash
pnpm tsc --noEmit
pnpm vitest run src/main/dev-server/__tests__/ 2>/dev/null || true

# Verify: agent vẫn connect được (end-to-end test)
# Verify: pendingSlots không còn chứa raw tokens
```
