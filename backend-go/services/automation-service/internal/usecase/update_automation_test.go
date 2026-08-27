package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/automation-service/internal/domain"
)

func TestUpdateAutomation_OnlyChangesProvidedFields(t *testing.T) {
	repo := newFakeAutomationRepository()
	seedAutomation(t, repo, "tenant-1", "auto-1", `{"step_type":"agent"}`)
	original := repo.byID["auto-1"]

	uc := NewUpdateAutomation(repo)
	newName := "renamed"
	updated, err := uc.Execute(context.Background(), UpdateAutomationInput{TenantID: "tenant-1", ID: "auto-1", Name: &newName})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Name != "renamed" {
		t.Errorf("expected name updated, got %q", updated.Name)
	}
	if updated.RRule != original.RRule {
		t.Errorf("expected rrule to remain unchanged, got %q want %q", updated.RRule, original.RRule)
	}
	if updated.StepConfigJSON != original.StepConfigJSON {
		t.Errorf("expected step_config_json to remain unchanged, got %q", updated.StepConfigJSON)
	}
	if updated.Enabled != original.Enabled {
		t.Errorf("expected enabled to remain unchanged, got %v", updated.Enabled)
	}

	persisted := repo.byID["auto-1"]
	if persisted.Name != "renamed" {
		t.Errorf("expected the repository to receive the merged automation, got name=%q", persisted.Name)
	}
}

func TestUpdateAutomation_TogglesEnabledOnly(t *testing.T) {
	repo := newFakeAutomationRepository()
	seedAutomation(t, repo, "tenant-1", "auto-1", `{"step_type":"agent"}`)

	uc := NewUpdateAutomation(repo)
	enabled := false
	updated, err := uc.Execute(context.Background(), UpdateAutomationInput{TenantID: "tenant-1", ID: "auto-1", Enabled: &enabled})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Enabled {
		t.Error("expected enabled=false after toggling")
	}
	if updated.Name != "nightly-report" {
		t.Errorf("expected name to remain unchanged, got %q", updated.Name)
	}
}

func TestUpdateAutomation_NotFound_ReturnsError(t *testing.T) {
	repo := newFakeAutomationRepository()
	uc := NewUpdateAutomation(repo)

	_, err := uc.Execute(context.Background(), UpdateAutomationInput{TenantID: "tenant-1", ID: "missing"})
	if err == nil {
		t.Fatal("expected error for missing automation")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected an *apperrors.AppError, got %T", err)
	}
	if appErr.Kind != apperrors.KindNotFound {
		t.Errorf("expected KindNotFound, got %v", appErr.Kind)
	}
}

func TestUpdateAutomation_InvalidRRuleRejected(t *testing.T) {
	repo := newFakeAutomationRepository()
	seedAutomation(t, repo, "tenant-1", "auto-1", `{"step_type":"agent"}`)

	uc := NewUpdateAutomation(repo)
	badRule := "NOT-A-VALID-RRULE"
	_, err := uc.Execute(context.Background(), UpdateAutomationInput{TenantID: "tenant-1", ID: "auto-1", RRule: &badRule})
	if err == nil {
		t.Fatal("expected an error for a malformed rrule")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected an *apperrors.AppError, got %T", err)
	}
	if appErr.Kind != apperrors.KindInvalidArgument {
		t.Errorf("expected KindInvalidArgument, got %v", appErr.Kind)
	}

	// The rejected edit must not be persisted.
	if repo.byID["auto-1"].RRule == badRule {
		t.Error("expected the invalid rrule to not be persisted")
	}
}

func TestUpdateAutomation_EmptyNameRejected(t *testing.T) {
	repo := newFakeAutomationRepository()
	seedAutomation(t, repo, "tenant-1", "auto-1", `{"step_type":"agent"}`)

	uc := NewUpdateAutomation(repo)
	empty := ""
	_, err := uc.Execute(context.Background(), UpdateAutomationInput{TenantID: "tenant-1", ID: "auto-1", Name: &empty})
	if err == nil {
		t.Fatal("expected an error for an empty name")
	}
}

func TestUpdateAutomation_UpdatesStepTypeAndDtstart(t *testing.T) {
	repo := newFakeAutomationRepository()
	seedAutomation(t, repo, "tenant-1", "auto-1", `{"command":"echo hi"}`)

	uc := NewUpdateAutomation(repo)
	stepType := domain.StepTypeShell
	dtstart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	updated, err := uc.Execute(context.Background(), UpdateAutomationInput{
		TenantID: "tenant-1", ID: "auto-1", StepType: &stepType, Dtstart: &dtstart,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.StepType != domain.StepTypeShell {
		t.Errorf("expected step_type=shell, got %v", updated.StepType)
	}
	if !updated.DTStart.Equal(dtstart) {
		t.Errorf("expected dtstart=%v, got %v", dtstart, updated.DTStart)
	}
}
