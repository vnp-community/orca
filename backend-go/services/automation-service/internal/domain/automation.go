// Package domain holds automation-service's entities and value objects. Per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md, this
// package has zero imports outside stdlib, the RRULE library, and other
// domain/ files — no database, no gRPC, no framework.
package domain

import (
	"errors"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
)

// StepType mirrors workflow-service's StepType enum (see workflow.proto) —
// duplicated here rather than imported so domain/ stays free of the
// workflowv1 dependency; internal/adapter/grpcclient is the only place that
// translates between the two.
type StepType string

const (
	StepTypeUnspecified  StepType = ""
	StepTypeAgent        StepType = "agent"
	StepTypeShell        StepType = "shell"
	StepTypeNotification StepType = "notification"
	StepTypeWebhook      StepType = "webhook"
	StepTypeCondition    StepType = "condition"
	// StepTypeCleanupWorktrees lets an automation dispatch BL-AT-04's bulk
	// worktree-cleanup step on a schedule — mirrors
	// workflow-service.domain.StepTypeCleanupWorktrees.
	StepTypeCleanupWorktrees StepType = "cleanup_worktrees"
)

func (s StepType) Valid() bool {
	switch s {
	case StepTypeAgent, StepTypeShell, StepTypeNotification, StepTypeWebhook, StepTypeCondition, StepTypeCleanupWorktrees:
		return true
	default:
		return false
	}
}

var (
	// ErrEmptyTenant is returned when TenantID is empty — an automation with
	// no owning tenant is never a valid domain state.
	ErrEmptyTenant = errors.New("domain: tenant_id is required")
	// ErrEmptyName guards against a nameless automation, which would be
	// unidentifiable in any list/UI surface.
	ErrEmptyName = errors.New("domain: name is required")
	// ErrEmptyRRule guards against an automation with no recurrence — even a
	// manual-only automation needs a rule per the schema in
	// specs/backend-go/services/automation-service.md §5 (NOT NULL rrule).
	ErrEmptyRRule = errors.New("domain: rrule is required")
	// ErrInvalidRRule is returned when RRule fails to parse as an RFC 5545
	// recurrence string.
	ErrInvalidRRule = errors.New("domain: rrule is not a valid RFC 5545 recurrence rule")
	// ErrEmptyStepConfig guards against an AutomationRun with nothing to
	// execute recorded on it — see automation_run.go.
	ErrEmptyStepConfig = errors.New("domain: step_config_json is required")
	// ErrEmptyActions guards against an Automation with no work to do —
	// BR-AT-01: every automation must have at least one action in its chain.
	ErrEmptyActions = apperrors.New(apperrors.KindInvalidArgument, "AUTOMATION_EMPTY_ACTIONS", "automation must have at least one action", nil)
)

// Automation is a scheduled/triggered automation definition — the system of
// record owned by this service. Execution never happens here; RunNow
// delegates to workflow-service.ExecuteAdHocStep (see
// specs/backend-go/services/automation-service.md §2).
type Automation struct {
	ID       string
	TenantID string
	// ProjectID is a logical FK -> project-service.projects; empty means
	// unscoped (back-compat with pre-project-cap rows) — BR-AT-02.
	ProjectID string
	Name      string
	RRule     string
	// StepType/StepConfigJSON are DEPRECATED but kept as the *first action's*
	// mirror for back-compat reads/writes — see Actions below, the new
	// source of truth for what a dispatch actually runs (BR-AT-01).
	StepType       StepType
	StepConfigJSON string
	// Actions is the ordered chain a dispatch runs — BR-AT-01. Always
	// non-empty on a valid Automation (see NewAutomation).
	Actions []AutomationAction
	DTStart time.Time
	// Timezone is the IANA tz name RRULE occurrences are computed in;
	// always resolved to a concrete value ("UTC" if unset) by NewAutomation
	// so every Automation is structurally ready for scheduling.
	Timezone string
	// Enabled gates the scheduler ticker's due-row query
	// (WHERE enabled AND next_run_at <= now()) — a disabled automation is
	// never claimed even if its next_run_at is in the past.
	Enabled bool
	// TriggerType/TriggerEvent/TriggerFilter — BR-AT-09/BL-AT-03's
	// trigger schema. TriggerType defaults to TriggerTypeCron (back-compat
	// with rrule-only rows); TriggerEvent/TriggerFilter are only meaningful
	// when TriggerType == TriggerTypeEvent.
	TriggerType   TriggerType
	TriggerEvent  EventName
	TriggerFilter *TriggerFilter
	// NextRunAt is the next time the scheduler should dispatch this
	// automation; zero means "no further occurrences" (an exhausted
	// COUNT/UNTIL-bounded rule) or "not yet computed". Advanced by
	// internal/adapter/scheduler after each dispatch.
	NextRunAt time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewAutomationParams bundles NewAutomation's inputs. Introduced once the
// original positional-parameter constructor outgrew a readable argument
// list (BR-AT-01's actions chain and BR-AT-09's trigger fields stacked on
// top of the original 10 params) — ID/TenantID/Name/RRule/DTStart/Enabled/
// CreatedAt are required; everything else is optional with a documented
// default.
type NewAutomationParams struct {
	ID        string
	TenantID  string
	ProjectID string // optional; empty = unscoped (back-compat)
	Name      string
	RRule     string
	// Actions is the preferred way to specify what this automation runs.
	// When empty, StepType/StepConfigJSON (the deprecated single-step pair)
	// are normalized into a one-element Actions list instead — back-compat
	// for callers not yet updated to the chain shape.
	Actions        []AutomationAction
	StepType       StepType // deprecated; used only when Actions is empty
	StepConfigJSON string   // deprecated; used only when Actions is empty
	DTStart        time.Time
	Timezone       string // optional; empty = UTC
	Enabled        bool
	CreatedAt      time.Time
	TriggerType    TriggerType // optional; empty = TriggerTypeCron
	TriggerEvent   EventName   // required (one of the 5 documented names) iff TriggerType == TriggerTypeEvent
	TriggerFilter  *TriggerFilter
}

// NewAutomation constructs an Automation, enforcing the invariants a
// definition must satisfy to be dispatchable — including that RRule parses,
// so a malformed recurrence string is rejected at creation time rather than
// discovered later by the scheduler loop, and that the action chain is
// non-empty (BR-AT-01). NextRunAt is left zero; usecase.CreateAutomation
// computes it from the resulting RecurrenceRule.
func NewAutomation(p NewAutomationParams) (Automation, error) {
	if p.TenantID == "" {
		return Automation{}, ErrEmptyTenant
	}
	if p.Name == "" {
		return Automation{}, ErrEmptyName
	}
	if p.RRule == "" {
		return Automation{}, ErrEmptyRRule
	}
	if _, err := NewRecurrenceRule(p.RRule, p.DTStart); err != nil {
		return Automation{}, err
	}

	// Actions resolution — BR-AT-01: a populated Actions list wins; a
	// back-compat single StepType/StepConfigJSON pair is normalized into a
	// one-element Actions list; neither present is invalid.
	actions := make([]AutomationAction, len(p.Actions))
	copy(actions, p.Actions)
	if len(actions) == 0 && p.StepConfigJSON != "" {
		st := p.StepType
		if !st.Valid() {
			st = StepTypeAgent
		}
		actions = []AutomationAction{{StepType: st, StepConfigJSON: p.StepConfigJSON, OnFailure: OnFailureStop}}
	}
	if len(actions) == 0 {
		return Automation{}, ErrEmptyActions
	}
	for i := range actions {
		if !actions[i].StepType.Valid() {
			actions[i].StepType = StepTypeAgent
		}
		if actions[i].OnFailure == "" {
			actions[i].OnFailure = OnFailureStop
		} else if !actions[i].OnFailure.Valid() {
			return Automation{}, apperrors.New(apperrors.KindInvalidArgument, "AUTOMATION_INVALID_ON_FAILURE", "invalid on_failure policy", nil)
		}
	}

	trigger := p.TriggerType
	if trigger == "" {
		trigger = TriggerTypeCron // back-compat default
	}
	if trigger == TriggerTypeEvent {
		if !p.TriggerEvent.Valid() {
			return Automation{}, apperrors.New(apperrors.KindInvalidArgument, "AUTOMATION_INVALID_TRIGGER_EVENT", "trigger_event must be one of the 5 documented event names", nil)
		}
	} else if p.TriggerEvent != "" {
		return Automation{}, apperrors.New(apperrors.KindInvalidArgument, "AUTOMATION_UNEXPECTED_TRIGGER_EVENT", "trigger_event must be empty unless trigger_type=event", nil)
	}

	timezone := p.Timezone
	if timezone == "" {
		timezone = "UTC"
	}

	return Automation{
		ID:             p.ID,
		TenantID:       p.TenantID,
		ProjectID:      p.ProjectID,
		Name:           p.Name,
		RRule:          p.RRule,
		StepType:       actions[0].StepType,       // mirrors the first action, for back-compat reads
		StepConfigJSON: actions[0].StepConfigJSON, // mirrors the first action, for back-compat reads
		Actions:        actions,
		DTStart:        p.DTStart,
		Timezone:       timezone,
		Enabled:        p.Enabled,
		TriggerType:    trigger,
		TriggerEvent:   p.TriggerEvent,
		TriggerFilter:  p.TriggerFilter,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.CreatedAt,
	}, nil
}

// RecurrenceRule builds this automation's RecurrenceRule value object —
// guaranteed to succeed since NewAutomation already validated RRule parses.
func (a Automation) RecurrenceRule() (RecurrenceRule, error) {
	return NewRecurrenceRule(a.RRule, a.DTStart)
}
