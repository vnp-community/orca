# TASK-017: ProjectService Tests

**Phase:** 3 — Project Binding  
**Prerequisite:** TASK-013  
**Status:** ✅ DONE — 2026-07-29

---

## File cần tạo: `src/main/project/__tests__/ProjectService.test.ts`

**Setup:** Use `SqliteSingleConnectionPool` + `ALL_MIGRATIONS` (pattern từ TASK-010).

**Tests cần viết (≥ 15 tests):**

1. `create`: returns OrcaProject with correct fields
2. `create`: auto-adds creator as 'owner' member
3. `create`: throws DEV_SERVER_NOT_FOUND when invalid devServerId
4. `get`: returns null for non-existent project
5. `list`: returns only projects where userId is member
6. `list`: returns empty for user with no projects
7. `update`: updates name correctly
8. `update`: updates visibility correctly
9. `delete`: removes project
10. `addMember`: adds with role 'member'
11. `addMember`: upserts when called twice (last role wins)
12. `removeMember`: removes correctly
13. `updateMemberRole`: changes role
14. `assertAccess`: returns member for valid access
15. `assertAccess`: throws PROJECT_ACCESS_DENIED for non-member

## Acceptance Criteria

- [x] ≥ 15 tests, all pass
- [x] Tests cover all CRUD + access control
