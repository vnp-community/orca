package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// AssignDevServerGroupInput's GroupID empty means "unassign" (ungroup).
type AssignDevServerGroupInput struct {
	DevServerID string
	GroupID     string
}

// AssignDevServerGroup sets a dev server's group — admin-gated. Does NOT
// validate GroupID belongs to the caller's tenant before assigning — the
// Postgres FK (infra.dev_servers.group_id REFERENCES
// infra.dev_server_groups(id)) rejects a nonexistent id, and RLS scopes
// both tables to the same tenant, so a cross-tenant group id can never
// exist to assign in the first place.
type AssignDevServerGroup struct {
	repo DevServerRepository
}

func NewAssignDevServerGroup(repo DevServerRepository) *AssignDevServerGroup {
	return &AssignDevServerGroup{repo: repo}
}

func (uc *AssignDevServerGroup) Execute(ctx context.Context, in AssignDevServerGroupInput) (domain.DevServer, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.DevServer{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	if err := requireAdmin(ctx); err != nil {
		return domain.DevServer{}, err
	}

	ds, err := uc.repo.AssignGroup(ctx, tenantID, in.DevServerID, in.GroupID)
	if err != nil {
		return domain.DevServer{}, apperrors.New(apperrors.KindInternal, "INFRA_ASSIGN_DEV_SERVER_GROUP_FAILED", "failed to assign dev server group", err)
	}
	return ds, nil
}
