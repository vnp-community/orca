# TASK-MB-02-01: Track PTY output quiescence + publish `agent_completed`/`agent_error` events

**From Solution:** SOL-MB-02
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/usecase/attach_pty.go`, `backend-go/services/infra-fleet-service/internal/adapter/eventbus/publisher.go` (new)
**Depends on:** none
**Status:** `[x]` DONE — implemented `internal/adapter/eventbus/publisher.go` (new), `usecase.LifecycleEventPublisher` port in `ports.go`, `AttachPty`'s shared `*sync.Map` `liveStates` registry + `publishExitEvent` (agent_completed/agent_error on exit, best-effort/nil-safe), and `cmd/server/main.go` NATS/JetStream wiring (`INFRA` stream, `orca.infra.>` subjects, graceful degrade when NATS unavailable). `TestAttachPty_RelaysAgentOutputAndExit`/`TestAttachPty_ExitCodeZero_PublishesAgentCompleted` cover exactly-one-publish for both exit codes and no-publish-on-Output; `go build`/`go vet`/`go test -race` all green for `services/infra-fleet-service/...`.

---

## Context

`infra-fleet-service` has no `adapter/eventbus` package today (confirmed:
`grep -rl "EventPublisher\|domainevent" internal` is empty) — `tenant-service`'s
`internal/adapter/eventbus/publisher.go` (wrapping `common/eventbus.Publisher`)
is the real precedent to follow. `AttachPty.run`'s existing outbound relay
loop (`attach_pty.go`, the loop around its `PtyServerMessage{Output: ...}`/
`{Exited: ...}` sends) already observes every output chunk and exit — this
task taps that same loop rather than adding a second one.

## Changes to make

`backend-go/services/infra-fleet-service/internal/adapter/eventbus/publisher.go`:

```go
// Package eventbus implements usecase.LifecycleEventPublisher against NATS
// JetStream via common/eventbus — mirrors tenant-service's
// internal/adapter/eventbus/publisher.go shape (best-effort, not
// outbox-backed: a missed publish only means a mobile push notification is
// late/missing, not a correctness issue for the terminal session itself).
package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
)

const (
	SubjectAgentCompleted = "orca.infra.terminal_session.agent_completed"
	SubjectAgentError     = "orca.infra.terminal_session.agent_error"
	SubjectAgentWaiting   = "orca.infra.terminal_session.agent_waiting"
)

type AgentLifecyclePayload struct {
	PtyID        string   `json:"pty_id"`
	ConnectionID string   `json:"connection_id"`
	AgentKind    string   `json:"agent_kind"`
	ExitCode     *int32   `json:"exit_code,omitempty"`
	UserIDs      []string `json:"user_ids"`
}

type Publisher struct {
	pub *commoneventbus.Publisher
}

func New(pub *commoneventbus.Publisher) *Publisher { return &Publisher{pub: pub} }

func (p *Publisher) PublishAgentLifecycle(ctx context.Context, tenantID, subject string, payload AgentLifecyclePayload) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("eventbus: marshal agent lifecycle payload: %w", err)
	}
	return p.pub.Publish(ctx, subject, commoneventbus.Event{
		ID: uuid.NewString(), TenantID: tenantID, OccurredAt: time.Now().UTC(), Version: 1, Payload: raw,
	})
}
```

In `internal/usecase/ports.go`, add:

```go
// LifecycleEventPublisher publishes terminal-session agent-lifecycle
// events for notification-service to translate into mobile pushes
// (BL-MB-02). Best-effort — a publish failure must never fail the PTY
// relay loop itself.
type LifecycleEventPublisher interface {
	PublishAgentLifecycle(ctx context.Context, tenantID, subject string, payload eventbus.AgentLifecyclePayload) error
}
```
(import the `eventbus` package's payload type, or re-declare `AgentLifecyclePayload` in `usecase` and have the adapter accept that type instead — follow whichever direction this codebase's other ports already use for adapter-defined payload structs.)

In `attach_pty.go`, add a per-`ptyId` in-process quiescence registry and
tap the existing output/exit points:

```go
// ptyLiveState is a per-pod, in-memory map[ptyID]*ptyLiveState — inherits
// the same per-pod live-connection-ownership caveat this service's
// existing AttachPty pooling has (a connectionId's live transport lives on
// exactly one pod at a time): a GetTerminalAgentStatus call landing on a
// different pod sees no entry and falls back to today's behavior, an
// honest degrade (see TASK-MB-02-02).
type ptyLiveState struct {
	lastOutputAt time.Time
	agentRunning bool
}

// liveStates is package-level in this scaffold for brevity — wire it as a
// field on AttachPty (constructed once in cmd/server/main.go) so
// GetTerminalAgentStatus (TASK-MB-02-02) can share the same instance.
var liveStates sync.Map // map[string]*ptyLiveState

// Inside AttachPty.run's existing loop, on every non-empty ev.Data (Output) frame:
liveStates.Store(ptyID, &ptyLiveState{lastOutputAt: time.Now(), agentRunning: true})

// On ev.Exited:
liveStates.Delete(ptyID)
subject := eventbus.SubjectAgentCompleted
if ev.ExitCode != 0 {
	subject = eventbus.SubjectAgentError
}
exitCode := ev.ExitCode
_ = uc.events.PublishAgentLifecycle(ctx, tenantID, subject, eventbus.AgentLifecyclePayload{
	PtyID: ptyID, ConnectionID: session.ConnectionID, ExitCode: &exitCode, UserIDs: []string{session.CreatedByUserID},
}) // best-effort — log, don't fail the relay loop, on error
```

`session.CreatedByUserID` requires TASK-MB-02-02's `terminal_sessions`
schema extension — until that lands, thread an empty `UserIDs` slice
(notification-service's `TranslateEvent` already treats a payload naming no
recipient as a no-op per `ErrNoRecipients`, so this degrades safely, not
incorrectly).

Add `events LifecycleEventPublisher` as a new constructor parameter to
`NewAttachPty`, and wire a real `eventbus.New(pub)` into it from
`cmd/server/main.go`.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/... && go vet ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/usecase/... -run AttachPty
```

Test: fake `LifecycleEventPublisher`; a PTY stream with `Exited{ExitCode:0}`
publishes exactly one `agent_completed` event; `Exited{ExitCode:1}` publishes
exactly one `agent_error` event; an `Output` chunk publishes nothing.
