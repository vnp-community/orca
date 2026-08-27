package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// CheckoutInput has no Create field — the real agent's git.checkout has no
// create-branch (-b) semantics, see GitExecutor.Checkout's doc comment.
type CheckoutInput struct {
	WorktreeID string
	Branch     string
}

type Checkout struct {
	resolver ConnectionResolver
	local    GitExecutor
	relay    GitExecutor
}

func NewCheckout(resolver ConnectionResolver, local, relay GitExecutor) *Checkout {
	return &Checkout{resolver: resolver, local: local, relay: relay}
}

func (uc *Checkout) Execute(ctx context.Context, in CheckoutInput) (domain.CheckoutResult, error) {
	if in.WorktreeID == "" {
		return domain.CheckoutResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_WORKTREE_ID", "worktree_id is required", nil)
	}
	if in.Branch == "" {
		return domain.CheckoutResult{}, apperrors.New(apperrors.KindInvalidArgument, "GITGATEWAY_MISSING_BRANCH", "branch is required", nil)
	}
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return domain.CheckoutResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	result, err := executor.Checkout(ctx, repoPath, in.Branch)
	if err != nil {
		return domain.CheckoutResult{}, apperrors.New(apperrors.KindInternal, "GITGATEWAY_CHECKOUT_FAILED", "failed to checkout ref", err)
	}
	return result, nil
}
