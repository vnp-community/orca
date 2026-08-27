# Troubleshooting — Xử lý Sự cố (Web-First Model)

---

## 1. Không mở được Web UI trong Browser

### ❌ "ERR_CONNECTION_REFUSED" hoặc "Site can't be reached"

```bash
# Kiểm tra Orca Server có đang chạy không
sudo systemctl status orca-server
sudo journalctl -u orca-server -n 20 --no-pager

# Kiểm tra Nginx
sudo systemctl status nginx
sudo nginx -t

# Kiểm tra port 6768 đang listen
ss -tnlp | grep 6768

# Kiểm tra port 443 Nginx đang mở
ss -tnlp | grep :443
```

**Giải pháp phổ biến:**
```bash
# Orca Server bị crash → restart
sudo systemctl restart orca-server

# Nginx config lỗi → fix và reload
sudo nginx -t
sudo systemctl reload nginx
```

---

### ❌ "SSL: CERTIFICATE_VERIFY_FAILED" hoặc "NET::ERR_CERT_AUTHORITY_INVALID"

**Nguyên nhân:** Self-signed certificate chưa được trust.

**Giải pháp cho Developer (nếu dùng self-signed cert):**

macOS:
```bash
# Thêm cert vào Keychain
sudo security add-trusted-cert -d -r trustRoot \
  -k /Library/Keychains/System.keychain \
  orca.vnpblc.internal.crt
```

Chrome (bypass tạm):
```
Mở URL → Click "Advanced" → Click "Proceed to orca.vnpblc.internal (unsafe)"
```

Firefox:
```
Mở URL → "Advanced" → "Accept the Risk and Continue"
```

**Giải pháp lâu dài:** Dùng Let's Encrypt hoặc internal CA (developer machine đã trust CA).

---

### ❌ Browser hiện "Mixed Content" error

```
This HTTPS page cannot connect to a plain ws:// Orca server.
Open the web client over HTTP or pair with a wss:// endpoint.
```

**Nguyên nhân:** Orca Server được start với `--pairing-address ws://` thay vì `wss://`.

**Giải pháp:**
```bash
# Sửa systemd service: thay ws:// bằng wss://
sudo nano /etc/systemd/system/orca-server.service

# ExecStart phải là:
ExecStart=... orca serve --pairing-address wss://orca.vnpblc.internal

sudo systemctl daemon-reload && sudo systemctl restart orca-server
```

---

## 2. Pairing / Kết nối thất bại

### ❌ "Enter a valid Orca pairing URL or pairing code"

**Nguyên nhân:** Pairing URL không đúng format.

Format đúng:
```
orca://pair?code=TOKEN&endpoint=wss://host&pk=BASE64KEY
```

```bash
# Lấy lại pairing URL đúng từ server
sudo journalctl -u orca-server -n 30 --no-pager | grep -E "Web UI|Pairing URL|pair="
```

---

### ❌ "Connection timed out" khi paste pairing URL

**Kiểm tra:**
```bash
# Từ máy developer (test kết nối đến server)
curl -k https://orca.vnpblc.internal/
# Nếu curl timeout → vấn đề network

# Test DNS
nslookup orca.vnpblc.internal
ping orca.vnpblc.internal

# Test port 443
nc -zv orca.vnpblc.internal 443
```

**Giải pháp:**
```bash
# Thêm vào /etc/hosts nếu chưa có DNS
echo "10.10.0.100  orca.vnpblc.internal" | sudo tee -a /etc/hosts
```

---

### ❌ "Pairing token expired" hoặc "Invalid token"

**Nguyên nhân:** Server đã restart → token cũ không còn hiệu lực.

**Giải pháp:**
1. Xin DevOps pairing URL mới
2. Trong Orca Web UI: click **"Clear saved server"** → nhập URL mới
3. Trong Orca Desktop: Settings → Remove old environment → Add new pairing URL

---

### ❌ "This QR code grants limited (mobile) access"

**Nguyên nhân:** Bạn đang dùng **Mobile QR code** để kết nối trên Desktop/Browser.

**Giải pháp:**
```
Yêu cầu DevOps cấp Web Access URL (không phải mobile QR).
URL Web có dạng: https://orca.../web-index.html?pair=orca://pair?code=...&endpoint=wss://...
```

---

## 3. Orca Web UI bị treo / không load

### ❌ Trang trắng hoặc loading mãi

```bash
# 1. Mở DevTools (F12) → Console → xem error message

# Thường thấy:
# "WebSocket connection failed" → vấn đề WebSocket proxy (Nginx)
# "Failed to load module script" → web bundle bị thiếu

# 2. Kiểm tra Nginx WebSocket config
sudo nginx -t
sudo grep -A 5 "Upgrade" /etc/nginx/sites-available/orca-server

# Phải có:
# proxy_set_header Upgrade $http_upgrade;
# proxy_set_header Connection "upgrade";
```

---

### ❌ Terminal trong Orca Web không hiện hoặc bị lag

**Nguyên nhân:** WebSocket proxy timeout quá ngắn.

```nginx
# Sửa Nginx config: tăng timeout
location /ws {
    proxy_read_timeout  3600s;  # 1 giờ
    proxy_send_timeout  3600s;
    proxy_connect_timeout 10s;
}

sudo systemctl reload nginx
```

---

## 4. AI Agent Issues

### ❌ Agent không start hoặc "Claude Code not found"

```bash
# SSH vào Orca Server → kiểm tra claude-code cài chưa
ssh orca@orca-server
which claude
claude --version

# Nếu chưa cài:
npm install -g @anthropic-ai/claude-code
```

---

### ❌ "ANTHROPIC_API_KEY not set"

**Option A: Set server-side (shared key)**
```bash
# Sửa /etc/orca/secrets.env
sudo nano /etc/orca/secrets.env
# Thêm: ANTHROPIC_API_KEY=sk-ant-...
sudo systemctl restart orca-server
```

**Option B: Set client-side (per-developer)**
- Orca Web UI: Settings → Agents → Claude Code → API Key field
- Orca Desktop: Settings → API Keys → Claude

---

### ❌ Agent bị rate limited (429 Too Many Requests)

**Trong Orca UI:**
- Xem Usage panel → thời gian reset rate limit
- Đổi sang agent khác tạm thời (Gemini, Codex)

**Sửa dài hạn:**
- Dùng key tier cao hơn (Claude Pro, API credits)
- Phân phối usage: mỗi developer dùng key riêng

---

## 5. Worktree Issues

### ❌ Tạo worktree thất bại

```bash
# SSH vào dev server từ orca server
ssh dev-alpha

# Kiểm tra git status
git -C /srv/projects/vnp-blc status
git -C /srv/projects/vnp-blc worktree list

# Clean up orphaned worktrees
git -C /srv/projects/vnp-blc worktree prune

# Kiểm tra disk space
df -h /srv/projects
```

---

### ❌ Không thấy project trong Orca

**Nguyên nhân:** Project chưa được mount hoặc Orca Server không có quyền truy cập SSH vào dev server.

```bash
# Trên Orca Server: test SSH vào dev server
ssh -i /home/orca/.ssh/dev_key dev@dev-alpha.vnpblc.internal "echo OK"

# Nếu fail → kiểm tra SSH key của orca-server có trong authorized_keys của dev-alpha
```

---

## 6. Performance Issues

### ❌ Giao diện Web lag

**Kiểm tra:**
```bash
# Latency từ developer machine đến server
ping orca.vnpblc.internal

# Bandwidth test (từ dev machine)
curl -o /dev/null https://orca.vnpblc.internal/ -w "%{time_total}\n"

# Server load
ssh orca-server "uptime && free -h"
```

**Giải pháp:**
- Nếu mạng chậm → kết nối VPN khác, hoặc dùng Orca Desktop (native WebSocket, ít overhead hơn browser)
- Nếu server overload → scale up, giảm số worktrees đang chạy

---

### ❌ Nginx timeout khi agent chạy lâu

```nginx
# Tăng timeout trong Nginx
location /ws {
    proxy_read_timeout  7200s;   # 2 giờ
    proxy_send_timeout  7200s;
}

# Restart Nginx
sudo systemctl reload nginx
```

---

## 7. Quick Diagnostic Script

```bash
#!/bin/bash
# /opt/orca/scripts/diagnose.sh
# Chạy trên Orca Server để kiểm tra nhanh

echo "=== Orca Server Diagnostics ==="
echo ""

# Systemd service
echo "1. Orca Server Service:"
systemctl is-active orca-server && echo "  ✅ Running" || echo "  ❌ STOPPED"

# Port
echo ""
echo "2. Port 6768 (Orca):"
ss -tnlp | grep 6768 > /dev/null && echo "  ✅ Listening" || echo "  ❌ Not listening"

# Nginx
echo ""
echo "3. Nginx Service:"
systemctl is-active nginx && echo "  ✅ Running" || echo "  ❌ STOPPED"

# Port 443
echo ""
echo "4. Port 443 (HTTPS):"
ss -tnlp | grep :443 > /dev/null && echo "  ✅ Listening" || echo "  ❌ Not listening"

# TLS cert
echo ""
echo "5. TLS Certificate:"
echo | openssl s_client -connect localhost:443 -servername orca.vnpblc.internal 2>/dev/null \
  | openssl x509 -noout -dates 2>/dev/null | grep notAfter \
  | xargs -I{} echo "  ✅ Cert: {}" \
  || echo "  ⚠️  Could not check cert"

# Pairing URL
echo ""
echo "6. Current Pairing URL:"
journalctl -u orca-server -n 50 --no-pager | grep "Web UI:" | tail -1 | sed 's/^/  /'

# Active connections
echo ""
echo "7. Active WebSocket connections:"
WS_COUNT=$(ss -tnp | grep ":6768" | wc -l)
echo "  $WS_COUNT connections"

echo ""
echo "=== Done ==="
```

```bash
chmod +x /opt/orca/scripts/diagnose.sh
sudo bash /opt/orca/scripts/diagnose.sh
```

---

## 8. Liên hệ hỗ trợ

| Vấn đề | Liên hệ |
|--------|---------|
| Pairing URL không hoạt động | DevOps team (#devops Slack) |
| Server performance | DevOps team |
| Orca app crash / bugs | https://github.com/stablyai/orca/issues |
| API key, agent issues | Tự xử lý hoặc Team Lead |
| Không vào được project | Team Lead của project |
| Khẩn cấp | @devops-oncall trên Slack |
