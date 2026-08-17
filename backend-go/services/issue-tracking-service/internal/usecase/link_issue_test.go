package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/domain"
)

// fakeEnqueuer is an in-memory OutboxEnqueuer — the "test against fakes,
// not a real database" pattern from
// specs/backend-go/standards/testing-strategy.md's unit-test section.
type fakeEnqueuer struct {
	enqueued []enqueuedEvent
	err      error
}

type enqueuedEvent struct {
	tenantID string
	event    domain.OutboxEvent
}

func (f *fakeEnqueuer) Enqueue(ctx context.Context, tenantID string, event domain.OutboxEvent) error {
	if f.err != nil {
		return f.err
	}
	f.enqueued = append(f.enqueued, enqueuedEvent{tenantID: tenantID, event: event})
	return nil
}

func withTenant(ctx context.Context, tenantID string) context.Context {
	return tenant.WithTenantID(ctx, tenantID)
}

func TestLinkIssue_RequiresTenantContext(t *testing.T) {
	uc := NewLinkIssue(&fakeEnqueuer{})
	err := uc.Execute(context.Background(), LinkIssueInput{IssueID: "PROJ-1", TaskID: "task-1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestLinkIssue_RequiresIssueAndTaskID(t *testing.T) {
	uc := NewLinkIssue(&fakeEnqueuer{})
	ctx := withTenant(context.Background(), "tenant-1")

	if err := uc.Execute(ctx, LinkIssueInput{TaskID: "task-1"}); err == nil {
		t.Error("expected an error when issue_id is empty")
	}
	if err := uc.Execute(ctx, LinkIssueInput{IssueID: "PROJ-1"}); err == nil {
		t.Error("expected an error when task_id is empty")
	}
}

func TestLinkIssue_EnqueuesLinkCreatedEvent(t *testing.T) {
	enq := &fakeEnqueuer{}
	uc := NewLinkIssue(enq)
	ctx := withTenant(context.Background(), "tenant-1")

	err := uc.Execute(ctx, LinkIssueInput{IssueID: "PROJ-1", TaskID: "task-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(enq.enqueued) != 1 {
		t.Fatalf("expected 1 enqueued event, got %d", len(enq.enqueued))
	}
	got := enq.enqueued[0]
	if got.tenantID != "tenant-1" || got.event.Subject != LinkCreatedSubject || got.event.ID == "" {
		t.Errorf("unexpected enqueued event: %+v", got)
	}
}

func TestLinkIssue_EnqueueFailurePropagates(t *testing.T) {
	enq := &fakeEnqueuer{err: errors.New("database unavailable")}
	uc := NewLinkIssue(enq)
	ctx := withTenant(context.Background(), "tenant-1")

	err := uc.Execute(ctx, LinkIssueInput{IssueID: "PROJ-1", TaskID: "task-1"})
	if err == nil {
		t.Fatal("expected error to propagate from enqueue failure")
	}
}

func TestLinkIssue_NilEnqueuerFailsClosed(t *testing.T) {
	uc := NewLinkIssue(nil)
	ctx := withTenant(context.Background(), "tenant-1")

	err := uc.Execute(ctx, LinkIssueInput{IssueID: "PROJ-1", TaskID: "task-1"})
	if err == nil {
		t.Fatal("expected an error when the outbox store is not configured")
	}
}
