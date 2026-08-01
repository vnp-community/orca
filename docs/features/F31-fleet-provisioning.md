# F31 — Fleet Inventory & Bulk Provisioning

| Trường | Giá trị |
|--------|---------|
| **ID** | F31 |
| **Tên** | Fleet Inventory & Bulk Provisioning |
| **Ưu tiên** | P1 |
| **Trạng thái** | ✅ Phát hành |
| **CRs** | [remote-server/CR-001~004](../crs/v1/remote-server/) |
| **Phiên bản** | v4.1+ |
| **ADR References** | ADR-004 |
| **HLD References** | C3.5 |

---

## Mô tả

Orca hỗ trợ quản lý fleet dev servers theo mô hình **Infrastructure-as-Code** — khai báo danh sách server trong file YAML (`orca-fleet.yaml`), tự động group theo project/team, bulk provision nhiều server cùng lúc, và tự động bootstrap môi trường phát triển trên mỗi server.

---

## Vấn đề cần giải quyết

Trước đây mô hình Orca là **per-developer, per-connection**: mỗi developer phải thêm thủ công từng SSH target. Với team lớn và nhiều server:
- Không có **single source of truth** cho fleet
- Thêm server mới = phải thông báo từng developer
- Không thể tự động hóa setup môi trường

---

## Tính năng chi tiết

### Fleet Inventory Config File (CR-001)

**`deploy/dev/orca-fleet.yaml`** — khai báo fleet as-code:

```yaml
version: "1"

defaults:
  port: 22
  username: dev
  identityFile: ~/.ssh/orca_server_key
  relayGracePeriodSeconds: 86400

servers:
  - id: dev-alpha
    label: "Dev Alpha — vnp-blc"
    host: dev-alpha.vnpblc.internal
    project: vnp-blc
    team: backend
    environment: development
    repos:
      - path: /srv/projects/vnp-blc
        branch: main

  - id: dev-beta
    label: "Dev Beta — frontend"
    host: dev-beta.vnpblc.internal
    project: vnp-blc
    team: frontend
    environment: staging
```

**Import flow:** Settings → SSH Hosts → "Import Fleet Config" → parse YAML → upsert vào DB.

---

### Server Grouping by Project/Team (CR-002)

```typescript
// src/shared/ssh-types.ts — extended
interface SshTargetGroup {
  project: string               // 'vnp-blc', 'frontend-app'
  team?: string                 // 'backend', 'frontend', 'devops'
  environment?: string          // 'development', 'staging', 'production'
  servers: SshTarget[]
}

function groupSshTargetsByProject(targets: SshTarget[]): SshTargetGroupedList
```

UI: Fleet View hiển thị servers nhóm theo project — collapsed/expanded per group.

---

### Bulk Server Provisioning (CR-003)

```
orca fleet provision --project vnp-blc --concurrency 5
    │
    ├── Load orca-fleet.yaml → filter by project
    │
    └── For each server (parallel, max 5 concurrent):
             ├── SSH connect
             ├── FleetBootstrapService.bootstrap()
             ├── FleetHealthMonitor.initialCheck()
             └── Report: online | degraded | unhealthy
```

**CLI command:** `orca fleet provision [--project <name>] [--server <id>] [--dry-run] [--concurrency N]`

**Dry-run mode:** hiển thị danh sách servers sẽ được provision, không thực thi.

---

### Dev Server Bootstrap Automation (CR-004)

`FleetBootstrapService` tự động setup môi trường trên server mới:

| Step | Command | Điều kiện |
|------|---------|-----------|
| Check Node.js | `node --version` | Cần ≥ 22.x |
| Check Git | `git --version` | Cần ≥ 2.25 |
| Check disk | `df -h .` | Cần ≥ 5GB free |
| Install relay | SFTP upload + chmod | `orca-relay --version` mismatch |
| Verify SHA256 | `sha256sum orca-relay` | Đảm bảo binary toàn vẹn |
| Start relay | `nohup orca-relay --listen 7777` | Chưa chạy |
| Clone repos | `git clone <repo> <path>` | Nếu chưa có |

**Output:**
```
✅ dev-alpha: Node 22.4.0, Git 2.45.2, Disk 47GB, Relay v1.8.2 — healthy
⚠️ dev-beta:  Node 20.x (< 22 required) — manual upgrade needed
❌ dev-gamma: SSH timeout — unreachable
```

---

### UI Fleet Management

```
Settings → Fleet
├── Import Fleet Config (upload orca-fleet.yaml)
├── Fleet Overview:
│    ├── Group: vnp-blc / backend (3 servers)
│    │    ├── dev-alpha  [● healthy]  [Provision] [SSH]
│    │    └── dev-beta   [● staging]  [Provision] [SSH]
│    └── Group: frontend (2 servers)
│         └── ...
├── Provision All button (bulk)
└── Export current fleet as YAML
```

---

## Tiêu chí chấp nhận

- [x] Parse `orca-fleet.yaml` — validate schema (Zod), load vào DB
- [x] `groupSshTargetsByProject()` group đúng theo project field
- [x] Bulk provision với concurrency control
- [x] FleetBootstrapService check Node.js/Git/disk/relay
- [x] SFTP upload relay binary + SHA256 verify
- [x] Dry-run mode `--dry-run` không thực thi
- [x] Import Fleet Config UI trong Settings
- [x] Fleet View nhóm servers theo project
- [x] Bootstrap log hiển thị per-server progress
- [x] Export fleet config từ DB về YAML

---

## Yêu cầu kỹ thuật

| Component | File |
|-----------|------|
| Fleet config parser | `src/shared/fleet-config-parser.ts` |
| SshTargetGroup types | `src/shared/ssh-types.ts` (extended) |
| Fleet bootstrap service | `src/main/ssh/fleet-bootstrap-service.ts` |
| Fleet remote commands | `src/main/ssh/fleet-remote-commands.ts` |
| Fleet config importer | `src/main/ssh/fleet-config-importer.ts` |
| Fleet CLI | `src/cli/fleet-commands.ts` |
| Fleet YAML example | `deploy/dev/orca-fleet.yaml` |
| Fleet View UI | `src/renderer/src/components/fleet/FleetView.tsx` |
| Import dialog | `src/renderer/src/components/fleet/FleetImportDialog.tsx` |

---

## Metrics

| KPI | Mục tiêu |
|-----|----------|
| Parse orca-fleet.yaml (100 servers) | < 100ms |
| Bulk provision 10 servers (concurrency=5) | < 5 min |
| Bootstrap single server (Node+Git+disk check) | < 30s |
| SFTP relay upload (10MB binary) | < 15s |
