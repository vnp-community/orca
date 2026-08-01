# CR-000 — Tổng quan: Dev Server Management Gap Analysis

**Ngày:** 2026-07-22  
**Tác giả:** VNP-BLC DevOps Team  
**Status:** Implemented  
**Priority:** High  

---

## 1. Bối cảnh

Orca là AI-native IDE với khả năng kết nối remote server qua SSH. Tuy nhiên, sau khi khảo sát toàn bộ codebase (`src/main/ssh/`, `src/shared/ssh-types.ts`, `orca.yaml`, v.v.), xác nhận rằng **Orca hiện chưa có cấu hình hay cơ chế tự động để quản lý fleet các dev server**.

Mô hình hiện tại của Orca là **per-developer, per-connection** — mỗi developer phải thêm thủ công từng SSH target qua UI hoặc import `~/.ssh/config`. Không có cơ chế trung tâm để:

- Khai báo danh sách dev servers theo project/team
- Tự động provision server khi mới thêm vào fleet
- Monitor health của toàn bộ fleet
- Quản lý quyền truy cập theo team/role

---

## 2. Khảo sát hiện trạng

### 2.1 Những gì Orca ĐÃ có

| Tính năng | File / Module | Mức độ |
|-----------|---------------|--------|
| SSH Target CRUD (thủ công) | `ssh-connection-store.ts` | ✅ Đầy đủ |
| Import từ `~/.ssh/config` | `ssh-config-parser.ts`, `importFromSshConfig()` | ✅ Đầy đủ |
| Relay tự động deploy lên server | `ssh-relay-deploy.ts` | ✅ Đầy đủ |
| SSH Connection lifecycle (connect/reconnect) | `ssh-connection.ts` | ✅ Đầy đủ |
| Port forwarding (auto-detect + persist) | `ssh-port-forward.ts`, `SavedPortForward` | ✅ Đầy đủ |
| Relay grace period config | `SshTarget.relayGracePeriodSeconds` | ✅ Đầy đủ |
| Per-target identity file | `SshTarget.identityFile` | ✅ Đầy đủ |
| ProxyJump / ProxyCommand | `SshTarget.jumpHost`, `SshTarget.proxyCommand` | ✅ Đầy đủ |
| Node.js auto-install trên remote host | `ssh-remote-node-resolution.ts` | ✅ Đầy đủ |
| Ephemeral VM recipe (`orca-server` / `ssh` type) | `ephemeral-vm-recipes.ts` | ✅ Partial |

### 2.2 Những gì Orca **CHƯA** có

| Tính năng | Tác động | CR |
|-----------|---------|-----|
| Fleet inventory config file (YAML/JSON) | Không thể khai báo fleet as-code | CR-001 |
| Server grouping theo project/team | Dev không biết server nào thuộc project nào | CR-002 |
| Bulk provisioning từ inventory | Thêm 10 server = 10 lần thêm thủ công | CR-003 |
| Dev server bootstrap automation | Node.js, Git, repos phải setup tay | CR-004 |
| Fleet health monitoring | Không biết server nào down | CR-005 |
| Team-based access control (RBAC) | Không thể giới hạn ai được vào server nào | CR-006 |

---

## 3. Danh sách Change Requests

| CR | Tiêu đề | Priority | Effort |
|----|---------|---------|--------|
| [CR-001](./CR-001-fleet-inventory-config.md) | Fleet Inventory Config File | 🔴 Critical | M |
| [CR-002](./CR-002-server-grouping-by-project.md) | Server Grouping by Project/Team | 🟠 High | S |
| [CR-003](./CR-003-bulk-provisioning.md) | Bulk Server Provisioning from Inventory | 🟠 High | L |
| [CR-004](./CR-004-dev-server-bootstrap.md) | Dev Server Bootstrap Automation | 🟠 High | M |
| [CR-005](./CR-005-fleet-health-monitoring.md) | Fleet Health Monitoring | 🟡 Medium | M |
| [CR-006](./CR-006-team-rbac.md) | Team-based Access Control (RBAC) | 🟡 Medium | L |

---

## 4. Mức độ ưu tiên

```
Phase 1 (Bắt buộc để hoạt động với nhiều teams):
  CR-001 Fleet Inventory Config
  CR-002 Server Grouping
  CR-004 Bootstrap Automation

Phase 2 (Cải thiện vận hành):
  CR-003 Bulk Provisioning
  CR-005 Health Monitoring

Phase 3 (Enterprise):
  CR-006 RBAC
```

---

## 5. Implementation Summary

> **Implemented:** 2026-07-23

| CR | Status | Files đục tạo |
|----|--------|---------------|
| CR-001 | ✅ Implemented | `deploy/dev/orca-fleet.yaml`, `src/shared/fleet-config-parser.ts`, `src/main/ssh/fleet-remote-commands.ts` |
| CR-002 | ✅ Implemented | `src/shared/ssh-types.ts` (extended), `SshTargetGroupedList`, `SshTargetGroup`, `groupSshTargetsByProject()` |
| CR-003 | ✅ Implemented | `src/cli/specs/fleet.ts`, `src/cli/handlers/fleet.ts` (import, provision, list, sync, status) |
| CR-004 | ✅ Implemented | `src/main/ssh/fleet-bootstrap-service.ts`, `src/main/ssh/fleet-remote-commands.ts` |
| CR-005 | ✅ Implemented | `src/main/ssh/fleet-health-monitor.ts`, `fleet-health-store.ts`, `fleet-status-service.ts` |
| CR-006 | ⚠️ Partial | `src/shared/rbac-types.ts` (types + policy resolution); SSO/login flow = Phase 2 |
