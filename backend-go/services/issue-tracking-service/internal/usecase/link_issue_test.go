package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
)

// fakePublisher is an in-memory EventPublisher — the "test against fakes,
// not a real NATS broker" pattern from
// specs/backend-go/standards/testing-strategy.md's unit-test section.
type fakePublisher struct {
	published []publishedLink
	err       error
}

type publishedLink struct {
	tenantID string
	issueID  string
	taskID   string
}

func (f *fakePublisher) PublishLinkCreated(ctx context.Context, tenantID, issueID, taskID string) error {
	if f.err != nil {
		return f.err
	}
	f.published = append(f.published, publishedLink{tenantID: tenantID, issueID: issueID, taskID: taskID})
	return nil
}

func withTenant(ctx context.Context, tenantID string) context.Context {
	return tenant.WithTenantID(ctx, tenantID)
}

func TestLinkIssue_RequiresTenantContext(t *testing.T) {
	uc := NewLinkIssue(&fakePublisher{})
	err := uc.Execute(context.Background(), LinkIssueInput{IssueID: "PROJ-1", TaskID: "task-1"})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestLinkIssue_RequiresIssueAndTaskID(t *testing.T) {
	uc := NewLinkIssue(&fakePublisher{})
	ctx := withTenant(context.Background(), "tenant-1")

	if err := uc.Execute(ctx, LinkIssueInput{TaskID: "task-1"}); err == nil {
		t.Error("expected an error when issue_id is empty")
	}
	if err := uc.Execute(ctx, LinkIssueInput{IssueID: "PROJ-1"}); err == nil {
		t.Error("expected an error when task_id is empty")
	}
}

func TestLinkIssue_PublishesLinkCreatedEvent(t *testing.T) {
	pub := &fakePublisher{}
	uc := NewLinkIssue(pub)
	ctx := withTenant(context.Background(), "tenant-1")

	err := uc.Execute(ctx, LinkIssueInput{IssueID: "PROJ-1", TaskID: "task-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(pub.published) != 1 {
		t.Fatalf("expected 1 published event, got %d", len(pub.published))
	}
	got := pub.published[0]
	if got.tenantID != "tenant-1" || got.issueID != "PROJ-1" || got.taskID != "task-1" {
		t.Errorf("unexpected published event: %+v", got)
	}
}

func TestLinkIssue_PublishFailurePropagates(t *testing.T) {
	pub := &fakePublisher{err: errors.New("nats unavailable")}
	uc := NewLinkIssue(pub)
	ctx := withTenant(context.Background(), "tenant-1")

	err := uc.Execute(ctx, LinkIssueInput{IssueID: "PROJ-1", TaskID: "task-1"})
	if err == nil {
		t.Fatal("expected error to propagate from publisher failure")
	}
}

func TestLinkIssue_NilPublisherFailsClosed(t *testing.T) {
	uc := NewLinkIssue(nil)
	ctx := withTenant(context.Background(), "tenant-1")

	err := uc.Execute(ctx, LinkIssueInput{IssueID: "PROJ-1", TaskID: "task-1"})
	if err == nil {
		t.Fatal("expected an error when the event bus is not configured")
	}
}
