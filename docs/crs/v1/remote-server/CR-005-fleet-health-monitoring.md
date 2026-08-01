# CR-005 — Fleet Health Monitoring

**CR-ID:** CR-005  
**Ngày:** 2026-07-22  
**Priority:** 🟡 Medium  
**Effort:** Medium (2–3 ngày)  
**Depends on:** CR-001, CR-002  
**Status:** Implemented  

---

## 1. Vấn đề

Khi có 10+ dev servers, không có cách nào để:
- Biết server nào đang down mà không click từng cái trong UI
- Nhận alert khi server disconnect
- Xem tổng quan health của toàn bộ fleet
- Monitor resource usage (CPU, RAM, Disk) trên từng server

**Hiện tại Orca có:**
- `SshConnectionStatus`: `'disconnected' | 'connecting' | 'connected' | 'error'` — per connection
- UI hiển thị status icon cho từng SSH target
- `orca status` CLI command — nhưng chỉ cho **local runtime**, không phải remote fleet

**Không có:**
- Fleet-level health dashboard
- Proactive alerts khi server disconnect
- Resource monitoring trên remote servers
- Historical uptime/availability data

---

## 2. Phân tích codebase

### 2.1 Connection status hiện có

```typescript
// src/shared/ssh-types.ts
export type SshConnectionStatus =
  | 'disconnected'
  | 'connecting'
  | 'auth-failed'
  | 'deploying-relay'
  | 'connected'
  | 'reconnecting'
  | 'reconnection-failed'
  | 'error'

export type SshConnectionState = {
  targetId: string
  status: SshConnectionStatus
  error: string | null
  reconnectAttempt: number
  remotePlatform?: SshRemotePlatform
}
```

**Gap:** Status chỉ lưu trong memory, không persist, không aggregate fleet-level.

### 2.2 `orca status` CLI

```typescript
// src/cli/specs/ → status command
// → Chỉ check local Orca runtime reachable/unreachable
// → Không check remote SSH connections
```

---

## 3. Giải pháp đề xuất

### 3.1 `orca fleet status` command

```bash
$ orca fleet status

Fleet Health — VNP-BLC Dev Servers
────────────────────────────────────────────────────────────
SERVER              PROJECT      STATUS       UPTIME   RELAY
────────────────────────────────────────────────────────────
dev-alpha           vnp-blc      ✅ Connected  2d 4h   v1.4.2
dev-beta            vnp-ai-ops   ✅ Connected  5h 12m  v1.4.2
dev-gamma           vnp-claw     ❌ Error      0s      N/A
────────────────────────────────────────────────────────────
Summary: 2/3 connected | 1 error
```

```bash
# JSON output cho CI/monitoring integration
$ orca fleet status --json
{
  "servers": [
    {
      "id": "dev-alpha",
      "label": "Dev Alpha — vnp-blc",
      "status": "connected",
      "uptimeSeconds": 191580,
      "relayVersion": "1.4.2",
      "error": null
    },
    ...
  ],
  "summary": {
    "total": 3,
    "connected": 2,
    "disconnected": 0,
    "error": 1
  }
}
```

### 3.2 Health check script (workaround hiện tại)

```bash
#!/usr/bin/env bash
# deploy/dev/scripts/fleet-health-check.sh
# Workaround: SSH vào từng server và kiểm tra health

FLEET_CONFIG="${1:-deploy/dev/orca-fleet.yaml}"

echo "Fleet Health Check — $(date)"
echo "=========================================="

# Parse servers từ fleet config
while IFS= read -r line; do
    HOST=$(echo "$line" | jq -r '.host')
    LABEL=$(echo "$line" | jq -r '.label')
    USER=$(echo "$line" | jq -r '.username // "dev"')
    KEY="$(eval echo "${SSH_KEY:-~/.ssh/orca_server_key}")"

    # SSH health check với timeout 5s
    if ssh -i "$KEY" -o ConnectTimeout=5 -o StrictHostKeyChecking=no \
           -o BatchMode=yes "$USER@$HOST" "echo ok" &>/dev/null; then
        # Kiểm tra Node.js
        NODE_VER=$(ssh -i "$KEY" -o ConnectTimeout=5 -o BatchMode=yes \
                       "$USER@$HOST" "node --version 2>/dev/null" || echo "N/A")
        # Kiểm tra disk space
        DISK=$(ssh -i "$KEY" -o ConnectTimeout=5 -o BatchMode=yes \
                   "$USER@$HOST" "df -h /srv/projects | tail -1 | awk '{print \$5}'" 2>/dev/null || echo "N/A")

        echo "  ✅ $LABEL ($HOST)"
        echo "     Node: $NODE_VER | Disk used: $DISK"
    else
        echo "  ❌ $LABEL ($HOST) — UNREACHABLE"
    fi

done < <(yq e '.servers[]' "$FLEET_CONFIG" -o json 2>/dev/null || echo "[]" | jq -c '.')

echo "=========================================="
```

### 3.3 Cron job monitoring

```bash
# Chạy health check mỗi 5 phút, alert qua Slack khi có vấn đề
*/5 * * * * /opt/orca/scripts/fleet-health-check.sh 2>&1 | \
  grep "❌" | while read -r line; do
    curl -s -X POST "${SLACK_WEBHOOK}" \
      -d "{\"text\": \"⚠️ Orca Fleet Alert: $line\"}"
  done
```

### 3.4 Prometheus metrics (advanced)

Orca relay có thể expose `/metrics` endpoint:

```
# HELP orca_relay_connected Whether relay is connected
# TYPE orca_relay_connected gauge
orca_relay_connected{server="dev-alpha",project="vnp-blc"} 1
orca_relay_connected{server="dev-beta",project="vnp-ai-ops"} 1
orca_relay_connected{server="dev-gamma",project="vnp-claw"} 0

# HELP orca_relay_uptime_seconds Relay uptime in seconds
# TYPE orca_relay_uptime_seconds counter
orca_relay_uptime_seconds{server="dev-alpha"} 191580
```

---

## 4. Changes Required

### 4.1 Orca codebase

| File | Thay đổi |
|------|---------|
| `src/cli/specs/fleet.ts` | Thêm `fleet status` command |
| `src/cli/handlers/fleet.ts` | Query connection states + format output |
| `src/main/runtime/orca-runtime.ts` | Expose fleet status via IPC |
| `src/relay/` | (Optional) Thêm `/metrics` endpoint cho Prometheus |

### 4.2 Deploy scripts

| File | Thay đổi |
|------|---------|
| `deploy/dev/scripts/fleet-health-check.sh` | [NEW] Bash health check script |
| `deploy/dev/scripts/monitor-setup.sh` | [NEW] Setup cron + Slack alerts |

---

## 5. Workaround hiện tại

**Option A: Bash script health check** (xem 3.2 ở trên)

**Option B: Dùng Uptime Kuma** (self-hosted monitoring)

```yaml
# docker-compose.yml — thêm Uptime Kuma service
services:
  uptime-kuma:
    image: louislam/uptime-kuma:1
    container_name: uptime-kuma
    ports:
      - "3001:3001"
    volumes:
      - uptime-kuma-data:/app/data
    restart: unless-stopped
```

Cấu hình monitors trong Uptime Kuma:
- Monitor type: SSH
- Host: dev-alpha.vnpblc.internal, dev-beta, dev-gamma
- Check interval: 5 phút
- Alert: Slack/Email khi down

**Option C: Check qua Nginx logs** (chỉ biết active users, không biết server status)

---

## 6. Acceptance Criteria

- [x] `orca fleet status` hiển thị status của tất cả servers
- [x] `--json` output compatible với CI/CD pipelines
- [x] Alert notification khi server disconnect (Slack/webhook — qua `fleet-health-monitor` hooks)
- [x] Uptime tracking (24h window via `fleet-health-store.ts`)
- [x] Filter theo project/team trong `orca fleet status --project`
- [x] Dashboard trong Orca UI — fleet overview panel ✅ `FleetHealthDashboard.tsx` + `useFleetHealth` hook
- [x] (Optional) Prometheus metrics endpoint ✅ `GET /metrics` via `ws-transport.ts` HTTP hook + `runtime-rpc.ts`

---

## 7. Implementation Notes

> **Implemented:** 2026-07-23

| File | Status |
|------|--------|
| `src/main/ssh/fleet-health-monitor.ts` | ✅ [NEW] Monitors connection events, persists uptime history |
| `src/main/ssh/fleet-health-store.ts` | ✅ [NEW] SQLite-backed uptime tracking, `getUptimeForTarget(windowMs)` |
| `src/main/ssh/fleet-status-service.ts` | ✅ [NEW] `getFleetStatus()` → `FleetStatusReport` with health score |
| `src/shared/fleet-types.ts` | ✅ [NEW] `FleetServerStatus`, `FleetStatusReport` types |
| `src/cli/specs/fleet.ts` | ✅ [MODIFY] `fleet status` spec with `--project`, `--team`, `--json` flags |
| `src/cli/handlers/fleet.ts` | ✅ [MODIFY] `fleet status` handler: table + JSON output |
| `src/main/ipc/ssh.ts` | ✅ [MODIFY] `fleet:getStatus`, `fleet:getUptimeHistory` IPC handlers |

---

## Implementation Status

> **✅ IMPLEMENTED — 2026-07-23 | All 7 AC done**

| Layer | Files | Status |
|-------|-------|--------|
| Backend: FleetHealthMonitor | `src/main/ssh/fleet-health-monitor.ts` | ✅ Done |
| Backend: FleetHealthStore | `src/main/ssh/fleet-health-store.ts` | ✅ Done |
| Backend: FleetStatusService | `src/main/ssh/fleet-status-service.ts` | ✅ Done |
| Backend: FleetRemoteCommands | `src/main/ssh/fleet-remote-commands.ts` | ✅ Done |
| Backend: Metrics endpoint | `src/main/runtime/rpc/ws-transport.ts` + `runtime-rpc.ts` | ✅ Done |
| Frontend: FleetHealthDashboard | `src/renderer/src/components/fleet/` | ✅ Done |
| GlobalSettings: fleetMetricsEnabled | `src/shared/types.ts` | ✅ Done |
