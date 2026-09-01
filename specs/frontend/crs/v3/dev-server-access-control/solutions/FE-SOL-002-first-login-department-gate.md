# FE-SOL-002: First-Login Department Selection Gate

**CR:** [CR-DS-008](../../../../../docs/crs/v2/dev-server/CR-DS-008-first-login-department-gate-and-access-request.md) §2.1
**Status:** ✅ Completed — 2026-08-28
**Files:** `frontend/src/renderer/src/components/DepartmentGate.tsx` (new),
`frontend/src/renderer/src/App.tsx` (+2 lines: import + mount),
`frontend/src/shared/tenant-user-profile-types.ts` (new),
`frontend/src/preload/api-types.ts` / `frontend/src/renderer/src/web/web-preload-api.ts`
(new `tenantProfile` namespace)

---

## Resolved: "does admin get gated too"

**Admin bypasses the gate entirely** — same direction as FE-SOL-003's admin
skip-onboarding bypass. Admins manage the org (approve agents, assign
groups, grant department access) from the Admin Console; requiring them to
first pick a department they may not belong to would block the very console
that assigns everyone else's department. Reversible default, documented in
code (`DepartmentGate.tsx`'s `isAdmin` comment).

## Implementation note: sibling overlay, not a literal wrap

Rather than wrapping `App.tsx`'s ~2000-line return tree in a
`<DepartmentGate>` JSX wrapper (high-risk diff against a file with no
existing test coverage for that render path), `DepartmentGate` is mounted as
a `position:fixed inset-0 z-[200]` sibling near the top of the render tree —
the same pattern already established by `AgentHibernationGate`/
`RetainedAgentsSyncGate` (leaf components, internally decide whether to
render anything). Its z-index is higher than `OnboardingFlow`'s (`z-100`),
so when the gate is active it visually and interactionally blocks
onboarding and the rest of the app the same way `OnboardingFlow` itself
already blocks everything below it without wrapping it. Functionally
equivalent to the originally-sketched wrap; zero risk to `App.tsx`'s
existing render logic.

## Behavior

- Fetches `tenantProfile.getUserProfile()` once (skipped entirely for
  admins). Empty `departmentId` → fetches `tenantProfile.listDepartments()`
  and renders the blocking screen (shadcn `Select` + `Button`, styled to
  match `OnboardingFlow`'s own overlay treatment).
- Submit calls `tenantProfile.setUserDepartment({ departmentId })`
  (wraps the pre-existing `profile.updateUser` channel) then clears the gate.
- **Fails open** on a profile-fetch error (logs to console, does not block)
  — a transient network hiccup must not brick the whole app for an
  already-onboarded user.

## Prerequisite fix landed alongside this

`profile.getUserProfile` / `profile.listDepts` / `profile.updateUser` in
`channels_tenant_project.go` were returning raw `*tenantv1.UserProfile`/
`Department` proto messages — `encoding/json`'s snake_case struct tags
(`json:"department_id,omitempty"`) meant every field this gate needs
(`departmentId`, `companyId`, etc.) shipped as `undefined` to the frontend.
Fixed via camelCase view structs (`userProfileView`/`departmentView`),
same pattern as `channels_dev_server_access_control.go`'s own fix earlier
this session. `profile.updateUser`'s `userId` was also made optional
(defaults to the caller's own id via `cmp.Or`), matching
`profile.getUserProfile`'s existing contract — the gate calls it without
knowing its own user id. Regression-tested in
`channels_tenant_project_test.go`'s `TestProfileChannels_EmitCamelCaseJSON`
(marshals the actual dispatch result and asserts camelCase keys / absence
of snake_case keys).

---

## Goal

A gate component wrapping the app shell (above `OnboardingFlow`, not inside
it — must block *everything*, including onboarding itself, per the original
request: "phải xác nhận vào phòng ban này trước khi được hiển thị onboarding
hoặc sử dụng các tính năng của orca").

```
AppShell
  └─ DepartmentGate                    ← NEW, wraps everything below
       ├─ (hasDepartment === null) → loading spinner
       ├─ (hasDepartment === false) → DepartmentSelectionScreen  ← NEW
       └─ (hasDepartment === true) → OnboardingFlow / rest of app (unchanged)
```

- `DepartmentSelectionScreen`: fetch tenant's department list, single-select,
  submit calls tenant-service's `SetUserDepartment` (already exists — no new
  backend RPC needed for this specific screen, unlike FE-SOL-001/003).
- Loading state must not flash the app shell before the check resolves —
  same "wait before rendering" discipline `WebRootBoundary`'s `/auth/me`
  check already follows (see `backend-go/docs/execution-plan.md`'s account
  of that exact bootstrap-ordering bug class).

## Open decision this depends on

CR-DS-008 §3.1: does an admin also see this gate, or bypass it? Do not
implement until answered — the gate's top-level branch condition changes
shape depending on the answer (`hasDepartment === false` vs
`hasDepartment === false && role !== 'admin'`).
