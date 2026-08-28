# TASK-MB-04-05: Wire `GetMobileWorktreeStatus` into `project-service`'s gRPC server

**From Solution:** SOL-MB-04
**Priority:** P0
**Service:** `project-service`
**File:** `backend-go/services/project-service/internal/adapter/grpc/server.go`, `backend-go/services/project-service/cmd/server/main.go`
**Depends on:** TASK-MB-04-04
**Status:** `[x]` DONE — added `getMobileWorktreeStatus` field + `Deps.GetMobileWorktreeStatus` + `GetMobileWorktreeStatus` handler to `grpc/server.go`; `cmd/server/main.go` dials `NewInfraFleetTerminalStatusResolver(cfg.InfraFleetServiceAddr)` and wires `NewGetMobileWorktreeStatus(worktreeRepo, repo, terminalStatusResolver)` into `grpc.New`'s Deps; `go build`/`go vet`/`go test ./services/project-service/...` all pass.

---

## Context

Thin translation-only wiring, following the same pattern every other RPC
in this file already uses.

## Changes to make

Add field to `Server`:

```go
getMobileWorktreeStatus *usecase.GetMobileWorktreeStatus
```

Add handler:

```go
func (s *Server) GetMobileWorktreeStatus(ctx context.Context, req *projectv1.GetMobileWorktreeStatusRequest) (*projectv1.GetMobileWorktreeStatusResponse, error) {
	result, err := s.getMobileWorktreeStatus.Execute(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*projectv1.MobileWorktreeStatus, 0, len(result.Worktrees))
	for _, wt := range result.Worktrees {
		out = append(out, &projectv1.MobileWorktreeStatus{
			Id: wt.ID, Name: wt.Name, Agent: wt.Agent, Status: wt.Status,
			DurationMs: wt.DurationMs, LastOutput: wt.LastOutput,
		})
	}
	return &projectv1.GetMobileWorktreeStatusResponse{
		Worktrees:        out,
		GeneratedAtUnixMs: result.GeneratedAt.UnixMilli(),
	}, nil
}
```

In `cmd/server/main.go`, construct
`grpcclient.NewInfraFleetTerminalStatusResolver(infraFleetAddr)` and the
`GetMobileWorktreeStatus` usecase, passing them into `grpc.New(...)`'s
extended parameter list — reuse the same `infraFleetAddr` config value
`InfraFleetDevServerLister` already dials.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/project-service/... && go vet ./services/project-service/...
go test ./services/project-service/...
```
