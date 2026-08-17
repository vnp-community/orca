package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/automation-service/internal/domain"
)

// CreateAutomationInput mirrors the gRPC request 1:1 by design — see
// architecture/03's note that usecase granularity mirrors today's RPC
// methods so the TS->Go mapping stays traceable. TenantID is NOT trusted
// from the request body — see Execute below.
type CreateAutomationInput struct {
	Name           string
	RRule          string
	StepConfigJSON string
}

// CreateAutomation is automation-service's definition-creation path.
type CreateAutomation struct {
	repo AutomationRepository
}

func NewCreateAutomation(repo AutomationRepository) *CreateAutomation {
	return &CreateAutomation{repo: repo}
}

func (uc *CreateAutomation) Execute(ctx context.Context, in CreateAutomationInput) (domain.Automation, error) {
	// TenantID comes from context (populated by the gRPC tenant-extraction
	// interceptor from validated caller metadata), never from the request
	// body, per architecture/05-data-architecture.md's tenant-isolation rule
	// — even though CreateAutomationRequest happens to carry a tenant_id
	// field on the wire (see this service's README "deviations" note).
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.Automation{}, apperrors.New(apperrors.KindUnauthenticated, "AUTOMATION_NO_TENANT", "no tenant in request context", err)
	}

	now := time.Now().UTC()
	automation, err := domain.NewAutomation(uuid.NewString(), tenantID, in.Name, in.RRule, in.StepConfigJSON, now, now)
	if err != nil {
		return domain.Automation{}, apperrors.New(apperrors.KindInvalidArgument, "AUTOMATION_INVALID", err.Error(), err)
	}

	if err := uc.repo.Create(ctx, automation); err != nil {
		return domain.Automation{}, apperrors.New(apperrors.KindInternal, "AUTOMATION_CREATE_FAILED", "failed to persist automation", err)
	}
	return automation, nil
}
