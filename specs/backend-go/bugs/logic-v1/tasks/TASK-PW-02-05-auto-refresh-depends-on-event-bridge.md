# TASK-PW-02-05: File-explorer auto-refresh after agent completion (no git-gateway-service work — tracking task only)

**From Solution:** SOL-PW-02
**Priority:** P2 — tracking/coordination task, not an implementation task
**Service:** `api-gateway` (consumer already built elsewhere — see Depends on)
**File:** none (no file in this task's own scope)
**Depends on:** TASK-PW-04-07
**Status:** `[ ]` TODO

---

## Context

BUG-PW-02's second gap ("auto-refresh after agent complete never fires")
is **not fixed in `git-gateway-service`**. The frontend's
`workspace.refreshFileTree` channel is already real
(`backend-go/services/api-gateway/internal/adapter/wscompat/channels_git.go:946-975`)
— the only missing piece is something pushing a `workspace.event` WS frame
to trigger the frontend into calling it, which is exactly what
[SOL-PW-04](../solutions/SOL-PW-04-workspace-integration-event-bus.md)'s
`api-gateway` workspace-event bridge builds
(`TASK-PW-04-07-api-gateway-workspace-event-bridge.md`).

This task exists only to make the dependency explicit and trackable — it
has no code of its own. **Do not implement anything against this task
file directly**; close it once TASK-PW-04-07 ships and a manual/E2E check
confirms a task-status-change event triggers the frontend's
`workspace.refreshFileTree` call for a connected session.

## Changes to make

None. If, upon TASK-PW-04-07 landing, `workspace.refreshFileTree`'s
trigger still doesn't reach the frontend for the file-explorer case
specifically, re-open this task with the actual gap found (e.g. the
bridge fires a different frame type than the frontend's file-explorer
listener expects) rather than assuming SOL-PW-04's frame shape auto-maps 1:1.

## Verify

```bash
# No backend-go build/test verification for this task — it is a
# dependency-tracking placeholder. Verification happens transitively via
# TASK-PW-04-07's own Verify section plus a manual check:
#   1. Deploy TASK-PW-04-07.
#   2. Trigger a task status change that flows through task-service's
#      outbox (e.g. ExecuteTask completing).
#   3. Confirm a connected WS session receives a workspace.event frame and
#      the frontend's file explorer calls workspace.refreshFileTree.
```
