# BUG-DS-007 — Hai Systemd Service Files Không Đồng Bộ

**ID:** BUG-DS-007  
**Mức độ:** 🟡 Low  
**Module:** `deploy/dev/agent/` + `deploy/dev/scripts/`  
**Phát hiện:** 2026-07-26  
**Status:** 🔴 Open

---

## Mô Tả

Có 2 systemd service files khác nhau với tên, cấu hình log, và cách tạo khác nhau:

1. **`deploy/dev/agent/orca-agent.service`** — service file mới, được commit vào repo
2. **`orca-agent-direct.service`** — service file được tạo bởi `start-agent-direct.sh` tại runtime

---

## Sự Khác Biệt

| Thuộc tính | `orca-agent.service` (repo) | `orca-agent-direct.service` (generated) |
|------------|---------------------------|-----------------------------------------|
| **Tên service** | `orca-agent` | `orca-agent-direct` |
| **Log output** | `journald` (StandardOutput=journal) | File (`~/orca-agent/logs/agent-direct.log`) |
| **SyslogIdentifier** | `orca-agent` | _(không có)_ |
| **User** | `ubuntu` (hardcode) | `${DEV_SERVER_USER}` (từ .env) |
| **ExecStart** | `/bin/bash ~/orca-agent/start.sh` | `/bin/bash /home/ubuntu/orca-agent/start.sh` |
| **StartLimitBurst** | 10 | _(không có)_ |
| **PrivateTmp** | yes | _(không có)_ |
| **Cài bằng cách** | `sudo cp` thủ công | `start-agent-direct.sh` qua SSH |

---

## Hậu Quả

**1. Log location không nhất quán**:
- `orca-agent.service`: logs ở `journald` → xem bằng `journalctl -u orca-agent`
- `orca-agent-direct.service`: logs ở file → xem bằng `tail -f ~/orca-agent/logs/agent-direct.log`
- `connect-agent.sh --logs` dùng `tail` trên file → không hoạt động nếu dùng `orca-agent.service`

**2. Hai service có thể conflict**:
- Nếu cả hai đều enabled và running → 2 instances agent chạy song song → 2 token registered → race condition

**3. Tên service khác nhau**:
- Scripts check `sudo systemctl status orca-agent` → không tìm thấy `orca-agent-direct`
- Documentation có thể sai tên

---

## Fix

**Phương án A — Xóa `start-agent-direct.sh` generated service, dùng `orca-agent.service`**:

Sửa `orca-agent.service` để flexible hơn:
```ini
[Service]
User=%i  # Dùng template service: orca-agent@ubuntu.service
Environment=HOME=/home/%i
```

Sửa `start-agent-direct.sh` để copy `orca-agent.service` từ repo thay vì generate:
```bash
scp deploy/dev/agent/orca-agent.service ubuntu@172.20.2.31:/tmp/
ssh ubuntu@172.20.2.31 "sudo mv /tmp/orca-agent.service /etc/systemd/system/ && sudo systemctl daemon-reload && sudo systemctl enable orca-agent"
```

**Phương án B — Đồng bộ cấu hình log**:

Sửa `orca-agent.service` để cũng log ra file (nhất quán với generated service):
```ini
StandardOutput=append:/home/ubuntu/orca-agent/logs/agent-direct.log
StandardError=append:/home/ubuntu/orca-agent/logs/agent-direct.log
```

---

## Files Liên Quan

| File | Vai trò |
|------|---------|
| `deploy/dev/agent/orca-agent.service` | Service file trong repo |
| `deploy/dev/scripts/start-agent-direct.sh` | Script generate service file khác |
