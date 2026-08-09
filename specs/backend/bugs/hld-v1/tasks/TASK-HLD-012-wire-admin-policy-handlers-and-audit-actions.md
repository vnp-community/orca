# TASK-HLD-012: Thêm audit actions cho Policy + wiring `AdminPolicyHandlers` trong `http-server.ts`

**Priority:** 🔴 CRITICAL — Bắt buộc để TASK-HLD-011 compile và chạy được
**Effort:** ~10 phút
**Status:** ✅ DONE — 2026-08-09 (3 audit action thêm vào `admin-types.ts`; `AdminPolicyHandlers` wired vào `http-server.ts`. `tsc --noEmit` sạch hoàn toàn cho cả 4 file liên quan (admin-policy-handlers.ts, admin-router.ts, admin-types.ts, http-server.ts) — BUG-BE-HLD-006 và BUG-BE-HLD-007 đã fix xong end-to-end, compile sạch, không chỉ file tồn tại.)
**Bug refs:** BUG-BE-HLD-007
**Solution ref:** [SOLUTION-admin-panel-exact.md](../solutions/SOLUTION-admin-panel-exact.md)
**Depends on:** TASK-HLD-011

---

## Mục tiêu

`AdminPolicyHandlers` (tạo ở TASK-HLD-011) import `AUDIT_ACTIONS.POLICY_CREATE/UPDATE/DELETE` từ `admin-types.ts` nhưng các hằng số này chưa tồn tại — cần thêm. Route `/policies*` đã mount trong `admin-router.ts` nhưng chưa có nơi nào gọi `createAdminRouter({..., policyHandlers: ...})` — nếu bỏ qua bước này, TypeScript sẽ báo lỗi thiếu field bắt buộc `policyHandlers`. Đây là điểm nối bắt buộc để BUG-BE-HLD-007 thực sự fix xong (không chỉ file tồn tại mà còn phải chạy được).

## File cần sửa/tạo

- `backend/src/main/admin/admin-types.ts` — thêm 3 audit action `POLICY_CREATE`/`POLICY_UPDATE`/`POLICY_DELETE`
- `backend/src/server/http-server.ts` — import `AdminPolicyHandlers` + instantiate trong `createAdminRouter({...})`

## Thay đổi cụ thể

### 1. `backend/src/main/admin/admin-types.ts`

Vị trí: object `AUDIT_ACTIONS` (dòng 59–72), thêm 3 dòng ngay sau `SESSION_KILL_ALL`.

Code thật hiện tại:

```typescript
export const AUDIT_ACTIONS = {
  LOGIN_SUCCESS:    'login.success',
  LOGIN_FAILURE:    'login.failure',
  LOGOUT:           'logout',
  SSO_LOGIN:        'sso.login',
  USER_CREATE:      'user.create',
  USER_DEACTIVATE:  'user.deactivate',
  SESSION_KILL:     'session.kill',
  SESSION_KILL_ALL: 'session.kill_all',
  SSH_CONNECT:      'ssh.connect',
  SSH_DISCONNECT:   'ssh.disconnect',
  SERVER_START:     'server.start',
  SERVER_STOP:      'server.stop',
} as const
```

Fix — thêm (không xoá gì, chỉ chèn thêm 3 field trước dòng đóng `} as const`):

```typescript
export const AUDIT_ACTIONS = {
  LOGIN_SUCCESS:    'login.success',
  LOGIN_FAILURE:    'login.failure',
  LOGOUT:           'logout',
  SSO_LOGIN:        'sso.login',
  USER_CREATE:      'user.create',
  USER_DEACTIVATE:  'user.deactivate',
  SESSION_KILL:     'session.kill',
  SESSION_KILL_ALL: 'session.kill_all',
  SSH_CONNECT:      'ssh.connect',
  SSH_DISCONNECT:   'ssh.disconnect',
  SERVER_START:     'server.start',
  SERVER_STOP:      'server.stop',
  // FIX BUG-BE-HLD-007: audit trail cho Access Policy CRUD
  POLICY_CREATE:    'policy.create',
  POLICY_UPDATE:    'policy.update',
  POLICY_DELETE:    'policy.delete',
} as const
```

`PolicyInput` type (dòng 41–52) đã tồn tại đúng như cần — **không cần đổi**.

### 2. `backend/src/server/http-server.ts`

Lines liên quan: import block (dòng 23–28) và `createAdminRouter({...})` call (dòng 99–111).

Code thật hiện tại:

```typescript
import { createAdminRouter }    from '../main/admin/admin-router'
import { AdminUserHandlers }    from '../main/admin/admin-user-handlers'
import { AdminSessionHandlers } from '../main/admin/admin-session-handlers'
import { AdminStatsHandler }    from '../main/admin/admin-stats-handler'
import { AdminAuditHandlers }   from '../main/admin/admin-audit-handlers'
import { AuditLogger }          from '../main/admin/audit-logger'
```

```typescript
      const auditLogger  = new AuditLogger(adminDb)
      const adminRouter  = createAdminRouter({
        userHandlers:    new AdminUserHandlers({
          userStore:    options.authManager.userStore,
          sessionStore: options.authManager.sessionStore,
          auditLogger
        }),
        sessionHandlers: new AdminSessionHandlers({
          sessionStore: options.authManager.sessionStore,
          auditLogger
        }),
        statsHandler:  new AdminStatsHandler(adminDb),
        auditHandlers: new AdminAuditHandlers(auditLogger)
      })
```

Fix:

```typescript
import { createAdminRouter }    from '../main/admin/admin-router'
import { AdminUserHandlers }    from '../main/admin/admin-user-handlers'
import { AdminSessionHandlers } from '../main/admin/admin-session-handlers'
import { AdminStatsHandler }    from '../main/admin/admin-stats-handler'
import { AdminAuditHandlers }   from '../main/admin/admin-audit-handlers'
import { AdminPolicyHandlers }  from '../main/admin/admin-policy-handlers'   // FIX BUG-BE-HLD-007
import { AuditLogger }          from '../main/admin/audit-logger'
```

```typescript
      const auditLogger  = new AuditLogger(adminDb)
      const adminRouter  = createAdminRouter({
        userHandlers:    new AdminUserHandlers({
          userStore:    options.authManager.userStore,
          sessionStore: options.authManager.sessionStore,
          auditLogger
        }),
        sessionHandlers: new AdminSessionHandlers({
          sessionStore: options.authManager.sessionStore,
          auditLogger
        }),
        statsHandler:  new AdminStatsHandler(adminDb),
        auditHandlers: new AdminAuditHandlers(auditLogger),
        // FIX BUG-BE-HLD-007: adminDb đã là ISyncDatabase — cùng instance AdminStatsHandler đang dùng
        policyHandlers: new AdminPolicyHandlers({ db: adminDb, auditLogger })
      })
```

Không cần thay đổi gì khác trong `http-server.ts` — `adminDb` (kiểu `ISyncDatabase`, dòng 96) đã sẵn có trong scope này.

## Verification

```bash
pnpm tsc --noEmit
# Expected: không còn lỗi thiếu field `policyHandlers` hoặc thiếu AUDIT_ACTIONS.POLICY_*

grep -n "POLICY_CREATE\|POLICY_UPDATE\|POLICY_DELETE" backend/src/main/admin/admin-types.ts
grep -n "AdminPolicyHandlers" backend/src/server/http-server.ts

# Smoke test end-to-end sau khi build (server chạy local, đã login admin, cookie trong session.txt)
curl -s -b session.txt http://localhost:6768/admin/api/policies
# Expected: { "policies": [], "total": 0 }  (hoặc danh sách nếu đã seed)

curl -s -b session.txt -X POST http://localhost:6768/admin/api/policies \
  -H "Content-Type: application/json" \
  -d '{"name":"test-policy"}'
# Expected: HTTP 201, body có id/name/teams/... theo shape OrcaAccessPolicy

grep -n "\"action\":\"policy.create\"" # kiểm tra dòng audit log tương ứng đã ghi vào orca_audit_log
```

Sau task này, cả 2 bug BUG-BE-HLD-006 và BUG-BE-HLD-007 đều đã fix xong end-to-end (file tồn tại + wiring hoạt động, không chỉ compile).
