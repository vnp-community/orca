# BACKLOG-028: Ephemeral VM worktree mount/copy-out — spec is wrong, real feature needs a product decision

**Origin:** `specs/frontend/crs/v3/ephemeral-vm/tasks/FE-TASK-EVM-007-worktree-mount-decision.md` (FE-SOL-EVM-005) + `specs/agent/crs/v3/ephemeral-vm/tasks/TASK-AG-EVM-012-worktree-mount-decision.md` (SOL-AG-EVM-005), CR-EVM-009
**Priority:** Low — nothing is broken today; `docs/features/F18-ephemeral-vm.md` just describes a mechanism that was never built
**Blocked on:** **A product decision** between two options (below) — both tasks that investigated this were explicitly scoped "investigation only, do not merge/code"
**Owner:** whoever owns CR-EVM-009 / `docs/features/F18-ephemeral-vm.md`

---

## What this is

`docs/features/F18-ephemeral-vm.md` describes the local worktree being
"mounted" into the ephemeral VM and results "copied out" afterward. The
real, shipped implementation (`ephemeral-vm-workspace-target.ts`'s
`prepareEphemeralVmWorkspaceTarget`) does neither: it provisions the VM,
reads the project root the recipe's own `create` command produced *inside*
the VM, and points the workspace at that — `setupMethod:
'imported-existing-folder'`. No local worktree content is ever copied in,
and nothing is copied out.

## What the investigation found (2026-09-09, both frontend and agent sides)

Mount/copy-out only has technical meaning for `connection_type: 'ssh'` —
for `connection_type: 'orca-server'` (most real recipes today), the VM and
the agent share one filesystem, so there's no "gap" to bridge in the first
place. This materially narrows the real feature's scope if it's ever built.

## The decision needed

1. **Nhánh A — fix the spec.** Correct `docs/features/F18-ephemeral-vm.md:44-47`
   to describe what's actually shipped (workspace points at the recipe's
   own in-VM project root; no local↔VM sync happens; for `ssh` targets this
   is a known limitation, not a bug). Small, safe, no new code.
2. **Nhánh B — build the real feature**, scoped to `connection_type: 'ssh'`
   only: a "Start from an existing local worktree" option when creating an
   SSH-type ephemeral VM workspace, requiring (a) a post-provision upload
   step (agent-side write-to-hidden-target capability — confirmed a real
   gap, doesn't exist yet), and (b) a definition of "the VM's task is done"
   (shared dependency on BACKLOG-022's decision) before a copy-out trigger
   point can even be chosen. Estimated as a real feature-sized effort, not
   a small fix.

Both investigations lean toward Nhánh A unless there's a concrete near-term
need for "start from an existing local worktree" specifically on SSH
targets — but this is a recommendation, not an automatic choice.

## What unblocks this

A human picks Nhánh A or B. If B, it additionally depends on BACKLOG-022's
decision (needs the same "agent task finished" signal for the copy-out
trigger).
