package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// IntakeChildCreator creates child Requests through the same core as CreateRequest, so a child
// also enters `classifying`, carries source_hints (type_hint included) and is idempotent per key.
type IntakeChildCreator struct{ create *CreateRequest }

var _ ChildRequestCreator = (*IntakeChildCreator)(nil)

func NewIntakeChildCreator(create *CreateRequest) *IntakeChildCreator {
	return &IntakeChildCreator{create: create}
}

// CreateChild must run inside the caller's transaction (SpawnChildRequest opens it).
func (c *IntakeChildCreator) CreateChild(ctx context.Context, in ChildRequestInput) (ChildRequestResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ChildRequestResult{}, domain.ErrRequestTenantRequired()
	}
	title, err := domain.NormalizeTitle(in.Title)
	if err != nil {
		return ChildRequestResult{}, err
	}
	body, err := domain.NormalizeBody(in.Body)
	if err != nil {
		return ChildRequestResult{}, err
	}
	src, err := domain.NormalizeSourceRef(domain.SourceRef{Provider: in.Provider})
	if err != nil {
		return ChildRequestResult{}, err
	}
	key, keyed, err := domain.BuildIdempotencyKey(src, in.ReporterID, in.IdempotencyKey)
	if err != nil {
		return ChildRequestResult{}, err
	}
	r, err := c.create.CreateWithinTx(ctx, CreateRequestInput{
		ProjectID: in.ProjectID, Title: title, Body: body, Source: src,
		Hints:         domain.SourceHints{TypeHint: string(in.TypeHint)},
		AllowTypeHint: true, ParentRequestID: in.ParentRequestID, LinkReason: string(in.LinkReason),
	}, src, tenantID, in.ReporterID, key, keyed)
	var lost claimLost
	if errors.As(err, &lost) {
		prior, gerr := c.create.repo.Get(ctx, lost.existingID)
		return ChildRequestResult{Request: prior}, gerr
	}
	if err != nil {
		return ChildRequestResult{}, err
	}
	return ChildRequestResult{Request: r, Created: true}, nil
}
