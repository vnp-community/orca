# ADR-017 — Dev Server Agent Layer Model (A0–A4)

| Trường | Giá trị |
|--------|---------|
| **ID** | ADR-017 |
| **Trạng thái** | 🚧 Proposed |
| **Ngày** | 2026-07-30 |
| **HLD Ref** | README (Architecture Layers A0–A4), C4.11 |
| **CR Ref** | CR-DS-001, CR-DS-002, CR-DS-003 |
| **Code Ref** | `src/agent/` (new package) |
| **Feature Ref** | F01–F04, F07, F12–F14, F17–F18, F27, F29, F35–F39 |
| **Amends** | [ADR-013](../v1/ADR-013-dev-server-agent-replaces-relay.md) |

---

## Bối cảnh

ADR-013 quyết định Dev Server Agent thay thế Thin Relay, nhưng không chỉ rõ **internal layer model** của Agent. HLD v6.0 README định nghĩa rõ 5 layers (A0–A4) của Agent:

```
═══════════════════ DEV SERVER AGENT (Data Plane) ═════════════════════
┌────────────────────────────────────────────────────────────────────┐
│ A0 RPC Server   JSON-RPC 2.0 over WebSocket                        │
│                 Signed Context Verifier (HMAC-SHA256)              │
├────────────────────────────────────────────────────────────────────┤
│ A1 Operations   PTY Manager (node-pty)                             │
│                 Worktree Manager (git worktree)                    │
│                 AI Agent Spawner (ProfileAwareAgentSpawner)        │
│                 Git Engine (exec, stream)                          │
│                 File System Engine (read, write, watch, search)    │
│                 SSH Tunnel (outbound, port forwarding)             │
├────────────────────────────────────────────────────────────────────┤
│ A2 Execution    Workflow Step Executor (agent, shell, action)      │
│                 Task Agent Executor                                │
│                 Ephemeral VM Runtime (F18)                         │
│                 Automation Runner (F14 legacy)                     │
├────────────────────────────────────────────────────────────────────┤
│ A3 Storage      AI Provider Credential Store (AES-256-GCM)         │
│                 AI Vault / Session Storage (F17)                   │
│                 Local SQLite (worktrees, sessions, task runs)      │
├────────────────────────────────────────────────────────────────────┤
│ A4 Reporting    Event Stream (PTY out, agent status, git, health)  │
│                 Local Audit Log (append-only)                      │
│                 Health Metrics (CPU, RAM, disk, latency)           │
└────────────────────────────────────────────────────────────────────┘
```

---

## Quyết định

### A0 — RPC Server Layer

```typescript
// src/agent/rpc/agent-rpc-server.ts
// Responsibilities:
// 1. Accept WebSocket connections from Gateway
// 2. Parse JSON-RPC 2.0 requests
// 3. Verify signed context (HMAC-SHA256) via ContextVerifier
// 4. Route to MethodRouter
// 5. Emit responses + events back

class AgentRpcServer {
  private wss: WebSocket.Server
  private contextVerifier: ContextVerifier
  private methodRouter: MethodRouter
  private reconnectManager: ReconnectManager  // exponential backoff

  // Connection direction: Agent → Gateway (Agent-initiated)
  async connect(gatewayUrl: string, agentToken: string): Promise<void> {
    const ws = new WebSocket(`${gatewayUrl}/agent/connect`, {
      headers: { Authorization: `Bearer ${agentToken}` }
    })
    this.wss.handleUpgrade(ws, ...)
  }
}
```

**Layer A0 dependencies:** ONLY `config.ts` (no business logic)

### A1 — Operations Layer

```typescript
// src/agent/pty/pty-manager.ts        — create/resize/write/kill PTY processes
// src/agent/pty/pty-session-store.ts  — per-userId PTY registry
// src/agent/pty/pty-output-streamer.ts— chunk PTY output → event.stream
// src/agent/pty/pty-state-persistence.ts — SQLite save/restore
//
// src/agent/worktree/worktree-engine.ts  — git worktree add/remove/list
// src/agent/worktree/worktree-fanout.ts  — N worktrees + spawn agents
//
// src/agent/agent-spawn/profile-aware-agent-spawner.ts
// src/agent/agent-spawn/agent-env-builder.ts
// src/agent/agent-spawn/agent-model-validator.ts  — approvedModels whitelist
// src/agent/agent-spawn/agent-state-detector.ts   — OSC: idle/running/waiting
// src/agent/agent-spawn/agent-usage-tracker.ts
//
// src/agent/git/git-engine.ts         — status, diff, add, commit, push (exec)
// src/agent/git/git-stream.ts         — stream git output → events
// src/agent/git/git-author-injector.ts— ctx.userEmail as git author
// src/agent/git/git-user-isolation.ts — per-userId GH_CONFIG_DIR
// src/agent/git/git-pr-creator.ts     — gh CLI: PR creation
//
// src/agent/fs/fs-engine.ts           — readDir, readFile, stat, glob
// src/agent/fs/fs-searcher.ts         — ripgrep integration
// src/agent/fs/fs-watcher.ts          — chokidar file watch → events
```

**Layer A1 rule:** Không import từ A2 hoặc A3 (one-way dependency down)

### A2 — Execution Layer

```typescript
// src/agent/execution/workflow-step-executor.ts
//   → Execute individual workflow steps:
//     agent step: call A1.AgentSpawner
//     shell step: spawn process, stream stdout
//     action step: execute registered action
//     webhook step: HTTP POST to webhook URL
//
// src/agent/execution/task-agent-executor.ts
//   → Build context preamble + spawn agent via A1
//
// src/agent/execution/ephemeral-vm-runtime.ts  [F18]
//   → Create/destroy ephemeral VM, port forward
//
// src/agent/execution/automation-runner.ts  [F14 legacy]
//   → Legacy automation trigger receiver
```

**Layer A2 rule:** May import A1 operations (bottom-up OK, lateral OK, A3 for creds only)

### A3 — Storage Layer

```typescript
// src/agent/storage/ai-credential-store.ts
//   → AES-256-GCM encrypted storage for AI API keys
//   → File: ~/.orca/ai-providers/<accountId>.enc
//   → Key: scrypt(ORCA_AI_CREDENTIAL_KEY + accountId)
//
// src/agent/storage/ai-vault.ts  [F17]
//   → Per-userId AI memory vault (SQLite + AES-256-GCM)
//
// src/agent/storage/local-db.ts
//   → SQLite: worktrees, PTY sessions, task run history
//   → Uses better-sqlite3 (sync, same thread — agent is single-process)
```

**Layer A3 rule:** No imports from A1/A2 (pure storage, no business logic)

### A4 — Reporting Layer

```typescript
// src/agent/reporting/event-emitter.ts
//   → Collect events from A0–A2 → batch → send to Gateway
//   → Buffer 1000 events when Gateway offline → replay on reconnect
//
// src/agent/reporting/local-audit-log.ts
//   → Append-only SQLite log: agent.spawn, git.push, credential.read
//   → Never update/delete
//
// src/agent/reporting/health-reporter.ts
//   → Collect: cpu% (os.cpus), ram%, disk%, network latency
//   → Push to Gateway every 30s OR on-demand via health.metrics RPC
```

---

## Rationale — Tại sao Layer Model quan trọng?

| Vấn đề | Giải pháp |
|--------|-----------|
| A0 phụ thuộc vào business logic → khó test | Layer A0 chỉ parse/route, không có logic |
| Storage scattered → credential leak | Layer A3 tập trung, auditable |
| Event duplication → duplicate processing | Layer A4 duy nhất phát events |
| Offline operation unclear | A1/A2/A3 độc lập với A0 → hoạt động local khi Gateway down |

---

## Agent Package Structure (C4.11)

```
src/agent/
├── index.ts               # Agent entry point
├── config.ts              # Config loader (config.yaml + env)
├── rpc/                   # A0: RPC Server
│   ├── agent-rpc-server.ts
│   ├── context-verifier.ts
│   ├── method-router.ts
│   ├── event-emitter.ts
│   └── reconnect-manager.ts
├── pty/                   # A1: PTY operations
├── worktree/              # A1: Worktree operations
├── agent-spawn/           # A1: AI Agent spawning
├── git/                   # A1: Git operations
├── fs/                    # A1: File system operations
├── execution/             # A2: Step execution
├── storage/               # A3: Persistent storage
└── reporting/             # A4: Events + metrics
```

---

## Hậu quả

**Tích cực:**
- Rõ ràng về dependency direction → easy to test each layer
- A3 (Storage) isolated → credential security auditable
- A4 (Reporting) centralized → no duplicate events
- A1 (Operations) pluggable → thêm operation không ảnh hưởng A0/A4

**Tiêu cực:**
- Build `src/agent/` from scratch (~40 files)
- Layer boundary phải được enforce qua ESLint import rules
- Single binary output (~80MB) — ncc bundle

---

## Trạng thái Implementation

❌ `src/agent/` package chưa tồn tại  
🎯 Phase 1: A0 (RpcServer + ContextVerifier) + A1 (PTY + Git)  
🎯 Phase 2: A2 (Step Executor) + A3 (CredentialStore)  
🎯 Phase 3: A4 (EventEmitter + HealthReporter) + build pipeline

---

## Cross-References

| Resource | Mô tả |
|---|---|
| [ADR-013](../v1/ADR-013-dev-server-agent-replaces-relay.md) | Quyết định thay relay bằng Agent |
| [ADR-014](../v1/ADR-014-gateway-agent-json-rpc-protocol.md) | JSON-RPC protocol (A0 layer) |
| [ADR-015](../v1/ADR-015-signed-execution-context-gateway-agent.md) | HMAC signed context (A0 layer) |
| [ADR-018](./ADR-018-control-plane-data-plane-separation.md) | Control Plane vs Data Plane |
| [flows/agent-connection-modes.md](../../flows/agent-connection-modes.md) | A0 connection modes |
| **HLD README v6.0** | Architecture Layers A0–A4 diagram |
| **HLD C4.11** | `src/agent/` module map chi tiết |
