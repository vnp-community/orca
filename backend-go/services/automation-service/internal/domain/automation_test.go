package domain

import (
	"testing"
	"time"
)

func TestNewAutomation_ValidatesInvariants(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name           string
		tenantID       string
		automationName string
		rrule          string
		stepConfig     string
		wantErr        error
	}{
		{"valid", "t1", "nightly-report", "FREQ=DAILY;INTERVAL=1", `{"step_type":"agent"}`, nil},
		{"empty tenant", "", "nightly-report", "FREQ=DAILY;INTERVAL=1", `{}`, ErrEmptyTenant},
		{"empty name", "t1", "", "FREQ=DAILY;INTERVAL=1", `{}`, ErrEmptyName},
		{"empty rrule", "t1", "nightly-report", "", `{}`, ErrEmptyRRule},
		{"empty step config", "t1", "nightly-report", "FREQ=DAILY;INTERVAL=1", "", ErrEmptyStepConfig},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewAutomation("a1", tt.tenantID, tt.automationName, tt.rrule, StepTypeAgent, tt.stepConfig, now, "UTC", true, now)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tt.wantErr != nil && err != tt.wantErr {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestNewAutomation_RejectsInvalidRRule(t *testing.T) {
	now := time.Now()
	_, err := NewAutomation("a1", "t1", "bad-rule", "NOT_A_RULE", StepTypeAgent, `{}`, now, "UTC", true, now)
	if err == nil {
		t.Fatal("expected an error for an invalid rrule")
	}
}

func TestNewAutomation_DefaultsStepTypeWhenInvalid(t *testing.T) {
	now := time.Now()
	a, err := NewAutomation("a1", "t1", "nightly-report", "FREQ=DAILY;INTERVAL=1", StepTypeUnspecified, `{}`, now, "UTC", true, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.StepType != StepTypeAgent {
		t.Errorf("expected StepType to default to agent, got %v", a.StepType)
	}
}

func TestNewAutomation_DefaultsTimezoneWhenEmpty(t *testing.T) {
	now := time.Now()
	a, err := NewAutomation("a1", "t1", "nightly-report", "FREQ=DAILY;INTERVAL=1", StepTypeAgent, `{}`, now, "", true, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Timezone != "UTC" {
		t.Errorf("expected Timezone to default to UTC, got %q", a.Timezone)
	}
}

func TestAutomationRun_StatusTransitions(t *testing.T) {
	now := time.Now()
	run, err := NewPendingRun("r1", "a1", "t1", "req-1", StepTypeAgent, RunTriggerManual, `{"step_type":"agent"}`, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.Status != RunStatusPending {
		t.Fatalf("expected pending status, got %v", run.Status)
	}

	running, err := run.MarkRunning(now)
	if err != nil {
		t.Fatalf("unexpected error transitioning to running: %v", err)
	}
	if running.Status != RunStatusRunning {
		t.Fatalf("expected running status, got %v", running.Status)
	}

	// Can't mark succeeded directly from pending.
	if _, err := run.MarkSucceeded(now, `{}`); err != ErrInvalidTransition {
		t.Errorf("expected ErrInvalidTransition from pending->succeeded, got %v", err)
	}

	succeeded, err := running.MarkSucceeded(now, `{"result":"ok"}`)
	if err != nil {
		t.Fatalf("unexpected error transitioning to succeeded: %v", err)
	}
	if succeeded.Status != RunStatusSucceeded || !succeeded.Status.Terminal() {
		t.Errorf("expected terminal succeeded status, got %v", succeeded.Status)
	}

	// A terminal run can't transition again.
	if _, err := succeeded.MarkFailed(now, "boom"); err != ErrInvalidTransition {
		t.Errorf("expected ErrInvalidTransition from a terminal state, got %v", err)
	}
}

func TestAutomationRun_MarkFailed_ValidFromPendingOrRunning(t *testing.T) {
	now := time.Now()
	run, err := NewPendingRun("r1", "a1", "t1", "req-1", StepTypeAgent, RunTriggerManual, `{}`, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Failing directly from Pending is valid — e.g. workflow-service was
	// unreachable before the call was even made, per
	// automation-service.md §8's fail-closed requirement.
	failed, err := run.MarkFailed(now, "workflow-service unavailable")
	if err != nil {
		t.Fatalf("unexpected error failing from pending: %v", err)
	}
	if failed.Status != RunStatusFailed {
		t.Errorf("expected failed status, got %v", failed.Status)
	}
}

func TestNewPendingRun_ValidatesInvariants(t *testing.T) {
	now := time.Now()
	if _, err := NewPendingRun("r1", "", "t1", "req-1", StepTypeAgent, RunTriggerManual, `{}`, now); err != ErrEmptyAutomationID {
		t.Errorf("expected ErrEmptyAutomationID, got %v", err)
	}
	if _, err := NewPendingRun("r1", "a1", "t1", "", StepTypeAgent, RunTriggerManual, `{}`, now); err != ErrEmptyRequestID {
		t.Errorf("expected ErrEmptyRequestID, got %v", err)
	}
}

func TestNewPendingRun_DefaultsTriggerWhenInvalid(t *testing.T) {
	now := time.Now()
	run, err := NewPendingRun("r1", "a1", "t1", "req-1", StepTypeAgent, RunTrigger("bogus"), `{}`, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.Trigger != RunTriggerManual {
		t.Errorf("expected Trigger to default to manual, got %v", run.Trigger)
	}
}
