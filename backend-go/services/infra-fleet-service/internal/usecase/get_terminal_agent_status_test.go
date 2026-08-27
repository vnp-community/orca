package usecase

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/eventbus"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestGetTerminalAgentStatus_RequiresTenantContext(t *testing.T) {
	uc := NewGetTerminalAgentStatus(&fakeTerminalSessionRepository{}, &fakeConnectionResolver{}, &fakeDevServerAgentClient{}, &sync.Map{}, &fakeLifecycleEventPublisher{}, &fakeQueuedPromptRepository{})
	_, err := uc.Execute(context.Background(), "pty-1")
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

// TestGetTerminalAgentStatus_NoLiveStateEntry_RegressionGuard: a cross-pod
// (or never-attached) ptyId has no liveStates entry — ReadyForInput must
// fall through unchanged to AgentStatus's own AgentRunning-derived value
// (TASK-MB-02-02's "honest degrade, not a wrong answer").
func TestGetTerminalAgentStatus_NoLiveStateEntry_RegressionGuard(t *testing.T) {
	sessions := &fakeTerminalSessionRepository{}
	resolver := &fakeConnectionResolver{}
	seedSession(t, sessions, resolver, "tenant-1", "pty-1", "conn-1")
	agent := &fakeDevServerAgentClient{agentStatusResult: AgentStatusResult{AgentRunning: true, ReadyForInput: true, AgentKind: "claude-code"}}
	lifecycleEvents := &fakeLifecycleEventPublisher{}
	uc := NewGetTerminalAgentStatus(sessions, resolver, agent, &sync.Map{}, lifecycleEvents, &fakeQueuedPromptRepository{})

	ctx := withTenant(context.Background(), "tenant-1")
	result, err := uc.Execute(ctx, "pty-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ReadyForInput != result.AgentRunning {
		t.Errorf("expected ReadyForInput == AgentRunning (%v) unchanged with no liveStates entry, got ReadyForInput=%v", result.AgentRunning, result.ReadyForInput)
	}
	if len(lifecycleEvents.callsSnapshot()) != 0 {
		t.Error("expected no agent_waiting publish with no liveStates entry")
	}
}

// TestGetTerminalAgentStatus_QuiescentOutput_ReadyForInputAndPublishesOnce
// covers the quiescence heuristic: a liveStates entry with lastOutputAt
// more than readyForInputQuiescence in the past flips ReadyForInput=true
// and publishes agent_waiting exactly once — a second poll while still
// quiescent must NOT republish (debounced via ptyLiveState.readyNotified).
func TestGetTerminalAgentStatus_QuiescentOutput_ReadyForInputAndPublishesOnce(t *testing.T) {
	sessions := &fakeTerminalSessionRepository{}
	resolver := &fakeConnectionResolver{}
	seedSession(t, sessions, resolver, "tenant-1", "pty-1", "conn-1")
	agent := &fakeDevServerAgentClient{agentStatusResult: AgentStatusResult{AgentRunning: true, ReadyForInput: false, AgentKind: "claude-code"}}
	lifecycleEvents := &fakeLifecycleEventPublisher{}
	liveStates := &sync.Map{}
	liveStates.Store("pty-1", &ptyLiveState{lastOutputAt: time.Now().Add(-5 * time.Second), agentRunning: true})
	uc := NewGetTerminalAgentStatus(sessions, resolver, agent, liveStates, lifecycleEvents, &fakeQueuedPromptRepository{})

	ctx := withTenant(context.Background(), "tenant-1")

	result, err := uc.Execute(ctx, "pty-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.ReadyForInput {
		t.Error("expected ReadyForInput=true for a >3s-quiescent liveStates entry")
	}
	calls := lifecycleEvents.callsSnapshot()
	if len(calls) != 1 {
		t.Fatalf("expected exactly one agent_waiting publish, got %d: %+v", len(calls), calls)
	}
	if calls[0].Subject != eventbus.SubjectAgentWaiting {
		t.Errorf("expected subject %q, got %q", eventbus.SubjectAgentWaiting, calls[0].Subject)
	}
	if calls[0].Payload.ConnectionID != "conn-1" {
		t.Errorf("expected payload ConnectionID %q, got %q", "conn-1", calls[0].Payload.ConnectionID)
	}

	// Second poll while still quiescent: debounced, no republish.
	result2, err := uc.Execute(ctx, "pty-1")
	if err != nil {
		t.Fatalf("unexpected error on second poll: %v", err)
	}
	if !result2.ReadyForInput {
		t.Error("expected ReadyForInput=true to still hold on the second poll")
	}
	if calls := lifecycleEvents.callsSnapshot(); len(calls) != 1 {
		t.Fatalf("expected still exactly one agent_waiting publish after a second poll while quiescent, got %d: %+v", len(calls), calls)
	}
}

// TestGetTerminalAgentStatus_AgentStatusError_DegradesToZeroValue mirrors
// the pre-existing best-effort AgentStatus-failure convention — unaffected
// by the liveStates/events additions.
func TestGetTerminalAgentStatus_AgentStatusError_DegradesToZeroValue(t *testing.T) {
	sessions := &fakeTerminalSessionRepository{}
	resolver := &fakeConnectionResolver{}
	seedSession(t, sessions, resolver, "tenant-1", "pty-1", "conn-1")
	agent := &fakeDevServerAgentClient{agentStatusErr: context.DeadlineExceeded}
	uc := NewGetTerminalAgentStatus(sessions, resolver, agent, &sync.Map{}, &fakeLifecycleEventPublisher{}, &fakeQueuedPromptRepository{})

	ctx := withTenant(context.Background(), "tenant-1")
	result, err := uc.Execute(ctx, "pty-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AgentRunning {
		t.Error("expected AgentRunning=false when AgentStatus fails")
	}
}

// TestGetTerminalAgentStatus_ReadyTransition_DrainsQueuedPrompt is
// TASK-MB-03-04's regression guard: a queued prompt present when the agent
// transitions running->ready must be delivered via WritePty exactly once
// (through GetAndDelete), and the queue row must be gone afterward — no
// double delivery even across repeated polls once the transition already
// fired.
func TestGetTerminalAgentStatus_ReadyTransition_DrainsQueuedPrompt(t *testing.T) {
	sessions := &fakeTerminalSessionRepository{}
	resolver := &fakeConnectionResolver{}
	seedSession(t, sessions, resolver, "tenant-1", "pty-1", "conn-1")
	agent := &fakeDevServerAgentClient{agentStatusResult: AgentStatusResult{AgentRunning: true, ReadyForInput: false, AgentKind: "claude-code"}}
	lifecycleEvents := &fakeLifecycleEventPublisher{}
	queue := &fakeQueuedPromptRepository{byPtyID: map[string]domain.QueuedPrompt{
		"pty-1": {PtyID: "pty-1", TenantID: "tenant-1", Prompt: "hello agent", QueuedAt: time.Now()},
	}}
	liveStates := &sync.Map{}
	liveStates.Store("pty-1", &ptyLiveState{lastOutputAt: time.Now().Add(-5 * time.Second), agentRunning: true})
	uc := NewGetTerminalAgentStatus(sessions, resolver, agent, liveStates, lifecycleEvents, queue)

	ctx := withTenant(context.Background(), "tenant-1")

	result, err := uc.Execute(ctx, "pty-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.ReadyForInput {
		t.Fatal("expected ReadyForInput=true on the ready transition")
	}

	writePtyCalls := agent.writePtyCalls
	if len(writePtyCalls) != 1 {
		t.Fatalf("expected exactly one WritePty call, got %d", len(writePtyCalls))
	}
	if string(writePtyCalls[0]) != "hello agent" {
		t.Errorf("expected queued prompt %q written, got %q", "hello agent", string(writePtyCalls[0]))
	}
	if _, stillQueued, _ := queue.Get(ctx, "pty-1"); stillQueued {
		t.Error("expected queue row to be gone after drain")
	}

	// A second poll while still quiescent (readyNotified already true) must
	// not re-drain — no queue entry left, so a second WritePty call would
	// mean a double-delivery bug.
	if _, err := uc.Execute(ctx, "pty-1"); err != nil {
		t.Fatalf("unexpected error on second poll: %v", err)
	}
	if len(agent.writePtyCalls) != 1 {
		t.Fatalf("expected still exactly one WritePty call after a second poll, got %d", len(agent.writePtyCalls))
	}
}
