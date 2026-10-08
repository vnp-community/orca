package contracttest

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type stubArtifacts struct{}

func (stubArtifacts) Describe(_ context.Context, req domain.Request, st domain.SubjectType, _ string) (string, string, error) {
	return req.ID, domain.SubjectDigest(st, req.ID), nil
}
func (stubArtifacts) Decided(context.Context, domain.Approval, bool) error  { return nil }
func (stubArtifacts) Closed(context.Context, domain.Approval, string) error { return nil }

type stubTransition struct{}

func (stubTransition) Execute(context.Context, usecase.TransitionInput) (usecase.TransitionResult, error) {
	return usecase.TransitionResult{}, nil
}

type stubReturner struct{}

func (stubReturner) Execute(context.Context, usecase.ReturnInput) (domain.Request, error) {
	return domain.Request{}, nil
}

type stubLocker struct{ req domain.Request }

func (l stubLocker) LockRequest(context.Context, string) (domain.Request, error) { return l.req, nil }

func approvalFor(st domain.SubjectType, stage domain.RequestStatus, req domain.Request) func(string, string) domain.Approval {
	return func(id, digest string) domain.Approval {
		by := "u1"
		return domain.Approval{ID: "a1", RequestID: req.ID, SubjectType: st, SubjectID: id, Stage: string(stage), SubjectDigest: digest, DecidedBy: &by, Comment: "c"}
	}
}

func TestSubjectHandlerContract_TransitionSubjectHandler(t *testing.T) {
	for _, tc := range []struct {
		st     domain.SubjectType
		status domain.RequestStatus
		typ    domain.RequestType
	}{
		{domain.SubjectSolution, domain.RequestStatusAwaitingAnalysisApproval, domain.RequestTypeChangeRequest},
		{domain.SubjectFindings, domain.RequestStatusAwaitingAnalysisApproval, domain.RequestTypeSpike},
		{domain.SubjectAnswer, domain.RequestStatusAwaitingAnalysisApproval, domain.RequestTypeQuestion},
		{domain.SubjectPlan, domain.RequestStatusAwaitingPlanApproval, domain.RequestTypeChangeRequest},
		{domain.SubjectTaskList, domain.RequestStatusAwaitingPlanApproval, domain.RequestTypeTask},
		{domain.SubjectPhase, domain.RequestStatusExecuting, domain.RequestTypeChangeRequest},
		{domain.SubjectPreDeploy, domain.RequestStatusExecuting, domain.RequestTypeOpsRequest},
	} {
		t.Run(string(tc.st), func(t *testing.T) {
			RunSubjectHandlerContract(t, func(t *testing.T) SubjectHandlerFixture {
				req := domain.Request{ID: "r1", Status: tc.status, Type: tc.typ, Size: domain.RequestSizeM}
				return SubjectHandlerFixture{
					Handler: &usecase.TransitionSubjectHandler{Artifacts: stubArtifacts{}, Transition: stubTransition{}, Returner: stubReturner{}, Requests: stubLocker{req}},
					Subject: tc.st, Request: req, Approval: approvalFor(tc.st, tc.status, req),
				}
			})
		})
	}
}

func TestSubjectHandlerContract_RequestTypeApprovalHandler(t *testing.T) {
	RunSubjectHandlerContract(t, func(t *testing.T) SubjectHandlerFixture {
		req := domain.Request{ID: "r1", Status: domain.RequestStatusAwaitingTypeConfirmation, Type: domain.RequestTypeBug, Size: domain.RequestSizeM}
		return SubjectHandlerFixture{
			Handler: &usecase.RequestTypeApprovalHandler{Returner: stubReturner{}, Requests: stubLocker{req}},
			Subject: domain.SubjectRequestType, Request: req, Approval: approvalFor(domain.SubjectRequestType, req.Status, req),
		}
	})
}
