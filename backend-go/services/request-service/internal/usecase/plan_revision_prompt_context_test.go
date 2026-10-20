package usecase

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// storedChangeFor puts a finished change of the Request into the fake store, as a first run would have left it.
func storedChangeFor(t *testing.T, k *planKit, requestID string, status domain.ChangeStatus, files ...string) {
	t.Helper()
	bundle := map[string]string{}
	for _, f := range files {
		bundle[f] = "text"
	}
	change := domain.OpenSpecChange{ID: uuid.NewString(), RequestID: requestID, ChangeID: "req-1-cache", Status: status, SyncState: domain.SyncStateInSync, Bundle: bundle}
	if _, err := k.proposal.changes.Upsert(context.Background(), change); err != nil {
		t.Fatal(err)
	}
}

func TestRunProposalGeneration_FirstRunHasNoPreviousFiles(t *testing.T) {
	eng := &scriptedEngine{answers: []ProposalResult{answer(tasksDoc(twoPhaseBody))}}
	k, run, w := proposalKit(t, eng, nil)
	if err := w.Execute(k.w.ctx(), run); err != nil {
		t.Fatal(err)
	}
	if got := eng.inputs[0].PreviousFiles; len(got) != 0 {
		t.Fatalf("a first run has nothing to revise: %v", got)
	}
}

func TestRunProposalGeneration_RevisionListsTheFilesOfTheReadyChangeSorted(t *testing.T) {
	eng := &scriptedEngine{answers: []ProposalResult{answer(tasksDoc(twoPhaseBody))}}
	k, run, w := proposalKit(t, eng, nil)
	storedChangeFor(t, k, run.RequestID, domain.ChangeStatusReady, "tasks.md", "specs/b/spec.md", "design.md", "specs/a/spec.md")
	if err := w.Execute(k.w.ctx(), run); err != nil {
		t.Fatal(err)
	}
	want := []string{"design.md", "specs/a/spec.md", "specs/b/spec.md", "tasks.md"}
	if got := eng.inputs[0].PreviousFiles; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("previous files = %v, want %v", got, want)
	}
}

func TestRunProposalGeneration_ChangeThatIsNotReadyHasNothingToEdit(t *testing.T) {
	for _, status := range []domain.ChangeStatus{domain.ChangeStatusPreparing, domain.ChangeStatusAbandoned, domain.ChangeStatusArchived} {
		eng := &scriptedEngine{answers: []ProposalResult{answer(tasksDoc(twoPhaseBody))}}
		k, run, w := proposalKit(t, eng, nil)
		storedChangeFor(t, k, run.RequestID, status, "tasks.md", "design.md")
		if err := w.Execute(k.w.ctx(), run); err != nil {
			t.Fatal(err)
		}
		if got := eng.inputs[0].PreviousFiles; len(got) != 0 {
			t.Fatalf("status %s: %v", status, got)
		}
	}
}

func TestPreviousChangeFiles_IsBounded(t *testing.T) {
	bundle := map[string]string{}
	for i := 0; i < maxPreviousChangeFiles+30; i++ {
		bundle[strings.Repeat("x", 1)+string(rune('a'+i%26))+strings.Repeat("y", i)] = "t"
	}
	got := previousChangeFiles(domain.OpenSpecChange{Status: domain.ChangeStatusReady, Bundle: bundle}, true)
	if len(got) != maxPreviousChangeFiles {
		t.Fatalf("%d names", len(got))
	}
	if previousChangeFiles(domain.OpenSpecChange{Status: domain.ChangeStatusReady, Bundle: bundle}, false) != nil {
		t.Fatal("no stored change, no names")
	}
}

// --- native engine ---

func seedSucceededPlanRun(k *planKit, requestID, raw string, at time.Time) {
	id := uuid.NewString()
	k.w.runs[id] = domain.AnalysisRun{ID: id, RequestID: requestID, Kind: domain.RunKindPlan, Status: domain.RunStatusSucceeded, RawOutput: &raw, StartedAt: at}
}

func TestRunPlanGeneration_FirstPlanPromptHasNoPriorPlan(t *testing.T) {
	k, _, run, completer, worker := planRunKit(t, domain.RequestTypeChangeRequest, domain.RequestSizeM, []string{planJSON(true)})
	if err := worker.Execute(k.w.ctx(), run); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(completer.prompts[0], "prior_plan") {
		t.Fatal("a first Plan has no prior plan")
	}
}

func TestRunPlanGeneration_RevisionPromptCarriesTheLastSucceededPlan(t *testing.T) {
	k, req, run, completer, worker := planRunKit(t, domain.RequestTypeChangeRequest, domain.RequestSizeM, []string{planJSON(true)})
	now := time.Now().UTC()
	seedSucceededPlanRun(k, req.ID, `{"title":"OLDEST-PLAN"}`, now.Add(-3*time.Hour))
	seedSucceededPlanRun(k, req.ID, `{"title":"LATEST-PLAN"}`, now.Add(-2*time.Hour))
	failed := uuid.NewString()
	k.w.runs[failed] = domain.AnalysisRun{ID: failed, RequestID: req.ID, Kind: domain.RunKindPlan, Status: domain.RunStatusFailed, StartedAt: now.Add(-time.Hour)}
	if err := worker.Execute(k.w.ctx(), run); err != nil {
		t.Fatal(err)
	}
	prompt := completer.prompts[0]
	if !strings.Contains(prompt, "<prior_plan>") || !strings.Contains(prompt, "LATEST-PLAN") || strings.Contains(prompt, "OLDEST-PLAN") {
		t.Fatalf("the prompt must show the last succeeded Plan only:\n%s", prompt)
	}
}

func TestRunPlanGeneration_RunHistoryThatCannotBeReadFailsTheRun(t *testing.T) {
	k, _, run, completer, worker := planRunKit(t, domain.RequestTypeChangeRequest, domain.RequestSizeM, []string{planJSON(true)})
	worker.runs = failingSucceededList{worker.runs}
	if err := worker.Execute(k.w.ctx(), run); err == nil {
		t.Fatal("a history that cannot be read must stop the run instead of dropping the prior plan")
	}
	if completer.calls() != 0 {
		t.Fatal("the model must not be called without the prior plan")
	}
}

type failingSucceededList struct{ AnalysisRunStore }

func (failingSucceededList) ListSucceeded(context.Context, string, domain.RunKind) ([]domain.AnalysisRun, error) {
	return nil, errBoom
}
