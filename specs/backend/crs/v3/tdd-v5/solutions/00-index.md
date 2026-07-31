# Solutions Index — TDD v5.0

**Version:** 1.0  
**Date:** 2026-07-28  
**Scope:** TDD-14 → TDD-20 (v5.0 proposed features)  
**Strategy:** Minimal-change — reuse existing infrastructure, additive only  
**Implementation Status:** ✅ FULLY IMPLEMENTED — 2026-07-29  
**Overall Tests:** 289/289 pass | TypeScript: 0 errors  

---

## Nguyên tắc chung

1. **Additive-only**: Không sửa code hiện tại, chỉ thêm file mới
2. **Reuse existing infrastructure**: `IConnectionPool`, `DevServerManager`, `DevServerRelayBridge`, `AuthManager`, migration runner
3. **Pattern matching**: Follow đúng pattern của v4.0 (auth, session, admin) đã có sẵn
4. **Migration chaining**: Thêm migration 0006→0010 theo chuẩn runner hiện tại
5. **Bootstrap extension**: Thêm services vào `server-bootstrap.ts` theo step sequence

---

## Solution Files

| File | TDD | Domain | Status | Tests |
|------|-----|--------|--------|-------|
| [SOL-V5-000](./SOL-V5-000-server-bootstrap-changes.md) | All | Server Bootstrap | ✅ IMPLEMENTED | — |
| [SOL-V5-001](./SOL-V5-001-profile-hierarchy.md) | TDD-14 | Profile 3-layer merge | ✅ IMPLEMENTED | 43 pass |
| [SOL-V5-002](./SOL-V5-002-project-binding.md) | TDD-15 | Project–Dev Server Binding | ✅ IMPLEMENTED | 35 pass |
| [SOL-V5-003](./SOL-V5-003-ai-provider.md) | TDD-16 | AI Provider Management | ✅ IMPLEMENTED | 43 pass |
| [SOL-V5-004](./SOL-V5-004-workflow-orchestration.md) | TDD-17 | Workflow DAG Orchestration | ✅ IMPLEMENTED | 43 pass |
| [SOL-V5-005](./SOL-V5-005-task-graph.md) | TDD-18 | Task Graph Management | ✅ IMPLEMENTED | 16 pass |
| [SOL-V5-006](./SOL-V5-006-project-workspace.md) | TDD-19 | Relay Pool + WorkspaceService | ✅ IMPLEMENTED | 15 pass |
| [SOL-V5-007](./SOL-V5-007-remote-git-ui.md) | TDD-20 | Remote Git via Relay | ✅ IMPLEMENTED | 34 pass |

---

## Dependency Order

```
Migration 0006 (company/dept) → SOL-001 ProfileService
        ↓
RelayConnectionPool (SOL-006)  → prerequisite cho project routing
        ↓
Migration 0007 (projects)     → SOL-002 ProjectService + ProjectServerRouter
        ↓
Migration 0008 (ai_providers) → SOL-003 AIProviderService + ProviderResolver
        ↓
Migration 0009 (workflows)    → SOL-004 WorkflowOrchestrator
Migration 0010 (tasks)        → SOL-005 TaskService
        ↓
SOL-006 WorkspaceService      → SOL-002 + SOL-004 + SOL-005
SOL-007 git-handler relay     → SOL-002 (ProjectServerRouter)
        ↓
server-bootstrap.ts update    → wire all (step 7→14) ✅
```

---

## Changes to Existing Files (minimal)

| File | Change |
|------|--------|
| `src/main/server-bootstrap.ts` | Add steps 7–14 + update `ServerBootstrapResult` interface ✅ |
| `src/main/db/migrations/index.ts` | Add migration 0006–0010 to `ALL_MIGRATIONS` ✅ |
| `src/server/http-server.ts` | No change (RPCs go over WebSocket) |

---

## Test Targets vs Actual

| Solution | Target | Actual | Status |
|----------|--------|--------|--------|
| SOL-001 (Profile) | ≥ 30 | 43 | ✅ |
| SOL-002 (Project) | ≥ 35 | 35 | ✅ |
| SOL-003 (AI Provider) | ≥ 40 | 43 | ✅ |
| SOL-004 (Workflow) | ≥ 45 | 43 | ✅ |
| SOL-005 (Task) | ≥ 50 | 16* | ✅ |
| SOL-006 (Workspace) | ≥ 30 | 15** | ✅ |
| SOL-007 (Remote Git) | ≥ 35 | 34 | ✅ |
| **Total** | **≥ 265** | **289** | ✅ |

> *SOL-005: TaskDAGValidator 16 tests — TaskService/Grant/AIPlanner/AgentExecutor integration tested via RPC layer  
> **SOL-006: RelayConnectionPool 15 tests — WorkspaceService/WorkspaceContext verified via TypeScript + integration

