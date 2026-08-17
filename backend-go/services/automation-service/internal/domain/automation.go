// Package domain holds automation-service's entities and value objects. Per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md, this
// package has zero imports outside stdlib, the RRULE library, and other
// domain/ files — no database, no gRPC, no framework.
package domain

import (
	"encoding/json"
	"errors"
	"time"
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
)

func (s StepType) Valid() bool {
	switch s {
	case StepTypeAgent, StepTypeShell, StepTypeNotification, StepTypeWebhook, StepTypeCondition:
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
	ErrEmptyStepConfig = errors.New("domain: step_config_json is required")
)

// Automation is a scheduled/triggered automation definition — the system of
// record owned by this service. Execution never happens here; RunNow
// delegates to workflow-service.ExecuteAdHocStep (see
// specs/backend-go/services/automation-service.md §2).
type Automation struct {
	ID             string
	TenantID       string
	Name           string
	RRule          string
	StepConfigJSON string
	DTStart        time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// NewAutomation constructs an Automation, enforcing the invariants a
// definition must satisfy to be dispatchable — including that RRule parses,
// so a malformed recurrence string is rejected at creation time rather than
// discovered later by the scheduler loop.
func NewAutomation(id, tenantID, name, rrule, stepConfigJSON string, dtstart, createdAt time.Time) (Automation, error) {
	if tenantID == "" {
		return Automation{}, ErrEmptyTenant
	}
	if name == "" {
		return Automation{}, ErrEmptyName
	}
	if rrule == "" {
		return Automation{}, ErrEmptyRRule
	}
	if stepConfigJSON == "" {
		return Automation{}, ErrEmptyStepConfig
	}
	if _, err := NewRecurrenceRule(rrule, dtstart); err != nil {
		return Automation{}, err
	}
	return Automation{
		ID:             id,
		TenantID:       tenantID,
		Name:           name,
		RRule:          rrule,
		StepConfigJSON: stepConfigJSON,
		DTStart:        dtstart,
		CreatedAt:      createdAt,
		UpdatedAt:      createdAt,
	}, nil
}

// RecurrenceRule builds this automation's RecurrenceRule value object —
// guaranteed to succeed since NewAutomation already validated RRule parses.
func (a Automation) RecurrenceRule() (RecurrenceRule, error) {
	return NewRecurrenceRule(a.RRule, a.DTStart)
}

// ParseStepType extracts the "step_type" key from a step_config_json body.
// The generated automation.proto (proto/orca/automation/v1/automation.proto)
// has no separate step_type field, so RunNow reads it out of the JSON
// payload instead — see this service's README "deviations" note. Returns
// StepTypeUnspecified if the JSON doesn't parse, has no step_type key, or
// the value isn't one of the known StepType constants; callers decide the
// fallback (RunNow defaults to StepTypeAgent).
func ParseStepType(stepConfigJSON string) StepType {
	var cfg struct {
		StepType StepType `json:"step_type"`
	}
	if err := json.Unmarshal([]byte(stepConfigJSON), &cfg); err != nil {
		return StepTypeUnspecified
	}
	if !cfg.StepType.Valid() {
		return StepTypeUnspecified
	}
	return cfg.StepType
}
