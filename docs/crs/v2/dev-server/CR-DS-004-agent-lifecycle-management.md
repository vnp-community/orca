# CR-DS-004 — Dev Server Agent Lifecycle Management

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-DS-004 |
| **Tên** | Dev Server Agent — Installation, Lifecycle & Version Management |
| **Loại** | Operational Specification |
| **Priority** | P0 — Critical |
| **Phiên bản** | v6.0 |
| **Ngày tạo** | 2026-07-30 |
| **Trạng thái** | Proposed |
| **Phụ thuộc** | CR-DS-001, CR-DS-002 |
| **Tác động HLD** | deployment.md |

---

## 1. Agent Package

### Build artifacts

```
orca-agent/
├── orca-agent-linux-x64          # Linux x64 binary (Node.js compiled)
├── orca-agent-linux-arm64        # Linux arm64
├── orca-agent-darwin-arm64       # macOS Apple Silicon
├── orca-agent-darwin-x64         # macOS Intel
├── orca-agent-windows-x64.exe    # Windows
└── checksums.sha256              # integrity verification
```

### Công nghệ build

```
Node.js 22 → pkg (pkg.js) hoặc ncc (vercel/ncc) → single binary
  - Bao gồm: node-pty, better-sqlite3, ws, ssh2
  - Không bao gồm: Electron, React, UI components
  - Size mục tiêu: < 80 MB
```

---

## 2. Deployment Methods

### Method A: systemd (Linux server — recommended)

```bash
# Install script (chạy trên dev server)
curl -fsSL https://orca-backend.company.com/agent/install.sh | \
  ORCA_BACKEND_URL=wss://orca-backend.company.com \
  ORCA_AGENT_TOKEN=<token-from-admin-panel> \
  bash

# Install script thực hiện:
# 1. Download binary phù hợp với OS/arch
# 2. Verify checksum
# 3. Install tới /usr/local/bin/orca-agent
# 4. Create systemd unit file
# 5. Enable và start service
```

```ini
# /etc/systemd/system/orca-agent.service
[Unit]
Description=Orca Dev Server Agent
After=network.target
Wants=network-online.target

[Service]
Type=simple
User=orca-agent
ExecStart=/usr/local/bin/orca-agent start \
  --backend-url wss://orca-backend.company.com \
  --token-file /etc/orca-agent/token
Restart=always
RestartSec=5
StartLimitIntervalSec=60
StartLimitBurst=5
Environment=ORCA_AI_CREDENTIAL_KEY=<key>
Environment=ORCA_AGENT_DATA_DIR=/var/lib/orca-agent
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

### Method B: Docker (containerized dev server)

```yaml
# docker-compose.yml
services:
  orca-agent:
    image: orca/agent:6.0
    restart: unless-stopped
    environment:
      - ORCA_BACKEND_URL=wss://orca-backend.company.com
      - ORCA_AGENT_TOKEN_FILE=/run/secrets/agent_token
      - ORCA_AI_CREDENTIAL_KEY=${AI_CREDENTIAL_KEY}
      - ORCA_WORKSPACE_ROOT=/workspace
    volumes:
      - /workspace:/workspace    # repo mount
      - agent-data:/var/lib/orca-agent
      - /var/run/docker.sock:/var/run/docker.sock  # for F18 ephemeral VM
    secrets:
      - agent_token
    ports:
      - "127.0.0.1:6790:6790"  # local-only diagnostic port
    networks:
      - orca-net

volumes:
  agent-data:
secrets:
  agent_token:
    file: ./secrets/agent_token
```

### Method C: macOS launchd (macOS dev machine as dev server)

```xml
<!-- ~/Library/LaunchAgents/com.orca.agent.plist -->
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>com.orca.agent</string>
  <key>ProgramArguments</key>
  <array>
    <string>/usr/local/bin/orca-agent</string>
    <string>start</string>
    <string>--backend-url</string>
    <string>wss://orca-backend.company.com</string>
  </array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>EnvironmentVariables</key>
  <dict>
    <key>ORCA_AI_CREDENTIAL_KEY</key><string>__KEY__</string>
  </dict>
</dict>
</plist>
```

---

## 3. Agent Registration Flow

```
Admin → Orca Backend Admin Panel → "Add Dev Server"
  │
  ├─ Admin fills: server name, hostname/IP, connection type
  │
  ├─ Backend generates: agentId (UUID), registrationToken (32 bytes)
  │
  ├─ Admin copies install command (includes token)
  │
  ├─ Admin runs install command on dev server
  │
  ├─ Agent starts → attempts first connection
  │
  ├─ Backend validates registrationToken → marks agent REGISTERED
  │
  ├─ Backend issues permanent agentSecret (HMAC-signed)
  │
  └─ Agent stores agentSecret securely → uses for all future connections

                                [Agent appears in Fleet list as ONLINE]
```

---

## 4. File System Layout on Dev Server

```
/usr/local/bin/
└── orca-agent              # Binary

/var/lib/orca-agent/        # ORCA_AGENT_DATA_DIR
├── agent.db                # Local SQLite (worktrees, sessions, task runs)
├── agent.pid               # PID file
├── logs/
│   ├── agent.log           # Rotating (10MB × 5)
│   └── audit.log           # RPC audit log (append-only)
├── ai-providers/           # AI provider credentials
│   ├── <accountId1>.enc    # AES-256-GCM encrypted
│   └── <accountId2>.enc
├── sessions/               # Active PTY sessions state
│   └── <sessionId>.state
└── worktrees/              # Worktree metadata cache
    └── <projectId>/

/etc/orca-agent/
├── config.yaml             # Agent configuration
└── token                   # Registration token (0600 permissions)
```

### agent config.yaml

```yaml
# /etc/orca-agent/config.yaml
agent:
  id: "agent-uuid-here"         # auto-generated on install
  name: "dev-server-alpha"      # display name

backend:
  url: "wss://orca-backend.company.com"
  reconnectIntervalMs: 5000
  maxReconnectIntervalMs: 60000

workspace:
  allowedRoots:              # security: only these paths accessible
    - "/workspace"
    - "/home/developer/repos"

pty:
  defaultShell: "/bin/bash"
  maxConcurrent: 50

health:
  reportIntervalMs: 60000
  cpuSampleIntervalMs: 1000

logging:
  level: "info"              # debug | info | warn | error
  maxFileSizeMb: 10
  maxFiles: 5

security:
  allowedCommands:           # whitelist for shell steps (F36)
    - "npm"
    - "yarn"
    - "pnpm"
    - "python"
    - "go"
    - "cargo"
    - "make"
  disallowShellOperators: true   # prevent && | ; $ ` injection
```

---

## 5. Agent Update Mechanism

### Admin-triggered update (from Backend Admin Panel)

```
Admin → Admin Panel → Fleet → [Server] → [Update Agent]
  │
  ├─ Backend sends: rpc.call('agent.requestUpdate', {
  │    version: '6.1.0',
  │    downloadUrl: 'https://orca-backend.company.com/agent/releases/6.1.0/linux-x64',
  │    checksum: 'sha256:...'
  │  })
  │
  ├─ Agent:
  │   1. Download new binary to temp path
  │   2. Verify checksum
  │   3. Test binary (--version flag)
  │   4. Send event.log: "Update ready, restarting in 10s"
  │   5. Drain active connections (wait max 30s)
  │   6. systemd restart → new binary takes over
  │
  └─ Backend: health check after 60s → verify new version in handshake
```

### Auto-update (optional, opt-in)

```yaml
# config.yaml
update:
  autoUpdate: true
  channel: "stable"     # stable | rc | beta
  checkIntervalHours: 24
  allowedUpdateWindow:
    start: "02:00"      # 2 AM
    end: "04:00"        # 4 AM
    timezone: "Asia/Ho_Chi_Minh"
```

### Version Compatibility Matrix

| Backend Version | Agent Version | Status |
|----------------|--------------|--------|
| 6.x | 6.x | ✅ Full support |
| 6.x | 5.x | ⚠️ Limited (thin relay features only) |
| 5.x | 6.x | ❌ Not supported (agent won't connect) |
| 6.0 | 6.1 | ✅ Minor version backward compat |

---

## 6. Health & Monitoring

### Agent states

| State | Mô tả |
|-------|-------|
| `ONLINE` | Connected, heartbeat OK, fully operational |
| `DEGRADED` | Connected but some capabilities failed (e.g., git not found) |
| `OFFLINE` | No heartbeat > 90s |
| `UPDATING` | Performing self-update |
| `DRAINING` | Accepting no new sessions, completing active ones |
| `ERROR` | Fatal error, needs manual intervention |

### Alerting

```typescript
// Backend → Alert when agent goes OFFLINE
// Config:
interface AgentAlertConfig {
  notifyOnOffline: boolean          // default: true
  notifyRecipients: 'team-lead' | 'admin' | 'both'
  offlineGracePeriodSeconds: 120    // delay before alerting
  webhookUrl?: string               // optional webhook
  slackChannel?: string
}
```

---

## 7. Acceptance Criteria

- [ ] Install script hoàn thành < 2 phút (download + setup)
- [ ] systemd service auto-starts sau server reboot
- [ ] Agent reconnects về Backend < 60s sau network loss
- [ ] Agent tiếp tục PTY sessions khi disconnected từ Backend
- [ ] Update không làm gián đoạn active PTY sessions (> 30s grace)
- [ ] Config file validation tại startup (rõ ràng nếu sai)
- [ ] Diagnostic endpoint `/health` trên localhost:6790
- [ ] Audit log tất cả RPC calls với userId + outcome
- [ ] allowedRoots enforced: không thể access ngoài configured paths
