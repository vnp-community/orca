package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type ListRequests struct {
	repo   RequestRepository
	engine *RequestEngineResolver
}

// WithEngine makes every Request of the page carry the engine it runs on.
func (uc *ListRequests) WithEngine(r *RequestEngineResolver) *ListRequests {
	uc.engine = r
	return uc
}

func NewListRequests(repo RequestRepository) *ListRequests {
	return &ListRequests{repo: repo}
}

func (uc *ListRequests) Execute(ctx context.Context, f ListFilter) (ListResult, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return ListResult{}, domain.ErrRequestTenantRequired()
	}
	f, err := f.Normalize()
	if err != nil {
		return ListResult{}, err
	}
	// Filter values must match what CreateRequest stored, so spelling differences cannot hide a match.
	if f.SourceProvider != "" && f.SourceRef != "" {
		n, err := domain.NormalizeSourceRef(domain.SourceRef{Provider: domain.SourceProvider(f.SourceProvider), Site: f.SourceSite, Ref: f.SourceRef})
		if err != nil {
			return ListResult{}, err
		}
		f.SourceSite, f.SourceRef = n.Site, n.Ref
	}
	if scope, ok := ProjectScope(ctx); ok && f.ProjectID == "" {
		if len(scope) == 0 {
			return ListResult{}, nil // a member of no project sees no Requests.
		}
		f.ProjectIDs = scope
	}
	res, err := uc.repo.List(ctx, f)
	if err != nil {
		return ListResult{}, err
	}
	uc.engine.Annotate(ctx, res.Requests)
	return res, nil
}
