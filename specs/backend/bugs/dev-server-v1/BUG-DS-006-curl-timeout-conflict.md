# BUG-DS-006 — curl Trong start.sh Không Có `--max-time`, Xung Đột TimeoutStopSec

**ID:** BUG-DS-006  
**Mức độ:** 🟡 Low  
**Module:** `deploy/dev/agent/start.sh` (được tạo bởi `start-agent-direct.sh`)  
**Phát hiện:** 2026-07-26  
**Status:** 🔴 Open

---

## Mô Tả

`start.sh` (wrapper script chạy bởi systemd) gọi `curl POST /api/agent-token` không có `--max-time`. `orca-agent.service` có `TimeoutStopSec=10s`. Nếu curl đang chờ response trong lúc systemd cố stop service (ví dụ: khi `systemctl restart`), systemd sẽ SIGKILL `start.sh` sau 10s — nhưng token đã được register trên server. Kết quả: orphan slot 60s trên server.

---

## Root Cause

**start.sh** (tạo bởi `start-agent-direct.sh` heredoc):

```bash
API_RESP=$(curl -sf -X POST \
  -H "Content-Type: application/json" \
  -H "${AUTH_HEADER}" \
  -d "{\"devServerId\":\"${DEV_SERVER_ID}\",...}" \
  "http://${ORCA_HTTP_HOST}:${ORCA_HTTP_PORT}/api/agent-token" 2>/dev/null)
# ← Không có --max-time
```

**orca-agent.service**:
```ini
TimeoutStopSec=10s   ← systemd kill process nếu không stop trong 10s
```

**Scenario lỗi**:
```
T=0:  systemctl restart orca-agent
T=0:  systemd SIGTERM start.sh (đang chạy curl)
T=0:  curl đang chờ HTTP response (server chậm)
T=5:  Server trả response, token T_new được registered trong pendingSlots
T=10: systemd SIGKILL start.sh (TimeoutStopSec expire)
T=10: start.sh bị kill → agent.js chưa được exec → token T_new orphan
T=70: Token T_new expire → onExpired() callback → status='error'
T=10+: systemd start lại start.sh → lấy token T_new2 → agent chạy bình thường
```

Hậu quả nhẹ: orphan slot 60s, server emit error event, UI flash "Error" rồi "Connecting" rồi "Connected".

---

## Fix

Thêm `--max-time 8` vào curl (nhỏ hơn TimeoutStopSec=10s):

```bash
# start-agent-direct.sh heredoc — sửa dòng curl:
API_RESP=$(curl -sf --max-time 8 -X POST \
  -H "Content-Type: application/json" \
  -H "${AUTH_HEADER}" \
  -d "{\"devServerId\":\"${DEV_SERVER_ID}\",...}" \
  "http://${ORCA_HTTP_HOST}:${ORCA_HTTP_PORT}/api/agent-token" 2>/dev/null) || {
    echo "ERROR: Token request timed out or failed"
    sleep 5
    exit 1
}
```

Hoặc tăng `TimeoutStopSec=30s` trong service file.

---

## Files Liên Quan

| File | Vai trò |
|------|---------|
| `deploy/dev/scripts/start-agent-direct.sh` | Tạo start.sh heredoc — thiếu `--max-time` |
| `deploy/dev/agent/orca-agent.service` | `TimeoutStopSec=10s` |
