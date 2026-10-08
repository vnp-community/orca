package grpc

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc"
)

type captureAudit struct{ events []usecase.RPCAuditEvent }

func (c *captureAudit) Record(_ context.Context, e usecase.RPCAuditEvent) {
	c.events = append(c.events, e)
}

func callAudit(rec usecase.RPCAuditRecorder, ctx context.Context, method string, req, resp any, herr error) {
	_, _ = AuditRPC(rec)(ctx, req, &grpc.UnaryServerInfo{FullMethod: method}, func(context.Context, any) (any, error) { return resp, herr })
}

func auditUserCtx() context.Context {
	return tenant.WithUserID(tenant.WithTenantID(context.Background(), "t-1"), "u-1")
}

func TestAuditRPC_DeniedApprovalDecisionsAreRecorded(t *testing.T) {
	cases := []struct {
		method string
		req    any
		action string
	}{
		{approvalSvc + "Approve", &requestv1.ApproveRequest{Id: "a-1"}, domain.ActionApprovalApprove},
		{approvalSvc + "Reject", &requestv1.RejectRequest{Id: "a-1"}, domain.ActionApprovalReject},
		{approvalSvc + "Cancel", &requestv1.ApprovalServiceCancelRequest{Id: "a-1"}, domain.ActionApprovalCancel},
	}
	for _, tc := range cases {
		rec := &captureAudit{}
		callAudit(rec, auditUserCtx(), tc.method, tc.req, nil, apperrors.ToGRPCStatus(domain.ErrNotApprover))
		if len(rec.events) != 1 {
			t.Fatalf("%s: %d entries, want 1", tc.method, len(rec.events))
		}
		e := rec.events[0]
		if e.Action != tc.action || e.Outcome != domain.AuditOutcomeDenied || e.TargetType != "approval" || e.TargetID != "a-1" ||
			e.TenantID != "t-1" || e.ActorID != "u-1" || e.ActorKind != domain.AuditActorUser {
			t.Errorf("%s: %+v", tc.method, e)
		}
	}
}

func TestAuditRPC_OtherFailuresAndSuccessfulDecisionsAreNotRecordedHere(t *testing.T) {
	rec := &captureAudit{}
	callAudit(rec, auditUserCtx(), approvalSvc+"Approve", &requestv1.ApproveRequest{Id: "a-1"}, nil, apperrors.ToGRPCStatus(domain.ErrApprovalNotPending))
	callAudit(rec, auditUserCtx(), approvalSvc+"Approve", &requestv1.ApproveRequest{Id: "a-1"}, &requestv1.ApproveResponse{}, nil)
	callAudit(rec, auditUserCtx(), requestSvc+"GetRequest", &requestv1.GetRequestRequest{Id: "r"}, nil, apperrors.ToGRPCStatus(domain.ErrNotApprover))
	if len(rec.events) != 0 {
		t.Fatalf("unexpected entries %+v (allowed approvals come from the approval.decided event)", rec.events)
	}
}

func TestAuditRPC_SolutionChooseRecordsTheOptionAndNeverTheComment(t *testing.T) {
	rec := &captureAudit{}
	callAudit(rec, tenant.WithActorType(auditUserCtx(), tenant.ActorAgent), requestSvc+"ChooseSolutionOption",
		&requestv1.ChooseSolutionOptionRequest{RequestId: "r-1", SolutionId: "s-req", OptionId: "opt-2", Comment: "COMMENT-MARKER", Rationale: "RATIONALE-MARKER"},
		&requestv1.ChooseSolutionOptionResponse{Solution: &requestv1.Solution{Id: "s-1"}}, nil)
	if len(rec.events) != 1 {
		t.Fatalf("%d entries, want 1", len(rec.events))
	}
	e := rec.events[0]
	if e.Action != domain.ActionSolutionChoose || e.TargetType != "solution" || e.TargetID != "s-1" || e.Metadata["option"] != "opt-2" || e.ActorKind != domain.AuditActorAgent {
		t.Fatalf("%+v", e)
	}
	for _, v := range e.Metadata {
		if v == "COMMENT-MARKER" || v == "RATIONALE-MARKER" {
			t.Fatalf("free text leaked into metadata: %v", e.Metadata)
		}
	}
	callAudit(rec, auditUserCtx(), requestSvc+"ChooseSolutionOption", &requestv1.ChooseSolutionOptionRequest{OptionId: "opt-1"}, nil, apperrors.ToGRPCStatus(domain.ErrFlowDisabled()))
	if len(rec.events) != 1 {
		t.Fatal("a failed choose must not be audited")
	}
}
