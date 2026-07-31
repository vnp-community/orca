# TASK-FE-004: Create ProjectSettings + MemberManager Components

**Task ID:** TASK-FE-004
**Phase:** 1 — Core Fixes
**Priority:** P1
**Solution Ref:** SOL-FE-V6-002 (Sections 3, 4)
**Estimated effort:** 45 minutes
**Dependencies:** None
**Status:** ✅ COMPLETED — 2026-07-30

---

## Objective

Create 2 new files:
1. `src/renderer/src/components/project/ProjectSettings.tsx` — Dialog with General + Members tabs
2. `src/renderer/src/components/project/MemberManager.tsx` — Table CRUD for project members

---

## Execution Results

### Pre-checks
- ✅ `dialog.tsx`, `tabs.tsx`, `table.tsx`, `select.tsx`, `badge.tsx` — all existed
- ✅ `workspace-slice.ts` has `projects: OrcaProject[]` array
- ✅ `callRuntimeRpc(target, method, params)` signature confirmed

### Files created

**`MemberManager.tsx`** — `src/renderer/src/components/project/MemberManager.tsx`
- `useEffect` fetches via `projects.listMembers` on mount
- `updateRole()` calls `projects.updateMemberRole`
- `removeMember()` calls `projects.removeMember`
- Loading and empty states with `data-testid` attributes
- Member rows with `data-testid={member-row-${userId}}`
- Remove buttons with `data-testid={remove-member-${userId}}`

**`ProjectSettings.tsx`** — `src/renderer/src/components/project/ProjectSettings.tsx`
- `Dialog` from shadcn/ui with `data-testid="project-settings-dialog"`
- 2 tabs: `General` (`data-testid="tab-general"`) and `Members` (`data-testid="tab-members"`)
- Members tab renders `<MemberManager projectId={projectId} />`
- Title shows `project?.name` from Zustand store

### Bug fixed
- TypeScript error: store `projects` has conflicting `Project` type — fixed by casting to `OrcaProject[]` with explicit import

---

## Acceptance Criteria

- [x] `src/renderer/src/components/project/ProjectSettings.tsx` created and compiles
- [x] `src/renderer/src/components/project/MemberManager.tsx` created and compiles
- [x] `ProjectSettings` renders a `Dialog` with 2 tabs: `General` and `Members`
- [x] `MemberManager` calls `projects.listMembers` RPC on mount
- [x] `MemberManager` calls `projects.updateMemberRole` on role change
- [x] `MemberManager` calls `projects.removeMember` on remove click
- [x] All `data-testid` attributes present as specified
- [x] No TypeScript errors (task scope)

---

## Output

```
ProjectSettings.tsx: CREATED — src/renderer/src/components/project/ProjectSettings.tsx
MemberManager.tsx: CREATED — src/renderer/src/components/project/MemberManager.tsx
TypeScript errors (task scope): 0
```
