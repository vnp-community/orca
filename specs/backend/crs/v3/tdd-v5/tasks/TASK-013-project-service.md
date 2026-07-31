# TASK-013: ProjectService CRUD

**Phase:** 3 — Project Binding  
**Solution ref:** [SOL-V5-002](../solutions/SOL-V5-002-project-binding.md) §4  
**Prerequisite:** TASK-001 (migration 0007), TASK-004 (project-types.ts), TASK-012 (bootstrap wired)  
**Status:** ✅ DONE — 2026-07-28

---

## Mô tả

Implement `ProjectService` — CRUD service cho projects và project members. Phải validate `devServerId` tồn tại trong `DevServerManager` khi create.

---

## File cần tạo: `src/main/project/ProjectService.ts`

Implement đầy đủ theo [SOL-V5-002 §4](../solutions/SOL-V5-002-project-binding.md):

**Public API:**
- `create(params)` → `OrcaProject` — validate devServerId, auto-add creator as owner
- `get(projectId)` → `OrcaProject | null`
- `list(userId)` → `OrcaProject[]` — chỉ projects user là member
- `update(projectId, patch, updatedBy)` → `void`
- `delete(projectId, deletedBy)` → `void`
- `addMember(projectId, userId, role)` → `void` — upsert ON CONFLICT
- `removeMember(projectId, userId)` → `void`
- `updateMemberRole(projectId, userId, role)` → `void`
- `getMembers(projectId)` → `ProjectMember[]`
- `getMember(projectId, userId)` → `ProjectMember | null`
- `assertAccess(projectId, userId)` → `ProjectMember` hoặc throw `PROJECT_ACCESS_DENIED`

**Key constraints:**
- Constructor: `(pool: IConnectionPool, devServerManager: DevServerManager)`
- `create()`: gọi `devServerManager.getServer(devServerId)` — throw `DEV_SERVER_NOT_FOUND` nếu null
- `create()`: tự động gọi `addMember(id, createdBy, 'owner')` sau INSERT
- `list(userId)`: JOIN `orca_project_members` WHERE `user_id = userId`
- `addMember()`: dùng `ON CONFLICT(project_id, user_id) DO UPDATE SET role = excluded.role`
- Map column names: `dev_server_id` → `devServerId`, `repo_path` → `repoPath`, `default_branch` → `defaultBranch`

---

## Verification

```bash
pnpm tsc --noEmit
```

## Acceptance Criteria

- [x] `ProjectService` class export
- [x] 11 methods implement
- [x] `create()` validates devServerId
- [x] `create()` auto-adds owner member
- [x] `assertAccess()` throws `PROJECT_ACCESS_DENIED` when not member
- [x] Không TypeScript errors
