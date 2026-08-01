# SOLUTION: Agent WebSocket Domain — Fix Bugs

**Domain:** agent-ws  
**TDD Reference:** TDD-AG-04 (Handshake-Session), TDD-AG-03 (Connection Modes)  
**Files cần thay đổi:** `src/relay/agent-session.ts`, `src/shared/agent-wire-protocol.ts`, `src/main/dev-server/ws-handshake.ts`  
**Tổng số bugs:** 1 (AWS-001)

---

## BUG-AG-AWS-001 — Fix Handshake method name diverges từ HLD

**File:** `src/relay/agent-session.ts`, `src/shared/agent-wire-protocol.ts`  
**Mức độ:** 🟡 MEDIUM

### Phân tích vấn đề

HLD BL-AWS-02 mô tả custom agent handshake:
```
{ type: 'agent.handshake', agentToken: 'tok_xxx', capabilities: ['execute', 'stream'] }
```

Nhưng `agent-session.ts` gửi handshake dạng JSON-RPC với `method: AGENT_HANDSHAKE_METHOD`  
(= `'agent.hello'`) thay vì `type: 'agent.handshake'`.

### Root Cause Analysis

Có 2 tình huống:

**Tình huống A (không phải bug):** Nếu `src/main/dev-server/ws-handshake.ts` cũng expect `AGENT_HANDSHAKE_METHOD = 'agent.hello'`, thì cả 2 bên đang nói cùng ngôn ngữ → không cần fix, chỉ cần update HLD.

**Tình huống B (bug thực sự):** Nếu server side expect `type: 'agent.handshake'` theo HLD literal, thì client dùng `method: 'agent.hello'` → mismatch → handshake fail.

### Bước 1: Xác nhận server-side expectation

```bash
# Kiểm tra ws-handshake.ts expect gì:
grep -n "agent.handshake\|agent.hello\|AGENT_HANDSHAKE" \
  src/main/dev-server/ws-handshake.ts \
  src/main/dev-server/agent-ws-server.ts
```

### Bước 2A: Nếu server expect `agent.hello` (Tình huống A — match, không phải bug)

Chỉ cần update `AGENT_HANDSHAKE_METHOD` constant cho rõ ràng:

```typescript
// src/shared/agent-wire-protocol.ts — đổi tên constant để rõ ràng hơn:
// BEFORE:
export const AGENT_HANDSHAKE_METHOD = 'agent.hello'

// AFTER (nếu muốn align với HLD):
// Option 1: Giữ nguyên 'agent.hello' nhưng update HLD documentation
export const AGENT_HANDSHAKE_METHOD = 'agent.hello'
// ^ Đây là protocol nội bộ (binary frame) — không phải HLD external protocol

// Option 2: Đổi sang 'agent.handshake' (align với HLD):
export const AGENT_HANDSHAKE_METHOD = 'agent.handshake'
```

### Bước 2B: Nếu server expect `agent.handshake` literal (Tình huống B — fix thực sự)

```typescript
// src/shared/agent-wire-protocol.ts:
// BEFORE:
export const AGENT_HANDSHAKE_METHOD = 'agent.hello'

// AFTER — align với HLD BL-AWS-02:
export const AGENT_HANDSHAKE_METHOD = 'agent.handshake'
```

```typescript
// src/relay/agent-session.ts — không cần đổi code vì dùng constant:
// Code hiện tại đã đúng — chỉ cần constant value thay đổi:
const rpc = {
  jsonrpc: '2.0' as const,
  id: 1,
  method: AGENT_HANDSHAKE_METHOD,  // ← sẽ tự động là 'agent.handshake' sau khi đổi constant
  params: {
    agentVersion:  '2.1.0',
    platform:      process.platform,
    arch:          process.arch,
    nodeVersion:   process.version,
    capabilities:  ['fs', 'git', 'preflight', 'agent-spawn', 'pty'],
    agentToken:    config.agentToken || undefined,
    devServerId:   config.devServerId,
    tools:         tools.map(t => t.name),
  },
}
```

### Bước 3: Cập nhật capabilities list (thiếu so với HLD)

Theo TDD-AG-04, capabilities phải bao gồm tất cả supported operations:

```typescript
// src/relay/agent-session.ts — sửa capabilities trong sendHandshake():
capabilities: [
  'fs',              // file system operations
  'git',             // git operations
  'preflight',       // preflight checks
  'agent-spawn',     // spawn AI agent CLIs (ProfileAwareAgentSpawner)
  'pty',             // PTY management (pty.create, pty.write, pty.resize)
  'ai-credentials',  // AI credential store
],
```

### Fix bổ sung: Handshake response handling

Theo TDD-AG-03 §3, server phải respond với `{ result: { ok: true, sessionId: '...' } }`.  
Code hiện tại trong `agent-session.ts` đã xử lý đúng:

```typescript
// agent-session.ts:232-241 — OK, không cần sửa
if (!handshakeDone) {
  if ((rpc.result as any)?.ok === true) {
    handshakeDone = true
    log.info(`Handshake OK: sessionId=${(rpc.result as any)?.sessionId}`)
    handshakeOkCallbacks.forEach(cb => cb())
  } else if (rpc.error) {
    log.error(`Handshake failed: ${JSON.stringify(rpc.error)}`)
    ws.close(1008, 'Handshake failed')
  }
  return
}
```

---

## Verification Plan

```bash
# 1. Kiểm tra server-side handler (xác nhận Tình huống A vs B):
grep -rn "agent.handshake\|agent.hello\|AGENT_HANDSHAKE" \
  src/main/dev-server/

# 2. Nếu đổi constant value → rebuild và test:
pnpm build:relay

# 3. Integration test:
# - Start agent với valid token
# - Verify handshake log: "Handshake OK: sessionId=..."
# - Verify không có: "Handshake failed"

# 4. Type check:
pnpm tsc --noEmit -p config/tsconfig.node.json
```

---

## Tóm tắt

| File | Action | Bug |
|------|--------|-----|
| `src/shared/agent-wire-protocol.ts` | MODIFY `AGENT_HANDSHAKE_METHOD` value | AWS-001 |
| `src/relay/agent-session.ts` | MODIFY capabilities list | AWS-001 |
| `src/main/dev-server/ws-handshake.ts` | VERIFY — cần kiểm tra trước khi fix | AWS-001 |

> **Lưu ý quan trọng:** Phải kiểm tra `ws-handshake.ts` trước khi đổi constant.  
> Nếu server đã expect `'agent.hello'` → chỉ cần update HLD docs, không cần đổi code.  
> Nếu server expect `'agent.handshake'` → đổi constant value là đủ (code dùng constant nên không cần sửa nhiều nơi).

---

## ✅ Implementation Status (2026-08-01)

AWS-001: Handshake method verified correct. WsHandshakeInfo.devServerId added. FULLY RESOLVED.
