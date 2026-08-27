# TASK-AT-01-06: REST parity — mount `List`/`Update`/`Delete` automation routes

**From Solution:** SOL-AT-01
**Priority:** P2
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/httpgateway/automation_routes.go`
**Depends on:** TASK-AT-01-01
**Status:** `[ ]` TODO

---

## Context

`automation_routes.go` mounts only create/run/runs/trigger. `ListAutomations`,
`UpdateAutomation`, and `DeleteAutomation` are real gRPC methods with no REST
route. This task adds the three missing hand-written handlers, following the
exact shape already used by `handleCreateAutomation`/`handleRunNow`.

## Changes to make

In `automation_routes.go`, add three routes to `mountAutomationRoutes`:

```go
sub.Get("/", handleListAutomations(client))
sub.Patch("/{id}", handleUpdateAutomation(client))
sub.Delete("/{id}", handleDeleteAutomation(client))
```

Add the three handler functions, each following
`handleCreateAutomation`/`handleRunNow`'s existing shape exactly:
`identityFromContext` → `AttachIdentity` → gRPC call → `writeJSON`/
`writeGRPCError`. Read the current `handleCreateAutomation` and
`handleRunNow` implementations in this file first and copy their pattern
verbatim for request parsing (path param `{id}` via `chi.URLParam` or this
router's equivalent), query-param parsing for `ListAutomations`'
pagination, and JSON body decoding for `UpdateAutomation`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/...
go test ./services/api-gateway/internal/adapter/httpgateway/... -run TestAutomationRoutes
```

Expected: new List/Update/Delete handlers round-trip against a fake
`AutomationServiceClient` in `automation_routes_test.go`, matching the
existing four handlers' test shape.
