package wscompat

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

type tokenChanAuth struct{}

func (tokenChanAuth) IssueMcpToken(_ context.Context, in *authv1.IssueMcpTokenRequest, _ ...grpc.CallOption) (*authv1.IssueMcpTokenResponse, error) {
	return &authv1.IssueMcpTokenResponse{
		Token:  &authv1.McpTokenInfo{Jti: "j1", Name: in.GetName(), Scopes: in.GetScopes(), CreatedAt: timestamppb.Now(), ExpiresAt: timestamppb.New(time.Now().Add(time.Hour))},
		Secret: "omp_S3CR3T",
	}, nil
}
func (tokenChanAuth) ListMcpTokens(context.Context, *emptypb.Empty, ...grpc.CallOption) (*authv1.ListMcpTokensResponse, error) {
	return &authv1.ListMcpTokensResponse{Tokens: []*authv1.McpTokenInfo{{Jti: "j1", Name: "ci", CreatedAt: timestamppb.Now(), ExpiresAt: timestamppb.New(time.Now().Add(time.Hour))}}}, nil
}
func (tokenChanAuth) RevokeMcpToken(context.Context, *authv1.RevokeMcpTokenRequest, ...grpc.CallOption) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil
}

type tokenChanPolicy struct{}

func (tokenChanPolicy) GetServerInfo(context.Context, *mcpv1.GetServerInfoRequest, ...grpc.CallOption) (*mcpv1.GetServerInfoResponse, error) {
	return &mcpv1.GetServerInfoResponse{Enabled: true, MaxTokenDays: 90}, nil
}
