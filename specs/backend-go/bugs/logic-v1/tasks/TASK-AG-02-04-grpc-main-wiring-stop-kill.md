# TASK-AG-02-04: Wire `StopAgentSession`/`KillAgentSession` into gRPC server + `main.go`

**From Solution:** SOL-AG-02
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go`, `backend-go/services/infra-fleet-service/cmd/server/main.go`
**Depends on:** TASK-AG-02-01, TASK-AG-02-02, TASK-AG-02-03
**Status:** `[ ]` TODO

---

## Context

Registers the two new RPCs against `*usecase.StopAgentSession`/`*usecase.KillAgentSession`, following `StopTerminalProcess`/`KillTerminalSession`'s existing wiring shape exactly.

## Changes to make

In `internal/adapter/grpc/server.go`, add `stopAgentSession *usecase.StopAgentSession` and `killAgentSession *usecase.KillAgentSession` fields to `Server`, thread them through the constructor, and add:

```go
func (s *Server) StopAgentSession(ctx context.Context, req *infrafleetv1.StopAgentSessionRequest) (*emptypb.Empty, error) {
	if err := s.stopAgentSession.Execute(ctx, req.GetSessionId()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) KillAgentSession(ctx context.Context, req *infrafleetv1.KillAgentSessionRequest) (*emptypb.Empty, error) {
	if err := s.killAgentSession.Execute(ctx, req.GetSessionId(), req.GetSignal()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}
```

In `cmd/server/main.go`, after `startAgentSessionUC := usecase.NewStartAgentSession(...)` (TASK-AG-01-07), add:

```go
stopAgentSessionUC := usecase.NewStopAgentSession(agentSessionStore, repo, agentClient)
killAgentSessionUC := usecase.NewKillAgentSession(agentSessionStore, repo, agentClient, nil) // writeActivity: see TASK-AG-02-03/06
```

Pass both into the gRPC server constructor call alongside the other
usecases.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go vet ./services/infra-fleet-service/...
```
