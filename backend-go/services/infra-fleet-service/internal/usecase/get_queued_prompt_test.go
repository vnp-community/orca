package usecase

import (
	"context"
	"testing"
	"time"
)

func TestGetQueuedPrompt_RequiresTenantContext(t *testing.T) {
	uc := NewGetQueuedPrompt(&fakeTerminalSessionRepository{}, &fakeConnectionResolver{}, &fakeQueuedPromptRepository{})
	_, _, _, err := uc.Execute(context.Background(), "pty-1")
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestGetQueuedPrompt_NoQueuedPrompt(t *testing.T) {
	sessions := &fakeTerminalSessionRepository{}
	resolver := &fakeConnectionResolver{}
	seedSession(t, sessions, resolver, "tenant-1", "pty-1", "conn-1")
	uc := NewGetQueuedPrompt(sessions, resolver, &fakeQueuedPromptRepository{})

	ctx := withTenant(context.Background(), "tenant-1")
	has, prompt, queuedAt, err := uc.Execute(ctx, "pty-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if has || prompt != "" || queuedAt != 0 {
		t.Errorf("expected zero-value result when nothing queued, got has=%v prompt=%q queuedAt=%d", has, prompt, queuedAt)
	}
}

func TestGetQueuedPrompt_ReturnsQueuedPrompt(t *testing.T) {
	sessions := &fakeTerminalSessionRepository{}
	resolver := &fakeConnectionResolver{}
	seedSession(t, sessions, resolver, "tenant-1", "pty-1", "conn-1")
	now := time.Now()
	prompt := mustQueuedPrompt(t, "pty-1", "tenant-1", "queued text", "device-1")
	prompt.QueuedAt = now
	queue := &fakeQueuedPromptRepository{}
	if err := queue.Upsert(context.Background(), prompt); err != nil {
		t.Fatalf("seed upsert failed: %v", err)
	}
	uc := NewGetQueuedPrompt(sessions, resolver, queue)

	ctx := withTenant(context.Background(), "tenant-1")
	has, gotPrompt, queuedAt, err := uc.Execute(ctx, "pty-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !has || gotPrompt != "queued text" || queuedAt != now.UnixMilli() {
		t.Errorf("expected has=true prompt=%q queuedAt=%d, got has=%v prompt=%q queuedAt=%d", "queued text", now.UnixMilli(), has, gotPrompt, queuedAt)
	}
}
