package grpc

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/auth-service/internal/usecase"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

// DirectoryInternalMethods are guarded by their own shared secret so a holder of another internal secret (OAuth,
// MCP) cannot read the tenant directory.
var DirectoryInternalMethods = []string{authv1.AuthService_ListActiveTenantAdmins_FullMethodName}

// DirectoryServer layers the internal directory read over any AuthServiceServer (same wrapper pattern as McpServer).
type DirectoryServer struct {
	authv1.AuthServiceServer
	admins *usecase.ListActiveTenantAdmins
}

func WithDirectory(inner authv1.AuthServiceServer, admins *usecase.ListActiveTenantAdmins) *DirectoryServer {
	return &DirectoryServer{AuthServiceServer: inner, admins: admins}
}

func (s *DirectoryServer) ListActiveTenantAdmins(ctx context.Context, _ *authv1.ListActiveTenantAdminsRequest) (*authv1.ListActiveTenantAdminsResponse, error) {
	out, err := s.admins.Execute(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &authv1.ListActiveTenantAdminsResponse{UserIds: out.UserIDs, Truncated: out.Truncated}, nil
}
