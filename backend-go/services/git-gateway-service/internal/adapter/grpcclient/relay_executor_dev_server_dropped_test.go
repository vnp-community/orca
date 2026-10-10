package grpcclient

import (
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/usecase"
)

// The fleet-health sample can be 30s stale, so the agent may drop after
// dispatch. That must read as a retryable DEV_SERVER_UNREACHABLE, not as a
// git failure.
func TestRelayExecutor_DevServerNotConnected_MapsToUnreachable(t *testing.T) {
	fake := &fakeInfraFleetServiceClient{
		relayByDevServerErr: status.Error(codes.FailedPrecondition, "INFRA_DEV_SERVER_NOT_CONNECTED: this dev server has no live agent connection right now"),
	}
	r := NewRelayExecutor(fake)

	ctx := usecase.WithDevServerID(ctxWithTenant(t), "ds-1")
	_, err := r.ListWorktreePaths(ctx, "/repo")
	ae := usecase.HostDispatchError(err)
	if ae == nil || ae.Code != usecase.CodeDevServerUnreachable {
		t.Fatalf("expected %s, got %v", usecase.CodeDevServerUnreachable, err)
	}
}

func TestRelayExecutor_OtherRelayErrors_KeepTheirShape(t *testing.T) {
	fake := &fakeInfraFleetServiceClient{
		relayByDevServerErr: status.Error(codes.Internal, "INFRA_AGENT_EXEC_FAILED: boom"),
	}
	r := NewRelayExecutor(fake)

	ctx := usecase.WithDevServerID(ctxWithTenant(t), "ds-1")
	_, err := r.ListWorktreePaths(ctx, "/repo")
	if err == nil || usecase.HostDispatchError(err) != nil {
		t.Fatalf("expected a plain relay error, got %v", err)
	}
}
