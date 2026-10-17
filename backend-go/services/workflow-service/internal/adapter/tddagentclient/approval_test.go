package tddagentclient

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// fakeStore is an in-memory StepApprovalRepository subset; resolve() plays the person deciding in Orca.
type fakeStore struct {
	mu   sync.Mutex
	rows []domain.StepApproval
	// decide is applied to a newly created pending row on the next upsert (simulates the UI)
	decide func(a *domain.StepApproval)
}

func newFakeStore() *fakeStore { return &fakeStore{} }

func (s *fakeStore) UpsertPendingStepApproval(_ context.Context, a domain.StepApproval) (domain.StepApproval, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.rows {
		r := &s.rows[i]
		if r.StepID == a.StepID && r.DecisionHash == a.DecisionHash {
			if r.Status == domain.StepApprovalPending && s.decide != nil {
				s.decide(r)
			}
			return *r, nil
		}
	}
	for i := range s.rows {
		if s.rows[i].StepID == a.StepID && s.rows[i].Status == domain.StepApprovalPending {
			s.rows[i].Supersede(time.Now())
		}
	}
	s.rows = append(s.rows, a)
	return a, nil
}

func (s *fakeStore) LatestStepApproval(_ context.Context, _, _, stepID string) (domain.StepApproval, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.rows) - 1; i >= 0; i-- {
		if s.rows[i].StepID == stepID && s.rows[i].Status != domain.StepApprovalSuperseded {
			return s.rows[i], nil
		}
	}
	return domain.StepApproval{}, domain.ErrStepApprovalNotFound
}

func (s *fakeStore) MarkStepApprovalSubmitted(_ context.Context, _, id string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.rows {
		if s.rows[i].ID == id {
			s.rows[i].MarkSubmitted(at)
		}
	}
	return nil
}

func approve(by string) func(a *domain.StepApproval) {
	return func(a *domain.StepApproval) { _ = a.Resolve(true, by, a.DecisionHash, "looks right", time.Now()) }
}

func awaitingG1(hash string) map[string]any {
	return view("awaiting_approval", "G1", "", false, map[string]any{
		"G1": map[string]any{"status": "awaiting_approval", "decision": map[string]any{"srs": "ready"}, "decision_hash": hash},
	})
}

func TestApprovalWaitsForOrcaThenSubmitsWithoutAdvancing(t *testing.T) {
	tools := newFakeTools()
	tools.on("run_get", func(map[string]any) (any, error) { return awaitingG1("h1"), nil })
	tools.on("approval_submit", func(map[string]any) (any, error) {
		return map[string]any{"accepted": true, "run_status": "running"}, nil
	})
	store := newFakeStore()
	looks := 0
	store.decide = func(a *domain.StepApproval) {
		looks++
		if looks >= 2 {
			approve("user-9")(a)
		}
	}
	h, _ := handlers(tools, store)
	res := must(t)(h.Approval(stepCtx("approve_g1"), `{"runId":"run-1","gate":"G1","role":"BA"}`))
	out := output(t, res)
	if res.Status != domain.ResultStatusCompleted || out["approvedBy"] != "user-9" || out["decisionHash"] != "h1" {
		t.Fatalf("got %s %v", res.Status, out)
	}
	args := tools.args["approval_submit"][0]
	if args["advance"] != false || args["decision_hash"] != "h1" || args["role"] != "BA" {
		t.Fatalf("submit args %v", args)
	}
	if r := args["rationale"].(string); !strings.Contains(r, "orca:user=user-9") || !strings.Contains(r, execID) {
		t.Fatalf("rationale %q", r)
	}
	if row := store.rows[0]; row.SubmittedAt == nil || row.ProjectID != "p1" || row.Kind != approvalKind {
		t.Fatalf("row %+v", row)
	}
}

func TestApprovalRejectedInOrcaFailsTheStep(t *testing.T) {
	tools := newFakeTools()
	tools.on("run_get", func(map[string]any) (any, error) { return awaitingG1("h1"), nil })
	store := newFakeStore()
	store.decide = func(a *domain.StepApproval) {
		_ = a.Resolve(false, "user-2", a.DecisionHash, "SRS incomplete", time.Now())
	}
	h, _ := handlers(tools, store)
	out := output(t, must(t)(h.Approval(stepCtx("approve_g1"), `{"runId":"run-1","gate":"G1","role":"BA"}`)))
	if out["error"] != CodeApprovalReject || tools.count("approval_submit") != 0 {
		t.Fatalf("got %v", out)
	}
}

func TestApprovalSupersedesWhenTheDecisionChanges(t *testing.T) {
	tools := newFakeTools()
	looks := 0
	tools.on("run_get", func(map[string]any) (any, error) {
		looks++
		if looks < 3 {
			return awaitingG1("h1"), nil
		}
		return awaitingG1("h2"), nil
	})
	tools.on("approval_submit", func(map[string]any) (any, error) { return map[string]any{"accepted": true}, nil })
	store := newFakeStore()
	store.decide = func(a *domain.StepApproval) {
		if a.DecisionHash == "h2" {
			approve("user-1")(a)
		}
	}
	h, _ := handlers(tools, store)
	out := output(t, must(t)(h.Approval(stepCtx("approve_g1"), `{"runId":"run-1","gate":"G1","role":"BA"}`)))
	if out["decisionHash"] != "h2" || store.rows[0].Status != domain.StepApprovalSuperseded {
		t.Fatalf("got %v rows %+v", out, store.rows)
	}
}

func TestApprovalMadeOutsideOrcaCompletesTheStep(t *testing.T) {
	tools := newFakeTools()
	tools.on("run_get", func(map[string]any) (any, error) {
		return view("running", "", "", false, map[string]any{"G1": map[string]any{"status": "approved"}}), nil
	})
	h, _ := handlers(tools, newFakeStore())
	out := output(t, must(t)(h.Approval(stepCtx("approve_g1"), `{"runId":"run-1","gate":"G1","role":"BA"}`)))
	if out["approvedBy"] != "external" {
		t.Fatalf("got %v", out)
	}
}

func TestApprovalAfterARestartKnowsWhoApproved(t *testing.T) {
	tools := newFakeTools()
	tools.on("run_get", func(map[string]any) (any, error) {
		return view("running", "", "", false, map[string]any{"G1": map[string]any{"status": "approved"}}), nil
	})
	store := newFakeStore()
	row, _ := domain.NewStepApproval("a1", tenantID, execID, "approve_g1", "p1", approvalKind, nil, nil, "h1", time.Now())
	approve("user-5")(&row)
	store.rows = append(store.rows, row)
	h, _ := handlers(tools, store)
	out := output(t, must(t)(h.Approval(stepCtx("approve_g1"), `{"runId":"run-1","gate":"G1","role":"BA"}`)))
	if out["approvedBy"] != "user-5" || store.rows[0].SubmittedAt == nil {
		t.Fatalf("got %v", out)
	}
}

func TestApprovalRefusedByTddAgentFailsWithItsCode(t *testing.T) {
	tools := newFakeTools()
	tools.on("run_get", func(map[string]any) (any, error) { return awaitingG1("h1"), nil })
	tools.on("approval_submit", func(map[string]any) (any, error) {
		return nil, &ToolError{Code: "ROLE_NOT_PERMITTED", Message: "role BA cannot approve G1 for orca-workflow"}
	})
	store := newFakeStore()
	store.decide = approve("user-1")
	h, _ := handlers(tools, store)
	out := output(t, must(t)(h.Approval(stepCtx("approve_g1"), `{"runId":"run-1","gate":"G1","role":"BA"}`)))
	if out["error"] != CodeApprovalRefused || !strings.Contains(out["message"].(string), "ROLE_NOT_PERMITTED") {
		t.Fatalf("got %v", out)
	}
}

func TestApprovalNeedsGateRoleAndAnExecution(t *testing.T) {
	h, _ := handlers(newFakeTools(), newFakeStore())
	if out := output(t, must(t)(h.Approval(stepCtx("x"), `{"runId":"run-1"}`))); out["error"] != CodeInvalidParams {
		t.Fatalf("got %v", out)
	}
}
