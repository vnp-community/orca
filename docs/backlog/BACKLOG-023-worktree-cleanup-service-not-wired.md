# BACKLOG-023: `WorktreeCleanupService` built and tested, deliberately NOT wired to auto-run — needs explicit go-ahead

**Origin:** `specs/frontend/crs/v4/automation/tasks/FE-TASK-AUTO-008-worktree-cleanup-service-decision.md` (FE-AUTO-SOL-006, CR-AUTO-006)
**Priority:** Medium-High — once wired, this runs unattended and deletes real user worktrees; low priority to build further until someone explicitly wants the feature live
**Blocked on:** **Explicit user/product authorization** to enable auto-deletion — not an engineering gap
**Owner:** whoever owns `desktop/src/main`'s composition root / worktree lifecycle policy

---

## What this is

`desktop/src/main/automations/WorktreeCleanupService.ts` implements
scheduled worktree cleanup (age/status-based, `git worktree remove --force`)
for repos with no other cleanup mechanism today (confirmed 2026-09-09:
`removeWorktree` in `frontend/src/renderer/src/store/slices/worktrees.ts` is
the only cleanup path live today, and it's fully manual, per-worktree,
user-clicked — no `autoCleanup`/`autoArchive`/staleness setting exists).

## Why it stops short of being wired in

`runCleanup()` deletes worktrees **without a per-instance confirmation**,
on a schedule, unattended. That's a hard-to-reverse action against real
user data. The task doc that originally scoped this listed "wire into
composition root" as a valid branch to take — but that's not the same as
explicit authorization from a human to turn on automatic deletion for
real users. Per this repo's own safety principle (confirm before
hard-to-reverse actions unless already explicitly authorized for that
specific action), the task stopped short of flipping it on.

## What's actually done vs. not

- **Done, safe, no side effects:** `WorktreeCleanupService.test.ts` — 7
  test cases (uncommitted-changes guard via `git status --porcelain`,
  correct deletion when clean+aged+status-matched, skip-on-error
  conservative behavior, `onCleanupComplete` callback correctness, etc.).
  All pass, no real worktrees touched.
- **Not done, on purpose:** initializing `WorktreeCleanupService` in
  `desktop/src/main`'s real composition root, which would make it start
  deleting real worktrees on the next build.

## What unblocks this

A human explicitly says "yes, enable scheduled auto-deletion of worktrees"
— at which point wiring it in is a small, low-risk change (the service and
its tests already exist and pass). Also worth deciding then: what config
surface (a `GlobalSettings` field?) lets a user opt in/out and tune the
age/status thresholds, since the task doc's "Phương án B" sketch assumes
that config exists but doesn't specify its shape.
