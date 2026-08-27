package domain

import (
	"testing"
	"time"
)

func TestNewRecurrenceRule_RejectsEmptyAndInvalid(t *testing.T) {
	dtstart := time.Date(2026, 8, 17, 9, 0, 0, 0, time.UTC)

	if _, err := NewRecurrenceRule("", dtstart); err != ErrEmptyRRule {
		t.Errorf("expected ErrEmptyRRule, got %v", err)
	}
	if _, err := NewRecurrenceRule("NOT_A_VALID_RRULE", dtstart); err == nil {
		t.Error("expected an error for a malformed RRULE string")
	}
	if _, err := NewRecurrenceRule("FREQ=DAILY;INTERVAL=1", dtstart); err != nil {
		t.Errorf("expected a valid daily rule to parse, got %v", err)
	}
}

func TestRecurrenceRule_NextOccurrenceAfter_Daily(t *testing.T) {
	dtstart := time.Date(2026, 8, 17, 9, 0, 0, 0, time.UTC)
	rule, err := NewRecurrenceRule("FREQ=DAILY;INTERVAL=1", dtstart)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	next, ok := rule.NextOccurrenceAfter(dtstart)
	if !ok {
		t.Fatal("expected a next occurrence")
	}
	want := dtstart.Add(24 * time.Hour)
	if !next.Equal(want) {
		t.Errorf("NextOccurrenceAfter(dtstart) = %v, want %v", next, want)
	}

	// Asking for the occurrence after a time between two ticks returns the
	// following tick, not the one just passed.
	mid := dtstart.Add(12 * time.Hour)
	next2, ok := rule.NextOccurrenceAfter(mid)
	if !ok {
		t.Fatal("expected a next occurrence")
	}
	if !next2.Equal(want) {
		t.Errorf("NextOccurrenceAfter(mid) = %v, want %v", next2, want)
	}
}

func TestRecurrenceRule_NextOccurrenceAfter_WeeklyInterval(t *testing.T) {
	dtstart := time.Date(2026, 8, 17, 9, 0, 0, 0, time.UTC) // a Monday
	rule, err := NewRecurrenceRule("FREQ=WEEKLY;INTERVAL=2;BYDAY=MO", dtstart)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	next, ok := rule.NextOccurrenceAfter(dtstart)
	if !ok {
		t.Fatal("expected a next occurrence")
	}
	want := dtstart.Add(14 * 24 * time.Hour)
	if !next.Equal(want) {
		t.Errorf("NextOccurrenceAfter(dtstart) = %v, want %v (two weeks later)", next, want)
	}
}

func TestRecurrenceRule_LatestOccurrenceAtOrBefore(t *testing.T) {
	dtstart := time.Date(2026, 8, 17, 9, 0, 0, 0, time.UTC)
	rule, err := NewRecurrenceRule("FREQ=DAILY;INTERVAL=1", dtstart)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Exactly at an occurrence, inclusive semantics return it.
	latest, ok := rule.LatestOccurrenceAtOrBefore(dtstart)
	if !ok {
		t.Fatal("expected a latest occurrence")
	}
	if !latest.Equal(dtstart) {
		t.Errorf("LatestOccurrenceAtOrBefore(dtstart) = %v, want %v", latest, dtstart)
	}

	// Between two ticks, returns the one just passed.
	mid := dtstart.Add(12 * time.Hour)
	latest2, ok := rule.LatestOccurrenceAtOrBefore(mid)
	if !ok {
		t.Fatal("expected a latest occurrence")
	}
	if !latest2.Equal(dtstart) {
		t.Errorf("LatestOccurrenceAtOrBefore(mid) = %v, want %v", latest2, dtstart)
	}

	// Before DTSTART, there is no prior occurrence.
	before := dtstart.Add(-time.Hour)
	if _, ok := rule.LatestOccurrenceAtOrBefore(before); ok {
		t.Error("expected no occurrence before DTSTART")
	}
}

func TestRecurrenceRule_CountBoundedRuleExhausts(t *testing.T) {
	dtstart := time.Date(2026, 8, 17, 9, 0, 0, 0, time.UTC)
	rule, err := NewRecurrenceRule("FREQ=DAILY;COUNT=2", dtstart)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Two occurrences: dtstart, dtstart+1d. Asking after the last must
	// report no further occurrence rather than fabricating one.
	last := dtstart.Add(24 * time.Hour)
	if _, ok := rule.NextOccurrenceAfter(last); ok {
		t.Error("expected no occurrence after a COUNT=2 rule's last tick")
	}
}
