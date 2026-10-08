package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// LookupRequestBySource answers "does this external issue already have a Request?" using
// the same normalisation and key as CreateRequest, so spelling differences cannot hide a match.
type LookupRequestBySource struct {
	repo   RequestRepository
	idem   RequestIdempotencyRepository
	active ActiveSourceFinder
	flag   func(ctx context.Context) (bool, error)
}

// LookupOption switches the use case to the internal RPC semantics of CR-REQ-024.
type LookupOption func(*LookupRequestBySource)

// WithActiveSourceFinder answers only for Requests that are not completed or cancelled, lets an empty
// site match any site, and returns the id alone (never title or body).
func WithActiveSourceFinder(f ActiveSourceFinder) LookupOption {
	return func(u *LookupRequestBySource) { u.active = f }
}

// WithLookupFlagGate makes the lookup answer found=false while the request flow is off, so
// issue-status-sync falls back to its worktree/PR sync; an unreadable flag counts as off.
func WithLookupFlagGate(effective func(ctx context.Context) (bool, error)) LookupOption {
	return func(u *LookupRequestBySource) { u.flag = effective }
}

func NewLookupRequestBySource(repo RequestRepository, idem RequestIdempotencyRepository, opts ...LookupOption) *LookupRequestBySource {
	u := &LookupRequestBySource{repo: repo, idem: idem}
	for _, o := range opts {
		o(u)
	}
	return u
}

func (uc *LookupRequestBySource) Execute(ctx context.Context, ref domain.SourceRef) (domain.Request, bool, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return domain.Request{}, false, domain.ErrRequestTenantRequired()
	}
	n, err := domain.NormalizeSourceRef(ref)
	if err != nil {
		return domain.Request{}, false, err
	}
	if n.Provider == domain.SourceProviderManual || n.Provider == domain.SourceProviderMCP {
		return domain.Request{}, false, domain.ErrRequestSourceRefRequired(n.Provider)
	}
	if uc.active != nil {
		return uc.findActive(ctx, n)
	}
	id, err := uc.idem.Find(ctx, string(n.Provider), n.Site, n.Ref)
	if err != nil || id == "" {
		return domain.Request{}, false, err
	}
	r, err := uc.repo.Get(ctx, id)
	if err != nil {
		return domain.Request{}, false, err
	}
	return r, true, nil
}

func (uc *LookupRequestBySource) findActive(ctx context.Context, n domain.SourceRef) (domain.Request, bool, error) {
	if uc.flag != nil {
		on, err := uc.flag(ctx)
		if err != nil || !on {
			return domain.Request{}, false, nil
		}
	}
	id, ok, err := uc.active.FindActiveBySource(ctx, string(n.Provider), n.Site, n.Ref)
	if err != nil || !ok {
		return domain.Request{}, false, err
	}
	return domain.Request{ID: id}, true, nil
}
