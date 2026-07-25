# Task Index — Remote Server Frontend Implementation

**Version:** 1.0  
**Date:** 2026-07-22  
**Scope:** Frontend tasks cho 6 Change Requests (CR-001 → CR-006)  
**Source:** `specs/frontend/crs/v1/remote-server/solutions/`  
**Target codebase:** `src/renderer/src/` + `src/preload/` + `src/shared/`

---

## Quy tắc cho AI khi thực thi

1. **Đọc solution file trước** — mỗi task có link đến solution tương ứng
2. **Kiểm tra file tồn tại** trước khi tạo mới (tránh overwrite)
3. **Không hardcode** — dùng `translate()` cho tất cả string user-facing
4. **Dual-target** — code phải chạy cả Desktop (Electron) và Web (browser)
5. **Zustand pattern** — state trong slice, không useState local cho shared state
6. **Import paths** — dùng `@/` prefix (e.g. `@/store`, `@/components/ui/button`)
7. **Test sau mỗi task** — TypeScript build check (`tsc --noEmit`)

---

## Dependency Graph

```
TASK-001-A (SshSlice types)
    └─→ TASK-001-B (IPC preload)
    └─→ TASK-001-C (FleetImportDialog)
    └─→ TASK-001-D (useIpcEvents fleet handler)
    └─→ TASK-001-E (SshSettingsPanel button)

TASK-002-A (SshSlice grouping state) [depends: TASK-001-A]
    └─→ TASK-002-B (Selectors)
    └─→ TASK-002-C (FleetFilterBar)
    └─→ TASK-002-D (SshTargetGroupedList)
    └─→ TASK-002-E (SshTargetGroup + Row)
    └─→ TASK-002-F (Sidebar SshStatusSection)

TASK-003-A (ProvisioningSlice) [depends: TASK-001-A, TASK-002-A]
    └─→ TASK-003-B (IPC provisioning events)
    └─→ TASK-003-C (FleetProvisionWizard)
    └─→ TASK-003-D (ProvisionProgressPanel)

TASK-004-A (BootstrapSlice) [depends: TASK-001-A]
    └─→ TASK-004-B (IPC bootstrap events)
    └─→ TASK-004-C (ServerBootstrapPanel)
    └─→ TASK-004-D (BootstrapStepList + LogViewer)

TASK-005-A (Health metrics in SshSlice) [depends: TASK-002-A]
    └─→ TASK-005-B (useFleetHealthPolling hook)
    └─→ TASK-005-C (FleetHealthDashboard)
    └─→ TASK-005-D (FleetHealthTable + Alerts)

TASK-006-A (OrcaInstanceSwitcher — Phase 1)
TASK-006-B (AuthSlice + Login — Phase 2) [depends: TASK-001-A]
```

---

## Danh sách Tasks theo Phase

### Phase 1 — Foundation

| Task ID | Mô tả | File | CR | Status |
|---------|-------|------|----|--------|
| [TASK-001-A](./TASK-001-A-ssh-slice-types.md) | Mở rộng SshSlice types | ssh.ts, types.ts | CR-001 | ✅ DONE |
| [TASK-001-B](./TASK-001-B-ipc-fleet-api.md) | Expose fleet API qua preload | preload/index.ts, web-preload-api.ts | CR-001 | ✅ DONE |
| [TASK-001-C](./TASK-001-C-fleet-import-dialog.md) | Tạo FleetImportDialog + FleetImportProgress | components/settings/ssh/ | CR-001 | ✅ DONE |
| [TASK-001-D](./TASK-001-D-ipc-events-fleet.md) | Thêm fleet IPC event handlers | useIpcEvents.ts | CR-001 | ✅ DONE |
| [TASK-001-E](./TASK-001-E-ssh-settings-panel.md) | Cập nhật SshSettingsPanel | SshSettingsPanel.tsx | CR-001 | ✅ DONE |
| [TASK-002-A](./TASK-002-A-ssh-slice-grouping.md) | Thêm grouping state vào SshSlice | ssh.ts | CR-002 | ✅ DONE |
| [TASK-002-B](./TASK-002-B-selectors.md) | Tạo selectors cho grouping/filtering | store/selectors.ts | CR-002 | ✅ DONE |
| [TASK-002-C](./TASK-002-C-fleet-filter-bar.md) | Tạo FleetFilterBar component | components/settings/ssh/ | CR-002 | ✅ DONE |
| [TASK-002-D](./TASK-002-D-ssh-grouped-list.md) | Tạo SshTargetGroupedList | components/settings/ssh/ | CR-002 | ✅ DONE |
| [TASK-002-E](./TASK-002-E-ssh-target-group-row.md) | Tạo SshTargetGroup + cập nhật SshTargetRow | components/settings/ssh/ | CR-002 | ✅ DONE |
| [TASK-002-F](./TASK-002-F-sidebar-ssh-status.md) | Tạo SshStatusSection trong Sidebar | components/sidebar/ | CR-002 | ✅ DONE |

### Phase 2 — Operations

| Task ID | Mô tả | File | CR | Status |
|---------|-------|------|----|--------|
| [TASK-003-A](./TASK-003-A-provisioning-slice.md) | Tạo ProvisioningSlice | store/slices/provisioning.ts | CR-003 | ✅ DONE |
| [TASK-003-B](./TASK-003-B-provisioning-ipc.md) | Provisioning IPC events | useIpcEvents.ts, preload | CR-003 | ✅ DONE |
| [TASK-003-C](./TASK-003-C-provision-wizard.md) | Tạo FleetProvisionWizard + Selector | components/settings/ssh/ | CR-003 | ✅ DONE |
| [TASK-003-D](./TASK-003-D-provision-progress.md) | Tạo ProvisionProgressPanel + ConfirmStep | components/settings/ssh/ | CR-003 | ✅ DONE |
| [TASK-004-A](./TASK-004-A-bootstrap-slice.md) | Tạo BootstrapSlice | store/slices/bootstrap.ts | CR-004 | ✅ DONE |
| [TASK-004-B](./TASK-004-B-bootstrap-ipc.md) | Bootstrap IPC events | useIpcEvents.ts, preload | CR-004 | ✅ DONE |
| [TASK-004-C](./TASK-004-C-bootstrap-panel.md) | Tạo ServerBootstrapPanel + IdleScreen | components/settings/ssh/ | CR-004 | ✅ DONE |
| [TASK-004-D](./TASK-004-D-bootstrap-steps-log.md) | Tạo BootstrapStepList + LogViewer | components/settings/ssh/ | CR-004 | ✅ DONE |

### Phase 3 — Monitoring & Security

| Task ID | Mô tả | File | CR | Status |
|---------|-------|------|----|--------|
| [TASK-005-A](./TASK-005-A-health-slice.md) | Thêm health metrics vào SshSlice | store/slices/ssh.ts | CR-005 | ✅ DONE |
| [TASK-005-B](./TASK-005-B-health-polling.md) | Tạo useFleetHealthPolling hook | hooks/ | CR-005 | ✅ DONE |
| [TASK-005-C](./TASK-005-C-health-dashboard.md) | Tạo FleetHealthDashboard | components/settings/ssh/ | CR-005 | ✅ DONE |
| [TASK-005-D](./TASK-005-D-health-table-alerts.md) | Tạo FleetHealthTable + FleetAlertStrip | components/settings/ssh/ | CR-005 | ✅ DONE |
| [TASK-006-A](./TASK-006-A-instance-switcher.md) | Tạo OrcaInstanceSwitcher (Phase 1) | web/ | CR-006 | ✅ DONE |
| [TASK-006-B](./TASK-006-B-auth-slice-login.md) | Tạo AuthSlice + OrcaLoginScreen (Phase 2) | store/slices/, web/ | CR-006 | ✅ DONE |

---

## Trạng thái Legend

- ✅ DONE — Chưa bắt đầu
- 🔄 IN PROGRESS — Đang thực hiện
- ✅ DONE — Hoàn thành
- ❌ BLOCKED — Bị block bởi dependency
