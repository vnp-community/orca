package grpcclient

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// requestWithCriteria builds a Request whose acceptance_criteria column holds the given items as JSON.
func requestWithCriteria(t *testing.T, items ...[3]string) domain.Request {
	t.Helper()
	var parts []string
	for _, it := range items {
		raw, err := json.Marshal(map[string]string{"id": it[0], "text": it[1], "status": it[2]})
		if err != nil {
			t.Fatal(err)
		}
		parts = append(parts, string(raw))
	}
	return domain.Request{Number: 42, Title: "Cache pages", AcceptanceCriteriaJSON: []byte("[" + strings.Join(parts, ",") + "]")}
}

func TestPlanTaskContext_CarriesTheTextOfTheCriteriaTheTaskServes(t *testing.T) {
	req := requestWithCriteria(t,
		[3]string{"AC-1", "Pages load  in\nunder one second", string(domain.ACStatusActive)},
		[3]string{"AC-2", "Cache is cleared on deploy", string(domain.ACStatusActive)},
		[3]string{"AC-3", "retired criterion", string(domain.ACStatusRetired)},
	)
	in := usecase.PlanTreeInput{Request: req, Proposal: domain.PlanProposal{Summary: "Add an LRU cache."}}
	got := planTaskContext(in, domain.TaskProposal{Satisfies: []string{"AC-1", "AC-3", "AC-9"}})
	for _, want := range []string{"REQ-42: Cache pages", "- AC-1: Pages load in under one second", "Add an LRU cache."} {
		if !strings.Contains(got, want) {
			t.Errorf("context lacks %q:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"AC-2", "AC-3", "AC-9", "retired"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("context names %q, which this task does not serve as an active criterion:\n%s", unwanted, got)
		}
	}
	if plain := planTaskContext(in, domain.TaskProposal{}); strings.Contains(plain, "Acceptance criteria") || !strings.Contains(plain, "Add an LRU cache.") {
		t.Errorf("a task serving none keeps the old context:\n%s", plain)
	}
}

func TestPlanTaskContext_TheCeilingCutsTheSummaryBeforeTheCriteria(t *testing.T) {
	req := requestWithCriteria(t, [3]string{"AC-1", strings.Repeat("c", 500), string(domain.ACStatusActive)})
	in := usecase.PlanTreeInput{Request: req, Proposal: domain.PlanProposal{Summary: strings.Repeat("s", 5000)}}
	got := planTaskContext(in, domain.TaskProposal{Satisfies: []string{"AC-1"}})
	if n := utf8.RuneCountInString(got); n != maxAIContextRunes {
		t.Fatalf("context is %d runes, want the %d ceiling", n, maxAIContextRunes)
	}
	if !strings.Contains(got, "- AC-1: "+strings.Repeat("c", maxCriterionRunes)+"...") || strings.Contains(got, strings.Repeat("c", maxCriterionRunes+1)) {
		t.Fatalf("a long criterion must be shortened, not dropped:\n%.400s", got)
	}
}
