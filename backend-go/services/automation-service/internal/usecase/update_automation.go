package usecase

import (
	"time"

	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/automation-service/internal/domain"
)

// UpdateAutomationInput uses pointer fields: nil = "not being changed"
// (matches the proto's wrapper-typed field-mask shape) — mirrors the
// convention of not conflating "empty string" with "unset."
type UpdateAutomationInput struct {
	TenantID       string
	ID             string
	Name           *string
	RRule          *string
	StepConfigJSON *string
	StepType       *domain.StepType
	Enabled        *bool
	Dtstart        *time.Time
	Timezone       *string
	ProjectID      *string
	// Actions replaces the automation's whole action chain when non-empty.
	// An empty Actions leaves the existing chain unchanged — mirrors
	// UpdateAutomationRequest's "empty = no change" convention
	// (automation.proto); a chain can never be updated to empty (BR-AT-01
	// requires at least one action, so "empty" is unambiguous as "no
	// change").
	Actions      []domain.AutomationAction
	TriggerType  *domain.TriggerType
	TriggerEvent *domain.EventName
	// TriggerFilterJSON mirrors the wire's StringValue: nil = no change,
	// "" = clear the filter, non-empty = replace it (parsed via
	// domain.ParseTriggerFilter).
	TriggerFilterJSON *string
}

// UpdateAutomation persists a partial edit of an existing Automation.
//
// Concurrency note: the scheduler ticker (adapter/scheduler/) reads
// enabled/next_run_at on its own ~1-minute cadence while this usecase can
// toggle enabled or change rrule concurrently. A read-modify-write Update
// (as implemented) has a narrow race with a concurrent scheduler claim
// (SELECT ... FOR UPDATE SKIP LOCKED) — accepted as "at-least-once, not
// exactly-once, by design" (a stale-enabled window before the next tick
// corrects it). Not solved here.
type UpdateAutomation struct {
	repo AutomationRepository
}

func NewUpdateAutomation(repo AutomationRepository) *UpdateAutomation {
	return &UpdateAutomation{repo: repo}
}

func (uc *UpdateAutomation) Execute(ctx context.Context, in UpdateAutomationInput) (domain.Automation, error) {
	current, err := uc.repo.Get(ctx, in.TenantID, in.ID)
	if err != nil {
		return domain.Automation{}, apperrors.New(apperrors.KindNotFound, "AUTOMATION_NOT_FOUND", "automation not found", err)
	}
	next := current
	if in.Name != nil {
		next.Name = *in.Name
	}
	if in.RRule != nil {
		next.RRule = *in.RRule
	}
	if in.StepConfigJSON != nil {
		next.StepConfigJSON = *in.StepConfigJSON
	}
	if in.StepType != nil {
		next.StepType = *in.StepType
	}
	if in.Enabled != nil {
		next.Enabled = *in.Enabled
	}
	if in.Dtstart != nil {
		next.DTStart = *in.Dtstart
	}
	if in.Timezone != nil {
		next.Timezone = *in.Timezone
	}
	if in.ProjectID != nil {
		next.ProjectID = *in.ProjectID
	}
	switch {
	case len(in.Actions) > 0:
		next.Actions = in.Actions
	case in.StepType != nil || in.StepConfigJSON != nil:
		// Legacy single-step update path: StepType/StepConfigJSON are the
		// deprecated mirror of Actions[0] (see domain.NewAutomation) — a
		// caller not yet updated to the chain shape edits them directly,
		// so replace the first action to match rather than leaving it
		// silently stale.
		onFailure := domain.OnFailureStop
		if len(next.Actions) > 0 {
			onFailure = next.Actions[0].OnFailure
		}
		next.Actions = []domain.AutomationAction{{StepType: next.StepType, StepConfigJSON: next.StepConfigJSON, OnFailure: onFailure}}
	}
	if in.TriggerType != nil {
		next.TriggerType = *in.TriggerType
	}
	if in.TriggerEvent != nil {
		next.TriggerEvent = *in.TriggerEvent
	}
	if in.TriggerFilterJSON != nil {
		filter, err := domain.ParseTriggerFilter(*in.TriggerFilterJSON)
		if err != nil {
			return domain.Automation{}, apperrors.New(apperrors.KindInvalidArgument, "AUTOMATION_INVALID_TRIGGER_FILTER", "trigger_filter_json is not valid JSON", err)
		}
		next.TriggerFilter = filter
	}
	// domain.Automation has no standalone Validate method — reuse
	// NewAutomation's invariant checks (non-empty name/rrule/actions,
	// rrule parses as RFC 5545, trigger fields consistent) by rebuilding
	// from the merged fields. A syntactically valid-at-create rule doesn't
	// stay valid-by-construction after an in-place field edit, so this
	// re-validates on every update.
	rebuilt, err := domain.NewAutomation(domain.NewAutomationParams{
		ID:             next.ID,
		TenantID:       next.TenantID,
		ProjectID:      next.ProjectID,
		Name:           next.Name,
		RRule:          next.RRule,
		Actions:        next.Actions,
		StepType:       next.StepType,
		StepConfigJSON: next.StepConfigJSON,
		DTStart:        next.DTStart,
		Timezone:       next.Timezone,
		Enabled:        next.Enabled,
		CreatedAt:      next.CreatedAt,
		TriggerType:    next.TriggerType,
		TriggerEvent:   next.TriggerEvent,
		TriggerFilter:  next.TriggerFilter,
	})
	if err != nil {
		return domain.Automation{}, apperrors.New(apperrors.KindInvalidArgument, "AUTOMATION_INVALID", err.Error(), err)
	}
	// NewAutomation defaults an unspecified/invalid StepType, empty
	// Timezone, unspecified TriggerType, and normalizes Actions — carry
	// those defaults forward, but keep next's own NextRunAt/UpdatedAt
	// (NewAutomation always returns them zero/reset, and this usecase
	// intentionally leaves scheduling fields untouched).
	next.StepType = rebuilt.StepType
	next.StepConfigJSON = rebuilt.StepConfigJSON
	next.Actions = rebuilt.Actions
	next.Timezone = rebuilt.Timezone
	next.TriggerType = rebuilt.TriggerType

	// BR-AT-10/BR-AT-04 — reject an update that would introduce a cycle in
	// the event-triggered automation graph.
	if next.TriggerType == domain.TriggerTypeEvent {
		if err := DetectTriggerCycle(ctx, uc.repo, in.TenantID, next); err != nil {
			return domain.Automation{}, err
		}
	}

	if err := uc.repo.Update(ctx, in.TenantID, next); err != nil {
		return domain.Automation{}, apperrors.New(apperrors.KindInternal, "AUTOMATION_UPDATE_FAILED", "failed to persist update", err)
	}
	return next, nil
}
