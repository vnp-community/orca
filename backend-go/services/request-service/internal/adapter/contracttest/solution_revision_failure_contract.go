package contracttest

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

const failedRevisionAsk = "ASK-FAILED mantén el esquema y reduce el despliegue"

// solFailedRevisionIsRecoverable: the old proposal is superseded the moment a revision starts, so a revision that fails
// leaves the Request in analyzing with no proposal. Regenerate must work, must carry the ask that failed, and the
// superseded version must reach the next prompt.
func solFailedRevisionIsRecoverable(t *testing.T, env SolutionEnv) {
	k := newSolKit(t, env)
	k.completer.replies = []string{validSolutionDoc(t)}
	r := k.seed(t, domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)
	first, err := k.generate.Execute(k.ctx, usecase.GenerateSolutionInput{RequestID: r.ID})
	if err != nil {
		t.Fatal(err)
	}
	k.waitRun(t, first.RunID)

	k.completer.mu.Lock()
	k.completer.replies = []string{`{"schema_version":1,"options":[]}`}
	k.completer.mu.Unlock()
	failing, err := k.generate.Execute(k.ctx, usecase.GenerateSolutionInput{RequestID: r.ID, Feedback: failedRevisionAsk})
	if err != nil {
		t.Fatal(err)
	}
	run := k.waitRun(t, failing.RunID)
	if run.Status != domain.RunStatusFailed || run.ErrorCode == nil || *run.ErrorCode != domain.RunErrInvalidOutput {
		t.Fatalf("revision run = %+v", run)
	}
	if got := k.requestStatus(t, r.ID); got != domain.RequestStatusAnalyzing {
		t.Fatalf("request = %s after the failed revision, want analyzing", got)
	}
	old, err := env.SolutionStore.Get(k.ctx, first.SolutionID)
	if err != nil || old.Status != domain.SolutionStatusSuperseded {
		t.Fatalf("old proposal = %s, %v", old.Status, err)
	}
	if got, err := env.Approvals.FindPendingBySubject(k.ctx, k.tenantID, domain.SubjectSolution, first.SolutionID); err != nil || got != nil {
		t.Fatalf("approval of the superseded proposal is still pending: %+v %v", got, err)
	}
	if n := len(solutionsOf(t, k, r.ID)); n != 1 {
		t.Fatalf("%d solutions after the failure, want only the superseded one", n)
	}
	list, err := k.list.Execute(k.ctx, usecase.ListSolutionsInput{RequestID: r.ID})
	if err != nil || len(list.Runs) != 2 || list.Runs[0].Status != domain.RunStatusFailed {
		t.Fatalf("the failure must be visible to the UI: %+v %v", list.Runs, err)
	}

	k.completer.mu.Lock()
	k.completer.replies, k.completer.prompts = []string{validSolutionDoc(t)}, nil
	k.completer.mu.Unlock()
	retry, err := k.generate.Execute(k.ctx, usecase.GenerateSolutionInput{RequestID: r.ID})
	if err != nil || retry.RunID == failing.RunID {
		t.Fatalf("regenerate after the failure: %+v %v", retry, err)
	}
	done := k.waitRun(t, retry.RunID)
	if done.Status != domain.RunStatusSucceeded || done.Feedback != failedRevisionAsk {
		t.Fatalf("retry run = %+v, want success carrying the failed ask", done)
	}
	prompt := k.completer.prompts[0]
	fb := strings.Index(prompt, "<feedback>")
	closed := strings.Index(prompt, "</prior_artifacts>")
	if fb < 0 || !strings.Contains(prompt[fb:], failedRevisionAsk) || closed < 0 || !strings.Contains(prompt[:closed], "status=superseded") {
		t.Fatalf("retry prompt lacks the failed ask or the superseded version:\n%s", prompt)
	}
	k.pendingApproval(t, domain.SubjectSolution, retry.SolutionID)
	if got := k.requestStatus(t, r.ID); got != domain.RequestStatusAwaitingAnalysisApproval {
		t.Fatalf("request = %s after the retry", got)
	}
}

// solConcurrentRevisionsStartOneRun: several change requests race on one pending proposal. One wins; the rest are told the
// Request moved on (REQUEST_SOLUTION_WRONG_STATE) and start nothing. The same click repeated with one key is one run.
func solConcurrentRevisionsStartOneRun(t *testing.T, env SolutionEnv) {
	k := newSolKit(t, env)
	k.completer.replies = []string{validSolutionDoc(t)}
	r := k.seed(t, domain.RequestTypeChangeRequest, domain.RequestStatusAnalyzing)
	first, err := k.generate.Execute(k.ctx, usecase.GenerateSolutionInput{RequestID: r.ID})
	if err != nil {
		t.Fatal(err)
	}
	k.waitRun(t, first.RunID)

	// Keeps the winning run running while the others arrive.
	hold := make(chan struct{})
	k.completer.mu.Lock()
	k.completer.hold = hold
	k.completer.mu.Unlock()

	const n = 6
	results := make([]usecase.GenerateSolutionResult, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = retryDeadlock(func() error {
				res, err := k.generate.Execute(k.ctx, usecase.GenerateSolutionInput{RequestID: r.ID, Feedback: "ASK-RACE", IdempotencyKey: "race-" + string(rune('a'+i))})
				results[i] = res
				return err
			})
		}()
	}
	wg.Wait()
	winner, lost := "", 0
	for i := range errs {
		switch {
		case errs[i] == nil && results[i].Created && winner == "":
			winner = results[i].RunID
		case hasErrorCode(errs[i], "REQUEST_SOLUTION_WRONG_STATE"):
			lost++
		default:
			t.Fatalf("racer %d: %+v %v", i, results[i], errs[i])
		}
	}
	if winner == "" || lost != n-1 {
		t.Fatalf("winner=%q lost=%d of %d", winner, lost, n)
	}

	// Same click again with the winner's key: the same run, nothing new.
	var winnerKey string
	for i := range results {
		if results[i].RunID == winner {
			winnerKey = "race-" + string(rune('a'+i))
		}
	}
	again, err := k.generate.Execute(k.ctx, usecase.GenerateSolutionInput{RequestID: r.ID, Feedback: "ASK-RACE", IdempotencyKey: winnerKey})
	if err != nil || again.Created || again.RunID != winner {
		t.Fatalf("repeated click: %+v %v", again, err)
	}

	close(hold)
	if run := k.waitRun(t, winner); run.Status != domain.RunStatusSucceeded {
		t.Fatalf("winning run = %+v", run)
	}
	list, err := k.list.Execute(k.ctx, usecase.ListSolutionsInput{RequestID: r.ID})
	if err != nil || len(list.Runs) != 2 {
		t.Fatalf("runs = %d, want the first attempt and exactly one revision (%v)", len(list.Runs), err)
	}
	if len(list.Solutions) != 2 || list.Solutions[0].Status != domain.SolutionStatusSuperseded || list.Solutions[1].Status != domain.SolutionStatusProposed {
		t.Fatalf("versions = %+v", list.Solutions)
	}
	k.pendingApproval(t, domain.SubjectSolution, list.Solutions[1].ID)
}

func hasErrorCode(err error, code string) bool {
	var ae *apperrors.AppError
	return errors.As(err, &ae) && ae.Code == code
}
