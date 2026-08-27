package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type PushInputStream struct {
	WorktreeID string
	Remote     string
	Branch     string
}

// PushStream is Push's incremental-progress counterpart (TASK-PW-03-08,
// SOL-PW-03). Like MergeBranch, it calls resolver.ResolveConnection inline
// (rather than dispatchExecutor) because it needs ResolvedConnection.Mode
// directly, to fail closed on relay-ssh BEFORE ever attempting a stream —
// the agent's git.execStream has no relay-ssh equivalent (see
// domain.ErrGitOpUnsupportedOverSSHRelay's doc comment). The frontend is
// expected to retry against the unary Push RPC on that error, per this
// task's spec.
type PushStream struct {
	resolver ConnectionResolver
	local    StreamingGitExecutor
	relay    StreamingGitExecutor
}

func NewPushStream(resolver ConnectionResolver, local, relay StreamingGitExecutor) *PushStream {
	return &PushStream{resolver: resolver, local: local, relay: relay}
}

func (uc *PushStream) Execute(ctx context.Context, in PushInputStream, sink func(domain.GitProgressLine) error) error {
	if in.WorktreeID == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	conn, err := uc.resolver.ResolveConnection(ctx, in.WorktreeID)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	if conn.Connected && conn.Mode == infrafleetv1.ConnectionMode_CONNECTION_MODE_RELAY_SSH {
		return apperrors.New(apperrors.KindFailedPrecondition, "GITGATEWAY_PUSH_STREAM_UNSUPPORTED_SSH_RELAY", "push progress streaming is not supported over an SSH-relay connection; retry against the unary Push RPC", domain.ErrGitOpUnsupportedOverSSHRelay)
	}
	executor := uc.local
	if conn.Connected {
		executor = uc.relay
	}
	// Not wrapped in apperrors.New like Push.Execute's own
	// GITGATEWAY_PUSH_FAILED — sink's error (e.g. the gRPC adapter's
	// stream.Send failing because the client disconnected) must propagate
	// unmodified, not be reshaped into a generic "push failed" condition.
	return executor.PushStream(ctx, conn.RepoPath, in.Remote, in.Branch, sink)
}
