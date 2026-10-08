package grpc

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AuditRPC records the two decisions that no outbox event describes (usecase.AuditTap covers the rest):
// a refused approval decision (nothing commits, so only the RPC layer sees it) and solution.choose.
func AuditRPC(rec usecase.RPCAuditRecorder) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		var ev usecase.RPCAuditEvent
		var ok bool
		switch {
		case err == nil:
			ev, ok = chosenSolutionEvent(info.FullMethod, req, resp)
		case status.Code(err) == codes.PermissionDenied:
			ev, ok = deniedApprovalEvent(info.FullMethod, req)
		}
		if !ok {
			return resp, err
		}
		tenantID, terr := tenant.RequireTenantID(ctx)
		if terr != nil {
			return resp, err
		}
		ev.TenantID = tenantID
		ev.ActorID, _ = tenant.UserID(ctx)
		ev.ActorKind = domain.AuditActorUser
		if tenant.ActorType(ctx) == tenant.ActorAgent {
			ev.ActorKind = domain.AuditActorAgent
		}
		rec.Record(ctx, ev)
		return resp, err
	}
}

func chosenSolutionEvent(fullMethod string, req, resp any) (usecase.RPCAuditEvent, bool) {
	r, isReq := req.(*requestv1.ChooseSolutionOptionRequest)
	if fullMethod != requestSvc+"ChooseSolutionOption" || !isReq {
		return usecase.RPCAuditEvent{}, false
	}
	id := r.GetSolutionId()
	if out, ok := resp.(*requestv1.ChooseSolutionOptionResponse); ok && out.GetSolution().GetId() != "" {
		id = out.GetSolution().GetId()
	}
	return usecase.RPCAuditEvent{Action: domain.ActionSolutionChoose, TargetType: "solution", TargetID: id,
		Outcome: domain.AuditOutcomeAllowed, Metadata: map[string]string{"option": r.GetOptionId()}}, id != ""
}

func deniedApprovalEvent(fullMethod string, req any) (usecase.RPCAuditEvent, bool) {
	action, id := "", ""
	switch r := req.(type) {
	case *requestv1.ApproveRequest:
		if fullMethod == approvalSvc+"Approve" {
			action, id = domain.ActionApprovalApprove, r.GetId()
		}
	case *requestv1.RejectRequest:
		if fullMethod == approvalSvc+"Reject" {
			action, id = domain.ActionApprovalReject, r.GetId()
		}
	case *requestv1.ApprovalServiceCancelRequest:
		if fullMethod == approvalSvc+"Cancel" {
			action, id = domain.ActionApprovalCancel, r.GetId()
		}
	}
	return usecase.RPCAuditEvent{Action: action, TargetType: "approval", TargetID: id, Outcome: domain.AuditOutcomeDenied}, action != "" && id != ""
}
