package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestListPlanRevisions_UnimplementedUntilWired(t *testing.T) {
	_, err := NewServer(nil, nil).ListPlanRevisions(context.Background(), &requestv1.ListPlanRevisionsRequest{RequestId: "r"})
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("err = %v", err)
	}
}

func TestToProtoPlanRevisions(t *testing.T) {
	at := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	got := toProtoPlanRevisions(usecase.PlanRevisionsPage{
		NextPageToken: "2",
		Revisions: []usecase.PlanRevisionView{{
			Revision: 3, RunID: "run-3", Status: "current", Engine: "openspec", RevisionFeedback: "split it", ActorID: "u-1", GeneratedAt: at,
			Commit: "abc", FilesChanged: 2, FileCount: 5, TextKept: true,
		}},
	})
	if got.GetNextPageToken() != "2" || len(got.GetRevisions()) != 1 {
		t.Fatalf("got %v", got)
	}
	r := got.GetRevisions()[0]
	if r.GetRevision() != 3 || r.GetRunId() != "run-3" || r.GetStatus() != "current" || r.GetEngine() != "openspec" || r.GetRevisionFeedback() != "split it" ||
		r.GetActorId() != "u-1" || !r.GetGeneratedAt().AsTime().Equal(at) || r.GetCommit() != "abc" || r.GetFilesChanged() != 2 || r.GetFileCount() != 5 || !r.GetTextKept() || r.GetStats() != nil {
		t.Fatalf("revision = %v", r)
	}
}

func TestListPlanRevisions_MapsUseCaseErrorsToStatusCodes(t *testing.T) {
	uc := usecase.NewListPlanRevisions(requestsFailingGet{err: domain.ErrPlanRunNotFound("x")}, nil, nil, nil)
	s := NewServer(nil, nil).WithPlan(PlanUseCases{Revisions: uc})
	ctx := tenant.WithUserID(tenant.WithTenantID(context.Background(), "t1"), "u1")
	if _, err := s.ListPlanRevisions(ctx, &requestv1.ListPlanRevisionsRequest{RequestId: "r"}); status.Code(err) != codes.NotFound {
		t.Fatalf("err = %v", err)
	}
	if _, err := s.ListPlanRevisions(context.Background(), &requestv1.ListPlanRevisionsRequest{RequestId: "r"}); status.Code(err) == codes.OK {
		t.Fatal("a call without a tenant must be refused")
	}
}
