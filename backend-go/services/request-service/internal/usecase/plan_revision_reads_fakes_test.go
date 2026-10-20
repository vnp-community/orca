package usecase

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

const threeTaskBody = twoPhaseBody + "- [ ] T2.2 Thêm kiểm thử [depends=T2.1]\n"

// versionedPlan writes several versions of one authored change through the real worker, the way revisions do in
// production: version 1 has three files, version 2 adds design.md and edits proposal.md, version 3 drops design.md and
// grows tasks.md.
type versionedPlan struct {
	k    *planKit
	w    *RunProposalGeneration
	eng  *scriptedEngine
	base domain.AnalysisRun
	runs []domain.AnalysisRun
}

func newVersionedPlan(t *testing.T) *versionedPlan {
	t.Helper()
	eng := &scriptedEngine{answers: []ProposalResult{answer(tasksDoc(twoPhaseBody)), answer(tasksDoc(twoPhaseBody)), answer(tasksDoc(threeTaskBody))}}
	k, run, w := proposalKit(t, eng, nil)
	eng.afterWrite = func(dir string) {
		switch len(eng.inputs) {
		case 2:
			k.proposal.ws.write(dir+"design.md", "## Context\nv2\n")
			k.proposal.ws.write(dir+"proposal.md", "## Why\nv2\n")
		case 3:
			delete(k.proposal.ws.files, dir+"design.md")
			k.proposal.ws.changed = append(k.proposal.ws.changed, ChangedPath{Path: dir + "tasks.md", State: "modified"})
		}
	}
	v := &versionedPlan{k: k, w: w, eng: eng, base: run}
	k.proposal.revisions.err = nil
	return v
}

// next runs the worker for one more Plan run and returns it as stored.
func (v *versionedPlan) next(t *testing.T, feedback string) domain.AnalysisRun {
	t.Helper()
	n := len(v.runs)
	run := v.base
	if n > 0 {
		run.ID = fmt.Sprintf("run-v%d", n+1)
		run.StartedAt = v.base.StartedAt.Add(time.Duration(n) * time.Minute)
		run.Status, run.IdempotencyKey = domain.RunStatusRunning, nil
		v.k.w.runs[run.ID] = run
	}
	run.Feedback = feedback
	v.k.w.runs[run.ID] = run
	if err := v.w.Execute(v.k.w.ctx(), run); err != nil {
		t.Fatalf("version %d: %v", n+1, err)
	}
	got := v.k.w.runs[run.ID]
	v.runs = append(v.runs, got)
	return got
}

func (v *versionedPlan) requestID() string { return v.base.RequestID }

func (v *versionedPlan) listUC() *ListPlanRevisions {
	return NewListPlanRevisions(v.k.w, fakeRuns{v.k.w}, v.k.proposal.revisions, v.k.proposal.changes)
}

func (v *versionedPlan) getUC() *GetPlanProposal {
	return NewGetPlanProposal(v.k.w, fakeRuns{v.k.w}).WithChanges(v.k.proposal.changes).WithRevisions(v.k.proposal.revisions)
}

func (v *versionedPlan) fileUC() *GetPlanBundleFile {
	return NewGetPlanBundleFile(v.k.w, v.k.proposal.changes).WithRevisions(fakeRuns{v.k.w}, v.k.proposal.revisions)
}

func (v *versionedPlan) ctx() context.Context { return v.k.w.ctx() }

// countingRevisions counts the reads of the version store, to prove a listing is not one query per version.
type countingRevisions struct {
	OpenSpecChangeRevisionRepository
	lists, gets int
}

func (c *countingRevisions) List(ctx context.Context, requestID string, before, limit int) ([]domain.OpenSpecChangeRevision, error) {
	c.lists++
	return c.OpenSpecChangeRevisionRepository.List(ctx, requestID, before, limit)
}

func (c *countingRevisions) Get(ctx context.Context, requestID string, revision int) (domain.OpenSpecChangeRevision, bool, error) {
	c.gets++
	return c.OpenSpecChangeRevisionRepository.Get(ctx, requestID, revision)
}
