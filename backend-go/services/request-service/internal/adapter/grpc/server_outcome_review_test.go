package grpc

import (
	"context"
	"strings"
	"testing"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
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
	_, checks["artifact"] = s.GetReviewArtifact(ctx, &requestv1.GetReviewArtifactRequest{})
	_, checks["revisions"] = s.ListReviewArtifactRevisions(ctx, &requestv1.ListReviewArtifactRevisionsRequest{})
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
	if _, err := s.GetReviewArtifact(ctx, &requestv1.GetReviewArtifactRequest{RequestId: "r", ArtifactId: "a"}); status.Code(err) != codes.Unimplemented {
		t.Errorf("drawings that are not wired are not served: %v", err)
	}
	withStore := (&Server{}).WithOutcomeReview(OutcomeReviewUseCases{Service: &usecase.OutcomeReviewService{}, Artifacts: &usecase.ReviewArtifactStore{}})
	if _, err := withStore.GetReviewArtifact(ctx, &requestv1.GetReviewArtifactRequest{RequestId: "r"}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("an artifact is read by id: %v", err)
	}
	for _, bad := range []*requestv1.ListReviewArtifactRevisionsRequest{
		{RequestId: "r", Section: "config", Variant: "design"}, {RequestId: "r", Section: "flow", Variant: "full"}, {Section: "flow", Variant: "delta"},
	} {
		if _, err := withStore.ListReviewArtifactRevisions(ctx, bad); status.Code(err) != codes.InvalidArgument {
			t.Errorf("%+v: %v", bad, err)
		}
	}
	out, err := s.ListOutcomeReviewRuns(ctx, &requestv1.ListOutcomeReviewRunsRequest{RequestId: "r"})
	if err != nil || len(out.GetRuns()) != 0 {
		t.Errorf("no runner means no runs: %v %v", out, err)
	}
}

// REQUEST_REVIEW_KILL_SWITCH: every review RPC, a read as well as a write, answers REQUEST_REVIEW_DISABLED and touches nothing.
func TestOutcomeReviewRPC_KillSwitchClosesEveryRPC(t *testing.T) {
	svc := &usecase.OutcomeReviewService{Config: domain.OutcomeReviewConfig{KillSwitch: true}}
	s := (&Server{}).WithOutcomeReview(OutcomeReviewUseCases{
		Service: svc, Artifacts: &usecase.ReviewArtifactStore{}, Decider: &usecase.ReviewDecider{}, Regenerator: &usecase.ReviewRegenerator{}, Profiles: &usecase.ReviewDrawingProfiles{},
	})
	ctx := roleCtx("t1", "admin")
	calls := map[string]func() error{
		"estimate": func() error {
			_, err := s.EstimateOutcomeReview(ctx, &requestv1.EstimateOutcomeReviewRequest{RequestId: "r"})
			return err
		},
		"generate": func() error {
			_, err := s.GenerateOutcomeReview(ctx, &requestv1.GenerateOutcomeReviewRequest{RequestId: "r"})
			return err
		},
		"get": func() error {
			_, err := s.GetOutcomeReview(ctx, &requestv1.GetOutcomeReviewRequest{RequestId: "r"})
			return err
		},
		"list": func() error {
			_, err := s.ListOutcomeReviews(ctx, &requestv1.ListOutcomeReviewsRequest{RequestId: "r"})
			return err
		},
		"runs": func() error {
			_, err := s.ListOutcomeReviewRuns(ctx, &requestv1.ListOutcomeReviewRunsRequest{RequestId: "r"})
			return err
		},
		"artifact": func() error {
			_, err := s.GetReviewArtifact(ctx, &requestv1.GetReviewArtifactRequest{RequestId: "r", ArtifactId: "a"})
			return err
		},
		"revisions": func() error {
			_, err := s.ListReviewArtifactRevisions(ctx, &requestv1.ListReviewArtifactRevisionsRequest{RequestId: "r", Section: "flow", Variant: "delta"})
			return err
		},
		"decide": func() error {
			_, err := s.DecideOutcomeReviewItem(ctx, &requestv1.DecideOutcomeReviewItemRequest{RequestId: "r"})
			return err
		},
		"approve": func() error {
			_, err := s.ApproveOutcomeReview(ctx, &requestv1.ApproveOutcomeReviewRequest{RequestId: "r"})
			return err
		},
		"decisions": func() error {
			_, err := s.ListOutcomeReviewDecisions(ctx, &requestv1.ListOutcomeReviewDecisionsRequest{RequestId: "r"})
			return err
		},
		"link": func() error {
			_, err := s.LinkOutcomeReviewChild(ctx, &requestv1.LinkOutcomeReviewChildRequest{RequestId: "r"})
			return err
		},
		"regenerate": func() error {
			_, err := s.RegenerateReviewArtifacts(ctx, &requestv1.RegenerateReviewArtifactsRequest{RequestId: "r"})
			return err
		},
		"restore": func() error {
			_, err := s.RestoreReviewArtifactRevision(ctx, &requestv1.RestoreReviewArtifactRevisionRequest{RequestId: "r"})
			return err
		},
		"compare": func() error {
			_, err := s.CompareReviewArtifactRevisions(ctx, &requestv1.CompareReviewArtifactRevisionsRequest{RequestId: "r"})
			return err
		},
		"profile": func() error {
			_, err := s.GetReviewDrawingProfile(ctx, &requestv1.GetReviewDrawingProfileRequest{})
			return err
		},
		"save": func() error {
			_, err := s.SaveReviewDrawingProfile(ctx, &requestv1.SaveReviewDrawingProfileRequest{})
			return err
		},
		"reset": func() error {
			_, err := s.ResetReviewDrawingProfile(ctx, &requestv1.ResetReviewDrawingProfileRequest{})
			return err
		},
	}
	for name, call := range calls {
		err := call()
		if status.Code(err) != codes.FailedPrecondition || !strings.Contains(err.Error(), domain.CodeReviewDisabled) {
			t.Errorf("%s: %v, want REQUEST_REVIEW_DISABLED", name, err)
		}
	}
	// With the switch off again the same RPC is no longer refused for that reason (this one fails later, on its input).
	svc.Config.KillSwitch = false
	if _, err := s.GetOutcomeReview(context.Background(), &requestv1.GetOutcomeReviewRequest{}); status.Code(err) != codes.InvalidArgument {
		t.Errorf("switch off: %v", err)
	}
}
