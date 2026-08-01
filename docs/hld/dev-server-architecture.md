# Dev Server — Vai trò, Chức năng & Kết nối với Orca Backend

**Nguồn:** Trích xuất từ HLD v1 (C1, C2, C3, C4)  
**Cập nhật:** 2026-07-30 v2.0 (chi tiết hóa AI Agent CLIs & External API Connectors)

---

## 1. Dev Server là gì?

Trong kiến trúc Orca, **Dev Server** là một **remote machine** (cloud server, EC2, VPS) nơi các công việc thực sự được thực thi: chạy AI agent, thao tác Git, đọc/ghi file, chạy shell commands. Nó đối lập với **Gateway** (Orca Web Server) — tầng điều phối và kiểm soát.

> **Mô hình phân tầng:** Gateway = Control Plane | Dev Server = Data Plane

---

## 2. Vai trò của Dev Server

| Vai trò | Mô tả |
|---------|-------|
| **Execution Environment** | Nơi AI agents thực sự chạy (Claude, Codex, Gemini...) qua PTY |
| **Code Host** | Chứa git repositories và worktrees thực tế |
| **File System Provider** | Cung cấp file tree, file content, file search cho UI |
| **Git Operations Host** | Thực thi `git status/diff/add/commit/push/pull/pr create` |
| **AI Credential Store** | Lưu trữ credential AI provider dạng mã hóa (`~/.orca/ai-providers/<id>.enc`) |
| **Workflow Step Executor** | Thực thi từng step trong workflow (agent, shell, action) |
| **Health Reporter** | Báo cáo CPU%, RAM, disk, network latency về Gateway |
| **AI Agent CLI Host** | Spawn và quản lý AI agent CLIs (Claude Code, Codex, Gemini CLI...) trực tiếp trên máy |
| **External API Caller** | Gọi GitHub/GitLab API từ Dev Server qua `gh`/`glab` CLI với per-user auth isolation |

---

## 3. Thành phần chạy trên Dev Server

Có **hai thế hệ** component chạy trên Dev Server:

### 3a. Orca Relay (v1 — legacy, deployed via SFTP)

Binary Node.js được upload lên remote host qua SFTP, phục vụ:

| Component | Chức năng |
|-----------|-----------|
| `PTY Handler` | Tạo/quản lý PTY sessions, stream I/O qua WebSocket |
| `Filesystem Handler` | File read/write/list, watching via parcel-watcher, ripgrep search |
| `Git Handler` | git diff, status, commit, push, worktree operations trên remote |
| `Port Scanner` | Scan localhost cho open ports mỗi 2 giây |
| `Agent Hook Server` | HTTP server intercept agent hooks, route tool calls qua relay |
| `Plugin Overlay` | Inject environment overlays, agent-specific env vars |

### 3b. Dev Server Agent (v6 — mới, chủ động connect về Gateway)

`orca-agent` binary (Node.js), các components:

| Component | Chức năng |
|-----------|-----------|
| `RPC Server` | JSON-RPC 2.0 over WebSocket, nhận calls từ Gateway |
| `Context Verifier` | Verify HMAC-SHA256 signed context, chống path traversal |
| `PTY Manager` | Create/resize/write/kill PTY sessions per userId |
| `Profile-Aware Agent Spawner` | Spawn AI agents với env từ resolved profile, whitelist models |
| `Worktree Engine` | git worktree add/remove/list, fan-out N worktrees |
| `Git Engine` | Full git ops + PR creation via `gh` CLI |
| `File System Engine` | readDir/readFile/writeFile với `SecureFs` enforcement |
| `AI Provider Credential Store` | AES-256-GCM encrypted credential files |
| `Workflow Step Executor` | Thực thi workflow steps (agent/shell/action) |
| `Health Reporter` | Emit health events mỗi 60s |
| `Reconnect Manager` | Exponential backoff (5s→60s max) khi mất kết nối Gateway |
| `Event Bus` | Internal pub/sub, fan-out events → Gateway stream |
| `Local SQLite` | State persistence local (worktrees, sessions, audit log) |

### Agent Startup Sequence

```
orca-agent start
  │
  ├─ 1. Load config (/etc/orca-agent/config.yaml)
  ├─ 2. Init local SQLite (run migrations)
  ├─ 3. Start Health Reporter
  ├─ 4. Start RPC Server (internal only)
  ├─ 5. Connect to Gateway (outbound WS)
  ├─ 6. Perform handshake (capabilities advertisement)
  ├─ 7. Start emitting health events every 60s
  └─ 8. Ready to receive RPC calls
```

---

## 4. Cách kết nối: Orca Backend ↔ Dev Server

Có **3 connection modes**, tùy cách agent/relay kết nối:

### Mode 1: `relay-ssh` (SSH exec channel)

```
Orca Gateway
    │
    ├── SSH connection (ssh2 library)
    │     └── SSH exec channel
    │              │
    │         SshChannelMultiplexer
    │              │
    │         JSON-RPC 2.0 frames (13-byte header)
    │              ↓
    │         Dev Server (Relay binary)
    │              ├── PTY Handler
    │              ├── Git Handler
    │              └── FS Handler
```

- **Auth:** SSH key authentication
- **Transport:** SSH exec channel → binary wire protocol
- **Use case:** Classical SSH remote host

### Mode 2: `relay-websocket` (Orca → Agent, outbound)

```
Orca Gateway
    │
    ├── HTTP Upgrade: ws://agent:PORT/orca-relay
    │     Header: Authorization: Bearer <agentToken>
    │              │
    │         WsTransport
    │              │
    │         SshChannelMultiplexer ⇔ JSON-RPC 2.0
    │              ↓
    │         Dev Server Agent (RpcServer)
```

- **Auth:** Bearer token (SHA-256 hashed, stored in DevServer config)
- **Transport:** WebSocket binary frames (13-byte header + UTF-8 JSON-RPC)
- **Use case:** Agent có public WS endpoint

### Mode 3: `direct-websocket` (Agent → Orca, inbound — v6 default)

```
Dev Server Agent (ReconnectManager)
    │
    ├── Outbound connect: wss://orca-gateway:6768/agent
    │              │
    │         Handshake:
    │           Agent → { type: 'agent.handshake', agentToken, name, version }
    │           Orca  → { type: 'handshake-ok', sessionId }
    │              │
    │         AgentWebSocketServer ⇔ WsTransport ⇔ JSON-RPC
    │              ↓
    │         Gateway routes calls → Agent RpcServer
```

- **Auth:** agentToken (Agent tự connect về Gateway)
- **Transport:** WebSocket binary frames
- **Use case:** Dev Server Agent v6 — agent **chủ động** connect ra ngoài, không cần mở inbound port

---

## 5. Wire Protocol Format

Tất cả communication đều dùng cùng wire protocol:

```
┌──────────────────────────────────────────────────────┐
│ TYPE[1B] | SEQ[4B BE] | ACK[4B BE] | LEN[4B BE] | PAYLOAD[LEN] │
└──────────────────────────────────────────────────────┘
         = 13 bytes header total
PAYLOAD  = UTF-8 JSON-RPC 2.0
TYPE     = 0x01 Regular | 0x09 KeepAlive
```

---

## 6. Security Model

| Điểm bảo mật | Cơ chế |
|--------------|--------|
| **Auth** | Bearer token (raw token → SHA-256 → stored hash) |
| **Context integrity** | `RpcExecutionContext` sign bằng HMAC-SHA256, TTL 30s |
| **Credential relay** | Browser encrypt (SubtleCrypto AES-GCM) → SSH relay → Dev Server decrypt — **Gateway không thấy plaintext** |
| **File path safety** | `SecureFs.validatePath()` enforce `projectRoot` + `allowedRoots` |
| **User isolation** | `GH_CONFIG_DIR` per userId, PTY ownership check |
| **Shell safety** | `execFile()` (không dùng shell), `disallowedCommands` whitelist |
| **Audit** | Mọi RPC call được log với `userId + outcome` |

---

## 7. Agent Isolation Model (Dev Server Agent v6)

Dev Server Agent không `fork()` per user như Gateway. Isolation được enforce qua:

| Mechanism | How |
|-----------|-----|
| PTY ownership | `ptyId` bound to `userId`, router checks ownership |
| File path | `SecureFs.validatePath()` checks `projectRoot` + `allowedRoots` |
| Git author | Injected từ `ctx.userEmail`, không thể bị override |
| AI env | `GH_CONFIG_DIR` per userId (isolated GitHub CLI config) |
| Shell commands | `execFile()` (no shell), `disallowedCommands` whitelist |
| Audit | All RPC calls logged với `userId + outcome` |

---

## 8. Sơ đồ tổng quan Control Plane ↔ Data Plane

```
═══════════════ GATEWAY (Control Plane) ═══════════════

 ProfileResolver  → resolved profile (Company ← Dept ← User)
 ProjectService   → project.devServerId
 AIProviderSvc    → provider metadata (no credentials)
 WorkflowOrch     → DAG dispatch to dev servers
 TaskService      → task → agent spawn via relay
         │
         │  AgentConnectionManager
         │  Signs: RpcExecutionContext (HMAC-SHA256, TTL 30s)
         │
         └──── [wss:// outbound / SSH channel] ────┐
                                                    ↓
═══════════════ DEV SERVER AGENT (Data Plane) ═══════════════

 RpcServer ← ContextVerifier (verify HMAC)
     │
     ├── PtyManager + ProfileAwareAgentSpawner  ← spawn AI agents
     ├── WorktreeEngine                         ← git worktree ops
     ├── GitEngine                              ← git + gh CLI + PR
     ├── FsEngine (SecureFs)                   ← file read/write/search
     ├── AiCredStore (AES-256-GCM)             ← credential files
     ├── StepExecutor                           ← workflow steps
     ├── HealthReporter                         ← metrics → Gateway
     └── EventBus → stream events → Gateway
```

---

## 9. Key Flows liên quan Dev Server

### Flow: AI Credential Write (Relay-Only, không qua Gateway plaintext)

```
Admin nhập API key
    → Browser: SubtleCrypto.encrypt(sessionKey, apiKey)
    → POST /rpc (encrypted blob)
    → Orca Server: relay.call('ai.provider.writeCredential')
    → SSH relay → Dev Server
    → Decrypt với ORCA_AI_CREDENTIAL_KEY
    → Write ~/.orca/ai-providers/<accountId>.enc
    [Orca Server KHÔNG thấy plaintext key]
```

### Flow: Project Workspace Switch

```
User chọn Project
    → WorkspaceContext.switchProject(projectId)
    → ProjectService.get() → { devServerId, repoPath }
    → RelayConnectionPool.getOrConnect(devServerId) [reuse nếu có]
    → Promise.all [git.status, worktree.list, fs.readDir, workflow.getActive]
    → WorkspaceContext ready
    → ExplorerPanel, GitPanel, AgentPanel render
```

### Flow: Task → Agent → Git → PR (End-to-end)

```
Lead tạo Task → AI decompose → subtasks
Developer mở Task → [Run Agent]
    → TaskAgentExecutor: build preamble + inject env
    → relay: spawn agent on dev server
    → Agent completes → WorkspaceEvent 'agent.complete'
        → GitPanel auto-refresh (git status)
        → ExplorerPanel refresh decorations
    → Developer: [Stage All] → [AI: Generate commit message]
        → relay: git.diff --staged → LLM → message
    → [Commit & Push]
        → relay: git.push (stream progress)
    → [Create PR]
        → relay: github.pr.create (gh CLI on dev server)
    → PR URL → Task.prUrl = PR.url
    → Task status → 'review'
```

### Flow: Remote File Explorer

```
User expands 📁 src/ trong Explorer
    │
    ▼ relay.call('fs.readDir', { path: '/srv/projects/vnp-blc/src', depth: 1 })
    │
    ├── relay → dev server: fs.readdir('/srv/projects/vnp-blc/src')
    ├── returns: [{ name: 'auth', isDir: true }, { name: 'index.ts', size: 2048 }]
    │
    ▼ overlay git status decorations (pre-fetched in WorkspaceContext.gitStatus)
    │
    ▼ Render file tree với inline git status badges [M] [A] [?]
```

---

## 10. Communication Matrix (Gateway ↔ Dev Server & Externals)

| From | To | Protocol | Format |
|------|----|----------|--------|
| AI Provider Svc | Dev Server | SSH relay | Encrypted blob (AES-256-GCM) |
| Workflow Engine | Dev Server | relay RPC | JSON-RPC 2.0 |
| Task Service | Dev Server | relay RPC | JSON-RPC 2.0 |
| Project Workspace | Dev Server | relay RPC | JSON-RPC 2.0 |
| Dev Server Agent | Gateway | WebSocket (outbound) | Binary frames + JSON-RPC |
| Fleet Health Monitor | Dev Server | SSH poll | SSH exec |
| **Dev Server Agent** | **AI Agent CLIs** | **PTY (node-pty)** | **stdin/stdout + OSC sequences** |
| **Dev Server Agent** | **GitHub API** | **HTTPS (gh CLI)** | **REST/GraphQL JSON** |
| **Dev Server Agent** | **GitLab API** | **HTTPS (glab CLI)** | **REST JSON** |

---

## 11. AI Agent CLIs trên Dev Server

Dev Server Agent spawn AI agent CLIs **trực tiếp trên máy** thông qua `ProfileAwareAgentSpawner`. Đây là **thành phần cốt lõi** của Data Plane — nơi LLM inference thực sự xảy ra.

### 11.1 ProfileAwareAgentSpawner — Kiến trúc

```
Gateway gửi RPC: agent.spawn({ model, trustPreset, env, cwd, initFile, taskId, userId })
    │
    ▼ Dev Server: ProfileAwareAgentSpawner
    │
    ├── [1] VALIDATE model whitelist
    │       model ∈ resolvedProfile.agent.approvedModels
    │       → Nếu không hợp lệ: return error { code: PermissionDenied, message: 'Model not approved' }
    │
    ├── [2] LOAD AI credentials từ AiCredStore
    │       AiCredStore.get(accountId)
    │       → fs.readFile(~/.orca/ai-providers/<accountId>.enc)
    │       → AES-256-GCM decrypt (ORCA_AI_CREDENTIAL_KEY)
    │       → plaintext API key in memory (NOT stored)
    │
    ├── [3] BUILD agent environment (per userId)
    │       ANTHROPIC_API_KEY=<from AiCredStore>     (claude)
    │       OPENAI_API_KEY=<from AiCredStore>         (codex)
    │       GOOGLE_API_KEY=<from AiCredStore>         (gemini)
    │       ANTHROPIC_MODEL=claude-opus-4-5
    │       GH_CONFIG_DIR=~/.config/gh/<userId>/      ← per-user isolation
    │       GLAB_CONFIG_DIR=~/.config/glab-cli/<userId>/
    │       PATH=/custom/bin:$PATH                    ← từ profile.shell.pathAdditions
    │       ORCA_PROJECT_ID=<projectId>
    │       ORCA_TASK_ID=<taskId>
    │       ORCA_USER_ID=<userId>
    │       ORCA_AGENT_HOOK_URL=http://localhost:<hookPort>
    │
    ├── [4] SPAWN agent via node-pty
    │       node-pty.spawn(agentBinary, args, { cwd, env, cols: 220, rows: 50 })
    │
    ├── [5] PARSE OSC sequences → state machine
    │       idle → running → waiting_for_input → completed / error
    │       OSC 133 (prompt boundaries), OSC 9;9 (working dir), OSC 1337 (custom)
    │
    ├── [6] STREAM PTY output → Gateway
    │       pty.onData → encodeDataFrame → ws.send
    │       Frame: { type: 'pty.output', ptyId, data: base64 }
    │
    ├── [7] EMIT status events → EventBus → Gateway
    │       agent.statusChanged: { taskId, status: 'running' | 'waiting' | 'completed' | 'error' }
    │
    └── [8] CLEANUP on completion
            pty.kill() → EventBus.emit('agent.exit', { taskId, exitCode })
            AiCredStore key zeroized from memory
```

### 11.2 Supported AI Agent CLIs

| Agent | Binary | Launch args | Trust preset | API key env |
|-------|--------|-------------|-------------|-------------|
| **Claude Code** | `claude` | `--print --trust standard` | `standard` / `full` | `ANTHROPIC_API_KEY` |
| **OpenAI Codex** | `codex` | `--model gpt-4o` | N/A | `OPENAI_API_KEY` |
| **Gemini CLI** | `gemini` | `--model gemini-2.0-flash` | N/A | `GOOGLE_API_KEY` |
| **OpenCode** | `opencode` | N/A | N/A | per-provider |
| **Ollama (local)** | `ollama` | `run <model>` | N/A | N/A (local HTTP) |
| **Custom binary** | `<path>` | profile-defined | N/A | profile-defined |

> **Ollama/vLLM:** local inference, không cần external API key.  
> Agent CLI nhận: `OLLAMA_HOST=http://localhost:11434` hoặc `OPENAI_BASE_URL=http://localhost:8000/v1`

### 11.3 Agent Lifecycle State Machine

```
         agent.spawn()
              │
              ▼
         ┌─────────┐
         │  IDLE   │ ← ptyId registered, PTY allocated
         └────┬────┘
              │ pty.onData (first output)
              ▼
         ┌─────────┐
         │ RUNNING │ ← agent executing, OSC 133 prompt opened
         └────┬────┘
              │
         ┌────┴─────────────────────────────┐
         │                                  │
         ▼                                  ▼
  ┌─────────────────┐               ┌──────────────┐
  │ WAITING_FOR_    │               │    ERROR     │
  │ INPUT           │               │ (exit != 0)  │
  └────────┬────────┘               └──────┬───────┘
           │ pty.write (user/auto)          │
           │ OR timeout → auto-confirm      │
           ▼                               │
      ┌──────────┐                         │
      │ RUNNING  │                         │
      └─────┬────┘                         │
            │ OSC 133 prompt closed        │
            ▼                              │
      ┌──────────────┐                    │
      │  COMPLETED   │◄───────────────────┘
      └──────────────┘
```

### 11.4 Error Recovery & Resilience

| Scenario | Dev Server behavior |
|----------|---------------------|
| Agent binary not found | Return `{ code: -32001, message: 'claude not found in PATH' }` |
| API key not configured | Return `{ code: -32003, message: 'Credential not found: <accountId>' }` |
| Model not whitelisted | Return `{ code: -32003, message: 'Model not approved by profile' }` |
| PTY crash (SIGKILL) | Emit `agent.exit { exitCode: 137 }`, cleanup PTY, log to audit |
| OOM (exit 137) | Same as above + health event `{ memPressure: true }` |
| Agent hung (no output 5min) | Keepalive timeout → SIGTERM → SIGKILL after 10s |
| Network loss to Gateway | Buffer output locally up to 10MB, replay on reconnect |

### 11.5 Agent Isolation trên Dev Server

| Mechanism | Cách thực hiện |
|-----------|----------------|
| AI credential | Load từ `~/.orca/ai-providers/<id>.enc`, không truyền qua network |
| GH config | `GH_CONFIG_DIR=~/.config/gh/<userId>/` riêng mỗi user |
| PTY ownership | `ptyId` bind với `userId`, chỉ owner mới write được |
| Model whitelist | `approvedModels` từ Company profile, không cho phép override |
| Usage tracking | Ghi token usage vào `agent_task_runs` local SQLite |
| Env zeroization | API key xóa khỏi memory sau khi spawn xong |


---

## 12. External APIs từ Dev Server — Connectors

Dev Server gọi External APIs thông qua **CLI tools** (`gh`, `glab`) hoặc trực tiếp qua HTTPS, với per-user auth isolation. Đây là **External API Connector layer** — độc lập khỏi Gateway, không expose credentials qua network.

### 12.1 GitHub Connector (gh CLI-based)

```
Dev Server Agent: git.pr.create({ title, body, base, draft, userId })
    │
    ▼ GitEngine.createPR()
    │
    ├── Set env: GH_CONFIG_DIR=~/.config/gh/<userId>/
    ├── Validate args (SHELL_METACHARACTERS check)
    ├── execFile('gh', ['pr', 'create', '--title', title, '--body', body, '--base', base])
    │     → HTTPS api.github.com/graphql (gh CLI handles OAuth token)
    │     → timeout: 30s
    ├── Parse stdout: PR URL (e.g. https://github.com/org/repo/pull/123)
    └── Return: { url, stdout, stderr }
         → stream to Gateway → update Task.prUrl
```

**GitHub operations (full list):**

| Operation | RPC Method | gh CLI command | Notes |
|-----------|-----------|----------------|-------|
| Create PR | `git.pr.create` | `gh pr create` | Returns PR URL |
| View PR | `git.pr.view` | `gh pr view --json` | Returns PR metadata |
| Merge PR | `git.pr.merge` | `gh pr merge --squash` | Requires PR author or admin |
| List issues | `git.issue.list` | `gh issue list --json` | Paginated, max 30 |
| Create issue | `git.issue.create` | `gh issue create` | Returns issue URL |
| Clone repo | `git.repo.clone` | `gh repo clone` | Uses git over HTTPS |
| Preflight | `preflight.check` | `gh auth status` | Returns auth status |

**Auth isolation:**
```
~/.config/gh/<userId>/               ← per-user gh config dir
    hosts.yml                        ← GitHub token (set by gist deploy script)
    config.yml                       ← gh preferences

GH_CONFIG_DIR env var set by ProfileAwareAgentSpawner + GitEngine per call
```

### 12.2 GitLab Connector (glab CLI-based)

```
Dev Server Agent: git.mr.create({ title, body, base, userId })
    │
    ▼ GitEngine.createMR()
    │
    ├── Set env: GLAB_CONFIG_DIR=~/.config/glab-cli/<userId>/
    ├── execFile('glab', ['mr', 'create', '--title', title, '--description', body, '--target-branch', base])
    │     → HTTPS gitlab.com API (glab CLI handles PAT token)
    │     → timeout: 30s
    └── Return: { url, stdout, stderr }
```

**GitLab operations (full list):**

| Operation | RPC Method | glab CLI command | Notes |
|-----------|-----------|-----------------|-------|
| Create MR | `git.mr.create` | `glab mr create` | Returns MR URL |
| View MR | `git.mr.view` | `glab mr view` | Returns MR metadata |
| List MRs | `git.mr.list` | `glab mr list` | Filtered by state |
| Create issue | `git.issue.create` | `glab issue create` | Returns issue URL |
| Pipeline status | `git.pipeline.status` | `glab pipeline status` | CI/CD status |
| Preflight | `preflight.check` | `glab auth status` | Returns auth status |

**Auth isolation:**
```
~/.config/glab-cli/<userId>/         ← per-user glab config dir
    config.yml                       ← GitLab PAT token
GLAB_CONFIG_DIR env var set per call
```

### 12.3 AI Provider APIs (qua credentials lưu local)

AI agent CLIs gọi AI Provider APIs **trực tiếp từ Dev Server** — không qua Gateway:

| Provider | Agent | Endpoint | Auth | Credential source |
|----------|-------|---------|------|------------------|
| Anthropic | claude | `api.anthropic.com` | `ANTHROPIC_API_KEY` | `AiCredStore` |
| OpenAI | codex | `api.openai.com` | `OPENAI_API_KEY` | `AiCredStore` |
| Google | gemini | `generativelanguage.googleapis.com` | `GOOGLE_API_KEY` | `AiCredStore` |
| **Ollama (local)** | any | `http://localhost:11434` | None | Env: `OLLAMA_HOST` |
| **vLLM (local)** | any | `http://localhost:8000/v1` | None | Env: `OPENAI_BASE_URL` |
| **LM Studio** | any | `http://localhost:1234/v1` | None | Env: `OPENAI_BASE_URL` |

```
Local inference providers (Ollama/vLLM/LM Studio):
  → Không cần external API key
  → Agent CLI nhận env:
      OLLAMA_HOST=http://localhost:11434
      OPENAI_BASE_URL=http://localhost:8000/v1  (vLLM compatible)
  → Dev Server phải có GPU hoặc CPU inference capable
  → Health check: curl http://localhost:11434/api/version
```

### 12.4 Preflight Check Flow

```
Gateway → relay.call('preflight.check', { userId })
    │
    ▼ Dev Server thực thi song song:
    │
    ├── gh auth status   (GH_CONFIG_DIR=~/.config/gh/<userId>/)     → ok | not_logged_in | error
    ├── glab auth status (GLAB_CONFIG_DIR=~/.config/glab-cli/<userId>/) → ok | not_logged_in | error
    ├── node --version                                               → version string
    ├── ollama api/version (nếu có port 11434)                      → ok | not_running
    └── df -h / (disk space check)                                  → available GB
    │
    ▼ Aggregate results:
    {
      github: { ok: true, user: 'user@org' },
      gitlab: { ok: false, error: 'not logged in' },
      node: { ok: true, version: 'v22.14.0' },
      ollama: { ok: true, version: '0.3.14' },
      disk: { ok: true, availableGB: 42 }
    }
    │
    ▼ Stream về Gateway
    ▼ Gateway: mergePreflightStatuses(localResults, relayResults)
         relay results OVERRIDE local (Dev Server is authoritative for CLI tools)
```

### 12.5 External API Connector Design Principles

| Principle | Cách thực hiện |
|-----------|----------------|
| **CLI-based, not SDK** | Dùng `gh`/`glab` CLI thay vì GitHub SDK — CLI handles token rotation, rate limiting |
| **Per-user isolation** | `GH_CONFIG_DIR`/`GLAB_CONFIG_DIR` per userId — không bao giờ share global config |
| **No shell injection** | `execFile()` với array args — không dùng shell string concatenation |
| **Metachar validation** | Tất cả user input qua `SHELL_METACHARACTERS` check trước khi pass vào CLI |
| **Timeout mandatory** | Tất cả external calls có timeout (30s default, max 60s) |
| **Idempotency** | PR create check existing PR trước (gh pr list --head branch) |
| **Error forwarding** | stderr từ CLI được forward về Gateway trong `{ error.details }` |
| **Auth never through Gateway** | GitHub/GitLab tokens chỉ nằm trên Dev Server filesystem |


---

## 13. Feature → Dev Server Component Mapping (updated)

| Feature | Gateway Component | Dev Server Component |
|---------|------------------|---------------------|
| F01 Parallel Worktrees | ProjectService (quota, RBAC) | WorktreeEngine |
| F02 Terminal Splits | WsSessionRouter (routing) | PtyManager |
| F04 AI Agent Support | ProfileResolver + ProviderResolver | ProfileAwareAgentSpawner + **AI Agent CLIs** |
| F06 GitHub/Linear | WebCredentialStore (tokens) | **GitEngine (gh CLI → GitHub API)** |
| F12 File Explorer | — (proxy) | FsEngine |
| F27 Fleet Health | FleetHealthMonitor (aggregate) | HealthReporter |
| F28 Dev Server Onboard | DevServerProvisioner | First-connect bootstrap |
| F30 Remote Integrations | WebCredentialStore | **GitEngine (gh/glab auth isolation)** |
| F34 Project Binding | ProjectService | ContextVerifier (enforce) |
| F35 AI Provider Mgmt | AIProviderService (meta only) | AiCredStore + **AI Provider APIs** |
| F36 Workflow Orchestration | WorkflowOrchestrator (DAG, dispatch) | StepExecutor + **AI Agent CLIs** |
| F37 Task Graph | TaskService (grant, plan) | ProfileAwareAgentSpawner + **AI Agent CLIs** |
| F38 Project Workspace | WorkspaceContext | FsEngine + GitEngine |
| F39 Remote Git UI | — (proxy) | **GitEngine (gh CLI → GitHub/GitLab API)** |

---

## 14. Sơ đồ tổng quan (updated)

```
═══════════════ GATEWAY (Control Plane) ═══════════════

 ProfileResolver  → resolved profile (Company ← Dept ← User)
 ProjectService   → project.devServerId
 AIProviderSvc    → provider metadata (no credentials)
 WorkflowOrch     → DAG dispatch to dev servers
 TaskService      → task → agent spawn via relay
         │
         │  AgentConnectionManager
         │  Signs: RpcExecutionContext (HMAC-SHA256, TTL 30s)
         │
         └──── [wss:// outbound / SSH channel] ────┐
                                                    ↓
═══════════════ DEV SERVER AGENT (Data Plane) ═══════════════

 RpcServer ← ContextVerifier (verify HMAC)
     │
     ├── PtyManager + ProfileAwareAgentSpawner
     │           │
     │           ├──── node-pty ────→ Claude Code (claude)
     │           │                 → OpenAI Codex (codex)
     │           │                 → Gemini CLI (gemini)
     │           │                 → Custom agent binary
     │           │                     │ stdin/stdout + OSC
     │           │                     ↓
     │           └── AiCredStore ───→ AI Provider APIs
     │                                 api.anthropic.com
     │                                 api.openai.com
     │                                 generativelanguage.googleapis.com
     │                                 localhost:11434 (Ollama/vLLM)
     │
     ├── WorktreeEngine    ← git worktree ops
     │
     ├── GitEngine ────────→ gh CLI ──→ api.github.com
     │                  └──→ glab CLI → gitlab.com API
     │
     ├── FsEngine (SecureFs)
     ├── StepExecutor ─────→ AI Agent CLIs + shell.exec
     ├── HealthReporter ───→ events → Gateway
     └── EventBus → stream all events → Gateway
```
