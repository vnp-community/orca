# TASK-CLI-02-04: `infra-fleet-service` gRPC adapter — wire the 3 new RPC handlers

**From Solution:** SOL-CLI-02
**Priority:** P1
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go`
**Depends on:** TASK-CLI-02-02, TASK-CLI-02-03
**Status:** [x] DONE — Server struct/`New(...)`/handlers wired (reused the existing `toProtoTerminalSession` helper, per the task's own note); `cmd/server/main.go` updated. This package had no `server_test.go` at all yet, so one was created from scratch with local fakes for `ConnectionResolver`/`TerminalSessionRepository`/`DevServerAgentClient` (the existing fakes live unexported in `internal/usecase`); all 3 verify-listed contract cases pass.

---

## Context

TASK-CLI-02-02/03 added the three new usecases; this task wires them into `Server`'s struct, `New(...)` constructor, and the three gRPC method bodies, following the exact pattern every other handler in this file already uses (`s.<usecase>.Execute(ctx, ...)` -> `apperrors.ToGRPCStatus`/build the proto response).

## Changes to make

**1. `Server` struct** (add alongside the existing `waitTerminalSession`/`getTerminalAgentStatus` fields):

```go
	getAgentTerminalSession *usecase.GetAgentTerminalSession
	sendTerminalInput       *usecase.SendTerminalInput
	getTerminalScrollback   *usecase.GetTerminalScrollback
```

**2. `New(...)`** — add the three new params (append after `getHostCapabilities`, matching the constructor's existing param order) and assign them in the returned `&Server{...}` literal.

**3. Handler bodies** (append near `GetTerminalAgentStatus`'s handler):

```go
func (s *Server) GetAgentTerminalSession(ctx context.Context, req *infrafleetv1.GetAgentTerminalSessionRequest) (*infrafleetv1.GetAgentTerminalSessionResponse, error) {
	session, found, err := s.getAgentTerminalSession.Execute(ctx, req.GetWorktreeId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	resp := &infrafleetv1.GetAgentTerminalSessionResponse{Found: found}
	if found {
		resp.Session = &infrafleetv1.TerminalSession{
			PtyId: session.PtyID, ConnectionId: session.ConnectionID, Cwd: session.Cwd,
			CreatedAtUnixMs: session.CreatedAt.UnixMilli(), LastActiveAtUnixMs: session.LastActiveAt.UnixMilli(),
		}
	}
	return resp, nil
}

func (s *Server) SendTerminalInput(ctx context.Context, req *infrafleetv1.SendTerminalInputRequest) (*emptypb.Empty, error) {
	if err := s.sendTerminalInput.Execute(ctx, req.GetPtyId(), req.GetData()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) GetTerminalScrollback(ctx context.Context, req *infrafleetv1.GetTerminalScrollbackRequest) (*infrafleetv1.GetTerminalScrollbackResponse, error) {
	result, err := s.getTerminalScrollback.Execute(ctx, req.GetPtyId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.GetTerminalScrollbackResponse{Text: result.Text, Truncated: result.Truncated}, nil
}
```

Check the existing `toProtoTerminalSession`-equivalent helper (search this file for how `ListTerminalSessions`'s handler already builds a `*infrafleetv1.TerminalSession` from `domain.TerminalSession` — reuse that helper instead of the inline literal above if one already exists, to avoid duplicating the field mapping).

**4. `cmd/server/main.go`** — construct the three new usecases (per TASK-CLI-02-02/03's `New*` calls) and pass them into `infragrpc.New(...)`'s call, in the same position as the struct/constructor param order from step 2.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/adapter/grpc/... -v
```

Expected new cases in `server_test.go`: one contract test per new RPC, exercising the extended proto through a fake usecase — `GetAgentTerminalSession{found:false}` returns `Found: false` with `Session` unset (not a zero-value struct that looks populated); `SendTerminalInput` returns `emptypb.Empty` on success; `GetTerminalScrollback` round-trips `text`/`truncated`.
