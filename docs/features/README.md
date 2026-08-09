# Orca — Feature Specifications

**Cập nhật:** 2026-08-08 | **Phiên bản:** v5.0  
**Tổng số features:** 42 (F01–F42)

---

## Tổng quan theo trạng thái

| Trạng thái | Số lượng | Features |
|-----------|---------|---------|
| ✅ Phát hành | 30 | F01–F13, F16, F19, F21–F31, F40–F42 |
| 🚧 Phát triển | 10 | F14, F15, F17, F18, F32–F39 |
| 📋 Kế hoạch | 2 | F20, F32 |

> F41–F42 được bổ sung 2026-08-08 từ rà soát code `frontend/` thực tế (không có trong PRD gốc) — xem ghi chú "Notes" trong từng file.

---

## Feature Registry

### Group 1: Core Desktop IDE (F01–F13)

| ID | Tên | Priority | Status | HLD | ADR |
|----|-----|----------|--------|-----|-----|
| [F01](./F01-parallel-worktrees.md) | Parallel Worktrees | P0 | ✅ | C3.1, C4.1 | — |
| [F02](./F02-terminal-splits.md) | Terminal Splits | P0 | ✅ | C3.1 | — |
| [F03](./F03-mobile-companion.md) | Mobile Companion | P0 | ✅ | C3.4 | — |
| [F04](./F04-ai-agent-support.md) | AI Agent Support | P0 | ✅ | C3.1, C3.8 | — |
| [F05](./F05-design-mode.md) | Design Mode | P1 | ✅ | C3.4 | — |
| [F06](./F06-github-linear-integration.md) | GitHub/Linear Integration | P1 | ✅ | C3.9 | — |
| [F07](./F07-ssh-worktrees.md) | SSH Worktrees | P1 | ✅ | C3.5 | ADR-004 |
| [F08](./F08-annotate-ai-diffs.md) | Annotate AI Diffs | P1 | ✅ | C3.4 | — |
| [F09](./F09-orca-cli.md) | Orca CLI | P1 | ✅ | C2 | — |
| [F10](./F10-quick-open.md) | Quick Open | P1 | ✅ | C3.4 | — |
| [F11](./F11-notifications.md) | Notifications | P1 | ✅ | C3.1 | — |
| [F12](./F12-file-explorer-editor.md) | File Explorer & Editor | P1 | ✅ | C3.4 | — |
| [F13](./F13-text-search.md) | Text Search | P1 | ✅ | C3.4 | — |

### Group 2: Advanced Desktop Features (F14–F21)

| ID | Tên | Priority | Status | HLD | ADR |
|----|-----|----------|--------|-----|-----|
| [F14](./F14-automations.md) | Automations | P2 | 🚧 | C3.1 | — |
| [F15](./F15-computer-use.md) | Computer Use | P2 | 🚧 | C3.4 | — |
| [F16](./F16-rich-repo-previews.md) | Rich Repo Previews | P2 | ✅ | C3.4 | — |
| [F17](./F17-memory-ai-vault.md) | Memory & AI Vault | P2 | 🚧 | C3.1 | — |
| [F18](./F18-ephemeral-vm.md) | Ephemeral VM | P2 | 🚧 | C3.1 | — |
| [F19](./F19-localization.md) | Localization | P2 | ✅ | C3.4 | — |
| [F20](./F20-speech-input.md) | Speech Input | P3 | 📋 | C3.4 | — |
| [F21](./F21-auto-update.md) | Auto Update | P0 | ✅ | C2 | — |

### Group 3: Web Server Mode & Enterprise (F22–F32)

| ID | Tên | Priority | Status | HLD | ADR |
|----|-----|----------|--------|-----|-----|
| [F22](./F22-web-server-mode.md) | Web Server Mode | P0 | ✅ | C2, C3.6 | ADR-001 |
| [F23](./F23-multi-user-auth.md) | Multi-User Auth | P0 | ✅ | C3.1, C2 | ADR-003 |
| [F24](./F24-per-user-sandbox.md) | Per-User Sandbox | P0 | ✅ | C3.1 | ADR-003 |
| [F25](./F25-admin-panel.md) | Admin Panel | P1 | ✅ | C3.1 | — |
| [F26](./F26-multi-database.md) | Multi-Database Support | P1 | ✅ | C3.7, C4.3 | ADR-002 |
| [F27](./F27-fleet-health-monitoring.md) | Fleet Health Monitoring | P1 | ✅ | C3.5 | ADR-004 |
| [F28](./F28-dev-server-onboarding.md) | Dev Server Onboarding | P1 | ✅ | C3.5 | ADR-004 |
| [F29](./F29-agent-websocket-protocol.md) | Agent WebSocket Protocol | P1 | ✅ | C3.8, C4.5 | ADR-005 |
| [F30](./F30-remote-integrations.md) | Remote Integrations | P1 | ✅ | C3.9 | ADR-006 |
| [F31](./F31-fleet-provisioning.md) | Fleet Provisioning | P1 | ✅ | C3.5 | ADR-004 |
| [F32](./F32-team-rbac.md) | Team RBAC | P2 | 📋 | C3.1 | — |

### Group 4: v5.0 — Profile, Project, AI Provider, Workflow, Task, Workspace (F33–F39)

| ID | Tên | Priority | Status | HLD | ADR |
|----|-----|----------|--------|-----|-----|
| [F33](./F33-user-profile-hierarchy.md) | User Profile Hierarchy | P0 | 🚧 | C3.10, C4.7 | ADR-007 |
| [F34](./F34-project-dev-server-binding.md) | Project-Dev Server Binding | P0 | 🚧 | C3.10, C4.8 | ADR-007, ADR-011 |
| [F35](./F35-ai-provider-account-management.md) | AI Provider Account Management | P0 | 🚧 | C2.14, C3.11 | ADR-008 |
| [F36](./F36-multi-server-workflow-orchestration.md) | Multi-Server Workflow Orchestration | P1 | 🚧 | C2.15, C3.11b | ADR-009 |
| [F37](./F37-task-graph-management.md) | Task Graph Management | P0 | 🚧 | C2.16, C3.11c | ADR-010 |
| [F38](./F38-project-workspace.md) | Project Workspace — Unified IDE | P0 | 🚧 | C3.12, C4.10 | ADR-011 |
| [F39](./F39-remote-git-ui.md) | Remote Git UI | P0 | 🚧 | C3.12, C4.10 | ADR-012 |

### Group 5: Observability (F40)

| ID | Tên | Priority | Status | HLD | ADR |
|----|-----|----------|--------|-----|-----|
| [F40](./F40-full-flow-tracing.md) | Full-Flow Tracing | P1 | ✅ | C3.1, C3.8 | — |

### Group 6: In-App Engagement & Onboarding UX (F41–F42)

> Bổ sung 2026-08-08 từ code thực tế, chưa có trong PRD gốc.

| ID | Tên | Priority | Status | HLD | ADR |
|----|-----|----------|--------|-----|-----|
| [F41](./F41-desktop-pet-companion.md) | Desktop Pet Companion | P3 | ✅ | C3.1 | — |
| [F42](./F42-contextual-onboarding-tours.md) | Contextual Onboarding Tours | P2 | ✅ | C3.4 | — |

---

## DB Migration → Feature Dependency

| Migration | Feature bật sau migration |
|-----------|--------------------------|
| 0001 initial | F01, F02, F04, F07 |
| 0002 automations | F14 |
| 0003 sessions | F22, F23 |
| 0004 app_tables | F23, F24, F25, F27, F28 |
| 0005 auth | F23, F25 |
| **0006 company+dept** | **F33** |
| **0007 projects** | **F34** |
| **0008 ai_providers** | **F35** |
| **0009 workflows** | **F36** |
| **0010 tasks** | **F37, F38, F39** |

---

## v5.0 Implementation Order (theo dependency)

```
Phase 1 — Foundation (v5.0a):
  F33 Profile Hierarchy → migration 0006 → ProfileResolver → Profile API
  F34 Project Binding   → migration 0007 → ProjectService → ProjectServerRouter

Phase 2 — AI Provider (v5.0b):
  F35 AI Provider Mgmt → migration 0008 → relay ai.provider.* → Admin UI

Phase 3 — Workspace (v5.0c):
  F38 Project Workspace → RelayConnectionPool → WorkspaceContext
  F39 Remote Git UI     → relay git-handler → GitPanel

Phase 4 — Task & Workflow (v5.0d):
  F37 Task Graph        → migration 0010 → TaskService → TaskAIPlanner
  F36 Workflow          → migration 0009 → DAGBuilder → WorkflowOrchestrator
```

---

## Coding Standards cho v5.0 features

Tất cả v5.0 features phải tuân thủ:
1. **Zero Mock**: không dùng mock data trong implementation
2. **Zero Hardcode**: config qua env vars hoặc DB settings
3. **IPlatformServices**: code v5.0 mới (Profile/Project/AI Provider/Workflow/Task —
   `src/main/profile`, `project`, `ai-providers`, `workflow`, `task`) không import
   electron trực tiếp. Áp dụng cho code **mới** thuộc các domain này; không áp dụng
   hồi tố cho code Electron/desktop main-process đã tồn tại trước restructure_v1
   (xem `specs/frontend/crs/v1/restructure_v1/README.md` — nguyên tắc "Additive only").
4. **IConnectionPool**: không access DB dialect trực tiếp
5. **relay.call()**: tất cả remote operations qua relay RPC
6. **Test coverage**: unit test cho service layer, integration test cho relay

Tham khảo: `docs/adrs/v1/README.md` — Architectural Principles
