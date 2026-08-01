# TASK-AWS-001: Xóa X-Orca-Admin auth bypass trong agent-token-routes

**Priority:** 🔴 CRITICAL SECURITY — RCE nếu ORCA_AGENT_API_SECRET chưa set  
**Effort:** ~10 phút  
**Status:** ✅ DONE  
**Bug refs:** BUG-AWS-004  
**Solution ref:** [SOLUTION-agent-ws-exact.md](../solutions/SOLUTION-agent-ws-exact.md)

---

## Mục tiêu

Xóa fallback `X-Orca-Admin: 1` trong `isAuthorized()` — nếu `ORCA_AGENT_API_SECRET` không được set, BLOCK tất cả requests (thay vì cho phép với header bypass).

## File cần sửa

```
src/server/agent-token-routes.ts
```

## Thay đổi cụ thể

### Lines 36–44 — Thay thế hàm `isAuthorized()`:

**TRƯỚC (buggy — security bypass):**
```typescript
function isAuthorized(req: IncomingMessage): boolean {
  const apiSecret = process.env['ORCA_AGENT_API_SECRET']
  if (apiSecret) {
    const auth = req.headers['authorization'] ?? ''
    return auth === `Bearer ${apiSecret}`
  }
  // Dev fallback: X-Orca-Admin: 1 header (no secret configured)
  return req.headers['x-orca-admin'] === '1'
}
```

**SAU (secure — block if no secret):**
```typescript
function isAuthorized(req: IncomingMessage): boolean {
  const apiSecret = process.env['ORCA_AGENT_API_SECRET']?.trim()
  if (!apiSecret) {
    // SECURITY: No secret configured — block all requests.
    // Set ORCA_AGENT_API_SECRET env var to enable this endpoint.
    console.error(
      '[SECURITY] ORCA_AGENT_API_SECRET not configured. ' +
      'POST /api/agent-token is BLOCKED. ' +
      'Set ORCA_AGENT_API_SECRET to a strong random secret.'
    )
    return false
  }
  const auth = req.headers['authorization'] ?? ''
  return auth === `Bearer ${apiSecret}`
}
```

## Bước tiếp theo — Startup warning

Tìm `src/main/server-bootstrap.ts` (hoặc `src/server/index.ts`) và thêm:

```typescript
// Thêm vào cuối bootstrap, sau tất cả setup, TRƯỚC listen():
if (!process.env['ORCA_AGENT_API_SECRET']) {
  console.warn(
    '[SECURITY WARNING] ORCA_AGENT_API_SECRET not set. ' +
    'The /api/agent-token endpoint is DISABLED. ' +
    'Dev servers cannot be connected until this is configured.\n' +
    'Generate a secret: openssl rand -hex 32'
  )
}
```

## Verification

```bash
# Test: curl không có auth → 401 (không phải 200)
curl -s -o /dev/null -w "%{http_code}" \
  -H "X-Orca-Admin: 1" \
  -X POST http://localhost:6768/api/agent-token \
  -d '{"devServerId":"test"}'
# Expected: 401

# Test: curl với correct Bearer token → 200
ORCA_AGENT_API_SECRET=mysecret curl -s \
  -H "Authorization: Bearer mysecret" \
  -X POST http://localhost:6768/api/agent-token \
  -d '{"devServerId":"test"}'
# Expected: 200 với token JSON

pnpm tsc --noEmit
```
