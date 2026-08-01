# Luồng Dữ liệu — Agent Orchestration

**Domain:** Agent Orchestration  
**Nghiệp vụ:** BL-AG-01 → BL-AG-05  
**Kiến trúc tham chiếu:** HLD v1 — C3.1 Main Process, Dev Server Agent, F04 AI Agent Support

---

## Thành phần tham gia

| Thành phần | Layer | Vai trò |
|------------|-------|---------|
| Renderer (React UI) | UI | Agent card, sidebar status, prompt input |
| contextBridge / Preload | IPC | Electron sandbox bridge |
| Main Process / Orca Web Server | Business Logic | AgentManager, AgentSpawner, OSC hook parser |
| AgentConnectionManager | Backend | Quản lý pool WebSocket connection — mỗi Dev Server mở 1 WS đến Orca |
| Dev Server Agent | Remote PTY Server | Chạy node-pty.spawn(agent) trên Dev Server; **chủ động connect WS đến Orca** |
| Server Database | Persistence | orca_sessions (sessionId, worktreeId, agentType, devServerId) |
| AI Credential File | Secrets | `~/.orca/ai-providers/<accountId>.enc` trên Dev Server |
| AI Agent Process | Remote Process | Claude Code, Codex, OpenCode, Gemini CLI — chạy trên Dev Server |

> **Hướng kết nối:**  
> Dev Server chủ động **mở WebSocket kết nối đến Orca Server** (`ws://orca:6768/agent`).  
> Orca Server nhận, xác thực agentToken, lưu connection vào `AgentConnectionManager`.  
> Khi cần spawn agent, Orca Server **gửi JSON-RPC request ngược lại qua kết nối đó**.

---

## Sơ đồ kết nối cơ bản

```
Dev Server                                    Orca Server
(172.20.2.31)                                 (172.20.2.39)
    │                                              │
    │── WS connect ws://orca:6768/agent ──────────►│
    │   agent.handshake { agentToken }             │
    │◄─ handshake-ok { sessionId } ───────────────│
    │                                              │
    │◄════════════ JSON-RPC 2.0 ══════════════════►│
    │   (bidirectional — Dev Server đã mở kết nối) │
    │                                              │
    │  Khi Orca muốn spawn agent:                  │
    │◄── JSON-RPC request: agent.spawn { ... } ───│
    │── JSON-RPC result: { ptyId, pid } ──────────►│
    │                                              │
    │  PTY output stream:                          │
    │── JSON-RPC event: agent.output { data } ────►│
```

---

## BL-AG-01 — Khởi động AI Agent

```
Người dùng (Alex/Maya/Carlos/Sam)
    │
    ▼
[Renderer] click "Start Agent" trên worktree card
    │ contextBridge.invoke('agent.start', { worktreeId, agentType, trustPreset })
    ▼
[Main Process — AgentManager.start()]
    ├─ Load AgentConfig { binary, startupCommand, args, trustPresetEnvVars }
    ├─ ProfileResolver.resolve(userId) → ResolvedProfile (env vars, model)
    ├─ AIProviderResolver.resolve(userId, projectId, devServerId)
    │     → { provider, agentCommand, apiKeyEnvVar }
    ├─ Build agent env:
    │     { ...profile.envVars, ...trustPresetEnvVars }
    │     (apiKey được đọc trực tiếp từ file .enc trên Dev Server khi spawn)
    │
    ├─ Lấy WS connection đã có từ AgentConnectionManager:
    │     conn = AgentConnectionManager.getConnection(devServerId)
    │     [connection này do Dev Server đã mở sẵn đến Orca]
    │
    ├─ Gửi JSON-RPC request qua WS connection đó:
    │     conn.call('agent.spawn', {          ← JSON-RPC → Dev Server
    │       agentBinary,
    │       args,
    │       cwd: worktreePath,               ← path trên Dev Server
    │       env,
    │       userId,                           ← per-userId PTY isolation
    │       cols, rows
    │     })
    │
    ▼
[Dev Server Agent — nhận JSON-RPC request]
    ├─ Verify RpcExecutionContext (HMAC-SHA256, 30s TTL)
    ├─ node-pty.spawn(agentBinary, args, { cwd: worktreePath, env })
    ├─ Lưu PTY handle: ptySessionStore[userId + worktreeId] = ptyHandle
    └─ Trả JSON-RPC result: { ptyId, pid }
    │
    ▼
[Dev Server → stream PTY output về Orca qua WS đang mở]
    │ JSON-RPC event: agent.output { ptyId, data: "<OSC output>" }
    ▼
[Main Process — AgentHookParser]
    ├─ Parse OSC 133 sequences → detect state: "idle"
    └─ INSERT orca_sessions { id, worktreeId, agentType, devServerId, startedAt }  ← DB
    │
    ▼
[Main Process] emit: agent:started { sessionId, status: 'idle' }
    │
    ▼
[Renderer] agent card: status = "idle"

Luồng:
User → Renderer → IPC → Main (build env + resolve profile/provider)
                       → AgentConnectionManager.getConnection(devServerId)
                         [WS do Dev Server đã mở sẵn đến Orca]
                       → JSON-RPC call: agent.spawn → Dev Server
                       → Dev Server: node-pty.spawn(agentBinary)
                       → JSON-RPC event: agent.output [WS stream ngược về Orca]
                       → Main: OSC parse → status detect
                       → Server DB: INSERT session
                       → Renderer: agent:started event
```

**Input:** `{ worktreeId, agentType, trustPreset }`  
**DB:** INSERT orca_sessions (bao gồm devServerId)  
**Event:** `agent:started { sessionId, status: 'idle' }`

---

## BL-AG-02 — Dừng Agent

```
Người dùng
    │
    ▼
[Renderer] click "Stop" trên agent card
    │ contextBridge.invoke('agent.stop', { sessionId, force: false })
    ▼
[Main Process — AgentManager.stop()]
    ├─ conn = AgentConnectionManager.getConnection(devServerId)
    ├─ conn.call('agent.sendInput', {           ← JSON-RPC → Dev Server
    │     ptyId,
    │     data: '\x03'                          ← Ctrl+C (interrupt)
    │   })
    │     → Dev Server: ptyHandle.write('\x03') → PTY stdin
    ├─ Wait 10s cho graceful exit
    │     (PTY close event stream về qua WS: agent.exit { ptyId, code })
    ├─ UPDATE orca_sessions SET status='stopped'  ← Server DB
    └─ emit: agent:stopped

[A1] Timeout 10s → hiển thị "Force Kill?" dialog
    │ contextBridge.invoke('agent.stop', { sessionId, force: true })
    ▼
[Main Process]
    ├─ conn.call('agent.kill', {                ← JSON-RPC → Dev Server
    │     ptyId,
    │     signal: 'SIGKILL'
    │   })
    │     → Dev Server: ptyHandle.kill('SIGKILL')
    └─ Dev Server: xóa ptySessionStore[userId + worktreeId]

Luồng:
User → Renderer → IPC → Main
                       → JSON-RPC: agent.sendInput(Ctrl+C) [qua WS Dev Server đã mở]
                       → Dev Server: PTY stdin write
                       → [timeout] → JSON-RPC: agent.kill [qua WS]
                       → Server DB: UPDATE session status
                       → Renderer: agent:stopped event
```

---

## BL-AG-03 — Resume Agent Session

```
Người dùng (Alex/Maya/Carlos)
    │
    ▼
[Renderer] click "Resume" trên worktree cũ
    │ contextBridge.invoke('agent.resume', { worktreeId })
    ▼
[Main Process — AgentManager.resume()]
    ├─ SELECT sessionId, devServerId FROM orca_sessions
    │     WHERE worktreeId=? ORDER BY startedAt DESC   ← Server DB
    ├─ Build resume args:
    │     Claude: ["claude", "--resume", sessionId]
    │     Codex:  ["codex", "--session-file", sessionFilePath]
    ├─ conn = AgentConnectionManager.getConnection(devServerId)
    └─ conn.call('agent.spawn', {               ← JSON-RPC → Dev Server
           agentBinary, args: [...resumeArgs],
           cwd: worktreePath, env, userId
         })
       → Dev Server: node-pty.spawn(agent --resume <id>)
       → stream output → Main: OSC parse → agent:resumed

Luồng:
User → Renderer → IPC → Main → Server DB (load sessionId + devServerId)
                              → JSON-RPC: agent.spawn(resumeArgs)
                                [qua WS Dev Server đã mở đến Orca]
                              → Dev Server: node-pty.spawn(agent --resume)
                              → PTY output stream → Main: OSC parse
                              → Renderer: agent:resumed, context summary
```

---

## BL-AG-04 — Switch Account / Provider

```
[Dev Server → PTY output → JSON-RPC event: agent.output → Main]
    │ Main parse: rate-limit pattern match
    │ emit: agent:rateLimited { pattern: "rate limit exceeded", resetAt }
    ▼
[Renderer] alert: "Claude Code bị rate limited. Reset lúc HH:MM"
    │ Options: [Switch account 2] [Switch provider] [Wait]
    ▼
[Main Process — AgentManager.switchAccount()]
    ├─ UPDATE orca_sessions SET status='stopped'    ← Server DB
    ├─ BL-AG-02: conn.call('agent.kill', { ptyId })
    │     → Dev Server: kill PTY
    ├─ AIProviderResolver.resolve() với account mới
    │     → resolvedProvider: { provider: 'openai', apiKeyEnvVar: 'OPENAI_API_KEY' }
    ├─ BL-AG-01: conn.call('agent.spawn', { newEnv }) → Dev Server
    └─ BL-AG-03: conn.call('agent.spawn', { resumeArgs }) nếu compatible

Luồng:
Dev Server PTY output → JSON-RPC event [WS] → Main (pattern match)
                     → Renderer (alert)
User choice → IPC → Main
                   → JSON-RPC: agent.kill [WS] → Dev Server: stop PTY
                   → AIProviderResolver (new credentials)
                   → JSON-RPC: agent.spawn(newEnv) [WS] → Dev Server: new PTY
```

---

## BL-AG-05 — Monitor Trạng thái Agent Real-time

```
[Dev Server — node-pty] PTY output (OSC escape sequences + text)
    │ Liên tục gửi JSON-RPC event qua WS kết nối đã mở:
    │ { jsonrpc: '2.0', method: 'agent.output', params: { ptyId, data } }
    ▼
[Orca Server — AgentConnectionManager receives WS message]
    │
    ▼
[Main Process — AgentHookParser]
    ├─ Parse OSC 133 sequences:
    │   ESC]133;A ST → command started  → status = "running"
    │   ESC]133;D;<code> ST → finished → check exit code
    ├─ Pattern match text:
    │   "waiting for input"  → status = "waiting"
    │   RATE_LIMIT_PATTERNS  → emit: agent:rateLimited
    │   "task completed"     → status = "completed"
    └─ emit: agent:statusChanged { sessionId, status, detail }
    │
    ▼
[Main Process] → IPC event → Renderer
    └─ Update agent card: status indicator, spinner, progress
    └─ WebSocket (TweetNaCl E2E) → Mobile App (Sam) nếu paired

Luồng:
Dev Server PTY → JSON-RPC event [WS Dev Server→Orca, persistent]
             → Orca AgentConnectionManager receive
             → AgentHookParser (OSC parse + pattern match)
             → Renderer (IPC push)
             → Mobile App (WS TweetNaCl push)
```

---

## Sơ đồ tổng quan — Agent Orchestration

```
┌─────────────┐   IPC (contextBridge)   ┌──────────────────────────────────────┐
│  Renderer   │◄───────────────────────►│  Orca Server (172.20.2.39)           │
│  Agent card │                         │  AgentManager                        │
│  Status     │                         │  AgentHookParser (OSC parse)         │
│  Prompt UI  │                         │  AgentConnectionManager              │
└──────┬──────┘                         │  (pool WS connections từ Dev Servers)│
       │                                └────────────────┬─────────────────────┘
       │ WS TweetNaCl E2E                                │
       ▼                                                 │ WebSocket
┌──────────────┐                                         │ (Dev Server đã connect
│ Mobile App   │                                         │  vào đây, persistent)
│ (Sam)        │                                         │
└──────────────┘                                         │
                                                         │
                              ┌──────────────────────────▼──────────────────────┐
                              │  Dev Server (172.20.2.31)                        │
                              │  Dev Server Agent (agent.js / systemd service)   │
                              │                                                  │
                              │  [Khởi động]: ws connect → ws://orca:6768/agent │
                              │              agent.handshake { agentToken }      │
                              │              ← handshake-ok { sessionId }        │
                              │                                                  │
                              │  [Nhận JSON-RPC từ Orca]:                        │
                              │  ← agent.spawn { binary, args, cwd, env, userId}│
                              │  → result { ptyId, pid }                         │
                              │                                                  │
                              │  [Stream output về Orca]:                        │
                              │  → agent.output { ptyId, data }                 │
                              │  → agent.exit   { ptyId, code }                 │
                              │                                                  │
                              │  node-pty.spawn(agentBinary)                    │
                              │  ptySessionStore[userId+worktreeId] = handle    │
                              └──────────────────┬───────────────────────────────┘
                                                 │ PTY stdin/stdout
                              ┌──────────────────▼───────────────────────────────┐
                              │  AI Agent Process (trên Dev Server)              │
                              │  claude / codex / opencode / gemini              │
                              │  cwd = worktreePath (filesystem trên Dev Server) │
                              └──────────────────────────────────────────────────┘

Chiều kết nối WebSocket:
  Dev Server ──WS connect──► Orca Server   (Dev Server là WS client)
  Orca Server ──JSON-RPC──► Dev Server     (gửi request qua connection đó)
  Dev Server ──JSON-RPC──► Orca Server     (stream events ngược lại)
```
