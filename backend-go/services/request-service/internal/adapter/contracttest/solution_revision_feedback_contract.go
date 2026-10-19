package contracttest

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// startRunWithFeedback commits a run (and its draft) carrying feedback for a fresh Request.
func startRunWithFeedback(t *testing.T, k *solKit, feedback string) domain.AnalysisRun {
	t.Helper()
	r := k.seed(t, domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)
	run, draft := startInput(t, k, r, domain.AnalysisModeComplete, domain.RunKindSolution, "")
	run.Feedback = feedback
	if _, err := startIn(t, k, run, draft, 0); err != nil {
		t.Fatal(err)
	}
	return run
}

// solRunFeedbackByIDs pins the batched read of run feedback: empty and repeated ids, a list longer than one batch,
// ids that name no run, text that must survive the database, and tenant isolation.
func solRunFeedbackByIDs(t *testing.T, env SolutionEnv) {
	k := newSolKit(t, env)
	store := env.SolutionStore

	got, err := store.RunFeedbackByIDs(k.ctx, nil)
	if err != nil || len(got) != 0 {
		t.Fatalf("nil ids -> %v, %v", got, err)
	}
	if got, err = store.RunFeedbackByIDs(k.ctx, []string{}); err != nil || len(got) != 0 {
		t.Fatalf("empty ids -> %v, %v", got, err)
	}
	if got, err = store.RunFeedbackByIDs(k.ctx, []string{"", "not-a-uuid"}); err != nil || len(got) != 0 {
		t.Fatalf("blank and malformed ids must be skipped, not fail: %v, %v", got, err)
	}

	long := strings.Repeat("đổi sang cách khác 🙂\n", 100) // multi-line, multi-byte, 2000 runes
	withText := startRunWithFeedback(t, k, long)
	second := startRunWithFeedback(t, k, "second ask")
	third := startRunWithFeedback(t, k, "third ask")
	none := startRunWithFeedback(t, k, "")

	got, err = store.RunFeedbackByIDs(k.ctx, []string{withText.ID, second.ID, none.ID, uuid.NewString()})
	if err != nil || len(got) != 2 || got[withText.ID] != long || got[second.ID] != "second ask" {
		t.Fatalf("many ids -> %d entries, err=%v", len(got), err)
	}
	if _, ok := got[none.ID]; ok {
		t.Fatal("a run without feedback must be absent from the map")
	}

	got, err = store.RunFeedbackByIDs(k.ctx, []string{second.ID, second.ID, second.ID})
	if err != nil || len(got) != 1 || got[second.ID] != "second ask" {
		t.Fatalf("repeated ids -> %v, %v", got, err)
	}

	// More ids than one batch: the real runs sit in the first, a middle and the last batch.
	ids := []string{withText.ID}
	for i := 0; i < 2*usecase.RunFeedbackBatchSize+100; i++ {
		ids = append(ids, uuid.NewString())
		if i == usecase.RunFeedbackBatchSize+10 {
			ids = append(ids, second.ID)
		}
	}
	ids = append(ids, third.ID)
	got, err = store.RunFeedbackByIDs(k.ctx, ids)
	if err != nil || len(got) != 3 || got[withText.ID] != long || got[second.ID] != "second ask" || got[third.ID] != "third ask" {
		t.Fatalf("over one batch (%d ids) -> %d entries, err=%v", len(ids), len(got), err)
	}

	foreign := CtxForTenant(newTenant())
	if got, err = store.RunFeedbackByIDs(foreign, []string{withText.ID, second.ID}); err != nil || len(got) != 0 {
		t.Fatalf("another tenant read feedback: %v, %v", got, err)
	}
	if _, err = store.RunFeedbackByIDs(tenant.WithTenantID(k.ctx, ""), []string{second.ID}); err == nil {
		t.Fatal("a read without a tenant must fail")
	}
}

// solRevisionRoundsCarryFeedback runs a first attempt and three requested revisions end to end: every listed version
// shows the feedback that produced it (use case and RPC) and the last prompt carries all earlier asks.
func solRevisionRoundsCarryFeedback(t *testing.T, env SolutionEnv) {
	k := newSolKit(t, env)
	k.completer.replies = []string{validSolutionDoc(t)}
	r := k.seed(t, domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)
	asks := []string{"", "ASK-ONE giữ nguyên schema", "ASK-TWO thêm rollback", "ASK-THREE thu hẹp phạm vi"}
	var solutionIDs []string
	for _, ask := range asks {
		res, err := k.generate.Execute(k.ctx, usecase.GenerateSolutionInput{RequestID: r.ID, Feedback: ask})
		if err != nil {
			t.Fatalf("round %q: %v", ask, err)
		}
		if run := k.waitRun(t, res.RunID); run.Status != domain.RunStatusSucceeded {
			t.Fatalf("round %q: run = %+v", ask, run)
		}
		solutionIDs = append(solutionIDs, res.SolutionID)
	}

	list, err := k.list.Execute(k.ctx, usecase.ListSolutionsInput{RequestID: r.ID})
	if err != nil || len(list.Solutions) != len(asks) {
		t.Fatalf("list: %d solutions, %v", len(list.Solutions), err)
	}
	for i, s := range list.Solutions {
		if s.ID != solutionIDs[i] || s.RevisionFeedback != asks[i] {
			t.Errorf("version %d: id match=%v revision_feedback=%q, want %q", i+1, s.ID == solutionIDs[i], s.RevisionFeedback, asks[i])
		}
	}
	rpc, err := k.client.ListSolutions(k.rpc(), &requestv1.ListSolutionsRequest{RequestId: r.ID})
	if err != nil || len(rpc.GetSolutions()) != len(asks) {
		t.Fatalf("rpc list: %v", err)
	}
	for i, s := range rpc.GetSolutions() {
		if s.GetRevisionFeedback() != asks[i] {
			t.Errorf("rpc version %d revision_feedback = %q, want %q", i+1, s.GetRevisionFeedback(), asks[i])
		}
	}

	prompt := k.completer.prompts[len(k.completer.prompts)-1]
	open, closed := strings.Index(prompt, "<prior_artifacts>"), strings.Index(prompt, "</prior_artifacts>")
	if open < 0 || closed < open {
		t.Fatalf("last prompt has no prior_artifacts block:\n%s", prompt)
	}
	for _, earlier := range asks[1:3] {
		if at := strings.Index(prompt, earlier); at < open || at > closed {
			t.Errorf("earlier ask %q is not inside <prior_artifacts>", earlier)
		}
	}
	if fb := strings.Index(prompt, "<feedback>"); fb < closed || !strings.Contains(prompt[fb:], asks[3]) {
		t.Error("the newest ask must sit in <feedback>")
	}
	if n := strings.Count(prompt, "--- artifact "); n != 3 {
		t.Errorf("prior artifacts in the last prompt = %d, want 3", n)
	}
}
