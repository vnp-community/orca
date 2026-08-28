package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type PullInputStream struct {
	WorktreeID string
}

// PullStream is Pull's incremental-progress counterpart — see
// PushStream's doc comment for the shared relay-ssh-fails-closed shape and
// rationale; this differs only in which StreamingGitExecutor method it
// calls and PullInputStream's narrower field set (mirrors PullInput).
type PullStream struct {
	resolver ConnectionResolver
	local    StreamingGitExecutor
	relay    StreamingGitExecutor
}

func NewPullStream(resolver ConnectionResolver, local, relay StreamingGitExecutor) *PullStream {
	return &PullStream{resolver: resolver, local: local, relay: relay}
}

func (uc *PullStream) Execute(ctx context.Context, in PullInputStream, sink func(domain.GitProgressLine) error) error {
	if in.WorktreeID == "" {
		return apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	conn, err := uc.resolver.ResolveConnection(ctx, in.WorktreeID)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	if conn.Connected && conn.Mode == infrafleetv1.ConnectionMode_CONNECTION_MODE_RELAY_SSH {
		return apperrors.New(apperrors.KindFailedPrecondition, "GITGATEWAY_PULL_STREAM_UNSUPPORTED_SSH_RELAY", "pull progress streaming is not supported over an SSH-relay connection; retry against the unary Pull RPC", domain.ErrGitOpUnsupportedOverSSHRelay)
	}
	executor := uc.local
	if conn.Connected {
		executor = uc.relay
	}
	// Not wrapped in apperrors.New like Pull.Execute's own
	// GITGATEWAY_PULL_FAILED — see PushStream.Execute's identical note.
	return executor.PullStream(ctx, conn.RepoPath, sink)
}
