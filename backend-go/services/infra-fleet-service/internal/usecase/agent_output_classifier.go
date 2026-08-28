package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
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

	// startupTimeout defaults to defaultStartupTimeout — overridable (test
	// only, via a shorter value assigned after construction) so
	// BR-AG-04's escalation path is exercisable without a real 30s wait.
	startupTimeout time.Duration
}

func NewAgentOutputClassifier(sessions AgentSessionRepository, relay DevServerAgentClient, publisher AgentStatusEventPublisher, kill *KillAgentSession) *AgentOutputClassifier {
	return &AgentOutputClassifier{sessions: sessions, relay: relay, publisher: publisher, kill: kill, startupTimeout: defaultStartupTimeout}
}

// defaultStartupTimeout — BR-AG-04.
const defaultStartupTimeout = 30 * time.Second

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
	timer := time.AfterFunc(c.startupTimeout, func() { c.onStartupTimeout(context.Background(), tenantID, session) })
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
// "never started" from "stopped normally." Re-attaches tenantID to ctx —
// this runs off a time.AfterFunc callback whose ctx (context.Background(),
// see Run) carries none, and KillAgentSession.Execute requires one.
func (c *AgentOutputClassifier) onStartupTimeout(ctx context.Context, tenantID string, session domain.AgentSession) {
	ctx = tenant.WithTenantID(ctx, tenantID)
	_ = c.kill.Execute(ctx, session.ID, "SIGKILL")
	_ = c.sessions.UpdateStatus(ctx, tenantID, session.ID, domain.AgentStatusError, time.Now().UTC())
	_ = c.publisher.PublishStatusChanged(ctx, tenantID, session.ID, domain.AgentStatusError)
}
