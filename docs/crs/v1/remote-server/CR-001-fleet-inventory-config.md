# CR-001 — Fleet Inventory Config File

**CR-ID:** CR-001  
**Ngày:** 2026-07-22  
**Priority:** 🔴 Critical  
**Effort:** Medium (2–3 ngày)  
**Status:** Implemented  

---

## 1. Vấn đề

Orca hiện quản lý SSH targets **hoàn toàn trong SQLite** của từng developer (per-machine, per-user). Không có file config trung tâm nào khai báo fleet các dev server cho một team/project.

**Hậu quả:**
- Developer mới vào team phải hỏi "server nào để làm project X?" và thêm tay
- DevOps không có nguồn sự thật duy nhất (single source of truth) cho fleet
- Thêm/xoá server = thông báo cho từng developer, từng developer update thủ công
- Không versioning, không audit trail cho fleet changes

---

## 2. Phân tích codebase hiện tại

Orca có `orca.yaml` ở project root với format:

```yaml
# /path/to/project/orca.yaml (hiện tại)
scripts:
  setup: |
    npm install
```

Và có `EphemeralVmRecipe` trong `src/shared/ephemeral-vm-recipes.ts` với schema Zod — nhưng chỉ cho **ephemeral VM** (on-demand), không phải persistent fleet.

`SshTarget` type hiện có:
```typescript
// src/shared/ssh-types.ts
type SshTarget = {
  id: string; label: string; host: string; port: number
  username: string; identityFile?: string; jumpHost?: string
  // ... không có: projectTag, teamTag, environment, role
}
```

**Gap:** Không có field để group server theo project/team, không có file config khai báo fleet.

---

## 3. Giải pháp đề xuất

### 3.1 Tạo file `orca-fleet.yaml` tại `deploy/dev/`

```yaml
# deploy/dev/orca-fleet.yaml
# ============================================================
# VNP-BLC Dev Server Fleet Inventory
# ============================================================
# Khai báo tất cả dev servers mà Orca Server cần quản lý.
# Import vào Orca: Settings → SSH Hosts → Import Fleet Config
# ============================================================

version: "1"

defaults:
  port: 22
  username: dev
  identityFile: ~/.ssh/orca_server_key
  relayGracePeriodSeconds: 86400   # 24 giờ

servers:
  # ── Project: VNP-BLC ─────────────────────────────────────
  - id: dev-alpha
    label: "Dev Alpha — vnp-blc"
    host: dev-alpha.vnpblc.internal
    project: vnp-blc
    team: backend
    environment: development
    repos:
      - path: /srv/projects/vnp-blc
        name: vnp-blc
      - path: /srv/projects/vnp-blc-infra
        name: vnp-blc-infra
    portForwards:
      - remotePort: 8080
        localPort: 18080
        label: "API Server"
      - remotePort: 5432
        localPort: 15432
        label: "PostgreSQL"

  # ── Project: VNP-AI-OPS ──────────────────────────────────
  - id: dev-beta
    label: "Dev Beta — vnp-ai-ops"
    host: dev-beta.vnpblc.internal
    project: vnp-ai-ops
    team: ai-platform
    environment: development
    repos:
      - path: /srv/projects/vnp-ai-ops
        name: vnp-ai-ops

  # ── Project: VNP-CLAW ────────────────────────────────────
  - id: dev-gamma
    label: "Dev Gamma — vnp-claw"
    host: dev-gamma.vnpblc.internal
    project: vnp-claw
    team: frontend
    environment: development
    repos:
      - path: /srv/projects/vnp-claw
        name: vnp-claw
```

### 3.2 Thay đổi cần thiết trong Orca

**Option A: Extend `orca.yaml` (ít xâm phạm nhất)**

Thêm section `fleet` vào `orca.yaml` của project:

```yaml
# project/orca.yaml
scripts:
  setup: npm install

fleet:
  servers:
    - id: dev-alpha
      label: "Dev Alpha"
      host: dev-alpha.vnpblc.internal
      # ...
```

**Option B: File riêng `orca-fleet.yaml` (khuyến nghị)**

File riêng biệt, không phụ thuộc project cụ thể, quản lý toàn bộ fleet của team/org.

---

## 4. Changes Required

### 4.1 Trong Orca codebase

| File | Thay đổi |
|------|---------|
| `src/shared/ssh-types.ts` | Thêm fields `project?`, `team?`, `environment?`, `repos?` vào `SshTarget` |
| `src/main/ssh/ssh-connection-store.ts` | Thêm method `importFromFleetConfig(yamlPath)` |
| `src/main/ssh/ssh-config-parser.ts` | Thêm parser cho `orca-fleet.yaml` format |
| `src/main/ipc/` | Thêm IPC handler `ssh.importFleetConfig` |
| `src/renderer/src/` | Thêm UI: "Import Fleet Config" button |

### 4.2 Trong deploy/dev/

| File | Thay đổi |
|------|---------|
| `deploy/dev/orca-fleet.yaml` | [NEW] Fleet inventory file |
| `deploy/dev/scripts/import-fleet.sh` | [NEW] Script import fleet config |

---

## 5. Schema đề xuất cho `orca-fleet.yaml`

```typescript
// Extend SshTarget với metadata mới
type FleetServer = SshTarget & {
  project?: string          // "vnp-blc", "vnp-ai-ops", "vnp-claw"
  team?: string             // "backend", "frontend", "ai-platform"
  environment?: 'development' | 'staging' | 'production'
  repos?: Array<{
    path: string            // /srv/projects/vnp-blc
    name: string            // vnp-blc
  }>
}

type FleetConfig = {
  version: "1"
  defaults?: Partial<FleetServer>
  servers: FleetServer[]
}
```

---

## 6. Workaround hiện tại (trước khi CR được implement)

Dùng `~/.ssh/config` import kết hợp với document thủ công:

```bash
# ~/.ssh/config (trên Orca Server)
Host dev-alpha
  HostName dev-alpha.vnpblc.internal
  User dev
  IdentityFile ~/.ssh/orca_server_key
  # Orca không đọc custom comments, nhưng có thể dùng tags trong label

Host dev-beta
  HostName dev-beta.vnpblc.internal
  ...
```

Trong Orca: Settings → SSH Hosts → "Import from ~/.ssh/config"

---

## 7. Acceptance Criteria

- [x] File `orca-fleet.yaml` có thể parse được bởi Orca
- [x] `importFromFleetConfig()` tạo `SshTarget` entries trong SQLite
- [x] Servers được group/filter theo `project`, `team`
- [x] Thay đổi trong `orca-fleet.yaml` có thể sync lại (re-import via `orca fleet sync`)
- [x] Không overwrite manual target khi re-import (fleet metadata only update)
- [x] Có thể export fleet hiện tại ra `orca-fleet.yaml`

---

## 8. Implementation Notes

> **Implemented:** 2026-07-23

| File | Status |
|------|--------|
| `deploy/dev/orca-fleet.yaml` | ✅ [NEW] Fleet inventory với 4 servers: dev-alpha, dev-alpha2, dev-beta, dev-gamma |
| `src/shared/fleet-config-parser.ts` | ✅ [NEW] Zod schema, `parseFleetConfig()`, `fleetServerToSshTarget()` |
| `src/shared/fleet-types.ts` | ✅ [NEW] `FleetStatusReport`, `BootstrapResult`, `BootstrapStep` |
| `src/main/ssh/ssh-connection-store.ts` | ✅ [MODIFY] `importFromFleetConfig()`, `exportToFleetConfig()`, fleet filter methods |
| `src/main/ipc/ssh.ts` | ✅ [MODIFY] `ssh.importFleetConfig`, `ssh.watchFleetConfig`, `fleet:*` IPC handlers |
| `src/shared/ssh-types.ts` | ✅ [MODIFY] `project`, `team`, `environment`, `tags`, `fleetId`, `repos` fields added |

---

## Implementation Status

> **✅ IMPLEMENTED — 2026-07-23 | 6/6 AC done**

Fleet inventory and configuration implemented via `FleetBootstrapService` and SSH connection store.

| File | Status |
|------|--------|
| `src/main/ssh/fleet-bootstrap-service.ts` | ✅ Fleet config + inventory |
| `src/shared/fleet-types.ts` | ✅ Fleet type definitions |
| `src/shared/rbac-types.ts` | ✅ RBAC + fleet RBAC types |
