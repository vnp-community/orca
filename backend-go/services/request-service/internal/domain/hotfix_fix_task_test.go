package domain

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestHotfixFixProposal_IsOneValidTaskWithTheDiagnosis(t *testing.T) {
	req := Request{ID: "r1", Type: RequestTypeHotfix, Size: RequestSizeS, Title: "  Checkout crashes  "}
	diag := validDiagnosis(t)
	p := NormalizePlanProposal(HotfixFixProposal(req, diag, DefaultPlanLimits.MaxTitleRunes))

	if len(p.Phases) != 0 || len(p.Tasks) != 1 {
		t.Fatalf("shape = %d phases, %d tasks", len(p.Phases), len(p.Tasks))
	}
	task := p.Tasks[0]
	if task.Title != "Hotfix: Checkout crashes" || task.TaskType != "bug" {
		t.Fatalf("task = %q / %q", task.Title, task.TaskType)
	}
	for _, want := range []string{diag.RootCause.Statement, diag.FixDirections[0].Summary, diag.Reproduction.Steps[0], "smallest safe change", "severity high", "request-service"} {
		if !strings.Contains(task.Description, want) {
			t.Errorf("description lacks %q:\n%s", want, task.Description)
		}
	}
	if err := ValidatePlanProposal(req.Type, ShapeSingleTask, DefaultPlanLimits, p); err != nil {
		t.Fatalf("a hotfix proposal must pass Plan validation: %v", err)
	}
	if err := (hotfixPolicy{}).PlanPreconditions(context.Background(), req, p); err != nil {
		t.Fatalf("a hotfix proposal must pass the hotfix policy: %v", err)
	}
}

func TestHotfixFixProposal_LongTextIsCappedToWhatValidationAccepts(t *testing.T) {
	req := Request{Type: RequestTypeHotfix, Title: strings.Repeat("t", 500)}
	diag := validDiagnosis(t)
	diag.RootCause.Statement = strings.Repeat("c", 20000)
	diag.Summary = strings.Repeat("s", 20000)
	p := NormalizePlanProposal(HotfixFixProposal(req, diag, DefaultPlanLimits.MaxTitleRunes))

	if n := utf8.RuneCountInString(p.Tasks[0].Title); n != DefaultPlanLimits.MaxTitleRunes {
		t.Fatalf("title has %d runes", n)
	}
	if err := ValidatePlanProposal(req.Type, ShapeSingleTask, DefaultPlanLimits, p); err != nil {
		t.Fatalf("capped proposal rejected: %v", err)
	}
}

func TestHotfixFixProposal_SummaryFallsBackToTheRootCause(t *testing.T) {
	diag := validDiagnosis(t)
	diag.Summary = ""
	p := HotfixFixProposal(Request{Title: "x"}, diag, 200)
	if p.Summary != diag.RootCause.Statement {
		t.Fatalf("summary = %q", p.Summary)
	}
}
