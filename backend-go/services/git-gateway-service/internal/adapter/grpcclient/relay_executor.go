package grpcclient

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// ErrRelayNotImplemented is returned by every RelayExecutor method — the
// connected-host dispatch path (git-gateway-service.md §2 step 3': relay via
// infra-fleet-service's provider-registry client to the Dev Server Agent) is
// not implemented in this scaffold.
var ErrRelayNotImplemented = errors.New("grpcclient: relay to Dev Server Agent via infra-fleet-service is not implemented in this scaffold")

// RelayExecutor is a STUB usecase.GitExecutor for the connected-host case
// (ConnectionResolver reports Connected=true). Every method returns
// ErrRelayNotImplemented rather than silently no-op'ing or falling back to
// local execution — git-gateway-service.md §8 explicitly requires a typed
// error on relay failure instead of a silent local fallback, since that
// would operate on the wrong worktree.
//
// TODO(git-gateway-service): replace with a real client that calls
// infra-fleet-service's provider-registry relay (Option A's existing wire
// protocol per architecture/08-inter-service-communication.md) to reach the
// Dev Server Agent's git handler (agent/src/relay/agent-git-handler.ts).
type RelayExecutor struct{}

func NewRelayExecutor() *RelayExecutor {
	return &RelayExecutor{}
}

func (r *RelayExecutor) GetStatus(_ context.Context, _ string) (domain.GitStatus, error) {
	return domain.GitStatus{}, ErrRelayNotImplemented
}

func (r *RelayExecutor) GetDiff(_ context.Context, _ string, _ bool) (domain.DiffResult, error) {
	return domain.DiffResult{}, ErrRelayNotImplemented
}

func (r *RelayExecutor) Commit(_ context.Context, _, _ string, _ []string) (domain.CommitResult, error) {
	return domain.CommitResult{}, ErrRelayNotImplemented
}

func (r *RelayExecutor) Push(_ context.Context, _, _, _ string) (domain.PushResult, error) {
	return domain.PushResult{}, ErrRelayNotImplemented
}

func (r *RelayExecutor) Pull(_ context.Context, _ string) (domain.PullResult, error) {
	return domain.PullResult{}, ErrRelayNotImplemented
}
