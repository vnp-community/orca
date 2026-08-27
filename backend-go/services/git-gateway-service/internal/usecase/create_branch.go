package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type CreateBranchInput struct {
	WorktreeID string
	Branch     string
	BaseRef    string
	Checkout   bool
}

// CreateBranch calls resolver.ResolveConnection inline — see MergeBranch's
// doc comment for why (needs Mode to fail closed on relay-ssh).
type CreateBranch struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewCreateBranch(resolver ConnectionResolver, local, relay GitExecutor) *CreateBranch {
	return &CreateBranch{resolver: resolver, local: local, relay: relay}
}

func (uc *CreateBranch) Execute(ctx context.Context, in CreateBranchInput) (string, error) {
	if in.WorktreeID == "" || in.Branch == "" {
		return "", apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_ARGS", "worktree_id and branch are required", nil)
	}
	conn, err := uc.resolver.ResolveConnection(ctx, in.WorktreeID)
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	if conn.Connected && conn.Mode == infrafleetv1.ConnectionMode_CONNECTION_MODE_RELAY_SSH {
		return "", apperrors.New(apperrors.KindFailedPrecondition, "GITGATEWAY_CREATE_BRANCH_UNSUPPORTED_SSH_RELAY", "create-branch is not supported over an SSH-relay connection", domain.ErrGitOpUnsupportedOverSSHRelay)
	}
	executor := uc.local
	if conn.Connected {
		executor = uc.relay
	}
	branch, err := executor.CreateBranch(ctx, conn.RepoPath, in.Branch, in.BaseRef, in.Checkout)
	if err != nil {
		return "", apperrors.New(apperrors.KindInternal, "GITGATEWAY_CREATE_BRANCH_FAILED", "failed to create branch", err)
	}
	return branch, nil
}
