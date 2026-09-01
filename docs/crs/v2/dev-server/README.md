# CR v2 — Dev Server Agent Change Requests

**Phiên bản:** v6.0  
**Ngày:** 2026-07-30  
**Trạng thái:** Proposed  
**Kiến trúc liên quan:** [HLD README](../../hld/README.md), [C2 Containers](../../hld/C2-containers.md), [C3 Components](../../hld/C3-components.md), [C4 Code](../../hld/C4-code.md)

---

## Tóm tắt thay đổi kiến trúc v6.0

### Trước (v5.x): Thin Relay Model

```
Orca Backend Server                     Dev Server (Remote Host)
[ALL Business Logic]         SSH/WS     [Thin Relay Binary]
  Worktree, Agent,          ─────────▶  PTY bridge
  Git, File Ops,                        FS ops
  AI Spawn, ...                         Git exec
                                        Port scan
```

### Sau (v6.0): Dev Server Agent Model

```
Orca Backend Server (Control Plane)     Dev Server Agent (Data Plane)
[Auth, Tenant, Policy,      wss://      [Full Orca Backend — headless]
 Project, AI Registry,     ─────────▶  PTY Manager
 Workflow Dispatch,                     AI Agent Spawner
 Task Management,                       Worktree Engine
 Fleet Monitoring]                      Git Engine
                                        File System Engine
                                        AI Provider Cred Store
                                        Workflow Step Executor
                                        Health Reporter
```

**Nguyên tắc cốt lõi:**
- **Gateway = Control Plane**: Ai được làm gì, ở đâu, với tài nguyên nào
- **Agent = Data Plane**: Thực sự làm công việc trên dev server
- **Agent autonomous**: Hoạt động khi mất kết nối về Gateway
- **Signed Context**: HMAC-SHA256 ensures Gateway identity + prevents replay

---

## Change Requests

| CR ID | Tên | Priority | Phụ thuộc |
|-------|-----|---------|-----------|
| [CR-DS-001](./CR-DS-001-dev-server-agent-architecture.md) | Dev Server Agent — Full Backend on Dev Server | P0 | — |
| [CR-DS-002](./CR-DS-002-gateway-agent-rpc-protocol.md) | Gateway–Agent RPC Protocol v3 | P0 | CR-DS-001 |
| [CR-DS-003](./CR-DS-003-feature-delegation-matrix.md) | Feature Delegation Matrix | P0 | CR-DS-001, CR-DS-002 |
| [CR-DS-004](./CR-DS-004-agent-lifecycle-management.md) | Agent Lifecycle & Deployment | P0 | CR-DS-001 |
| [CR-DS-005](./CR-DS-005-agent-session-context-propagation.md) | Session Context Propagation | P0 | CR-DS-001, CR-DS-002 |
| [CR-DS-006](./CR-DS-006-dev-server-approval-and-grouping.md) | Dev Server Agent Approval & Grouping | P1 | CR-DS-001/002, CR-AG-004 — ✅ Hoàn tất (backend + frontend) |
| [CR-DS-007](./CR-DS-007-department-based-access-control.md) | Department-Based Dev Server Access Control | P1 | CR-DS-006 — ✅ Hoàn tất (backend + frontend) |
| [CR-DS-008](./CR-DS-008-first-login-department-gate-and-access-request.md) | First-Login Department Gate & Access Request Flow | P2 | CR-DS-006, CR-DS-007 — ✅ Hoàn tất (backend + frontend) |

---

## HLD Documents Updated

| File | Thay đổi |
|------|---------|
| [README.md](../../hld/README.md) | v6.0 architecture diagram, layer model (Control/Data plane split), tech stack |
| [C3-components.md](../../hld/C3-components.md) | Added **C3.13 Dev Server Agent Components** (12 components) + updated Feature→Component map |
| [C4-code.md](../../hld/C4-code.md) | Added **C4.11 Dev Server Agent Module Map** (`src/agent/` structure, 3 key data flows) |
| [security.md](../../hld/security.md) | Added **Trust Boundary 5** (Gateway↔Agent) + v6.0 security checklist (12 items) |
| C2-containers.md | **Pending** — Replace Orca Relay container with Dev Server Agent container |
| deployment.md | **Pending** — Add agent deployment section (systemd, Docker, launchd) |

---

## Implementation Phases

### Phase 1 — Core Agent (v6.0a)
- [ ] `src/agent/` project setup (TypeScript, build config)
- [ ] AgentRpcServer + ContextVerifier + ReconnectManager
- [ ] PtyManager (agent-side)
- [ ] AgentDb (local SQLite)
- [ ] EventBus

### Phase 2 — Operations (v6.0b)
- [ ] ProfileAwareAgentSpawner + AgentEnvBuilder
- [ ] WorktreeEngine + GitEngine
- [ ] FsEngine + SecureFs
- [ ] AiCredentialStore

### Phase 3 — Gateway Integration (v6.0c)
- [ ] AgentConnectionManager (Gateway side)
- [ ] SignedContextIssuer
- [ ] AgentDispatcher
- [ ] Update WorkflowOrchestrator → use AgentDispatcher
- [ ] Update TaskAgentExecutor → use AgentDispatcher

### Phase 4 — Lifecycle & Ops (v6.0d)
- [ ] HealthReporter + DiagnosticServer
- [ ] StepExecutor (agent, shell, action types)
- [ ] Install script (systemd + Docker + launchd)
- [ ] Agent update mechanism (admin-triggered)
- [ ] Agent token management in Admin Panel

---

## Env Variables (v6.0)

### Gateway (Orca Backend Server)

| Variable | Mô tả |
|---------|-------|
| `ORCA_GATEWAY_SECRET` | Master secret for signing agent contexts |
| `ORCA_AGENT_REGISTRY_DB` | Where agent registrations are stored |

### Dev Server Agent

| Variable | Mô tả |
|---------|-------|
| `ORCA_BACKEND_URL` | Gateway WebSocket URL (wss://...) |
| `ORCA_AGENT_SECRET` | Agent authentication secret (from registration) |
| `ORCA_AI_CREDENTIAL_KEY` | Master key for AES-256-GCM credential encryption |
| `ORCA_AGENT_DATA_DIR` | Agent data directory (default: /var/lib/orca-agent) |
| `ORCA_AGENT_LOG_LEVEL` | Log level (debug/info/warn/error) |
