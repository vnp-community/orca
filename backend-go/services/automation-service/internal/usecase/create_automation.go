package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/automation-service/internal/domain"
)

// maxAutomationsPerProject caps a project at 20 automations — BR-AT-02.
// ProjectID == "" (unscoped/back-compat) skips the cap.
const maxAutomationsPerProject = 20

// CreateAutomationInput mirrors the gRPC request 1:1 by design — see
// architecture/03's note that usecase granularity mirrors today's RPC
// methods so the TS->Go mapping stays traceable. TenantID is NOT trusted
// from the request body — see Execute below. DTStart/Timezone are the raw
// wire strings (RFC3339 / IANA tz name); empty means "default" — see
// Execute for the resolution rules.
type CreateAutomationInput struct {
	Name      string
	RRule     string
	ProjectID string // BR-AT-02; empty = unscoped
	// Actions is the preferred way to specify what this automation runs —
	// BR-AT-01. StepType/StepConfigJSON below are the deprecated
	// single-step back-compat path, used only when Actions is empty.
	Actions        []domain.AutomationAction
	StepType       domain.StepType
	StepConfigJSON string
	DTStart        string // RFC3339; empty = defaults to now
	Timezone       string // IANA tz name; empty = UTC
	TriggerType    domain.TriggerType
	TriggerEvent   domain.EventName
	TriggerFilter  *domain.TriggerFilter
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

	dtstart := now
	if in.DTStart != "" {
		parsed, err := time.Parse(time.RFC3339, in.DTStart)
		if err != nil {
			return domain.Automation{}, apperrors.New(apperrors.KindInvalidArgument, "AUTOMATION_INVALID_DTSTART", "dtstart must be RFC3339", err)
		}
		dtstart = parsed
	}

	timezone := in.Timezone
	if timezone != "" {
		loc, err := time.LoadLocation(timezone)
		if err != nil {
			return domain.Automation{}, apperrors.New(apperrors.KindInvalidArgument, "AUTOMATION_INVALID_TIMEZONE", "timezone must be a valid IANA tz name", err)
		}
		dtstart = dtstart.In(loc)
	}

	// BR-AT-02 — per-project cap, skipped for unscoped (ProjectID == "")
	// automations.
	if in.ProjectID != "" {
		count, err := uc.repo.CountByProject(ctx, tenantID, in.ProjectID)
		if err != nil {
			return domain.Automation{}, apperrors.New(apperrors.KindInternal, "AUTOMATION_COUNT_FAILED", "failed to count existing automations", err)
		}
		if count >= maxAutomationsPerProject {
			return domain.Automation{}, apperrors.New(apperrors.KindFailedPrecondition, "AUTOMATION_PROJECT_LIMIT_EXCEEDED", "project already has 20 automations", nil)
		}
	}

	// New automations default enabled=true — the generated
	// CreateAutomationRequest has no enabled field (Automation does; see
	// automation.proto), so there is nothing on the wire to read here.
	automation, err := domain.NewAutomation(domain.NewAutomationParams{
		ID:             uuid.NewString(),
		TenantID:       tenantID,
		ProjectID:      in.ProjectID,
		Name:           in.Name,
		RRule:          in.RRule,
		Actions:        in.Actions,
		StepType:       in.StepType,
		StepConfigJSON: in.StepConfigJSON,
		DTStart:        dtstart,
		Timezone:       timezone,
		Enabled:        true,
		CreatedAt:      now,
		TriggerType:    in.TriggerType,
		TriggerEvent:   in.TriggerEvent,
		TriggerFilter:  in.TriggerFilter,
	})
	if err != nil {
		return domain.Automation{}, apperrors.New(apperrors.KindInvalidArgument, "AUTOMATION_INVALID", err.Error(), err)
	}

	// BR-AT-10/BR-AT-04 — reject a create that would introduce a cycle in
	// the event-triggered automation graph. Only meaningful for
	// event-triggered automations; DetectTriggerCycle is a no-op graph
	// build otherwise (cron/manual automations never add edges).
	if automation.TriggerType == domain.TriggerTypeEvent {
		if err := DetectTriggerCycle(ctx, uc.repo, tenantID, automation); err != nil {
			return domain.Automation{}, err
		}
	}

	// Compute the FIRST next_run_at from rrule+dtstart so the scheduler has
	// something to claim without waiting for a first manual RunNow. Anchored
	// at dtstart itself (via a just-before-dtstart probe) rather than "now",
	// so a future dtstart's first occurrence is dtstart, and a past
	// dtstart's first occurrence is the earliest one — catch-up happens
	// naturally the next time the ticker runs, per §8's "a missed tick must
	// not silently drop a run".
	rule, err := automation.RecurrenceRule()
	if err != nil {
		// Unreachable: NewAutomation already validated rrule parses.
		return domain.Automation{}, apperrors.New(apperrors.KindInternal, "AUTOMATION_RRULE_BUILD_FAILED", "failed to build recurrence rule", err)
	}
	if next, ok := rule.NextOccurrenceAfter(dtstart.Add(-time.Second)); ok {
		automation.NextRunAt = next
	}

	if err := uc.repo.Create(ctx, automation); err != nil {
		return domain.Automation{}, apperrors.New(apperrors.KindInternal, "AUTOMATION_CREATE_FAILED", "failed to persist automation", err)
	}
	return automation, nil
}
