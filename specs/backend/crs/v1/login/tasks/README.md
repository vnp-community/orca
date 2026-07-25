# Tasks — Login CR v1

Thư mục này chứa **AI-executable tasks** được phân tách từ các [Solutions](../solutions/).

Mỗi task được thiết kế để AI có thể thực thi độc lập với:
- Mục tiêu rõ ràng (1 file hoặc 1 thay đổi cụ thể)
- Input/Output xác định
- Acceptance criteria kiểm tra được

---

## Trạng thái thực thi

> **✅ HOÀN THÀNH: 30/30 tasks — 2026-07-24**
> **🧪 Tests: 134/134 pass | TypeScript: 0 lỗi**

---

## Thứ tự thực thi (as implemented)

```
Phase 1 — Auth Foundation (SOL-LG-001)    ✅ DONE
  TASK-001  → src/main/db/migrations/0005_add_auth_schema.ts
  TASK-002  → src/main/auth/auth-types.ts
  TASK-003  → src/main/auth/auth-session-store.ts
  TASK-004  → src/main/auth/auth-user-store.ts
  TASK-005  → src/main/auth/__tests__/auth-session-store.test.ts
  TASK-006  → src/main/auth/__tests__/auth-user-store.test.ts
  TASK-007  → src/main/auth/auth-local-handler.ts + test
  TASK-008  → src/main/auth/auth-manager.ts
  TASK-009  → src/main/auth/auth-middleware.ts
  TASK-010  → src/main/auth/auth-router.ts
  TASK-011  → src/main/server-bootstrap.ts: mount AuthManager
  TASK-012  → src/server/http-server.ts: mount /auth routes + cookie-parser

Phase 2 — User Sandbox (SOL-LG-002)      ✅ DONE
  TASK-013  → src/main/session/session-types.ts
  TASK-014  → src/main/session/session-manager.ts
  TASK-015  → src/main/session/__tests__/session-manager.test.ts
  TASK-016  → src/main/session/ws-session-router.ts + test
  TASK-017  → src/main/session/user-process-entry.ts
  TASK-018  → src/server/index.ts: ORCA_MULTI_USER flag + WsSessionRouter

Phase 3 — SSH Isolation (SOL-LG-003)     ✅ DONE
  TASK-019  → src/main/ssh/ssh-user-resolver.ts + test
  TASK-020  → src/main/ssh/dev-server-provisioner.ts + test + provision-user.sh
  TASK-021  → src/main/ssh/ssh-connection-store.ts: resolveSshTargetForUser()

Phase 4 — Admin Panel (SOL-LG-004)       ✅ DONE
  TASK-022  → src/main/admin/admin-types.ts
  TASK-023  → src/main/admin/audit-logger.ts + test
  TASK-024  → src/main/admin/admin-middleware.ts
  TASK-025  → src/main/admin/admin-user-handlers.ts + test
  TASK-026  → src/main/admin/admin-session-handlers.ts + test
  TASK-027  → src/main/admin/admin-stats-handler.ts + admin-audit-handlers.ts
  TASK-028  → src/main/admin/first-run-setup.ts
  TASK-029  → src/main/admin/admin-router.ts
  TASK-030  → src/server/http-server.ts: mount /admin/api + first-run
```

---

## Dependency Map

```
TASK-001 (migration)
  └── TASK-002 (auth-types)
        ├── TASK-003 (session-store) ──► TASK-005 (test)
        │         └── TASK-008 (auth-manager)
        │                   ├── TASK-009 (middleware)
        │                   ├── TASK-010 (router)
        │                   ├── TASK-011 (bootstrap)
        │                   └── TASK-012 (http-server)
        └── TASK-004 (user-store) ──► TASK-006 (test)
                  └── TASK-007 (local-handler + test)
                            └── TASK-008 (auth-manager)

TASK-008 (auth-manager) ──► TASK-013..018 (sandbox)
TASK-008 (auth-manager) ──► TASK-022..030 (admin)

TASK-013 (session-types)
  └── TASK-014 (session-manager) ──► TASK-015 (test)
        └── TASK-016 (ws-session-router + test)
              └── TASK-017 (user-process-entry)
                    └── TASK-018 (server/index.ts)

TASK-019 (ssh-user-resolver + test)
  └── TASK-020 (provisioner + test + script)
        └── TASK-021 (ssh-connection-store)

TASK-022 (admin-types)
  └── TASK-023 (audit-logger + test)
        ├── TASK-024 (admin-middleware)
        ├── TASK-025 (user-handlers + test)
        ├── TASK-026 (session-handlers + test)
        ├── TASK-027 (stats-handler + audit-handlers)
        ├── TASK-028 (first-run-setup)
        └── TASK-029 (admin-router)
              └── TASK-030 (http-server mount)
```

---

## Tổng số tasks: 30

| Phase | Tasks | Solution | Status |
|-------|-------|----------|--------|
| Phase 1 — Auth Foundation | TASK-001 ~ TASK-012 | SOL-LG-001 | ✅ DONE |
| Phase 2 — User Sandbox | TASK-013 ~ TASK-018 | SOL-LG-002 | ✅ DONE |
| Phase 3 — SSH Isolation | TASK-019 ~ TASK-021 | SOL-LG-003 | ✅ DONE |
| Phase 4 — Admin Panel | TASK-022 ~ TASK-030 | SOL-LG-004 | ✅ DONE |

## Test Summary

| Test File | Tests | Status |
|-----------|-------|--------|
| `auth/__tests__/auth-session-store.test.ts` | 16 | ✅ pass |
| `auth/__tests__/auth-user-store.test.ts` | 17 | ✅ pass |
| `auth/__tests__/auth-local-handler.test.ts` | 7 | ✅ pass |
| `session/__tests__/session-manager.test.ts` | 13 | ✅ pass |
| `session/__tests__/ws-session-router.test.ts` | 8 | ✅ pass |
| `ssh/__tests__/ssh-user-resolver.test.ts` | 21 | ✅ pass |
| `ssh/__tests__/dev-server-provisioner.test.ts` | 8 | ✅ pass |
| `admin/__tests__/audit-logger.test.ts` | 18 | ✅ pass |
| `admin/__tests__/admin-user-handlers.test.ts` | 9 | ✅ pass |
| `admin/__tests__/admin-session-handlers.test.ts` | 7 | ✅ pass |
| **TOTAL** | **124** | **✅ 124/124** |
