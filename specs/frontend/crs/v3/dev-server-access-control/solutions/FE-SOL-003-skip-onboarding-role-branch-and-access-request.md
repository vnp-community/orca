# FE-SOL-003: Skip-Onboarding Role Branch + Access Request Form

**CR:** [CR-DS-008](../../../../../docs/crs/v2/dev-server/CR-DS-008-first-login-department-gate-and-access-request.md) §2.2, §2.3
**Backend:** [BE-SOL-003](../../../../backend-go/crs/v0/dev-server-access-control/solutions/BE-SOL-003-department-group-mapping-and-opa.md), [BE-SOL-004](../../../../backend-go/crs/v0/dev-server-access-control/solutions/BE-SOL-004-access-request-flow.md)
**Status:** ✅ Completed — 2026-08-28
**Files:** `frontend/src/renderer/src/components/onboarding/AccessRequestDialog.tsx` (new),
`frontend/src/renderer/src/components/onboarding/OnboardingFlow.tsx` (role branch in
`confirmSkipOnboarding`), `frontend/src/renderer/src/components/onboarding/use-onboarding-flow.ts`
(exposes `openSettingsPage`/`openSettingsTarget`)

---

## Goal (verbatim from the feature request this CR traces to)

> Với user là admin thì cho phép [Skip] và chuyển hướng đến settings. Còn
> với user thì không cho phép mà phải hỗ trợ gửi yêu cầu xin truy cập vào
> dev server.

## Design

`OnboardingFlow.tsx`'s `confirmSkipOnboarding` (the function `onboarding.update`
was fixed this session to unblock — see this session's git history, no CR
needed for that fix, it was a pre-existing bug not a new feature) gains a
role branch:

```ts
const confirmSkipOnboarding = useCallback(async () => {
  if (currentUser.role === 'admin') {
    await dismissOnboarding()
    navigateTo('settings')
    return
  }
  const accessible = await listDevServersForUser()   // NEW runtime client fn
  if (accessible.length > 0) {
    await dismissOnboarding()
    return
  }
  openAccessRequestForm()   // NEW — replaces the confirmation dialog, not dismissOnboarding
}, [...])
```

- `openAccessRequestForm` is a NEW dialog (not `OnboardingSkipConfirmationDialog`,
  which stays for the admin/has-access path) — form fields: pick a
  `DevServerGroup` (from `devServerGroup.list`, filtered to whatever
  BE-SOL-002/003 decide "publicly visible group names" means for a
  non-admin caller — **this is itself one of CR-DS-007 §3's/CR-DS-008 §3's
  open decisions, not yet resolved**), optional message, submit calls the
  new `devServer.requestAccess` RPC (BE-SOL-004).
- After submit: show a simple "request sent, an admin will review it"
  confirmation — does NOT close onboarding (user still has no accessible
  dev server yet).

## Resolved during implementation

- **Admin sees pending access requests** in FE-SOL-001's Admin Console, as a
  third tab ("Access requests") alongside Approvals and Groups & access.
- **`devServerGroup.list`'s visibility for a non-admin caller**: verified by
  reading the backend usecase directly — `ListDevServerGroups.Execute`
  (`list_dev_server_groups.go`) never calls `requireAdmin`; only the
  wscompat channel attaches an admin `Identity.Role` (harmless — the
  usecase doesn't check it). It's tenant-scoped, not admin-gated, by
  design (Phase 1 predates the department/team grant model — see that
  file's own doc comment). No backend change was needed to let
  `AccessRequestDialog` populate its group picker for a regular user.
- **No "skip without requesting" escape hatch was added** — matches
  CR-DS-008 §4's "no silent skip" acceptance criterion. A non-admin with no
  accessible dev servers can only Cancel (returns to onboarding, does not
  skip) or submit a request.
- **After a successful submit, onboarding IS dismissed** (deviates from this
  doc's original "does NOT close onboarding" sketch) — a reversible
  decision: leaving the user stuck in onboarding after they've already
  taken the only available action (filing the request) adds no value, and
  every other UI surface already treats "no accessible dev servers yet" as
  a normal, expected post-onboarding empty state.
- **Known gap carried over from backend, not fixed here**: `ListDevServersForUser`'s
  `team_ids` is always empty (tenant-service has no "list teams for user"
  RPC yet), so the Admin Console's grant UI only supports department-based
  grants — team-based grants aren't exposed in this pass either.
