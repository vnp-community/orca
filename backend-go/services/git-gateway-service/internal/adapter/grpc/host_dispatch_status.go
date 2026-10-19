package grpc

import (
	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/usecase"
)

// toStatus maps a usecase error to a gRPC status. Usecases wrap dispatch
// failures in generic codes (WORKTREE_RESOLVE_FAILED, ...) whose cause is
// never sent; for host-routing failures the actionable inner code
// (REPO_HOST_UNBOUND, DEV_SERVER_UNREACHABLE) must reach the client.
func toStatus(err error) error {
	if hostErr := usecase.HostDispatchError(err); hostErr != nil {
		return apperrors.ToGRPCStatus(hostErr)
	}
	return apperrors.ToGRPCStatus(err)
}
