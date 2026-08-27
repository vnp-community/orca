# TASK-MB-04-02: Capture a per-`ptyId` output ring buffer + 500-char truncation

**From Solution:** SOL-MB-04
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/domain/terminal_session.go`, `backend-go/services/infra-fleet-service/internal/usecase/attach_pty.go`, `backend-go/services/infra-fleet-service/internal/usecase/get_terminal_agent_status.go`
**Depends on:** TASK-MB-02-01 (`AttachPty`'s output-relay hook + `ptyLiveState` registry), TASK-MB-02-02
**Status:** `[ ]` TODO

---

## Context

`TerminalSession`'s own doc comment says this domain type deliberately
"Holds no PTY bytes" — this task does NOT contradict that: the output
buffer lives only in TASK-MB-02-01's per-pod, in-memory `ptyLiveState`
registry (extended here with a `lastOutput []byte` field), never written to
Postgres. BR-MB-15's 500-char cap happens at the read boundary, not the
write boundary, so the in-memory buffer can hold more (2048 bytes) without
coupling truncation to storage size. Same cross-pod caveat as
TASK-MB-02-01/02 applies: a status request landing on a different pod than
the live `AttachPty` stream sees an empty preview — an honest absence.

## Changes to make

In `attach_pty.go`, extend `ptyLiveState` (from TASK-MB-02-01):

```go
const lastOutputBufferBytes = 2048 // headroom above BR-MB-15's 500-char cap; truncation happens at the read boundary

type ptyLiveState struct {
	lastOutputAt time.Time
	agentRunning bool
	lastOutput   []byte // ring-buffer tail of recent PTY output
}

func appendOutput(buf []byte, chunk []byte) []byte {
	buf = append(buf, chunk...)
	if len(buf) > lastOutputBufferBytes {
		buf = buf[len(buf)-lastOutputBufferBytes:]
	}
	return buf
}
```

Update the output-observation point (TASK-MB-02-01's `liveStates.Store`
call) to preserve and append to the existing buffer rather than overwrite it:

```go
prev, _ := liveStates.Load(ptyID)
var buf []byte
if prev != nil {
	buf = prev.(*ptyLiveState).lastOutput
}
liveStates.Store(ptyID, &ptyLiveState{lastOutputAt: time.Now(), agentRunning: true, lastOutput: appendOutput(buf, ev.Data)})
```

Add a truncation helper (domain layer, so it's unit-testable independent of
the in-memory registry):

```go
// backend-go/services/infra-fleet-service/internal/domain/terminal_session.go

// TruncatedForMobile applies BR-MB-15's 500-char cap at the point of
// exposure — keeps the buffer's internal size independent of the mobile
// contract, so a future non-mobile consumer wanting more context isn't
// retroactively capped by this rule. Tail-truncated: keeps the MOST RECENT
// bytes, not the head.
func TruncatedForMobile(lastOutput []byte) string {
	s := string(lastOutput)
	if len(s) <= 500 {
		return s
	}
	return s[len(s)-500:]
}
```

In `get_terminal_agent_status.go`'s `Execute`, populate the new proto field:

```go
if v, ok := uc.liveStates.Load(ptyID); ok {
	result.LastOutputPreview = domain.TruncatedForMobile(v.(*ptyLiveState).lastOutput)
}
```

(`AgentStatusResult` needs a new `LastOutputPreview string` field, and the
`grpc/server.go` handler for `GetTerminalAgentStatus` needs to map it onto
`GetTerminalAgentStatusResponse.LastOutputPreview`.)

`ListTerminalSessions`'s response mapping (`grpc/server.go`) should
similarly populate `TerminalSession.LastOutputPreview` from the same
`liveStates` registry, keyed by each session's `PtyID` — empty when no
live entry exists (cross-pod case), not an error.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/... && go vet ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/domain/... -run TruncatedForMobile
go test ./services/infra-fleet-service/internal/usecase/... -run 'AttachPty|GetTerminalAgentStatus'
```

Test: `TruncatedForMobile` caps at exactly 500 chars, tail-truncated (keeps
most recent output, not oldest). `appendOutput` caps at 2048 bytes, also
tail-truncated.
