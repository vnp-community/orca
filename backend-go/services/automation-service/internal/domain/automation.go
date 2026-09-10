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
	// StepTypeCommitPush — CR-AUTO-003/TASK-BE-AUTO-005. See
	// workflow-service's domain.StepTypeCommitPush (this const is the same
	// duplication-not-import convention this whole block's doc comment
	// already explains).
	StepTypeCommitPush StepType = "commit_push"
)

func (s StepType) Valid() bool {
	switch s {
	case StepTypeAgent, StepTypeShell, StepTypeNotification, StepTypeWebhook, StepTypeCondition, StepTypeCleanupWorktrees, StepTypeCommitPush:
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
	// ErrEmptyStepConfig guards against an automation with nothing to
	// execute — RunNow would have no step to delegate to workflow-service.
	// CR-AUTO-002: this invariant predates the Actions chain and is
	// deliberately NOT relaxed here — an Actions-only automation (no legacy
	// single step) satisfies it with a harmless "{}" placeholder at the
	// usecase layer instead (see CreateAutomation.Execute).
	ErrEmptyStepConfig = errors.New("domain: step_config_json is required")
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
	// StepType is now a first-class column (migration 0002) rather than a
	// key inside StepConfigJSON — see the former ParseStepType note this
	// replaces. Both internal/adapter/grpcclient (calling workflow-service)
	// and internal/adapter/grpc (translating the wire Automation message,
	// which reuses workflow-service's own StepType enum) map to/from this.
	// DEPRECATED as of CR-AUTO-002: kept for automation rows created before
	// `Actions` existed — see resolveActions in internal/usecase.
	StepType       StepType
	StepConfigJSON string
	DTStart        time.Time
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
	// Actions — CR-AUTO-002/TASK-BE-AUTO-003. Ordered action chain, set
	// directly by the usecase layer after NewAutomation returns (mirrors
	// how internal/adapter/scheduler sets NextRunAt post-construction —
	// same established pattern, not a new one). Empty means "legacy
	// automation, resolve StepType/StepConfigJSON instead" — see
	// usecase.resolveActions (TASK-BE-AUTO-004).
	Actions []AutomationAction
	// MaxRunHistory — CR-AUTO-007/TASK-BE-AUTO-010. 0 = default (100),
	// enforced by the usecase layer, not here.
	MaxRunHistory int32
	// RunTimeoutSeconds — CR-AUTO-007/TASK-BE-AUTO-010. 0 = default (7200).
	RunTimeoutSeconds int32
	// RunningRunID/RunningSince — CR-AUTO-007/TASK-BE-AUTO-011's concurrency
	// guard. Empty/zero means "not currently running". Set/cleared only by
	// the postgres repository's atomic acquire/release (never constructed
	// directly — see AutomationRepository.AcquireRunLock/ReleaseRunLock).
	RunningRunID string
	RunningSince time.Time
}

// AutomationActionType mirrors workflow-service's StepType enum pattern —
// a plain string type, not imported from the generated proto, so domain/
// stays free of the automationv1 dependency (internal/adapter/grpc is the
// only place that translates between the two).
type AutomationActionType string

const (
	AutomationActionTypeUnspecified      AutomationActionType = ""
	AutomationActionTypeCreateWorktree   AutomationActionType = "create_worktree"
	AutomationActionTypeRunAgent         AutomationActionType = "run_agent"
	AutomationActionTypeCommitPush       AutomationActionType = "commit_push"
	AutomationActionTypeCreatePR         AutomationActionType = "create_pr"
	AutomationActionTypeSendNotification AutomationActionType = "send_notification"
	AutomationActionTypeRunScript        AutomationActionType = "run_script"
)

// AutomationAction — CR-AUTO-002. One step in an automation's action chain.
// ConfigJSON is opaque, type-specific — see automation.proto's
// AutomationAction.config_json doc comment; this layer never decodes it
// field-by-field, matching StepConfigJSON's existing convention.
type AutomationAction struct {
	ID                string
	Type              AutomationActionType
	ConfigJSON        string
	ContinueOnFailure bool
}

// ActionResult — CR-AUTO-002. Per-action outcome within one AutomationRun,
// recorded in dispatch order.
type ActionResult struct {
	ActionID   string
	Status     string // running|completed|failed|skipped
	OutputJSON string
	Error      string
}

// NewAutomationParams bundles NewAutomation's inputs. A params struct
// rather than positional args — BR-AT-02's ProjectID and BR-AT-09's trigger
// fields stacked on top of the original 10 params would otherwise make an
// unreadable positional call.
type NewAutomationParams struct {
	ID        string
	TenantID  string
	ProjectID string // optional; empty = unscoped (back-compat) — BR-AT-02
	Name      string
	RRule     string
	// StepType/StepConfigJSON are the legacy single-step path — see
	// Automation.StepType's doc comment. Actions (CR-AUTO-002) is set by the
	// caller directly on the returned Automation, not validated here — see
	// this file's ErrEmptyStepConfig doc comment for why.
	StepType       StepType
	StepConfigJSON string
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
// discovered later by the scheduler loop. StepType defaults to
// StepTypeAgent and Timezone to "UTC" when unspecified, so every Automation
// this constructor returns is already structurally valid for dispatch —
// callers never need a second defaulting pass. NextRunAt is left zero;
// usecase.CreateAutomation computes it from the resulting RecurrenceRule.
// Actions/MaxRunHistory/RunTimeoutSeconds (CR-AUTO-002/007) are NOT set
// here — the caller (usecase.CreateAutomation) sets them directly on the
// returned Automation, mirroring NextRunAt's own post-construction
// convention.
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
	if p.StepConfigJSON == "" {
		return Automation{}, ErrEmptyStepConfig
	}
	if _, err := NewRecurrenceRule(p.RRule, p.DTStart); err != nil {
		return Automation{}, err
	}

	stepType := p.StepType
	if !stepType.Valid() {
		stepType = StepTypeAgent
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
		StepType:       stepType,
		StepConfigJSON: p.StepConfigJSON,
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
