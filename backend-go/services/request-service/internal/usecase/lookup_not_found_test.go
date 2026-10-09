package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// Stubs embed the port so only the methods under test need bodies.
type missRequests struct {
	RequestRepository
	err error
}

func (s missRequests) Get(context.Context, string) (domain.Request, error) {
	return domain.Request{}, s.err
}
func (s missRequests) GetByNumber(context.Context, int64) (domain.Request, error) {
	return domain.Request{}, s.err
}

type presentClarifications struct{ ClarificationRepository }

func (presentClarifications) Get(context.Context, string) (domain.Clarification, error) {
	return domain.Clarification{ID: "c", RequestID: "r"}, nil
}

type presentDecisions struct{ DecisionRepository }

func (presentDecisions) Get(context.Context, string) (domain.Decision, error) {
	return domain.Decision{ID: "d", RequestID: "r"}, nil
}

type passThroughTx struct{}

func (passThroughTx) InTx(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

type missIndex struct {
	ArtifactIndexRepository
	err error
}

func (s missIndex) Resolve(context.Context, string) (domain.IndexEntry, error) {
	return domain.IndexEntry{}, s.err
}

type missRuns struct {
	AnalysisRunStore
	err error
}

func (s missRuns) Get(context.Context, string) (domain.AnalysisRun, error) {
	return domain.AnalysisRun{}, s.err
}

var errDBDown = errors.New("connection reset by peer")

// wantMissOrInternal pins TASK-E2E-017 for a lookup that re-labels a repository miss: a miss becomes the
// RPC's own not-found code, any other failure is passed through and is never a NotFound.
func wantMissOrInternal(t *testing.T, name string, run func(repoErr error) error, wantCode string) {
	t.Helper()
	t.Run(name+"/miss is the RPC not-found", func(t *testing.T) {
		err := run(domain.ErrRequestNotFound("r"))
		var ae *apperrors.AppError
		if !errors.As(err, &ae) || ae.Kind != apperrors.KindNotFound || ae.Code != wantCode {
			t.Fatalf("want NotFound %s, got %v", wantCode, err)
		}
	})
	t.Run(name+"/infrastructure error is not NotFound", func(t *testing.T) {
		err := run(errDBDown)
		if err == nil || isNotFoundKind(err) {
			t.Fatalf("want the database error passed through, got %v", err)
		}
		if !errors.Is(err, errDBDown) {
			t.Fatalf("cause lost: %v", err)
		}
	})
}

func TestLookupsDoNotTurnInfrastructureErrorsIntoNotFound(t *testing.T) {
	ctx := tenant.WithUserID(tenant.WithTenantID(context.Background(), "t1"), "u1")

	wantMissOrInternal(t, "ClarificationQueries.Get", func(e error) error {
		_, err := NewClarificationQueries(missRequests{err: e}, presentClarifications{}).Get(ctx, "c")
		return err
	}, "REQUEST_CLARIFICATION_NOT_FOUND")

	wantMissOrInternal(t, "DecisionQueries.Get", func(e error) error {
		_, _, err := NewDecisionQueries(missRequests{err: e}, presentDecisions{}).Get(ctx, "d")
		return err
	}, "REQUEST_DECISION_NOT_FOUND")

	wantMissOrInternal(t, "ConfirmDecision", func(e error) error {
		_, err := NewConfirmDecision(missRequests{err: e}, presentDecisions{}, passThroughTx{}, nil).Execute(ctx, ConfirmDecisionInput{DecisionID: "d"})
		return err
	}, "REQUEST_DECISION_NOT_FOUND")

	wantMissOrInternal(t, "ArtifactReader.Resolve(request number)", func(e error) error {
		_, err := NewArtifactReader(missRequests{err: e}, nil, nil, nil, nil, nil).Resolve(ctx, domain.FormatRequestID(1))
		return err
	}, "REQUEST_ARTIFACT_NOT_FOUND")

	wantMissOrInternal(t, "ExportArtifactProjection.exportSolution(index)", func(e error) error {
		uc := &ExportArtifactProjection{index: missIndex{err: e}}
		_, err := uc.exportSolution(ctx, domain.Request{ID: "r"}, "S-1")
		return err
	}, "REQUEST_ARTIFACT_NOT_FOUND")

	wantMissOrInternal(t, "GetPlanProposal.find(run)", func(e error) error {
		_, err := (&GetPlanProposal{runs: missRuns{err: e}}).find(ctx, "r", "run-1")
		return err
	}, "REQUEST_PLAN_RUN_NOT_FOUND")
}
