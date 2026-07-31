# TDD-AG-08: Deployment

**Version:** 4.0  
**Date:** 2026-07-28  
**Source:** `deploy/dev/agent/`

---

## 1. File Structure

```
deploy/dev/agent/
├─ agent.js          ← Main agent script (Node.js, no build required)
├─ package.json      ← { "dependencies": { "ws": "^8" } }
├─ start.sh          ← Auto-token-fetch script (bash)
├─ agent.service     ← systemd unit file template
└─ README.md         ← Setup instructions
```

---

## 2. start.sh

```bash
#!/bin/bash
# start.sh — Fetch fresh token and start agent
# Called by systemd ExecStart

set -euo pipefail

ORCA_HOST="${ORCA_HOST:-orca-server.internal}"
ORCA_HTTP_PORT="${ORCA_HTTP_PORT:-6769}"
ORCA_AGENT_API_SECRET="${ORCA_AGENT_API_SECRET:-}"
DEV_SERVER_ID="${DEV_SERVER_ID:-$(hostname)}"
AGENT_NAME="${AGENT_NAME:-$(hostname)}"
WORK_DIR="${WORK_DIR:-$HOME}"

# Fetch new token from Orca Server
TOKEN_RESPONSE=$(curl -sf \
  -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ORCA_AGENT_API_SECRET" \
  -d "{\"devServerId\": \"$DEV_SERVER_ID\", \"name\": \"$AGENT_NAME\", \"ttl\": 300}" \
  "https://$ORCA_HOST:$ORCA_HTTP_PORT/api/agent-token")

AGENT_TOKEN=$(echo "$TOKEN_RESPONSE" | node -e "process.stdin.resume(); let d=''; process.stdin.on('data', c=>d+=c); process.stdin.on('end', ()=>console.log(JSON.parse(d).token))")

export AGENT_TOKEN
export ORCA_URL="wss://$ORCA_HOST:$ORCA_HTTP_PORT/agent"
export WORK_DIR

cd /opt/orca
exec node agent.js
```

---

## 3. systemd Service

```ini
# /etc/systemd/system/orca-agent.service

[Unit]
Description=Orca Dev Server Agent
After=network.target

[Service]
Type=simple
User=orca-agent
WorkingDirectory=/opt/orca
EnvironmentFile=/opt/orca/agent.env
ExecStart=/opt/orca/start.sh

# Restart on failure
Restart=on-failure
RestartSec=5
StartLimitInterval=60
StartLimitBurst=5   # Max 5 restarts in 60s

# Resource limits
MemoryMax=256M
CPUQuota=50%

[Install]
WantedBy=multi-user.target
```

---

## 4. agent.env

```bash
# /opt/orca/agent.env — secrets file (chmod 600)

ORCA_HOST=orca-server.internal
ORCA_HTTP_PORT=6769
ORCA_AGENT_API_SECRET=<secret>   # Bearer token cho POST /api/agent-token
DEV_SERVER_ID=prod-server        # Unique ID trong Orca Server
AGENT_NAME=Production Server
WORK_DIR=/home/orca-agent
```

---

## 5. Manual Setup Steps

```bash
# 1. Install Node.js (v18+)
curl -fsSL https://deb.nodesource.com/setup_20.x | sudo bash -
sudo apt-get install -y nodejs

# 2. Create agent user
sudo useradd -m -s /bin/bash orca-agent

# 3. Copy agent files
sudo mkdir -p /opt/orca
sudo cp agent.js package.json start.sh /opt/orca/
sudo chown -R orca-agent:orca-agent /opt/orca
sudo chmod +x /opt/orca/start.sh

# 4. Install dependencies
cd /opt/orca && sudo -u orca-agent npm install --production

# 5. Configure env
sudo -u orca-agent nano /opt/orca/agent.env
sudo chmod 600 /opt/orca/agent.env

# 6. Install systemd service
sudo cp agent.service /etc/systemd/system/orca-agent.service
sudo systemctl daemon-reload
sudo systemctl enable orca-agent
sudo systemctl start orca-agent

# 7. Check status
sudo systemctl status orca-agent
journalctl -u orca-agent -f
```

---

## 6. Auto-Provisioning (via Fleet Bootstrap)

```
FleetBootstrapService (server-side) tự động:
1. SSH vào remote server
2. Upload agent.js + start.sh + package.json
3. npm install
4. Install systemd service
5. Start service
6. Health check: verify agent connects
```

---

## 7. Upgrade Process

```bash
# Upgrade agent.js (zero-downtime):
# 1. Đặt agent.js mới vào /opt/orca/agent.js
# 2. systemctl restart orca-agent
# 3. start.sh fetch fresh token → agent.js new version connects
```

---

## 8. Troubleshooting

```bash
# View logs
journalctl -u orca-agent -n 100 --no-pager

# Common issues:
# 1. "AGENT_TOKEN is required" → start.sh failed to fetch token
#    → Check ORCA_AGENT_API_SECRET, Orca Server URL, network connectivity

# 2. "Handshake timeout" → Token expired before agent connected
#    → Check system time sync (NTP), reduce network latency

# 3. "Connection refused" → Orca Server not reachable
#    → Check firewall rules for port 6769

# 4. Restart loop → Check: StartLimitBurst exceeded? → systemctl reset-failed orca-agent
```

---

## 9. Security Hardening

```bash
# Restrict agent user
sudo usermod -s /sbin/nologin orca-agent   # No interactive login

# Strict env file
sudo chown root:orca-agent /opt/orca/agent.env
sudo chmod 640 /opt/orca/agent.env

# No sudo for agent user
# agent.env không chứa AGENT_TOKEN (đây là ephemeral — fetch mỗi lần start)
```
