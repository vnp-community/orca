# TASK-TG-04-08: [BLOCKED / FUTURE SCOPE] PTY output streaming — needs `agent/` + `infra-fleet-service` changes, not `task-service`-only

**From Solution:** SOL-TG-04
**Priority:** P2 — documented and scoped out, not silently dropped
**Service:** `agent/` (Dev Server Agent), `infra-fleet-service`, `task-service` (all three, cross-repo)
**File:** none — this task produces no code change; it is a scope record
**Depends on:** TASK-TG-04-03 (the synchronous `SimpleExecutor.Execute` this gap sits on top of)
**Status:** `[ ]` TODO — **blocked**: cannot be completed as a `task-service`-only change; requires its own design pass once prioritized

---

## Context

`BUG-TG-04`'s spec asks for streaming PTY output into a Task Activity Feed
over WebSocket while a simple-path execution is running. This CANNOT be
closed with `task-service`-only changes, and must not be silently dropped
from tracking just because it's out of this bug set's backend-go-only
scope. This task exists to record that explicitly, as a blocked/future-scope
entry, per SOL-TG-04's own explicit framing:

> "This part of the spec cannot be closed with task-service-only changes."

`agent.execPrompt`'s real, already-documented contract
(`simple_executor.go:92-98`) is fully synchronous: one request, one final
`{stdout, stderr, exitCode, timedOut}` response, no incremental delivery.
Achieving the spec's streaming requirement needs work in THREE places, only
one of which (`task-service`) is in this bug set's repo/scope:

1. **`agent/` (Dev Server Agent)** — a new capability that emits
   incremental output: either a new streaming RPC method alongside
   `agent.execPrompt`, or push notifications over the existing relay
   connection keyed by `stepId`. Genuinely new agent-side work — not a
   `backend-go`-only gap. `agent/`'s current `agent.execPrompt` handler
   (`agent-print-mode-exec.ts`) has no incremental-output concept to
   extend; this is new surface, not a wiring fix.

2. **`infra-fleet-service`** — needs a server-streaming gRPC endpoint to
   carry those chunks from the relay connection to `task-service` (or
   directly to `api-gateway`), analogous to the terminal-data streaming
   endpoint `infra-fleet-service.md` already describes for PTY sessions
   ("a dedicated server-streaming RPC once the route is resolved,"
   `infra-fleet-service.md:363-366`). `SimpleExecutor` would need to switch
   from a unary `Relay` call to consuming that stream.

3. **`task-service`** — republish received chunks as `task.agent_output`
   events (outbox → NATS → `api-gateway` WS push), the `TaskActivityEvent`
   union BUG-TG-04 names in full. This third piece IS in `task-service`'s
   scope, but is meaningless without (1) and (2) landing first — there is
   nothing to republish yet.

None of this is designed further here — it is a multi-service, cross-repo
change with its own tradeoffs (buffering, backpressure, partial-output
persistence) that deserves its own design pass once prioritized, per
SOL-TG-04's explicit deferral. Do not attempt to implement a partial version
of this (e.g. polling `agent.execPrompt` mid-flight, or fabricating
incremental chunks from the synchronous response) as a substitute — that
would misrepresent the feature as done when the underlying agent-side
capability doesn't exist.

## Changes to make

None. This task is a scope record, not an implementation. When this is
picked up for real:

1. Write a new design doc/solution scoped to `agent/` + `infra-fleet-service`
   + `task-service` together (not a `backend-go`-only SOL — this genuinely
   needs an `agent/`-repo design pass first).
2. Confirm whether `infra-fleet-service.md`'s existing terminal-data
   streaming RPC sketch (cited above) can be reused/extended for
   `agent.execPrompt` output, or needs a parallel, distinct streaming path.
3. Only then does `task-service`'s `task.agent_output` outbox-republish
   piece (#3 above) become implementable — it has no prerequisite from this
   bug set otherwise blocking it structurally, but has no real chunks to
   republish until (1)/(2) exist.

## Verify

Not applicable — no code change. If this task is ever marked done without
`agent/` and `infra-fleet-service` changes landing first, that is itself a
bug: re-open it.
