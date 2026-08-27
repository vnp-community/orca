# TASK-TM-04-06: Pass `ShellIntegration` through `infra-fleet-service`'s spawn path

**From Solution:** SOL-TM-04
**Priority:** P1
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/usecase/spawn_terminal_session.go`
**Depends on:** TASK-TM-04-05 (proto field)
**Status:** `[ ]` TODO

---

## Context

Coordination decides *whether* (the boolean), execution decides *how* (the
bootstrap script itself, entirely in `agent/`) — `infra-fleet-service`
never inspects this value, it only forwards it from
`SpawnTerminalSessionInput` through `SpawnPtyInput` to the agent's
`pty.create` params, the same pass-through shape `Shell` already has.

## Changes to make

In `backend-go/services/infra-fleet-service/internal/usecase/ports.go`,
extend `SpawnPtyInput`:

```go
// SpawnPtyInput carries pty.create's request fields.
type SpawnPtyInput struct {
	Cwd              string
	Shell            string
	Cols             int32
	Rows             int32
	ShellIntegration bool // BR-TM-13 — forwarded to the agent's pty.create, never inspected here
}
```

In `backend-go/services/infra-fleet-service/internal/usecase/spawn_terminal_session.go`,
extend `SpawnTerminalSessionInput` and the `agent.SpawnPty` call:

```go
type SpawnTerminalSessionInput struct {
	ConnectionID     string
	Cwd              string
	Shell            string
	Cols             int32
	Rows             int32
	ShellIntegration bool // BR-TM-13 — forwarded to SpawnPtyInput, never inspected here
}
```

```go
result, err := uc.agent.SpawnPty(ctx, devServer, SpawnPtyInput{
	Cwd: in.Cwd, Shell: in.Shell, Cols: in.Cols, Rows: in.Rows,
	ShellIntegration: in.ShellIntegration,
})
```

In `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/methods.go`,
add the field to the `pty.create` params map inside `SpawnPty`:

```go
params := map[string]any{"cwd": in.Cwd}
if in.Shell != "" {
	params["shellOverride"] = in.Shell
}
if in.Cols > 0 {
	params["cols"] = in.Cols
}
if in.Rows > 0 {
	params["rows"] = in.Rows
}
if in.ShellIntegration {
	params["shellIntegration"] = in.ShellIntegration
}
```

In `backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go`'s
`SpawnTerminalSession` handler, forward the new proto field into the
usecase input:

```go
func (s *Server) SpawnTerminalSession(ctx context.Context, req *infrafleetv1.SpawnTerminalSessionRequest) (*infrafleetv1.SpawnTerminalSessionResponse, error) {
	session, err := s.spawnTerminalSession.Execute(ctx, usecase.SpawnTerminalSessionInput{
		ConnectionID: req.GetConnectionId(), Cwd: req.GetCwd(), Shell: req.GetShell(),
		Cols: req.GetCols(), Rows: req.GetRows(), ShellIntegration: req.GetShellIntegration(),
	})
	// ... unchanged from here ...
}
```

(Check the exact current field list in `SpawnTerminalSessionInput{...}`
inside that handler before editing — add `ShellIntegration:
req.GetShellIntegration()` to whatever fields are already there, do not
otherwise change the shape.)

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
```

Extend `spawn_terminal_session_test.go` with a fake `DevServerAgentClient`:
- `ShellIntegration` on the input reaches `SpawnPtyInput.ShellIntegration`
  unmodified
- defaults to `false` when unset (existing test cases that don't set it
  keep passing unmodified — confirms no behavior change for existing
  callers)

```bash
go test ./services/infra-fleet-service/internal/usecase/... -run TestSpawnTerminalSession -v
go test ./services/infra-fleet-service/internal/adapter/devserveragent/... -v
```

Expected: clean build, all cases pass including pre-existing ones.
