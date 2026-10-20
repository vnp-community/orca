package usecase

import (
	"context"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// openspecProposalKit runs the openspec worker to success and returns what a reader would see.
func openspecProposalKit(t *testing.T, report AgentReport) (*planKit, domain.AnalysisRun, *fakeChanges) {
	t.Helper()
	a := answer(tasksDoc(twoPhaseBody))
	a.Report, a.KnowledgeRequested, a.SkillHash = report, report.Knowledge.Available, "tdd-knowledge=1"
	eng := &scriptedEngine{answers: []ProposalResult{a}}
	k, run, w := proposalKit(t, eng, nil)
	if err := w.Execute(k.w.ctx(), run); err != nil {
		t.Fatal(err)
	}
	return k, run, k.proposal.changes
}

func TestGetPlanProposal_OpenSpecRunCarriesTheBundleAndWhatToWarnAbout(t *testing.T) {
	k, run, changes := openspecProposalKit(t, AgentReport{Present: true, Knowledge: KnowledgeReport{Available: true, Snapshot: "snap-1", Read: []string{"FR-1"}, Conflicts: []string{"ADR-12 vs AC-5"}}})
	v, err := NewGetPlanProposal(k.w, fakeRuns{k.w}).WithChanges(changes).Execute(k.w.ctx(), run.RequestID, run.ID, 0)
	if err != nil || v.Engine != "openspec" || v.Proposal == nil {
		t.Fatalf("%+v %v", v, err)
	}
	if v.Bundle == nil || v.Bundle.Branch != "request/42-proposal" || v.Bundle.Commit != "sha-1" || !domain.ValidChangeID(v.Bundle.ChangeID) || len(v.Bundle.Files) != 3 {
		t.Fatalf("bundle: %+v", v.Bundle)
	}
	if v.Bundle.Files[0].Path != "proposal.md" || v.Bundle.Files[1].Path != "specs/ai/spec.md" || v.Bundle.Files[2].Path != "tasks.md" {
		t.Fatalf("files are listed in path order: %+v", v.Bundle.Files)
	}
	if v.Knowledge == nil || !v.Knowledge.Available || v.Knowledge.Snapshot != "snap-1" || len(v.Knowledge.Read) != 1 {
		t.Fatalf("knowledge: %+v", v.Knowledge)
	}
	codes := map[string]string{}
	for _, c := range v.Checks {
		codes[c.Code] = c.Severity
	}
	if codes["STRUCTURE_OK"] != "info" || codes["KNOWLEDGE_CONFLICT"] != "warning" || codes["KNOWLEDGE_UNAVAILABLE"] != "" {
		t.Fatalf("checks: %+v", v.Checks)
	}
}

func TestGetPlanProposal_WarnsWhenTheAgentHadNoKnowledgeOrCitedNothing(t *testing.T) {
	k, run, changes := openspecProposalKit(t, AgentReport{Present: false})
	v, _ := NewGetPlanProposal(k.w, fakeRuns{k.w}).WithChanges(changes).Execute(k.w.ctx(), run.RequestID, run.ID, 0)
	got := map[string]bool{}
	for _, c := range v.Checks {
		got[c.Code] = true
	}
	if !got["KNOWLEDGE_UNAVAILABLE"] {
		t.Fatalf("an agent that worked without the graph must be called out: %+v", v.Checks)
	}
	k2, run2, changes2 := openspecProposalKit(t, AgentReport{Present: true, Knowledge: KnowledgeReport{Available: true}})
	v2, _ := NewGetPlanProposal(k2.w, fakeRuns{k2.w}).WithChanges(changes2).Execute(k2.w.ctx(), run2.RequestID, run2.ID, 0)
	got = map[string]bool{}
	for _, c := range v2.Checks {
		got[c.Code] = true
	}
	if !got["KNOWLEDGE_NOT_CITED"] {
		t.Fatalf("available but uncited graph: %+v", v2.Checks)
	}
}

func TestGetPlanProposal_NativeRunHasNoBundleAndNoEmptyFields(t *testing.T) {
	k, req, run, _, worker := planRunKit(t, domain.RequestTypeTask, domain.RequestSizeS, []string{planJSON(false)})
	if err := worker.Execute(k.w.ctx(), run); err != nil {
		t.Fatal(err)
	}
	v, err := NewGetPlanProposal(k.w, fakeRuns{k.w}).WithChanges(newFakeChanges()).Execute(k.w.ctx(), req.ID, run.ID, 0)
	if err != nil || v.Engine != "native" || v.Bundle != nil || v.Knowledge != nil || len(v.Checks) != 0 {
		t.Fatalf("a native Plan has no authored change: %+v %v", v, err)
	}
}

func TestGetPlanBundleFile_ReadsFromTheStoredCopyAndRefusesEverythingElse(t *testing.T) {
	k, run, changes := openspecProposalKit(t, AgentReport{Present: true})
	change, _, _ := changes.GetByRequest(context.Background(), run.RequestID)
	get := NewGetPlanBundleFile(k.w, changes)

	for _, p := range []string{"tasks.md", "openspec/changes/" + change.ChangeID + "/tasks.md", "./tasks.md", `specs\ai\spec.md`} {
		f, err := get.Execute(k.w.ctx(), run.RequestID, p, 0)
		if err != nil || f.Content == "" || f.Omitted {
			t.Fatalf("%q: %+v %v", p, f, err)
		}
	}
	f, _ := get.Execute(k.w.ctx(), run.RequestID, "openspec/changes/"+change.ChangeID+"/specs/ai/spec.md", 0)
	if f.Path != "specs/ai/spec.md" {
		t.Fatalf("the answer names the file relative to the change: %q", f.Path)
	}

	for _, bad := range []string{"", ".", "..", "../x", "a/../../x", "/etc/passwd", "openspec/specs/x.md", "openspec/changes/other/tasks.md/../../../../etc/passwd", "tasks.md\x00", strings.Repeat("a", 600)} {
		if _, err := get.Execute(k.w.ctx(), run.RequestID, bad, 0); errCode(err) != "REQUEST_PLAN_BUNDLE_PATH_INVALID" && errCode(err) != "REQUEST_PLAN_BUNDLE_FILE_NOT_FOUND" {
			t.Fatalf("%q must be refused, got %v", bad, err)
		}
	}
	if _, err := get.Execute(k.w.ctx(), run.RequestID, "design.md", 0); errCode(err) != "REQUEST_PLAN_BUNDLE_FILE_NOT_FOUND" {
		t.Fatalf("a file the change does not have: %v", err)
	}
	// Another change's directory is not this change's directory, even when the name starts the same.
	if _, err := get.Execute(k.w.ctx(), run.RequestID, "openspec/changes/"+change.ChangeID+"-evil/tasks.md", 0); err == nil {
		t.Fatal("a sibling directory must not match by prefix")
	}
}

func TestGetPlanBundleFile_NoChangeNoAnswer(t *testing.T) {
	w := newSolWorld()
	req := w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusPlanning)
	_, err := NewGetPlanBundleFile(w, newFakeChanges()).Execute(w.ctx(), req.ID, "tasks.md", 0)
	requireCode(t, err, "REQUEST_PLAN_BUNDLE_NOT_FOUND")
	if _, err := NewGetPlanBundleFile(w, newFakeChanges()).Execute(w.ctx(), "missing", "tasks.md", 0); err == nil {
		t.Fatal("an unknown Request must fail")
	}
	if _, err := NewGetPlanBundleFile(w, newFakeChanges()).Execute(context.Background(), req.ID, "tasks.md", 0); err == nil {
		t.Fatal("no tenant, no answer")
	}
}

func TestGetPlanBundleFile_AnOmittedFileSaysSo(t *testing.T) {
	w := newSolWorld()
	req := w.seedRequest(domain.RequestTypeChangeRequest, domain.RequestStatusPlanning)
	changes := newFakeChanges()
	changes.Upsert(context.Background(), domain.OpenSpecChange{ID: "c", RequestID: req.ID, ChangeID: "req-1-x", Bundle: map[string]string{"big.md": "<omitted size=9999999 sha256=abc>"}})
	f, err := NewGetPlanBundleFile(w, changes).Execute(w.ctx(), req.ID, "big.md", 0)
	if err != nil || !f.Omitted || !strings.Contains(f.Content, "size=9999999") {
		t.Fatalf("%+v %v", f, err)
	}
}

func TestGetPlanProposal_BundleSaysHowTheSyncAndTheArchiveAreDoing(t *testing.T) {
	k, run, changes := openspecProposalKit(t, AgentReport{})
	c, _, _ := changes.GetByRequest(k.w.ctx(), run.RequestID)
	c.Status, c.SyncState, c.SyncError, c.SyncAttempts = domain.ChangeStatusReady, domain.SyncStateFailed, "openspec archive exited with code 1", domain.MaxSyncAttempts
	if _, err := changes.Upsert(k.w.ctx(), c); err != nil {
		t.Fatal(err)
	}
	v, err := NewGetPlanProposal(k.w, fakeRuns{k.w}).WithChanges(changes).Execute(k.w.ctx(), run.RequestID, run.ID, 0)
	if err != nil || v.Bundle == nil || v.Bundle.Status != "ready" || v.Bundle.SyncState != "failed" || v.Bundle.SyncAttempts != domain.MaxSyncAttempts ||
		!strings.Contains(v.Bundle.SyncError, "exited with code 1") {
		t.Fatalf("%+v %v", v.Bundle, err)
	}
}
