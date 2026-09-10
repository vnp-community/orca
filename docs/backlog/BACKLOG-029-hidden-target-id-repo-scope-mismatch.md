# BACKLOG-029: `hidden_target_id` never populated at repo-scope — likely a scope mismatch, not a missing field

**Origin:** `specs/backend-go/crs/v3/ephemeral-vm/tasks/TASK-BE-EVM-015-git-gateway-hidden-target-routing.md` ("Đính chính 2026-09-09" section), CR-EVM-005
**Priority:** Low — current behavior (empty `hidden_target_id` at repo-scope) is safe: it just falls back to normal `DevServerID`-only routing, no incorrect routing occurs
**Blocked on:** **A product/architecture decision** — the field may not apply at this call site at all, not an engineering gap
**Owner:** whoever owns CR-EVM-005 / git-gateway-service's repo-scoped dispatch (`dispatchExecutorForRepo`)

---

## What this is

`git-gateway-service`'s hidden-target routing mechanism (context threading
+ `RelayExecutor.relay()`'s "ViaHiddenTarget" dispatch) is fully
implemented and tested. It works correctly for the **worktree-scoped**
path (`dispatchExecutor`, via `ConnectionResolver.ResolveConnection`) —
confirmed real end to end as of TASK-BE-EVM-018.

The **repo-scoped** path (`dispatchExecutorForRepo`, used by
`CreateWorktree`/`DetectWorktrees`/`PrefetchCreateBase`/`ResolvePrBase`/
`ResolveMrBase` — everything that runs *before* a worktree exists) still
never gets a populated `hidden_target_id`: `project-service`'s `GetRepo`
handler doesn't set it, even though the proto field and the client-side
mapping are both already wired and ready.

## Why this probably isn't just "add the missing code"

`ResolveConnection`'s `HiddenTargetID` (the worktree-scoped path that
already works) is resolved via **`WorktreeID`/`WorkspaceID`** — one
specific workspace's ephemeral VM runtime, if any. But a repo can have
*multiple* worktrees, each potentially backed by a *different* ephemeral VM
runtime (or none). "The hidden target of repo X" isn't a well-defined
concept the way "the hidden target of worktree Y" is.

`dispatchExecutorForRepo`'s callers all run **before a worktree exists** —
at that point there's no specific worktree to ask "which ephemeral VM
runtime backs this" in the first place, because none has been created yet.

## The decision needed

1. Is there actually a use case where `CreateWorktree`/etc. (pre-worktree
   repo-scoped operations) need to route through a specific ephemeral VM
   hidden target? If not, `hidden_target_id` staying empty here is
   *correct*, not a gap — the backlog item should just be closed as "not
   applicable."
2. If yes, define what "the repo's hidden target" even means (a
   tenant/project-level default? A most-recently-used runtime?) before
   writing a cross-service join (`project-service` → `infra-fleet-service`'s
   `ephemeral_vm_runtimes`) to populate it — a real design question, not a
   lookup that was simply forgotten.

## What NOT to do

Don't guess a join and ship it — a wrong join could route a repo-scoped
operation to the *wrong* runtime's hidden target (worse than today's safe
empty-string fallback). This was deliberately left unresolved rather than
patched with an assumption.
