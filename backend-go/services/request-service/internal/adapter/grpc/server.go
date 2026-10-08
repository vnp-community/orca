package grpc

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// Server implements RequestService. RPCs whose use cases are not attached (With*) answer Unimplemented.
type Server struct {
	requestv1.UnimplementedRequestServiceServer
	getRequest     *usecase.GetRequest
	listRequests   *usecase.ListRequests
	intake         IntakeUseCases
	classification ClassificationUseCases
	lifecycle      LifecycleUseCases
	solution       SolutionUseCases
	artifact       ArtifactUseCases
	clarification  ClarificationUseCases
	flowSettings   *usecase.FlowSettings
	compliance     ComplianceUseCases
	execution      ExecutionUseCases
}

func NewServer(getRequest *usecase.GetRequest, listRequests *usecase.ListRequests) *Server {
	return &Server{getRequest: getRequest, listRequests: listRequests}
}

func (s *Server) GetRequest(ctx context.Context, req *requestv1.GetRequestRequest) (*requestv1.GetRequestResponse, error) {
	r, err := s.getRequest.Execute(ctx, req.GetId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &requestv1.GetRequestResponse{Request: toProtoRequest(r)}, nil
}

func (s *Server) ListRequests(ctx context.Context, req *requestv1.ListRequestsRequest) (*requestv1.ListRequestsResponse, error) {
	f, err := filterFromProto(req)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	res, err := s.listRequests.Execute(ctx, f)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := &requestv1.ListRequestsResponse{NextPageToken: res.NextPageToken}
	for _, r := range res.Requests {
		out.Requests = append(out.Requests, toProtoRequest(r))
	}
	return out, nil
}
