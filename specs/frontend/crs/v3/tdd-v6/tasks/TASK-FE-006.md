# TASK-FE-006: Create DeptProfileAdmin Component

**Task ID:** TASK-FE-006
**Phase:** 2 — New Components
**Priority:** P2
**Solution Ref:** SOL-FE-V6-001 (Section 2.1)
**Estimated effort:** 30 minutes
**Dependencies:** None (TASK-FE-005 confirmed ProfileEditor.tsx correct)
**Status:** ✅ COMPLETED — 2026-07-30

---

## Execution Results

### Pre-checks
- ✅ `ProfileEditor.tsx` accepts `scope: 'user' | 'dept' | 'company'` and `scopeId?: string`
- ✅ `Department` type in `profile-types.ts` has `id`, `name`, `leadId?`, `memberCount?` (added by TASK-FE-005)
- ✅ `Badge`, `Skeleton` available in `components/ui/`

### File created
**`src/renderer/src/components/profile/DeptProfileAdmin.tsx`**

Key behaviors:
- `useEffect` on mount → `callRuntimeRpc(target, 'profile.listDepts', {})` via correct target
- Loading state: 3 Skeleton placeholders (`data-testid="dept-profile-loading"`)
- Empty state: message with `data-testid="dept-profile-empty"`
- Department badge selector: `variant="default"` for active, `"outline"` for others
- Each badge: `data-testid={dept-badge-${dept.id}}` with optional memberCount display
- On badge click: sets `activeDeptId` → renders `<ProfileEditor scope="dept" scopeId={activeDeptId} />`
- No-selection edge case: `data-testid="dept-no-selection"` message

### AdminApp.tsx
- EXISTS at `src/renderer/src/components/admin/AdminApp.tsx` — **wiring SKIPPED** per task (optional, low priority)

### TypeScript errors (task scope): **0**

---

## Acceptance Criteria

- [x] `DeptProfileAdmin.tsx` created at correct path
- [x] Shows loading skeleton while fetching departments
- [x] Shows empty state when no departments
- [x] Badge selector renders one badge per department
- [x] Clicking a badge sets that dept as active
- [x] Active dept badge has `variant="default"` (highlighted)
- [x] `ProfileEditor scope="dept" scopeId={activeDeptId}` renders when dept selected
- [x] Shows "no selection" text when no dept selected (edge case)
- [x] All `data-testid` attributes present
- [x] No TypeScript errors

---

## Output

```
DeptProfileAdmin.tsx: CREATED — src/renderer/src/components/profile/DeptProfileAdmin.tsx
ProfileEditor scopeId prop: ALREADY EXISTS (added by TASK-FE-005)
AdminApp.tsx wired: SKIPPED (exists but optional — low priority)
TypeScript errors: 0
```
