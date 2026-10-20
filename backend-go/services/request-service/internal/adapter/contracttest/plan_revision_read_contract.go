package contracttest

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// RunPlanRevisionReadContract is what both dialects must answer when the versions of a Plan are read back (CR-REQ-045): the
// list, one version of the proposal, and one file of an older version, over the real run and version stores.
func RunPlanRevisionReadContract(t *testing.T, env PlanRevisionEnv) {
	t.Helper()
	scenarios := []struct {
		name string
		fn   func(t *testing.T, env PlanRevisionEnv)
	}{
		{"ThreeVersionsReadBackOneByOne", planReadThreeVersions},
		{"PrunedTextAndPaging", planReadPrunedAndPaged},
	}
	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) { s.fn(t, env) })
	}
}

type planReadReaders struct {
	list *usecase.ListPlanRevisions
	get  *usecase.GetPlanProposal
	file *usecase.GetPlanBundleFile
}

func newPlanReadReaders(env PlanRevisionEnv) planReadReaders {
	return planReadReaders{
		list: usecase.NewListPlanRevisions(env.Requests, env.AnalysisRuns, env.Revisions, env.Changes),
		get:  usecase.NewGetPlanProposal(env.Requests, env.AnalysisRuns).WithChanges(env.Changes).WithRevisions(env.Revisions),
		file: usecase.NewGetPlanBundleFile(env.Requests, env.Changes).WithRevisions(env.AnalysisRuns, env.Revisions),
	}
}

// addPlanVersion writes what one authoring run leaves behind: the run, the current change and the stored version, the
// last two together as the worker does.
func addPlanVersion(t *testing.T, env PlanRevisionEnv, k *solKit, r domain.Request, n int, feedback string, bundle map[string]string) domain.AnalysisRun {
	t.Helper()
	run, draft := startInput(t, k, r, domain.AnalysisModeAgentProposal, domain.RunKindPlan, "")
	run.SolutionID, draft, run.Feedback = "", domain.Solution{}, feedback
	res, err := startIn(t, k, run, draft, 0)
	if err != nil || !res.Created {
		t.Fatalf("start version %d: %+v %v", n, res, err)
	}
	raw, err := usecase.EncodePlanProposal(domain.PlanProposal{Title: fmt.Sprintf("Plan %d", n), Summary: "s", Tasks: []domain.TaskProposal{{Title: "t", TaskType: "task"}}})
	if err != nil {
		t.Fatal(err)
	}
	done := res.Run
	done.Succeed(time.Now())
	done.RawOutput = &raw
	err = env.Tx.InTx(k.ctx, func(ctx context.Context) error {
		change, found, err := env.Changes.GetByRequest(ctx, r.ID)
		if err != nil {
			return err
		}
		next := domain.OpenSpecChange{RequestID: r.ID, ChangeID: "req-1-x", Branch: "request/1-proposal", Status: domain.ChangeStatusReady, SyncState: domain.SyncStateInSync,
			Commit: fmt.Sprintf("sha%04d", n), Bundle: bundle}
		if found {
			next.ID = change.ID
		} else {
			next.ID = uuid.NewString()
		}
		stored, err := env.Changes.Upsert(ctx, next)
		if err != nil {
			return err
		}
		return env.Revisions.Append(ctx, domain.OpenSpecChangeRevision{
			ID: uuid.NewString(), RequestID: r.ID, Revision: n, RunID: done.ID, Commit: stored.Commit, TasksMDDigest: "tm", BundleDigest: stored.BundleDigest(),
			Files: domain.NewRevisionFiles(bundle), Bundle: bundle,
		}, domain.PlanRevisionTextKept)
	})
	if err != nil {
		t.Fatalf("store version %d: %v", n, err)
	}
	if ok, err := env.AnalysisRuns.FinishOwned(k.ctx, done, "w-test"); err != nil || !ok {
		t.Fatalf("finish version %d: %v %v", n, ok, err)
	}
	time.Sleep(5 * time.Millisecond) // started_at orders the versions
	return done
}

func planReadThreeVersions(t *testing.T, env PlanRevisionEnv) {
	k := newSolKit(t, env.SolutionEnv)
	r := k.seed(t, domain.RequestTypeChangeRequest, domain.RequestStatusPlanning)
	rd := newPlanReadReaders(env)
	addPlanVersion(t, env, k, r, 1, "", map[string]string{"tasks.md": "t1\n", "proposal.md": "p1\n"})
	second := addPlanVersion(t, env, k, r, 2, "add a design", map[string]string{"tasks.md": "t1\n", "proposal.md": "p2\n", "design.md": "d2\n"})
	third := addPlanVersion(t, env, k, r, 3, "drop it", map[string]string{"tasks.md": "t3\n", "proposal.md": "p2\n"})

	page, err := rd.list.Execute(k.ctx, r.ID, 0, "")
	if err != nil || len(page.Revisions) != 3 || page.NextPageToken != "" {
		t.Fatalf("list: %+v %v", page, err)
	}
	v3, v2, v1 := page.Revisions[0], page.Revisions[1], page.Revisions[2]
	if v3.Revision != 3 || v3.RunID != third.ID || v3.Status != "current" || v3.RevisionFeedback != "drop it" || v3.FileCount != 2 || v3.FilesChanged != 2 || v3.ActorID != k.userID || v3.GeneratedAt.IsZero() {
		t.Fatalf("v3 = %+v", v3)
	}
	if v2.RunID != second.ID || v2.Status != "superseded" || v2.FileCount != 3 || v2.FilesChanged != 2 || v2.Commit != "sha0002" || !v2.TextKept {
		t.Fatalf("v2 = %+v", v2)
	}
	if v1.RevisionFeedback != "" || v1.FileCount != 2 || v1.FilesChanged != 2 || v1.Engine != "openspec" {
		t.Fatalf("v1 = %+v", v1)
	}

	view, err := rd.get.Execute(k.ctx, r.ID, "", 2)
	if err != nil || view.RunID != second.ID || view.Proposal == nil || view.Proposal.Title != "Plan 2" || view.Bundle == nil || view.Checks != nil {
		t.Fatalf("v2 proposal: %+v %v", view, err)
	}
	changes := map[string]string{}
	for _, f := range view.Bundle.Files {
		changes[f.Path] = f.Change
	}
	if changes["design.md"] != "added" || changes["proposal.md"] != "changed" || changes["tasks.md"] != "unchanged" {
		t.Fatalf("v2 changes = %v", changes)
	}
	cur, err := rd.get.Execute(k.ctx, r.ID, "", 0)
	if err != nil || cur.RunID != third.ID || cur.Checks == nil {
		t.Fatalf("current: %+v %v", cur, err)
	}
	removed := false
	for _, f := range cur.Bundle.Files {
		removed = removed || (f.Path == "design.md" && f.Change == "removed")
	}
	if !removed {
		t.Fatalf("the current version lists the file it removed: %+v", cur.Bundle.Files)
	}

	f, err := rd.file.Execute(k.ctx, r.ID, "design.md", 2)
	if err != nil || f.Content != "d2\n" {
		t.Fatalf("v2 design.md = %+v %v", f, err)
	}
	if _, err := rd.file.Execute(k.ctx, r.ID, "design.md", 3); !hasErrorCode(err, "REQUEST_PLAN_BUNDLE_FILE_NOT_FOUND") {
		t.Fatalf("v3 has no design.md: %v", err)
	}
	if _, err := rd.get.Execute(k.ctx, r.ID, "", 4); !hasErrorCode(err, "REQUEST_PLAN_REVISION_NOT_FOUND") {
		t.Fatalf("v4: %v", err)
	}
	if _, err := rd.list.Execute(CtxForTenant(newTenant()), r.ID, 0, ""); err == nil {
		t.Fatal("another tenant cannot list the versions")
	}
}

func planReadPrunedAndPaged(t *testing.T, env PlanRevisionEnv) {
	k := newSolKit(t, env.SolutionEnv)
	r := k.seed(t, domain.RequestTypeChangeRequest, domain.RequestStatusPlanning)
	rd := newPlanReadReaders(env)
	total := domain.PlanRevisionTextKept + 2
	for n := 1; n <= total; n++ {
		addPlanVersion(t, env, k, r, n, "", map[string]string{"tasks.md": fmt.Sprintf("version %d\n", n)})
	}

	var seen []int
	token := ""
	for pages := 0; pages < total; pages++ {
		page, err := rd.list.Execute(k.ctx, r.ID, 5, token)
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range page.Revisions {
			seen = append(seen, v.Revision)
			if want := v.Revision > total-domain.PlanRevisionTextKept; v.TextKept != want || v.FileCount != 1 {
				t.Fatalf("version %d: %+v", v.Revision, v)
			}
		}
		if token = page.NextPageToken; token == "" {
			break
		}
	}
	if len(seen) != total || seen[0] != total || seen[total-1] != 1 {
		t.Fatalf("paging walked %v", seen)
	}

	if _, err := rd.file.Execute(k.ctx, r.ID, "tasks.md", 2); !hasErrorCode(err, "REQUEST_PLAN_REVISION_TEXT_PRUNED") {
		t.Fatalf("version 2 lost its text: %v", err)
	}
	f, err := rd.file.Execute(k.ctx, r.ID, "tasks.md", 3)
	if err != nil || !strings.HasPrefix(f.Content, "version 3") {
		t.Fatalf("version 3 keeps its text: %+v %v", f, err)
	}
	// A pruned version still lists its files and reads its proposal.
	view, err := rd.get.Execute(k.ctx, r.ID, "", 1)
	if err != nil || view.Proposal == nil || view.Bundle == nil || len(view.Bundle.Files) != 1 {
		t.Fatalf("version 1: %+v %v", view, err)
	}
}
