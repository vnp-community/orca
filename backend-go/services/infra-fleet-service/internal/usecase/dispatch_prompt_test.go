package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// TestDispatchPrompt_AgentNotRunning_InjectsImmediately covers BR-MB-09: an
// idle (no agent) pty writes straight through, no queue row written.
func TestDispatchPrompt_AgentNotRunning_InjectsImmediately(t *testing.T) {
	sessions := &fakeTerminalSessionRepository{}
	resolver := &fakeConnectionResolver{}
	seedSession(t, sessions, resolver, "tenant-1", "pty-1", "conn-1")
	agent := &fakeDevServerAgentClient{agentStatusResult: AgentStatusResult{AgentRunning: false}}
	queue := &fakeQueuedPromptRepository{}
	uc := NewDispatchPrompt(sessions, resolver, agent, queue)

	ctx := withTenant(context.Background(), "tenant-1")
	out, err := uc.Execute(ctx, DispatchPromptInput{PtyID: "pty-1", Prompt: "do the thing", DeviceID: "device-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Outcome != "INJECTED_IMMEDIATELY" {
		t.Fatalf("expected INJECTED_IMMEDIATELY, got %q", out.Outcome)
	}
	if len(agent.writePtyCalls) != 1 || string(agent.writePtyCalls[0]) != "do the thing" {
		t.Fatalf("expected WritePty called once with the prompt, got %v", agent.writePtyCalls)
	}
	if len(queue.upsertCallsSnapshot()) != 0 {
		t.Error("expected no queue row written on immediate injection")
	}
}

// TestDispatchPrompt_AgentRunningNotReady_Queues covers BR-MB-10: an agent
// running and not yet ready holds the prompt instead of writing it.
func TestDispatchPrompt_AgentRunningNotReady_Queues(t *testing.T) {
	sessions := &fakeTerminalSessionRepository{}
	resolver := &fakeConnectionResolver{}
	seedSession(t, sessions, resolver, "tenant-1", "pty-1", "conn-1")
	agent := &fakeDevServerAgentClient{agentStatusResult: AgentStatusResult{AgentRunning: true, ReadyForInput: false}}
	queue := &fakeQueuedPromptRepository{}
	uc := NewDispatchPrompt(sessions, resolver, agent, queue)

	ctx := withTenant(context.Background(), "tenant-1")
	out, err := uc.Execute(ctx, DispatchPromptInput{PtyID: "pty-1", Prompt: "do the thing", DeviceID: "device-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Outcome != "QUEUED" {
		t.Fatalf("expected QUEUED, got %q", out.Outcome)
	}
	if len(agent.writePtyCalls) != 0 {
		t.Error("expected WritePty NOT called when queuing")
	}
	upserts := queue.upsertCallsSnapshot()
	if len(upserts) != 1 || upserts[0].Prompt != "do the thing" {
		t.Fatalf("expected one queue upsert with the prompt, got %v", upserts)
	}
}

// TestDispatchPrompt_ExistingQueuedPrompt_RejectsWithoutOverwrite covers
// BR-MB-12: a second dispatch onto an already-queued pty is rejected unless
// the caller explicitly confirms overwrite; the existing row is left
// unchanged and the preview matches the first 200 chars.
func TestDispatchPrompt_ExistingQueuedPrompt_RejectsWithoutOverwrite(t *testing.T) {
	sessions := &fakeTerminalSessionRepository{}
	resolver := &fakeConnectionResolver{}
	seedSession(t, sessions, resolver, "tenant-1", "pty-1", "conn-1")
	agent := &fakeDevServerAgentClient{agentStatusResult: AgentStatusResult{AgentRunning: true, ReadyForInput: false}}
	longPrompt := make([]byte, 250)
	for i := range longPrompt {
		longPrompt[i] = 'x'
	}
	queue := &fakeQueuedPromptRepository{}
	if err := queue.Upsert(context.Background(), mustQueuedPrompt(t, "pty-1", "tenant-1", string(longPrompt), "device-0")); err != nil {
		t.Fatalf("seed upsert failed: %v", err)
	}
	uc := NewDispatchPrompt(sessions, resolver, agent, queue)

	ctx := withTenant(context.Background(), "tenant-1")
	out, err := uc.Execute(ctx, DispatchPromptInput{PtyID: "pty-1", Prompt: "new prompt", DeviceID: "device-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Outcome != "REJECTED_NEEDS_CONFIRMATION" {
		t.Fatalf("expected REJECTED_NEEDS_CONFIRMATION, got %q", out.Outcome)
	}
	if out.ExistingPreview != string(longPrompt[:200]) {
		t.Errorf("expected preview to be first 200 chars of existing prompt, got len=%d", len(out.ExistingPreview))
	}
	existing, ok, err := queue.Get(ctx, "pty-1")
	if err != nil || !ok {
		t.Fatalf("expected existing row still present, ok=%v err=%v", ok, err)
	}
	if existing.Prompt != string(longPrompt) {
		t.Error("expected existing queued prompt to remain unchanged")
	}
	if len(agent.writePtyCalls) != 0 {
		t.Error("expected WritePty NOT called on rejection")
	}
}

// TestDispatchPrompt_ExistingQueuedPrompt_OverwriteReplacesRow covers the
// overwrite=true counterpart of BR-MB-12.
func TestDispatchPrompt_ExistingQueuedPrompt_OverwriteReplacesRow(t *testing.T) {
	sessions := &fakeTerminalSessionRepository{}
	resolver := &fakeConnectionResolver{}
	seedSession(t, sessions, resolver, "tenant-1", "pty-1", "conn-1")
	agent := &fakeDevServerAgentClient{agentStatusResult: AgentStatusResult{AgentRunning: true, ReadyForInput: false}}
	queue := &fakeQueuedPromptRepository{}
	if err := queue.Upsert(context.Background(), mustQueuedPrompt(t, "pty-1", "tenant-1", "old prompt", "device-0")); err != nil {
		t.Fatalf("seed upsert failed: %v", err)
	}
	uc := NewDispatchPrompt(sessions, resolver, agent, queue)

	ctx := withTenant(context.Background(), "tenant-1")
	out, err := uc.Execute(ctx, DispatchPromptInput{PtyID: "pty-1", Prompt: "new prompt", Overwrite: true, DeviceID: "device-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Outcome != "QUEUED" {
		t.Fatalf("expected QUEUED, got %q", out.Outcome)
	}
	existing, ok, err := queue.Get(ctx, "pty-1")
	if err != nil || !ok {
		t.Fatalf("expected a row present after overwrite, ok=%v err=%v", ok, err)
	}
	if existing.Prompt != "new prompt" {
		t.Errorf("expected row replaced with new prompt, got %q", existing.Prompt)
	}
}

func mustQueuedPrompt(t *testing.T, ptyID, tenantID, prompt, deviceID string) domain.QueuedPrompt {
	t.Helper()
	qp, err := domain.NewQueuedPrompt(ptyID, tenantID, prompt, deviceID, time.Now())
	if err != nil {
		t.Fatalf("failed to construct test QueuedPrompt: %v", err)
	}
	return qp
}
