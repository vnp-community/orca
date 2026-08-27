# TASK-TM-04-07: Pass `shellIntegration` through `terminal.create` wscompat channel

**From Solution:** SOL-TM-04
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal.go`
**Depends on:** TASK-TM-04-05 (proto field), TASK-TM-04-06 (infra-fleet-service accepts it)
**Status:** `[ ]` TODO

---

## Context

The last hop of the BR-TM-13 pass-through chain: the WebSocket client sets
`shellIntegration` in `terminal.create`'s args, and it rides straight
through to `SpawnTerminalSessionRequest.shell_integration` — the one line
this feature adds to `channels_terminal.go`, matching every other field
`terminalCreateArgs` already forwards unexamined.

## Changes to make

In `backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal.go`,
extend `terminalCreateArgs`:

```go
type terminalCreateArgs struct {
	ConnectionID     string `json:"connectionId"`
	Cwd              string `json:"cwd"`
	Shell            string `json:"shell"`
	Cols             int32  `json:"cols"`
	Rows             int32  `json:"rows"`
	ShellIntegration bool   `json:"shellIntegration"` // BR-TM-13
}
```

Extend `registerTerminalCreateChannel`'s `SpawnTerminalSessionRequest` call:

```go
spawnResp, err := client.SpawnTerminalSession(invokeCtx, &infrafleetv1.SpawnTerminalSessionRequest{
	ConnectionId:     in.ConnectionID,
	Cwd:              in.Cwd,
	Shell:            in.Shell,
	Cols:             in.Cols,
	Rows:             in.Rows,
	ShellIntegration: in.ShellIntegration,
})
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/...
```

Extend `wscompat/channels_terminal_test.go`: `terminal.create` with
`shellIntegration: true` in args produces a `SpawnTerminalSessionRequest`
with `ShellIntegration: true`; an existing test case that omits the field
keeps passing with `ShellIntegration: false` (regression guard — no
behavior change for existing callers).

```bash
go test ./services/api-gateway/internal/adapter/wscompat/... -run TestTerminalCreate -v
```

Expected: clean build, all cases pass including pre-existing ones.

## Manual/integration smoke (optional, end-to-end confirmation)

Spawn a `pwsh.exe` relay PTY with `shellIntegration: true` via
`terminal.create`, run a command, confirm raw OSC 133;A/B/C/D bytes appear
in the `terminal.output`/`Output` frames the client receives — proves
end-to-end byte flow without backend-go interpreting them, per the
architecture boundary SOL-TM-04 preserves (see that solution's own test
plan for the full manual-smoke script).
