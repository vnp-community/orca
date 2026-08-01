# CR-003 — Bulk Server Provisioning from Fleet Inventory

**CR-ID:** CR-003  
**Ngày:** 2026-07-22  
**Priority:** 🟠 High  
**Effort:** Large (3–5 ngày)  
**Depends on:** CR-001 (Fleet Inventory)  
**Status:** Implemented  

---

## 1. Vấn đề

Orca tự động deploy `orca-relay` binary lên server khi kết nối lần đầu — đây là tính năng mạnh. Nhưng để **thêm 10 dev server vào Orca**, DevOps phải:

1. Mở Orca UI → SSH Hosts → Add
2. Điền host, port, user, identity file cho từng server
3. Click Connect → Orca deploy relay → Done
4. Lặp lại x10

**Không có** cách nào thêm nhiều servers cùng lúc từ một config file.

---

## 2. Phân tích codebase

### 2.1 Cơ chế relay hiện tại

`ssh-relay-deploy.ts` — deploy relay khi connect:
```typescript
// Tự động upload orca-relay binary lên server qua SFTP
// Sau đó start relay process → WebSocket multiplexer
export async function deployRelay(connection: SshConnection): Promise<RelayDeployResult>
```

**Ưu điểm:** Relay deploy hoàn toàn tự động, không cần cài gì trên remote server ngoài Node.js.

**Gap:** Không có CLI/API để trigger deploy cho multiple targets cùng lúc.

### 2.2 CLI hiện tại

```
orca serve        → start headless server
orca status       → check runtime status
orca environment  → manage runtime environments
# KHÔNG có:
# orca fleet import <config>
# orca fleet provision
# orca fleet sync
```

---

## 3. Giải pháp đề xuất

### 3.1 CLI command mới: `orca fleet`

```bash
# Import fleet từ config file
orca fleet import deploy/dev/orca-fleet.yaml

# Provision tất cả servers trong fleet (deploy relay)
orca fleet provision --all
orca fleet provision --project vnp-blc
orca fleet provision --server dev-alpha

# Check status toàn bộ fleet
orca fleet status

# Sync fleet config (remove/add servers theo config)
orca fleet sync deploy/dev/orca-fleet.yaml

# List servers
orca fleet list
orca fleet list --project vnp-blc --json
```

### 3.2 Script workaround hiện tại

```bash
#!/usr/bin/env bash
# deploy/dev/scripts/import-fleet.sh
# Workaround: đọc orca-fleet.yaml và thêm từng target qua Orca IPC

FLEET_CONFIG="${1:-deploy/dev/orca-fleet.yaml}"

# Parse YAML (cần yq hoặc python)
servers=$(yq e '.servers[]' "${FLEET_CONFIG}" -o json)

echo "$servers" | while IFS= read -r server; do
  HOST=$(echo "$server" | jq -r '.host')
  PORT=$(echo "$server" | jq -r '.port // 22')
  USER=$(echo "$server" | jq -r '.username // "dev"')
  LABEL=$(echo "$server" | jq -r '.label')
  KEY=$(echo "$server" | jq -r '.identityFile // "~/.ssh/orca_server_key"')

  echo "Adding: $LABEL ($USER@$HOST:$PORT)"

  # Dùng Orca CLI để add target (nếu CLI hỗ trợ)
  # orca ssh add --host "$HOST" --port "$PORT" --user "$USER" --label "$LABEL" --key "$KEY"

  # Hiện tại phải dùng IPC trực tiếp hoặc thêm thủ công
  echo "  → Manual: Orca UI → SSH Hosts → Add → $HOST"
done
```

### 3.3 Luồng bulk provision đề xuất

```
orca fleet import orca-fleet.yaml
  ↓
Parse fleet config
  ↓
For each server in config:
  1. addTarget(server) → SshTarget in SQLite
  2. connect(targetId)
     → SSH handshake
     → detect Node.js version (ssh-remote-node-resolution.ts)
     → SFTP upload orca-relay binary (ssh-relay-deploy.ts)
     → start relay process
     → ✅ Server provisioned
  ↓
Report: N/M servers provisioned successfully
```

---

## 4. Changes Required

### 4.1 Orca CLI

| File | Thay đổi |
|------|---------|
| `src/cli/specs/` | Thêm `fleet.ts` command spec |
| `src/cli/handlers/` | Thêm `fleet.ts` handler |
| `src/cli/dispatch.ts` | Register fleet commands |

### 4.2 Fleet command spec

```typescript
// src/cli/specs/fleet.ts
export const FLEET_COMMAND_SPECS: CommandSpec[] = [
  {
    path: ['fleet', 'import'],
    summary: 'Import servers from a fleet config file',
    usage: 'orca fleet import <config-file>',
    examples: ['orca fleet import deploy/dev/orca-fleet.yaml']
  },
  {
    path: ['fleet', 'provision'],
    summary: 'Deploy Orca relay to fleet servers',
    usage: 'orca fleet provision [--all] [--project <name>] [--server <id>]',
    allowedFlags: [...GLOBAL_FLAGS, 'all', 'project', 'server']
  },
  {
    path: ['fleet', 'status'],
    summary: 'Show connection status of fleet servers',
    usage: 'orca fleet status [--project <name>] [--json]',
    allowedFlags: [...GLOBAL_FLAGS, 'project']
  },
  {
    path: ['fleet', 'list'],
    summary: 'List all servers in fleet',
    usage: 'orca fleet list [--project <name>] [--json]',
    allowedFlags: [...GLOBAL_FLAGS, 'project']
  }
]
```

---

## 5. Workaround hiện tại

**Phương án A: Import `~/.ssh/config` thủ công**

```bash
# Trên Orca Server: tạo ~/.ssh/config với tất cả dev servers
cat > /home/orca/.ssh/config << 'EOF'
Host dev-alpha
  HostName dev-alpha.vnpblc.internal
  User dev
  IdentityFile ~/.ssh/orca_server_key
  Port 22

Host dev-beta
  HostName dev-beta.vnpblc.internal
  User dev
  IdentityFile ~/.ssh/orca_server_key
  Port 22

Host dev-gamma
  HostName dev-gamma.vnpblc.internal
  User dev
  IdentityFile ~/.ssh/orca_server_key
  Port 22
EOF

# Trong Orca UI: Settings → SSH Hosts → Import from ~/.ssh/config
# → Tất cả 3 servers được import cùng lúc
```

**Đây là workaround tốt nhất hiện tại** — import `~/.ssh/config` có thể add nhiều servers cùng lúc.

---

## 6. File tham khảo

Xem `deploy/dev/orca-fleet.yaml` (sẽ tạo theo CR-001) cho format đề xuất.

---

## 7. Acceptance Criteria

- [x] `orca fleet import <file>` thêm tất cả servers từ config vào Orca
- [x] `orca fleet provision` deploy relay lên tất cả servers
- [x] Provision song song (concurrent) với `--concurrency` flag (default: p-limit)
- [x] Report rõ ràng: thành công / thất bại / lý do
- [x] Idempotent: chạy lại không tạo duplicate (fleet metadata only update)
- [x] `orca fleet status` hiển thị: connected/disconnected/error cho từng server

---

## 8. Implementation Notes

> **Implemented:** 2026-07-23

| File | Status |
|------|--------|
| `src/cli/specs/fleet.ts` | ✅ [NEW] `fleet import`, `fleet provision`, `fleet status`, `fleet list`, `fleet sync`, `fleet bootstrap` specs |
| `src/cli/handlers/fleet.ts` | ✅ [NEW] Full handler implementation với p-limit concurrency |
| `src/main/ipc/ssh.ts` | ✅ [MODIFY] `ssh.importFleetConfig` IPC handler, `fleet:getStatus` |
| `src/shared/fleet-config-parser.ts` | ✅ [NEW] YAML parser, Zod schema validation |
| `src/main/ssh/ssh-connection-store.ts` | ✅ [MODIFY] `importFromFleetConfig()`, `exportToFleetConfig()` |

---

## Implementation Status

> **✅ IMPLEMENTED — 2026-07-23 | 6/6 AC done**

Bulk provisioning implemented via `FleetBootstrapService` and `DevServerProvisioner`.

| File | Status |
|------|--------|
| `src/main/ssh/fleet-bootstrap-service.ts` | ✅ Bulk provision flow |
| `src/main/ssh/dev-server-provisioner.ts` | ✅ Per-server idempotent provision |
| `src/main/ssh/fleet-remote-commands.ts` | ✅ Remote command execution |
