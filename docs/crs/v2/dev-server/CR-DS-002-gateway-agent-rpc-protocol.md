# CR-DS-002 — Gateway ↔ Dev Server Agent RPC Protocol

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-DS-002 |
| **Tên** | Gateway–Agent RPC Protocol v3 |
| **Loại** | Protocol Specification |
| **Priority** | P0 — Critical |
| **Phiên bản** | v6.0 |
| **Ngày tạo** | 2026-07-30 |
| **Trạng thái** | Proposed |
| **Phụ thuộc** | CR-DS-001 |
| **Tác động HLD** | C3-components (C3.13), C4-code (C4.11) |

---

## Tổng quan Protocol

### Transport Layer

```
Orca Backend Server ←──────────────────────────────────────────→ Dev Server Agent
                     Persistent WebSocket (wss://) hoặc SSH tunnel
                     
Option A (recommended): Agent kết nối ra ngoài (outbound WS)
  Agent → wss://orca-backend.company.com/agent/connect
  Header: Authorization: Bearer <agent-registration-token>

Option B (fallback): Backend SSH vào agent
  Backend → SSH → Agent listening on unix socket
  (dùng khi agent behind NAT, không thể outbound HTTPS)
```

### Protocol Frame (JSON-RPC 2.0 over WebSocket)

```
┌──────────────────────────────────────────────────────┐
│ Direction   │ Message Type    │ Flow                  │
├─────────────┼─────────────────┼───────────────────────┤
│ Gateway→Ag  │ rpc.request     │ Method call + params  │
│ Gateway→Ag  │ rpc.cancel      │ Cancel running request│
│ Ag→Gateway  │ rpc.response    │ Result / error        │
│ Ag→Gateway  │ event.stream    │ PTY output stream     │
│ Ag→Gateway  │ event.agentStat │ AI agent status change│
│ Ag→Gateway  │ event.health    │ Health metrics (60s)  │
│ Ag→Gateway  │ event.gitChange │ git status change     │
│ Ag→Gateway  │ event.log       │ Agent log entry       │
│ Both        │ ping/pong       │ Keepalive             │
└──────────────────────────────────────────────────────┘
```

---

## Handshake Sequence

```
Agent boots
  │
  ├─▶ Agent connects: wss://orca-backend/agent/connect
  │   Header: Authorization: Bearer <agentToken>
  │
  ├─◀─ Gateway: { type: 'handshake.challenge', nonce: 'xyz...' }
  │
  ├─▶ Agent: {
  │     type: 'handshake.response',
  │     nonce: 'xyz...',
  │     agentId: 'agent-uuid',
  │     capabilities: { ... },   // AgentCapabilities
  │     signature: HMAC(nonce + agentId, sharedSecret)
  │   }
  │
  ├─◀─ Gateway: {
  │     type: 'handshake.ok',
  │     sessionKey: '<session-key>',   // for this connection
  │     config: {
  │       heartbeatIntervalMs: 30000,
  │       maxConcurrentRpcs: 20,
  │       logLevel: 'info'
  │     }
  │   }
  │
  └─▶ Agent starts sending health events every 60s
      Agent ready to receive RPC calls
```

---

## RPC Methods — Gateway → Agent

### PTY & Terminal

```typescript
// Tạo PTY session mới
'pty.create'(params: {
  sessionId: string           // assigned by gateway, must be unique
  userId: string              // for audit / isolation
  projectId: string
  workdir: string             // absolute path on dev server
  shell?: string              // '/bin/bash' | '/bin/zsh' etc.
  env?: Record<string, string>
  cols: number
  rows: number
}) → { ptyId: string, pid: number }

// Resize PTY
'pty.resize'(params: { ptyId: string, cols: number, rows: number }) → void

// Write to PTY stdin
'pty.write'(params: { ptyId: string, data: string }) → void

// Kill PTY
'pty.kill'(params: { ptyId: string, signal?: string }) → void

// List active PTYs (for session resume)
'pty.list'(params: { userId?: string }) → { ptys: PtyInfo[] }
```

### AI Agent Operations

```typescript
// Spawn AI agent trong PTY
'agent.spawn'(params: {
  ptyId: string               // PTY để spawn agent trong đó
  agentType: string           // 'claude' | 'codex' | 'gemini' | ...
  providerAccountId: string   // F35 account ID (credentials on agent)
  model?: string
  trustPreset: 'minimal' | 'standard' | 'permissive'
  workdir: string
  prompt?: string             // initial prompt injection
  sessionId?: string          // for --resume
  envOverrides?: Record<string, string>
  taskPreamble?: string       // F37 task context
}) → { agentSessionId: string }

// Kill agent process
'agent.kill'(params: { agentSessionId: string }) → void

// Send prompt to running agent
'agent.sendPrompt'(params: { agentSessionId: string, prompt: string }) → void

// Detect available agents on PATH
'agent.detect'(params: {}) → {
  agents: {
    name: string, version: string, path: string, available: boolean
  }[]
}
```

### Worktree Operations

```typescript
'worktree.create'(params: {
  repoPath: string
  branch?: string
  baseBranch: string
  worktreeName: string
  userId: string
  projectId: string
}) → { worktreeId: string, path: string }

'worktree.list'(params: { repoPath: string, userId?: string }) → {
  worktrees: WorktreeInfo[]
}

'worktree.remove'(params: {
  worktreePath: string
  force?: boolean
}) → { success: boolean }

'worktree.fanout'(params: {
  repoPath: string
  baseBranch: string
  count: number
  prompt: string
  agentType: string
  providerAccountId: string
  userId: string
  projectId: string
}) → { worktrees: { id: string, path: string, ptyId: string }[] }
```

### Git Operations

```typescript
'git.status'(params: { repoPath: string }) → GitStatus

'git.diff'(params: {
  repoPath: string
  staged?: boolean
  path?: string
  base?: string
}) → { diff: string }

'git.add'(params: { repoPath: string, paths: string[] }) → void

'git.commit'(params: {
  repoPath: string
  message: string
  userId: string    // for audit log
}) → { commitHash: string }

'git.push'(params: {
  repoPath: string
  branch: string
  remote?: string
  force?: boolean
}) → { success: boolean }
  // streams git push progress via event.stream

'git.log'(params: {
  repoPath: string
  limit?: number
  branch?: string
}) → { commits: CommitInfo[] }

'git.generateCommitMessage'(params: {
  repoPath: string
  staged: boolean
  providerAccountId: string
  model?: string
}) → { message: string }

'github.pr.create'(params: {
  repoPath: string
  title: string
  body: string
  base: string
  head: string
  draft?: boolean
  userId: string    // for per-user GH_CONFIG_DIR
}) → { prUrl: string, prNumber: number }
```

### File System Operations

```typescript
'fs.readDir'(params: {
  path: string
  projectRoot: string   // safety: path must be under projectRoot
  recursive?: boolean
}) → { entries: FsEntry[] }

'fs.readFile'(params: { path: string, projectRoot: string }) → {
  content: string
  encoding: string
}

'fs.writeFile'(params: {
  path: string
  projectRoot: string
  content: string
}) → void

'fs.watch'(params: {
  watchId: string
  paths: string[]
  projectRoot: string
}) → void
  // streams fs changes via event.stream (watchId)

'fs.search'(params: {
  query: string
  projectRoot: string
  includeGlobs?: string[]
  excludeGlobs?: string[]
  maxResults?: number
}) → { matches: SearchMatch[] }
```

### AI Provider — Credential Management

```typescript
'aiProvider.writeCredential'(params: {
  accountId: string
  encryptedPayload: string    // AES-256-GCM encrypted by browser SubtleCrypto
  encryptionMeta: {
    iv: string
    authTag: string
    keyDerivationSalt: string
  }
}) → { success: boolean }

'aiProvider.deleteCredential'(params: { accountId: string }) → void

'aiProvider.testConnection'(params: {
  accountId: string
  provider: string
  model: string
}) → {
  success: boolean
  latencyMs: number
  error?: string
}

'aiProvider.listConfigured'(params: {}) → {
  accounts: { accountId: string, provider: string, configuredAt: number }[]
}
```

### Workflow Step Execution

```typescript
'step.execute'(params: {
  executionId: string      // workflow execution ID from gateway
  stepId: string
  stepType: 'agent' | 'shell' | 'action'
  definition: StepDefinition
  inputs: Record<string, string>   // resolved variables
  userId: string
  projectId: string
  providerAccountId?: string
}) → { stepRunId: string }
  // streams step output via event.stream (stepRunId)

'step.cancel'(params: { stepRunId: string }) → void
```

### Health & Diagnostics

```typescript
'health.get'(params: {}) → AgentHealthSnapshot

'health.diagnose'(params: {}) → {
  checks: {
    name: string
    status: 'ok' | 'warn' | 'error'
    message?: string
    valueMs?: number
  }[]
}
```

---

## Event Stream — Agent → Gateway

```typescript
// PTY output (streaming terminal data to UI)
{
  type: 'event.stream'
  streamId: string      // ptyId hoặc stepRunId
  data: string          // base64-encoded terminal data
  seq: number           // sequence number for ordering
}

// AI Agent status change
{
  type: 'event.agentStatus'
  agentSessionId: string
  status: 'idle' | 'running' | 'waiting_input' | 'completed' | 'error'
  exitCode?: number
  usage?: { inputTokens: number, outputTokens: number }
  timestamp: number
}

// Git status change (debounced 500ms)
{
  type: 'event.gitChange'
  repoPath: string
  projectId: string
  status: GitStatus
  timestamp: number
}

// File system change (from fs.watch)
{
  type: 'event.fsChange'
  watchId: string
  changes: FsChange[]
  timestamp: number
}

// Health metrics (every 60s)
{
  type: 'event.health'
  agentId: string
  metrics: {
    cpuPercent: number
    memoryUsedMb: number
    memoryTotalMb: number
    diskUsedGb: number
    diskTotalGb: number
    networkLatencyMs: number
    activePtys: number
    activeAgentSessions: number
    activeWorktrees: number
    uptime: number
  }
  timestamp: number
}

// Step execution output
{
  type: 'event.stepOutput'
  executionId: string
  stepId: string
  stepRunId: string
  output: string        // stdout/stderr chunk
  seq: number
  timestamp: number
}

// Step completion
{
  type: 'event.stepComplete'
  executionId: string
  stepId: string
  stepRunId: string
  status: 'completed' | 'failed'
  output?: any          // structured output từ step
  error?: string
  timestamp: number
}

// Log entry
{
  type: 'event.log'
  level: 'debug' | 'info' | 'warn' | 'error'
  message: string
  context?: Record<string, any>
  timestamp: number
}
```

---

## Error Handling

```typescript
// RPC Error response
{
  type: 'rpc.error'
  id: string
  error: {
    code: number
    message: string
    data?: any
  }
}

// Error codes
const AgentErrorCodes = {
  UNAUTHORIZED: 4001,           // agent token invalid
  CAPABILITY_NOT_SUPPORTED: 4002, // agent không hỗ trợ feature này
  RESOURCE_NOT_FOUND: 4004,     // ptyId, worktreeId không tồn tại
  PROJECT_ROOT_VIOLATION: 4010,  // path traversal attempt
  CONCURRENT_LIMIT: 4029,        // too many concurrent operations
  EXECUTION_TIMEOUT: 4408,       // step execution timeout
  AGENT_BINARY_NOT_FOUND: 5001, // AI agent không install
  CREDENTIAL_NOT_FOUND: 5002,   // AI provider account not configured
  GIT_ERROR: 5003,              // git command failed
  PTY_ERROR: 5004,              // PTY creation failed
}
```

---

## Connection Resilience

### Agent-side reconnect

```
Agent disconnects (network loss, server restart)
  │
  ├─ Buffer pending events in memory (max 1000 events, 10MB)
  ├─ Local operations continue normally (PTY, git, agent still running)
  │
  └─ Reconnect attempt with exponential backoff:
       attempt 1: 5s
       attempt 2: 10s
       attempt 3: 20s
       attempt 4: 40s
       attempt 5+: 60s max
  │
  └─ On reconnect success:
       Send 'reconnect.sync' with:
         - lastEventSeq (so gateway can detect gaps)
         - currentState (active ptys, running agents, active worktrees)
       Flush buffered events (in order by seq)
```

### Gateway-side timeout

```
No heartbeat from agent for > 90s → mark agent OFFLINE
  → Notify team lead / admin (F11 notification)
  → Fleet health monitor reflects offline status (F27)
  → Pending RPC calls → timeout error to client
  → Workflow executions on this agent → paused (F36 resumability)
```

---

## Security Model

| Concern | Implementation |
|---------|---------------|
| **Agent authentication** | HMAC-signed handshake với shared secret (registered per-agent) |
| **Transport encryption** | TLS 1.3 (wss://) mandatory |
| **RPC authorization** | Gateway injects userId + permissions; Agent validates against local policy |
| **Path traversal** | Agent validates all FS paths are under `projectRoot` |
| **Shell injection** | All shell commands via `execFile()`, no string interpolation |
| **Credential isolation** | Credentials per accountId, không cross-account access |
| **Audit log** | Agent logs mọi RPC call với userId + outcome |

---

## Tham chiếu

- [CR-DS-001](./CR-DS-001-dev-server-agent-architecture.md) — Architecture overview
- [CR-DS-003](./CR-DS-003-feature-delegation-matrix.md) — Feature mapping
- HLD: [C4-code.md](../../hld/C4-code.md) — C4.11 module map
