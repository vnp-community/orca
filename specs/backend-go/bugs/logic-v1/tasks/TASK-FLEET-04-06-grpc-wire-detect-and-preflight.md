# TASK-FLEET-04-06: Wire `DetectDevServerAgents`/`CheckDevServerPreflight` into the gRPC server

**From Solution:** SOL-FLEET-04
**Priority:** P1
**Service:** `infra-fleet-service` (grpc adapter)
**File:** `backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go`
**Depends on:** TASK-FLEET-04-01, TASK-FLEET-04-04, TASK-FLEET-04-05
**Status:** [x] DONE — wired both handlers using `tenant.RequireTenantID(ctx)` + `apperrors.ToGRPCStatus` (matching this file's actual established convention — the task's `identityFromContext`/`mapAppError` placeholders don't exist in this codebase). Wired Server fields/constructor + main.go bootstrap. New server_test.go fakes (fakeDevServerAgent implementing the full usecase.DevServerAgentClient interface) cover request->response marshaling for both RPCs and no-tenant->Unauthenticated gRPC status mapping. Full suite + `-race` pass.

---

## Context

Thin gRPC handlers mapping the two new proto RPCs to their usecases,
following this file's existing handler shape.

## Changes to make

```go
func (s *Server) DetectDevServerAgents(ctx context.Context, req *infrafleetv1.DetectDevServerAgentsRequest) (*infrafleetv1.DetectDevServerAgentsResponse, error) {
    identity, err := identityFromContext(ctx) // or however this file resolves tenantID today
    if err != nil {
        return nil, mapAppError(err)
    }
    result, err := s.detectDevServerAgents.Execute(ctx, identity.TenantID, req.GetDevServerId())
    if err != nil {
        return nil, mapAppError(err)
    }
    return &infrafleetv1.DetectDevServerAgentsResponse{Agents: result.Agents, Platform: result.Platform}, nil
}

func (s *Server) CheckDevServerPreflight(ctx context.Context, req *infrafleetv1.CheckDevServerPreflightRequest) (*infrafleetv1.CheckDevServerPreflightResponse, error) {
    identity, err := identityFromContext(ctx)
    if err != nil {
        return nil, mapAppError(err)
    }
    result, err := s.checkDevServerPreflight.Execute(ctx, identity.TenantID, req.GetDevServerId(), req.GetProbePort())
    if err != nil {
        return nil, mapAppError(err)
    }
    return &infrafleetv1.CheckDevServerPreflightResponse{
        Git:  &infrafleetv1.CheckResult{Installed: result.Git.Installed, Version: result.Git.Version, MeetsMin: result.Git.MeetsMin},
        Node: &infrafleetv1.CheckResult{Installed: result.Node.Installed, Version: result.Node.Version, MeetsMin: result.Node.MeetsMin},
        Disk: &infrafleetv1.DiskCheckResult{FreeGb: result.Disk.FreeGB, MeetsMin: result.Disk.MeetsMin},
        Port: &infrafleetv1.PortCheckResult{Port: result.Port.Port, Available: result.Port.Available},
        Gh:   &infrafleetv1.CheckResult{Installed: result.GH.Installed, Version: result.GH.Version, MeetsMin: result.GH.MeetsMin},
    }, nil
}
```

Wire `s.detectDevServerAgents *usecase.DetectDevServerAgents` and
`s.checkDevServerPreflight *usecase.CheckDevServerPreflight` into `Server`'s
constructor and construct both at bootstrap in `cmd/server/main.go`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/adapter/grpc/... -run 'TestDetectDevServerAgents|TestCheckDevServerPreflight' -v
```
