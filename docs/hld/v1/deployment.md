# Deployment Architecture

**Tài liệu:** Deployment trên các platforms  
**Tham chiếu:** SRS Section 6.1  
**Cập nhật:** 2026-07-28

---

## 1. Supported Platforms

| Platform | Arch | Installer | Status |
|---------|------|---------|--------|
| macOS | arm64 (M1+) | .dmg | ✅ Primary |
| macOS | x64 (Intel) | .dmg | ✅ Supported |
| Windows | x64 | .exe (NSIS) | ✅ Supported |
| Linux | x64 | .AppImage / .deb | ✅ Supported |
| Linux | arm64 | .AppImage | ⚠️ Experimental |

---

## 2. Application Distribution

```
Distribution Channels:
  ├── Direct download (orca.ai website)
  ├── macOS: Homebrew Cask (brew install --cask orca)
  ├── Windows: WinGet / Scoop
  ├── Linux: Snap / .deb repository
  └── Auto-update: electron-updater (RELEASES.json feed)
```

---

## 3. File System Layout

### macOS
```
~/Library/Application Support/orca/
├── database.sqlite           # Main SQLite database
├── logs/                     # Application logs
├── daemon/
│   ├── daemon.sock           # Unix socket
│   ├── daemon.pid            # PID file
│   └── daemon.log            # Daemon logs
├── snapshots/                # Terminal scrollback snapshots
└── agent-cache/              # Agent binary cache

/Applications/Orca.app/       # Application bundle
```

### Linux
```
~/.config/orca/               # Config + database
~/.local/share/orca/          # Logs + daemon
~/.local/bin/orca             # CLI symlink
```

### Windows
```
%APPDATA%\orca\               # Config + database
%LOCALAPPDATA%\Programs\orca\ # Application
```

---

## 4. Remote Host Relay Layout

```
Remote Host (SSH target):
~/.local/bin/orca-relay       # Relay binary
~/.local/share/orca-relay/
├── relay.log                 # Rotating log (max 10MB)
├── version                   # Installed version
└── hooks/                    # Agent hook configs
```

---

## 5. Build Pipeline

```
Source Code (TypeScript)
       │
       ▼
Vite Build (renderer: React → JS bundle)
       │
       ▼
esbuild (main process + preload + relay: TS → CJS)
       │
       ▼
electron-builder
  ├── macOS: code sign + notarize → .dmg
  ├── Windows: code sign → .exe
  └── Linux: → .AppImage + .deb
       │
       ▼
Auto-update feed (RELEASES.json + delta updates)
```

---

## 6. Daemon Process Model

```
Orca Desktop Launch
       │
       ▼
Main Process checks: daemon running?
  ├── YES: connect to existing daemon.sock
  └── NO:  spawn daemon process
              │
              ▼
          daemon-entry.ts
              │
              ├── Open Unix socket: ~/.local/share/orca/daemon/daemon.sock
              ├── Initialize PTY manager
              ├── Load session state from SQLite
              └── Ready → write PID file
```

**Daemon Lifecycle:**
- Starts with app (or on first CLI command)
- Survives app restart (background daemon)
- Shutdown: SIGTERM → graceful (save sessions) → SIGKILL (5s timeout)
- Crash recovery: daemon-health watchdog restarts automatically

---

## 7. Network Ports

| Service | Port | Protocol | Binding |
|---------|------|---------|---------|
| Mobile WebSocket | 7624 (default) | WS (TLS optional) | 0.0.0.0 (LAN only) |
| Relay (on remote) | Random ephemeral | WS | 127.0.0.1 |
| CLI daemon HTTP | 7777 (optional) | HTTP | 127.0.0.1 |
| Agent hook server | Random | HTTP | 127.0.0.1 |
| **Orca Web HTTP** | **:6769** | **HTTP/HTTPS** | **0.0.0.0 (behind Nginx)** |
| **Orca Web WS** | **:6768** | **WebSocket** | **0.0.0.0 (behind Nginx)** |
| **Agent WS endpoint** | **:6768/agent** | **WebSocket** | **per above** |
| **Health/Metrics** | **:6769/health** | **HTTP** | **0.0.0.0** |

---

## 8. Update Strategy

```
electron-updater flow:
  1. Check update feed (RELEASES.json) every 24h
  2. Diff update: download only changed files
  3. Background download
  4. Show nudge notification when ready
  5. Install on next restart (or immediate if user clicks)
  
  macOS: sparkle-compatible, auto-install without elevation
  Windows: NSIS installer, may need UAC
  Linux: AppImage replace-on-update
```

---

## 9. CI/CD Integration

```
DevOps → Orca CLI → Orca Daemon (headless)
                         │
    orca worktree create --base main --agent claude --prompt "..."
                         │
                         ▼
                   [Agent runs in PTY]
                         │
                   orca agent wait --timeout 300
                         │
                   orca snapshot --worktree <id> > result.txt
                         │
                   orca worktree remove <id>
```

**Docker support:**
```dockerfile
FROM node:22-slim
RUN apt-get install -y git
COPY orca-cli /usr/local/bin/orca
RUN orca serve --daemon  # headless mode, no display
```

---

## 10. Web Server Deployment (Docker)

```
 deploy/prod/Dockerfile:

  FROM node:22-slim
  RUN apt-get install -y git openssh-client
  WORKDIR /app
  COPY --chown=node:node . .
  RUN npm ci --omit=dev
  USER node
  ENV NODE_ENV=production \
      ORCA_MULTI_USER=1 \
      ORCA_DB_URL=postgresql://orca:pass@db/orca
  EXPOSE 6769 6768
  HEALTHCHECK --interval=30s CMD curl -f http://localhost:6769/health/ready
  CMD ["node", "dist/server.js"]
```

**Docker Compose tối thiểu:**
```yaml
services:
  orca:
    image: orca-server:latest
    ports:
      - "6769:6769"
      - "6768:6768"
    environment:
      ORCA_MULTI_USER: "1"
      ORCA_DB_URL: "postgresql://orca:pass@db/orca"
    depends_on: [db]
  db:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: orca
      POSTGRES_USER: orca
      POSTGRES_PASSWORD: pass
```

**Nginx reverse proxy:**
```nginx
server {
    listen 443 ssl;
    server_name orca.example.com;
    ssl_certificate /etc/ssl/orca.crt;
    ssl_certificate_key /etc/ssl/orca.key;

    location / {
        proxy_pass http://localhost:6769;
        proxy_http_version 1.1;
    }
    location /ws {
        proxy_pass http://localhost:6768;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

---

## 11. Multi-User Server File Layout

```
~/.orca/
├── database.sqlite          # Desktop mode (single-user)
├── server/
│   ├── server.db              # Server DB (nếu SQLite mode)
│   ├── credentials.key       # Server encryption key
│   └── fleet/
│       └── orca-fleet.yaml    # Fleet config
└── users/
    ├── <userId-1>/
    │   ├── orca.sock          # Unix socket
    │   ├── orca.db            # Per-user SQLite
    │   ├── credentials.enc    # AES-256-GCM tokens
    │   └── worktrees/         # Per-user git worktrees
    └── <userId-2>/
        └── ...
```

---

## v5.0 — Deployment Extensions

### 12. v5.0 Environment Variables

**Orca Server (bổ sung v5.0):**
```bash
# Profile Hierarchy (F33)
# (dùng DB tables - không cần thêm env var)

# AI Provider credential key — dùng để decrypt khi relay credentials
# Chú ý: đây là key TRÊN ORCA SERVER để verify relay requests
# Key thực sự encrypt credentials nằm trên Dev Server
ORCA_AI_CREDENTIAL_VERIFY_KEY=<256-bit hex>

# Task Graph (F37)
ORCA_TASK_AI_DECOMPOSE_ENABLED=true
ORCA_TASK_MAX_DEPTH=10               # Max subtask nesting depth
ORCA_TASK_PUBLIC_SHARE_RATE_LIMIT=100  # req/min per share token

# Workflow Orchestration (F36)
ORCA_WORKFLOW_MAX_PARALLEL_STEPS=10
ORCA_WORKFLOW_DEFAULT_TIMEOUT_MS=1800000  # 30 min
ORCA_WORKFLOW_MAX_TIMEOUT_MS=7200000      # 2 hours
ORCA_WORKFLOW_TEMPLATE_APPROVAL=true      # require admin approval for company scope
```

**Dev Server (thêm vào mỗi dev server — F35):**
```bash
# AI Provider credential encryption key (per dev server, không share với Orca Server)
ORCA_AI_CREDENTIAL_KEY=<256-bit hex unique per dev server>
# Tạo: openssl rand -hex 32
```

---

### 13. v5.0 File Layout trên Dev Server

```
# Dev Server filesystem additions (v5.0)
~/.orca/
├── ai-providers/                       # AI Provider credentials (F35)
│   ├── <accountId-1>.enc              # AES-256-GCM encrypted API key
│   ├── <accountId-2>.enc
│   └── health-cache.json              # Last health check results
└── relay/                             # Relay binary (existing)
    └── orca-relay-linux-x64            # Auto-deployed by Orca Server
```

---

### 14. DB Schema Additions (v5.0 — Migrations 0006–0010)

```sql
-- Migration 0006: Company + Dept (F33)
CREATE TABLE orca_company (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  profile_json TEXT,          -- OrcaProfile JSON
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE orca_departments (
  id TEXT PRIMARY KEY,
  company_id TEXT NOT NULL REFERENCES orca_company(id),
  name TEXT NOT NULL,
  parent_dept_id TEXT REFERENCES orca_departments(id),
  profile_json TEXT,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Add dept_id to orca_users
ALTER TABLE orca_users ADD COLUMN dept_id TEXT REFERENCES orca_departments(id);
ALTER TABLE orca_users ADD COLUMN profile_json TEXT;

-- Migration 0007: Projects (F34)
CREATE TABLE orca_projects (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  dev_server_id TEXT NOT NULL REFERENCES orca_dev_servers(id),
  repo_path TEXT NOT NULL,
  default_branch TEXT DEFAULT 'main',
  visibility TEXT DEFAULT 'team',
  created_by TEXT REFERENCES orca_users(id),
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE orca_project_members (
  project_id TEXT NOT NULL REFERENCES orca_projects(id),
  user_id TEXT NOT NULL REFERENCES orca_users(id),
  role TEXT NOT NULL DEFAULT 'member',  -- owner|member|viewer
  PRIMARY KEY (project_id, user_id)
);

-- Migration 0008: AI Providers (F35)
CREATE TABLE orca_ai_provider_accounts (
  id TEXT PRIMARY KEY,
  dev_server_id TEXT NOT NULL REFERENCES orca_dev_servers(id),
  provider TEXT NOT NULL,              -- anthropic|openai|gemini|azure|bedrock|ollama|vllm
  scope TEXT NOT NULL DEFAULT 'server', -- server|project|user
  scope_ref_id TEXT,                   -- projectId or userId (null for server scope)
  label TEXT NOT NULL,
  model TEXT,                          -- default model for this account
  base_url TEXT,                       -- for Ollama/vLLM
  status TEXT DEFAULT 'pending',       -- pending|active|invalid|quota_exceeded|unreachable
  last_health_check DATETIME,
  quota_limit_day INTEGER DEFAULT 0,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE orca_provider_usage (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL REFERENCES orca_ai_provider_accounts(id),
  date TEXT NOT NULL,                  -- YYYY-MM-DD
  tokens_used INTEGER DEFAULT 0,
  requests INTEGER DEFAULT 0,
  cost_usd REAL DEFAULT 0,
  UNIQUE (account_id, date)
);

-- Migration 0009: Workflows (F36)
CREATE TABLE orca_workflow_templates (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  version INTEGER DEFAULT 1,
  scope TEXT NOT NULL DEFAULT 'personal', -- company|team|personal
  scope_ref_id TEXT,                   -- company_id or team_id
  parent_template_id TEXT REFERENCES orca_workflow_templates(id),
  visibility TEXT DEFAULT 'private',   -- private|team|company|public
  share_token TEXT UNIQUE,
  definition_json TEXT NOT NULL,       -- WorkflowDefinition YAML/JSON
  created_by TEXT REFERENCES orca_users(id),
  approved_at DATETIME,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE orca_workflow_executions (
  id TEXT PRIMARY KEY,
  template_id TEXT REFERENCES orca_workflow_templates(id),
  definition_snapshot TEXT NOT NULL,  -- frozen copy at execution time
  status TEXT DEFAULT 'pending',      -- pending|running|paused|completed|failed|cancelled
  inputs_json TEXT,
  current_wave INTEGER DEFAULT 0,
  triggered_by TEXT REFERENCES orca_users(id),
  project_id TEXT REFERENCES orca_projects(id),
  started_at DATETIME,
  completed_at DATETIME,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE orca_step_executions (
  id TEXT PRIMARY KEY,
  execution_id TEXT NOT NULL REFERENCES orca_workflow_executions(id),
  step_id TEXT NOT NULL,
  status TEXT DEFAULT 'pending',
  output_json TEXT,
  error TEXT,
  dev_server_id TEXT,
  account_id TEXT,
  started_at DATETIME,
  completed_at DATETIME
);

-- Migration 0010: Task Graph (F37)
CREATE TABLE orca_tasks (
  id TEXT PRIMARY KEY,
  type TEXT NOT NULL DEFAULT 'task',  -- epic|story|task|subtask|bug|spike
  title TEXT NOT NULL,
  description TEXT,
  status TEXT DEFAULT 'backlog',
  priority TEXT DEFAULT 'medium',
  parent_id TEXT REFERENCES orca_tasks(id),
  project_id TEXT REFERENCES orca_projects(id),
  assignee_id TEXT REFERENCES orca_users(id),
  reporter_id TEXT NOT NULL REFERENCES orca_users(id),
  estimated_hours REAL,
  actual_hours REAL,
  labels TEXT DEFAULT '[]',           -- JSON array
  prompt_template TEXT,
  ai_context TEXT,
  visibility TEXT DEFAULT 'team',
  pr_url TEXT,
  progress_percent INTEGER DEFAULT 0,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE orca_task_edges (
  from_task_id TEXT NOT NULL REFERENCES orca_tasks(id),
  to_task_id TEXT NOT NULL REFERENCES orca_tasks(id),
  edge_type TEXT NOT NULL DEFAULT 'depends_on',  -- depends_on|blocks|relates_to
  PRIMARY KEY (from_task_id, to_task_id, edge_type)
);

CREATE TABLE orca_task_grants (
  id TEXT PRIMARY KEY,
  task_id TEXT NOT NULL REFERENCES orca_tasks(id),
  scope TEXT NOT NULL,                -- user|team|company
  scope_id TEXT,                      -- userId or teamId (null for company)
  permission TEXT NOT NULL,           -- view|comment|edit|execute|manage
  apply_tree INTEGER DEFAULT 0,       -- 1 = cascade to all descendants
  expires_at DATETIME,
  created_by TEXT REFERENCES orca_users(id),
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE orca_task_comments (
  id TEXT PRIMARY KEY,
  task_id TEXT NOT NULL REFERENCES orca_tasks(id),
  user_id TEXT NOT NULL REFERENCES orca_users(id),
  body TEXT NOT NULL,
  comment_type TEXT DEFAULT 'comment', -- comment|activity|ai_output
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Indexes
CREATE INDEX idx_tasks_parent ON orca_tasks(parent_id);
CREATE INDEX idx_tasks_project ON orca_tasks(project_id);
CREATE INDEX idx_tasks_assignee ON orca_tasks(assignee_id);
CREATE INDEX idx_task_grants_task ON orca_task_grants(task_id);
CREATE INDEX idx_task_edges_from ON orca_task_edges(from_task_id);
CREATE INDEX idx_task_edges_to ON orca_task_edges(to_task_id);
CREATE INDEX idx_step_exec_execution ON orca_step_executions(execution_id);
CREATE INDEX idx_provider_usage_date ON orca_provider_usage(account_id, date);
```

---

### 15. Docker Compose — v5.0 Complete

```yaml
# docker-compose.v5.yml
version: '3.9'
services:
  orca:
    image: stablyai/orca:5.0
    ports:
      - "6768:6768"  # WebSocket RPC
      - "6769:6769"  # HTTP REST + Admin
    environment:
      # Core
      NODE_ENV: production
      ORCA_MULTI_USER: "1"
      ORCA_SERVER_SECRET: "${ORCA_SERVER_SECRET}"    # for WebCredentialStore
      ORCA_CREDENTIAL_KEY: "${ORCA_CREDENTIAL_KEY}"  # for per-user cred files
      # Database
      ORCA_DB_DSN: "postgresql://orca:${DB_PASS}@postgres:5432/orca"
      # AI Provider verify key (NOT the dev-server encryption key)
      ORCA_AI_CREDENTIAL_VERIFY_KEY: "${AI_VERIFY_KEY}"
      # Workflow
      ORCA_WORKFLOW_MAX_PARALLEL_STEPS: "10"
      ORCA_WORKFLOW_TEMPLATE_APPROVAL: "true"
      # Task Graph
      ORCA_TASK_MAX_DEPTH: "10"
      ORCA_TASK_AI_DECOMPOSE_ENABLED: "true"
    volumes:
      - orca_data:/app/data
    depends_on:
      postgres:
        condition: service_healthy

  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: orca
      POSTGRES_USER: orca
      POSTGRES_PASSWORD: "${DB_PASS}"
    volumes:
      - pg_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U orca"]
      interval: 10s
      timeout: 5s
      retries: 5

  nginx:
    image: nginx:1.25-alpine
    ports:
      - "443:443"
      - "80:80"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
      - ./certs:/etc/nginx/certs:ro
    depends_on:
      - orca

volumes:
  orca_data:
  pg_data:
```

---

### 16. Dev Server Setup (v5.0 — per server)

```bash
# Trên mỗi Dev Server, thêm vào environment (systemd unit hoặc .env)

# Tạo AI credential key (unique per server)
ORCA_AI_CREDENTIAL_KEY=$(openssl rand -hex 32)

# Relay binary sẽ đọc env này khi encrypt/decrypt AI credentials
# Đặt trong: /etc/orca/dev-server.env hoặc systemd EnvironmentFile

# Orca Server sẽ auto-deploy relay binary qua SSH
# Relay đọc ORCA_AI_CREDENTIAL_KEY từ environment của process
```
