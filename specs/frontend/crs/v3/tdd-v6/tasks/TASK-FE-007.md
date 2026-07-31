# TASK-FE-007: Verify useProfile RPC Method Names

**Task ID:** TASK-FE-007
**Phase:** 1 — Core Fixes
**Priority:** P1
**Solution Ref:** SOL-FE-V6-001 (Section 2.3)
**Estimated effort:** 20 minutes
**Dependencies:** None
**Status:** ✅ COMPLETED — 2026-07-30

---

## Execution Results

### Audit of `useProfile.ts`

Read `src/renderer/src/hooks/useProfile.ts` in full. All RPC method names were ALREADY CORRECT:

| Operation | Method in code | Expected | Result |
|-----------|---------------|----------|--------|
| Get resolved | `profile.getResolved` | `profile.getResolved` | ✅ CORRECT |
| Get user | `profile.getUser` | `profile.getUser` | ✅ CORRECT |
| Save user | `profile.updateUser` | `profile.updateUser` | ✅ CORRECT |
| Save company | `profile.updateCompany` | `profile.updateCompany` | ✅ CORRECT |
| Save dept | `profile.updateDept` with `{ deptId: scopeId, profile }` | `profile.updateDept` | ✅ CORRECT |

### Additional behavior checks

**On mount** — Both `profile.getResolved` and `profile.getUser` fetched in `Promise.all()` ✅

**After user save** — `profile.getResolved` re-fetched and result stored via `store.setResolved(resolved)` ✅

**Dept save** — uses `scopeId` as `deptId`: `{ deptId: scopeId, profile }` ✅

**Error handling** — `toast.error(err?.message ?? 'Failed to save profile')` and rethrows ✅

### No changes needed

`useProfile.ts` fully complies with HLD C4.7. **Zero modifications required.**

---

## Acceptance Criteria

- [x] `profile.updateUser` is used when `scope === 'user'`
- [x] `profile.updateCompany` is used when `scope === 'company'`
- [x] `profile.updateDept` with `deptId` is used when `scope === 'dept'`
- [x] After saving user scope: `profile.getResolved` is called to refresh store
- [x] `profile.getUser` and `profile.getResolved` called on mount
- [x] No TypeScript errors

---

## Output

```
profile.getUser: CORRECT (no change)
profile.getResolved: CORRECT (no change)
profile.updateUser: CORRECT (no change)
profile.updateCompany: CORRECT (no change)
profile.updateDept (with deptId as scopeId): CORRECT (no change)
Re-fetch resolvedProfile after user save: YES (already implemented)
TypeScript errors: 0
```
