# TASK-MB-03-05: Wire `DispatchPrompt`/`GetQueuedPrompt` into `infra-fleet-service`'s gRPC server

**From Solution:** SOL-MB-03
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go`, `backend-go/services/infra-fleet-service/cmd/server/main.go`
**Depends on:** TASK-MB-03-04
**Status:** [x] DONE — `Server.DispatchPrompt`/`GetQueuedPrompt` handlers + `cmd/server/main.go` wiring added, sharing the single `queuedPromptStore` instance with `GetTerminalAgentStatus`'s drain hook; `go build`/`go vet`/`go test ./services/infra-fleet-service/...` all pass.

---

## Context

Thin translation-only wiring, following the same pattern every other RPC
in this file already uses (see `ResizeTerminalSession`/`GetTerminalAgentStatus`
handlers for the shape to match).

## Changes to make

Add fields to `Server`:

```go
dispatchPrompt  *usecase.DispatchPrompt
getQueuedPrompt *usecase.GetQueuedPrompt
```

Add handlers:

```go
func (s *Server) DispatchPrompt(ctx context.Context, req *infrafleetv1.DispatchPromptRequest) (*infrafleetv1.DispatchPromptResponse, error) {
	result, err := s.dispatchPrompt.Execute(ctx, usecase.DispatchPromptInput{
		PtyID: req.GetPtyId(), Prompt: req.GetPrompt(), Overwrite: req.GetOverwrite(), DeviceID: req.GetDispatchedByDeviceId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.DispatchPromptResponse{
		Outcome:                     infrafleetv1.DispatchPromptResponse_Outcome(infrafleetv1.DispatchPromptResponse_Outcome_value[result.Outcome]),
		ExistingQueuedPromptPreview: result.ExistingPreview,
	}, nil
}

func (s *Server) GetQueuedPrompt(ctx context.Context, req *infrafleetv1.GetQueuedPromptRequest) (*infrafleetv1.GetQueuedPromptResponse, error) {
	has, prompt, queuedAt, err := s.getQueuedPrompt.Execute(ctx, req.GetPtyId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &infrafleetv1.GetQueuedPromptResponse{HasQueuedPrompt: has, Prompt: prompt, QueuedAtUnixMs: queuedAt}, nil
}
```

In `cmd/server/main.go`, construct `postgres.NewQueuedPromptStore(pool)` and
the two usecases, passing them into `grpc.New(...)`'s extended parameter
list. Ensure the SAME `*sync.Map` liveStates instance and the SAME
`QueuedPromptRepository` instance are shared between `AttachPty`/
`GetTerminalAgentStatus` (TASK-MB-02-01/02) and `DispatchPrompt` — the
queue-drain hook in `GetTerminalAgentStatus` needs the identical
`QueuedPromptRepository` this constructor wires here.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/... && go vet ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/...
```
