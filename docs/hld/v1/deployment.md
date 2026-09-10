# Deployment Architecture

**Tài liệu:** Deployment trên các platforms  
**Tham chiếu:** SRS Section 6.1  
**Cập nhật:** 2026-07-28 (corrections 2026-08-14, xem ghi chú dưới)

> **Ghi chú đối chiếu code (2026-08-14):** Nội dung §1–§9 (desktop/Electron distribution) chưa được đối chiếu lại trong đợt này. §10 trở đi (Web Server / v5.0 extensions) đã được đối chiếu trực tiếp với `backend/src/**`, `deploy/prod/`, `deploy/dev/`, và lịch sử deploy thật gần đây (`git log`) — dùng nhãn ✅ (implemented) / ⚠️ (partial/corrected) / 🚧 (proposed, chưa implement) / ❌ (không tồn tại trong code). Nguồn: `audit/backend/backend-vs-design-review.md`, và trực tiếp `docs/adrs/v2/ADR-020-enterprise-rollout-phases-backward-compat.md` (tự khai `❌ Chưa implement`).

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
| **Orca Web HTTP** | **:6769** (`ORCA_HTTP_PORT`) | **HTTP/HTTPS** | **0.0.0.0 (behind Nginx)** |
| **Orca Web WS** | **:6768** (`ORCA_PORT`), single-user mode only | **WebSocket** | **0.0.0.0 (behind Nginx)** |
| **Agent WS endpoint** | **:6769/agent** ⚠️ corrected — was documented as `:6768/agent` | **WebSocket** | **httpPort, see below** |
| **Health/Metrics** | **:6769/health** | **HTTP** | **0.0.0.0** |

⚠️ **Port correction (verified against `backend/src/server/index.ts:46-47,106,108`):** `AgentWebSocketServer` attaches to `httpPort` (default 6769), not `rpcPort` (6768) — the code's own log line confirms `ws://0.0.0.0:${httpPort}/agent`. Additionally, when `ORCA_MULTI_USER=1`, browser WebSocket connections are ALSO routed through `httpPort` (6769) via `WsSessionRouter`, not `rpcPort` (6768) — the "Orca Web WS :6768" row above only holds for single-user mode. See `audit/backend/backend-vs-design-review.md` §2.2/§2.4/§2.5 for full detail.

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

> ⚠️ The Dockerfile/Compose/Nginx snippets in this section are simplified illustrative examples,
> not a literal reproduction of the real files. §10a below documents how Orca is actually built,
> shipped, and run today — grounded directly in `deploy/prod/`, `deploy/dev/`, and recent deploy
> history (`git log`).

```
 deploy/prod/Dockerfile (illustrative, simplified):

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

## 10a. How Orca Is Actually Deployed Today (✅ real, grounded in repo)

Orca ships **two parallel, real deployment paths** — both exist in `deploy/` today, not as proposals:

**1. Release image (`deploy/prod/`)** — the packaged product, versioned via `deploy/prod/.env.example`:
```
deploy/prod/
├── Dockerfile              # multi-stage build FROM SOURCE
├── docker-compose.yml       # single-container: orca-server (SQLite by default)
├── entrypoint.sh
└── scripts/
    ├── deploy.sh
    ├── admin-setup.sh        # first-run admin creation helper
    └── health-check.sh
```
Defaults: `ORCA_MULTI_USER=0` (single-user PairCode mode, backward compat), SQLite (no DB config needed), `ORCA_AUTH_MODE=none`. Postgres/MySQL/TiDB are opt-in via `ORCA_DB_URL` (commented out in `.env.example`).

**2. Live operational deployment (`deploy/dev/`)** — what actually runs the team's own multi-user server today (`b15.openledger.vn`), driven by `deploy/dev/.env` and two build/ship strategies:
```
deploy/dev/scripts/
├── build-local.sh                # builds backend/out/server + frontend/out/web locally
├── sync-to-server.sh              # rsyncs FULL source tree; server builds via Docker (docker-compose.orca.yml)
└── sync-to-server-artifact.sh     # builds LOCALLY, rsyncs ONLY build output + manifests,
                                   # server does a single-stage artifact build
                                   # (docker-compose.orca.artifact.yml + Dockerfile.artifact)
```
This deployment runs `ORCA_MULTI_USER=1`, `ORCA_AUTH_MODE=local`, and — as of the two most recent deploy-related commits on this branch — a **shared Postgres database** (not SQLite), because `ORCA_MULTI_USER=1` forks one process per authenticated user, and without a shared DB each fork would get its own isolated SQLite file (breaking Team/OrcaProject sharing and task grants across users):

```
7987228b4  fix(deploy): tạm tắt ORCA_DB_URL — migrations SQLite không tương thích Postgres
cbdf8b455  fix(backend): BUG-BE-RPC-003 — migrations + pg-adapter sửa xong cho Postgres, bật lại
```
i.e. Postgres was briefly disabled because the migration files weren't dialect-portable, then re-enabled once the migrations + `pg-adapter.ts`'s `?`→`$N` placeholder translation were fixed and verified end-to-end (see `docs/guides/database/postgres-shared-database-design.md` and the comment in `docker-compose.orca.artifact.yml`).

**Real env vars in active use** (from `deploy/dev/.env`, confirmed read by `backend/src/main/db/config-loader.ts` and `backend/src/main/credentials/index.ts`):
```bash
ORCA_DOMAIN=wss://b15.openledger.vn
ORCA_PORT=6768                    # WS/RPC
ORCA_HTTP_PORT=6769                # HTTP
ORCA_MULTI_USER=1
ORCA_AUTH_MODE=local
ORCA_SERVER_SECRET=...             # WebCredentialStore AES-256-GCM master secret (real; NOT "ORCA_CREDENTIAL_KEY")
ORCA_DB_USER / ORCA_DB_PASSWORD / ORCA_DB_NAME   # shared Postgres — real env var accepted by
                                                   # config-loader.ts is ORCA_DB_URL (DSN form),
                                                   # NOT "ORCA_DB_DSN" as §15 below used to say
ORCA_AGENT_API_SECRET=...          # static shared secret for self-service agent token issuance
AGENT_ORCA_URL=wss://b15.openledger.vn/agent   # direct-websocket agent connection target
```

**What is NOT real:** the "v5.0 Environment Variables" in §12 below (`ORCA_AI_CREDENTIAL_VERIFY_KEY`, `ORCA_TASK_*`, `ORCA_WORKFLOW_*`) have zero references anywhere in `backend/src/` — they describe config surface that was designed but never wired up to be read by the server. Treat that whole list as aspirational until a corresponding `process.env[...]` read exists in code.

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

> ⚠️ **Mixed real/aspirational content below.** §12–16 were written as a forward-looking design
> spec alongside F33/F35/F36/F37. Verified against `backend/src/` (`grep` for each var name,
> 2026-08-14): `ORCA_AI_CREDENTIAL_KEY` (Dev Server side) and the migrations in §14 are real.
> **`ORCA_AI_CREDENTIAL_VERIFY_KEY`, `ORCA_TASK_AI_DECOMPOSE_ENABLED`, `ORCA_TASK_MAX_DEPTH`,
> `ORCA_TASK_PUBLIC_SHARE_RATE_LIMIT`, `ORCA_WORKFLOW_MAX_PARALLEL_STEPS`,
> `ORCA_WORKFLOW_DEFAULT_TIMEOUT_MS`, `ORCA_WORKFLOW_MAX_TIMEOUT_MS`, `ORCA_WORKFLOW_TEMPLATE_APPROVAL`
> have ZERO references in `backend/src/`** — none of these knobs are read by the server today; the
> corresponding features (Task Graph limits, Workflow parallelism/timeouts) run on hardcoded
> defaults in code, not configurable via env var. Treat this list as a proposed config surface,
> not a current one.

### 12. v5.0 Environment Variables — 🚧 mostly proposed (see banner above)

**Orca Server (bổ sung v5.0) — 🚧 none of these are read by `backend/src/` today:**
```bash
# Profile Hierarchy (F33)
# (dùng DB tables - không cần thêm env var)

# AI Provider credential key — dùng để decrypt khi relay credentials
# Chú ý: đây là key TRÊN ORCA SERVER để verify relay requests
# Key thực sự encrypt credentials nằm trên Dev Server
ORCA_AI_CREDENTIAL_VERIFY_KEY=<256-bit hex>          # 🚧 not read anywhere in backend/src/

# Task Graph (F37)
ORCA_TASK_AI_DECOMPOSE_ENABLED=true                   # 🚧 not read anywhere in backend/src/
ORCA_TASK_MAX_DEPTH=10               # Max subtask nesting depth   # 🚧 not read
ORCA_TASK_PUBLIC_SHARE_RATE_LIMIT=100  # req/min per share token   # 🚧 not read

# Workflow Orchestration (F36)
ORCA_WORKFLOW_MAX_PARALLEL_STEPS=10                   # 🚧 not read
ORCA_WORKFLOW_DEFAULT_TIMEOUT_MS=1800000  # 30 min    # 🚧 not read
ORCA_WORKFLOW_MAX_TIMEOUT_MS=7200000      # 2 hours    # 🚧 not read
ORCA_WORKFLOW_TEMPLATE_APPROVAL=true      # require admin approval for company scope  # 🚧 not read
```

**Dev Server (thêm vào mỗi dev server — F35) — ✅ real, confirmed `agent/src/relay/agent-credential-store.ts:9,45`:**
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
├── credentials/                        # ⚠️ corrected — real path (was documented as ai-providers/)
│   ├── <accountId-1>.enc              # AES-256-GCM encrypted API key
│   ├── <accountId-2>.enc
│   └── health-cache.json              # Last health check results
└── relay/                             # Relay binary (existing)
    └── orca-relay-linux-x64            # Auto-deployed by Orca Server
```
⚠️ The path `~/.orca/ai-providers/` previously documented here only exists in `agent/src/relay/ai-provider-handler.ts` — a dead-code file with zero callers (confirmed via impact analysis: `impactedCount: 0`) that also falsely claims AES-256-GCM encryption without performing it. The live implementation with real callers is `agent/src/relay/agent-credential-store.ts`, which uses `~/.orca/credentials/<accountId>.enc`.

---

### 14. DB Schema Additions (Migrations 0006–0017 — corrected count and names)

> ⚠️ **Corrected against real migration files** (`backend/src/main/db/migrations/`, 17 files as of
> 2026-08-14, not 10). The `CREATE TABLE`/`ALTER TABLE` bodies below are illustrative of the
> intent (approximately right) but **two table names are wrong** — see inline notes. Full
> file:line evidence in `audit/backend/backend-vs-design-review.md` §2.7.
>
> Real migration files, for reference: `0001_initial_schema.ts` (settings, projects, repos,
> ssh_targets), `0002_add_automations.ts`, `0003_add_workspace_sessions.ts`,
> `0004_orca_app_tables.ts` (`orca_projects`, `orca_repos`, `orca_ssh_targets`,
> `orca_global_settings` — this is what occupies the name `orca_projects` first, which is why
> migration 0007 below had to rename to `orca_v5_projects`), `0005_add_auth_schema.ts`
> (`orca_users`, `orca_sessions`, `orca_audit_log`, `orca_access_policies`), then 0006–0017 below.

```sql
-- Migration 0006: Company + Dept (F33)
CREATE TABLE orca_companies (            -- ⚠️ corrected: real table is PLURAL "orca_companies",
  id TEXT PRIMARY KEY,                   --   not "orca_company" as previously documented
  name TEXT NOT NULL,
  profile_json TEXT,          -- OrcaProfile JSON
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE orca_departments (
  id TEXT PRIMARY KEY,
  company_id TEXT NOT NULL REFERENCES orca_companies(id),
  name TEXT NOT NULL,
  parent_dept_id TEXT REFERENCES orca_departments(id),
  profile_json TEXT,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
-- Also creates orca_user_profiles (not previously documented)

-- Add dept_id to orca_users
ALTER TABLE orca_users ADD COLUMN dept_id TEXT REFERENCES orca_departments(id);
ALTER TABLE orca_users ADD COLUMN profile_json TEXT;

-- Migration 0007: Projects (F34)
CREATE TABLE orca_v5_projects (          -- ⚠️ corrected: real table is "orca_v5_projects", NOT
  id TEXT PRIMARY KEY,                   --   "orca_projects" — that name was already taken by
  name TEXT NOT NULL,                    --   migration 0004's orca_projects table; code renamed
  dev_server_id TEXT NOT NULL REFERENCES orca_dev_servers(id),  -- to avoid the collision
  repo_path TEXT NOT NULL,
  default_branch TEXT DEFAULT 'main',
  visibility TEXT DEFAULT 'team',
  created_by TEXT REFERENCES orca_users(id),
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE orca_v5_project_members (   -- ⚠️ corrected: real table is "orca_v5_project_members"
  project_id TEXT NOT NULL REFERENCES orca_v5_projects(id),
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
  project_id TEXT,                     -- ⚠️ real migration has NO FK constraint on this column
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
  project_id TEXT REFERENCES orca_v5_projects(id),  -- ⚠️ corrected: real FK target is orca_v5_projects
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

**Migrations 0011–0017 (not covered by the original v5.0 design doc — added since, all ✅ real):**

| # | Table(s) / change | Purpose |
|---|---|---|
| 0011 | `orca_terminal_sessions` | Terminal scrollback/session persistence |
| 0012 | `orca_port_forwards`, `orca_push_subscriptions` | Saved port forwards + Web Push subscriptions |
| 0013 | `orca_workflow_executions.root_trace_id` | F40 trace-correlation across server restarts |
| 0014 | `orca_workflow_executions.paused_at` | Workflow pause/resume support |
| 0015 | `orca_ai_provider_accounts.rotation_grace_until` | AI provider key-rotation grace period (see security.md §5.2) |
| 0016 | `orca_teams`, `orca_team_members.priority`, `orca_project_source_projects`, `orca_tasks.active_execution_task_id`/`agent_session_id` | Team/OrcaProject sharing + task execution linkage |
| 0017 | `orca_teams.profile_json` | Team-level profile hierarchy |

Total as of 2026-08-14: **17 migrations** (`backend/src/main/db/migrations/0001`–`0017`), not 10.

---

### 15. Docker Compose — v5.0 Complete

> ⚠️ Corrected: `ORCA_CREDENTIAL_KEY` (previously listed below) does not exist in code — the real
> WebCredentialStore secret is `ORCA_SERVER_SECRET` alone, so that line has been removed.
> `ORCA_DB_DSN` has been corrected to `ORCA_DB_URL`, the variable `config-loader.ts` actually
> reads (`backend/src/main/db/config-loader.ts:23`). The `ORCA_AI_CREDENTIAL_VERIFY_KEY`/
> `ORCA_WORKFLOW_*`/`ORCA_TASK_*` lines remain 🚧 proposed — see banner at the top of this v5.0
> section.

```yaml
# docker-compose.v5.yml (illustrative — compare against the REAL, currently-used
# deploy/dev/docker-compose.orca.artifact.yml in §10a above for what actually runs today)
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
      ORCA_SERVER_SECRET: "${ORCA_SERVER_SECRET}"    # for WebCredentialStore (real)
      # Database
      ORCA_DB_URL: "postgresql://orca:${DB_PASS}@postgres:5432/orca"   # ⚠️ corrected from ORCA_DB_DSN
      # AI Provider verify key (NOT the dev-server encryption key) — 🚧 proposed, not read by code
      ORCA_AI_CREDENTIAL_VERIFY_KEY: "${AI_VERIFY_KEY}"
      # Workflow — 🚧 proposed, not read by code
      ORCA_WORKFLOW_MAX_PARALLEL_STEPS: "10"
      ORCA_WORKFLOW_TEMPLATE_APPROVAL: "true"
      # Task Graph — 🚧 proposed, not read by code
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

---

## 17. Proposed Enterprise Rollout Plan — 🚧 NOT this document, NOT implemented

A separate 3-phase enterprise rollout plan exists as a **proposal only**:
`docs/adrs/v2/ADR-020-enterprise-rollout-phases-backward-compat.md`. It describes:
- Migration-version-gated feature flags (`FeatureFlags.isEnabled()`, `src/main/feature-flags.ts`)
- A dual-mode `AgentDispatcher` that tries Agent (v6) first and falls back to legacy Relay (v5)
- An `ORCA_MAX_MIGRATION` env var to cap feature availability without a DB rollback
- `requireMigration(version)` middleware returning HTTP 503 `FEATURE_NOT_AVAILABLE` for routes
  gated behind not-yet-applied migrations
- A week-by-week phased deployment timeline (Phase 1: migrations 0001–0005 → Phase 2: 0006–0010 →
  Phase 3: Agent v6.0 binary rollout with an Admin Panel "Agent v6.0 / Legacy Relay" mode badge)

**None of this exists in code.** ADR-020 self-declares `❌ Chưa implement` for every listed
artifact (`src/main/feature-flags.ts`, `src/main/dev-server/agent-dispatcher.ts`,
`src/main/api/feature-gate-middleware.ts`, the Admin Panel mode badge, `deploy/agent/install.sh`).
There is no `ORCA_MAX_MIGRATION` env var anywhere in `backend/src/`, no `FeatureFlags` class, no
dual-mode `AgentDispatcher`, and no HTTP 503 migration-gate middleware. The "Dev Server Agent"
(`agent/` package) that this plan's Phase 3 refers to already exists and is what §7/§10a of this
document describe (direct-websocket / relay-websocket over the wire protocol in
`agent/src/relay/*`) — but it is the *only* connection mode in production; there is no legacy
"Relay v5" it falls back to, and no dispatcher that chooses between the two. If this rollout plan
is adopted, update this document then — until it is, treat §12 above (migration-based DB schema
growth) as the real and only "rollout" mechanism Orca has today: migrations just run in order,
unconditionally, on server start.
