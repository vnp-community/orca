# TASK-FLEET-01-06: Wire `ImportFleetInventory` into the gRPC server

**From Solution:** SOL-FLEET-01
**Priority:** P1
**Service:** `infra-fleet-service` (grpc adapter)
**File:** `backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go`
**Depends on:** TASK-FLEET-01-01, TASK-FLEET-01-05
**Status:** `[ ]` TODO

---

## Context

The proto RPC and the usecase both exist after prior tasks; this task is the
thin gRPC handler that maps between them, following the existing
`CreateSshTarget`/`ListSshTargets` handler shape already in this file.

## Changes to make

```go
// internal/adapter/grpc/server.go
func (s *Server) ImportFleetInventory(ctx context.Context, req *infrafleetv1.ImportFleetInventoryRequest) (*infrafleetv1.ImportFleetInventoryResponse, error) {
    servers := make([]usecase.FleetServerInput, 0, len(req.GetServers()))
    for _, sv := range req.GetServers() {
        servers = append(servers, usecase.FleetServerInput{
            Host: sv.GetHost(), UserName: sv.GetUser(), VaultSSHRole: sv.GetVaultSshRole(),
            Project: sv.GetProject(), Tags: sv.GetTags(),
        })
    }
    result, err := s.importFleetInventory.Execute(ctx, usecase.ImportFleetInventoryInput{Servers: servers, DryRun: req.GetDryRun()})
    if err != nil {
        return nil, mapAppError(err) // existing error-mapping helper in this file
    }
    resp := &infrafleetv1.ImportFleetInventoryResponse{
        Imported: int32(result.Imported), Updated: int32(result.Updated), Skipped: int32(result.Skipped),
    }
    for _, e := range result.Errors {
        resp.Errors = append(resp.Errors, &infrafleetv1.ImportFleetInventoryError{Host: e.Host, User: e.UserName, Reason: e.Reason})
    }
    return resp, nil
}
```

Wire `s.importFleetInventory *usecase.ImportFleetInventory` into `Server`'s
constructor alongside its existing usecase fields, and construct it with
`usecase.NewImportFleetInventory(sshTargetRepo)` at service bootstrap
(`cmd/server/main.go`).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/adapter/grpc/... -run TestImportFleetInventory -v
```

Expected: a gRPC-level test asserts request→usecase-input and
usecase-result→response marshaling, and that a usecase error maps to the
correct gRPC status code.
