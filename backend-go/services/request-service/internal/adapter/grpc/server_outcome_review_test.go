package grpc

import (
	"testing"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestOutcomeReviewRPC_UnimplementedUntilWired(t *testing.T) {
	s := &Server{}
	ctx := roleCtx("t1", "admin")
	checks := map[string]error{}
	_, checks["estimate"] = s.EstimateOutcomeReview(ctx, &requestv1.EstimateOutcomeReviewRequest{})
	_, checks["generate"] = s.GenerateOutcomeReview(ctx, &requestv1.GenerateOutcomeReviewRequest{})
	_, checks["get"] = s.GetOutcomeReview(ctx, &requestv1.GetOutcomeReviewRequest{})
	_, checks["list"] = s.ListOutcomeReviews(ctx, &requestv1.ListOutcomeReviewsRequest{})
	_, checks["runs"] = s.ListOutcomeReviewRuns(ctx, &requestv1.ListOutcomeReviewRunsRequest{})
	for name, err := range checks {
		if status.Code(err) != codes.Unimplemented {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestOutcomeReviewRPC_RefusesBadInputBeforeDoingAnything(t *testing.T) {
	s := (&Server{}).WithOutcomeReview(OutcomeReviewUseCases{Service: &usecase.OutcomeReviewService{}})
	ctx := roleCtx("t1", "admin")
	if _, err := s.GenerateOutcomeReview(ctx, &requestv1.GenerateOutcomeReviewRequest{RequestId: "r", Phase: "sometime"}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("a phase that does not exist: %v", err)
	}
	if _, err := s.GetOutcomeReview(ctx, &requestv1.GetOutcomeReviewRequest{ReviewId: "x"}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("a review is read through its Request: %v", err)
	}
	// Asking for a context run when none is wired is said plainly, not ignored.
	if _, err := s.GenerateOutcomeReview(ctx, &requestv1.GenerateOutcomeReviewRequest{RequestId: "r", RunContext: true}); status.Code(err) != codes.Unimplemented {
		t.Errorf("run_context without a runner: %v", err)
	}
	out, err := s.ListOutcomeReviewRuns(ctx, &requestv1.ListOutcomeReviewRunsRequest{RequestId: "r"})
	if err != nil || len(out.GetRuns()) != 0 {
		t.Errorf("no runner means no runs: %v %v", out, err)
	}
}
