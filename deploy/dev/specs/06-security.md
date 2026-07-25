# Security — Bảo mật Web-First Deployment

**Mô hình:** Orca Server expose Web UI + WebSocket qua HTTPS (Nginx). Developer kết nối bằng Pairing Token (E2E encrypted).

---

## 1. Mô hình bảo mật tổng thể

```
┌────────────────── TRUST BOUNDARY 1: Network ───────────────────────┐
│                                                                     │
│  Internet / LAN → Nginx (TLS) → Orca Server                        │
│                                                                     │
│  - TLS 1.3 trên toàn bộ traffic                                    │
│  - Nginx không forward traffic chưa authenticate                    │
│  - Orca Server chỉ bind 127.0.0.1 (không expose trực tiếp)        │
└─────────────────────────────────────────────────────────────────────┘

┌────────────────── TRUST BOUNDARY 2: Pairing ───────────────────────┐
│                                                                     │
│  Client (Browser/App) ←→ Orca Server                               │
│                                                                     │
│  - Token-based pairing (random 32 bytes)                           │
│  - Curve25519 keypair exchange (TweetNaCl)                         │
│  - Derived shared secret → E2E encryption per session              │
│  - Token expire khi server restart                                  │
└─────────────────────────────────────────────────────────────────────┘

┌────────────────── TRUST BOUNDARY 3: Agent ─────────────────────────┐
│                                                                     │
│  Orca Server → AI Agents (Claude, Gemini...)                       │
│                                                                     │
│  - Trust presets: Minimal / Standard / Full                        │
│  - API keys không bao giờ log                                      │
│  - Agent runs với user permissions (no root)                       │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 2. TLS / HTTPS Security

### 2.1 Cấu hình Nginx TLS cứng

```nginx
# /etc/nginx/sites-available/orca-server
server {
    listen 443 ssl http2;
    server_name orca.vnpblc.internal;

    ssl_certificate     /etc/letsencrypt/live/orca.vnpblc.internal/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/orca.vnpblc.internal/privkey.pem;

    # TLS 1.3 only (hoặc 1.2 minimum)
    ssl_protocols TLSv1.2 TLSv1.3;

    # Strong cipher suites
    ssl_ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384:ECDHE-ECDSA-CHACHA20-POLY1305:ECDHE-RSA-CHACHA20-POLY1305;
    ssl_prefer_server_ciphers off;

    # HSTS (6 tháng, gồm subdomains)
    add_header Strict-Transport-Security "max-age=15768000; includeSubDomains" always;

    # OCSP stapling
    ssl_stapling on;
    ssl_stapling_verify on;
    resolver 8.8.8.8 8.8.4.4 valid=300s;

    # Session
    ssl_session_cache shared:SSL:10m;
    ssl_session_timeout 1d;
    ssl_session_tickets off;

    # Security headers
    add_header X-Frame-Options SAMEORIGIN always;
    add_header X-Content-Type-Options nosniff always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;
    add_header Content-Security-Policy "default-src 'self' wss://orca.vnpblc.internal; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline';" always;
}
```

### 2.2 Test SSL cấu hình

```bash
# Test TLS config
openssl s_client -connect orca.vnpblc.internal:443 -tls1_3

# Test với SSL Labs (nếu có public domain)
# https://www.ssllabs.com/ssltest/analyze.html?d=orca.vnpblc.com
# Mục tiêu: Grade A+
```

---

## 3. Pairing Token Security

### 3.1 Cách Pairing Token hoạt động

```
1. orca serve khởi động → tạo token mới (32 random bytes)
2. Token nhúng vào pairing URL: orca://pair?code=TOKEN&pk=SERVER_PUBKEY
3. Client gửi TOKEN + client public key lên server
4. Server verify TOKEN → derive shared secret (Curve25519)
5. Mọi message sau đó: encrypted với shared secret (XSalsa20-Poly1305)
6. Token expire: server restart, hoặc explicit revoke
```

### 3.2 Phân phối Token an toàn

- ✅ Gửi qua **Slack DM** (private channel)
- ✅ Gửi qua **email** (encrypted nếu có thể)
- ✅ Hiện **QR code** trực tiếp trên màn hình cho dev scan
- ❌ Không post lên channel public
- ❌ Không commit vào git
- ❌ Không để trong file log không được bảo vệ

### 3.3 Revoke Token (khi cần)

```bash
# Revoke tất cả active sessions → restart server
sudo systemctl restart orca-server

# Tạo link mới và phân phối lại cho team
bash /opt/orca/scripts/get-pairing-links.sh
```

### 3.4 Per-developer tokens (Advanced)

Để revoke từng developer riêng, chạy nhiều Orca instances:

```bash
# Instance cho team Alpha
orca serve --port 6768 --pairing-address wss://orca-alpha.vnpblc.internal &

# Instance cho team Beta
orca serve --port 6769 --pairing-address wss://orca-beta.vnpblc.internal &

# Revoke team Alpha → restart instance Alpha only, không ảnh hưởng Beta
```

---

## 4. Network Security

### 4.1 Firewall rules

```bash
# Orca Server: chỉ mở 22 (SSH admin), 80 (redirect), 443 (HTTPS)
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow 22/tcp      # SSH quản trị (restrict IP nếu có thể)
sudo ufw allow 80/tcp      # HTTP → redirect HTTPS
sudo ufw allow 443/tcp     # HTTPS (Nginx)
# PORT 6768 KHÔNG mở! Chỉ bind 127.0.0.1, Nginx proxy
sudo ufw enable

# Restrict SSH chỉ từ IP của DevOps
sudo ufw allow from 10.0.0.5 to any port 22  # DevOps machine IP
```

### 4.2 Orca Server bind localhost only

```bash
# orca serve mặc định bind 0.0.0.0:6768
# Restrict về localhost (Nginx sẽ proxy):
orca serve --port 6768 --pairing-address wss://orca.vnpblc.internal
# Kiểm tra:
ss -tnlp | grep 6768
# Phải thấy: 127.0.0.1:6768 (không phải 0.0.0.0:6768)
```

### 4.3 Rate limiting qua Nginx

```nginx
# /etc/nginx/nginx.conf
limit_req_zone $binary_remote_addr zone=orca_ws:10m rate=10r/s;

server {
    location /ws {
        limit_req zone=orca_ws burst=20 nodelay;
        # ... proxy config ...
    }
}
```

---

## 5. Bảo vệ AI API Keys

### 5.1 Quy tắc

- **KHÔNG** hardcode API key trong code
- **KHÔNG** commit vào git (dù private repo)
- **KHÔNG** expose trong Orca logs
- Mỗi developer có **key riêng** (hoặc team key riêng cho project)

### 5.2 Set key qua systemd environment (server-side)

```ini
# /etc/systemd/system/orca-server.service
[Service]
# Keys được set tại đây, không log ra stdout
EnvironmentFile=/etc/orca/secrets.env   # file riêng, chmod 600

# /etc/orca/secrets.env
ANTHROPIC_API_KEY=sk-ant-...
GEMINI_API_KEY=...
```

```bash
sudo mkdir -p /etc/orca
sudo nano /etc/orca/secrets.env
sudo chmod 600 /etc/orca/secrets.env
sudo chown orca:orca /etc/orca/secrets.env
```

### 5.3 Developer set key riêng (client-side)

Trong Orca Web UI hoặc Desktop:
- Settings → Agents → Claude Code → **API Key**
- Key được lưu **encrypted trong session** của client (không gửi lên server)
- Nếu dùng web: lưu trong localStorage (encrypted bởi pairing shared secret)

---

## 6. Access Control

### 6.1 Phân quyền bằng Orca Instance riêng

| Team | Orca Instance | Domain | Port |
|------|---------------|--------|------|
| Team Alpha (vnp-blc) | orca-alpha | orca-alpha.vnpblc.internal | 6768 |
| Team Beta (vnp-ai-ops) | orca-beta | orca-beta.vnpblc.internal | 6769 |
| Tech Leads (all) | orca-admin | orca-admin.vnpblc.internal | 6770 |

Mỗi instance có token riêng → chỉ developer nhận được token của instance đó mới vào được.

### 6.2 Phân quyền bằng Agent Trust Presets

Orca có 3 cấp độ trust cho agent:

| Tier | Tên | Quyền |
|------|-----|-------|
| 0 | **Minimal** | Chỉ read, không write, không exec shell |
| 1 | **Standard** | Read + write trong worktree, exec hạn chế |
| 2 | **Full** | Toàn quyền (network, system calls) |

Cấu hình trong Orca: Settings → Agents → Trust Preset → chọn Tier

> **Khuyến nghị:** Junior dev → Standard. Senior/Lead → Full (cần thiết).

---

## 7. Monitoring và Audit

### 7.1 Nginx Access Logs

```bash
# Xem real-time access
tail -f /var/log/nginx/orca-access.log

# Log format thêm thông tin
# /etc/nginx/nginx.conf
log_format orca_combined '$remote_addr - $remote_user [$time_local] '
                         '"$request" $status $body_bytes_sent '
                         '"$http_referer" "$http_user_agent" '
                         '$request_time';
```

### 7.2 Orca Server Logs

```bash
# Realtime logs
sudo journalctl -u orca-server -f

# Xem connections mới (pairing events)
sudo journalctl -u orca-server | grep -E "paired|connected|disconnected"
```

### 7.3 Cảnh báo khi có connection bất thường

```bash
# Script check số WebSocket connections
#!/bin/bash
# /opt/orca/scripts/check-connections.sh
CONN_COUNT=$(ss -tnp | grep "6768" | wc -l)
if [ $CONN_COUNT -gt 50 ]; then
  echo "⚠️  WARNING: $CONN_COUNT connections to Orca Server" | \
    curl -s -X POST https://hooks.slack.com/services/XXX \
    -d "{\"text\": \"⚠️ Orca: $CONN_COUNT connections\"}"
fi
```

---

## 8. Incident Response

### 8.1 Revoke tất cả access ngay lập tức

```bash
# 1. Stop Orca Server → tất cả sessions bị drop
sudo systemctl stop orca-server

# 2. Review logs trước khi restart
sudo journalctl -u orca-server -n 100 --no-pager

# 3. Restart với token mới
sudo systemctl start orca-server

# 4. Phân phối pairing link mới chỉ cho người được phép
bash /opt/orca/scripts/get-pairing-links.sh
```

### 8.2 Phát hiện token leak

Nếu nghi ngờ pairing URL bị leak:
1. Restart Orca Server ngay (invalidate token cũ)
2. Audit Nginx access logs: tìm IP lạ
3. Phân phối token mới chỉ qua kênh an toàn

---

## 9. Security Checklist

### DevOps setup
- [ ] Nginx TLS cấu hình đúng (TLS 1.3, HSTS, security headers)
- [ ] Orca Server chỉ bind 127.0.0.1 (không expose trực tiếp)
- [ ] Firewall: chỉ mở 22, 80, 443
- [ ] SSL cert hợp lệ (không warning trong browser)
- [ ] secrets.env file chmod 600, chỉ đọc bởi user orca

### Phân phối token
- [ ] Pairing URL gửi qua kênh private (không public Slack channel)
- [ ] Mỗi team/project nhận token riêng
- [ ] Lịch rotate token: mỗi sprint / mỗi tháng

### Ongoing
- [ ] Review Nginx access logs tuần/tháng
- [ ] Update Orca khi có bản vá bảo mật
- [ ] Ngay khi developer nghỉ → restart Orca instance của team đó
