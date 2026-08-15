# Architecture Decision Records — Orca v5.0 / v6.0

**Phạm vi:** v1 ADRs — kiến trúc HLD v5.0 (ADR-001–012) và v6.0 Dev Server Agent (ADR-013–015)  
**Ngày tạo:** 2026-07-28 | **Cập nhật:** 2026-07-30 (v6.0 additions + HLD sync)  
**Định dạng:** MADR (Markdown Architecture Decision Records)

> 📌 Xem **[ADR v2](../v2/README.md)** cho các quyết định kiến trúc bổ sung từ HLD v6.0:
> ADR-016 (Multi-DB migrations 0006–0010), ADR-017 (Dev Server Agent Layers A0–A4),
> ADR-018 (Control Plane vs Data Plane), ADR-019 (Agent Autonomous Operation),
> ADR-020 (Enterprise Rollout Phases).

---

## Danh sách ADR

| ID | Tiêu đề | Trạng thái | Features | HLD Ref | Ngày |
|----|---------|------------|----------|---------|------|
| [ADR-001](./ADR-001-platform-abstraction-iplatformservices.md) | Platform Abstraction via IPlatformServices | ✅ Accepted | F22 | C3.6, C4.2 | 2026-07-28 |
| [ADR-002](./ADR-002-multi-database-iconnectionpool.md) | Multi-Database via IConnectionPool Interface | ✅ Accepted + Partial (0006–0010) | F26, F33–F37 | C3.7, C4.3 | 2026-07-28 |
| [ADR-003](./ADR-003-per-user-process-isolation.md) | Per-User Process Isolation via SessionManager | ✅ Accepted | F23, F24 | C3.1, C2 | 2026-07-28 |
| [ADR-004](./ADR-004-relay-binary-ssh-remote-execution.md) | SSH Relay Binary for Remote Dev Server Execution | 🔄 Superseded → ADR-013 | F07, F27–F31 | C3.5, C4.4 | 2026-07-28 |
| [ADR-005](./ADR-005-agent-websocket-binary-wire-protocol.md) | Agent WebSocket Binary Wire Protocol (13-byte Header) | 🔄 Superseded → ADR-014 | F29 | C3.8, C4.5 | 2026-07-28 |
| [ADR-006](./ADR-006-web-credential-store-aes256gcm.md) | WebCredentialStore — AES-256-GCM Per-User Credential Isolation | ✅ Accepted | F30 | C3.9 | 2026-07-28 |
| [ADR-007](./ADR-007-profile-hierarchy-deep-merge.md) | 3-Layer Profile Hierarchy with Deep-Merge Strategy | 🚧 Proposed → [ADR-016](../v2/ADR-016-db-migrations-0006-0010-schema.md) | F33, F34 | C3.10, C4.7 | 2026-07-28 |
| [ADR-008](./ADR-008-ai-provider-credential-on-dev-server.md) | AI Provider Credentials Stored on Dev Server (Not Orca Server) | 🚧 Proposed | F35 | C2.14, C3.11 | 2026-07-28 |
| [ADR-009](./ADR-009-workflow-dag-orchestration.md) | Workflow Orchestration via DAG with Topological Sort | 🚧 Proposed | F36 | C2.15, C3.11b | 2026-07-28 |
| [ADR-010](./ADR-010-task-graph-dag-model.md) | Task Graph as DAG with BFS Access Control | 🚧 Proposed | F37 | C2.16, C3.11c | 2026-07-28 |
| [ADR-011](./ADR-011-project-workspace-relay-connection-pool.md) | Project Workspace via AgentConnectionManager + WorkspaceContext | ⚠️ Amended (v6.0) | F34, F38 | C3.12, C3.13 | 2026-07-28 |
| [ADR-012](./ADR-012-remote-git-ui-via-relay-rpc.md) | Remote Git UI via Agent RPC (Not Local Git) | 🚧 Proposed | F39 | C3.12, C3.13 | 2026-07-28 |
| [ADR-013](./ADR-013-dev-server-agent-replaces-relay.md) | Dev Server Agent Replaces Thin Relay Binary | 🚧 Proposed (v6.0) → [ADR-017](../v2/ADR-017-dev-server-agent-layer-model.md) | F01–F04, F07, F12–F14, F17, F36–F39 | C3.13, C4.11 | 2026-07-30 |
| [ADR-014](./ADR-014-gateway-agent-json-rpc-protocol.md) | Gateway–Agent JSON-RPC 2.0 Protocol v3 | 🚧 Proposed (v6.0) | F29 | C3.13, C4.11 | 2026-07-30 |
| [ADR-015](./ADR-015-signed-execution-context-gateway-agent.md) | Signed Execution Context for Gateway–Agent Trust | 🚧 Proposed (v6.0) | F24, F32, F33, F35 | security.md, C3.13 | 2026-07-30 |

---

## Trạng thái ADR

| Trạng thái | Ý nghĩa |
|-----------|---------| 
| ✅ Accepted | Đã implement trong codebase hiện tại |
| 🚧 Proposed | Đề xuất chưa implement |
| ⚠️ Amended | Đã implement nhưng cập nhật thiết kế cho v6.0 |
| 🔄 Superseded | Đã bị thay thế bởi ADR khác (giữ lại để lưu lịch sử) |
| ⛔ Deprecated | Không còn áp dụng |

---

## Feature → ADR Traceability

| Feature | Tên | ADR(s) |
|---------|-----|--------|
| F01 | Parallel Worktrees | ADR-013 (agent-side worktree) |
| F02 | Terminal Splits | ADR-013 (PTY on agent) |
| F04 | AI Agent Support | ADR-013 (profile-aware spawner), ADR-015 (signed ctx) |
| F07 | SSH Worktrees | ADR-013 (supersedes ADR-004) |
| F09 | Orca CLI | ADR-013 (auth required, dispatch to agent) |
| F12 | File Explorer | ADR-013 (fs.* on agent) |
| F13 | Text Search | ADR-013 (ripgrep on agent) |
| F17 | AI Memory Vault | ADR-013 (local SQLite on agent) |
| F22 | Web Server Mode | ADR-001 |
| F23 | Multi-User Auth | ADR-003 |
| F24 | Per-User Sandbox | ADR-003, ADR-015 |
| F26 | Multi-Database | ADR-002 |
| F27 | Fleet Health Monitoring | ADR-013 (health reporter) |
| F28 | Dev Server Onboarding | ADR-013 (agent registration) |
| F29 | Agent WebSocket Protocol | ADR-014 (supersedes ADR-005) |
| F30 | Remote Integrations | ADR-006 |
| F31 | Fleet Provisioning | ADR-013 (agent deployment) |
| F32 | Team RBAC | ADR-015 (RBAC in signed ctx) |
| F33 | User Profile Hierarchy | ADR-007, ADR-015 (profile in signed ctx) |
| F34 | Project-Dev Server Binding | ADR-007, ADR-011 |
| F35 | AI Provider Account Management | ADR-008, ADR-013 (cred store on agent) |
| F36 | Multi-Server Workflow Orchestration | ADR-009, ADR-013 (step executor on agent) |
| F37 | Task Graph Management | ADR-010, ADR-013 (task agent executor) |
| F38 | Project Workspace | ADR-011 (amended v6.0) |
| F39 | Remote Git UI | ADR-012, ADR-013 (git engine on agent) |

---

## ADR → Code Traceability

| ADR | Code đã implement | Code cần tạo (v5.0/v6.0) |
|-----|-------------------|--------------------------|
| ADR-001 | `src/platform/`, `src/platform/adapters/` | — |
| ADR-002 | `src/main/db/`, migrations 0001–0005 | **migrations 0006–0010** (schema documented in [ADR-016](../v2/ADR-016-db-migrations-0006-0010-schema.md)) |
| ADR-003 | `src/main/session/`, `ws-session-router.ts` | — |
| ADR-004 | `src/relay/`, `src/main/ssh/` | ~~relay deprecated~~ (→ ADR-013) |
| ADR-005 | `src/main/dev-server/agent-ws-server.ts` | ~~binary protocol deprecated~~ (→ ADR-014) |
| ADR-006 | `src/main/credentials/web-credential-store.ts` | AI provider scopes |
| ADR-007 | — | `src/main/profile/ProfileResolver.ts`, `ProfileService.ts`, `ProfileCache.ts` |
| ADR-008 | — | `src/agent/ai-credentials/credential-writer.ts`, `src/main/ai-providers/AIProviderService.ts` |
| ADR-009 | `src/main/runtime/orchestration/` (partial) | `src/main/workflow/DAGBuilder.ts`, `WorkflowOrchestrator.ts`, `TemplateResolver.ts` |
| ADR-010 | — | `src/main/task/TaskService.ts`, `TaskDAGValidator.ts`, `TaskGrantService.ts`, `TaskAIPlanner.ts`, `TaskAgentExecutor.ts` |
| ADR-011 | — | `src/main/dev-server/agent-connection-manager.ts`, `src/renderer/src/context/WorkspaceContext.tsx` |
| ADR-012 | `src/main/runtime/rpc/methods/git.ts` (local only) | `src/agent/git/git-engine.ts`, `src/agent/git/git-stream.ts`, `git-pr-creator.ts` |
| **ADR-013** | — | `src/agent/` (new package), `src/main/dev-server/agent-connection-manager.ts`, `agent-dispatcher.ts` |
| **ADR-014** | — | `src/agent/rpc/agent-rpc-server.ts`, `src/agent/rpc/method-router.ts`, `event-emitter.ts` |
| **ADR-015** | — | `src/agent/rpc/context-verifier.ts`, `src/main/dev-server/signed-context-issuer.ts` |

---

## Architectural Principles (rút ra từ tất cả ADRs)

### 1. Interface-First Isolation
> *"Không bao giờ import platform-specific modules trực tiếp trong business logic."*  
> — ADR-001: `IPlatformServices` pattern

### 2. Security by Credential Location
> *"Credentials sống gần nơi chúng được dùng, không phải nơi dễ quản lý nhất."*  
> — ADR-008: AI keys trên Dev Server Agent, không trên Orca Gateway

### 3. Control Plane vs Data Plane Separation (v6.0)
> *"Gateway quyết định ai được làm gì; Agent thực thi những gì được ủy quyền."*  
> — ADR-013: Dev Server Agent architecture

### 4. Signed Context for Cross-Process Trust (v6.0)
> *"Mọi cross-process call mang đủ identity + policy đã được sign — không lookup lại."*  
> — ADR-015: HMAC-SHA256 signed context, 30s TTL

### 5. DAG for Parallel Work
> *"Khi có dependency thì dùng DAG; khi không có dependency thì chạy song song."*  
> — ADR-009 (Workflow), ADR-010 (Task Graph)

### 6. Event Bus Over Tight Coupling
> *"Panels giao tiếp qua events, không phải function calls trực tiếp."*  
> — ADR-011: WorkspaceContext micro-emitter

### 7. BFS for Graph Traversal
> *"Cycle detection và grant resolution đều dùng BFS — predictable, bounded depth."*  
> — ADR-010: Task grant resolution

### 8. Agent Autonomous Operation (v6.0)
> *"Agent phải hoạt động đầy đủ ngay cả khi Gateway offline — stateful, self-sufficient."*  
> — ADR-013: Local SQLite, reconnect manager, event buffering

---

## Ngữ cảnh chung

Tất cả ADRs được viết dựa trên phân tích:
- **Codebase:** `src/main/`, `src/relay/`, `src/platform/`, `src/server/`, `src/agent/` (v6.0 new)
- **Deploy:** `deploy/dev/agent/agent.js`, `deploy/agent/` (v6.0)
- **HLD:** `docs/hld/v1/README.md`, `docs/hld/v1/C3-components.md`, `docs/hld/v1/C4-code.md`, `docs/hld/v1/security.md` (tầm nhìn v6.0/proposed) + `docs/hld/backend-server-architecture.md`, `docs/hld/dev-server-architecture.md`, `docs/hld/web-server-architecture.md` (kiến trúc hiện hành, đã cập nhật 2026-08-14)
- **Features:** `docs/features/F01–F39`
- **CRs:** `docs/crs/v2/dev-server/CR-DS-001–005`

---

## Gaps & Open Questions (cập nhật v6.0)

| # | Gap | ADR | Priority |
|---|-----|-----|---------|
| G1 | SEQ overflow (u32 wrap-around) trong ADR-005 | ADR-005 (deprecated) | — |
| G2 | AgentConnectionManager: cross-user connection sharing cần isolation | ADR-011, ADR-015 | High |
| G3 | Profile cache TTL 60s: suspended user vẫn có valid profile ≤60s | ADR-007 | Medium |
| G4 | Workflow `definitionSnapshot` JSON field size cần index strategy | ADR-009 | Low |
| G5 | Task progress calculation recursive — cần materialized view cho large trees | ADR-010 | Medium |
| G6 | `ORCA_AI_CREDENTIAL_KEY` rotation: cần migration script để re-encrypt | ADR-008, ADR-013 | High |
| G7 | Windows: git.exec path handling trong Agent (Git for Windows) | ADR-013 | Medium |
| **G8** | **Clock skew Gateway↔Agent > 5s → context expire sớm** | **ADR-015** | **High** |
| **G9** | **Agent binary size ~80MB — too large cho slow CI/CD** | **ADR-013** | **Low** |
| **G10** | **Multi-Gateway ORCA_GATEWAY_SECRET rotation: zero-downtime cần 30s overlap** | **ADR-015** | **Medium** |
| **G11** | **Agent reconnect event buffer 1000 events: overflow strategy cần spec** | **ADR-014** | **Medium** |
| **G12** | **Backward compat: Gateway serving both relay (v5) and agent (v6) concurrently** | **ADR-013** | **High** |



---

## ADR → Code Traceability

| ADR | Code đã implement | Code cần tạo (v5.0) |
|-----|-------------------|---------------------|
| ADR-001 | `src/platform/`, `src/platform/adapters/` | — |
| ADR-002 | `src/main/db/`, migrations 0001–0005 | migrations 0006–0010 |
| ADR-003 | `src/main/session/` | — |
| ADR-004 | `src/relay/`, `src/main/ssh/`, `deploy/dev/agent/` | `src/relay/git-handler.ts` |
| ADR-005 | `src/main/dev-server/agent-ws-server.ts`, `src/relay/protocol.ts` | — |
| ADR-006 | `src/main/credentials/web-credential-store.ts` | AI provider scopes |
| ADR-007 | — | `src/main/profile/ProfileResolver.ts`, `ProfileService.ts` |
| ADR-008 | — | `src/main/ai-providers/AIProviderService.ts`, relay: `ai.provider.*` |
| ADR-009 | `src/main/runtime/orchestration/` (partial) | DAGBuilder, WorkflowOrchestrator |
| ADR-010 | — | `src/main/task/TaskService.ts`, `TaskGrantService.ts`, `TaskAIPlanner.ts` |
| ADR-011 | `src/main/dev-server/dev-server-relay-bridge.ts` (partial) | `relay-connection-pool.ts`, `WorkspaceContext.tsx` |
| ADR-012 | `src/main/runtime/rpc/methods/git.ts` (local only) | `src/relay/git-handler.ts`, remote git RPC methods |

---

## Architectural Principles (rút ra từ các ADRs)

### 1. Interface-First Isolation
> *"Không bao giờ import platform-specific modules trực tiếp trong business logic."*  
> — ADR-001: `IPlatformServices` pattern

### 2. Security by Credential Location
> *"Credentials sống gần nơi chúng được dùng, không phải nơi dễ quản lý nhất."*  
> — ADR-008: AI keys chỉ trên Dev Server

### 3. Relay as Universal Remote Primitive
> *"Mọi remote operation đều qua relay RPC — không có special-case SSH commands."*  
> — ADR-004, ADR-012: relay.call() pattern

### 4. DAG for Parallel Work
> *"Khi có dependency thì dùng DAG; khi không có dependency thì chạy song song."*  
> — ADR-009 (Workflow), ADR-010 (Task Graph)

### 5. Event Bus Over Tight Coupling
> *"Panels giao tiếp qua events, không phải function calls trực tiếp."*  
> — ADR-011: WorkspaceContext micro-emitter

### 6. BFS for Graph Traversal
> *"Cycle detection và grant resolution đều dùng BFS — predictable, bounded depth."*  
> — ADR-010: Task grant resolution

---

## Ngữ cảnh chung

Tất cả ADRs được viết dựa trên phân tích:
- **Codebase:** `src/main/`, `src/relay/`, `src/platform/`, `src/server/`
- **Deploy:** `deploy/dev/agent/agent.js`
- **HLD:** `docs/hld/C2-containers.md`, `C3-components.md`, `C4-code.md`
- **Features:** `docs/features/F01–F39`

---

## Gaps & Open Questions

| # | Gap | ADR | Priority |
|---|-----|-----|---------|
| G1 | SEQ overflow (u32 wrap-around) chưa được handle | ADR-005 | Medium |
| G2 | RelayConnectionPool: cross-user connection sharing cần isolation layer | ADR-011 | High |
| G3 | Profile cache TTL 60s: suspended user vẫn có valid profile ≤60s | ADR-007 | Medium |
| G4 | Workflow `definitionSnapshot` JSON field size cần index strategy | ADR-009 | Low |
| G5 | Task progress calculation recursive — cần materialized view cho large trees | ADR-010 | Medium |
| G6 | `ORCA_AI_CREDENTIAL_KEY` rotation: cần migration script để re-encrypt | ADR-008 | High |
| G7 | Windows: git.exec path handling trong relay (Git for Windows) | ADR-012 | Medium |
