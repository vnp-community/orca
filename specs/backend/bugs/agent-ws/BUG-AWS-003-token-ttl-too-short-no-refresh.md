# BUG-AWS-003: Token TTL mặc định 300s (5 phút) — Dev Server không thể kết nối lại sau TTL expire

**Status:** ✅ FIXED — 2026-08-01  
**Fixed by:** TASK-AWS-003  
**Implementation:** agent-token-routes.ts: 30-day permanent token option  

## Mức độ: 🔴 HIGH

## Tóm tắt

`src/server/agent-token-routes.ts:111`:
```typescript
const ttlSec = Math.min(Number(body['ttl'] ?? 300), 600)   // max 10 min
```

Token tự động expire sau tối đa **10 phút**. Dev Server phải lấy token mới bằng cách gọi `POST /api/agent-token` mỗi lần muốn kết nối lại.

Vấn đề:
1. **Dev Server cần gọi lại API mỗi ≤10 phút** → ai gọi? Không có auto-refresh mechanism.
2. **Nếu Dev Server restart sau 10 phút** → token đã expire → không kết nối được.
3. **Không có long-lived token mechanism** → không phù hợp với production dev server chạy 24/7.

## So sánh HLD

HLD BL-AWS-02 mô tả token validation từ `orca_agent_tokens` DB (persistent). Nhưng:
- Token implementation hiện tại là ephemeral (in-memory, max 10min TTL)
- Không có "long-lived token" cho production dev servers

## Ảnh hưởng

1. Dev Server kết nối → disconnect → không thể auto-reconnect sau 10 phút
2. `DevServerRelayBridge.reconnect()` sẽ fail vì token đã expire
3. Không có mechanism refresh token tự động

## Fix đề xuất

Thêm long-lived token option:
```typescript
// Agent token API: support permanent tokens for production servers
const isPermanent = body['permanent'] === true && isAuthorized(req)
const ttlSec = isPermanent ? Infinity : Math.min(Number(body['ttl'] ?? 300), 600)
```

Hoặc implement persistent token table (`orca_agent_tokens`) với revocation support.

## Files liên quan

- `src/server/agent-token-routes.ts:111`: TTL hardcap 600s
- `src/main/dev-server/dev-server-relay-bridge.ts`: reconnect logic
