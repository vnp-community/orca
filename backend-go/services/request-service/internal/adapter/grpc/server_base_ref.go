package grpc

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *Server) SetRequestBaseRef(ctx context.Context, req *requestv1.SetRequestBaseRefRequest) (*requestv1.SetRequestBaseRefResponse, error) {
	if s.baseRef.Set == nil {
		return nil, status.Error(codes.Unimplemented, "SetRequestBaseRef is not wired")
	}
	actor, err := actorFromContext(ctx)
	if err != nil {
		return nil, baseRefStatus(err)
	}
	// An agent session never changes the ref (the use case refuses it), so the kind comes from the caller, not a constant.
	kind := domain.ActorKindUser
	if tenant.ActorType(ctx) == tenant.ActorAgent {
		kind = domain.ActorKindAgent
	}
	res, err := s.baseRef.Set.Execute(ctx, usecase.SetRequestBaseRefInput{
		RequestID: req.GetRequestId(), BaseRef: req.GetBaseRef(), ExpectedVersion: req.GetExpectedVersion(),
		ActorID: actor, ActorKind: kind,
	})
	if err != nil {
		return nil, baseRefStatus(err)
	}
	return &requestv1.SetRequestBaseRefResponse{Request: toProtoRequest(res.Request), BaseRefUnverified: res.Unverified}, nil
}
