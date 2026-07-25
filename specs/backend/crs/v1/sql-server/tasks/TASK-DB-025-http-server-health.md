# TASK-DB-025: Cập nhật `src/server/http-server.ts` — expose health routes ✅ DONE

**Source:** SOL-DB-006 §4.4  
**Phase:** 4 | **Effort:** S (30–45 min) | **Status:** ✅ COMPLETED 2026-07-23  
**Depends on:** TASK-DB-023

---

## Objective

Cập nhật `src/server/http-server.ts` để accept optional `dbMonitor` và route `/health*` paths trước khi serve static files.

---

## Context cần đọc TRƯỚC

```bash
cat src/server/http-server.ts
```

---

## Modification Pattern

```typescript
// Thêm import:
import type { HealthChecker } from '../main/db/health'
import { createHealthEndpoint } from './health-endpoint'

// Thêm vào options interface (nếu có) hoặc function signature:
export interface HttpServerOptions {
  webRoot: string
  port: number
  dbMonitor?: HealthChecker  // ← NEW
}

// Trong createServer handler, thêm TRƯỚC static file serving:
const healthHandler = options.dbMonitor
  ? createHealthEndpoint(options.dbMonitor, { includePoolStats: true })
  : null

// ...trong request handler:
if (url.pathname.startsWith('/health') && healthHandler) {
  await healthHandler(req, res)
  return
}
// ... existing static file logic ...
```

---

## Verification

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca

# TypeScript compile
npx tsc --noEmit 2>&1 | grep "http-server" | head -10

# Test health routes via HTTP (requires running server)
# curl http://localhost:6769/health
# curl http://localhost:6769/health/ready
# curl http://localhost:6769/health/metrics

# Run http-server unit tests
pnpm vitest run src/server/__tests__/http-server.test.ts 2>/dev/null || echo "No test file"
```

---

## Done criteria

- [x] `src/server/http-server.ts` accept `dbMonitor?: HealthChecker` option
- [x] `/health` routes handled TRƯỚC static file serving
- [x] Khi `dbMonitor` không được cung cấp, health routes không được expose
- [x] Existing static file serving behavior không thay đổi
- [x] TypeScript compile không có lỗi mới
