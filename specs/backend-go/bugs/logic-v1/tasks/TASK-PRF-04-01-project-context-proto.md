# TASK-PRF-04-01: Add `GetProjectContext` RPC and `ProjectContext` message to `project.proto`

**From Solution:** SOL-PRF-04
**Priority:** P0 — every other task in this set needs generated stubs from this
**Service:** `project-service`
**File:** `backend-go/proto/orca/project/v1/project.proto`
**Depends on:** none
**Status:** `[x]` DONE — GetProjectContext RPC + GetProjectContextRequest/ProjectContext messages added; buf generate + breaking clean; full workspace build clean

---

## Context

This is the one BL-PRF-04 spec step (step 1, "Load Project") whose owning
RPC doesn't exist yet — `project-service.md` §2's Boundary decision already
names it exactly ("`project-service` exposes a read-only `GetProjectContext`
... Callers do a two-step saga: resolve context here, then call the
execution-owning service"), it was just never implemented. Both
`workflow-service` (TASK-PRF-04-05/06) and `task-service`
(TASK-PRF-04-07/08) call this RPC to build the agent-spawn env/preamble.

## Changes to make

In `backend-go/proto/orca/project/v1/project.proto`, add the RPC to the
`ProjectService` service block and the two new messages:

```protobuf
rpc GetProjectContext(GetProjectContextRequest) returns (ProjectContext);
```

```protobuf
message GetProjectContextRequest {
  string project_id = 1;
}

message ProjectContext {
  string project_id = 1;
  string project_name = 2;
  string description = 3;
  string repo_url = 4;          // from the project's primary Repo (position 0), if any
  string dev_server_id = 5;
  string dev_server_hostname = 6; // resolved via infra-fleet-service, best-effort — empty if unresolvable
}
```

Check the current highest field number in `Project`/other request messages
before assigning `1-6` above — this task's numbers assume a fresh message
with no prior fields, so no renumbering conflict is expected, but confirm
against the live file.

## Verify

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
go build ./proto/...
```

Expected: clean build; `buf breaking` reports only additions (new RPC, new
messages).
