# Component Mapping — Luồng Dữ liệu × 3 Thành phần Kiến trúc

Tài liệu này lập bảng **mapping chi tiết** giữa từng luồng nghiệp vụ và 3 thành phần hệ thống:

| Thành phần | Ký hiệu | Mô tả |
|------------|---------|-------|
| **Web (Browser)** | WEB | React SPA / Admin SPA chạy trong browser |
| **Backend Server** | SRV | Orca Web Server (Express + WS :6768 + HTTP :6769) |
| **Dev Server (Agent)** | DEV | Node.js agent binary chạy trên dev server — **đóng vai trò WebSocket client**, chủ động kết nối vào Orca Server |

> **Topology kết nối cốt lõi:**
> ```
> [Browser] <──── HTTP/WebSocket ────> [Orca Server :6768/:6769]
>                                              ^
>                                              | WebSocket (Dev Server là client)
>                                              | JSON-RPC 2.0 bidirectional
>                                              v
>                                       [Dev Server Agent]
>                                       ws://orca:6768/agent
> ```

**Cập nhật:** 2026-08-01  
**Nguồn:** [`docs/flows/logic/`](./logic/)

---

## Ký hiệu

| Ký hiệu | Nghĩa |
|---------|-------|
| Y | Thành phần tham gia vào luồng |
| - | Thành phần không tham gia |
| -> | Gửi request/call đến |
| <- | Nhận response/event từ |

---

## Bảng Tổng Hợp

| # | Luồng | Nghiệp vụ | WEB | SRV | DEV |
|---|-------|-----------|-----|-----|-----|
| 1 | BL-AUTH-01 | Local Login | Y | Y | - |
| 2 | BL-AUTH-02 | Session Management & Isolation | Y | Y | - |
| 3 | BL-AUTH-03 | Per-User Process Sandbox | - | Y | - |
| 4 | BL-AUTH-04 | Admin User CRUD & Session Kill | Y | Y | - |
| 5 | BL-AUTH-05 | Audit Log | Y | Y | - |
| 6 | BL-AG-01 | Khởi động AI Agent | Y | Y | Y |
| 7 | BL-AG-02 | Dừng Agent | Y | Y | Y |
| 8 | BL-AG-03 | Resume Agent Session | Y | Y | Y |
| 9 | BL-AG-04 | Switch Account / Provider | Y | Y | Y |
| 10 | BL-AG-05 | Monitor Trạng thái Agent Real-time | Y | Y | Y |
| 11 | BL-TM-01 | Tạo PTY Session | Y | Y | **Y (runtime)** |
| 12 | BL-TM-02 | Split Terminal | Y | Y | **Y (runtime)** |
| 13 | BL-TM-03 | Lưu và Khôi phục Scrollback | Y | Y | **Y (runtime)** |
| 14 | BL-TM-04 | Shell Integration (OSC 133) | Y | Y | **Y (runtime)** |
| 15 | BL-SSH-01 | Kết nối SSH Host | Y | Y | Y |
| 16 | BL-SSH-02 | Deploy Orca Relay Binary | Y | Y | Y |
| 17 | BL-SSH-03 | SSH Auto-Reconnect | Y | Y | Y |
| 18 | BL-SSH-04 | Auto Port Forwarding | Y | Y | Y |
| 19 | BL-AWS-01 | relay-websocket Mode | - | Y | Y |
| 20 | BL-AWS-02 | direct-websocket Mode | - | Y | Y |
| 21 | BL-AWS-03 | Agent Token Management | Y | Y | - |
| 22 | BL-PRF-01 | Tạo và Cập nhật Profile | Y | Y | - |
| 23 | BL-PRF-02 | Profile Inheritance Resolution | - | Y | - |
| 24 | BL-PRF-03 | Project-Dev Server Assignment | Y | Y | - |
| 25 | BL-PRF-04 | Profile-Aware Agent Execution Routing | Y | Y | Y |
| 26 | BL-AIP-01 | Đăng ký AI Provider Account | Y | Y | Y |
| 27 | BL-AIP-02 | Provider Account Resolution | - | Y | - |
| 28 | BL-AIP-03 | Provider Health Check & Quota | Y | Y | Y |
| 29 | BL-WF-01 | Workflow Template Management | Y | Y | - |
| 30 | BL-WF-02 | Multi-Server Workflow Execution | Y | Y | Y |
| 31 | BL-WF-03 | Workflow Sharing & Library | Y | Y | - |
| 32 | BL-TG-01 | Task Graph CRUD | Y | Y | - |
| 33 | BL-TG-02 | AI-Assisted Task Planning | Y | Y | Y |
| 34 | BL-TG-03 | Task Access Control & Sharing | Y | Y | - |
| 35 | BL-TG-04 | Task Prompt -> Agent Execution | Y | Y | **Y (runtime: agent.exec)** |
| 36 | BL-PW-01 | Project Workspace Context | Y | Y | **Y (runtime: git+fs+PTY)** |
| 37 | BL-PW-02 | Remote File Explorer | Y | Y | **Y (runtime: fs.readDir/readFile/grep)** |
| 38 | BL-PW-03 | Remote Git UI Operations | Y | Y | **Y (runtime: git CLI)** |
| 39 | BL-PW-04 | Workspace Integration (Agent+Git+Tasks) | Y | Y | **Y (runtime: agent+git+pty)** |
| 40 | BL-FLEET-01 | Fleet Inventory Config | Y | Y | - |
| 41 | BL-FLEET-02 | Bulk Server Provisioning | Y | Y | Y |
| 42 | BL-FLEET-03 | Fleet Health Monitoring | Y | Y | Y |
| 43 | BL-FLEET-04 | Dev Server Onboarding Wizard | Y | Y | Y |
| 44 | BL-PI-01 | Import GitHub/GitLab Issues | Y | Y | - |
| 45 | BL-PI-02 | Tạo Worktree từ Issue/Task | Y | Y | **Y (runtime: git+agent)** |
| 46 | BL-PI-03 | Cập nhật Trạng thái Issue | Y | Y | - |
| 47 | BL-PI-04 | Submit PR Review lên GitHub | Y | Y | **Y (runtime: gh CLI via relay)** |
| 48 | BL-WT-01 | Tạo Worktree | Y | Y | **Y (runtime: git+PTY)** |
| 49 | BL-WT-02 | Fan-out Prompt tới Nhiều Worktree | Y | Y | **Y (runtime: git+PTY+agent)** |
| 50 | BL-WT-03 | Xóa Worktree An Toàn | Y | Y | **Y (runtime: git+PTY)** |
| 51 | BL-WT-04 | So sánh Kết quả Giữa Worktrees | Y | Y | **Y (runtime: git diff)** |
| 52 | BL-WT-05 | Merge Worktree Thắng | Y | Y | **Y (runtime: git merge)** |
| 53 | BL-CLI-01 | Tạo Worktree qua CLI | - | Y | Y* |
| 54 | BL-CLI-02 | Quản lý Agent qua CLI | - | Y | Y* |
| 55 | BL-CLI-03 | Chạy Orca Headless Mode | - | Y | Y* |
| 56 | BL-AT-01 | Cấu hình Automation | Y | Y | - |
| 57 | BL-AT-02 | Chạy Automation theo Schedule | Y | Y | Y* |
| 58 | BL-AT-03 | Event-based Automation Trigger | - | Y | Y* |
| 59 | BL-AT-04 | Cleanup Worktrees Theo Policy | - | Y | Y* |
| 60 | BL-CR-01 | Xem Diff của Agent Changes | Y | Y | Y* |
| 61 | BL-CR-02 | Annotate Dòng Code trong Diff | Y | Y | Y* |
| 62 | BL-CR-03 | Gửi Feedback về Agent | Y | Y | Y* |
| 63 | BL-CR-04 | Tạo Commit Message bằng AI | Y | Y | Y* |
| 64 | BL-CR-05 | Tạo Pull Request với AI | Y | Y | Y* |

> **Y* = chỉ khi dùng Remote Dev Server** (worktree trên remote host), nếu local thì không qua DEV  
> **Y (runtime)** = phần thực thi bắt buộc phải chạy trên Dev Server (node-pty, git CLI, agent process, fs ops) qua `relay.call()`  
> **CLI/Headless**: CLI → HTTP/WS → Orca Server (không phải Unix Socket Daemon như HLD cũ mô tả)  
> **Automation**: AutomationService dùng Electron WebContents.send() — chạy trong Electron Main Process  
> **Code Review**: Tất cả git ops và agent inject cần dual-path (local PTY hoặc relay → Dev Server PTY)  
> **Terminal (BL-TM-*)**: node-pty và shell process phải chạy trên Dev Server, KHÔNG phải Orca Server  
> **Worktree (BL-WT-*)**: Tất cả git CLI (worktree add/remove/diff/merge) phải chạy trên Dev Server  
> **Task Agent (BL-TG-04)**: agent.exec relay call → Dev Server spawn agent process  
> **Project-Integration (BL-PI-02, BL-PI-04)**: git ops và gh CLI phải chạy trên Dev Server  
> **Remote-Integration (BL-INT-01)**: credential relay — Dev Server dùng credential từ Orca Server

---

## Chi tiết từng luồng — Thành phần và Giao thức

---

### 1. AUTH — Xác thực & Quản lý User

#### BL-AUTH-01 — Local Login

| Thành phần | Tham gia | Cấu phần sử dụng | Giao thức |
|------------|----------|------------------|-----------|
| WEB | Y | Login form, `POST /auth/local`, nhận `Set-Cookie` | HTTP POST |
| SRV | Y | `AuthRouter.localLogin()`, `bcrypt.compare()`, `crypto.randomBytes()`, `orca_users` DB, `orca_sessions` DB, `orca_audit_log` DB | Express REST |
| DEV | - | — | — |

**Chi tiết luồng:**
```
[WEB] POST /auth/local { email, password }
  -> [SRV] AuthRouter: SELECT user WHERE email=? -> bcrypt.compare(password, hash)
  -> [SRV] INSERT orca_sessions { token, userId, expiresAt: now+8h }
  -> [SRV] INSERT orca_audit_log { action: 'login.success' }
  -> [SRV] Set-Cookie: orca_session=<token>; HttpOnly; Secure
  -> [WEB] redirect /app -> WebSocket connect ws://orca:6768 (cookie auth)
  -> [SRV] WsSessionRouter: verify cookie -> fork child process per user
```

---

#### BL-AUTH-02 — Session Management & Isolation

| Thành phần | Tham gia | Cấu phần sử dụng | Giao thức |
|------------|----------|------------------|-----------|
| WEB | Y | WebSocket connect `ws://orca:6768`, cookie | WebSocket |
| SRV | Y | `WsSessionRouter`, `SessionManager`, `fork(userId)`, `orca_sessions` DB, Unix Socket proxy | WS + Unix Socket |
| DEV | - | — | — |

**Chi tiết luồng:**
```
[WEB] WebSocket ws://orca:6768 + Cookie: orca_session=<token>
  -> [SRV] WsSessionRouter: SELECT session WHERE token=? AND expiresAt > now
  -> [SRV] SessionManager: childProcess[userId]? -> fork new child process
  -> [SRV] Child Process: ~/.orca/users/<userId>/orca.sock (Unix Socket)
  -> [SRV] WsSessionRouter proxy: Browser WS <-> Unix Socket <-> Child Process

Per-User Data:
  ~/.orca/users/<userId>/
    orca.sock       <- Unix socket
    orca.db         <- Per-user SQLite
    credentials.enc <- AES-256-GCM tokens
    worktrees/      <- Git worktrees
```

---

#### BL-AUTH-04 — Admin User CRUD & Session Kill

| Thành phần | Tham gia | Cấu phần sử dụng | Giao thức |
|------------|----------|------------------|-----------|
| WEB | Y | Admin SPA, `POST/PATCH/DELETE /admin/api/users` | HTTP REST |
| SRV | Y | `AdminRouter`, `requireAdmin()` guard, `bcrypt.hash(12 rounds)`, `orca_users` DB, `SessionManager.SIGTERM()` | Express REST |
| DEV | - | — | — |

**Chi tiết luồng:**
```
[WEB] Admin SPA -> POST /admin/api/users { email, name, role, password }
  -> [SRV] AdminRouter: requireAdmin() guard (verify role='admin' from session)
  -> [SRV] bcrypt.hash(password, 12)
  -> [SRV] INSERT orca_users { id, email, name, role, passwordHash }
  -> [SRV] INSERT orca_audit_log { action: 'user.create' }

DEACTIVATE USER:
  [WEB] PATCH /admin/api/users/:id { is_active: false }
  -> [SRV] UPDATE orca_users SET is_active=0
  -> [SRV] DELETE orca_sessions WHERE userId=?
  -> [SRV] SessionManager: SIGTERM child process

KILL SESSION:
  [WEB] DELETE /admin/api/sessions/:id
  -> [SRV] DELETE orca_sessions WHERE id=?
  -> [SRV] WsSessionRouter: drop WebSocket connection
```

---

### 2. AGENT ORCHESTRATION — Điều phối AI Agent

#### BL-AG-01 — Khởi động AI Agent

| Thành phần | Tham gia | Cấu phần sử dụng | Giao thức |
|------------|----------|------------------|-----------|
| WEB | Y | Agent card UI, "Start Agent" button, `contextBridge.invoke` | Electron IPC / WS RPC |
| SRV | Y | `AgentManager.start()`, `ProfileResolver.resolve()`, `AIProviderResolver.resolve()`, `AgentConnectionManager.getConnection()`, `AgentHookParser`, `orca_sessions` DB | Internal + WS JSON-RPC |
| DEV | Y | `agent.spawn` handler, `node-pty.spawn(agentBinary)`, `ptySessionStore[userId+worktreeId]`, Verify `RpcExecutionContext` HMAC-SHA256 | JSON-RPC 2.0 over WS |

**Chi tiết luồng:**
```
[WEB] click "Start Agent" -> contextBridge.invoke('agent.start', { worktreeId, agentType })
  -> [SRV] AgentManager: ProfileResolver.resolve(userId) -> ResolvedProfile
  -> [SRV] AIProviderResolver.resolve() -> { provider, apiKeyEnvVar }
  -> [SRV] AgentConnectionManager.getConnection(devServerId)
         [WS persistent: Dev Server da mo san den Orca]
  -> [SRV->DEV] JSON-RPC call: agent.spawn {
       agentBinary, args, cwd, env, userId, cols, rows
     }
  -> [DEV] Verify RpcExecutionContext (HMAC-SHA256, 30s TTL)
  -> [DEV] node-pty.spawn(agentBinary, args, { cwd: worktreePath, env })
  -> [DEV] ptySessionStore[userId+worktreeId] = ptyHandle
  -> [DEV->SRV] JSON-RPC result: { ptyId, pid }
  -> [DEV->SRV] JSON-RPC event: agent.output { ptyId, data } [stream lien tuc]
  -> [SRV] AgentHookParser: parse OSC 133 -> detect state: 'idle'
  -> [SRV] INSERT orca_sessions { sessionId, worktreeId, agentType, devServerId }
  -> [SRV->WEB] IPC event: agent:started { sessionId, status: 'idle' }
  -> [WEB] agent card: status = "idle"

Huong WebSocket:
  DEV --ws connect--> SRV   (Dev Server la WS client, mo 1 lan luc khoi dong)
  SRV --JSON-RPC-->   DEV   (gui agent.spawn qua connection do)
  DEV --JSON-RPC-->   SRV   (stream agent.output events nguoc lai)
```

---

#### BL-AG-05 — Monitor Trạng thái Agent Real-time

| Thành phần | Tham gia | Cấu phần sử dụng | Giao thức |
|------------|----------|------------------|-----------|
| WEB | Y | Agent card, status indicator, spinner, progress | WS event push |
| SRV | Y | `AgentConnectionManager` (nhan WS msg), `AgentHookParser` (OSC parse + pattern match) | WS JSON-RPC |
| DEV | Y | `node-pty` output stream, `agent.output` JSON-RPC events (lien tuc) | JSON-RPC event |

**Chi tiết luồng:**
```
[DEV] node-pty: PTY output (OSC escape sequences + text)
  -> [DEV->SRV] JSON-RPC event: { method: 'agent.output', params: { ptyId, data } }
     [qua WS persistent Dev Server->Orca]
  -> [SRV] AgentConnectionManager receives WS message
  -> [SRV] AgentHookParser:
      ESC]133;A ST        -> status = "running"
      ESC]133;D;<code> ST -> check exit code -> "completed" / "failed"
      "waiting for input" -> status = "waiting"
      RATE_LIMIT_PATTERNS -> emit: agent:rateLimited { resetAt }
      "task completed"    -> status = "completed"
  -> [SRV->WEB] IPC push: agent:statusChanged { sessionId, status, detail }
  -> [WEB] update agent card: spinner, status badge, progress indicator
  -> [SRV->WEB] WS TweetNaCl E2E -> Mobile App (neu paired)
```

---

### 3. TERMINAL MANAGEMENT — Quản lý Terminal PTY

#### BL-TM-01 — Tạo PTY Session (qua Dev Server)

| Thành phần | Tham gia | Cấu phần sử dụng | Giao thức |
|------------|----------|------------------|-----------|
| WEB | Y | xterm.js terminal UI, WS RPC: `terminal.create` | WS RPC |
| SRV | Y | `WsSessionRouter`, `UserProcess`, `OrcaRuntime`, `DevServerRelayBridge`, `orca_sessions` DB | WS + relay JSON-RPC |
| DEV | Y | `pty-handler.ts`, `node-pty.spawn(shell)`, `ptySessionStore[userId+sessionId]`, PTY stdin/stdout | JSON-RPC 2.0 |

**Chi tiết luồng:**
```
[WEB] WebSocket RPC: terminal.create { cwd, cols, rows }
  -> [SRV] WsSessionRouter -> UserProcess (per-user child)
  -> [SRV] OrcaRuntime.terminal.create()
  -> [SRV] DevServerRelayBridge.call('terminal.create', { cwd, cols, rows, userId })
         [qua WS Dev Server da mo den Orca]
  -> [DEV] pty-handler: node-pty.spawn($SHELL, [], { cwd, cols, rows })
  -> [DEV] ptySessionStore[userId+sessionId] = ptyHandle
  -> [DEV->SRV] result: { ptyId, status: 'ready' }
  -> [SRV->WEB] terminal:created { sessionId }
  -> [WEB] xterm.js: attach I/O

Stdin flow (user types):
  [WEB] xterm onData -> WS RPC: terminal.input { sessionId, data }
  -> [SRV] relay.call('terminal.input', { ptyId, data })
  -> [DEV] ptyHandle.write(data) -> shell stdin

Stdout flow (shell output):
  [DEV] PTY data event -> relay event: terminal.output { ptyId, data }
  -> [SRV] receive -> forward to Browser WS session
  -> [WEB] xterm.js terminal.write(data) -> display
```

---

### 4. REMOTE DEVELOPMENT — Phát triển Từ xa

#### BL-SSH-02 — Deploy Orca Relay Binary

| Thành phần | Tham gia | Cấu phần sử dụng | Giao thức |
|------------|----------|------------------|-----------|
| WEB | Y | SSH host panel, "Deploy Relay" button, contextBridge.invoke | Electron IPC |
| SRV | Y | `RelayManager.deploy()`, `ssh2` lib, SFTP upload, SSH exec, `orca_ssh_hosts` SQLite | SSH (ssh2) + SFTP |
| DEV | Y | Relay binary process `~/.orca/bin/orca-relay`, WS server :6799 | WS server mode |

**Chi tiết luồng:**
```
[WEB] click "Deploy Relay" -> contextBridge.invoke('ssh.deployRelay', { hostId })
  -> [SRV] RelayManager.deploy(hostId):
      SFTP: sftp.put(relayBinaryPath, '~/.orca/bin/orca-relay')
      SSH exec: chmod +x ~/.orca/bin/orca-relay
      SSH exec: ~/.orca/bin/orca-relay start --ws-port 6799
  -> [DEV] Relay binary: WebSocket server listening :6799
  -> [SRV] SSH tunnel: localPort:6799 -> remote:6799
  -> [SRV] WebSocket connect: ws://localhost:<tunnelPort> via SSH tunnel -> DEV:6799
  -> [SRV] UPDATE orca_ssh_hosts SET relayDeployed=1, relayPort=6799
  -> [WEB] relay status: "connected"

Sau khi deploy, DEV chuyen sang mode client:
  DEV relay --ws connect--> SRV :6768/agent  (Dev Server = WS client)
  SRV --JSON-RPC--> DEV relay                (dispatch commands nguoc qua WS do)
  DEV relay --JSON-RPC events--> SRV         (stream results)
```

---

#### BL-SSH-04 — Auto Port Forwarding

| Thành phần | Tham gia | Cấu phần sử dụng | Giao thức |
|------------|----------|------------------|-----------|
| WEB | Y | Port forward card UI, "Open in Browser" button | IPC event |
| SRV | Y | `PortForwardManager.onNewPort()`, `ssh2.forwardOut()`, `port_forwards` SQLite | SSH port forward |
| DEV | Y | Relay binary port scanner, detect bound ports, `port:opened` event | Relay protocol |

**Chi tiết luồng:**
```
[DEV] Relay Binary port scanner: detect port moi bind (e.g. 3000)
  -> [DEV->SRV] relay protocol: { type: 'port:opened', port: 3000, pid: 1234 }
  -> [SRV] PortForwardManager.onNewPort():
      ssh2.forwardOut(localhost, localPort, remote, 3000)
      INSERT port_forwards { localPort, remotePort, pid, hostId }
      localUrl = http://localhost:<localPort>
  -> [SRV->WEB] IPC: portForward:created { localUrl, remotePort }
  -> [WEB] port forward card: "Port 3000 -> http://localhost:8080" + [Open in Browser]
```

---

### 5. AGENT WEBSOCKET — Protocol Kết nối Agent

#### BL-AWS-02 — direct-websocket Mode (Dev Server -> Orca)

| Thành phần | Tham gia | Cấu phần sử dụng | Giao thức |
|------------|----------|------------------|-----------|
| WEB | - | — | — |
| SRV | Y | `AgentWsRouter` (ws://orca:6768/agent), `AgentConnectionManager.register()`, `orca_agent_tokens` DB, HMAC signer | WS server + JSON-RPC |
| DEV | Y | Dev Server Agent binary (WS client), `agent.handshake { agentToken }`, JSON-RPC executor, `RpcExecutionContext` verifier | WS client + JSON-RPC |

**Chi tiết luồng:**
```
[DEV] (khoi dong) ws connect: ws://orca:6768/agent
  -> [DEV->SRV] handshake: { type: 'agent.handshake', agentToken: 'tok_xxx' }
  -> [SRV] AgentWsRouter: SELECT WHERE token_hash=SHA256(agentToken) AND is_active=1
  -> [SRV] handshake-ok: { type: 'handshake-ok', sessionId: uuid() }
  -> [SRV] AgentConnectionManager.register(sessionId, ws)  <- luu vao pool

Sau handshake -- bidirectional JSON-RPC 2.0:
  SRV --> DEV: { method: 'agent.spawn',  params: { binary, args, cwd, env, userId } }
  DEV --> SRV: { result: { ptyId, pid } }
  DEV --> SRV: { method: 'agent.output', params: { ptyId, data } }  [stream]
  DEV --> SRV: { method: 'agent.exit',   params: { ptyId, code } }

Security:
  Token: SHA-256 hash stored, plaintext khong persist
  RpcExecutionContext: HMAC-SHA256, 30s TTL, DEV verify truoc khi exec
```

---

### 6. PROFILE MANAGEMENT — Quản lý Profile

#### BL-PRF-02 — Profile Inheritance Resolution (3-layer merge)

| Thành phần | Tham gia | Cấu phần sử dụng | Giao thức |
|------------|----------|------------------|-----------|
| WEB | - | — | — |
| SRV | Y | `ProfileResolver.resolve(userId)`, in-memory cache TTL 60s, `deepMerge(company<-dept<-user)`, `orca_company`, `orca_departments`, `orca_users.profile_json` DB | Internal + DB |
| DEV | - | — | — |

**Chi tiết luồng:**
```
[SRV] ProfileResolver.resolve(userId) <- goi boi AgentManager / WorkflowOrchestrator
  -> Check in-memory cache: profileCache.get(userId)
      IF hit (TTL < 60s): return cached ResolvedProfile (khong query DB)
      IF miss:
  -> [SRV] DB: SELECT company_profile FROM orca_company
  -> [SRV] DB: SELECT dept_profile FROM orca_departments
               WHERE id=(SELECT dept_id FROM orca_users WHERE id=?)
  -> [SRV] DB: SELECT user_profile FROM orca_users WHERE id=?
  -> [SRV] deepMerge: company <- dept <- user (user overrides all)
           Arrays: user replaces (khong concat) -- ADR-007
  -> [SRV] profileCache.set(userId, resolved, TTL=60s)
  -> Return ResolvedProfile { agentModel, envVars, allowedProviders, maxConcurrentAgents }
```

---

#### BL-PRF-04 — Profile-Aware Agent Execution Routing

| Thành phần | Tham gia | Cấu phần sử dụng | Giao thức |
|------------|----------|------------------|-----------|
| WEB | Y | Task/worktree UI, "Run Agent" button | WS RPC |
| SRV | Y | `ProfileAwareAgentSpawner`, `ProfileResolver`, `ProjectServerRouter`, `AIProviderResolver`, `RelayConnectionPool` | Internal orchestration |
| DEV | Y | `node-pty.spawn(cmd)` voi merged env vars, doc credential tu `.enc` file | JSON-RPC + PTY |

**Chi tiết luồng:**
```
[WEB] "Run Agent" -> IPC -> [SRV] ProfileAwareAgentSpawner.spawn(userId, worktreeId)
  -> [SRV] ProfileResolver.resolve(userId) -> ResolvedProfile { agentModel, envVars }
  -> [SRV] ProjectServerRouter.getServer(projectId) -> devServerId: "dev-01"
  -> [SRV] RelayConnectionPool.getOrConnect("dev-01") -> relay connection
  -> [SRV] AIProviderResolver.resolve() -> { provider, apiKeyEnvVar, accountId }
  -> [SRV->DEV] relay.call('agent.spawn', {
        cmd: "claude",
        env: { ...profile.envVars, ANTHROPIC_API_KEY: <read from .enc on DEV> },
        cwd: worktreePath, userId
    })
  -> [DEV] doc ~/.orca/ai-providers/<accountId>.enc -> decrypt apiKey (in-memory only)
  -> [DEV] node-pty.spawn("claude", { env: { ...envVars, ANTHROPIC_API_KEY }, cwd })
  -> Agent running voi dung profile + provider
```

---

### 7. AI PROVIDER MANAGEMENT

#### BL-AIP-01 — Đăng ký AI Provider Account

| Thành phần | Tham gia | Cấu phần sử dụng | Giao thức |
|------------|----------|------------------|-----------|
| WEB | Y | Admin SPA, `SubtleCrypto` (AES-GCM encrypt apiKey), `POST /api/ai-providers/accounts` | HTTP POST + SubtleCrypto |
| SRV | Y | `AIProviderService.create()`, `ProviderCredentialWriter`, `AgentConnectionManager.getConnection()`, `orca_ai_provider_accounts` DB (metadata only) | REST + WS JSON-RPC |
| DEV | Y | `ai.credential.write` handler, `scrypt(masterKey+accountId)`, `AES-256-GCM encrypt`, ghi `~/.orca/ai-providers/<accountId>.enc` | JSON-RPC |

**Chi tiết luồng:**
```
STEP 1 -- Encrypt trong Browser:
[WEB] Admin nhap apiKey -> SubtleCrypto:
  key = PBKDF2(sessionToken + userId, salt)
  encryptedBlob = AES-GCM(key, apiKey)
  POST /api/ai-providers/accounts {
    name, provider, devServerId, priority, scope,
    credentialBlob: base64(encryptedBlob)   <- apiKey KHONG co plaintext
  }

STEP 2 -- Luu metadata tai Server (khong luu credential):
[SRV] AIProviderService.create():
  INSERT orca_ai_provider_accounts { id, name, provider, devServerId, ... }
  -> trigger ProviderCredentialWriter.write(accountId, devServerId, credentialBlob)

STEP 3 -- Relay credential den Dev Server:
[SRV] ProviderCredentialWriter:
  conn = AgentConnectionManager.getConnection(devServerId)
  [WS do Dev Server da mo den Orca]
  decrypt session layer -> lay lai plaintext apiKey (in-memory)
  -> [SRV->DEV] JSON-RPC: ai.credential.write {
       accountId, provider, credentials: { apiKey }  <- plaintext chi trong transit
     }

[DEV] ai.credential.write handler:
  masterKey = scrypt(ORCA_AI_CREDENTIAL_KEY + ':' + accountId, accountId)
  stored = AES-256-GCM(masterKey, apiKey)
  ghi file: ~/.orca/ai-providers/<accountId>.enc
  -> [DEV->SRV] JSON-RPC result: { ok: true }

Security: apiKey plaintext chi ton tai in-memory tai SRV trong thoi gian transit WS
```

---

#### BL-AIP-03 — Provider Health Check & Quota

| Thành phần | Tham gia | Cấu phần sử dụng | Giao thức |
|------------|----------|------------------|-----------|
| WEB | Y | Admin dashboard `GET /api/ai-providers/health` | HTTP REST |
| SRV | Y | `ProviderHealthChecker` (cron 15min), `AgentConnectionManager`, `orca_ai_provider_accounts` DB, `orca_provider_usage` DB, Alert webhook | Cron + WS JSON-RPC |
| DEV | Y | `ai.ping` handler, doc `.enc` file, decrypt apiKey, goi test API (GET /v1/models) | JSON-RPC + external API |

**Chi tiết luồng:**
```
[SRV] Cron moi 15 phut -> ProviderHealthChecker:
  FOR each account WHERE status IN ('healthy', 'degraded') (parallel per server):
    conn = AgentConnectionManager.getConnection(account.devServerId)
    -> [SRV->DEV] JSON-RPC: ai.ping { accountId, provider }
    -> [DEV] doc ~/.orca/ai-providers/<accountId>.enc -> decrypt apiKey (in-memory)
    -> [DEV] GET <provider_api>/v1/models (test call)
    -> [DEV->SRV] JSON-RPC result: { latencyMs, ok: true/false }
    -> [SRV] UPDATE orca_ai_provider_accounts SET status, latencyMs, lastCheckedAt
    IF status changed:
      -> [SRV->WEB] WS push: provider status changed
      -> [SRV] POST webhookUrl (Slack/PagerDuty neu cau hinh)

Quota tracking (sau moi agent/workflow hoan thanh):
  [SRV] recordTokenUsage(accountId, tokensUsed)
  -> UPSERT orca_provider_usage(account_id, date, tokens_used)
  IF usage > 80% quotaLimit: sendAlert('quota_warning_80pct')
  IF usage >= quotaLimit:
    UPDATE status='quota_exceeded'
    [AIProviderResolver se skip account nay -> next in cascade]
```

---

### 8. WORKFLOW ORCHESTRATION

#### BL-WF-02 — Multi-Server Workflow Execution

| Thành phần | Tham gia | Cấu phần sử dụng | Giao thức |
|------------|----------|------------------|-----------|
| WEB | Y | Workflow builder UI, "Execute" button, SSE progress monitor | HTTP POST + SSE |
| SRV | Y | `WorkflowOrchestrator`, `TemplateResolver`, `WorkflowServerResolver`, `StepExecutors`, `orca_workflow_executions` DB, `orca_step_executions` DB | REST + WS JSON-RPC |
| DEV | Y | `agent.spawn` handler (agent steps), `shell.exec` handler (shell steps), PTY + shell execution | JSON-RPC |

**Chi tiết luồng:**
```
[WEB] POST /api/workflows/execute { templateId, params: { task: "..." } }
  -> [SRV] WorkflowOrchestrator.execute():
      TemplateResolver.resolve(templateId) -> merged template
      WorkflowServerResolver: "project:vnp-blc" -> devServerId: "dev-01"
      BUILD DAG + topological sort:
        [plan] -> [implement] -> [test || format] -> [review]
      INSERT orca_workflow_executions { id, status: 'running' }

  WAVE 1: [plan] (no deps)
    -> [SRV->DEV] relay.call('agent.spawn', { cmd: 'claude', prompt: 'Analyze...' })
    -> [DEV] node-pty.spawn('claude') + inject prompt
    -> [DEV->SRV] agent.output stream -> wait agent:complete
    -> [SRV] UPDATE orca_step_executions SET status='done', output=planOutput
    -> [SRV->WEB] SSE: step 'plan' completed

  WAVE 2: [implement] <- inject planOutput as context
    -> [SRV->DEV] relay.call('agent.spawn', { prompt: planOutput + 'Implement...' })

  WAVE 3: [test] || [format] -- parallel on same or different DEV servers
    -> [SRV->DEV-01] relay.call('shell.exec', { command: 'npm test' })
    -> [SRV->DEV-01] relay.call('shell.exec', { command: 'make fmt' })

  WAVE 4: [review] <- all previous done
    -> [SRV->DEV] relay.call('agent.spawn', { prompt: 'Review...' })
    -> [DEV->SRV] agent:complete

  -> [SRV] UPDATE orca_workflow_executions SET status='completed'
  -> [SRV->WEB] SSE: workflow:completed { executionId, summary }
```

---

### 9. TASK GRAPH MANAGEMENT

#### BL-TG-04 — Task Prompt -> Agent Execution

| Thành phần | Tham gia | Cấu phần sử dụng | Giao thức |
|------------|----------|------------------|-----------|
| WEB | Y | Task detail UI, "Run Agent" button, SSE execution feed | HTTP POST + SSE |
| SRV | Y | `TaskAgentExecutor`, `TaskGrantService`, `ProfileAwareAgentSpawner`, `ProfileResolver`, `AIProviderResolver`, `orca_tasks` DB | REST + WS JSON-RPC |
| DEV | Y | `agent.spawn` handler, `node-pty.spawn(cmd)`, PTY stream, AI agent process | JSON-RPC + PTY |

**Chi tiết luồng:**
```
[WEB] POST /api/tasks/:id/execute
  -> [SRV] TaskAgentExecutor.execute(taskId, userId):
      hasPermission(userId, taskId, 'execute')  <- DB: 5-level grant check
      check blocking deps: SELECT depends_on WHERE not status='done' -> BLOCKED_BY_DEPS?
      load task + parent + project context  <- DB

      build preamble:
        "You are working on Task #<id>: <title>
         Type: <type> | Priority: <priority>
         Description: <description>
         AI Context: <ai_context>
         Parent task: <parent.title>  (if exists)"

  -> [SRV] ProfileAwareAgentSpawner.spawn(userId, worktreeId):
      ProfileResolver -> ResolvedProfile (BL-PRF-02, cached)
      AIProviderResolver -> ProviderConfig { accountId, devServerId }
      relay = RelayConnectionPool.getOrConnect(devServerId)

  -> [SRV->DEV] relay.call('agent.spawn', {
        cmd: providerConfig.agentCommand,    // "claude" / "codex" / etc
        env: { ...profile.envVars, [apiKeyEnvVar]: '<key from .enc>' },
        cwd: worktreePath,
        initialPrompt: preamble
    })
  -> [DEV] node-pty.spawn(cmd, { env, cwd })
  -> [DEV] inject initialPrompt vao PTY stdin sau khi agent idle

  -> [SRV] UPDATE orca_tasks SET status='in_progress', agent_session_id=?
  -> [DEV->SRV] agent.output stream (real-time)
  -> [SRV->WEB] SSE: task:agentOutput { data }       [real-time display]
  -> [DEV->SRV] agent.exit { code: 0 }
  -> [SRV] UPDATE orca_tasks SET status='review'
  -> [SRV->WEB] SSE: task:agentCompleted { taskId }
```

---

### 10. PROJECT WORKSPACE

#### BL-PW-01 — Project Workspace Context

| Thành phần | Tham gia | Cấu phần sử dụng | Giao thức |
|------------|----------|------------------|-----------|
| WEB | Y | Explorer, Git panel, Agent panel, Tasks panel (shared WebSocket) | WS RPC |
| SRV | Y | `WorkspaceContextManager`, `ProjectService`, `RelayConnectionPool`, `FleetHealthMonitor`, `ProfileResolver`, `WorkflowService` | WS + relay JSON-RPC |
| DEV | Y | `git.status`, `git.worktree.list`, `fs.readDir` handlers, relay binary | JSON-RPC |

**Chi tiết luồng:**
```
[WEB] sidebar click project -> WS RPC: workspace.switch({ projectId })
  -> [SRV] WorkspaceContextManager.switch(userId, projectId):
      hasAccess(userId, projectId, 'view')  <- DB
      project = ProjectService.get(projectId)  <- DB (name, repoPath, devServerId)
      server = SshHostService.get(project.devServerId)  <- DB
      relay = RelayConnectionPool.get(project.devServerId)
      healthStatus = FleetHealthMonitor.getCached(server.id)
      IF unreachable: offlineMode=true (read-only, cached data)

  -> [SRV] Promise.all (parallel):
      [SRV->DEV] relay.call('git.status',        { cwd: project.repoPath })
      [SRV->DEV] relay.call('git.worktree.list', { repoPath })
      [SRV->DEV] relay.call('fs.readDir',        { path: repoPath, depth: 2 })
      [SRV]      WorkflowService.getActiveExecutions(projectId)  <- DB

  -> [DEV] git status --porcelain / git worktree list / fs.readdir()
  -> [DEV->SRV] JSON-RPC results (all parallel)

  -> [SRV] ProfileResolver.resolve(userId) -> ResolvedProfile (cached/DB)
  -> [SRV] Build WorkspaceContext { gitStatus, worktrees, fileTree, workflows, profile }
  -> [SRV->WEB] WS push: workspaceContext (render all 4 panels simultaneously)
  -> [SRV] Start background polls:
      git status: moi 5s (khi Git tab active hoac agent running)
      server health: moi 30s
```

---

#### BL-PW-03 — Remote Git UI Operations

| Thành phần | Tham gia | Cấu phần sử dụng | Giao thức |
|------------|----------|------------------|-----------|
| WEB | Y | Git panel: stage/unstage/commit/push/PR buttons, progress bar | WS RPC |
| SRV | Y | `relay.call('git.*')`, `RelayConnectionPool` | WS JSON-RPC relay |
| DEV | Y | `git-engine`: git CLI (add/commit/push/diff), `gh pr create`, stream output | JSON-RPC + Git CLI |

**Chi tiết luồng:**
```
Toan bo luong: Stage -> Commit -> Push -> PR

[WEB] git.stage({ files: ['src/app.ts'] })
  -> [SRV->DEV] relay.call('git.add', { files })
  -> [DEV] git add src/app.ts
  -> [DEV->SRV] refresh git.status -> [SRV->WEB] Git panel update

[WEB] git.commit({ message: 'feat: add OAuth' })
  -> [SRV->DEV] relay.call('git.commit', { message })
  -> [DEV] git commit -m "feat: add OAuth"

[WEB] git.push({ branch: 'feature/oauth' })
  -> [SRV->DEV] relay.call('git.push', { branch })
  -> [DEV] git push origin feature/oauth  (stream progress)
  -> [DEV->SRV] progress events stream
  -> [SRV->WEB] WS push: progress bar updates

[WEB] github.pr.create({ title, body, head, base })
  -> [SRV->DEV] relay.call('github.pr.create', { title, body, head, base })
  -> [DEV] gh pr create ... (su dung GitHub token tu ~/.orca/credentials.enc)
  -> [DEV->SRV] result: { prUrl }
  -> [SRV->WEB] prUrl displayed + task.prUrl updated
```

---

### 11. FLEET MANAGEMENT

#### BL-FLEET-03 — Fleet Health Monitoring

| Thành phần | Tham gia | Cấu phần sử dụng | Giao thức |
|------------|----------|------------------|-----------|
| WEB | Y | Admin SPA health dashboard, color-coded status (green/yellow/red) | WS/SSE event |
| SRV | Y | `FleetHealthMonitor` (cron 30s), `AgentConnectionManager`, `health_metrics` DB, webhook alerts | Cron + WS JSON-RPC |
| DEV | Y | `health.get` handler: CPU/RAM/disk metrics, agentCount, latency | JSON-RPC |

**Chi tiết luồng:**
```
[SRV] Cron moi 30 giay -> FleetHealthMonitor:
  FOR each active server in orca_dev_servers (parallel):
    conn = AgentConnectionManager.getConnection(serverId)
    [WS do Dev Server da mo den Orca -- khong can mo moi]
    -> [SRV->DEV] relay.call('health.get')
    -> [DEV] collect metrics:
        { cpu: 45%, ram: 60%, disk: 30%, agentCount: 2, latency: 12ms }
    -> [DEV->SRV] JSON-RPC result { metrics }

    IF timeout 5s: status = 'unreachable'
    IF cpu > 90% OR ram > 90%: status = 'warning'
    IF disk > 95%: status = 'critical'

    -> [SRV] INSERT health_metrics { serverId, cpu, ram, disk, latency, timestamp }
    IF status changed:
      -> [SRV->WEB] WebSocket event: fleet:serverStatusChanged { oldStatus, newStatus }
      -> [SRV] POST webhookUrl { serverId, status, metrics } (Slack/PagerDuty)

[WEB] health dashboard: badges cap nhat real-time
      green = healthy | yellow = warning | red = critical/unreachable
```

---

#### BL-FLEET-02 — Bulk Server Provisioning

| Thành phần | Tham gia | Cấu phần sử dụng | Giao thức |
|------------|----------|------------------|-----------|
| WEB | Y | Fleet panel, "Provision All" button, progress table | HTTP POST + SSE |
| SRV | Y | `FleetProvisioner.provision()`, `ssh2` lib, SFTP, `orca_dev_servers` DB | SSH + SFTP |
| DEV | Y | Nhan relay binary, systemd service, relay start | Shell exec |

**Chi tiết luồng:**
```
[WEB] POST /admin/api/fleet/provision { serverIds[], template: 'standard' }
  -> [SRV] FleetProvisioner.provision():
      FOR each serverId (parallel Promise.all):
        ssh2.connect(server)
        SFTP: sftp.put(relayBinaryPath, '~/.orca/bin/orca-relay')
        SSH exec: chmod +x ~/.orca/bin/orca-relay
        SSH exec: systemctl enable orca-relay && systemctl start orca-relay
        SSH exec: orca-relay --version  (verify)
        UPDATE orca_dev_servers SET status='active', relayVersion=?  <- DB
        emit: server:provisioned { serverId }
  -> [SRV->WEB] SSE/WS: per-server progress updates
  -> [WEB] progress table: per-server status (success/failed)
```

---

## Ma trận Giao thức x Thành phần

| Giao thức | WEB -> SRV | SRV -> DEV | DEV -> SRV |
|-----------|------------|------------|------------|
| HTTP REST | POST/PATCH/DELETE /api/* | - | - |
| WebSocket | ws://orca:6768 (user session) | - (SRV KHONG mo WS den DEV) | ws://orca:6768/agent (DEV la client) |
| JSON-RPC 2.0 | - | Gui qua WS connection DEV da mo | Events/results qua WS connection do |
| SSE | <- SRV push events (workflow, task) | - | - |
| SSH (ssh2) | - | SSH connect (provisioning, relay deploy only) | - |
| Unix Socket | - | (Internal SRV only: WsSessionRouter <-> Child Process) | - |

> **Quy tac ket noi quan trong:**
> - **Dev Server LUON la WebSocket client** -- mo ket noi den `ws://orca:6768/agent` khi khoi dong
> - **Orca Server KHONG bao gio mo WebSocket den Dev Server** (ngoai tru fleet provisioning dung SSH)
> - Moi lenh dieu phoi tu Orca -> Dev Server deu di **nguoc qua WS connection DEV da mo**
> - Dev Server tra ket qua va stream events **qua cung WS connection do**
> - Keepalive ping: SRV gui moi 30s qua WS, DEV pong lai de giu connection song

---

## Phan loai luong theo tinh phuc tap

### Chi WEB <-> SRV (khong co DEV)
Cac luong thuan backend, khong can thuc thi tren dev server:

| Nhom | Luong | Mo ta |
|------|-------|-------|
| Auth | BL-AUTH-01 den 05 | Toan bo auth: login, session, sandbox, admin CRUD, audit |
| Profile | BL-PRF-01/02/03 | Profile CRUD, inheritance resolution, project assignment |
| Agent Token | BL-AWS-03 | Tao/revoke agent tokens |
| Workflow | BL-WF-01/03 | Template CRUD, workflow sharing & library |
| Task | BL-TG-01/02/03 | Task CRUD, AI planning (inline call), access control |
| Integration | BL-PI-01/03/04 | GitHub/GitLab issue sync, status update, PR review |
| CLI | BL-CLI-01/02/03 | CLI <-> Daemon (backend only, Unix Socket) |
| Fleet Config | BL-FLEET-01 | Fleet inventory config (YAML/UI) |

### WEB <-> SRV <-> DEV (full 3-tier)
Cac luong phuc tap nhat -- can thuc thi thuc su tren dev server:

| Nhom | Luong | Diem dac biet |
|------|-------|---------------|
| Agent | BL-AG-01 den 05 | Agent lifecycle, PTY spawn + stream qua WS |
| Terminal | BL-TM-01 den 04 | Terminal PTY qua relay JSON-RPC |
| AI Credential | BL-AIP-01/03 | Credential write + health check qua WS JSON-RPC |
| Profile Agent | BL-PRF-04 | Profile-aware agent spawn (merge 3 layers + route) |
| Workflow | BL-WF-02 | Multi-server workflow, wave execution, DAG |
| Task Execute | BL-TG-04 | Task -> agent exec end-to-end |
| Workspace | BL-PW-01 den 04 | Workspace context, file explorer, git ops, integration |
| Fleet | BL-FLEET-02/03/04 | Provisioning (SSH), health monitoring, onboarding |
| Remote Dev | BL-SSH-01 den 04 | SSH connect, relay deploy, reconnect, port forward |

---

## Lien ket den tai lieu chi tiet

| Domain | Logic Flow | Code Flow |
|--------|------------|-----------|
| Auth | [auth.md](./logic/auth.md) | [auth/](./code/auth/) |
| Agent Orchestration | [agent-orchestration.md](./logic/agent-orchestration.md) | [agent-orchestration/](./code/agent-orchestration/) |
| Agent WebSocket | [agent-ws.md](./logic/agent-ws.md) | [agent-ws/](./code/agent-ws/) |
| Terminal | [terminal-management.md](./logic/terminal-management.md) | [terminal-management/](./code/terminal-management/) |
| Remote Dev | [remote-development.md](./logic/remote-development.md) | [remote-development/](./code/remote-development/) |
| Profile | [profile.md](./logic/profile.md) | [profile/](./code/profile/) |
| AI Providers | [ai-providers.md](./logic/ai-providers.md) | [ai-providers/](./code/ai-providers/) |
| Workflow | [workflow-orchestration.md](./logic/workflow-orchestration.md) | [workflow-orchestration/](./code/workflow-orchestration/) |
| Task Graph | [task-graph.md](./logic/task-graph.md) | [task-graph/](./code/task-graph/) |
| Project Workspace | [project-workspace.md](./logic/project-workspace.md) | [project-workspace/](./code/project-workspace/) |
| Fleet | [fleet.md](./logic/fleet.md) | [fleet/](./code/fleet/) |
| Project Integration | [project-integration.md](./logic/project-integration.md) | [project-integration/](./code/project-integration/) |
| CLI & Headless | [cli-headless.md](./logic/cli-headless.md) | [cli-headless/](./code/cli-headless/) |
