# TASK-DS-010 — Merge Service Files: `orca-agent.service` + Sửa Deploy Script

**Solution:** [SOL-DS-005 §2](../solutions/SOL-DS-005-daemon-hardening.md)  
**Bug:** [BUG-DS-007](../BUG-DS-007-service-file-inconsistency.md)  
**Files:** `deploy/dev/agent/orca-agent.service`, `deploy/dev/scripts/start-agent-direct.sh`, `deploy/dev/scripts/connect-agent.sh`  
**Phụ thuộc:** Không  
**Estimated:** 20 phút  
**Status:** ✅ DONE — 2026-07-27

---

## Mục Tiêu

Đồng nhất 2 systemd service files thành 1 source of truth. Cập nhật `orca-agent.service` (repo) để dùng file log (nhất quán với `connect-agent.sh --logs`). Cập nhật `start-agent-direct.sh` để copy service file từ repo thay vì generate riêng.

---

## Context

Đọc trước:
- `deploy/dev/agent/orca-agent.service` — service file hiện tại trong repo
- `deploy/dev/scripts/start-agent-direct.sh` — tìm phần generate service file (heredoc với `[Unit]`)
- `deploy/dev/scripts/connect-agent.sh` — hàm `agent_logs()` — xem đang dùng journald hay file

---

## Thay Đổi Cần Thực Hiện

### Thay đổi 1: `deploy/dev/agent/orca-agent.service`

**TÌM** các dòng log hiện tại:
```ini
# Log output ra journald
StandardOutput=journal
StandardError=journal
SyslogIdentifier=orca-agent
```

**THAY BẰNG** (log ra file — nhất quán với `connect-agent.sh --logs`):
```ini
# Log ra file (nhất quán với connect-agent.sh --logs dùng tail)
StandardOutput=append:/home/ubuntu/orca-agent/logs/agent-direct.log
StandardError=append:/home/ubuntu/orca-agent/logs/agent-direct.log
```

**Cũng sửa** `TimeoutStopSec` để match với curl `--max-time 8`:

**TÌM:**
```ini
TimeoutStopSec=10s
```

**THAY BẰNG:**
```ini
TimeoutStopSec=15s
```

### Thay đổi 2: `deploy/dev/scripts/start-agent-direct.sh`

Tìm đoạn generate service file trong heredoc (dạng `cat << EOF > /tmp/orca-agent-direct.service`).

**THAY THẾ toàn bộ đoạn generate** bằng:
```bash
# Dùng service file từ repo (source of truth) thay vì generate
SERVICE_SRC="${SCRIPT_DIR}/../agent/orca-agent.service"
if [ ! -f "${SERVICE_SRC}" ]; then
  err "Không tìm thấy service file: ${SERVICE_SRC}. Chạy từ thư mục deploy/dev/scripts/"
fi
log "Copying service file từ repo: ${SERVICE_SRC}"
scp "${SERVICE_SRC}" "${DEV_SERVER_USER}@${DEV_SERVER_HOST}:/tmp/orca-agent-direct.service"
```

Và cập nhật dòng `ssh ... mv` để giữ tên `orca-agent-direct`:
```bash
ssh "${DEV_SERVER_USER}@${DEV_SERVER_HOST}" \
  "sudo mv /tmp/orca-agent-direct.service /etc/systemd/system/orca-agent-direct.service && \
   sudo systemctl daemon-reload && \
   sudo systemctl enable orca-agent-direct"
```

### Thay đổi 3: `deploy/dev/scripts/connect-agent.sh` — hàm `agent_logs()`

Cập nhật để check cả file log và journald (fallback):

**TÌM** hàm `agent_logs()`:
```bash
agent_logs() {
  log "Logs agent từ ${DEV_SERVER_HOST}..."
  # ... (tùy cách hiện tại)
}
```

**THAY BẰNG:**
```bash
agent_logs() {
  log "Logs agent (50 dòng cuối) từ ${DEV_SERVER_HOST}..."
  echo ""
  # Ưu tiên file log; fallback journald nếu file chưa có
  ssh_dev "
    LOG_FILE=\${AGENT_DEPLOY_DIR}/logs/agent-direct.log
    if [ -f \"\${LOG_FILE}\" ]; then
      tail -50 \"\${LOG_FILE}\"
    else
      journalctl -u orca-agent-direct -n 50 --no-pager 2>/dev/null \
        || echo 'Chưa có logs. Agent chưa chạy hoặc log file chưa được tạo.'
    fi
  "
}
```

> [!NOTE]
> Tên biến `AGENT_DEPLOY_DIR`, `DEV_SERVER_HOST`, `DEV_SERVER_USER` phụ thuộc vào code thực tế trong file. Điều chỉnh cho phù hợp.

---

## Verify

```bash
# Deploy service file mới:
bash deploy/dev/scripts/connect-agent.sh --deploy

# Kiểm tra service file trên dev server:
ssh ubuntu@172.20.2.31 "cat /etc/systemd/system/orca-agent-direct.service" | grep StandardOutput
# Expected: StandardOutput=append:/home/ubuntu/orca-agent/logs/agent-direct.log

# Test logs command:
bash deploy/dev/scripts/connect-agent.sh --logs
# Expected: hiện logs từ file (không phải "No entries" từ journald)
```

---

## Definition of Done

- [x] `orca-agent.service` dùng `StandardOutput=append:...agent-direct.log` thay vì journal
- [x] `orca-agent.service` có `TimeoutStopSec=15s` (> curl 8s + 2s buffer)
- [x] `connect-agent.sh agent_logs()` có fallback: file → journald → error message
- [x] Cả 2 service names (`orca-agent-direct`, `orca-agent`) được check trong fallback
