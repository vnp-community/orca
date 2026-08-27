package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/automation-service/internal/domain"
)

func TestCreateAutomation_PersistsStructuralStepType(t *testing.T) {
	repo := newFakeAutomationRepository()
	uc := NewCreateAutomation(repo)
	ctx := withTenant(context.Background(), "tenant-1")

	got, err := uc.Execute(ctx, CreateAutomationInput{
		Name:           "nightly-report",
		RRule:          "FREQ=DAILY;INTERVAL=1",
		StepType:       domain.StepTypeShell,
		StepConfigJSON: `{"command":"echo hi"}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.StepType != domain.StepTypeShell {
		t.Errorf("expected StepType=shell to be persisted structurally, got %v", got.StepType)
	}

	stored, err := repo.Get(ctx, "tenant-1", got.ID)
	if err != nil {
		t.Fatalf("unexpected error fetching stored automation: %v", err)
	}
	if stored.StepType != domain.StepTypeShell {
		t.Errorf("expected the repository round-trip to keep StepType=shell, got %v", stored.StepType)
	}
}

func TestCreateAutomation_DefaultsEnabledTrue(t *testing.T) {
	repo := newFakeAutomationRepository()
	uc := NewCreateAutomation(repo)
	ctx := withTenant(context.Background(), "tenant-1")

	got, err := uc.Execute(ctx, CreateAutomationInput{
		Name: "nightly-report", RRule: "FREQ=DAILY;INTERVAL=1",
		StepType: domain.StepTypeAgent, StepConfigJSON: `{}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Enabled {
		t.Error("expected a new automation to default enabled=true")
	}
}

func TestCreateAutomation_EmptyDTStartDefaultsToNow(t *testing.T) {
	repo := newFakeAutomationRepository()
	uc := NewCreateAutomation(repo)
	ctx := withTenant(context.Background(), "tenant-1")

	before := time.Now().UTC()
	got, err := uc.Execute(ctx, CreateAutomationInput{
		Name: "nightly-report", RRule: "FREQ=DAILY;INTERVAL=1",
		StepType: domain.StepTypeAgent, StepConfigJSON: `{}`,
	})
	after := time.Now().UTC()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.DTStart.Before(before) || got.DTStart.After(after) {
		t.Errorf("expected DTStart to default to now (between %v and %v), got %v", before, after, got.DTStart)
	}
}

func TestCreateAutomation_ExplicitDTStartAndTimezone(t *testing.T) {
	repo := newFakeAutomationRepository()
	uc := NewCreateAutomation(repo)
	ctx := withTenant(context.Background(), "tenant-1")

	got, err := uc.Execute(ctx, CreateAutomationInput{
		Name: "nightly-report", RRule: "FREQ=DAILY;INTERVAL=1",
		StepType: domain.StepTypeAgent, StepConfigJSON: `{}`,
		DTStart: "2026-01-01T00:00:00Z", Timezone: "Asia/Ho_Chi_Minh",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Timezone != "Asia/Ho_Chi_Minh" {
		t.Errorf("expected Timezone=Asia/Ho_Chi_Minh, got %q", got.Timezone)
	}
	if !got.DTStart.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("expected DTStart=2026-01-01T00:00:00Z, got %v", got.DTStart)
	}
}

func TestCreateAutomation_RejectsMalformedDTStart(t *testing.T) {
	repo := newFakeAutomationRepository()
	uc := NewCreateAutomation(repo)
	ctx := withTenant(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, CreateAutomationInput{
		Name: "nightly-report", RRule: "FREQ=DAILY;INTERVAL=1",
		StepType: domain.StepTypeAgent, StepConfigJSON: `{}`,
		DTStart: "not-a-timestamp",
	})
	if err == nil {
		t.Fatal("expected an error for a malformed dtstart")
	}
}

func TestCreateAutomation_RejectsUnknownTimezone(t *testing.T) {
	repo := newFakeAutomationRepository()
	uc := NewCreateAutomation(repo)
	ctx := withTenant(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, CreateAutomationInput{
		Name: "nightly-report", RRule: "FREQ=DAILY;INTERVAL=1",
		StepType: domain.StepTypeAgent, StepConfigJSON: `{}`,
		Timezone: "Not/A_Timezone",
	})
	if err == nil {
		t.Fatal("expected an error for an unknown timezone")
	}
}

func TestCreateAutomation_ComputesFirstNextRunAt(t *testing.T) {
	repo := newFakeAutomationRepository()
	uc := NewCreateAutomation(repo)
	ctx := withTenant(context.Background(), "tenant-1")

	got, err := uc.Execute(ctx, CreateAutomationInput{
		Name: "nightly-report", RRule: "FREQ=DAILY;INTERVAL=1",
		StepType: domain.StepTypeAgent, StepConfigJSON: `{}`,
		DTStart: "2026-01-01T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The first occurrence of a daily rule anchored at dtstart is dtstart
	// itself.
	if !got.NextRunAt.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("expected NextRunAt=dtstart for the first occurrence, got %v", got.NextRunAt)
	}
}

func TestCreateAutomation_RequiresTenantContext(t *testing.T) {
	repo := newFakeAutomationRepository()
	uc := NewCreateAutomation(repo)

	_, err := uc.Execute(context.Background(), CreateAutomationInput{
		Name: "nightly-report", RRule: "FREQ=DAILY;INTERVAL=1",
		StepType: domain.StepTypeAgent, StepConfigJSON: `{}`,
	})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}
