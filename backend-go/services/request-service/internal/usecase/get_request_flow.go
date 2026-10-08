package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// FlowView is the registry entry plus the standard status path for one type and size.
// StatusPath excludes the backlog, type-change and cancel branches.
type FlowView struct {
	Flow       domain.FlowDefinition
	HasPhases  bool
	StatusPath []domain.RequestStatus
}

// GetRequestFlow reads the in-memory registry only, so the frontend need not copy it.
type GetRequestFlow struct{}

func NewGetRequestFlow() *GetRequestFlow { return &GetRequestFlow{} }

// Execute accepts an empty size, which counts as "not L".
func (uc *GetRequestFlow) Execute(ctx context.Context, typ, size string) (FlowView, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return FlowView{}, domain.ErrRequestTenantRequired()
	}
	rt, err := domain.ParseRequestType(typ)
	if err != nil {
		return FlowView{}, err
	}
	var sz domain.RequestSize
	if size != "" {
		if sz, err = domain.ParseSize(size); err != nil {
			return FlowView{}, err
		}
	}
	flow, err := domain.FlowFor(rt)
	if err != nil {
		return FlowView{}, err
	}
	path, err := domain.HappyPath(flow, sz)
	if err != nil {
		return FlowView{}, err
	}
	return FlowView{Flow: flow, HasPhases: flow.PhasesFor(sz), StatusPath: path}, nil
}
