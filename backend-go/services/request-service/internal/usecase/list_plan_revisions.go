package usecase

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

const (
	planRevisionsDefaultPage = 20
	planRevisionsMaxPage     = 100
)

// PlanRevisionView is one version of a Plan: a successful Plan run, numbered oldest first.
type PlanRevisionView struct {
	Revision int
	RunID    string
	// Status is current for the newest version and superseded for the others.
	Status string
	Engine string
	// RevisionFeedback is what the person asked to change; empty for a first Plan, a retry and an answer to a clarification.
	RevisionFeedback string
	ActorID          string
	GeneratedAt      time.Time
	// Commit, FilesChanged, FileCount and TextKept describe the authored documents (openspec only).
	Commit       string
	FilesChanged int
	FileCount    int
	TextKept     bool
	Stats        *domain.RunStats
}

type PlanRevisionsPage struct {
	Revisions     []PlanRevisionView
	NextPageToken string
}

// ListPlanRevisions lists the versions of a Request's Plan, newest first. The runs give the numbers, the feedback and the
// time; the stored change versions give the documents, joined by run so a Request that predates them still lists.
type ListPlanRevisions struct {
	requests  RequestRepository
	runs      AnalysisRunStore
	revisions OpenSpecChangeRevisionRepository
	changes   OpenSpecChangeRepository
}

func NewListPlanRevisions(requests RequestRepository, runs AnalysisRunStore, revisions OpenSpecChangeRevisionRepository, changes OpenSpecChangeRepository) *ListPlanRevisions {
	return &ListPlanRevisions{requests: requests, runs: runs, revisions: revisions, changes: changes}
}

func (uc *ListPlanRevisions) Execute(ctx context.Context, requestID string, pageSize int, pageToken string) (PlanRevisionsPage, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return PlanRevisionsPage{}, domain.ErrRequestTenantRequired()
	}
	req, err := uc.requests.Get(ctx, requestID)
	if err != nil {
		return PlanRevisionsPage{}, err
	}
	runs, err := uc.runs.ListSucceeded(ctx, req.ID, domain.RunKindPlan)
	if err != nil {
		return PlanRevisionsPage{}, err
	}
	total := len(runs)
	before, err := planRevisionsBefore(pageToken, total)
	if err != nil {
		return PlanRevisionsPage{}, err
	}
	size := planRevisionsPageSize(pageSize)
	hi := min(before-1, total)
	if hi < 1 {
		return PlanRevisionsPage{}, nil
	}
	lo := max(1, hi-size+1)

	rows, err := uc.rowsByRun(ctx, req.ID, hi, size+1) // one more than the page: the version before its oldest item
	if err != nil {
		return PlanRevisionsPage{}, err
	}
	if hi == total {
		if err := uc.addLegacyCurrent(ctx, req.ID, runs[total-1], rows); err != nil {
			return PlanRevisionsPage{}, err
		}
	}
	page := PlanRevisionsPage{Revisions: make([]PlanRevisionView, 0, hi-lo+1)}
	for n := hi; n >= lo; n-- {
		var prev *domain.OpenSpecChangeRevision
		if n > 1 {
			if r, ok := rows[runs[n-2].ID]; ok {
				prev = &r
			}
		}
		page.Revisions = append(page.Revisions, planRevisionView(n, total, runs[n-1], rows, prev))
	}
	if lo > 1 {
		page.NextPageToken = strconv.Itoa(lo)
	}
	return page, nil
}

// rowsByRun reads the stored versions with a number up to hi, newest first.
func (uc *ListPlanRevisions) rowsByRun(ctx context.Context, requestID string, hi, limit int) (map[string]domain.OpenSpecChangeRevision, error) {
	out := map[string]domain.OpenSpecChangeRevision{}
	if uc.revisions == nil {
		return out, nil
	}
	list, err := uc.revisions.List(ctx, requestID, hi+1, limit)
	if err != nil {
		return nil, err
	}
	for _, r := range list {
		out[r.RunID] = r
	}
	return out, nil
}

// addLegacyCurrent stands in for the stored version of a Request whose newest Plan run predates the history table: the
// current change row is that version. Older versions of such a Request have no documents.
func (uc *ListPlanRevisions) addLegacyCurrent(ctx context.Context, requestID string, newest domain.AnalysisRun, rows map[string]domain.OpenSpecChangeRevision) error {
	if _, stored := rows[newest.ID]; stored || newest.Mode != domain.AnalysisModeAgentProposal || uc.changes == nil {
		return nil
	}
	change, found, err := uc.changes.GetByRequest(ctx, requestID)
	if err != nil || !found {
		return err
	}
	rows[newest.ID] = domain.OpenSpecChangeRevision{
		RequestID: requestID, RunID: newest.ID, Commit: change.Commit, Files: domain.NewRevisionFiles(change.Bundle), TextKept: true,
	}
	return nil
}

func planRevisionView(n, total int, run domain.AnalysisRun, rows map[string]domain.OpenSpecChangeRevision, prev *domain.OpenSpecChangeRevision) PlanRevisionView {
	status := "superseded"
	if n == total {
		status = "current"
	}
	v := PlanRevisionView{Revision: n, RunID: run.ID, Status: status, Engine: string(run.Mode.EngineFor()), ActorID: run.ActorID, Stats: run.Stats, GeneratedAt: run.StartedAt}
	if run.FinishedAt != nil {
		v.GeneratedAt = *run.FinishedAt
	}
	if !strings.HasPrefix(run.Feedback, domain.ClarificationFeedbackPrefix) {
		v.RevisionFeedback = run.Feedback
	}
	row, stored := rows[run.ID]
	switch {
	case stored:
		v.Commit, v.TextKept, v.FileCount = row.Commit, row.TextKept, len(row.Files)
		v.FilesChanged = v.FileCount // the first version, or one whose predecessor has no index, counts every file
		if prev != nil {
			v.FilesChanged = countChangedFiles(domain.DiffFileIndex(prev.Files, row.Files))
		}
	case run.Mode == domain.AnalysisModeAgentProposal:
		v.TextKept = false // an authored change whose documents were never kept
	default:
		v.TextKept = true // a native Plan is its proposal, which the run keeps
	}
	return v
}

func countChangedFiles(diff []domain.FileChange) int {
	n := 0
	for _, c := range diff {
		if c.Kind != domain.FileUnchanged {
			n++
		}
	}
	return n
}

func planRevisionsPageSize(n int) int {
	switch {
	case n <= 0:
		return planRevisionsDefaultPage
	case n > planRevisionsMaxPage:
		return planRevisionsMaxPage
	}
	return n
}

// planRevisionsBefore reads the token (the number the previous page ended with): the page holds the versions below it.
func planRevisionsBefore(token string, total int) (int, error) {
	if token == "" {
		return total + 1, nil
	}
	n, err := strconv.Atoi(token)
	if err != nil || n < 1 {
		return 0, domain.ErrPlanRevisionPageTokenInvalid()
	}
	return min(n, total+1), nil
}
