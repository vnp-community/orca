# Web Access Setup — HTTPS, Nginx Reverse Proxy và Pairing

**Mục tiêu:** Cấu hình Nginx làm reverse proxy cho Orca Server, bật HTTPS/TLS để developer truy cập Web UI an toàn qua browser.

---

## 1. Kiến trúc Nginx ↔ Orca

```
Internet (Developer)
       │
       │ HTTPS :443  (hoặc HTTP :80 redirect)
       ▼
┌──────────────────────────────────┐
│  Nginx (Reverse Proxy)           │
│  TLS termination                 │
│  WebSocket upgrade support       │
└────────────────┬─────────────────┘
                 │
        HTTP + WebSocket
        ws://127.0.0.1:6768
                 │
                 ▼
┌──────────────────────────────────┐
│  Orca Server (orca serve)        │
│  Port 6768                       │
│  Web UI bundle + WS runtime      │
└──────────────────────────────────┘
```

**Quan trọng:** `orca serve` cần Nginx để:
1. Cung cấp TLS (wss:// thay vì ws://) → Browser yêu cầu HTTPS cho mixed-content an toàn
2. Serve static Web UI bundle qua HTTPS
3. Expose 1 port duy nhất ra ngoài (443)

---

## 2. Cài Nginx

```bash
sudo apt-get install -y nginx
sudo systemctl enable nginx
sudo systemctl start nginx
```

---

## 3. Cài TLS Certificate

### 3.1 Dùng Let's Encrypt (miễn phí, cho domain public)

```bash
# Cài certbot
sudo apt-get install -y certbot python3-certbot-nginx

# Cấp certificate (thay domain thực)
sudo certbot --nginx -d orca.vnpblc.com --email admin@vnpblc.com --agree-tos --non-interactive

# Verify auto-renewal
sudo certbot renew --dry-run
```

### 3.2 Dùng Self-Signed Certificate (cho internal/dev)

```bash
# Tạo self-signed cert (dùng cho domain .internal)
sudo openssl req -x509 -nodes -days 3650 -newkey rsa:4096 \
  -keyout /etc/ssl/private/orca.vnpblc.internal.key \
  -out /etc/ssl/certs/orca.vnpblc.internal.crt \
  -subj "/C=VN/ST=HCM/L=HoChiMinh/O=VNPBlc/CN=orca.vnpblc.internal"

# Developer cần add cert vào trusted store của máy
# (Hoặc dùng Nginx cert + internal CA)
```

### 3.3 Dùng Internal CA (Production recommended)

```bash
# Nếu có internal PKI/CA, issue certificate từ CA đó
# Tất cả developer machine đã trust internal CA → không cần warning
openssl req -new -key orca.vnpblc.internal.key \
  -out orca.vnpblc.internal.csr \
  -subj "/CN=orca.vnpblc.internal"
# → Gửi CSR cho CA team để sign
```

---

## 4. Cấu hình Nginx

### 4.1 Tạo Nginx config cho Orca

```nginx
# /etc/nginx/sites-available/orca-server
server {
    listen 80;
    server_name orca.vnpblc.internal orca.vnpblc.com;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name orca.vnpblc.internal orca.vnpblc.com;

    # TLS
    ssl_certificate     /etc/ssl/certs/orca.vnpblc.internal.crt;
    ssl_certificate_key /etc/ssl/private/orca.vnpblc.internal.key;
    ssl_protocols       TLSv1.2 TLSv1.3;
    ssl_ciphers         HIGH:!aNULL:!MD5;
    ssl_session_cache   shared:SSL:10m;
    ssl_session_timeout 10m;

    # Security headers
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header X-Frame-Options SAMEORIGIN always;
    add_header X-Content-Type-Options nosniff always;

    # ─────────────────────────────────────────────
    # Orca Web UI (static files)
    # ─────────────────────────────────────────────
    location / {
        proxy_pass         http://127.0.0.1:6768;
        proxy_http_version 1.1;
        proxy_set_header   Host $host;
        proxy_set_header   X-Real-IP $remote_addr;
        proxy_set_header   X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto $scheme;

        # Cache tĩnh cho assets
        proxy_cache_bypass $http_upgrade;
    }

    # ─────────────────────────────────────────────
    # Orca WebSocket Runtime (ws:// → wss://)
    # ─────────────────────────────────────────────
    location /runtime {
        proxy_pass         http://127.0.0.1:6768;
        proxy_http_version 1.1;

        # WebSocket upgrade (quan trọng!)
        proxy_set_header   Upgrade $http_upgrade;
        proxy_set_header   Connection "upgrade";
        proxy_set_header   Host $host;
        proxy_set_header   X-Real-IP $remote_addr;

        # Timeout dài cho WebSocket sessions
        proxy_read_timeout  3600s;
        proxy_send_timeout  3600s;
        proxy_connect_timeout 10s;

        # Không buffer WebSocket frames
        proxy_buffering off;
    }

    # ─────────────────────────────────────────────
    # Orca WebSocket Pairing (ws:// → wss://)
    # ─────────────────────────────────────────────
    location /ws {
        proxy_pass         http://127.0.0.1:6768;
        proxy_http_version 1.1;
        proxy_set_header   Upgrade $http_upgrade;
        proxy_set_header   Connection "upgrade";
        proxy_set_header   Host $host;
        proxy_read_timeout  3600s;
        proxy_send_timeout  3600s;
        proxy_buffering off;
    }

    # Access logs
    access_log /var/log/nginx/orca-access.log;
    error_log  /var/log/nginx/orca-error.log;
}
```

### 4.2 Kích hoạt config

```bash
sudo ln -s /etc/nginx/sites-available/orca-server /etc/nginx/sites-enabled/
sudo nginx -t   # Test config
sudo systemctl reload nginx
```

---

## 5. Cấu hình DNS (nội bộ)

### 5.1 Với internal DNS server

```dns
# Thêm vào DNS server nội bộ (Pi-hole, Bind9, dnsmasq)
orca.vnpblc.internal  A  10.10.0.100   # IP của Orca Server
```

### 5.2 Với /etc/hosts (không có DNS server)

DevOps thông báo để mỗi developer thêm vào `/etc/hosts`:

```
# Thêm vào /etc/hosts trên máy developer
10.10.0.100  orca.vnpblc.internal
```

macOS/Linux:
```bash
echo "10.10.0.100  orca.vnpblc.internal" | sudo tee -a /etc/hosts
```

Windows (PowerShell as Admin):
```powershell
Add-Content -Path "C:\Windows\System32\drivers\etc\hosts" -Value "10.10.0.100  orca.vnpblc.internal"
```

---

## 6. Khởi động Orca với đúng pairing-address

Sau khi có HTTPS domain, restart Orca với `--pairing-address` trỏ vào domain đó:

```bash
# Cập nhật systemd service
sudo nano /etc/systemd/system/orca-server.service
```

Sửa dòng ExecStart:
```ini
ExecStart=/home/orca/.local/bin/orca serve \
  --port 6768 \
  --pairing-address wss://orca.vnpblc.internal
```

```bash
sudo systemctl daemon-reload
sudo systemctl restart orca-server
```

Verify output:
```bash
sudo journalctl -u orca-server -n 20 --no-pager
# ✓ Web UI: https://orca.vnpblc.internal/web-index.html?pair=...
# ✓ Pairing URL: orca://pair?code=...&endpoint=wss://orca.vnpblc.internal
```

---

## 7. Phân phối Pairing URL cho Developer

### 7.1 Script lấy link nhanh

```bash
#!/bin/bash
# /opt/orca/scripts/get-pairing-links.sh

echo ""
echo "======================================"
echo " Orca Server — Pairing Links          "
echo "======================================"

# Lấy từ systemd logs
LOGS=$(sudo journalctl -u orca-server -n 50 --no-pager)

WEB_URL=$(echo "$LOGS" | grep "Web UI:" | tail -1 | sed 's/.*Web UI: //')
PAIRING_URL=$(echo "$LOGS" | grep "Pairing URL:" | tail -1 | sed 's/.*Pairing URL: //')

echo ""
echo "📌 Web Browser URL:"
echo "   $WEB_URL"
echo ""
echo "📌 Orca Desktop Pairing URL:"
echo "   $PAIRING_URL"
echo ""
echo "======================================"
echo "Gửi một trong hai link trên cho developer."
echo "Browser link tự động embedded pairing data."
```

### 7.2 Gửi cho developer

**Option A: Gửi Web URL** (đơn giản nhất)
```
Gửi link này qua Slack:
https://orca.vnpblc.internal/web-index.html?pair=orca%3A%2F%2Fpair%3Fcode%3Dabc123...
→ Developer mở link → tự động kết nối ✅
```

**Option B: Gửi QR Code** (cho Orca Mobile)
```bash
# Generate QR code từ pairing URL
npm install -g qrcode-terminal
qrcode-terminal "orca://pair?code=abc123..."
# → In QR ra terminal/Slack → developer scan bằng Orca Mobile
```

---

## 8. Nhiều project / Nhiều Orca instance

Nếu cần nhiều Orca instance riêng (mỗi project 1 instance):

```nginx
# Orca Alpha (Project vnp-blc)
server {
    listen 443 ssl;
    server_name orca-alpha.vnpblc.internal;
    location / { proxy_pass http://127.0.0.1:6768; ... }
    location /ws { proxy_pass http://127.0.0.1:6768; ... }
}

# Orca Beta (Project vnp-ai-ops)
server {
    listen 443 ssl;
    server_name orca-beta.vnpblc.internal;
    location / { proxy_pass http://127.0.0.1:6769; ... }
    location /ws { proxy_pass http://127.0.0.1:6769; ... }
}
```

```bash
# Chạy 2 orca serve instances trên 2 port khác nhau
orca serve --port 6768 --pairing-address wss://orca-alpha.vnpblc.internal  &
orca serve --port 6769 --pairing-address wss://orca-beta.vnpblc.internal   &
```

---

## 9. Kiểm tra

```bash
# Test HTTPS
curl -k https://orca.vnpblc.internal/

# Test WebSocket (dùng wscat)
npm install -g wscat
wscat -c wss://orca.vnpblc.internal/ws

# Test từ browser
# Mở Chrome → https://orca.vnpblc.internal → phải hiện Orca Web UI
```

---

## 10. Checklist

- [ ] Nginx đã cài và running
- [ ] TLS certificate đã cài (Let's Encrypt / self-signed / internal CA)
- [ ] Nginx config đúng, test pass (`nginx -t`)
- [ ] DNS record `orca.vnpblc.internal` trỏ đúng IP
- [ ] `orca serve --pairing-address wss://orca.vnpblc.internal` đang chạy
- [ ] Mở browser → `https://orca.vnpblc.internal` → thấy Orca Web UI ✅
- [ ] Pairing URL copied và test kết nối thành công
- [ ] Pairing URL đã phân phối cho team
