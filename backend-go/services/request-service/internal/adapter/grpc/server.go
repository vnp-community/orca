package grpc

import (
	"context"

	"github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	requestv1.UnimplementedRequestServiceServer
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) GetRequest(ctx context.Context, req *requestv1.GetRequestRequest) (*requestv1.GetRequestResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetRequest not implemented")
}

func (s *Server) ListRequests(ctx context.Context, req *requestv1.ListRequestsRequest) (*requestv1.ListRequestsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListRequests not implemented")
}
