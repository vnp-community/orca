package contracttest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// PlanRevisionEnv adds the stores of the version history to the analysis-run environment.
type PlanRevisionEnv struct {
	SolutionEnv
	Changes   usecase.OpenSpecChangeRepository
	Revisions usecase.OpenSpecChangeRevisionRepository
}

// RunPlanRevisionContract is what both dialects must do for the versions of a Plan (CR-REQ-045).
func RunPlanRevisionContract(t *testing.T, env PlanRevisionEnv) {
	t.Helper()
	scenarios := []struct {
		name string
		fn   func(t *testing.T, env PlanRevisionEnv)
	}{
		{"RequestWithoutVersions", revNoVersions},
		{"AppendJoinsTheTransactionOfTheChange", revAppendWithChange},
		{"RollbackLosesBothTheChangeAndTheVersion", revRollback},
		{"NumberAndRunAreUnique", revUniqueness},
		{"OnlyTheNewestVersionsKeepTheirText", revTextWindow},
		{"ListPagesNewestFirst", revListPaging},
		{"GetAndGetByRun", revGets},
		{"TenantIsolation", revTenantIsolation},
		{"ListSucceededReturnsTheSucceededRunsOfAKindOldestFirst", revListSucceeded},
	}
	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) { s.fn(t, env) })
	}
}

func revFixture(t *testing.T, env PlanRevisionEnv) (context.Context, domain.Request) {
	t.Helper()
	ctx := CtxForTenant(newTenant())
	return ctx, createRequest(t, env.Env, ctx, nil)
}

func newRevision(req domain.Request, n int, bundle map[string]string) domain.OpenSpecChangeRevision {
	change := domain.OpenSpecChange{Bundle: bundle}
	return domain.OpenSpecChangeRevision{
		ID: uuid.NewString(), RequestID: req.ID, Revision: n, RunID: uuid.NewString(), Commit: fmt.Sprintf("sha%04d", n),
		TasksMDDigest: fmt.Sprintf("tm-%d", n), BundleDigest: change.BundleDigest(), Files: domain.NewRevisionFiles(bundle), Bundle: bundle,
	}
}

func revNoVersions(t *testing.T, env PlanRevisionEnv) {
	ctx, req := revFixture(t, env)
	if _, found, err := env.Revisions.Get(ctx, req.ID, 1); err != nil || found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if _, found, err := env.Revisions.GetByRun(ctx, uuid.NewString()); err != nil || found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if got, err := env.Revisions.List(ctx, req.ID, 0, 10); err != nil || len(got) != 0 {
		t.Fatalf("list = %v err=%v", got, err)
	}
	if got, err := env.Revisions.List(ctx, "not-a-uuid", 0, 10); err != nil || len(got) != 0 {
		t.Fatalf("a malformed id lists nothing: %v err=%v", got, err)
	}
}

func revAppendWithChange(t *testing.T, env PlanRevisionEnv) {
	ctx, req := revFixture(t, env)
	bundle := map[string]string{"tasks.md": "# t\n", "design.md": "d\n", "big.md": "<omitted size=4194304 sha256=" + strings.Repeat("e", 64) + ">"}
	rev := newRevision(req, 1, bundle)
	err := env.Tx.InTx(ctx, func(ctx context.Context) error {
		if _, err := env.Changes.Upsert(ctx, domain.OpenSpecChange{ID: uuid.NewString(), RequestID: req.ID, ChangeID: "req-1-x", Status: domain.ChangeStatusReady,
			SyncState: domain.SyncStateInSync, Bundle: bundle}); err != nil {
			return err
		}
		return env.Revisions.Append(ctx, rev, domain.PlanRevisionTextKept)
	})
	if err != nil {
		t.Fatal(err)
	}
	got, found, err := env.Revisions.Get(ctx, req.ID, 1)
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if got.ID != rev.ID || got.RunID != rev.RunID || got.Commit != rev.Commit || got.TasksMDDigest != rev.TasksMDDigest || got.BundleDigest != rev.BundleDigest ||
		!got.TextKept || got.Bundle["tasks.md"] != "# t\n" || got.CreatedAt.IsZero() || time.Since(got.CreatedAt) > time.Hour {
		t.Fatalf("%+v", got)
	}
	if len(got.Files) != 3 || got.Files[0].Path != "big.md" || !got.Files[0].Omitted || got.Files[0].Size != 4194304 || got.Files[0].SHA256 != strings.Repeat("e", 64) {
		t.Fatalf("the index keeps the real size and hash of an omitted file: %+v", got.Files)
	}
	if _, found, _ := env.Changes.GetByRequest(ctx, req.ID); !found {
		t.Fatal("the change was stored with its version")
	}
}

func revRollback(t *testing.T, env PlanRevisionEnv) {
	ctx, req := revFixture(t, env)
	boom := errors.New("later step failed")
	err := env.Tx.InTx(ctx, func(ctx context.Context) error {
		if _, err := env.Changes.Upsert(ctx, domain.OpenSpecChange{ID: uuid.NewString(), RequestID: req.ID, ChangeID: "req-1-x", Status: domain.ChangeStatusReady, SyncState: domain.SyncStateInSync}); err != nil {
			return err
		}
		if err := env.Revisions.Append(ctx, newRevision(req, 1, map[string]string{"a": "b"}), domain.PlanRevisionTextKept); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if _, found, _ := env.Revisions.Get(ctx, req.ID, 1); found {
		t.Fatal("the version must roll back with the transaction")
	}
	if _, found, _ := env.Changes.GetByRequest(ctx, req.ID); found {
		t.Fatal("the change must roll back with its version")
	}
}

func revUniqueness(t *testing.T, env PlanRevisionEnv) {
	ctx, req := revFixture(t, env)
	first := newRevision(req, 1, map[string]string{"a": "b"})
	if err := env.Revisions.Append(ctx, first, domain.PlanRevisionTextKept); err != nil {
		t.Fatal(err)
	}
	sameNumber := newRevision(req, 1, map[string]string{"a": "c"})
	if err := env.Revisions.Append(ctx, sameNumber, domain.PlanRevisionTextKept); err == nil {
		t.Fatal("a second version with the same number must be refused")
	}
	sameRun := newRevision(req, 2, map[string]string{"a": "c"})
	sameRun.RunID = first.RunID
	if err := env.Revisions.Append(ctx, sameRun, domain.PlanRevisionTextKept); err == nil {
		t.Fatal("a second version of the same run must be refused")
	}
	if got, _ := env.Revisions.List(ctx, req.ID, 0, 10); len(got) != 1 {
		t.Fatalf("a refused version leaves nothing behind: %d", len(got))
	}
}

func revTextWindow(t *testing.T, env PlanRevisionEnv) {
	ctx, req := revFixture(t, env)
	total := domain.PlanRevisionTextKept + 1
	for n := 1; n <= total; n++ {
		if err := env.Revisions.Append(ctx, newRevision(req, n, map[string]string{"tasks.md": fmt.Sprintf("version %d", n)}), domain.PlanRevisionTextKept); err != nil {
			t.Fatal(err)
		}
	}
	oldest, found, _ := env.Revisions.Get(ctx, req.ID, 1)
	if !found || oldest.TextKept || oldest.Bundle != nil {
		t.Fatalf("version 1 loses its text when version %d arrives: %+v", total, oldest)
	}
	if len(oldest.Files) != 1 || oldest.Files[0].Path != "tasks.md" || oldest.BundleDigest == "" || oldest.Commit == "" {
		t.Fatalf("the index and digests of an old version stay: %+v", oldest)
	}
	for n := 2; n <= total; n++ {
		got, found, _ := env.Revisions.Get(ctx, req.ID, n)
		if !found || !got.TextKept || got.Bundle["tasks.md"] != fmt.Sprintf("version %d", n) {
			t.Fatalf("version %d keeps its text: %+v", n, got)
		}
	}
	listed, _ := env.Revisions.List(ctx, req.ID, 0, 100)
	for _, r := range listed {
		if r.Bundle != nil {
			t.Fatalf("a listing never carries the text: version %d", r.Revision)
		}
		if want := r.Revision >= 2; r.TextKept != want {
			t.Fatalf("listing: version %d TextKept = %v", r.Revision, r.TextKept)
		}
	}
}

func revListPaging(t *testing.T, env PlanRevisionEnv) {
	ctx, req := revFixture(t, env)
	for n := 1; n <= 25; n++ {
		if err := env.Revisions.Append(ctx, newRevision(req, n, map[string]string{"a": "b"}), domain.PlanRevisionTextKept); err != nil {
			t.Fatal(err)
		}
	}
	page1, err := env.Revisions.List(ctx, req.ID, 0, 10)
	if err != nil || len(page1) != 10 || page1[0].Revision != 25 || page1[9].Revision != 16 {
		t.Fatalf("page 1: %v err=%v", revNumbers(page1), err)
	}
	page2, _ := env.Revisions.List(ctx, req.ID, page1[9].Revision, 10)
	if len(page2) != 10 || page2[0].Revision != 15 || page2[9].Revision != 6 {
		t.Fatalf("page 2: %v", revNumbers(page2))
	}
	page3, _ := env.Revisions.List(ctx, req.ID, page2[9].Revision, 10)
	if len(page3) != 5 || page3[4].Revision != 1 {
		t.Fatalf("page 3: %v", revNumbers(page3))
	}
	if all, _ := env.Revisions.List(ctx, req.ID, 0, 0); len(all) != 25 {
		t.Fatalf("no limit means every version: %d", len(all))
	}
}

func revNumbers(revs []domain.OpenSpecChangeRevision) []int {
	out := make([]int, len(revs))
	for i, r := range revs {
		out[i] = r.Revision
	}
	return out
}

func revGets(t *testing.T, env PlanRevisionEnv) {
	ctx, req := revFixture(t, env)
	one, two := newRevision(req, 1, map[string]string{"a": "1"}), newRevision(req, 2, map[string]string{"a": "2"})
	for _, r := range []domain.OpenSpecChangeRevision{one, two} {
		if err := env.Revisions.Append(ctx, r, domain.PlanRevisionTextKept); err != nil {
			t.Fatal(err)
		}
	}
	if got, found, err := env.Revisions.Get(ctx, req.ID, 2); err != nil || !found || got.RunID != two.RunID {
		t.Fatalf("%+v %v %v", got, found, err)
	}
	if _, found, err := env.Revisions.Get(ctx, req.ID, 3); err != nil || found {
		t.Fatalf("an unknown number is not found: %v %v", found, err)
	}
	if _, found, err := env.Revisions.Get(ctx, "not-a-uuid", 1); err != nil || found {
		t.Fatalf("a malformed request id is not found: %v %v", found, err)
	}
	if got, found, err := env.Revisions.GetByRun(ctx, one.RunID); err != nil || !found || got.Revision != 1 || got.Bundle["a"] != "1" {
		t.Fatalf("%+v %v %v", got, found, err)
	}
	if _, found, err := env.Revisions.GetByRun(ctx, "not-a-uuid"); err != nil || found {
		t.Fatalf("a malformed run id is not found: %v %v", found, err)
	}
}

func revTenantIsolation(t *testing.T, env PlanRevisionEnv) {
	ctx, req := revFixture(t, env)
	rev := newRevision(req, 1, map[string]string{"a": "b"})
	if err := env.Revisions.Append(ctx, rev, domain.PlanRevisionTextKept); err != nil {
		t.Fatal(err)
	}
	other := tenant.WithTenantID(context.Background(), newTenant())
	if _, found, _ := env.Revisions.Get(other, req.ID, 1); found {
		t.Fatal("another tenant read a version")
	}
	if _, found, _ := env.Revisions.GetByRun(other, rev.RunID); found {
		t.Fatal("another tenant read a version by run")
	}
	if got, _ := env.Revisions.List(other, req.ID, 0, 10); len(got) != 0 {
		t.Fatal("another tenant listed versions")
	}
}

func revListSucceeded(t *testing.T, env PlanRevisionEnv) {
	k := newSolKit(t, env.SolutionEnv)
	r := k.seed(t, domain.RequestTypeChangeRequest, domain.RequestStatusPlanning)
	finish := func(kind domain.RunKind, succeed bool) domain.AnalysisRun {
		run, draft := startInput(t, k, r, domain.AnalysisModeComplete, kind, "")
		if kind == domain.RunKindPlan {
			run.SolutionID, draft = "", domain.Solution{}
		}
		res, err := startIn(t, k, run, draft, 0)
		if err != nil || !res.Created {
			t.Fatalf("start %s: %+v %v", kind, res, err)
		}
		done := res.Run
		if succeed {
			done.Succeed(time.Now())
		} else {
			done.Fail("REQUEST_PLAN_AI_FAILED", "failed", time.Now())
		}
		if ok, err := env.AnalysisRuns.FinishOwned(k.ctx, done, "w-test"); err != nil || !ok {
			t.Fatalf("finish %s: %v %v", kind, ok, err)
		}
		time.Sleep(5 * time.Millisecond) // started_at orders the versions
		return done
	}
	first := finish(domain.RunKindPlan, true)
	finish(domain.RunKindPlan, false)
	finish(domain.RunKindSolution, true)
	third := finish(domain.RunKindPlan, true)

	got, err := env.AnalysisRuns.ListSucceeded(k.ctx, r.ID, domain.RunKindPlan)
	if err != nil || len(got) != 2 || got[0].ID != first.ID || got[1].ID != third.ID {
		t.Fatalf("want the two succeeded Plan runs oldest first, got %d: %+v %v", len(got), got, err)
	}
	if sol, _ := env.AnalysisRuns.ListSucceeded(k.ctx, r.ID, domain.RunKindSolution); len(sol) != 1 {
		t.Fatalf("a kind lists its own runs only: %d", len(sol))
	}
	if none, err := env.AnalysisRuns.ListSucceeded(CtxForTenant(newTenant()), r.ID, domain.RunKindPlan); err != nil || len(none) != 0 {
		t.Fatalf("another tenant sees nothing: %d %v", len(none), err)
	}
	if none, err := env.AnalysisRuns.ListSucceeded(k.ctx, "not-a-uuid", domain.RunKindPlan); err != nil || len(none) != 0 {
		t.Fatalf("a malformed id lists nothing: %d %v", len(none), err)
	}
}
