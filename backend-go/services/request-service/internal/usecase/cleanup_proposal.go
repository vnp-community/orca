package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// CleanupProposal is the manual door to the janitor: the Request's reporter or an administrator removes the worktree of the
// Request's OpenSpec proposal. The branch and the stored change stay, so the proposal can still be read.
type CleanupProposal struct {
	requests RequestRepository
	janitor  *ProposalJanitor
}

func NewCleanupProposal(requests RequestRepository, janitor *ProposalJanitor) *CleanupProposal {
	return &CleanupProposal{requests: requests, janitor: janitor}
}

func (uc *CleanupProposal) Execute(ctx context.Context, requestID string) (ProposalRelease, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return ProposalRelease{}, domain.ErrRequestTenantRequired()
	}
	req, err := loadReadableRequest(ctx, uc.requests, requestID)
	if err != nil {
		return ProposalRelease{}, err
	}
	if callerIsMachine(ctx) || !canWriteRequest(ctx, req) {
		return ProposalRelease{}, errRequestForbidden("only the reporter or an administrator cleans up a proposal")
	}
	rel, err := uc.janitor.Cleanup(ctx, req.ID)
	if errors.Is(err, ErrProposalRunActive) {
		return rel, domain.ErrProposalRunActive(req.ID)
	}
	return rel, err
}
