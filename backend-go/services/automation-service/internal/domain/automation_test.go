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
		// CR-AUTO-002: an automation with no legacy step_config_json has
		// nothing for NewAutomation to validate — an Actions-only automation
		// satisfies this at the usecase layer with a "{}" placeholder
		// instead (see usecase.CreateAutomation's doc comment).
		{"empty step config", "t1", "nightly-report", "FREQ=DAILY;INTERVAL=1", "", ErrEmptyStepConfig},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewAutomation(NewAutomationParams{
				ID: "a1", TenantID: tt.tenantID, Name: tt.automationName, RRule: tt.rrule,
				StepType: StepTypeAgent, StepConfigJSON: tt.stepConfig, DTStart: now, Timezone: "UTC", Enabled: true, CreatedAt: now,
			})
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
	_, err := NewAutomation(NewAutomationParams{
		ID: "a1", TenantID: "t1", Name: "bad-rule", RRule: "NOT_A_RULE",
		StepType: StepTypeAgent, StepConfigJSON: `{}`, DTStart: now, Timezone: "UTC", Enabled: true, CreatedAt: now,
	})
	if err == nil {
		t.Fatal("expected an error for an invalid rrule")
	}
}

func TestNewAutomation_DefaultsStepTypeWhenInvalid(t *testing.T) {
	now := time.Now()
	a, err := NewAutomation(NewAutomationParams{
		ID: "a1", TenantID: "t1", Name: "nightly-report", RRule: "FREQ=DAILY;INTERVAL=1",
		StepType: StepTypeUnspecified, StepConfigJSON: `{}`, DTStart: now, Timezone: "UTC", Enabled: true, CreatedAt: now,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.StepType != StepTypeAgent {
		t.Errorf("expected StepType to default to agent, got %v", a.StepType)
	}
}

func TestNewAutomation_DefaultsTimezoneWhenEmpty(t *testing.T) {
	now := time.Now()
	a, err := NewAutomation(NewAutomationParams{
		ID: "a1", TenantID: "t1", Name: "nightly-report", RRule: "FREQ=DAILY;INTERVAL=1",
		StepType: StepTypeAgent, StepConfigJSON: `{}`, DTStart: now, Timezone: "", Enabled: true, CreatedAt: now,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Timezone != "UTC" {
		t.Errorf("expected Timezone to default to UTC, got %q", a.Timezone)
	}
}

func TestNewAutomation_EmptyStepConfigRejected(t *testing.T) {
	now := time.Now()
	_, err := NewAutomation(NewAutomationParams{
		ID: "a1", TenantID: "t1", Name: "nightly-report", RRule: "FREQ=DAILY;INTERVAL=1",
		DTStart: now, Timezone: "UTC", Enabled: true, CreatedAt: now,
	})
	if err != ErrEmptyStepConfig {
		t.Fatalf("expected ErrEmptyStepConfig, got %v", err)
	}
}

// TestNewAutomation_DoesNotTouchActions is CR-AUTO-002's regression guard:
// NewAutomation validates only the legacy step_type/step_config_json pair —
// Actions is set by the caller directly on the returned Automation
// (mirrors NextRunAt's own post-construction convention), never touched
// here. See NewAutomation's own doc comment for why.
func TestNewAutomation_DoesNotTouchActions(t *testing.T) {
	now := time.Now()
	a, err := NewAutomation(NewAutomationParams{
		ID: "a1", TenantID: "t1", Name: "nightly-report", RRule: "FREQ=DAILY;INTERVAL=1",
		StepType: StepTypeAgent, StepConfigJSON: `{}`, DTStart: now, Timezone: "UTC", Enabled: true, CreatedAt: now,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Actions != nil {
		t.Errorf("expected Actions to be left nil by NewAutomation, got %+v", a.Actions)
	}
}

func TestNewAutomation_TriggerEventRequiresValidEventName(t *testing.T) {
	now := time.Now()
	_, err := NewAutomation(NewAutomationParams{
		ID: "a1", TenantID: "t1", Name: "n", RRule: "FREQ=DAILY;INTERVAL=1",
		StepType: StepTypeAgent, StepConfigJSON: `{}`, DTStart: now, Timezone: "UTC", Enabled: true, CreatedAt: now,
		TriggerType: TriggerTypeEvent,
	})
	if err == nil {
		t.Fatal("expected an error for trigger_type=event with no trigger_event")
	}

	a, err := NewAutomation(NewAutomationParams{
		ID: "a1", TenantID: "t1", Name: "n", RRule: "FREQ=DAILY;INTERVAL=1",
		StepType: StepTypeAgent, StepConfigJSON: `{}`, DTStart: now, Timezone: "UTC", Enabled: true, CreatedAt: now,
		TriggerType: TriggerTypeEvent, TriggerEvent: EventAgentCompleted,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.TriggerEvent != EventAgentCompleted {
		t.Errorf("expected trigger_event=agent:completed, got %v", a.TriggerEvent)
	}
}

func TestNewAutomation_TriggerEventRejectedWhenTypeIsNotEvent(t *testing.T) {
	now := time.Now()
	_, err := NewAutomation(NewAutomationParams{
		ID: "a1", TenantID: "t1", Name: "n", RRule: "FREQ=DAILY;INTERVAL=1",
		StepType: StepTypeAgent, StepConfigJSON: `{}`, DTStart: now, Timezone: "UTC", Enabled: true, CreatedAt: now,
		TriggerType: TriggerTypeCron, TriggerEvent: EventAgentCompleted,
	})
	if err == nil {
		t.Fatal("expected an error for a non-event trigger_type with a non-empty trigger_event")
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
