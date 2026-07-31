# TASK-DS-009 — Thêm `--max-time 8` Vào curl Trong start.sh Heredoc

**Solution:** [SOL-DS-005 §1](../solutions/SOL-DS-005-daemon-hardening.md)  
**Bug:** [BUG-DS-006](../BUG-DS-006-curl-timeout-conflict.md)  
**File:** `deploy/dev/scripts/start-agent-direct.sh`  
**Phụ thuộc:** Không  
**Estimated:** 15 phút  
**Status:** ✅ DONE — 2026-07-27

---

## Mục Tiêu

Thêm `--max-time 8` vào lệnh `curl` trong heredoc của `start-agent-direct.sh`. Ngăn curl chạy lâu hơn `TimeoutStopSec=10s` của systemd — tránh orphan token slots khi systemd SIGKILL process.

---

## Context

Đọc trước:
- `deploy/dev/scripts/start-agent-direct.sh` — tìm heredoc tạo `start.sh` (thường là `cat << 'EOF' > ...`)
- Trong heredoc, tìm dòng `curl -sf -X POST`

---

## Thay Đổi Cần Thực Hiện

**File:** `deploy/dev/scripts/start-agent-direct.sh`

Tìm trong heredoc dòng curl (bên trong `cat << 'EOF'` block):

**TÌM:**
```bash
API_RESP=$(curl -sf -X POST \
  -H "Content-Type: application/json" \
  -H "${AUTH_HEADER}" \
  -d "${BODY}" \
  "http://${ORCA_HTTP_HOST}:${ORCA_HTTP_PORT}/api/agent-token" 2>/dev/null) || {
    echo "[$(date -u +%FT%TZ)] ERROR: Failed to get token from Orca"
```

**THAY BẰNG:**
```bash
API_RESP=$(curl -sf --max-time 8 --retry 2 --retry-delay 2 -X POST \
  -H "Content-Type: application/json" \
  -H "${AUTH_HEADER}" \
  -d "${BODY}" \
  "http://${ORCA_HTTP_HOST}:${ORCA_HTTP_PORT}/api/agent-token" 2>/dev/null) || {
    echo "[$(date -u +%FT%TZ)] ERROR: Token request failed or timed out (max 8s)"
```

> [!NOTE]
> Tên biến `AUTH_HEADER`, `BODY`, `ORCA_HTTP_HOST`, `ORCA_HTTP_PORT` có thể khác nhau — điều chỉnh theo code thực tế trong file. Chỉ cần thêm `--max-time 8 --retry 2 --retry-delay 2` vào sau `-sf`.

---

## Verify

```bash
# Deploy script mới lên dev server:
bash deploy/dev/scripts/connect-agent.sh --deploy

# Kiểm tra start.sh trên dev server (phải có --max-time 8):
ssh ubuntu@172.20.2.31 "grep 'max-time' ~/orca-agent/start.sh"
# Expected: --max-time 8

# Test: simulate slow server (nếu có staging):
# curl phải fail sau 8s, script exit 1, systemd restart clean
```

---

## Definition of Done

- [x] `--max-time 8` đã thêm vào curl trong heredoc
- [x] `--retry 2 --retry-delay 2` đã thêm (tự retry network error)
- [x] Error message rõ ràng "failed/timed out (max 8s)"
- [x] Comment giải thích `< TimeoutStopSec=10s` trong script
