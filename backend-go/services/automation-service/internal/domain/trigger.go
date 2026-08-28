package domain

import (
	"encoding/json"
	"strings"
)

// TriggerType is BL-AT-03's `trigger.type` — cron (rrule-driven, the
// original/default shape), manual (RunNow-only; rrule/dtstart still stored
// but next_run_at never advances), or event (see EventName below).
type TriggerType string

const (
	TriggerTypeCron   TriggerType = "cron"
	TriggerTypeManual TriggerType = "manual"
	TriggerTypeEvent  TriggerType = "event"
)

// EventName is a closed set — the five names BL-AT-03 documents. An
// unrecognized string is rejected by NewAutomation, not silently accepted
// and never matched.
type EventName string

const (
	EventAgentCompleted  EventName = "agent:completed"
	EventAgentError      EventName = "agent:error"
	EventWorktreeCreated EventName = "worktree:created"
	EventPRMerged        EventName = "pr:merged"
	EventIssueAssigned   EventName = "issue:assigned"
)

func (e EventName) Valid() bool {
	switch e {
	case EventAgentCompleted, EventAgentError, EventWorktreeCreated, EventPRMerged, EventIssueAssigned:
		return true
	default:
		return false
	}
}

// TriggerFilter is BR-AT-09's closed comparison grammar — no arbitrary
// expression evaluation. {"field": "agent", "equals": "claude"}; field is a
// dot-path into the event payload, "equals" is the only operator v1 needs.
type TriggerFilter struct {
	Field  string
	Equals string
}

// Matches performs a fail-safe-false dot-path lookup + string-equals — an
// automation with a broken filter never fires rather than firing on
// everything.
func (f TriggerFilter) Matches(payload map[string]any) bool {
	if f.Field == "" {
		return false
	}
	parts := strings.Split(f.Field, ".")
	var cur any = payload
	for _, p := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return false
		}
		v, ok := m[p]
		if !ok {
			return false
		}
		cur = v
	}
	s, ok := cur.(string)
	return ok && s == f.Equals
}

// ParseTriggerFilter decodes the wire's trigger_filter_json into a
// TriggerFilter — empty input is a valid "no filter" (nil, nil), matching
// automation.proto's "empty = no filter (always matches)" convention.
func ParseTriggerFilter(raw string) (*TriggerFilter, error) {
	if raw == "" {
		return nil, nil
	}
	var f TriggerFilter
	if err := json.Unmarshal([]byte(raw), &f); err != nil {
		return nil, err
	}
	return &f, nil
}
