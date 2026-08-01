# BUG-AG-ORCH-013: relay-ws mode fallback token `'relay-secret'` hardcode — security vulnerability

## Mức độ: 🟡 MEDIUM

## Tóm tắt

`agent-connection-relay.ts:26`:
```typescript
const token = config.agentToken || 'relay-secret'
```

Nếu `config.agentToken` không được set hoặc là empty string → token mặc định là chuỗi literal **`'relay-secret'`**.

Bất kỳ ai kết nối đến `ws://<dev-server>:<port>/orca-relay?token=relay-secret` đều sẽ được authenticated.

## Ảnh hưởng

1. **Security**: Attacker biết hardcoded token có thể:
   - Gọi `agent.spawn` để spawn arbitrary processes trên Dev Server
   - Gọi `fs.*` để đọc/ghi files trên Dev Server
   - Gọi `git.*` để thao tác repository
2. **Silent fail**: Admin không biết token chưa được set → không có warning/error

## Vị trí cần fix

`src/relay/agent-connection-relay.ts:26`:
```typescript
// Before:
const token = config.agentToken || 'relay-secret'

// After:
const token = config.agentToken?.trim()
if (!token) {
  throw new Error(
    'AGENT_TOKEN is required for relay-websocket mode. ' +
    'Set ORCA_AGENT_TOKEN environment variable on the Dev Server.'
  )
}
```

## Cách phát hiện

Hiện tại không có startup validation cho `agentToken`. Cần thêm vào `agent-entry.ts` hoặc `agent-config.ts`.

## Liên quan đến luồng

- **BL-AG-01**: Agent authentication — missing security check
- **BUG-AG-AIP-002**: hardcode placeholder (bug tương tự ở agent credential store)

---

## ✅ Fix Status: RESOLVED (2026-08-01)

**Fix:** agent-connection-relay.ts: token read from config.agentToken (ORCA_AGENT_TOKEN env var). FATAL error if empty/missing. No hardcoded fallback.
