package usecase

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func TestListSolutions_RevisionFeedback_BatchedNoNPlusOne(t *testing.T) {
	w := newSolWorld()
	r := w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)
	const versions = 30
	for i := 0; i < versions; i++ {
		runID := uuid.NewString()
		fb := ""
		if i%3 != 0 {
			fb = fmt.Sprintf("ask %d", i)
		}
		w.runs[runID] = domain.AnalysisRun{ID: runID, TenantID: "t1", RequestID: r.ID, Kind: domain.RunKindSolution, Status: domain.RunStatusSucceeded, Feedback: fb, StartedAt: time.Now()}
		w.solutions[uuid.NewString()] = domain.Solution{
			ID: runID + "-s", TenantID: "t1", RequestID: r.ID, Kind: domain.SolutionKindSolution, Status: domain.SolutionStatusSuperseded,
			OptionsJSON: []byte(`{}`), GenerationRunID: runID, Version: 2, CreatedAt: time.Now().Add(time.Duration(i) * time.Second),
		}
	}
	res := must(NewListSolutions(w, fakeSolutions{w}, fakeRuns{w}).Execute(w.ctx(), ListSolutionsInput{RequestID: r.ID}))
	if len(res.Solutions) != versions || w.feedbackReads != 1 {
		t.Fatalf("solutions = %d, feedback reads = %d; want %d solutions and one batched read", len(res.Solutions), w.feedbackReads, versions)
	}
	withFeedback := 0
	for _, s := range res.Solutions {
		if s.RevisionFeedback != w.runs[s.GenerationRunID].Feedback {
			t.Fatalf("solution %s: %q", s.ID, s.RevisionFeedback)
		}
		if s.RevisionFeedback != "" {
			withFeedback++
		}
	}
	if withFeedback != 20 {
		t.Fatalf("versions with feedback = %d", withFeedback)
	}
}

func TestListSolutions_RevisionFeedback_NoSolutionsNoRead(t *testing.T) {
	w := newSolWorld()
	r := w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)
	res := must(NewListSolutions(w, fakeSolutions{w}, fakeRuns{w}).Execute(w.ctx(), ListSolutionsInput{RequestID: r.ID}))
	if len(res.Solutions) != 0 || w.feedbackReads != 0 {
		t.Fatalf("solutions=%d reads=%d", len(res.Solutions), w.feedbackReads)
	}
}

func TestListSolutions_RevisionFeedback_ReadFailureIsReturned(t *testing.T) {
	w := newSolWorld()
	r := w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)
	w.solutions["s1"] = domain.Solution{ID: "s1", TenantID: "t1", RequestID: r.ID, Kind: domain.SolutionKindSolution, Status: domain.SolutionStatusProposed, GenerationRunID: uuid.NewString(), CreatedAt: time.Now()}
	w.failFeedback = errors.New("db down")
	_, err := NewListSolutions(w, fakeSolutions{w}, fakeRuns{w}).Execute(w.ctx(), ListSolutionsInput{RequestID: r.ID})
	if err == nil || !strings.Contains(err.Error(), "db down") {
		t.Fatalf("err = %v", err)
	}
}

func TestRunFeedbackBatches(t *testing.T) {
	if got := RunFeedbackBatches(nil); len(got) != 0 {
		t.Fatalf("nil -> %v", got)
	}
	if got := RunFeedbackBatches([]string{"", "a", "b", "a", ""}); len(got) != 1 || strings.Join(got[0], ",") != "a,b" {
		t.Fatalf("dedupe -> %v", got)
	}
	ids := make([]string, 0, 2*RunFeedbackBatchSize+1)
	for i := 0; i < 2*RunFeedbackBatchSize+1; i++ {
		ids = append(ids, fmt.Sprintf("id-%d", i))
	}
	got := RunFeedbackBatches(append(ids, ids[:10]...))
	if len(got) != 3 || len(got[0]) != RunFeedbackBatchSize || len(got[1]) != RunFeedbackBatchSize || len(got[2]) != 1 {
		t.Fatalf("batch sizes = %d/%d/%d (%d batches)", len(got[0]), len(got[1]), len(got[2]), len(got))
	}
}
