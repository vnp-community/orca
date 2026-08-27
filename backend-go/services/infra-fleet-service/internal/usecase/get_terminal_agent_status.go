package usecase

import (
	"context"
	"sync"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/eventbus"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// readyForInputQuiescence is the heuristic silence threshold (TASK-MB-02-02):
// no pty output for this long, while the agent is still running, is treated
// as "waiting at a prompt for input" — tunable, not a business rule. A real
// prompt-detection signal (TASK-MB-02-03) would replace this heuristic.
const readyForInputQuiescence = 3 * time.Second

// GetTerminalAgentStatus backs BOTH the terminal.agentStatus and
// terminal.isRunningAgent wscompat channels (see
// GetTerminalAgentStatusResponse's proto doc comment) — one RPC, since both
// questions ("is an agentic CLI running in this pane" / "richer: is it
// idle-and-ready") read the same underlying signal.
//
// A session-lookup failure (unknown pty_id, connection no longer bound) is a
// real error. An agent-level failure to answer (AgentStatus returning an
// error) is NOT — it degrades to the honest zero value
// {AgentRunning:false}, matching InspectTerminalProcess's "known=false, not
// a fabricated result" convention, since this is documented as best-effort
// (see DevServerAgentClient.AgentStatus's FLAGGED doc comment).
type GetTerminalAgentStatus struct {
	sessions   TerminalSessionRepository
	resolver   ConnectionResolver
	agent      DevServerAgentClient
	liveStates *sync.Map // map[string]*ptyLiveState — shared with AttachPty (TASK-MB-02-01), same registry instance, injected via cmd/server/main.go
	events     LifecycleEventPublisher
	// queue is the SAME QueuedPromptRepository instance DispatchPrompt uses
	// (TASK-MB-03-04/05, shared via cmd/server/main.go) — nil-safe: a nil
	// queue simply skips the drain below (matches events' nil-safety), so
	// this constructor stays usable from tests that don't care about
	// mobile prompt dispatch.
	queue QueuedPromptRepository
}

func NewGetTerminalAgentStatus(sessions TerminalSessionRepository, resolver ConnectionResolver, agent DevServerAgentClient, liveStates *sync.Map, events LifecycleEventPublisher, queue QueuedPromptRepository) *GetTerminalAgentStatus {
	return &GetTerminalAgentStatus{sessions: sessions, resolver: resolver, agent: agent, liveStates: liveStates, events: events, queue: queue}
}

func (uc *GetTerminalAgentStatus) Execute(ctx context.Context, ptyID string) (AgentStatusResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return AgentStatusResult{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}

	session, devServer, err := resolveTerminalSession(ctx, tenantID, ptyID, uc.sessions, uc.resolver)
	if err != nil {
		return AgentStatusResult{}, err
	}

	result, err := uc.agent.AgentStatus(ctx, devServer, ptyID)
	if err != nil {
		return AgentStatusResult{AgentRunning: false}, nil
	}

	// BR-MB-15: last-output preview is populated whenever a live entry
	// exists, independent of AgentRunning — a completed/idle session can
	// still usefully show its last output to a mobile client.
	if uc.liveStates != nil {
		if v, ok := uc.liveStates.Load(ptyID); ok {
			result.LastOutputPreview = domain.TruncatedForMobile(v.(*ptyLiveState).lastOutput)
		}
	}

	if result.AgentRunning && uc.liveStates != nil {
		if v, ok := uc.liveStates.Load(ptyID); ok {
			// live is the SAME *ptyLiveState pointer AttachPty stores/reads —
			// mutating readyNotified through it needs no re-Store.
			live := v.(*ptyLiveState)
			result.ReadyForInput = time.Since(live.lastOutputAt) > readyForInputQuiescence
			if result.ReadyForInput && !live.readyNotified {
				live.readyNotified = true // debounce: publish once per transition into quiescence, not every poll while still quiescent
				if uc.events != nil {
					_ = uc.events.PublishAgentLifecycle(ctx, tenantID, eventbus.SubjectAgentWaiting, eventbus.AgentLifecyclePayload{
						PtyID: ptyID, ConnectionID: session.ConnectionID, AgentKind: result.AgentKind, UserIDs: userIDsFor(session),
					})
				}
				// Queue-drain (TASK-MB-03-04): this running->ready transition
				// is the one moment a BR-MB-10-queued prompt actually gets
				// delivered — not a separate poller. GetAndDelete is atomic,
				// so a prompt racing in via a concurrent DispatchPrompt call
				// is delivered at most once either way. Best-effort: a failed
				// drain here just leaves the prompt queued for the next
				// ready-transition (or a later poll after the row error
				// clears), never fails this status read.
				if uc.queue != nil {
					if queued, hasQueued, err := uc.queue.GetAndDelete(ctx, ptyID); err == nil && hasQueued {
						_ = uc.agent.WritePty(ctx, devServer, ptyID, []byte(queued.Prompt))
					}
				}
			}
		}
		// No live-state entry (cross-pod: the live AttachPty stream for
		// this ptyId runs on a different pod) falls through to AgentStatus's
		// own ReadyForInput == AgentRunning value, unchanged from today — an
		// honest degrade, not a wrong answer.
	}
	return result, nil
}
