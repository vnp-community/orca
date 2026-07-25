# Orca Server Setup — Cài đặt và Chạy `orca serve`

**Mục tiêu:** Cài Orca ở chế độ headless server (`orca serve`), phục vụ Web UI tích hợp sẵn, cho phép developer kết nối bằng browser hoặc Orca Desktop App.

---

## 1. Chuẩn bị Server

### 1.1 Cập nhật hệ thống

```bash
sudo apt-get update && sudo apt-get upgrade -y
sudo apt-get install -y \
  curl wget git build-essential \
  openssh-server \
  ca-certificates gnupg lsb-release \
  nginx certbot python3-certbot-nginx
```

### 1.2 Cài Node.js 22 LTS

```bash
curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash -
sudo apt-get install -y nodejs

node --version   # v22.x.x
npm --version    # 10.x.x
```

### 1.3 Cài Git 2.35+

```bash
sudo add-apt-repository ppa:git-core/ppa -y
sudo apt-get update && sudo apt-get install -y git
git --version   # 2.x.x
```

---

## 2. Cài Orca CLI / App

### 2.1 Download Orca Linux AppImage (bao gồm CLI + serve mode)

```bash
# Tạo user orca chuyên dụng
sudo useradd -m -s /bin/bash orca
sudo su - orca
mkdir -p ~/.local/bin

# Download Orca Linux build
curl -fsSL https://github.com/stablyai/orca/releases/latest/download/orca-linux.AppImage \
  -o ~/.local/bin/orca

chmod +x ~/.local/bin/orca

# Verify
~/.local/bin/orca --version
```

### 2.2 Thêm Orca vào PATH

```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc

# Test
orca --version
```

---

## 3. Build Web UI Bundle (nếu cần)

Orca tự bundle Web UI khi ship từ GitHub. Nếu build từ source:

```bash
# Clone source (nếu cần build custom)
git clone https://github.com/stablyai/orca.git /opt/orca-src
cd /opt/orca-src

# Cài dependencies
npm install -g pnpm
pnpm install

# Build web bundle
pnpm build:web
# Output: out/web/ (web UI bundle)

# Copy web bundle để orca serve có thể serve
# (Orca tự detect out/web/ nếu chạy từ thư mục src)
```

> **Lưu ý:** Với binary AppImage, web bundle đã được nhúng vào. Không cần build thủ công.

---

## 4. Khởi động `orca serve`

### 4.1 Chạy thử (foreground, để test)

```bash
# Dạng đơn giản nhất
orca serve

# Output mẫu:
# ✓ Orca runtime started
# ✓ WebSocket: ws://0.0.0.0:6768
# ✓ Web UI:    http://localhost:6768/web-index.html?pair=orca%3A%2F%2Fpair%3Fcode%3Dabc123...
# ✓ Pairing URL: orca://pair?code=abc123&endpoint=ws://orca-server:6768&pk=...
```

### 4.2 Với địa chỉ public (quan trọng cho team access)

```bash
# Với custom domain/IP để client kết nối được
orca serve \
  --port 6768 \
  --pairing-address wss://orca.vnpblc.internal

# Output:
# ✓ Web UI: https://orca.vnpblc.internal/web-index.html?pair=...
# ✓ Pairing URL: orca://pair?code=...&endpoint=wss://orca.vnpblc.internal
```

**Giải thích flags:**

| Flag | Mô tả |
|------|-------|
| `--port 6768` | Port WebSocket server lắng nghe (mặc định: 6768) |
| `--pairing-address wss://host` | Địa chỉ client dùng để kết nối (qua Nginx TLS) |
| `--project-root /path` | Root của project (tuỳ chọn, cho single-project server) |
| `--no-pairing` | Tắt pairing (dùng cho internal testing) |
| `--json` | Output JSON thay vì human-readable |

---

## 5. Systemd Service (Production)

### 5.1 Tạo service file

```ini
# /etc/systemd/system/orca-server.service
[Unit]
Description=Orca AI Orchestrator Server
After=network.target nginx.service
Wants=network.target

[Service]
Type=simple
User=orca
Group=orca
WorkingDirectory=/home/orca
ExecStart=/home/orca/.local/bin/orca serve \
  --port 6768 \
  --pairing-address wss://orca.vnpblc.internal
Restart=always
RestartSec=5

# Môi trường
Environment=HOME=/home/orca
Environment=PATH=/home/orca/.local/bin:/usr/local/bin:/usr/bin:/bin
Environment=NODE_ENV=production

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=read-only
ReadWritePaths=/home/orca
PrivateTmp=true

# Logs
StandardOutput=journal
StandardError=journal
SyslogIdentifier=orca-server

[Install]
WantedBy=multi-user.target
```

### 5.2 Khởi động service

```bash
sudo systemctl daemon-reload
sudo systemctl enable orca-server
sudo systemctl start orca-server

# Kiểm tra
sudo systemctl status orca-server
sudo journalctl -u orca-server -f   # xem logs realtime
```

---

## 6. Lấy Pairing URL để phân phối cho Developer

```bash
# Lấy pairing URL/code từ logs
sudo journalctl -u orca-server --no-pager | grep -E "Pairing|Web UI|pair="

# Hoặc chạy với --json để parse tự động
orca serve --json --port 6768 --pairing-address wss://orca.vnpblc.internal
# Output JSON:
# {
#   "webUrl": "https://orca.vnpblc.internal/web-index.html?pair=...",
#   "pairingUrl": "orca://pair?code=abc123&endpoint=wss://orca.vnpblc.internal&pk=...",
#   "qrCode": "data:image/png;base64,...",
#   "port": 6768
# }
```

### 6.1 Script tạo pairing link mới

```bash
#!/bin/bash
# /opt/orca/scripts/generate-pairing-link.sh

echo "Generating new Orca pairing link..."
sudo systemctl restart orca-server
sleep 2
PAIRING=$(sudo journalctl -u orca-server -n 20 --no-pager | grep "Web UI:" | tail -1)
echo ""
echo "=== Share this link with developers ==="
echo $PAIRING
```

---

## 7. Cập nhật Pairing Token

Mỗi lần `orca serve` khởi động, nó tạo **token mới**. Để revoke access của tất cả client cũ → restart service:

```bash
# Rotate tất cả tokens (revoke tất cả sessions)
sudo systemctl restart orca-server

# Rồi tạo và phân phối pairing link mới cho team
```

---

## 8. Port và Firewall

```bash
# Chỉ mở port 443 (HTTPS qua Nginx) và 6768 (nội bộ)
sudo ufw default deny incoming
sudo ufw allow 22/tcp     # SSH quản trị
sudo ufw allow 443/tcp    # HTTPS (Nginx → Orca)
sudo ufw allow 80/tcp     # HTTP → redirect HTTPS
# Không mở 6768 ra ngoài! Nginx làm reverse proxy
sudo ufw enable
```

---

## 9. Checklist hoàn thành

- [ ] Node.js 22+ đã cài
- [ ] Git 2.35+ đã cài
- [ ] User `orca` đã tạo
- [ ] Orca binary đã download và có execute permission
- [ ] `orca serve` chạy thử thành công → thấy Pairing URL
- [ ] Systemd service đã tạo và đang chạy
- [ ] Nginx reverse proxy đã cấu hình (xem file 02)
- [ ] HTTPS certificate đã cài
- [ ] Pairing URL đã test kết nối từ browser
- [ ] Pairing URL đã gửi cho team
