# Backend Solutions — Remote Server Management CRs (v1)
## Index

**Version:** 1.1  
**Date:** 2026-07-22  
**Scope:** Backend implementation solutions cho CR-001 → CR-006 (remote server fleet management)  
**Basis:** TDD Backend (`specs/backend/tdd/`)

---

## Tóm tắt giải pháp

| CR | Tiêu đề | Backend Changes | Effort | Phase | Status |
|----|---------|----------------|--------|-------|--------|
| [CR-001](./SOL-001-fleet-inventory-config.md) | Fleet Inventory Config | `ssh-types.ts` + `ssh-connection-store.ts` + IPC handler | M | 1 | ✅ Done |
| [CR-002](./SOL-002-server-grouping.md) | Server Grouping | Grouping queries + IPC handlers | S | 1 | ✅ Done |
| [CR-003](./SOL-003-bulk-provisioning.md) | Bulk Provisioning | CLI `orca fleet` + parallel relay deploy | L | 2 | ✅ Done |
| [CR-004](./SOL-004-bootstrap-automation.md) | Bootstrap Automation | SSH remote commands + `fleet bootstrap` CLI | M | 1 | ✅ Done |
| [CR-005](./SOL-005-fleet-health-monitoring.md) | Fleet Health Monitoring | Fleet status CLI + IPC + Prometheus metrics | M | 2 | ✅ Done |
| [CR-006](./SOL-006-team-rbac.md) | Team RBAC | Multi-instance (immediate) + OIDC/policy layer (long-term) | L | 3 | ⚡ Partial (Ph.1+2 done) |

---

## Dependency Graph

```
CR-001 (Fleet Inventory Schema)
  ├─── CR-002 (Grouping) — depends on CR-001 schema extension
  ├─── CR-003 (Bulk Provision) — depends on CR-001 import
  ├─── CR-004 (Bootstrap) — depends on CR-001 fleet config
  └─── CR-005 (Health Monitor) — depends on CR-001 + CR-002
         └─── CR-006 (RBAC) — depends on CR-001 + CR-002
```

---

## Backend Architecture Layers bị tác động

Căn cứ theo `specs/backend/tdd/`:

| Layer (TDD) | CR ảnh hưởng |
|-------------|-------------|
| `06-persistence.md` — SQLite schema | CR-001, CR-002, CR-006 |
| `07-runtime-service.md` — OrcaRuntimeService | CR-001, CR-003, CR-005 |
| `05-ssh-relay.md` — SSH & Relay | CR-003, CR-004 |
| `09-ipc-handlers.md` — IPC | CR-001, CR-002, CR-003, CR-005 |
| `04-rpc-server.md` — RPC Security | CR-006 |
| `02-main-process.md` — CLI | CR-003, CR-004, CR-005 |

---

## Implementation Order (khuyến nghị)

```
Sprint 1:  CR-001 → CR-002 → CR-004  (Phase 1 core)
Sprint 2:  CR-003 → CR-005            (Phase 2 operations)
Sprint 3:  CR-006                     (Phase 3 enterprise)
```

---

## Overall Implementation Status — 2026-07-22

> **🎯 25/25 TASKS COMPLETE** | Backend CR-001 → CR-006 (Phase 1 & 2) fully implemented

### New Files Created (9)

| File | CR | Purpose |
|------|----|---------|
| [`src/main/ssh/fleet-config-parser.ts`](../../../../src/main/ssh/fleet-config-parser.ts) | CR-001 | YAML fleet config parsing + Zod validation |
| [`src/main/ssh/fleet-remote-commands.ts`](../../../../src/main/ssh/fleet-remote-commands.ts) | CR-004 | Remote SSH commands: Node/Git install, repo clone |
| [`src/main/ssh/fleet-bootstrap-service.ts`](../../../../src/main/ssh/fleet-bootstrap-service.ts) | CR-004 | Bootstrap orchestration (7-step pipeline) |
| [`src/main/ssh/fleet-health-store.ts`](../../../../src/main/ssh/fleet-health-store.ts) | CR-005 | In-memory health history, uptime calculation |
| [`src/main/ssh/fleet-health-monitor.ts`](../../../../src/main/ssh/fleet-health-monitor.ts) | CR-005 | Periodic poll, webhook + IPC alerts |
| [`src/main/ssh/fleet-status-service.ts`](../../../../src/main/ssh/fleet-status-service.ts) | CR-005 | `getFleetStatus()` standalone service |
| [`src/main/runtime/rpc/fleet-metrics-handler.ts`](../../../../src/main/runtime/rpc/fleet-metrics-handler.ts) | CR-005 | Prometheus `/metrics` endpoint factory |
| [`src/shared/fleet-types.ts`](../../../../src/shared/fleet-types.ts) | CR-004/005 | `FleetServerStatus`, `FleetStatusReport` shared types |
| [`src/shared/rbac-types.ts`](../../../../src/shared/rbac-types.ts) | CR-006 | RBAC types: OrcaUser, OrcaAccessPolicy, ScopedPairingToken |
| [`src/main/audit/audit-log.ts`](../../../../src/main/audit/audit-log.ts) | CR-006 | NDJSON audit log with `record()` + `query()` |
| [`src/cli/specs/fleet.ts`](../../../../src/cli/specs/fleet.ts) | CR-003 | 6 CLI fleet command specs |
| [`src/cli/handlers/fleet.ts`](../../../../src/cli/handlers/fleet.ts) | CR-003 | CLI fleet handlers (`FLEET_HANDLERS` map) |

### Modified Files (5)

| File | CRs | Changes |
|------|-----|---------|
| [`src/shared/ssh-types.ts`](../../../../src/shared/ssh-types.ts) | CR-001/002 | `SshTarget` extended, `SshTargetGroup` type, grouping function |
| [`src/main/ssh/ssh-connection-store.ts`](../../../../src/main/ssh/ssh-connection-store.ts) | CR-001/002 | import/export fleet, 5 grouping query methods |
| [`src/main/ipc/ssh.ts`](../../../../src/main/ipc/ssh.ts) | CR-001/002/003/004/005 | 12+ new IPC handlers |
| [`src/main/runtime/rpc/methods/ssh.ts`](../../../../src/main/runtime/rpc/methods/ssh.ts) | CR-001/002/003/004/005 | 8 new RPC methods |
| [`src/main/runtime/runtime-rpc.ts`](../../../../src/main/runtime/runtime-rpc.ts) | CR-006 | `ORCA_PORT`, `ORCA_DOMAIN` env vars, scoped token auth |
| [`src/main/persistence.ts`](../../../../src/main/persistence.ts) | CR-006 | `ORCA_DATA_DIR` env var |
| [`src/main/runtime/device-registry.ts`](../../../../src/main/runtime/device-registry.ts) | CR-006 | 5 scoped token methods |
| [`src/cli/dispatch.ts`](../../../../src/cli/dispatch.ts) | CR-003 | Register `FLEET_HANDLERS` |
| [`src/cli/specs/index.ts`](../../../../src/cli/specs/index.ts) | CR-003 | Register `FLEET_COMMAND_SPECS` |

### Pending (không block CR delivery)

- [x] CR-005: Wire `fleetHealthMonitor.start()` vào app startup ✅ (2026-07-24 — `server-bootstrap.ts`)
- [x] CR-005: Wire `createFleetMetricsHandler()` vào RPC HTTP server ✅ (2026-07-24 — `ws-transport.ts` + `runtime-rpc.ts`)
- [x] CR-005: Persist `fleetAlertWebhookUrl` vào GlobalSettings ✅ (2026-07-24 — `src/shared/types.ts`)
- [ ] CR-006: Phase 3 — OIDC/SSO handler (`src/main/auth/oidc-handler.ts`) — **DEFERRED** (future sprint)
- [ ] CR-006: Fleet config `access:` block parsing — **DEFERRED** (future sprint)
