# Frontend Solutions — Remote Server Change Requests (v1)

**Version:** 1.1  
**Date:** 2026-07-22 (Created) / 2026-07-23 (Implemented)  
**Scope:** Frontend (Renderer process + Store + IPC layer)  
**TDD Reference:** `specs/frontend/tdd/`  
**Implementation Status:** ✅ All 6 CRs implemented and TypeScript-verified

---

## Tổng quan

Bộ tài liệu này mô tả **giải pháp frontend** cho các Change Requests quản lý remote server fleet trong Orca IDE.

---

## Danh sách Solutions

| CR | Tiêu đề | File Solution | Trạng thái | Tasks |
|----|---------|---------------|------------|-------|
| CR-001 | Fleet Inventory Config File | [SOL-CR-001.md](./SOL-CR-001-fleet-inventory-config.md) | ✅ Implemented | TASK-001-A/B/C/D/E |
| CR-002 | Server Grouping by Project/Team | [SOL-CR-002.md](./SOL-CR-002-server-grouping-by-project.md) | ✅ Implemented | TASK-002-A/B/C/D/E/F |
| CR-003 | Bulk Server Provisioning | [SOL-CR-003.md](./SOL-CR-003-bulk-provisioning.md) | ✅ Implemented | TASK-003-A/B/C/D |
| CR-004 | Dev Server Bootstrap Automation | [SOL-CR-004.md](./SOL-CR-004-dev-server-bootstrap.md) | ✅ Implemented | TASK-004-A/B/C/D |
| CR-005 | Fleet Health Monitoring | [SOL-CR-005.md](./SOL-CR-005-fleet-health-monitoring.md) | ✅ Implemented | TASK-005-A/B/C/D |
| CR-006 | Team-based Access Control (RBAC) | [SOL-CR-006.md](./SOL-CR-006-team-rbac.md) | ✅ Implemented (Phase 1 + Phase 2 frontend) | TASK-006-A/B |

---

## Kiến trúc frontend áp dụng

Theo TDD hiện tại, mọi solution phải tuân thủ:

```
Frontend Layer Stack:
┌─────────────────────────────────────────────────┐
│  UI Components (React + shadcn/ui + Tailwind)    │
│  src/renderer/src/components/                    │
├─────────────────────────────────────────────────┤
│  Custom Hooks (useXxx)                           │
│  src/renderer/src/hooks/                         │
├─────────────────────────────────────────────────┤
│  Zustand Store (slices)                          │
│  src/renderer/src/store/slices/                  │
├─────────────────────────────────────────────────┤
│  Runtime Client Layer (callRuntimeRpc / window.api)│
│  src/renderer/src/runtime/                       │
├─────────────────────────────────────────────────┤
│  IPC / WebSocket (window.api.ssh.*)              │
│  src/preload/index.ts + web-preload-api.ts        │
└─────────────────────────────────────────────────┘
```

### Nguyên tắc thiết kế bắt buộc

1. **Dual render target**: Mọi component phải hoạt động cả Desktop (Electron) lẫn Web (browser)
2. **Zustand slices**: State mới phải tổ chức theo slice pattern
3. **window.api abstraction**: Mọi backend call qua `window.api.*`
4. **i18n first**: Mọi user-facing text phải qua `translate()` hoặc `t()`
5. **Lazy loading**: Heavy components phải dùng `lazyWithRetry()`
6. **IPC events**: Sự kiện từ backend xử lý trong `useIpcEvents()` hook
7. **scheduleRuntimeGraphSync()**: Gọi sau mọi mutating operation

---

## Thứ tự implement đề xuất

```
Phase 1 — Foundation (CR-001 + CR-002):
  SOL-CR-001: SshSlice mở rộng + FleetConfig import UI
  SOL-CR-002: Server grouping UI + filter panel

Phase 2 — Operations (CR-003 + CR-004):
  SOL-CR-003: Bulk provisioning wizard + progress tracking
  SOL-CR-004: Bootstrap automation UI + status tracking

Phase 3 — Monitoring & Security (CR-005 + CR-006):
  SOL-CR-005: Fleet health dashboard + real-time status
  SOL-CR-006: Multi-instance deployment UI + RBAC (long-term)
```
