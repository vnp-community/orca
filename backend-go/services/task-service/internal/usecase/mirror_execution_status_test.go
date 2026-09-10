package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// TestMirrorExecutionStatus_UnknownExternalRefIDIsNoOp proves BE-SOL-003/
// TASK-FT-003-05's idempotence contract: a status update for an unknown
// external_ref_id is a no-op, not an error.
func TestMirrorExecutionStatus_UnknownExternalRefIDIsNoOp(t *testing.T) {
	links := &fakeExecutionLinkRepository{}
	uc := NewMirrorExecutionStatus(links)

	err := uc.Execute(context.Background(), MirrorExecutionStatusInput{
		TenantID: "tenant-1", ExternalRefID: "unknown-ref", NewStatus: "completed",
	})
	if err != nil {
		t.Fatalf("expected a no-op for an unknown external_ref_id, got error: %v", err)
	}
}

// TestMirrorExecutionStatus_MatchingRefUpdatesStatusMirror proves the
// matching-row case: status_mirror is updated for the execution_links row
// whose external_ref_id matches.
func TestMirrorExecutionStatus_MatchingRefUpdatesStatusMirror(t *testing.T) {
	links := &fakeExecutionLinkRepository{}
	link, err := links.CreateExecutionLink(context.Background(), "tenant-1", "task-1", domain.EngineOrchestration, "run-123")
	if err != nil {
		t.Fatalf("seeding execution link: %v", err)
	}

	uc := NewMirrorExecutionStatus(links)
	if err := uc.Execute(context.Background(), MirrorExecutionStatusInput{
		TenantID: "tenant-1", ExternalRefID: "run-123", NewStatus: "completed",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := links.GetExecutionLink(context.Background(), "tenant-1", link.ID)
	if err != nil {
		t.Fatalf("re-fetching link: %v", err)
	}
	if got.StatusMirror != "completed" {
		t.Errorf("expected status_mirror completed, got %q", got.StatusMirror)
	}
}

// TestMirrorExecutionStatus_TenantScoped proves a matching external_ref_id
// under a DIFFERENT tenant is not updated — tenant isolation applies to
// this cross-service mirror the same as every other tenant-scoped write.
func TestMirrorExecutionStatus_TenantScoped(t *testing.T) {
	links := &fakeExecutionLinkRepository{}
	link, err := links.CreateExecutionLink(context.Background(), "tenant-1", "task-1", domain.EngineWorkflow, "exec-456")
	if err != nil {
		t.Fatalf("seeding execution link: %v", err)
	}

	uc := NewMirrorExecutionStatus(links)
	if err := uc.Execute(context.Background(), MirrorExecutionStatusInput{
		TenantID: "tenant-OTHER", ExternalRefID: "exec-456", NewStatus: "completed",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := links.GetExecutionLink(context.Background(), "tenant-1", link.ID)
	if err != nil {
		t.Fatalf("re-fetching link: %v", err)
	}
	if got.StatusMirror == "completed" {
		t.Error("expected a different tenant's matching ref to NOT update status_mirror")
	}
}
