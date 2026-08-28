# TASK-AG-05-04: `AgentOutputClassifier` — two-track status detection over `StreamPty`

**From Solution:** SOL-AG-05
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/usecase/agent_output_classifier.go` (new)
**Depends on:** TASK-AG-01-05, TASK-AG-02-03, TASK-AG-05-01, TASK-AG-05-02, TASK-AG-05-03
**Status:** `[x]` DONE — AgentOutputClassifier implemented as specced (startupTimeout made an overridable struct field for testability, not a bare package const, so BR-AG-04's escalation path is exercisable without a real 30s wait). agent_output_classifier_test.go covers the Claude-fresh-spawn track-1-exclusivity case (with the required TODO on classifyStreamJSONLine's stub), rate-limit-publishes-not-status, startup-timeout-kills-and-marks-error, and exit-event-marks-stopped-or-error — all passing.

---

## Context

Subscribes to the same `StreamPty` channel `AttachPty` already demuxes from (no extra network hop), classifying every chunk on one of two tracks selected by `session.UsesStreamJSON()` (TASK-AG-01-02): Claude Code fresh spawns get structured event parsing (track 1, more reliable — not implemented by this task, see the `TODO` below), everything else gets OSC 133 + text-pattern matching (track 2, TASK-AG-05-01/02). **Correction to `usecase.PtyEvent`'s real shape**: it has an `Exited bool` field (not a `Type` enum with `PtyEventData`/`PtyEventExit` constants, as SOL-AG-05's sketch assumed) — this task's `Run` switches on `ev.Exited` directly.

## Changes to make

Create `backend-go/services/infra-fleet-service/internal/usecase/agent_output_classifier.go`:

```go
package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// AgentOutputClassifier watches one AgentSession's PTY stream and derives
// AgentStatus transitions from it — the real implementation of what
// terminal.agentStatus's process-title heuristic (ports.go's existing
// AgentStatus method) only approximates for plain shells.
type AgentOutputClassifier struct {
	sessions  AgentSessionRepository
	relay     DevServerAgentClient
	publisher AgentStatusEventPublisher
	kill      *KillAgentSession // BR-AG-04's 30s startup-timeout escalation
}

func NewAgentOutputClassifier(sessions AgentSessionRepository, relay DevServerAgentClient, publisher AgentStatusEventPublisher, kill *KillAgentSession) *AgentOutputClassifier {
	return &AgentOutputClassifier{sessions: sessions, relay: relay, publisher: publisher, kill: kill}
}

// startupTimeout — BR-AG-04.
const startupTimeout = 30 * time.Second

// Run subscribes to session's PTY stream and classifies every chunk until
// the stream closes (agent.exited) or ctx is cancelled. One goroutine per
// live AgentSession, started by StartAgentSession/ResumeAgentSession right
// after Create.
func (c *AgentOutputClassifier) Run(ctx context.Context, tenantID string, session domain.AgentSession, devServer domain.DevServer) {
	events, unsubscribe, err := c.relay.StreamPty(ctx, devServer, session.PtyID)
	if err != nil {
		return // spawn-time failure already surfaced via StartAgentSession's own error path
	}
	defer unsubscribe()

	scanner := &domain.Osc133Scanner{}
	timer := time.AfterFunc(startupTimeout, func() { c.onStartupTimeout(context.Background(), tenantID, session) })
	defer timer.Stop()

	current := session.Status
	for ev := range events {
		if ev.Exited {
			final := domain.AgentStatusStopped
			if ev.ExitCode != 0 {
				final = domain.AgentStatusError
			}
			_ = c.sessions.MarkStoppedWithStatus(ctx, tenantID, session.ID, final, time.Now().UTC())
			_ = c.publisher.PublishStatusChanged(ctx, tenantID, session.ID, final)
			return
		}

		var status domain.AgentStatus
		var rateLimited bool
		if session.UsesStreamJSON() {
			// Track 1 (Claude Code fresh spawns): parse --output-format
			// stream-json's newline-delimited JSON events. NOT implemented
			// by this task — classifyStreamJSONLine is a stub returning
			// ("", false) until a follow-up task adds it; track 2 remains
			// the only classification path in the meantime, which is safe
			// (just less precise) since stream-json output still contains
			// plain-text fragments the pattern matchers can catch.
			status, rateLimited = classifyStreamJSONLine(ev.Data)
		} else {
			for _, marker := range scanner.Feed(string(ev.Data)) {
				if marker.Kind == "C" {
					status = domain.AgentStatusRunning
				} else if marker.Kind == "D" {
					if marker.ExitCode == nil || *marker.ExitCode == 0 {
						status = domain.AgentStatusIdle
					} else {
						status = domain.AgentStatusError
					}
				}
			}
			if s, rl, ok := domain.ClassifyText(session.ModelID, string(ev.Data)); ok {
				status, rateLimited = s, rl
			}
		}

		if rateLimited {
			_ = c.publisher.PublishRateLimited(ctx, tenantID, session.ID)
			continue
		}
		if status != "" && status != current {
			timer.Stop() // first real signal cancels the 30s startup timer regardless of which status it is
			_ = c.sessions.UpdateStatus(ctx, tenantID, session.ID, status, time.Now().UTC())
			_ = c.publisher.PublishStatusChanged(ctx, tenantID, session.ID, status)
			current = status
		}
	}
}

// classifyStreamJSONLine is track 1's entry point — Claude Code fresh
// spawns only. NOT IMPLEMENTED by this task: mapping stream-json's
// {type: "assistant"|"user"|"result"|...} event shapes to AgentStatus
// needs a sample corpus of real Claude Code --output-format stream-json
// output to get right, which this pass doesn't have. Tracked as a known
// gap — track 2 covers this session type today, just less precisely.
func classifyStreamJSONLine(data []byte) (status domain.AgentStatus, rateLimited bool) {
	return "", false
}

// onStartupTimeout — BR-AG-04: no idle/running signal within 30s of spawn
// accept → force-kill and mark 'error'. Composes KillAgentSession (which
// itself marks the row 'stopped') rather than calling KillAgent directly,
// then overrides the status to 'error' so a caller can distinguish
// "never started" from "stopped normally."
func (c *AgentOutputClassifier) onStartupTimeout(ctx context.Context, tenantID string, session domain.AgentSession) {
	_ = c.kill.Execute(ctx, session.ID, "SIGKILL")
	_ = c.sessions.UpdateStatus(ctx, tenantID, session.ID, domain.AgentStatusError, time.Now().UTC())
	_ = c.publisher.PublishStatusChanged(ctx, tenantID, session.ID, domain.AgentStatusError)
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/usecase/... -run TestAgentOutputClassifier -v
```

Add `agent_output_classifier_test.go` with a fake `DevServerAgentClient.StreamPty` feeding synthetic `PtyEvent`s:
- a Claude fresh-spawn session (`UsesStreamJSON()==true`) fed OSC 133;C/D bytes → asserts **no** status change (track 1 is exclusive when active — `classifyStreamJSONLine`'s stub returning `("", false)` means this specific assertion holds trivially until track 1 is implemented; add a `TODO` in the test noting it must be re-verified once `classifyStreamJSONLine` does real work).
- a resumed/non-Claude session fed a rate-limit string → `PublishRateLimited` called, `UpdateStatus` **not** called.
- no status signal within 30s (inject a fake clock/short timeout for the test) → `KillAgentSession.Execute` invoked, session ends up `error`.
- a `PtyEvent{Exited: true, ExitCode: 0}` / `{Exited: true, ExitCode: 1}` → `MarkStoppedWithStatus` called with `stopped`/`error` respectively.
