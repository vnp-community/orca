# TASK-FE2E-003 — Cập nhật comment `installAuthFailedRedirect` trong `main-web-bootstrap.tsx`

**Source Solution:** [SOL-FE2E-002](../solutions/SOL-FE2E-002-remove-paircode-fallback-from-login.md) §2.2
**Priority:** P1
**Loại:** Sửa comment (không đổi logic)
**Depends on:** TASK-FE2E-001
**Estimated:** 5 phút
**Status:** ✅ DONE — 2026-08-09

---

## Context

```bash
grep -n "installAuthFailedRedirect\|E2EE-paired environments" frontend/src/renderer/src/web/main-web-bootstrap.tsx
```

## Thay đổi cần thực hiện

**File:** `frontend/src/renderer/src/web/main-web-bootstrap.tsx`

**TÌM:**
```ts
 * Guards: only runs once (redirected flag), only for session-auth environments
 * (E2EE-paired environments should reconnect, not logout).
```

**THAY BẰNG:**
```ts
 * Guards: only runs once (redirected flag), only for session-auth environments.
 * (E2EE pairing is no longer reachable from the multi-user bootstrap path —
 * see CR-FE2E-002 — this check is now a defensive no-op for that path, kept
 * because bootstrapWebApp() and main.tsx's WebRoot still share this file's
 * exported helpers with tests.)
```

> [!IMPORTANT]
> Chỉ sửa comment — hàm `installAuthFailedRedirect()` và logic `env?.id !== 'session-auth'` giữ nguyên 100%, không đổi 1 dòng code thực thi nào.

## Verify

```bash
cd frontend
node_modules/.bin/vitest run --config config/vitest.config.ts src/renderer/src/web/__tests__/main-web-bootstrap.test.ts 2>/dev/null || \
node_modules/.bin/vitest run --config config/vitest.config.ts src/renderer/src/web -t "auth-failed"
```

## Definition of Done

- [x] Comment cập nhật đúng nội dung trên
- [x] Không có thay đổi nào khác trong file (diff chỉ gồm comment)
- [x] Test hiện có của `main-web-bootstrap` không đổi kết quả (xác nhận ở TASK-FE2E-005)
