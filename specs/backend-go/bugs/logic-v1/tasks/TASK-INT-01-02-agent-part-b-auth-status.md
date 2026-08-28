# TASK-INT-01-02: Add `github.auth.status`/`gitlab.auth.status` to Part B's relay-ssh dispatcher

**From Solution:** SOL-INT-01
**Priority:** P2
**Service:** `agent` (TypeScript, `agent/src/relay/` — NOT a backend-go service; flagged explicitly per SOL-INT-01's "Agent changes needed" section, included here because the solution calls it out as a required, if small, companion change)
**File:** `agent/src/relay/relay.ts` (or wherever `RelayDispatcher` registers its method table — confirm exact file before editing)
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

`infra-fleet-service.md` §10 flags that the agent process runs two
independently-implemented RPC surfaces (Part A: direct-websocket/
relay-websocket via `agent-rpc-dispatch.ts`; Part B: relay-ssh via
`relay.ts`'s `RelayDispatcher`/`preflight-handler.ts`) that frequently
diverge in method names. `github.auth.status`/`gitlab.auth.status` are
implemented only in Part A (`agent-rpc-dispatch.ts:1029-1047`) — a
repo-wide check confirms zero occurrences in Part B. This means
relay-ssh Dev Servers cannot answer either call today; until this task
lands, `TASK-INT-01-01`'s channels get a typed
`domain.ErrAgentMethodNotFound` for those Dev Servers, degraded honestly by
`SOL-INT-03`'s merge layer (not a silent wrong answer).

## Changes to make

In `agent/src/relay/relay.ts`, locate `RelayDispatcher`'s method
registration table (`grep -n "RelayDispatcher\|register" agent/src/relay/relay.ts`)
and add two cases delegating to the SAME handlers Part A already uses —
`external-api-connector.ts`'s `handleGitHubAuthStatus`/
`handleGitLabAuthStatus` are already transport-agnostic (imported by Part
A's dispatcher today; nothing in their signature ties them to Part A):

```ts
// relay.ts — RelayDispatcher's method table, alongside its existing
// preflight.check / git.* entries. Reuses agent-rpc-dispatch.ts's
// handlers verbatim — see external-api-connector.ts's
// handleGitHubAuthStatus/handleGitLabAuthStatus, already imported by Part
// A. This closes the Part A/Part B divergence infra-fleet-service.md §10
// flags by name — see SOL-INT-01.
'github.auth.status': async (params) => handleGitHubAuthStatus(params.userId, config),
'gitlab.auth.status': async (params) => handleGitLabAuthStatus(params.userId, config),
```

Confirm the exact handler signatures (`handleGitHubAuthStatus`/
`handleGitLabAuthStatus`'s actual parameter list in
`external-api-connector.ts:359-390`) and `RelayDispatcher`'s actual
registration mechanism (a plain object literal vs. a `.register()` call)
before pasting — adjust the snippet's shape to match exactly, this is
copy-adjacent reuse of already-tested code, not new logic.

## Verify

```bash
cd /opt/repos/orca/agent
npx tsc --noEmit
npx vitest run src/relay/relay.test.ts src/relay/agent-connection-stdio.test.ts
grep -rn "github.auth.status\|gitlab.auth.status" src/relay/
```

Expected: clean typecheck; existing relay-ssh tests stay green; the `grep`
now shows both Part A (`agent-rpc-dispatch.ts`) and Part B (`relay.ts`)
registering the two methods. Add a relay-ssh-mode test asserting
`github.auth.status`/`gitlab.auth.status` resolve through
`RelayDispatcher` and produce the same `{ok, stdout, stderr}` shape Part A
already returns.
