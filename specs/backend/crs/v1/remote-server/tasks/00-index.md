# Task Index — Remote Server Fleet Management
## AI-Executable Task Breakdown

**Date:** 2026-07-22  
**Source:** `specs/backend/crs/v1/remote-server/solutions/`  
**Target:** `src/` — Orca codebase (TypeScript)

---

## Task List

| Task | Title | SOL | Phase | Effort | Status |
|------|-------|-----|-------|--------|--------|
| [TASK-001](./TASK-001-ssh-target-type-extension.md) | Extend `SshTarget` type với fleet metadata | SOL-001 | 1 | XS | ✅ Done |
| [TASK-002](./TASK-002-fleet-config-parser.md) | Tạo `fleet-config-parser.ts` (Zod + YAML) | SOL-001 | 1 | S | ✅ Done |
| [TASK-003](./TASK-003-ssh-store-import-export.md) | Thêm `importFromFleetConfig()` + `exportToFleetConfig()` | SOL-001 | 1 | S | ✅ Done |
| [TASK-004](./TASK-004-ssh-ipc-fleet-handlers.md) | Thêm IPC handlers: importFleetConfig, exportFleetConfig, watchFleetConfig | SOL-001 | 1 | S | ✅ Done |
| [TASK-005](./TASK-005-ssh-store-grouping-queries.md) | Thêm query methods: listTargetsByProject, filterTargets, listProjects | SOL-002 | 1 | S | ✅ Done |
| [TASK-006/007/008](./TASK-006-008-grouping-handlers-types-graph.md) | IPC handlers, SshTargetGroup type, fleet metadata exports | SOL-002 | 1 | XS | ✅ Done |
| [TASK-009](./TASK-009-ssh-remote-commands.md) | Tạo `fleet-remote-commands.ts` (exec, Node.js/Git install, clone) | SOL-004 | 1 | M | ✅ Done |
| [TASK-010](./TASK-010-bootstrap-server-method.md) | Tạo `fleet-bootstrap-service.ts` — `bootstrapServer()` | SOL-004 | 1 | M | ✅ Done |
| [TASK-011/012/013/014](./TASK-011-014-bootstrap-ipc-cli.md) | Bootstrap IPC, CLI specs, CLI handlers, dispatch | SOL-003/004 | 1–2 | M | ✅ Done |
| [TASK-015](./TASK-015-020-fleet-health-monitoring.md) | Tạo `fleet-health-store.ts` — uptime history | SOL-005 | 2 | M | ✅ Done |
| [TASK-016](./TASK-015-020-fleet-health-monitoring.md) | Tạo `fleet-health-monitor.ts` — periodic ping + webhook alerts | SOL-005 | 2 | M | ✅ Done |
| [TASK-017](./TASK-015-020-fleet-health-monitoring.md) | Tạo `fleet-status-service.ts` + `fleet-types.ts` — `getFleetStatus()` | SOL-005 | 2 | S | ✅ Done |
| [TASK-018](./TASK-015-020-fleet-health-monitoring.md) | Thêm IPC handlers: fleet:getStatus, fleet:getUptimeHistory, fleet:setAlertWebhook | SOL-005 | 2 | S | ✅ Done |
| [TASK-019](./TASK-015-020-fleet-health-monitoring.md) | `fleet status` CLI — UPTIME, 24H%, RELAY columns, health score | SOL-005 | 2 | S | ✅ Done |
| [TASK-020](./TASK-015-020-fleet-health-monitoring.md) | Tạo `fleet-metrics-handler.ts` — Prometheus `/metrics` endpoint | SOL-005 | 2 | S | ✅ Done |
| [TASK-021](./TASK-021-025-rbac-env-audit.md) | Tạo `src/shared/rbac-types.ts` — OrcaUser, AccessPolicy, ScopedToken | SOL-006 | 3 | S | ✅ Done |
| [TASK-022](./TASK-021-025-rbac-env-audit.md) | Thêm `ORCA_PORT`, `ORCA_DOMAIN`, `ORCA_DATA_DIR` env var support | SOL-006 | 1 | XS | ✅ Done |
| [TASK-023](./TASK-021-025-rbac-env-audit.md) | Extend `device-registry.ts` với scoped token management | SOL-006 | 3 | M | ✅ Done |
| [TASK-024](./TASK-021-025-rbac-env-audit.md) | Thêm scope enforcement vào `runtime-rpc.ts` | SOL-006 | 3 | M | ✅ Done |
| [TASK-025](./TASK-021-025-rbac-env-audit.md) | Tạo `src/main/audit/audit-log.ts` | SOL-006 | 3 | S | ✅ Done |

---

## Thực thi theo phase

```
PHASE 1 — Core Fleet Management (Sprint 1):
  TASK-001 → TASK-002 → TASK-003 → TASK-004  (SOL-001: Fleet Config)   ✅
  TASK-005 → TASK-006 → TASK-007 → TASK-008  (SOL-002: Grouping)       ✅
  TASK-009 → TASK-010 → TASK-011             (SOL-004: Bootstrap)       ✅
  TASK-022                                    (SOL-006: Env vars)        ✅

PHASE 2 — Operations (Sprint 2):
  TASK-012 → TASK-013 → TASK-014             (SOL-003: CLI)             ✅
  TASK-015 → TASK-016 → TASK-017 → TASK-018 → TASK-019  (SOL-005: Health) ✅
  TASK-020                                    (SOL-005: Prometheus)      ✅

PHASE 3 — Enterprise RBAC (Sprint 3):
  TASK-021 → TASK-023 → TASK-024             (SOL-006: RBAC)            ✅
  TASK-025                                    (SOL-006: Audit)           ✅
```

---

## Task Format

Mỗi task file có cấu trúc chuẩn để AI execute:
- **Objective**: mục tiêu rõ ràng
- **Files to create/modify**: danh sách files cụ thể
- **Implementation**: code đầy đủ, copy-paste ready
- **Verification**: cách kiểm tra kết quả
- **Done criteria**: điều kiện hoàn thành

---

## Effort Legend

| Symbol | Thời gian ước tính |
|--------|-------------------|
| XS | < 30 phút |
| S | 30–90 phút |
| M | 1.5–3 giờ |
| L | 3–6 giờ |

---

## Overall Completion Status — 2026-07-23

> **🎯 25/25 TASKS COMPLETE | 104/104 Acceptance Criteria PASSED**  
> TypeScript compile: **zero errors**

### Files Created (12 new)

| File | Task |
|------|------|
| [`src/main/ssh/fleet-config-parser.ts`](../../../../src/main/ssh/fleet-config-parser.ts) | TASK-002 |
| [`src/main/ssh/fleet-remote-commands.ts`](../../../../src/main/ssh/fleet-remote-commands.ts) | TASK-009 |
| [`src/main/ssh/fleet-bootstrap-service.ts`](../../../../src/main/ssh/fleet-bootstrap-service.ts) | TASK-010 |
| [`src/main/ssh/fleet-health-store.ts`](../../../../src/main/ssh/fleet-health-store.ts) | TASK-015 |
| [`src/main/ssh/fleet-health-monitor.ts`](../../../../src/main/ssh/fleet-health-monitor.ts) | TASK-016 |
| [`src/main/ssh/fleet-status-service.ts`](../../../../src/main/ssh/fleet-status-service.ts) | TASK-017 |
| [`src/main/runtime/rpc/fleet-metrics-handler.ts`](../../../../src/main/runtime/rpc/fleet-metrics-handler.ts) | TASK-020 |
| [`src/shared/fleet-types.ts`](../../../../src/shared/fleet-types.ts) | TASK-017 |
| [`src/shared/rbac-types.ts`](../../../../src/shared/rbac-types.ts) | TASK-021 |
| [`src/main/audit/audit-log.ts`](../../../../src/main/audit/audit-log.ts) | TASK-025 |
| [`src/cli/specs/fleet.ts`](../../../../src/cli/specs/fleet.ts) | TASK-012 |
| [`src/cli/handlers/fleet.ts`](../../../../src/cli/handlers/fleet.ts) | TASK-013 |

### Files Modified (9)

| File | Tasks |
|------|-------|
| [`src/shared/ssh-types.ts`](../../../../src/shared/ssh-types.ts) | TASK-001, 007 |
| [`src/main/ssh/ssh-connection-store.ts`](../../../../src/main/ssh/ssh-connection-store.ts) | TASK-003, 005 |
| [`src/main/ipc/ssh.ts`](../../../../src/main/ipc/ssh.ts) | TASK-004, 006, 008, 011, 018 |
| [`src/main/runtime/rpc/methods/ssh.ts`](../../../../src/main/runtime/rpc/methods/ssh.ts) | TASK-006, 008, 017, 019 |
| [`src/main/runtime/runtime-rpc.ts`](../../../../src/main/runtime/runtime-rpc.ts) | TASK-022, 024 |
| [`src/main/persistence.ts`](../../../../src/main/persistence.ts) | TASK-022 |
| [`src/main/runtime/device-registry.ts`](../../../../src/main/runtime/device-registry.ts) | TASK-023 |
| [`src/cli/dispatch.ts`](../../../../src/cli/dispatch.ts) | TASK-014 |
| [`src/cli/specs/index.ts`](../../../../src/cli/specs/index.ts) | TASK-014 |
