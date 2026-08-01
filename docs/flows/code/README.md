# Orca Flows — Code & Mechanism Documentation

Tài liệu mô tả các **luồng xử lý** và **cơ chế hoạt động** của hệ thống Orca — được tổ chức theo **nhóm nghiệp vụ** tương ứng với `docs/logic/`.  
**Cập nhật**: 2026-08-01 (tổ chức lại theo nhóm nghiệp vụ domain)

---

## Cấu trúc thư mục

```
flows/code/
├── auth/                    # Xác thực & Quản lý User (BL-AUTH-*)
├── remote-development/      # Phát triển Từ xa qua SSH (BL-SSH-*)
├── agent-ws/                # Agent WebSocket Protocol (BL-AWS-*)
├── agent-orchestration/     # Điều phối AI Agent (BL-AG-*)
├── terminal-management/     # Quản lý Terminal PTY (BL-TM-*)
├── profile/                 # User Profile Hierarchy (BL-PRF-*)
├── ai-providers/            # AI Provider Management (BL-AIP-*)
├── project-workspace/       # Project Workspace & Remote Git (BL-PW-*)
├── task-graph/              # Task Graph Management (BL-TG-*)
├── workflow-orchestration/  # Workflow Orchestration (BL-WF-*)
├── project-integration/     # Tích hợp GitHub/GitLab (BL-PI-*)
├── cli-headless/            # CLI & Headless Mode (BL-CLI-*)
└── fleet/                   # Fleet Management (BL-FLEET-*)
```

---

## Danh sách Flows theo Nhóm Nghiệp vụ

### 1. Auth — Xác thực & Quản lý User

Tương ứng với nhóm [BL-AUTH](../../logic/auth/) trong `docs/logic/`.

| File | Mô tả | Trạng thái |
|------|-------|------------|
| [auth/authentication.md](./auth/authentication.md) | E2EE auth (Curve25519 ECDH + XChaCha20-Poly1305) + v5.0 HTTP session auth | Core Flow |
| [auth/account-management.md](./auth/account-management.md) | Device registry, DeviceEntry, ScopedPairingToken, revocation | Core Flow |
| [auth/session-management.md](./auth/session-management.md) | WebSocket connection lifecycle, Tab state, Client isolation | Core Flow |
| [auth/multi-user-session.md](./auth/multi-user-session.md) | Web Server Mode: bcrypt login, WsSessionRouter per-user fork, per-user sandbox | Enterprise Flow |

---

### 2. Remote Development — Phát triển Từ xa

Tương ứng với nhóm [BL-SSH](../../logic/remote-development/) trong `docs/logic/`.

| File | Mô tả | Trạng thái |
|------|-------|------------|
| [remote-development/web-pairing-connection.md](./remote-development/web-pairing-connection.md) | Kết nối web client từ PairCode → E2EE WebSocket → app running | Core Flow |
| [remote-development/ssh-relay-connection.md](./remote-development/ssh-relay-connection.md) | Orca Server SSH vào Dev Machine, deploy relay binary, multiplexed JSON-RPC | Core Flow |
| [remote-development/dev-server-connection-types.md](./remote-development/dev-server-connection-types.md) | 3 loại kết nối Orca ↔ Dev Server: relay-ssh, WebSocket, Unix socket | Core Flow |
| [remote-development/relay-management.md](./remote-development/relay-management.md) | SSH relay lifecycle, deploy, reconnect + v5 RelayConnectionPool + v6 HMAC context | Core Flow |
| [remote-development/remote-servers.md](./remote-development/remote-servers.md) | Remote Servers — pairing, WebSocket RPC, Electron vs web mode | ✅ VERIFIED |

---

### 3. Agent WebSocket — Kết nối Agent qua WebSocket

Tương ứng với nhóm [BL-AWS](../../logic/agent-ws/) trong `docs/logic/`.

| File | Mô tả | Trạng thái |
|------|-------|------------|
| [agent-ws/agent-connection-modes.md](./agent-ws/agent-connection-modes.md) | 2 agent modes (direct-ws/relay-ws), handshake + v6 HMAC context + per-userId isolation | Core Flow |

---

### 4. Agent Orchestration — Điều phối AI Agent

Tương ứng với nhóm [BL-AG](../../logic/agent-orchestration/) trong `docs/logic/`.

| File | Mô tả | Trạng thái |
|------|-------|------------|
| [agent-orchestration/runtime.md](./agent-orchestration/runtime.md) | OrcaRuntimeService: PTY, worktrees, git, agents + v5 multi-DB + v6 Agent RPC | Core Flow |
| [agent-orchestration/agent_comparison.md](./agent-orchestration/agent_comparison.md) | So sánh các agent modes và kiến trúc kết nối | Reference |
| [agent-orchestration/orca_architecture_proposals.md](./agent-orchestration/orca_architecture_proposals.md) | Kiến trúc tổng quan Orca và các đề xuất thiết kế | Reference |

---

### 5. Terminal Management — Quản lý Terminal PTY

Tương ứng với nhóm [BL-TM](../../logic/terminal-management/) trong `docs/logic/`.

| File | Mô tả | Trạng thái |
|------|-------|------------|
| [terminal-management/terminal-create-flow.md](./terminal-management/terminal-create-flow.md) | terminal.create: Browser → WsSessionRouter → UserProcess → OrcaRuntime → DevServerRelayBridge → relay PTY | Core Flow |

---

### 6. Profile — User Profile Hierarchy

Tương ứng với nhóm [BL-PRF](../../logic/profile/) trong `docs/logic/`.

| File | Mô tả | Trạng thái |
|------|-------|------------|
| [profile/profile-resolution.md](./profile/profile-resolution.md) | 3-tier profile merge (Company←Dept←User), cache TTL 60s, agent env injection | Enterprise Flow |

---

### 7. AI Providers — Quản lý AI Provider Accounts

Tương ứng với nhóm [BL-AIP](../../logic/ai-providers/) trong `docs/logic/`.

| File | Mô tả | Trạng thái |
|------|-------|------------|
| [ai-providers/ai-provider-credential.md](./ai-providers/ai-provider-credential.md) | AI key write: SubtleCrypto → relay SSH → AES-256-GCM (zero plaintext on server) | Enterprise Flow |
| [ai-providers/ai_provider_accounts.md](./ai-providers/ai_provider_accounts.md) | AI Provider Account management — registration, resolution, quota | Enterprise Flow |

---

### 8. Project Workspace — Workspace Tích hợp & Remote Git

Tương ứng với nhóm [BL-PW](../../logic/project-workspace/) trong `docs/logic/`.

| File | Mô tả | Trạng thái |
|------|-------|------------|
| [project-workspace/project-workspace-switch.md](./project-workspace/project-workspace-switch.md) | switchProject → RelayConnectionPool → Promise.all init → WorkspaceContext | Enterprise Flow |
| [project-workspace/project-folder-web-mode.md](./project-workspace/project-folder-web-mode.md) | Project folder management trong web mode | Enterprise Flow |
| [project-workspace/remote-git-ui.md](./project-workspace/remote-git-ui.md) | Explorer + Git Panel: status, diff, commit, push stream, PR creation | Enterprise Flow |

---

### 9. Task Graph — Quản lý Task theo Đồ thị

Tương ứng với nhóm [BL-TG](../../logic/task-graph/) trong `docs/logic/`.

| File | Mô tả | Trạng thái |
|------|-------|------------|
| [task-graph/task-agent-execution.md](./task-graph/task-agent-execution.md) | Task → AI decompose → agent spawn → git commit → PR (end-to-end) | Enterprise Flow |

---

### 10. Workflow Orchestration — Điều phối Workflow Đa Máy chủ

Tương ứng với nhóm [BL-WF](../../logic/workflow-orchestration/) trong `docs/logic/`.

| File | Mô tả | Trạng thái |
|------|-------|------------|
| [workflow-orchestration/workflow-orchestration.md](./workflow-orchestration/workflow-orchestration.md) | Template inheritance → DAG → wave execution → StepExecutors → resumable | Enterprise Flow |

---

### 11. Project Integration — Tích hợp GitHub/GitLab

Tương ứng với nhóm [BL-PI](../../logic/project-integration/) trong `docs/logic/`.

| File | Mô tả | Trạng thái |
|------|-------|------------|
| [project-integration/github-integration.md](./project-integration/github-integration.md) | GitHub CLI integration — preflight check, auth, status derivation, web mode fix | ✅ VERIFIED |
| [project-integration/connect-integrations.md](./project-integration/connect-integrations.md) | Connect Integrations — GitHub/GitLab/Linear/Jira — full flow + web fixes | ✅ VERIFIED |

---

### 12. CLI & Headless — Giao diện Lệnh

Tương ứng với nhóm [BL-CLI](../../logic/cli-headless/) trong `docs/logic/`.

| File | Mô tả | Trạng thái |
|------|-------|------------|
| [cli-headless/enable-orca-cli.md](./cli-headless/enable-orca-cli.md) | Enable Orca CLI — Electron install, web stub, auto-install on server boot | ✅ VERIFIED |

---

### 13. Fleet — Quản lý Fleet Dev Servers

Tương ứng với nhóm [BL-FLEET](../../logic/fleet/) trong `docs/logic/`.

| File | Mô tả | Trạng thái |
|------|-------|------------|
| [fleet/enterprise-migration-impact-assessment.md](./fleet/enterprise-migration-impact-assessment.md) | Enterprise migration impact — v5→v6 breaking changes, rollback plan | Reference |

---

## Kiến trúc tổng quan v6.0

```
[Browser]
    │ POST /auth/local → orca_session cookie (8h)
    │ WS :6768 + Cookie → WsSessionRouter → per-user OrcaRuntimeRpcServer
    │
[Orca Server (172.20.2.39)] ← NodeAdapter, Multi-DB, Migration 0001–0010
    │
    ├── RelayConnectionPool (shared SSH relay per devServerId)
    │   → DevServerRelayBridge → SSH → relay binary
    │
    ├── AgentConnectionManager (persistent WS pool per devServerId)
    │   → AgentWsConnection (direct-ws mode agents)
    │
    ├── ProfileResolver (3-tier: company←dept←user, cache 60s)
    ├── ProjectServerRouter (project → devServerId binding)
    ├── AIProviderService (metadata only, no plaintext keys)
    ├── WorkflowOrchestrator (DAG, wave execution, resumable)
    └── TaskService + TaskAgentExecutor (task → agent → git → PR)
    │
    │ relay RPC + HMAC-SHA256 signed RpcExecutionContext (30s TTL)
    │
[Dev Server Agent v6.0] ← JSON-RPC 2.0, context-verifier
    ├── pty-session-store    (per-userId PTY isolation)
    ├── profile-aware-spawner (inject profile env)
    ├── git-engine           (status/diff/commit/push/PR)
    ├── fs-handler           (readDir/readFile/grep)
    ├── worktree-engine      (add/remove/fanout)
    └── ai-credentials       (AES-256-GCM write/read)
```

---

## HLD Flow → Data Flow Mapping

| HLD Flow | Data Flow Document | Features |
|---|---|---|
| Flow 1: Local Multi-Agent | [agent-connection-modes.md](./agent-connection-modes.md) | F01/F04 |
| Flow 2: Remote SSH | [ssh-relay-connection.md](./ssh-relay-connection.md), [relay-management.md](./relay-management.md) | F07/SSH |
| Flow 3: Mobile Monitoring | [session-management.md](./session-management.md) | F03 |
| Flow 4: CI/CD | (CLI + daemon flows) | F09/F14 |
| Flow 5: Web Server | [multi-user-session.md](./multi-user-session.md) | F22/23/24/25 |
| Flow 6: Agent WebSocket | [agent-connection-modes.md](./agent-connection-modes.md) | F29 |
| **Flow 7: Profile Resolution** | **[profile-resolution.md](./profile-resolution.md)** | **F33** |
| **Flow 8: AI Credential** | **[ai-provider-credential.md](./ai-provider-credential.md)** | **F35** |
| **Flow 9: Project Switch** | **[project-workspace-switch.md](./project-workspace-switch.md)** | **F34/38** |
| **Flow 10: Task→Agent→PR** | **[task-agent-execution.md](./task-agent-execution.md), [remote-git-ui.md](./remote-git-ui.md)** | **F37/39** |

---

## Feature → Flow Traceability

| Feature | Primary Flow | Secondary Flows |
|---|---|---|
| F22 Web Server Mode | [multi-user-session.md](./multi-user-session.md) | [authentication.md](./authentication.md) |
| F23 Multi-User Auth | [multi-user-session.md](./multi-user-session.md) | — |
| F24 Per-User Sandbox | [multi-user-session.md](./multi-user-session.md) | [agent-connection-modes.md](./agent-connection-modes.md) |
| F25 Admin Panel | [multi-user-session.md](./multi-user-session.md) | — |
| F27 Fleet Health | [relay-management.md](./relay-management.md) | [runtime.md](./runtime.md) |
| F29 Agent WebSocket | [agent-connection-modes.md](./agent-connection-modes.md) | — |
| F33 Profile Hierarchy | [profile-resolution.md](./profile-resolution.md) | [task-agent-execution.md](./task-agent-execution.md) |
| F34 Project Binding | [project-workspace-switch.md](./project-workspace-switch.md) | [relay-management.md](./relay-management.md) |
| F35 AI Provider Mgmt | [ai-provider-credential.md](./ai-provider-credential.md) | [task-agent-execution.md](./task-agent-execution.md) |
| F36 Workflow Orchestration | [workflow-orchestration.md](./workflow-orchestration.md) | [relay-management.md](./relay-management.md) |
| F37 Task Graph | [task-agent-execution.md](./task-agent-execution.md) | [profile-resolution.md](./profile-resolution.md), [remote-git-ui.md](./remote-git-ui.md) |
| F38 Project Workspace | [project-workspace-switch.md](./project-workspace-switch.md) | [remote-git-ui.md](./remote-git-ui.md) |
| F39 Remote Git UI | [remote-git-ui.md](./remote-git-ui.md) | [project-workspace-switch.md](./project-workspace-switch.md) |

---

## Server context

| | |
|-|-|
| **Internal IP** | `172.20.2.39` |
| **Public URL** | `https://b15.openledger.vn` |
| **RPC port** | `6768` (WebSocket) |
| **HTTP port** | `6769` (web SPA + health) |
| **Container** | `orca-server` (docker compose) |
| **Version** | `1.4.138` |
| **gh CLI** | `v2.96.0` (installed 2026-07-25) |

---

## Quy ước

- **VERIFIED:** Đã xác nhận trực tiếp với source code và/hoặc server production
- **RECHECK:** Cần xác nhận lại — có thể còn giả định chưa verify
- Tài liệu ghi rõ line number và file path để dễ trace lại
