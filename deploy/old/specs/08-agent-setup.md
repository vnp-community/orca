# Agent Setup — Triển khai Orca Agent lên Dev Server

**Mục tiêu:** Kết nối dev server `172.20.2.31` với Orca Server qua WebSocket Agent Protocol.  
Hỗ trợ 2 mode: `direct-websocket` (agent → Orca) và `relay-websocket` (Orca → agent).

Agent tự động discover các CLI tools đã cài và expose chúng qua JSON-RPC `tools/call`.

---

## 0. Tools Available trên dev server (172.20.2.31)

| Tool | Binary | Status | Mô tả |
|------|--------|--------|-------|
| `claude_code` | `~/.local/bin/claude` | ✅ v2.1.215 | Claude Code AI (--print mode) |
| `claude_code_file` | `~/.local/bin/claude` | ✅ v2.1.215 | Claude Code trên specific file |
| `gh` | `/usr/bin/gh` | ✅ authed (binhjax) | GitHub CLI — PR, issue, repo, api |
| `git` | `/usr/bin/git` | ✅ user=BinhNT | Git operations |
| `gitnexus` | `gitnexus` (npm global) | ✅ v1.6.9 | Code intelligence graph queries |
| `codegraph` | `~/.local/bin/codegraph` | ✅ v1.4.1 | Local code analysis |
| `docker` | `/usr/bin/docker` | ✅ | Container management |
| `shell` | `bash` | ✅ | Tùy ý shell command |
| `read_file` | built-in | ✅ | Đọc file từ dev server |
| `list_dir` | built-in | ✅ | List directory |

Agent sẽ discover tại startup — tool nào không tìm thấy binary sẽ bị skip.

## 1. Kiến trúc tổng quan

```
Mode 1: direct-websocket (khuyến nghị cho dev)
─────────────────────────────────────────────────────────────
 Dev Server 172.20.2.31          Orca Server b15.openledger.vn
   orca-agent (Node.js)  ─── wss://b15.openledger.vn/agent ──►
                          ─── agent.handshake(token) ─────────►
                          ◄── handshake-ok ───────────────────
                          ◄══ JSON-RPC framed ════════════════►
   
   Ưu điểm: không cần mở port inbound. Hoạt động qua NAT/firewall.
   Token TTL: 300s (5 phút) — sinh từ Orca admin API.

Mode 2: relay-websocket (khi Orca cần initiate)
─────────────────────────────────────────────────────────────
 Dev Server 172.20.2.31          Orca Server b15.openledger.vn
   agent.js :6799/orca-relay  ◄── ws://172.20.2.31:6799 ──────
                               ◄── Bearer token ───────────────
                               ─── agent.handshake ───────────►
                               ◄── handshake-ok ───────────────
                               ◄══ JSON-RPC framed ════════════►
   
   Yêu cầu: port 6799 mở trên dev server (hoặc SSH tunnel).
   Token: static secret trong .env (AGENT_RELAY_TOKEN).
```

---

## 2. Prerequisite trên dev server (172.20.2.31)

```bash
# Cài Node.js 22 LTS
curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash -
sudo apt-get install -y nodejs

# Kiểm tra
node --version  # >= 18.0.0

# Tạo thư mục agent
mkdir -p ~/orca-agent/logs
```

---

## 3. Deploy agent (một lần)

Từ **máy dev** (macOS):

```bash
# Copy agent lên dev server
rsync -avz --exclude node_modules \
  deploy/dev/agent/ \
  ubuntu@172.20.2.31:~/orca-agent/

# Cài dependencies
ssh ubuntu@172.20.2.31 "cd ~/orca-agent && npm install --production"
```

Hoặc dùng script tự động:
```bash
bash deploy/dev/scripts/connect-agent.sh --deploy
```

---

## 4. Kết nối: direct-websocket mode

### Bước 1 — Sinh token từ Orca server

```bash
# Script sẽ sinh token 300s và in lệnh chạy agent
bash deploy/dev/scripts/connect-agent.sh
```

Output:
```
 🔗 Orca Agent — direct-websocket mode
═══════════════════════════════════════════════════════════
Dev Server: 172.20.2.31
Orca URL:   wss://b15.openledger.vn/agent
Token:      agt-dev-local-1722033600000
Expires:    300s

Chạy lệnh này trên dev server (172.20.2.31):
  ORCA_URL=wss://b15.openledger.vn/agent \
  AGENT_TOKEN=agt-dev-local-1722033600000 \
  node agent.js
```

### Bước 2 — Chạy agent trên dev server

```bash
# SSH vào dev server
ssh ubuntu@172.20.2.31

# Chạy agent với token đã sinh
cd ~/orca-agent
ORCA_URL=wss://b15.openledger.vn/agent \
AGENT_TOKEN=agt-dev-local-XXXX \
node agent.js
```

### Hoặc: auto deploy + start

```bash
bash deploy/dev/scripts/connect-agent.sh --deploy
# → deploy + sinh token + SSH vào dev server + khởi động agent
```

### Bước 3 — Thêm Dev Server trong Orca UI

1. Mở Orca web UI → Settings → Dev Servers
2. Click **Add Dev Server**
3. Chọn **Connection Type: direct-websocket**
4. Click **Test Connection** → Orca sẽ sinh token và hiển thị lệnh agent
5. Chạy lệnh agent theo hướng dẫn trong UI
6. Click **Add** khi agent connected

---

## 5. Kết nối: relay-websocket mode

### Bước 1 — Cấu hình token

Trong `deploy/dev/.env`:
```bash
AGENT_RELAY_TOKEN=my-secret-token-here  # đổi thành random string
AGENT_PORT=6799
```

### Bước 2 — Mở port trên dev server

```bash
# Nếu dùng ufw:
sudo ufw allow 6799/tcp

# Nếu dùng iptables:
sudo iptables -A INPUT -p tcp --dport 6799 -j ACCEPT
```

### Bước 3 — Chạy agent

```bash
# Trên dev server 172.20.2.31:
cd ~/orca-agent
AGENT_PORT=6799 AGENT_TOKEN=my-secret-token-here \
MODE=relay-websocket \
node agent.js

# Agent lắng nghe tại: ws://172.20.2.31:6799/orca-relay
```

### Bước 4 — Thêm Dev Server trong Orca UI

1. Orca UI → Settings → Dev Servers → **Add Dev Server**
2. Connection Type: **relay-websocket**
3. WebSocket URL: `ws://172.20.2.31:6799/orca-relay?token=my-secret-token-here`
4. Click **Test Connection** → Orca kết nối vào agent

---

## 6. Management commands

```bash
# Kiểm tra agent đang chạy
bash deploy/dev/scripts/connect-agent.sh --status

# Xem logs (50 dòng cuối)
bash deploy/dev/scripts/connect-agent.sh --logs

# Dừng agent
bash deploy/dev/scripts/connect-agent.sh --stop

# Khởi động lại (direct-ws)
bash deploy/dev/scripts/connect-agent.sh --start

# Khởi động lại (relay-ws)
bash deploy/dev/scripts/connect-agent.sh --start --mode=relay-ws
```

---

## 7. Chạy agent như systemd service (production)

```bash
# Tạo file service trên dev server
sudo tee /etc/systemd/system/orca-agent.service << 'EOF'
[Unit]
Description=Orca Dev Server Agent
After=network.target

[Service]
Type=simple
User=ubuntu
WorkingDirectory=/home/ubuntu/orca-agent
Environment=ORCA_URL=wss://b15.openledger.vn/agent
Environment=AGENT_TOKEN=FILL_IN_TOKEN_HERE
Environment=DEV_SERVER_ID=dev-local
Environment=LOG_LEVEL=info
ExecStart=/usr/bin/node /home/ubuntu/orca-agent/agent.js
Restart=on-failure
RestartSec=10
StandardOutput=append:/home/ubuntu/orca-agent/logs/agent.log
StandardError=append:/home/ubuntu/orca-agent/logs/agent.log

[Install]
WantedBy=multi-user.target
EOF

# Enable và start
sudo systemctl daemon-reload
sudo systemctl enable orca-agent
sudo systemctl start orca-agent
sudo systemctl status orca-agent
```

> **Lưu ý:** Với `direct-websocket`, token expire sau 300s. Không nên dùng systemd với token tĩnh.  
> Dùng systemd phù hợp hơn với `relay-websocket` (token static trong `.env`).

---

## 8. Troubleshooting

| Triệu chứng | Nguyên nhân | Fix |
|------------|-------------|-----|
| `AGENT_TOKEN is required` | Chưa set env var | Chạy `connect-agent.sh` lấy token mới |
| `Handshake REJECTED` | Token hết hạn | Token TTL 300s — sinh lại |
| `Connection refused` | Agent chưa chạy | Chạy `connect-agent.sh --start` |
| `WebSocket error: ENOTFOUND` | DNS không resolve | Kiểm tra ORCA_URL |
| Relay-ws: `Unauthorized` | Sai AGENT_RELAY_TOKEN | Match với ORCA UI URL `?token=xxx` |

```bash
# Debug mode
LOG_LEVEL=debug ORCA_URL=... AGENT_TOKEN=... node agent.js

# Kiểm tra connectivity từ dev server
curl -v --http1.1 -H "Upgrade: websocket" \
  -H "Connection: Upgrade" \
  -H "Sec-WebSocket-Key: test" \
  -H "Sec-WebSocket-Version: 13" \
  https://b15.openledger.vn/agent
```
