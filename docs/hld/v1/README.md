# High Level Design — Orca AI Orchestrator IDE

**Mô hình:** C4 Architecture (Simon Brown)  
**Phiên bản:** 3.0 — 2026-07-30 (v6.0: Dev Server Agent architecture — full backend on dev servers)  
**Tham chiếu:** SRS.md, features/, logic/, crs/v2/dev-server/

> **v6.0 Architectural Shift:** Thin Relay binary (v4/v5) được thay thế bằng **Dev Server Agent** — một headless Orca backend đầy đủ chạy trên mỗi dev server. Orca Backend Server trở thành **Control Plane thuần túy** (auth, tenant, policy, dispatch). Mọi data-plane operations (PTY, git, agent, FS) thực thi trực tiếp trên Agent.

---

## Tổng quan C4 Model

C4 Model mô tả kiến trúc phần mềm qua 4 mức độ trừu tượng:

| Level | File | Mô tả |
|-------|------|-------|
| L1 | [C1-system-context.md](./C1-system-context.md) | Hệ thống trong môi trườ́ng — users, external systems |
| L2 | [C2-containers.md](./C2-containers.md) | 16 containers: desktop, web server, mobile, relay, daemon, CLI, Profile/Project, AI Provider, Workflow, Task Graph |
| L3 | [C3-components.md](./C3-components.md) | Components: Main Process, Daemon, Relay, UI, Fleet, Platform, DB, AgentWS, Credentials, Profile+Project, AI Provider, Workflow, Task Graph |
| L4 | [C4-code.md](./C4-code.md) | Module map chi tiết + data flows cho tất cả subsystems (C4.1 – C4.9) |
| — | [deployment.md](./deployment.md) | Deployment trên các platform (macOS, Windows, Linux, Docker) |
| — | [security.md](./security.md) | Security model, auth, E2E encryption, multi-user trust |

---

## Sơ đồ tóm tắt

```
┌────────────────────────────────────────────────────────────────┐
│              EXTERNAL USERS & SYSTEMS                           │
│  Developer  Tech Lead  Remote Dev  Mobile User  Admin  AgentDev │
└────────────────────────────────────────────────────────────────┘
               │                    │                  │
    ┌─────────┴────────┐     ┌────┴────┐  ┌───┴──────┐
    │  Orca Desktop  │     │ Orca Web │  │ Orca CLI  │
    │  (Electron)    │     │ Server   │  │ (Node.js) │
    └──────┬───────┘     └────┬────┘  └───┬──────┘
           │                   │              │
           └────────┬───────┘              │
                    │ IPlatformServices          │
            ┌──────┴─────────────────────┘
            │  Orca Main Process / Node Adapter │
            │  ┌──────────────────────────┐ │
            │  │ Auth + Session Manager        │ │
            │  │ Admin Panel + Audit Log       │ │
            │  ├──────────────────────────┤ │
            │  │ DB Layer (IDatabase)          │ │
            │  │ SQLite | MySQL | PgSQL | TiDB │ │
            │  ├──────────────────────────┤ │
            │  │ Fleet Health Monitor          │ │
     ## Architecture Layers (v6.0)

Orca v6.0 phân tầng rõ ràng theo **Control Plane** (Gateway) và **Data Plane** (Agent):

```
═══════════════════ ORCA BACKEND SERVER (Control Plane) ═══════════════
┌────────────────────────────────────────────────────────────────────┐
│ L0 UI Layer    React SPA (Renderer + Admin SPA + Task/Workflow UI) │
├────────────────────────────────────────────────────────────────────┤
│ L1 Platform    IPlatformServices: NodeAdapter (server)             │
│                ElectronAdapter (desktop only)                      │
├────────────────────────────────────────────────────────────────────┤
│ L2 Auth        AuthManager, SessionManager, bcrypt 12r             │
│                HTTP-only session cookie, SSO stub                  │
├────────────────────────────────────────────────────────────────────┤
│ L3 Control     Tenant/Team/User/Profile (F32, F33)                 │
│                Project Registry (F34)                              │
│                AI Provider Registry — metadata only (F35)          │
│                Workflow DAG Builder + Dispatcher (F36)             │
│                Task Graph + Grant System (F37)                     │
│                Fleet Monitor + Provisioning (F27, F31)             │
│                Agent Connection Manager                            │
│                Signed Context Issuer (CR-DS-005)                   │
├────────────────────────────────────────────────────────────────────┤
│ L4 Repository  IStateRepository: SqlRepository                     │
├────────────────────────────────────────────────────────────────────┤
│ L5 Database    IDatabase + IConnectionPool                         │
│                SQLite | MySQL | PostgreSQL | TiDB                  │
│                Migrations 0001–0010                                │
└────────────────────────────────────────────────────────────────────┘

═══════════════════ DEV SERVER AGENT (Data Plane) ═════════════════════
┌────────────────────────────────────────────────────────────────────┐
│ A0 RPC Server  JSON-RPC 2.0 over WebSocket                         │
│                Signed Context Verifier (HMAC-SHA256)               │
├────────────────────────────────────────────────────────────────────┤
│ A1 Operations  PTY Manager (node-pty)                              │
│                Worktree Manager (git worktree)                     │
│                AI Agent Spawner (ProfileAwareAgentSpawner)         │
│                Git Engine (exec, stream)                           │
│                File System Engine (read, write, watch, search)     │
│                SSH Tunnel (outbound, port forwarding)              │
├────────────────────────────────────────────────────────────────────┤
│ A2 Execution   Workflow Step Executor (agent, shell, action)       │
│                Task Agent Executor                                 │
│                Ephemeral VM Runtime (F18)                          │
│                Automation Runner (F14 legacy)                      │
├────────────────────────────────────────────────────────────────────┤
│ A3 Storage     AI Provider Credential Store (AES-256-GCM)          │
│                AI Vault / Session Storage (F17)                    │
│                Local SQLite (worktrees, sessions, task runs)       │
├────────────────────────────────────────────────────────────────────┤
│ A4 Reporting   Event Stream (PTY out, agent status, git, health)   │
│                Local Audit Log (append-only)                       │
│                Health Metrics (CPU, RAM, disk, latency)            │
└────────────────────────────────────────────────────────────────────┘
```��────────────────────────────────────────────────┤
│ L1 Platform         IPlatformServices                                     │
│                     ElectronAdapter | NodeAdapter                         │
├────────────────────────────────────────────────────────────────┤
│ L2 Auth             AuthManager, SessionManager                           │
│                     bcrypt 12r, HTTP-only session cookie                  │
├────────────────────────────────────────────────────────────────┤
│ L3 Business Logic   Worktree, Agent, SSH, Git                             │
│                     Fleet, AgentWS, Integration (GitHub/GitLab)           │
│                     Profile/Project (F33-34)                              │
│                     AI Provider Accounts (F35)                            │
│                     Workflow Orchestration (F36)                          │
│                     Task Graph + AI Planning (F37)                        │
├────────────────────────────────────────────────────────────────┤
│ L4 Repository       IStateRepository                                      │
│                     JsonFileRepository | SqlRepository                    │
├────────────────────────────────────────────────────────────────┤
│ L5 Database         IDatabase + IConnectionPool                           │
│                     SQLite | MySQL | PostgreSQL | TiDB                     │
│                     Migrations 0001–0010                                  │
└────────────────────────────────────────────────────────────────┘
```

**Cross-cutting Concerns:**
- **Security**: Audit Log (append-only), RBAC policy table, Agent Token (SHA-256 hash)
- **Observability**: `/health/ready`, `/health/metrics` (Prometheus), Fleet webhooks
- **Credentials**: `WebCredentialStore` AES-256-GCM per-user per-service

---

## Công nghệ Stack (v6.0)

### Orca Backend Server (Control Plane)

| Layer | Technology |
|-------|-----------|
| Desktop Shell | Electron 32+ |
| Web Server Mode | Node.js 22+ Express HTTP :6769 + WebSocket :6768 |
| Renderer UI | React 19, TypeScript |
| Admin SPA | React SPA tại `/admin` (separate bundle) |
| Platform Abstraction | `IPlatformServices` — `NodeAdapter` / `ElectronAdapter` |
| Auth | bcrypt 12 rounds + HTTP-only cookie session (8h TTL) |
| Multi-DB | `IConnectionPool`, `MigrationRunner`, SQLite/MySQL/PostgreSQL/TiDB |
| IPC | Electron IPC (desktop) + WebSocket RPC (server mode) |
| Mobile | React Native (iOS/Android) |
| Encryption (mobile) | TweetNaCl (NaCl box) |
| **Agent Connection** | **AgentConnectionManager — persistent WS pool to all Dev Server Agents** |
| **Signed Context** | **HMAC-SHA256 signed RpcExecutionContext (30s TTL)** |
| **Profile System** | **3-layer merge (Company←Dept←User), in-memory cache TTL 60s** |
| **AI Provider Registry** | **Metadata only, no credentials, 7 providers** |
| **Workflow Engine** | **DAG builder + dispatcher → delegates steps to Agent** |
| **Task Graph** | **DAG + BFS, AI decompose, grant 5-level, dispatch to Agent** |
| Build | Vite + electron-builder + Docker |
| Testing | Vitest + Playwright (E2E) |

### Dev Server Agent (Data Plane)

| Layer | Technology |
|-------|-----------|
| Runtime | Node.js 22+ (compiled single binary via `ncc`) |
| RPC Server | JSON-RPC 2.0 over WebSocket (outbound) |
| Terminal | node-pty (PTY management) + WebGL streams |
| Git | `child_process.execFile` + native git binary |
| File System | Node.js `fs`, chokidar (watch), ripgrep (search) |
| AI Agent Spawn | node-pty + OSC state detection |
| AI Credentials | AES-256-GCM files (`~/.orca/ai-providers/*.enc`) |
| Local DB | better-sqlite3 (worktrees, sessions, task runs) |
| SSH | ssh2 library (outbound tunneling) |
| Process Manager | systemd / launchd / Docker restart policies |
| Health | Periodic metrics collection, event streaming |
| Deployment | Single binary + install.sh script / Docker image |

---

## Feature → HLD Traceability Matrix

| Feature | Priority | Status | C2 Container | C3 Component | C4 Module | ADR |
|---------|----------|--------|--------------|--------------|-----------|-----|
| F01 Parallel Worktrees | P0 | ✅ | Main Process | C3.1 | C4.1 | — |
| F02 Terminal Splits | P0 | ✅ | Main Process, Daemon | C3.1, C3.2 | C4.1 | — |
| F03 Mobile Companion | P0 | ✅ | Mobile | C3.4 | — | — |
| F04 AI Agent Support | P0 | ✅ | Main Process | C3.1, C3.8 | C4.1 | — |
| F07 SSH Worktrees | P1 | ✅ | Relay | C3.3, C3.5 | C4.4 | — |
| F09 Orca CLI | P1 | ✅ | CLI | C2 | — | — |
| F14 Automations | P2 | 🚧 | Main Process | C3.1 | C4.1 | — |
| F22 Web Server Mode | P0 | ✅ | Orca Web Server | C3.6 | C4.2 | ADR-001 |
| F23 Multi-User Auth | P0 | ✅ | Orca Web Server | C3.1 | C4.1 | ADR-003 |
| F24 Per-User Sandbox | P0 | ✅ | Orca Web Server | C3.1 | C4.1 | ADR-003 |
| F25 Admin Panel | P1 | ✅ | Orca Web Server | C3.1 | — | — |
| F26 Multi-Database | P1 | ✅ | Server DB | C3.7 | C4.3 | ADR-002 |
| F27 Fleet Health Monitoring | P1 | ✅ | Dev Servers | C3.5 | C4.4 | ADR-004 |
| F28 Dev Server Onboarding | P1 | ✅ | Dev Servers | C3.5 | C4.4 | ADR-004 |
| F29 Agent WebSocket | P1 | ✅ | Agent WS | C3.8 | C4.5 | ADR-005 |
| F30 Remote Integrations | P1 | ✅ | Orca Web Server | C3.9 | C4.6 | ADR-006 |
| F31 Fleet Provisioning | P1 | ✅ | Dev Servers | C3.5 | C4.4 | ADR-004 |
| F32 Team RBAC | P2 | 🚧 | Orca Web Server | C3.1 | — | — |
| **F33 Profile Hierarchy** | **P0** | **🚧** | **Profile/Project Svc** | **C3.10** | **C4.7** | **ADR-007** |
| **F34 Project Binding** | **P0** | **🚧** | **Profile/Project Svc** | **C3.10** | **C4.8** | **ADR-007, ADR-011** |
| **F35 AI Provider Mgmt** | **P0** | **🚧** | **AI Provider Svc** | **C3.11** | **C4.9** | **ADR-008** |
| **F36 Workflow Orchestration** | **P1** | **🚧** | **Workflow Orchestrator** | **C3.11b** | **C4.9** | **ADR-009** |
| **F37 Task Graph** | **P0** | **🚧** | **Task Graph Svc** | **C3.11c** | **C4.9** | **ADR-010** |
| **F38 Project Workspace** | **P0** | **🚧** | **Profile/Project Svc** | **C3.12** | **C4.10** | **ADR-011** |
| **F39 Remote Git UI** | **P0** | **🚧** | **Dev Servers (relay)** | **C3.12** | **C4.10** | **ADR-012** |

---

## DB Migration → Feature Mapping

| Migration | Tables tạo | Features |
|-----------|-----------|---------|
| 0001 initial | orca_worktrees, orca_sessions | F01, F02 |
| 0002 automations | orca_automations | F14 |
| 0003 sessions | orca_workspace_sessions | F22, F23 |
| 0004 app_tables | orca_dev_servers, orca_users... | F23, F24, F25, F27 |
| 0005 auth | orca_audit_log, bcrypt auth | F23, F25 |
| **0006** (v5.0) | orca_company, orca_departments | **F33** |
| **0007** (v5.0) | orca_projects, orca_project_members | **F34** |
| **0008** (v5.0) | orca_ai_provider_accounts, orca_provider_usage | **F35** |
| **0009** (v5.0) | orca_workflow_templates, orca_workflow_executions, orca_step_executions | **F36** |
| **0010** (v5.0) | orca_tasks, orca_task_edges, orca_task_grants, orca_task_comments | **F37** |
