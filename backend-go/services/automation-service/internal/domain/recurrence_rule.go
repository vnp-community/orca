package domain

import (
	"fmt"
	"time"

	"github.com/teambition/rrule-go"
)

// RecurrenceRule wraps an RFC 5545 RRULE string plus its DTSTART — a pure
// value object with two pure functions, per
// specs/backend-go/services/automation-service.md §4. Library choice: this
// scaffold used `go get github.com/teambition/rrule-go`, which fetched
// cleanly and covers every feature the design doc calls out (frequency,
// interval, byweekday, count/until) — the hand-rolled "every N hours"
// fallback the task description allows for was not needed.
type RecurrenceRule struct {
	RRule   string
	DTStart time.Time
}

// NewRecurrenceRule constructs a RecurrenceRule, validating that RRule
// parses as a real RFC 5545 recurrence string against the given DTSTART —
// a malformed rule is rejected here, not discovered later by the scheduler
// loop trying to compute a next occurrence.
func NewRecurrenceRule(rruleStr string, dtstart time.Time) (RecurrenceRule, error) {
	if rruleStr == "" {
		return RecurrenceRule{}, ErrEmptyRRule
	}
	if _, err := parseRRule(rruleStr, dtstart); err != nil {
		return RecurrenceRule{}, fmt.Errorf("%w: %v", ErrInvalidRRule, err)
	}
	return RecurrenceRule{RRule: rruleStr, DTStart: dtstart}, nil
}

func parseRRule(rruleStr string, dtstart time.Time) (*rrule.RRule, error) {
	opt, err := rrule.StrToROption(rruleStr)
	if err != nil {
		return nil, err
	}
	opt.Dtstart = dtstart
	return rrule.NewRRule(*opt)
}

// NextOccurrenceAfter returns the first occurrence strictly after t, mirroring
// TS's nextAutomationOccurrenceAfter. The bool is false when the rule has no
// further occurrences (e.g. an exhausted COUNT/UNTIL-bounded rule).
func (r RecurrenceRule) NextOccurrenceAfter(t time.Time) (time.Time, bool) {
	rr, err := parseRRule(r.RRule, r.DTStart)
	if err != nil {
		return time.Time{}, false
	}
	next := rr.After(t, false)
	if next.IsZero() {
		return time.Time{}, false
	}
	return next, true
}

// LatestOccurrenceAtOrBefore returns the last occurrence at or before t,
// mirroring TS's latestAutomationOccurrenceAtOrBefore — used by the
// scheduler's missed-run-policy handling.
func (r RecurrenceRule) LatestOccurrenceAtOrBefore(t time.Time) (time.Time, bool) {
	rr, err := parseRRule(r.RRule, r.DTStart)
	if err != nil {
		return time.Time{}, false
	}
	latest := rr.Before(t, true)
	if latest.IsZero() {
		return time.Time{}, false
	}
	return latest, true
}
