# TASK-023: Document `accounts.*`'s agent-side gap and frontend `connectionId` prerequisite (blocked, no code)

**From Solution:** SOL-004
**Priority:** P3 — tracking only, blocks nothing in this repo's `backend-go` build
**Service:** `agent` (out of scope) / `frontend` (out of scope)
**File:** none — this task produces no code change; see "What to do" below
**Depends on:** TASK-021
**Status:** `[ ]` TODO

---

## Context

TASK-021 makes `accounts.*` relay correctly through
`infra-fleet-service`'s existing `Relay` RPC, but per SOL-004 two
prerequisites outside `backend-go`'s scope must land before those 4
channels do anything but fail:

1. **Agent-side companion work (small).** `Relay` only forwards
   `method`/`params` to whatever the Dev Server Agent's JSON-RPC dispatcher
   already implements. `accounts.selectClaude`/`accounts.selectCodex`/
   `accounts.removeClaude`/`accounts.removeCodex` do not exist as JSON-RPC
   methods on the agent yet — until they do, TASK-021's handlers return a
   "method not found" error from the agent's own dispatcher. Per
   `specs/backend-go/tdd/architecture/08-inter-service-communication.md`'s
   "Talking to the Dev Server Agent" section, `agent/` changes are
   explicitly out of scope for the Go rewrite of `backend/`. SOL-004 notes
   this is comparatively low-risk agent-side work: filesystem read/write
   against a known `~/.claude`/`~/.codex` config path, well within the
   agent's existing filesystem capability — not a new execution-plane
   capability class (contrast SOL-006's browser-driving gap, TASK-036).

2. **Frontend `connectionId` prerequisite.** `Relay` requires a
   `connectionId`, but every documented `accounts.*` call site passes only
   `{ accountId }`. `runtime-provider-accounts-client.ts` already calls
   `getActiveRuntimeTarget(settings)` client-side before each of these 4
   calls — the natural fix is threading that already-resolved
   environment's `connectionId` into the RPC params alongside `accountId`.
   This is a `frontend/` change, not a `backend-go` one:
   `wscompat`'s `Identity` (api-gateway/internal/usecase) carries only
   `TenantID`/`UserID`, with no session-scoped connection to derive a
   `connectionId` from, and inventing a resolution heuristic (e.g. "the
   tenant's only connection") would silently break multi-environment
   tenants.

This task exists so both gaps are tracked explicitly rather than
discovered later as "accounts.* still doesn't work after TASK-021/022
shipped" — it produces no `backend-go` diff.

---

## What to do

Not a code change. File (or link) two tracking issues:

1. **agent-side:** "Implement `accounts.selectClaude`/`selectCodex`/
   `removeClaude`/`removeCodex` JSON-RPC methods on the Dev Server Agent" —
   scope: read/write the Claude/Codex CLI's own login config file(s) on the
   agent's host, matching whatever shape `~/.claude`/`~/.codex`'s
   credentials file already has. Link this TASK file and SOL-004's "Agent-
   side companion work" section as the design reference.
2. **frontend-side:** "Thread `connectionId` (from `getActiveRuntimeTarget`)
   into `accounts.*`'s 4 RPC call params" in
   `runtime-provider-accounts-client.ts`. Link this TASK file and SOL-004's
   "Open prerequisite" section.

Also note for whoever picks up either: SOL-004 separately flags
`accounts.subscribe` (a streaming variant) as explicitly out of scope for
its own 4-method list — track that as a third, later item if/when it's
picked up, not part of either issue above.

---

## Verify

N/A — no code produced by this task. "Done" means both tracking issues
exist and are linked from this file (or from wherever the project tracks
issues), so TASK-021/TASK-022's shipped-but-inert state is discoverable.
