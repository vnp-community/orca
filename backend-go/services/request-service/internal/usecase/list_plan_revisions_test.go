package usecase

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func TestListPlanRevisions_NoRunsIsAnEmptyPage(t *testing.T) {
	k := newPlanKit(nil)
	req := k.w.seedRequest(domain.RequestTypeTask, domain.RequestStatusPlanning)
	page, err := NewListPlanRevisions(k.w, fakeRuns{k.w}, nil, nil).Execute(k.w.ctx(), req.ID, 0, "")
	if err != nil || len(page.Revisions) != 0 || page.NextPageToken != "" {
		t.Fatalf("page = %+v err = %v", page, err)
	}
}

func TestListPlanRevisions_NeedsATenantAndAKnownRequest(t *testing.T) {
	k := newPlanKit(nil)
	uc := NewListPlanRevisions(k.w, fakeRuns{k.w}, nil, nil)
	if _, err := uc.Execute(context.Background(), "x", 0, ""); !errorHasCode(err, "REQUEST_TENANT_REQUIRED") {
		t.Fatalf("no tenant: %v", err)
	}
	if _, err := uc.Execute(k.w.ctx(), uuid.NewString(), 0, ""); err == nil {
		t.Fatal("an unknown Request must not list")
	}
}

// nativeVersions seeds succeeded native Plan runs, oldest first; feedbacks[i] is the ask of version i+1.
func nativeVersions(k *planKit, reqID string, feedbacks ...string) []domain.AnalysisRun {
	base := time.Now().UTC().Add(-time.Hour)
	var out []domain.AnalysisRun
	for i, fb := range feedbacks {
		finished := base.Add(time.Duration(i)*time.Minute + 30*time.Second)
		run := domain.AnalysisRun{
			ID: fmt.Sprintf("native-%d", i+1), TenantID: "t1", RequestID: reqID, Kind: domain.RunKindPlan, Mode: domain.AnalysisModeComplete,
			Status: domain.RunStatusSucceeded, ActorID: fmt.Sprintf("u-%d", i+1), Feedback: fb, StartedAt: base.Add(time.Duration(i) * time.Minute), FinishedAt: &finished,
		}
		k.w.runs[run.ID] = run
		out = append(out, run)
	}
	return out
}

func TestListPlanRevisions_NativeVersionsNewestFirstWithTheirAsk(t *testing.T) {
	k := newPlanKit(nil)
	req := k.w.seedRequest(domain.RequestTypeTask, domain.RequestStatusAwaitingPlanApproval)
	runs := nativeVersions(k, req.ID, "", "split the migration", domain.ClarificationFeedbackPrefix+"3", "")
	page, err := NewListPlanRevisions(k.w, fakeRuns{k.w}, nil, nil).Execute(k.w.ctx(), req.ID, 0, "")
	if err != nil || len(page.Revisions) != 4 || page.NextPageToken != "" {
		t.Fatalf("page = %+v err = %v", page, err)
	}
	for i, want := range []int{4, 3, 2, 1} {
		if page.Revisions[i].Revision != want {
			t.Fatalf("order = %+v", page.Revisions)
		}
	}
	cur, second := page.Revisions[0], page.Revisions[2]
	if cur.Status != "current" || cur.RunID != runs[3].ID || cur.Engine != "native" || cur.ActorID != "u-4" || !cur.TextKept || cur.FileCount != 0 || cur.Commit != "" {
		t.Fatalf("current = %+v", cur)
	}
	if second.Status != "superseded" || second.RevisionFeedback != "split the migration" || !second.GeneratedAt.Equal(*runs[1].FinishedAt) {
		t.Fatalf("second = %+v", second)
	}
	if page.Revisions[1].RevisionFeedback != "" {
		t.Fatalf("the answer to a clarification is not a request for changes and must not show: %q", page.Revisions[1].RevisionFeedback)
	}
	if page.Revisions[3].RevisionFeedback != "" {
		t.Fatal("the first Plan has no ask")
	}
}

func TestListPlanRevisions_RunsThatDidNotSucceedAreNotVersions(t *testing.T) {
	k := newPlanKit(nil)
	req := k.w.seedRequest(domain.RequestTypeTask, domain.RequestStatusPlanning)
	nativeVersions(k, req.ID, "", "x")
	k.w.runs["failed"] = domain.AnalysisRun{ID: "failed", RequestID: req.ID, Kind: domain.RunKindPlan, Status: domain.RunStatusFailed, StartedAt: time.Now()}
	k.w.runs["solution"] = domain.AnalysisRun{ID: "solution", RequestID: req.ID, Kind: domain.RunKindSolution, Status: domain.RunStatusSucceeded, StartedAt: time.Now()}
	page, _ := NewListPlanRevisions(k.w, fakeRuns{k.w}, nil, nil).Execute(k.w.ctx(), req.ID, 0, "")
	if len(page.Revisions) != 2 {
		t.Fatalf("versions = %+v", page.Revisions)
	}
}

func TestListPlanRevisions_PagesWalkTheWholeHistory(t *testing.T) {
	k := newPlanKit(nil)
	req := k.w.seedRequest(domain.RequestTypeTask, domain.RequestStatusAwaitingPlanApproval)
	nativeVersions(k, req.ID, "", "a", "b", "c", "d")
	uc := NewListPlanRevisions(k.w, fakeRuns{k.w}, nil, nil)
	var seen []int
	token := ""
	for pages := 0; pages < 5; pages++ {
		page, err := uc.Execute(k.w.ctx(), req.ID, 2, token)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range page.Revisions {
			seen = append(seen, r.Revision)
		}
		if token = page.NextPageToken; token == "" {
			break
		}
	}
	if fmt.Sprint(seen) != "[5 4 3 2 1]" {
		t.Fatalf("walked %v", seen)
	}
}

func TestListPlanRevisions_PageSizeIsBoundedAndATokenMustBeOurs(t *testing.T) {
	k := newPlanKit(nil)
	req := k.w.seedRequest(domain.RequestTypeTask, domain.RequestStatusPlanning)
	feedbacks := make([]string, 120)
	nativeVersions(k, req.ID, feedbacks...)
	uc := NewListPlanRevisions(k.w, fakeRuns{k.w}, nil, nil)
	if page, _ := uc.Execute(k.w.ctx(), req.ID, 0, ""); len(page.Revisions) != 20 {
		t.Fatalf("default page = %d", len(page.Revisions))
	}
	if page, _ := uc.Execute(k.w.ctx(), req.ID, 1000, ""); len(page.Revisions) != 100 || page.NextPageToken != "21" {
		t.Fatalf("a page is at most 100: %d token %q", len(page.Revisions), page.NextPageToken)
	}
	for _, bad := range []string{"abc", "0", "-3"} {
		if _, err := uc.Execute(k.w.ctx(), req.ID, 10, bad); !errorHasCode(err, "REQUEST_PLAN_REVISION_PAGE_TOKEN_INVALID") {
			t.Errorf("token %q: %v", bad, err)
		}
	}
	if page, err := uc.Execute(k.w.ctx(), req.ID, 10, "9999"); err != nil || len(page.Revisions) != 10 || page.Revisions[0].Revision != 120 {
		t.Fatalf("a token past the end starts from the newest: %+v %v", page.Revisions, err)
	}
}

func TestListPlanRevisions_OpenSpecVersionsCountTheFilesThatChanged(t *testing.T) {
	v := newVersionedPlan(t)
	v.next(t, "")
	v.next(t, "add a design")
	v.next(t, "drop the design and add a test task")

	counting := &countingRevisions{OpenSpecChangeRevisionRepository: v.k.proposal.revisions}
	uc := NewListPlanRevisions(v.k.w, fakeRuns{v.k.w}, counting, v.k.proposal.changes)
	page, err := uc.Execute(v.ctx(), v.requestID(), 0, "")
	if err != nil || len(page.Revisions) != 3 {
		t.Fatalf("page = %+v err = %v", page, err)
	}
	if counting.lists != 1 || counting.gets != 0 {
		t.Fatalf("a listing is one read of the version store, got %d list(s) and %d get(s)", counting.lists, counting.gets)
	}
	v3, v2, v1 := page.Revisions[0], page.Revisions[1], page.Revisions[2]
	if v1.FileCount != 3 || v1.FilesChanged != 3 || v1.Commit != "sha-1" || !v1.TextKept || v1.Engine != "openspec" {
		t.Fatalf("v1 = %+v", v1)
	}
	// v2 adds design.md and edits proposal.md.
	if v2.FileCount != 4 || v2.FilesChanged != 2 || v2.RevisionFeedback != "add a design" {
		t.Fatalf("v2 = %+v", v2)
	}
	// v3 removes design.md, edits tasks.md and, because the scripted agent rewrites it, puts proposal.md back.
	if v3.FileCount != 3 || v3.FilesChanged != 3 || v3.Status != "current" {
		t.Fatalf("v3 = %+v", v3)
	}
}

func TestListPlanRevisions_AVersionWhoseTextWasDroppedStillListsItsFiles(t *testing.T) {
	v := newVersionedPlan(t)
	v.next(t, "")
	v.next(t, "x")
	rows := v.k.proposal.revisions
	rows.mu.Lock()
	rows.rows[0].Bundle, rows.rows[0].TextKept = nil, false
	rows.mu.Unlock()
	page, _ := v.listUC().Execute(v.ctx(), v.requestID(), 0, "")
	if old := page.Revisions[1]; old.TextKept || old.FileCount != 3 || old.Commit == "" {
		t.Fatalf("old = %+v", old)
	}
	if !page.Revisions[0].TextKept {
		t.Fatal("the newest versions keep their text")
	}
}

func TestListPlanRevisions_ARequestThatPredatesTheHistoryListsItsCurrentVersion(t *testing.T) {
	v := newVersionedPlan(t)
	first := v.next(t, "")
	second := v.next(t, "x")
	// Forget the history: the table did not exist when these Plans were written, only the change row did.
	v.k.proposal.revisions.mu.Lock()
	v.k.proposal.revisions.rows = nil
	v.k.proposal.revisions.mu.Unlock()
	page, err := v.listUC().Execute(v.ctx(), v.requestID(), 0, "")
	if err != nil || len(page.Revisions) != 2 {
		t.Fatalf("page = %+v err = %v", page, err)
	}
	cur, old := page.Revisions[0], page.Revisions[1]
	if cur.RunID != second.ID || !cur.TextKept || cur.FileCount != 4 || cur.Commit != "sha-1" || cur.FilesChanged != cur.FileCount {
		t.Fatalf("current = %+v", cur)
	}
	if old.RunID != first.ID || old.TextKept || old.FileCount != 0 || old.Commit != "" {
		t.Fatalf("an older version of such a Request has no documents: %+v", old)
	}
}

func TestListPlanRevisions_StoreFailuresSurface(t *testing.T) {
	v := newVersionedPlan(t)
	v.next(t, "")
	bad := &failingRevisionList{err: errRevisionStore}
	_, err := NewListPlanRevisions(v.k.w, fakeRuns{v.k.w}, bad, v.k.proposal.changes).Execute(v.ctx(), v.requestID(), 0, "")
	if err != errRevisionStore {
		t.Fatalf("err = %v", err)
	}
}

type failingRevisionList struct {
	OpenSpecChangeRevisionRepository
	err error
}

func (f *failingRevisionList) List(context.Context, string, int, int) ([]domain.OpenSpecChangeRevision, error) {
	return nil, f.err
}
