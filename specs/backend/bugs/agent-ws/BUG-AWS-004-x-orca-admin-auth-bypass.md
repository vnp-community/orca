# BUG-AWS-004: `X-Orca-Admin: 1` fallback cho auth — critical security bypass trong dev/staging

**Status:** ✅ FIXED — 2026-08-01  
**Fixed by:** TASK-AWS-001  
**Implementation:** agent-token-routes.ts: X-Orca-Admin bypass removed  

## Mức độ: 🔴 HIGH

## Tóm tắt

`src/server/agent-token-routes.ts:36-43`:
```typescript
function isAuthorized(req: IncomingMessage): boolean {
  const apiSecret = process.env['ORCA_AGENT_API_SECRET']
  if (apiSecret) {
    const auth = req.headers['authorization'] ?? ''
    return auth === `Bearer ${apiSecret}`
  }
  // Dev fallback: X-Orca-Admin: 1 header (no secret configured)
  return req.headers['x-orca-admin'] === '1'  ← ⚠️ NO secret needed!
}
```

Nếu `ORCA_AGENT_API_SECRET` **không được set** → bất kỳ ai gửi `X-Orca-Admin: 1` header có thể:
- `POST /api/agent-token` → nhận token → kết nối làm Dev Server
- Sau đó thực thi bất kỳ relay commands nào (git, fs, agent.spawn)

## Risk scenario

Orca deploy lên server production mà quên set `ORCA_AGENT_API_SECRET`:
```bash
curl -H "X-Orca-Admin: 1" -X POST http://orca-server/api/agent-token \
  -d '{"devServerId":"pwned","name":"attacker"}'
→ {"token":"agt_xxx","expiresIn":300,...}
```

Attacker kết nối với token → `agent.spawn` arbitrary process trên Dev Server.

## Ảnh hưởng

1. Unauthenticated token generation nếu env var thiếu
2. Không có startup warning nếu `ORCA_AGENT_API_SECRET` không được set
3. Comment trong code nói "dev-only" nhưng không có guard ngăn production deploy thiếu env var

## Fix đề xuất

```typescript
function isAuthorized(req: IncomingMessage): boolean {
  const apiSecret = process.env['ORCA_AGENT_API_SECRET']?.trim()
  if (!apiSecret) {
    // SECURITY: reject if no secret configured — refuse all requests
    console.error('[SECURITY] ORCA_AGENT_API_SECRET not set! POST /api/agent-token BLOCKED.')
    return false
  }
  const auth = req.headers['authorization'] ?? ''
  return auth === `Bearer ${apiSecret}`
}
```

Thêm startup validation:
```typescript
// server-bootstrap.ts
if (!process.env['ORCA_AGENT_API_SECRET']) {
  console.warn('[SECURITY WARNING] ORCA_AGENT_API_SECRET not set — /api/agent-token endpoint DISABLED')
}
```

## Files liên quan

- `src/server/agent-token-routes.ts:36-43`: fallback bypass
- `src/main/server-bootstrap.ts`: startup validation cần thêm
