# TASK-AWS-003: Thêm permanent token option (TTL 30 ngày)

**Priority:** 🔴 HIGH — Dev Server không thể duy trì kết nối > 10 phút  
**Effort:** ~20 phút  
**Status:** ✅ DONE  
**Bug refs:** BUG-AWS-003  
**Solution ref:** [SOLUTION-agent-ws-exact.md](../solutions/SOLUTION-agent-ws-exact.md)

---

## Mục tiêu

Thêm `permanent: true` option vào `POST /api/agent-token` để tạo long-lived token (TTL 30 ngày) cho production dev servers.

## File cần sửa

```
src/server/agent-token-routes.ts
```

## Thay đổi cụ thể

### Thay thế line 111 (TTL calculation):

**TRƯỚC:**
```typescript
const ttlSec  = Math.min(Number(body['ttl'] ?? 300), 600)   // max 10 min
const expiresAt = Date.now() + ttlSec * 1000
```

**SAU:**
```typescript
const isPermanent = body['permanent'] === true
const THIRTY_DAYS = 30 * 24 * 60 * 60  // 2,592,000 seconds

let ttlSec: number
if (isPermanent) {
  // Long-lived token for production dev servers: 30 days
  // Requires valid ORCA_AGENT_API_SECRET (already checked by isAuthorized)
  ttlSec = THIRTY_DAYS
} else {
  ttlSec = Math.min(Number(body['ttl'] ?? 300), 600)  // ephemeral: max 10 min
}
const expiresAt = Date.now() + ttlSec * 1000
```

## Response body update

Thêm `permanent` flag vào response để caller biết:
```typescript
// Trong sendJson response (sau line ~150):
sendJson(res, 201, {
  token,
  devServerId,
  expiresIn: ttlSec,
  permanent: isPermanent,
  createdAt: Date.now(),
})
```

## Usage example (deploy script)

```bash
# Production dev server — 30-day token:
curl -H "Authorization: Bearer $ORCA_AGENT_API_SECRET" \
  -X POST https://orca.example.com/api/agent-token \
  -H "Content-Type: application/json" \
  -d '{"devServerId":"prod-server-01","name":"Production Server","permanent":true}'

# Response:
# {"token":"agt_...","devServerId":"prod-server-01","expiresIn":2592000,"permanent":true}
```

## Verification

```bash
pnpm tsc --noEmit

# Test: permanent token expires at T+30 days (not T+10min)
# Test: non-permanent token still capped at 600s
```
