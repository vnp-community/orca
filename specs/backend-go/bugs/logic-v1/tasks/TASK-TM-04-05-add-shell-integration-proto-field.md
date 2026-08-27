# TASK-TM-04-05: Add `shell_integration` field to `SpawnTerminalSessionRequest`

**From Solution:** SOL-TM-04
**Priority:** P0 — generated stubs from this task back TASK-TM-04-06/07
**Service:** `infra-fleet-service` (proto is shared, `backend-go/proto/`)
**File:** `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`
**Depends on:** none
**Status:** `[x]` DONE — shell_integration (field 6) added to SpawnTerminalSessionRequest; `buf generate` regenerated stubs, `go build ./proto/...` clean. `buf breaking` against main not runnable in this worktree (see TASK-TM-03-03's note); verified additive-only by diff.

---

## Context

BR-TM-13's opt-in flag is backend-go's only real contribution to this
solution: a plain pass-through boolean, structurally identical to how
`Shell` itself already rides `SpawnTerminalSessionRequest` unexamined.
Additive-only field (field number 6), defaults to `false` — existing
callers that don't set it see no behavior change, so `buf breaking` stays
clean.

## Changes to make

In `backend-go/proto/orca/infrafleet/v1/infrafleet.proto`, extend
`SpawnTerminalSessionRequest`:

```protobuf
message SpawnTerminalSessionRequest {
  string connection_id = 1;  // empty = host-local; rejected in server-deployment mode, see usecase.SpawnTerminalSession
  string cwd = 2;
  string shell = 3;          // optional; agent applies its own default if empty
  int32  cols = 4;
  int32  rows = 5;
  // BR-TM-13 — opt-in shell-integration bootstrap (OSC 133). Forwarded
  // unexamined to the agent's pty.create; see SOL-TM-04. Defaults to
  // false — existing callers that don't set it see no behavior change.
  bool   shell_integration = 6;
}
```

## Regenerate stubs

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./proto/...
```

Expected: clean build, `buf breaking` reports no breaking changes (only an
additive field).
