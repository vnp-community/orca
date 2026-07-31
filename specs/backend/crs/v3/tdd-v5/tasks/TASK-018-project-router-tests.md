# TASK-018: ProjectServerRouter + Spawner Tests

**Phase:** 3 — Project Binding  
**Prerequisite:** TASK-014, TASK-015  
**Status:** ✅ DONE — 2026-07-29

---

## Files cần tạo

### `src/main/project/__tests__/ProjectServerRouter.test.ts` (≥ 10 tests)

Use mocks for ProjectService, DevServerManager, RelayConnectionPool.

Tests:
1. `getRelayForProject`: valid member → relay returned from pool
2. `getRelayForProject`: non-member → PROJECT_ACCESS_DENIED propagated
3. `getRelayForProject`: project not found → PROJECT_NOT_FOUND
4. `getRelayForProject`: dev server not found → DEV_SERVER_NOT_FOUND
5. `getProjectContext`: all fields populated
6. `getProjectContext`: profileResolver.resolve called with userId
7. `getProject`: delegates to projectService.get

### `src/main/project/__tests__/ProfileAwareAgentSpawner.test.ts` (≥ 8 tests)

Tests:
1. `spawn`: ORCA_PROJECT_ID in env
2. `spawn`: ORCA_USER_ID in env
3. `spawn`: pathAdditions prepended to PATH
4. `spawn`: resolvedProfile.shell.envVars injected
5. `spawn`: relay.call('agent.exec') invoked
6. `spawn`: trustPreset from profile passed
7. `spawn`: mcpServers from profile passed
8. `spawn`: ORCA_MODEL set from provider account

## Acceptance Criteria

- [x] ≥ 10 router tests pass
- [x] ≥ 8 spawner tests pass
