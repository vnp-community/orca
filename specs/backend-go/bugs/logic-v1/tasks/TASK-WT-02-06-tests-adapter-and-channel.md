# TASK-WT-02-06: Tests for the `fanout` adapters and `worktree.fanOut` channel

**From Solution:** SOL-WT-02
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_worktree_test.go`
**Depends on:** TASK-WT-02-03, TASK-WT-02-04
**Status:** `[ ]` TODO

---

## Context

Contract coverage for the gRPC-facing adapters and the wire-shape/error-surfacing of the new channel, per [SOL-WT-02](../solutions/SOL-WT-02-fan-out-worktree.md)'s Test plan.

## Changes to make

`backend-go/services/api-gateway/internal/adapter/fanout/agent_spawner_test.go` (new) — against fake `projectv1.ProjectServiceClient`/`infrafleetv1.InfraFleetServiceClient` (or a bufconn test server, matching whatever pattern this package's sibling adapters already use for gRPC-client contract tests): assert `SpawnAgentTerminal`'s two-hop resolution calls `GetProject` then `ResolveConnection` then `SpawnTerminalSession` in that order, and that `Shell` is populated via `agentLaunchCommand`.

`backend-go/services/api-gateway/internal/adapter/fanout/prompt_injector_test.go` (new) — asserts `InjectPrompt`'s frame sequence: an `Attach` frame sent before an `Input` frame, against a fake bidi-stream client.

`backend-go/services/api-gateway/internal/adapter/wscompat/channels_worktree_test.go` — add:
- `TestWorktreeFanOut_HappyPath` — fake `*usecase.FanOutCreateWorktrees`-shaped dependency (or construct the real usecase with fakes from [TASK-WT-02-05](./TASK-WT-02-05-tests-usecase.md), whichever this package's existing worktree-channel tests already prefer — check `channels_worktree_test.go`'s current fake style before choosing); assert the channel response is `{"items": [...]}` with every item's fields present.
- `TestWorktreeFanOut_NOutOfRange_ErrorSurfacesAsChannelError` — assert the channel returns a non-nil error (not a 200 with an empty `items` array) when the usecase rejects `FANOUT_N_OUT_OF_RANGE`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go test ./services/api-gateway/internal/adapter/fanout/... ./services/api-gateway/internal/adapter/wscompat/... -run "FanOut|WorktreeFanOut"
```

Expected: all cases pass.
