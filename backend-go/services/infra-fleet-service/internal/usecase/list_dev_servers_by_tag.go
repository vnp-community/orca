package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// ListDevServersByTagInput is ListDevServersByTag's request shape.
type ListDevServersByTagInput struct {
	Tag string
	// HealthyOnly filters the result against GetFleetHealth's own
	// reachability check (DevServerHealth.Reachable) — this usecase reuses
	// that existing health-tracking mechanism rather than adding a new one.
	HealthyOnly bool
}

// ListDevServersByTag backs workflow-service's "fleet:tag:<tag>"
// dispatch-target shape (TASK-WF-02-02/04): list this tenant's dev servers
// carrying tag, optionally filtered to only those GetFleetHealth's latest
// sample marks reachable.
type ListDevServersByTag struct {
	repo   DevServerRepository
	health FleetHealthPort
}

func NewListDevServersByTag(repo DevServerRepository, health FleetHealthPort) *ListDevServersByTag {
	return &ListDevServersByTag{repo: repo, health: health}
}

func (uc *ListDevServersByTag) Execute(ctx context.Context, in ListDevServersByTagInput) ([]domain.DevServer, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	if in.Tag == "" {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "INFRA_TAG_REQUIRED", "tag is required", nil)
	}

	devServers, err := uc.repo.ListByTag(ctx, tenantID, in.Tag)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "INFRA_LIST_DEV_SERVERS_BY_TAG_FAILED", "failed to list dev servers by tag", err)
	}
	if !in.HealthyOnly {
		return devServers, nil
	}

	samples, err := uc.health.GetFleetHealth(ctx, tenantID)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInternal, "INFRA_FLEET_HEALTH_FAILED", "failed to fetch fleet health", err)
	}
	reachable := make(map[string]bool, len(samples))
	for _, h := range samples {
		if h.Reachable {
			reachable[h.DevServerID] = true
		}
	}

	out := make([]domain.DevServer, 0, len(devServers))
	for _, ds := range devServers {
		if reachable[ds.ID] {
			out = append(out, ds)
		}
	}
	return out, nil
}
