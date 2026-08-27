# TASK-AG-02-06: [OPEN DESIGN DECISION] BR-AG-06 write-lock check — build a `WriteActivityTracker` or accept the fail-open default?

**From Solution:** SOL-AG-02
**Priority:** P2 — optional hardening; `KillAgentSession` (TASK-AG-02-03) already degrades safely without this
**Service:** `git-gateway-service` (if built) + `infra-fleet-service` (caller)
**File:** `backend-go/services/git-gateway-service/internal/usecase/ports.go` (if option 1 is chosen)
**Depends on:** TASK-AG-02-03
**Status:** `[ ]` TODO — needs a product decision before implementation, not an inferred default

---

## Context

No write-lock concept exists anywhere in backend-go today. `KillAgentSession` (TASK-AG-02-03) accepts a `WriteActivityChecker` port but runs correctly with it `nil` (fail-open). This task exists to force an explicit decision rather than let BR-AG-06 quietly stay unimplemented forever, per SOL-AG-02's own framing.

## Changes to make

This task is a decision record, not a committed design. Present the two
options below to the product/eng owner and record the outcome (edit this
task's `Status` line to reflect the decision) before writing code:

**Option 1 — best-effort in-memory tracker in `git-gateway-service`.** A
`WriteActivityTracker` increments/decrements an in-process counter keyed by
`worktree_id` around every `WriteFile`/`WriteFileChunk` dispatch, exposed
via a new lightweight RPC `HasInFlightWrite(worktree_id) returns (bool)`.
`KillAgentSession` calls it before killing. **Adds a new `infra --> git`
edge** to `02-microservices-decomposition.md`'s dependency graph, alongside
the existing `git --> infra` edge — a two-way service relationship the
current graph doesn't have. Best-effort by construction: a pod restart of
`git-gateway-service` loses in-flight counts (fails open by design, never
blocks a kill it can't prove is unsafe).

If chosen, implement as:

```go
// git-gateway-service/internal/usecase/ports.go
type WriteActivityTracker interface {
	BeginWrite(worktreeID string)
	EndWrite(worktreeID string)
	HasInFlightWrite(worktreeID string) bool
}
```

plus a new `HasInFlightWrite` RPC on `GitGatewayService` and an
`infra-fleet-service` grpcclient adapter implementing
`usecase.WriteActivityChecker` (TASK-AG-02-03) against it.

**Option 2 — don't build it.** Rely on the 10s grace period (SOL-AG-02's
`[A1]`) plus explicit user confirmation before a force-kill. Most file
writes are near-atomic OS `write()` calls, not multi-step transactions, so
the corruption window SIGKILL introduces is small. Avoids new cross-service
coupling entirely, at the cost of not literally satisfying BR-AG-06.

## Verify

No build/test target for this task itself — it gates whether TASK-AG-02-03's
`nil` `WriteActivityChecker` argument at `cmd/server/main.go` is later
replaced with a real implementation. If option 1 is chosen, its own
follow-up task(s) get their own `Verify` sections (new RPC → `buf generate`
+ `go build`; new tracker → unit test asserting `BeginWrite`/`EndWrite`
paired correctly under concurrent access).
