package usecase

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func changeOf(b *PlanBundleView, path string) string {
	for _, f := range b.Files {
		if f.Path == path {
			return f.Change
		}
	}
	return "<absent>"
}

func TestGetPlanProposal_Revision_OlderVersionOfAnAuthoredChange(t *testing.T) {
	v := newVersionedPlan(t)
	runs := []domain.AnalysisRun{v.next(t, ""), v.next(t, "add a design"), v.next(t, "drop it")}

	view, err := v.getUC().Execute(v.ctx(), v.requestID(), "", 2)
	if err != nil || view.RunID != runs[1].ID || view.Status != "succeeded" || view.Proposal == nil || view.Bundle == nil {
		t.Fatalf("view = %+v err = %v", view, err)
	}
	if view.Checks != nil || view.Knowledge != nil {
		t.Fatalf("checks and knowledge belong to the current version only: %+v %+v", view.Checks, view.Knowledge)
	}
	if got := changeOf(view.Bundle, "design.md"); got != "added" {
		t.Fatalf("design.md in v2 = %q", got)
	}
	if got := changeOf(view.Bundle, "proposal.md"); got != "changed" {
		t.Fatalf("proposal.md in v2 = %q", got)
	}
	if got := changeOf(view.Bundle, "tasks.md"); got != "unchanged" {
		t.Fatalf("tasks.md in v2 = %q", got)
	}
	for _, f := range view.Bundle.Files {
		if len(f.SHA256) != 64 {
			t.Fatalf("every file carries its hash: %+v", f)
		}
	}
	if view.Bundle.Commit != "sha-1" || view.Bundle.ChangeID == "" {
		t.Fatalf("bundle = %+v", view.Bundle)
	}

	first, err := v.getUC().Execute(v.ctx(), v.requestID(), "", 1)
	if err != nil || len(first.Bundle.Files) != 3 {
		t.Fatalf("v1 = %+v err = %v", first.Bundle, err)
	}
	for _, f := range first.Bundle.Files {
		if f.Change != "" {
			t.Fatalf("the first version has nothing to be compared with: %+v", f)
		}
	}
}

func TestGetPlanProposal_Revision_TheCurrentVersionKeepsChecksAndListsRemovedFiles(t *testing.T) {
	v := newVersionedPlan(t)
	v.next(t, "")
	v.next(t, "add a design")
	v.next(t, "drop it")

	view, err := v.getUC().Execute(v.ctx(), v.requestID(), "", 3)
	if err != nil || view.Checks == nil || view.Knowledge == nil {
		t.Fatalf("current version: %+v err = %v", view, err)
	}
	if got := changeOf(view.Bundle, "design.md"); got != "removed" {
		t.Fatalf("design.md in v3 = %q", got)
	}
	if got := changeOf(view.Bundle, "tasks.md"); got != "changed" {
		t.Fatalf("tasks.md in v3 = %q", got)
	}
	// Reading the current version without a number says the same thing.
	same, err := v.getUC().Execute(v.ctx(), v.requestID(), "", 0)
	if err != nil || changeOf(same.Bundle, "design.md") != "removed" || changeOf(same.Bundle, "tasks.md") != "changed" || len(same.Bundle.Files) != len(view.Bundle.Files) {
		t.Fatalf("revision 0 = %+v err = %v", same.Bundle, err)
	}
}

func TestGetPlanProposal_Revision_NotFound(t *testing.T) {
	v := newVersionedPlan(t)
	v.next(t, "")
	for _, n := range []int{2, 9, -1} {
		if _, err := v.getUC().Execute(v.ctx(), v.requestID(), "", n); !errorHasCode(err, "REQUEST_PLAN_REVISION_NOT_FOUND") {
			t.Errorf("revision %d: %v", n, err)
		}
	}
	if _, err := v.getUC().Execute(v.ctx(), uuid.NewString(), "", 1); err == nil {
		t.Fatal("another Request has no such version")
	}
}

func TestGetPlanProposal_Revision_NativeProposalOfTheRightRunWhileTheNewestRunFailed(t *testing.T) {
	k := newPlanKit(nil)
	req := k.w.seedRequest(domain.RequestTypeTask, domain.RequestStatusPlanning)
	runs := nativeVersions(k, req.ID, "", "ask")
	for i, title := range []string{"Plan one", "Plan two"} {
		raw, err := EncodePlanProposal(domain.PlanProposal{Title: title, Summary: "s", Tasks: []domain.TaskProposal{{Title: "t", TaskType: "task"}}})
		if err != nil {
			t.Fatal(err)
		}
		r := runs[i]
		r.RawOutput = &raw
		k.w.runs[r.ID] = r
	}
	code := "REQUEST_PLAN_AI_INVALID_JSON"
	k.w.runs["failed"] = domain.AnalysisRun{ID: "failed", RequestID: req.ID, Kind: domain.RunKindPlan, Status: domain.RunStatusFailed, ErrorCode: &code, StartedAt: time.Now()}
	get := NewGetPlanProposal(k.w, fakeRuns{k.w})

	latest, err := get.Execute(k.w.ctx(), req.ID, "", 0)
	if err != nil || latest.Status != "failed" || latest.Proposal != nil {
		t.Fatalf("the newest run is the failed one: %+v %v", latest, err)
	}
	for n, title := range map[int]string{1: "Plan one", 2: "Plan two"} {
		view, err := get.Execute(k.w.ctx(), req.ID, "", n)
		if err != nil || view.Proposal == nil || view.Proposal.Title != title || view.Engine != "native" || view.Bundle != nil {
			t.Fatalf("revision %d: %+v err = %v", n, view, err)
		}
	}
}

func TestGetPlanProposal_Revision_AnOlderVersionOfARequestThatPredatesTheHistoryHasNoFiles(t *testing.T) {
	v := newVersionedPlan(t)
	v.next(t, "")
	v.next(t, "x")
	v.k.proposal.revisions.mu.Lock()
	v.k.proposal.revisions.rows = nil
	v.k.proposal.revisions.mu.Unlock()

	old, err := v.getUC().Execute(v.ctx(), v.requestID(), "", 1)
	if err != nil || old.Proposal == nil || old.Bundle != nil {
		t.Fatalf("the proposal still reads, the documents are gone: %+v %v", old, err)
	}
	cur, err := v.getUC().Execute(v.ctx(), v.requestID(), "", 2)
	if err != nil || cur.Bundle == nil || len(cur.Bundle.Files) != 4 || cur.Checks == nil {
		t.Fatalf("the newest version is still the change row: %+v %v", cur.Bundle, err)
	}
}

func TestGetPlanBundleFile_Revision_OlderCurrentPrunedAndMissing(t *testing.T) {
	v := newVersionedPlan(t)
	v.next(t, "")
	v.next(t, "add a design")
	v.next(t, "drop it")
	file := v.fileUC()

	old, err := file.Execute(v.ctx(), v.requestID(), "design.md", 2)
	if err != nil || !strings.Contains(old.Content, "v2") || old.Omitted {
		t.Fatalf("v2 design.md = %+v err = %v", old, err)
	}
	if _, err := file.Execute(v.ctx(), v.requestID(), "design.md", 3); !errorHasCode(err, "REQUEST_PLAN_BUNDLE_FILE_NOT_FOUND") {
		t.Fatalf("v3 has no design.md: %v", err)
	}
	cur, err := file.Execute(v.ctx(), v.requestID(), "tasks.md", 3)
	currentRead, _ := file.Execute(v.ctx(), v.requestID(), "tasks.md", 0)
	if err != nil || cur.Content != currentRead.Content || !strings.Contains(cur.Content, "T2.2") {
		t.Fatalf("the newest version reads the current change: %+v err = %v", cur, err)
	}
	if first, err := file.Execute(v.ctx(), v.requestID(), "tasks.md", 1); err != nil || strings.Contains(first.Content, "T2.2") {
		t.Fatalf("v1 tasks.md = %+v err = %v", first, err)
	}

	rows := v.k.proposal.revisions
	rows.mu.Lock()
	rows.rows[0].Bundle, rows.rows[0].TextKept = nil, false
	rows.mu.Unlock()
	if _, err := file.Execute(v.ctx(), v.requestID(), "tasks.md", 1); !errorHasCode(err, "REQUEST_PLAN_REVISION_TEXT_PRUNED") {
		t.Fatalf("pruned: %v", err)
	}
	for _, n := range []int{4, -2} {
		if _, err := file.Execute(v.ctx(), v.requestID(), "tasks.md", n); !errorHasCode(err, "REQUEST_PLAN_REVISION_NOT_FOUND") {
			t.Errorf("revision %d: %v", n, err)
		}
	}
}

func TestGetPlanBundleFile_Revision_PathsStillMustStayInsideTheChange(t *testing.T) {
	v := newVersionedPlan(t)
	v.next(t, "")
	v.next(t, "x")
	change, _, _ := v.k.proposal.changes.GetByRequest(v.ctx(), v.requestID())
	for _, bad := range []string{"../../etc/passwd", "/etc/passwd", "openspec/changes/" + change.ChangeID + "-evil/tasks.md", ""} {
		_, err := v.fileUC().Execute(v.ctx(), v.requestID(), bad, 1)
		if !errorHasCode(err, "REQUEST_PLAN_BUNDLE_PATH_INVALID") && !errorHasCode(err, "REQUEST_PLAN_BUNDLE_FILE_NOT_FOUND") {
			t.Errorf("%q: %v", bad, err)
		}
	}
}

func TestGetPlanBundleFile_Revision_AVersionOfARequestThatPredatesTheHistoryAnswersPruned(t *testing.T) {
	v := newVersionedPlan(t)
	v.next(t, "")
	v.next(t, "x")
	v.k.proposal.revisions.mu.Lock()
	v.k.proposal.revisions.rows = nil
	v.k.proposal.revisions.mu.Unlock()
	if _, err := v.fileUC().Execute(v.ctx(), v.requestID(), "tasks.md", 1); !errorHasCode(err, "REQUEST_PLAN_REVISION_TEXT_PRUNED") {
		t.Fatalf("v1: %v", err)
	}
	if f, err := v.fileUC().Execute(v.ctx(), v.requestID(), "tasks.md", 2); err != nil || f.Content == "" {
		t.Fatalf("v2 is the current change: %+v %v", f, err)
	}
}

func TestGetPlanBundleFile_Revision_WithoutTheHistoryOnlyTheCurrentVersionIsKnown(t *testing.T) {
	v := newVersionedPlan(t)
	v.next(t, "")
	plain := NewGetPlanBundleFile(v.k.w, v.k.proposal.changes)
	if _, err := plain.Execute(v.ctx(), v.requestID(), "tasks.md", 1); !errorHasCode(err, "REQUEST_PLAN_REVISION_NOT_FOUND") {
		t.Fatalf("a server without the history cannot read a numbered version: %v", err)
	}
	if f, err := plain.Execute(v.ctx(), v.requestID(), "tasks.md", 0); err != nil || f.Content == "" {
		t.Fatalf("the current read is unchanged: %+v %v", f, err)
	}
}
