package usecase

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// PlanProposalView is the state of one Plan generation run as the caller reads it.
type PlanProposalView struct {
	RunID string
	// Status is running, succeeded or failed.
	Status       string
	ErrorCode    string
	ErrorMessage string
	Proposal     *domain.PlanProposal
	// Raw is the model's text when the run failed, so a person can see what it said.
	Raw string
	// Engine is native or openspec. Bundle, Checks and Knowledge are set for openspec runs only.
	Engine    string
	Bundle    *PlanBundleView
	Checks    []PlanCheckView
	Knowledge *PlanKnowledgeView
	// Stats is what the run's agent calls reported (tool call counts and usage); nil when none did. ElapsedMS is the run's
	// wall-clock time from start to finish, 0 while it runs.
	Stats     *domain.RunStats
	ElapsedMS int64
	// AutoPlan is the automatic generation of the Plan (CR-REQ-044); nil when the server was started without it.
	AutoPlan *PlanAutoView
}

// PlanBundleView names the authored OpenSpec change behind a Plan.
type PlanBundleView struct {
	ChangeID, Branch, Commit, TasksMDDigest string
	// Status is the change's own (preparing, ready, archived, abandoned); SyncState, SyncError and SyncAttempts say how the
	// tasks.md sync and the archive step are doing.
	Status, SyncState, SyncError string
	SyncAttempts                 int
	Files                        []PlanBundleFileView
}

type PlanBundleFileView struct {
	Path    string
	Size    int64
	Omitted bool
	// SHA256 is the hash of the stored text (or, for an omitted file, of the file); Change compares it with the version
	// before: added, changed, unchanged or removed, and empty when there is nothing to compare with.
	SHA256 string
	Change string
}

// PlanCheckView is one finding for the approver: warning or info (a refused change is never stored).
type PlanCheckView struct {
	Code, Where, Message, Severity string
}

// PlanKnowledgeView is what the authoring agent reported about the design knowledge; the agent's word, not a verified fact.
type PlanKnowledgeView struct {
	Available             bool
	Snapshot              string
	Read, Gaps, Conflicts []string
	SkillHash             string
}

// GetPlanProposal reads the result of a GeneratePlan run; with no run id it answers for the newest run of the Request.
type GetPlanProposal struct {
	requests RequestRepository
	runs     AnalysisRunStore
	changes  OpenSpecChangeRepository
	auto     *planAutoReader
	// revisions holds the stored versions of the authored change (CR-REQ-045); nil means only the current one is known.
	revisions OpenSpecChangeRevisionRepository
}

func NewGetPlanProposal(requests RequestRepository, runs AnalysisRunStore) *GetPlanProposal {
	return &GetPlanProposal{requests: requests, runs: runs}
}

// WithChanges lets the view carry the authored OpenSpec change of an openspec run.
func (uc *GetPlanProposal) WithChanges(c OpenSpecChangeRepository) *GetPlanProposal {
	uc.changes = c
	return uc
}

// WithRevisions lets the view read an older version of the authored change.
func (uc *GetPlanProposal) WithRevisions(r OpenSpecChangeRevisionRepository) *GetPlanProposal {
	uc.revisions = r
	return uc
}

// Execute reads a run. A positive revision reads that version of the Plan instead (the run behind it): its proposal, and
// for an authored change the files of that version; checks and knowledge belong to the current version only.
func (uc *GetPlanProposal) Execute(ctx context.Context, requestID, runID string, revision int) (PlanProposalView, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return PlanProposalView{}, domain.ErrRequestTenantRequired()
	}
	req, err := uc.requests.Get(ctx, requestID)
	if err != nil {
		return PlanProposalView{}, err
	}
	auto, err := uc.auto.read(ctx, req)
	if err != nil {
		return PlanProposalView{}, err
	}
	if revision != 0 {
		return uc.executeRevision(ctx, req, auto, revision)
	}
	run, err := uc.find(ctx, req.ID, runID)
	if err != nil {
		if runID == "" && auto != nil && errorHasCode(err, "REQUEST_PLAN_RUN_NOT_FOUND") {
			// No run yet: a job may be waiting to start one (busy project, between two attempts), or none exists yet; either way a
			// screen needs autoPlan.enabled to choose between "Orca is starting the plan" and the Generate button.
			return PlanProposalView{AutoPlan: auto}, nil
		}
		return PlanProposalView{}, err
	}
	view := PlanProposalView{RunID: run.ID, Status: string(run.Status), Engine: string(run.Mode.EngineFor()), Stats: run.Stats, AutoPlan: auto}
	if run.FinishedAt != nil {
		view.ElapsedMS = run.FinishedAt.Sub(run.StartedAt).Milliseconds()
	}
	switch run.Status {
	case domain.RunStatusFailed:
		view.ErrorCode, view.ErrorMessage = derefStr(run.ErrorCode), derefStr(run.ErrorMessage)
		view.Raw = derefStr(run.RawOutput)
	case domain.RunStatusSucceeded:
		p, err := DecodePlanProposal(derefStr(run.RawOutput))
		if err != nil {
			return PlanProposalView{}, err
		}
		view.Proposal = &p
		if run.Mode == domain.AnalysisModeAgentProposal && uc.changes != nil {
			if err := uc.addChange(ctx, &view, req.ID); err != nil {
				return PlanProposalView{}, err
			}
		}
	}
	return view, nil
}

// addChange attaches the authored change the proposal was read from, and what the approver should be warned about. The
// files carry their hash and what changed since the version before, when the stored versions say so.
func (uc *GetPlanProposal) addChange(ctx context.Context, view *PlanProposalView, requestID string) error {
	c, found, err := uc.changes.GetByRequest(ctx, requestID)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	files := domain.NewRevisionFiles(c.Bundle)
	prev, hasPrev, err := uc.versionBeforeNewest(ctx, requestID)
	if err != nil {
		return err
	}
	view.Bundle = bundleViewOf(c, c.Commit, c.TasksMDDigest, files, prev, hasPrev)
	view.Knowledge = &PlanKnowledgeView{Available: c.KGAvailable, Snapshot: c.KGSnapshot, Read: c.KGReadIDs, Gaps: c.KGGaps, Conflicts: c.KGConflicts, SkillHash: c.SkillHash}
	view.Checks = planChecksFor(c)
	return nil
}

// versionBeforeNewest is the file index of the version the current change replaced, when the Request has one stored.
func (uc *GetPlanProposal) versionBeforeNewest(ctx context.Context, requestID string) ([]domain.RevisionFile, bool, error) {
	if uc.revisions == nil {
		return nil, false, nil
	}
	newest, err := uc.revisions.List(ctx, requestID, 0, 2)
	if err != nil || len(newest) < 2 {
		return nil, false, err
	}
	return newest[1].Files, true, nil
}

// bundleViewOf builds the file list of one version; the change row supplies what does not differ between versions (id,
// branch, sync state). Files the version before had and this one lacks are listed as removed, so a reader sees them go.
func bundleViewOf(c domain.OpenSpecChange, commit, digest string, files, prev []domain.RevisionFile, hasPrev bool) *PlanBundleView {
	b := &PlanBundleView{ChangeID: c.ChangeID, Branch: c.Branch, Commit: commit, TasksMDDigest: digest,
		Status: string(c.Status), SyncState: string(c.SyncState), SyncError: c.SyncError, SyncAttempts: c.SyncAttempts}
	kinds := map[string]domain.FileChangeKind{}
	var removed []domain.FileChange
	if hasPrev {
		for _, d := range domain.DiffFileIndex(prev, files) {
			kinds[d.Path] = d.Kind
			if d.Kind == domain.FileRemoved {
				removed = append(removed, d)
			}
		}
	}
	for _, f := range files {
		b.Files = append(b.Files, PlanBundleFileView{Path: f.Path, Size: f.Size, Omitted: f.Omitted, SHA256: f.SHA256, Change: string(kinds[f.Path])})
	}
	for _, d := range removed {
		b.Files = append(b.Files, PlanBundleFileView{Path: d.Path, Size: d.Size, SHA256: d.SHA256, Change: string(domain.FileRemoved)})
	}
	return b
}

// planChecksFor lists what an approver should know before approving. Orca stores a change only after it passed every
// structure check, so the first entry says that; the rest are warnings about knowledge, which is advice, never a gate.
func planChecksFor(c domain.OpenSpecChange) []PlanCheckView {
	out := []PlanCheckView{{Code: "STRUCTURE_OK", Message: "the change passed the structure checks and openspec validate --strict", Severity: "info"}}
	if !c.KGAvailable {
		out = append(out, PlanCheckView{Code: "KNOWLEDGE_UNAVAILABLE", Message: "the agent worked without the project's design knowledge; the requirements, rules and decisions it cites were not looked up", Severity: "warning"})
	}
	for _, conflict := range c.KGConflicts {
		out = append(out, PlanCheckView{Code: "KNOWLEDGE_CONFLICT", Where: "design.md", Message: conflict, Severity: "warning"})
	}
	if c.KGAvailable && len(c.KGReadIDs) == 0 {
		out = append(out, PlanCheckView{Code: "KNOWLEDGE_NOT_CITED", Message: "the design knowledge was available but the agent cited no element of it", Severity: "warning"})
	}
	if c.RedactedSecrets > 0 {
		out = append(out, PlanCheckView{Code: "SECRETS_REDACTED", Message: fmt.Sprintf("%d secrets were masked in the stored copy; the file on the proposal branch is unchanged", c.RedactedSecrets), Severity: "info"})
	}
	if c.SkillHash == "" {
		out = append(out, PlanCheckView{Code: "SKILLS_UNVERIFIED", Message: "the dev server did not report that all knowledge skills were installed", Severity: "warning"})
	}
	return out
}

func (uc *GetPlanProposal) find(ctx context.Context, requestID, runID string) (domain.AnalysisRun, error) {
	if runID != "" {
		run, err := uc.runs.Get(ctx, runID)
		if err != nil && !isNotFoundKind(err) {
			return domain.AnalysisRun{}, err
		}
		if err != nil || run.RequestID != requestID || run.Kind != domain.RunKindPlan {
			return domain.AnalysisRun{}, domain.ErrPlanRunNotFound(runID)
		}
		return run, nil
	}
	recent, err := uc.runs.ListRecent(ctx, requestID, domain.RecentRunsLimit)
	if err != nil {
		return domain.AnalysisRun{}, err
	}
	for _, run := range recent {
		if run.Kind == domain.RunKindPlan {
			return run, nil
		}
	}
	return domain.AnalysisRun{}, domain.ErrPlanRunNotFound("")
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
