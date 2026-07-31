# SOL-DS-005 — Daemon Hardening: curl Timeout + Service Merge + Keepalive

**Fixes:** [BUG-DS-006](../BUG-DS-006-curl-timeout-conflict.md), [BUG-DS-007](../BUG-DS-007-service-file-inconsistency.md), [BUG-DS-008](../BUG-DS-008-keepalive-margin.md)  
**TDD Ref:** TDD-05 §6 (relay protocol keepalive), TDD-13 §6 (server bootstrap)  
**Files:** `deploy/dev/scripts/start-agent-direct.sh`, `deploy/dev/agent/orca-agent.service`, `deploy/dev/scripts/connect-agent.sh`, `deploy/dev/agent/agent.js`  
**Effort:** ~1 giờ  
**Status:** ✅ DONE — 2026-07-27 (TASK-DS-009, 010, 011)  
**Implemented in:**
- `start-agent-direct.sh` dng 50-59 — curl `--max-time 8 --retry 2 --retry-delay 2`
- `orca-agent.service` dng 47-59 — file logging + `TimeoutStopSec=15s`
- `connect-agent.sh` dng 288-306 — `agent_logs()` với file+journald fallback
- `agent.js` dng 593 — `startKeepalive(ws, ms = 5000)`

---

## Phần 1: BUG-DS-006 — curl `--max-time` trong start.sh

### Vấn đề
`TimeoutStopSec=10s` trong service file → systemd SIGKILL process sau 10s.
`curl` không có timeout → có thể chạy >10s → bị kill sau khi token đã registered.

### Fix: `deploy/dev/scripts/start-agent-direct.sh` (heredoc)

Tìm dòng curl trong heredoc:
```bash
# TRƯỚC:
API_RESP=\$(curl -sf -X POST \
  -H "Content-Type: application/json" \
  -H "\${AUTH_HEADER}" \
  -d "{...}" \
  "http://\${ORCA_HTTP_HOST}:\${ORCA_HTTP_PORT}/api/agent-token" 2>/dev/null) || {
```

Thay bằng:
```bash
# SAU — thêm --max-time 8 (< TimeoutStopSec=10s):
API_RESP=\$(curl -sf --max-time 8 --retry 2 --retry-delay 2 -X POST \
  -H "Content-Type: application/json" \
  -H "\${AUTH_HEADER}" \
  -d "{...}" \
  "http://\${ORCA_HTTP_HOST}:\${ORCA_HTTP_PORT}/api/agent-token" 2>/dev/null) || {
    echo "[\$(date -u +%FT%TZ)] ERROR: Token request failed/timed out (max 8s)" >> "\${LOG_DIR}/agent-direct.log"
    sleep 5
    exit 1
}
```

**Giải thích**:
- `--max-time 8`: fail nếu request >8s (< systemd's 10s kill)
- `--retry 2 --retry-delay 2`: tự retry 2 lần nếu network error (không retry nếu server error 4xx/5xx)
- Token sẽ chỉ được registered nếu curl thành công → không còn orphan slots

---

## Phần 2: BUG-DS-007 — Merge 2 Service Files

### Vấn đề
- `orca-agent.service` (repo): journald log, `User=ubuntu` hardcode
- `orca-agent-direct.service` (generated): file log, `User=${DEV_SERVER_USER}`

### Fix Strategy: Dùng `orca-agent.service` làm source of truth, sửa `start-agent-direct.sh` để deploy từ file này thay vì generate.

**Sửa `deploy/dev/agent/orca-agent.service`** — đổi log sang file (nhất quán):

```ini
[Unit]
Description=Orca Dev Server Agent (direct-websocket, auto-token)
Documentation=https://b15.openledger.vn
After=network-online.target
Wants=network-online.target
StartLimitIntervalSec=300
StartLimitBurst=10

[Service]
Type=simple
User=ubuntu
Group=ubuntu
WorkingDirectory=/home/ubuntu/orca-agent

# start.sh lấy token mới từ Orca mỗi lần start
ExecStart=/bin/bash /home/ubuntu/orca-agent/start.sh

# Luôn restart khi exit (token hết hạn, network drop, crash)
Restart=always
RestartSec=5s

# Ghi log ra file (nhất quán với connect-agent.sh --logs)
StandardOutput=append:/home/ubuntu/orca-agent/logs/agent-direct.log
StandardError=append:/home/ubuntu/orca-agent/logs/agent-direct.log

# Environment
Environment=HOME=/home/ubuntu
Environment=PATH=/home/ubuntu/.local/bin:/home/ubuntu/bin:/usr/local/bin:/usr/bin:/bin:/snap/bin
Environment=NODE_ENV=production

# Graceful shutdown
KillSignal=SIGTERM
TimeoutStopSec=15s

# Security
NoNewPrivileges=yes
PrivateTmp=yes

[Install]
WantedBy=multi-user.target
```

**Sửa `deploy/dev/scripts/start-agent-direct.sh`** — copy từ repo thay vì generate:

```bash
# TRƯỚC: tạo service file từ heredoc
cat << EOF > /tmp/orca-agent-direct.service
[Unit]
Description=...
...
EOF

# SAU: copy từ repo (source of truth)
SERVICE_SRC="${SCRIPT_DIR}/../agent/orca-agent.service"
if [ ! -f "${SERVICE_SRC}" ]; then
  err "Không tìm thấy service file: ${SERVICE_SRC}"
fi
cp "${SERVICE_SRC}" /tmp/orca-agent-direct.service

# SSH install:
ssh ... "sudo mv /tmp/orca-agent-direct.service /etc/systemd/system/orca-agent-direct.service"
# (giữ tên orca-agent-direct để không conflict nếu user có orca-agent.service khác)
```

**Update `connect-agent.sh --logs`** — check cả 2 locations:

```bash
agent_logs() {
  log "Logs agent (50 dòng cuối) từ ${DEV_SERVER_HOST}..."
  echo ""
  # Thử file log trước, fallback sang journald
  ssh_dev "tail -50 ${AGENT_DEPLOY_DIR}/logs/agent-direct.log 2>/dev/null || \
           journalctl -u orca-agent-direct -n 50 --no-pager 2>/dev/null || \
           echo 'Chưa có logs'"
}
```

---

## Phần 3: BUG-DS-008 — Keepalive Margin

### Thực tế từ relay-protocol.ts:
```typescript
export const KEEPALIVE_SEND_MS = 5_000  // Server gửi keepalive mỗi 5s
export const TIMEOUT_MS = 20_000         // Server timeout nếu không nhận ACK 20s
```

Agent hiện tại: `startKeepalive(ws, ms=8000)` → margin 12s.

Theo TDD-05 §6, KEEPALIVE_SEND_MS = 5s là chuẩn. Agent nên match với server.

### Fix: `deploy/dev/agent/agent.js`

```javascript
// TRƯỚC:
function startKeepalive(ws, ms = 8000) { ... }

// SAU — align với relay-protocol.ts KEEPALIVE_SEND_MS = 5_000:
function startKeepalive(ws, ms = 5000) {
  return setInterval(() => {
    if (ws.readyState === WebSocket.OPEN) {
      ws.send(encodeFrame(FRAME_TYPE.PING, ''));
      log.debug('→ PING (keepalive)');
    }
  }, ms);
}
```

**Margin mới**: 20s timeout - 5s interval = 15s buffer (tốt hơn đáng kể).

**Không cần thay đổi TIMEOUT_MS server** — 20s là đủ với keepalive 5s, kể cả LAN latency 1-5ms.

---

## Verification

### BUG-DS-006:
```bash
# Simulate systemctl restart trong lúc start.sh đang chạy:
# 1. Thêm delay vào start.sh để test: sleep 12 trước curl
# 2. systemctl restart orca-agent-direct
# Expected: curl bị kill sau 8s → exit 1 → systemd restart lại → token clean
```

### BUG-DS-007:
```bash
# Sau khi deploy:
ssh ubuntu@172.20.2.31 "cat /etc/systemd/system/orca-agent-direct.service" | grep StandardOutput
# Expected: StandardOutput=append:/home/ubuntu/orca-agent/logs/agent-direct.log

# Logs hoạt động:
bash deploy/dev/scripts/connect-agent.sh --logs
# Expected: hiện logs từ file (không phải empty)
```

### BUG-DS-008:
```bash
# Agent logs phải hiện PING mỗi ~5s thay vì ~8s:
ssh ubuntu@172.20.2.31 "tail -f ~/orca-agent/logs/agent-direct.log | grep PING"
# Expected: PING xuất hiện mỗi 5s
```

---

## Files Liên Quan

| File | Thay đổi |
|------|---------|
| `deploy/dev/scripts/start-agent-direct.sh` | Thêm `--max-time 8` vào curl; copy service từ repo |
| `deploy/dev/agent/orca-agent.service` | Đổi log sang file; `TimeoutStopSec=15s` |
| `deploy/dev/agent/agent.js` | `startKeepalive(ws, ms=5000)` |
| `deploy/dev/scripts/connect-agent.sh` | `agent_logs()` fallback journald |
