# TASK-016: Project RPC Methods

**Phase:** 3 — Project Binding  
**Solution ref:** [SOL-V5-002](../solutions/SOL-V5-002-project-binding.md)  
**Prerequisite:** TASK-013, TASK-014  
**Status:** ✅ DONE — 2026-07-28

---

## File cần tạo: `src/main/project/project-rpc-handler.ts`

**Methods:**
- `project.list` → `projectService.list(session.userId)`
- `project.get` → `projectService.get(params.projectId)` (check access)
- `project.create` → `projectService.create({ ..., createdBy: session.userId })`
- `project.update` → `projectService.update(params.projectId, params.patch, session.userId)` (check owner/admin)
- `project.delete` → `projectService.delete` (check owner/admin)
- `project.addMember` → owner/admin only
- `project.removeMember` → owner/admin only
- `project.updateMemberRole` → owner/admin only
- `project.getMembers` → check member access
- `project.agentSpawn` → `agentSpawner.spawn(params)` (check member access)

## Acceptance Criteria

- [x] 10 RPC methods registered
- [x] Access control: create/delete/member ops check role
- [x] Không TypeScript errors
