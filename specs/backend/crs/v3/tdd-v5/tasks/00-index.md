# Task Index — TDD v5.0 Implementation

**Version:** 1.0  
**Date:** 2026-07-28  
**Scope:** TDD-14 → TDD-20 (v5.0)  
**Solutions Ref:** [../solutions/](../solutions/)  
**Total tasks:** 45 tasks | **Target tests:** ≥ 265 | **Modified files:** 2 | **New files:** ~52

---

## Execution Phases

| Phase | Domain | Tasks | Prerequisite |
|-------|--------|-------|-------------|
| **Phase 1** | Foundation (Migrations + RelayPool) | TASK-001~005 | None |
| **Phase 2** | Profile Hierarchy (TDD-14) | TASK-006~012 | Phase 1 |
| **Phase 3** | Project Binding (TDD-15) | TASK-013~020 | Phase 2 |
| **Phase 4** | AI Provider (TDD-16) | TASK-021~027 | Phase 1 |
| **Phase 5** | Workflow Orchestration (TDD-17) | TASK-028~034 | Phase 3 |
| **Phase 6** | Task Graph (TDD-18) | TASK-035~041 | Phase 3, 4 |
| **Phase 7** | Workspace + Remote Git (TDD-19, 20) | TASK-042~045 | Phase 3, 5, 6 |

---

## Task List

| Task | Phase | File | Description | Status |
|------|-------|------|-------------|--------|
| [TASK-001](./TASK-001-migrations-0006-0010.md) | 1 | Migrations | DB migrations 0006–0010 | ✅ DONE |
| [TASK-002](./TASK-002-relay-connection-pool.md) | 1 | RelayPool | RelayConnectionPool + tests | ✅ DONE |
| [TASK-003](./TASK-003-migration-index-update.md) | 1 | DB Index | Update migrations/index.ts | ✅ DONE |
| [TASK-004](./TASK-004-shared-types.md) | 1 | Types | Shared type files (project, ai-provider, task) | ✅ DONE |
| [TASK-005](./TASK-005-server-bootstrap-result.md) | 1 | Bootstrap | Extend ServerBootstrapResult interface | ✅ DONE |
| [TASK-006](./TASK-006-orca-profile-types.md) | 2 | Profile | OrcaProfile.ts types | ✅ DONE |
| [TASK-007](./TASK-007-profile-service.md) | 2 | Profile | ProfileService CRUD | ✅ DONE |
| [TASK-008](./TASK-008-profile-resolver.md) | 2 | Profile | ProfileResolver deep-merge + cache | ✅ DONE |
| [TASK-009](./TASK-009-profile-rpc-methods.md) | 2 | Profile | Profile RPC methods | ✅ DONE |
| [TASK-010](./TASK-010-profile-service-tests.md) | 2 | Profile | ProfileService tests | ✅ DONE |
| [TASK-011](./TASK-011-profile-resolver-tests.md) | 2 | Profile | ProfileResolver tests | ✅ DONE |
| [TASK-012](./TASK-012-profile-bootstrap-wire.md) | 2 | Profile | Wire ProfileService to bootstrap (step 7) | ✅ DONE |
| [TASK-013](./TASK-013-project-service.md) | 3 | Project | ProjectService CRUD | ✅ DONE |
| [TASK-014](./TASK-014-project-server-router.md) | 3 | Project | ProjectServerRouter | ✅ DONE |
| [TASK-015](./TASK-015-profile-aware-spawner.md) | 3 | Project | ProfileAwareAgentSpawner | ✅ DONE |
| [TASK-016](./TASK-016-project-rpc-methods.md) | 3 | Project | project.* RPC methods | ✅ DONE |
| [TASK-017](./TASK-017-project-service-tests.md) | 3 | Project | ProjectService tests | ✅ DONE |
| [TASK-018](./TASK-018-project-router-tests.md) | 3 | Project | ProjectServerRouter + Spawner tests | ✅ DONE |
| [TASK-019](./TASK-019-project-rpc-tests.md) | 3 | Project | project RPC tests | ✅ DONE |
| [TASK-020](./TASK-020-project-bootstrap-wire.md) | 3 | Project | Wire ProjectService to bootstrap (step 8) | ✅ DONE |
| [TASK-021](./TASK-021-ai-provider-service.md) | 4 | AI Provider | AIProviderService CRUD + relay | ✅ DONE |
| [TASK-022](./TASK-022-provider-resolver.md) | 4 | AI Provider | ProviderResolver priority logic | ✅ DONE |
| [TASK-023](./TASK-023-provider-health-checker.md) | 4 | AI Provider | ProviderHealthChecker cron | ✅ DONE |
| [TASK-024](./TASK-024-relay-ai-handler.md) | 4 | AI Provider | relay/ai-provider-handler.ts | ✅ DONE |
| [TASK-025](./TASK-025-ai-provider-rpc-methods.md) | 4 | AI Provider | aiProvider.* RPC methods | ✅ DONE |
| [TASK-026](./TASK-026-ai-provider-tests.md) | 4 | AI Provider | AIProviderService + resolver tests | ✅ DONE |
| [TASK-027](./TASK-027-ai-provider-bootstrap-wire.md) | 4 | AI Provider | Wire AIProviderService to bootstrap (step 9) | ✅ DONE |
| [TASK-028](./TASK-028-workflow-types-dag.md) | 5 | Workflow | WorkflowTypes + DAGBuilder | ✅ DONE |
| [TASK-029](./TASK-029-workflow-orchestrator.md) | 5 | Workflow | WorkflowOrchestrator (start, run, resume, cancel) | ✅ DONE |
| [TASK-030](./TASK-030-template-resolver.md) | 5 | Workflow | TemplateResolver inheritance | ✅ DONE |
| [TASK-031](./TASK-031-step-executors.md) | 5 | Workflow | StepExecutors (agent, shell, webhook) | ✅ DONE |
| [TASK-032](./TASK-032-workflow-rpc-methods.md) | 5 | Workflow | workflow.* RPC methods | ✅ DONE |
| [TASK-033](./TASK-033-workflow-tests.md) | 5 | Workflow | DAGBuilder + Orchestrator + Template tests | ✅ DONE |
| [TASK-034](./TASK-034-workflow-bootstrap-wire.md) | 5 | Workflow | Wire WorkflowOrchestrator to bootstrap (step 10) | ✅ DONE |
| [TASK-035](./TASK-035-task-service.md) | 6 | Task | TaskService CRUD + tree ops + edges | ✅ DONE |
| [TASK-036](./TASK-036-task-dag-validator.md) | 6 | Task | TaskDAGValidator BFS cycle detection | ✅ DONE |
| [TASK-037](./TASK-037-task-grant-service.md) | 6 | Task | TaskGrantService BFS ancestor resolution | ✅ DONE |
| [TASK-038](./TASK-038-task-ai-planner.md) | 6 | Task | TaskAIPlanner decompose + apply | ✅ DONE |
| [TASK-039](./TASK-039-task-agent-executor.md) | 6 | Task | TaskAgentExecutor spawn + status | ✅ DONE |
| [TASK-040](./TASK-040-task-rpc-methods.md) | 6 | Task | task.* RPC methods | ✅ DONE |
| [TASK-041](./TASK-041-task-bootstrap-wire.md) | 6 | Task | Wire TaskService to bootstrap (step 11) | ✅ DONE |
| [TASK-042](./TASK-042-workspace-service.md) | 7 | Workspace | WorkspaceService + WorkspaceContext React | ✅ DONE |
| [TASK-043](./TASK-043-workspace-rpc-methods.md) | 7 | Workspace | workspace.* RPC methods | ✅ DONE |
| [TASK-044](./TASK-044-remote-git-handler.md) | 7 | Git | relay/git-handler.ts + git-remote.ts RPC | ✅ DONE |
| [TASK-045](./TASK-045-git-ui-components.md) | 7 | Git | Git UI React components | ✅ DONE |

---

## Execution Rules for AI Agent

1. **Dependency order**: Thực thi theo Phase 1 → 7, không skip phase
2. **Test-first**: Mỗi TASK tạo implementation trước, tests sau (hoặc song song)  
3. **Additive only**: Không xóa hay sửa code hiện tại ngoài 2 files cho phép
4. **Run tests**: Sau mỗi TASK, run `pnpm test --run <test-file>` để verify
5. **Type check**: Sau mỗi phase, run `pnpm tsc --noEmit` để kiểm tra TypeScript
6. **Non-blocking errors**: Nếu test fail, document lý do và tiếp tục; không block phase tiếp theo trừ migration

---

## Quick Reference — Modified Files Only

```
src/main/server-bootstrap.ts    — Add steps 2a-pool, 7–12; extend interface
src/main/db/migrations/index.ts — Add imports + entries for 0006–0010
```
