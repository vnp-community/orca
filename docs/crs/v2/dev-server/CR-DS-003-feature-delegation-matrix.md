# CR-DS-003 — Feature Delegation Matrix: Gateway vs Dev Server Agent

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-DS-003 |
| **Tên** | Feature Delegation Matrix — Gateway vs Agent |
| **Loại** | Architecture Decision |
| **Priority** | P0 — Critical |
| **Phiên bản** | v6.0 |
| **Ngày tạo** | 2026-07-30 |
| **Trạng thái** | Proposed |
| **Phụ thuộc** | CR-DS-001, CR-DS-002 |

---

## Nguyên tắc phân chia (Delegation Principles)

| Nguyên tắc | Mô tả |
|-----------|-------|
| **Control vs Data** | Backend quản lý **who can do what** (control); Agent thực hiện **actual work** (data) |
| **User isolation** | Backend enforce user isolation; Agent tin tưởng userId từ Gateway |
| **Stateless Gateway** | Gateway không giữ operational state (no PTY, no git state, no file state) |
| **Stateful Agent** | Agent giữ toàn bộ operational state (PTY sessions, worktrees, git, files) |
| **Single Source of Truth** | Policy & registry tại Gateway; Execution state tại Agent |

---

## Feature Delegation Matrix

### Ghi chú ký hiệu

| Symbol | Ý nghĩa |
|--------|---------|
| ✅ GW | Chạy hoàn toàn trên Orca Backend Server (Gateway) |
| ✅ AG | Chạy hoàn toàn trên Dev Server Agent |
| 🔀 SPLIT | Logic tách biệt: một phần GW, một phần AG |
| ↗ DISPATCH | Gateway dispatch xuống Agent để thực thi |
| 🔴 BREAKING | Breaking change so với v5.x |

---

### Group 1: Core Desktop IDE (F01–F13)

| Feature | GW Role | Agent Role | Pattern | Impact |
|---------|---------|------------|---------|--------|
| **F01 Parallel Worktrees** | Project scoping, quota check, user validation | `worktree.create`, `worktree.fanout`, git worktree ops | 🔀 SPLIT + ↗ | 🔴 |
| **F02 Terminal Splits** | WS multiplexing, session routing to correct agent | `pty.create`, `pty.resize`, `pty.write` — all PTY ops | 🔀 SPLIT | 🔴 |
| **F03 Mobile Companion** | Push token registry, notification dispatch | PTY output forwarding, agent status events | 🔀 SPLIT | 🟠 |
| **F04 AI Agent Support** | Provider account resolution (F35), model whitelist | `agent.spawn`, `agent.detect`, trust env inject | 🔀 SPLIT + ↗ | 🔴 |
| **F05 Design Mode** | No change | File ops on dev server | ✅ AG | 🟢 |
| **F06 GitHub/Linear Integration** | Token registry (WebCredentialStore) | PR creation (`github.pr.create`), git operations | 🔀 SPLIT | 🟠 |
| **F07 SSH Worktrees** | Fleet SSH config registry | SSH tunneling, port forwarding on agent | 🔀 SPLIT | 🟠 |
| **F08 Annotate AI Diffs** | Annotation storage (DB) | `git.diff` call to get diff | 🔀 SPLIT | 🟢 |
| **F09 Orca CLI** | Auth validation, command routing | Command execution on agent | 🔀 SPLIT | 🔴 |
| **F10 Quick Open** | No role | `fs.readDir`, `fs.search` | ✅ AG | 🟢 |
| **F11 Notifications** | Routing, aggregation, push to mobile | Emit `event.agentStatus`, `event.gitChange` | 🔀 SPLIT | 🟡 |
| **F12 File Explorer & Editor** | No role (proxy) | `fs.readDir`, `fs.readFile`, `fs.writeFile`, `fs.watch` | ✅ AG | 🟡 |
| **F13 Text Search** | No role (proxy) | `fs.search` (ripgrep on agent) | ✅ AG | 🟡 |

### Group 2: Advanced Desktop Features (F14–F21)

| Feature | GW Role | Agent Role | Pattern | Impact |
|---------|---------|------------|---------|--------|
| **F14 Automations** | Template registry, trigger scheduling | Step execution (`step.execute`), cron runner local | 🔀 SPLIT → superseded by F36 | 🔴 |
| **F15 Computer Use** | Desktop-only (Electron); N/A in server mode | N/A | ✅ GW (desktop) | 🟠 |
| **F16 Rich Repo Previews** | No role | `git.log`, metadata fetch | ✅ AG | 🟢 |
| **F17 Memory / AI Vault** | No role (proxy) | Session storage (local SQLite on agent) | ✅ AG | 🟠 |
| **F18 Ephemeral VM** | Scheduling, quota | VM lifecycle (docker/ssh), recipe exec | 🔀 SPLIT + ↗ | 🟠 |
| **F19 Localization** | Profile locale setting | UI locale (client-side) | ✅ GW | 🟢 |
| **F20 Speech Input** | No role | Client-side only | N/A | ⚪ |
| **F21 Auto Update** | Docker image update (server mode) | Agent self-update (systemd, launchd) | ✅ GW (server) / ✅ AG (agent) | 🔴 |

### Group 3: Web Server Mode & Enterprise (F22–F32)

| Feature | GW Role | Agent Role | Pattern | Impact |
|---------|---------|------------|---------|--------|
| **F22 Web Server Mode** | HTTP/WS server, SPA serving | No role | ✅ GW | ✅ Foundation |
| **F23 Multi-User Auth** | Login, session, bcrypt | No role (trusts JWT from GW) | ✅ GW | ✅ Foundation |
| **F24 Per-User Sandbox** | Session routing (WsSessionRouter) | Per-userId context on all RPC calls | 🔀 SPLIT | 🟡 |
| **F25 Admin Panel** | Company/Team/User CRUD, audit log | `health.get` for fleet status display | 🔀 SPLIT | 🟡 |
| **F26 Multi-Database** | IConnectionPool for all GW tables | Local SQLite on agent | ✅ GW + ✅ AG (separate) | 🟢 |
| **F27 Fleet Health Monitoring** | Aggregate health metrics from agents | `event.health` emission every 60s | 🔀 SPLIT | 🟡 |
| **F28 Dev Server Onboarding** | Agent registration, token issuing | Agent install & bootstrap | 🔀 SPLIT | 🟠 |
| **F29 Agent WebSocket Protocol** | WS multiplexer (UI ↔ Agent) | Event streaming to GW | 🔀 SPLIT | 🟡 |
| **F30 Remote Integrations** | Token storage (WebCredentialStore) | API calls (gh CLI with per-user config) | 🔀 SPLIT | 🟠 |
| **F31 Fleet Provisioning** | Provisioning wizard, RBAC | Agent auto-config on first boot | 🔀 SPLIT | 🟡 |
| **F32 Team RBAC** | Policy enforcement, audit log | Trust userId from GW context | ✅ GW (enforcement) | ✅ Foundation |

### Group 4: v5.0 Enterprise (F33–F39)

| Feature | GW Role | Agent Role | Pattern | Impact |
|---------|---------|------------|---------|--------|
| **F33 Profile Hierarchy** | CompanyService, DeptService, ProfileResolver | Receive resolved profile in RPC context | ✅ GW → inject | ✅ Purpose-built |
| **F34 Project-Dev Server Binding** | Project registry, devServerId mapping | Accept project context in all ops | ✅ GW → context | ✅ Purpose-built |
| **F35 AI Provider Account Mgmt** | Account registry, metadata, quota tracking | Credential write/read, health check, agent env inject | 🔀 SPLIT | ✅ Purpose-built |
| **F36 Workflow Orchestration** | DAG build, template registry, step dispatch | `step.execute` (agent, shell), `event.stepComplete` | 🔀 SPLIT + ↗ | ✅ Purpose-built |
| **F37 Task Graph Management** | Task CRUD, grant system, AI planning | `agent.spawn` with task preamble, execution tracking | 🔀 SPLIT + ↗ | ✅ Purpose-built |
| **F38 Project Workspace** | WorkspaceContext orchestration | All workspace data (files, git, worktrees) | 🔀 SPLIT | ✅ Purpose-built |
| **F39 Remote Git UI** | No role (proxy) | All git operations via RPC | ✅ AG | ✅ Purpose-built |

---

## Data Flow Patterns

### Pattern A: Pure Gateway (no agent)
```
UI → Gateway RPC → Gateway Service → DB → Response
Ví dụ: User login, profile update, team management
```

### Pattern B: Pure Agent (gateway proxies)
```
UI → Gateway WS → Agent Connection Manager → Agent RPC → Agent Response → UI
Ví dụ: fs.readFile, git.status, fs.search
```

### Pattern C: Split — Gateway validates → Agent executes
```
UI → Gateway
  1. Gateway validates: auth, RBAC, project membership
  2. Gateway enriches: inject resolvedProfile, providerAccountId
  3. Gateway dispatches: agent.rpc.call(agentId, method, enrichedParams)
  4. Agent executes với enriched context
  5. Agent returns result → Gateway → UI
Ví dụ: worktree.create, agent.spawn, git.commit
```

### Pattern D: Dispatch + Stream
```
UI triggers workflow step → Gateway dispatches → Agent executes
  Agent streams events back via event.stream
  Gateway fans events out to subscribed UI clients
Ví dụ: Workflow execution (F36), Task agent execution (F37)
```

---

## Context Propagation (Gateway → Agent)

Mỗi RPC call từ Gateway xuống Agent phải carry:

```typescript
interface RpcExecutionContext {
  // Identity
  userId: string
  userRole: 'admin' | 'lead' | 'developer'
  userEmail: string                      // for git author

  // Scope
  projectId: string
  projectRoot: string                    // absolute path on this dev server
  teamId?: string

  // Resolved settings (from F33)
  resolvedProfile: {
    agent: { trustPreset, approvedModels, maxTokensPerSession }
    shell: { env, pathAdditions, startupCommands }
    integrations: { githubOrg, prTemplate }
    fleet: { sshKeyPath, defaultConnectionType }
  }

  // AI Provider (from F35)
  providerAccountId?: string             // resolved by Gateway

  // Audit
  requestId: string                      // traceability
  sessionId: string                      // gateway session ID
}
```

---

## Backward Compatibility

| Concern | Strategy |
|---------|---------|
| Thin relay (v4/v5) | Thin relay vẫn work với v5 features; v6 requires Agent |
| Protocol version | Handshake negotiates version; Agent reports max supported |
| Feature flags | Gateway sends `featureFlags` in handshake config |
| Graceful degradation | If agent capability missing → Gateway returns 4002, shows warning in UI |
