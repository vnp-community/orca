package usecase

import (
	"slices"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// baseRefRig seeds an executing OpenSpec Request with one open task, so the same setup reaches both dispatch paths.
func baseRefRig(t *testing.T, baseRef string, contract bool) (*exRig, *contractTasks, domain.Request) {
	t.Helper()
	openspec := domain.EngineOpenSpec
	r := newExRig(t)
	req := r.store.seed(func(q *domain.Request) {
		q.Type, q.Size, q.Status, q.SolutionEngine, q.BaseRef = domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting, &openspec, baseRef
	})
	r.leaf(req, r.plan(req), "do it", domain.TaskStatusOpen)
	ct := &contractTasks{exTasks: r.tasks}
	if contract {
		r.advance.Contract = OpenSpecContractRuns{NewNonce: func() string { return "n0nce-n0nce-n0nce-n0nce" }}
		r.advance.Tasks = ct
	}
	return r, ct, req
}

func TestAdvanceExecution_Dispatch_PassesBaseRef_BothPaths(t *testing.T) {
	t.Run("plain path", func(t *testing.T) {
		r, ct, req := baseRefRig(t, "release/x", false)
		advance(t, r, req, "")
		if !slices.Equal(r.tasks.baseRefs, []string{"release/x"}) || len(ct.runs) != 0 {
			t.Fatalf("plain baseRefs=%v contract runs=%v", r.tasks.baseRefs, ct.runs)
		}
	})
	t.Run("contract path", func(t *testing.T) {
		r, ct, req := baseRefRig(t, "release/x", true)
		advance(t, r, req, "")
		if len(ct.runs) != 1 || ct.runs[0].BaseRef != "release/x" {
			t.Fatalf("contract runs = %+v", ct.runs)
		}
		if !slices.Equal(r.tasks.baseRefs, []string{"release/x"}) {
			t.Fatalf("the contract dispatch must reach the task client with the same ref: %v", r.tasks.baseRefs)
		}
	})
}

func TestAdvanceExecution_Dispatch_EmptyBaseRefStaysEmpty(t *testing.T) {
	for name, contract := range map[string]bool{"plain": false, "contract": true} {
		r, ct, req := baseRefRig(t, "", contract)
		advance(t, r, req, "")
		if !slices.Equal(r.tasks.baseRefs, []string{""}) || (contract && ct.runs[0].BaseRef != "") {
			t.Errorf("%s: an unset base ref must be sent empty so task-service resolves the repository default; got %v", name, r.tasks.baseRefs)
		}
	}
}

func TestAdvanceExecution_Dispatch_ReadsTheBaseRefOfTheRequestAtDispatchTime(t *testing.T) {
	r, _, req := baseRefRig(t, "main", false)
	// The Request is changed after the rig was built (a base ref set while it was still unlocked): the next pass must see it.
	cur := r.store.requests[req.ID]
	cur.BaseRef = "release/y"
	r.store.requests[req.ID] = cur
	advance(t, r, req, "")
	if !slices.Equal(r.tasks.baseRefs, []string{"release/y"}) {
		t.Fatalf("dispatched with a stale copy of the Request: %v", r.tasks.baseRefs)
	}
}
