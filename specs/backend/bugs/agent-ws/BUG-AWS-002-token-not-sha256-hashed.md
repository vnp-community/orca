# BUG-AWS-002: Token không SHA256 hash trước khi lưu — BL-AWS-03 security model khác HLD

**Status:** ✅ FIXED — 2026-08-01  
**Fixed by:** TASK-AWS-002  
**Implementation:** agent-ws-server.ts: SHA-256 hash in pendingSlots  

## Mức độ: 🟡 MEDIUM

## Tóm tắt

HLD BL-AWS-03 mô tả:
```
ProviderCredentialWriter:
  Hash: token_hash = SHA256(rawToken)
  INSERT orca_agent_tokens { token_hash, ... }  ← KHÔNG lưu raw token
```

Thực tế `src/server/agent-token-routes.ts`:
```typescript
// Line 113:
const token = generateAgentToken(devServerId)
// ...
pendingMeta.set(token, { devServerId, createdAt: Date.now(), expiresAt })
```

Token được lưu **in-memory** trong `pendingMeta Map` với **raw token** (không hash).  
Không có bảng `orca_agent_tokens` trong DB với `token_hash`.  
Token không persist qua restart.

## Chi tiết

`generateAgentToken()` từ `src/shared/agent-wire-protocol.ts` — token được tạo và chuyển trực tiếp đến `AgentWebSocketServer.registerSlot(token, ...)`.

`AgentWebSocketServer` xác thực token như thế nào? Cần kiểm tra `agent-ws-server.ts`.

Nếu AgentWsServer so sánh raw token (không hash) → nếu memory bị dump, tất cả tokens bị lộ.

## Ảnh hưởng

1. Token không persist → sau restart, Dev Server phải lấy token mới (hoặc dùng static token)
2. Token không hash trong memory → memory dump leak
3. Không có `token_hash` DB table → không thể list/revoke tokens qua Admin UI (như HLD mô tả)
4. BL-AWS-03 REVOKE TOKEN: `UPDATE orca_agent_tokens SET is_active=false` → không có bảng này

## Fix đề xuất

1. Tạo bảng `orca_agent_tokens` (persist)
2. Hash token trước khi lưu vào memory/DB
3. AgentWsServer verify bằng `SHA256(incoming) === stored_hash`

## Files liên quan

- `src/server/agent-token-routes.ts:113`: token raw lưu vào pendingMeta
- `src/main/dev-server/agent-ws-server.ts`: verify token
