# TASK-019: Project RPC Tests

**Phase:** 3 — Project Binding  
**Prerequisite:** TASK-016  
**Status:** ✅ DONE — 2026-07-29

---

## File cần tạo: `src/main/project/__tests__/project-rpc.test.ts` (≥ 5 tests)

Use mocks for ProjectService, ProjectServerRouter, AgentSpawner.

Tests:
1. `project.list` → returns user projects
2. `project.create` → delegates to service, returns new project
3. `project.addMember` → non-owner → throws FORBIDDEN
4. `project.agentSpawn` → calls spawner.spawn
5. `project.delete` → admin OK, non-admin/non-owner → FORBIDDEN

## Acceptance Criteria

- [x] ≥ 5 tests pass
