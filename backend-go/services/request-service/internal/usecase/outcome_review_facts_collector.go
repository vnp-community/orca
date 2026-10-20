package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ReviewDiff is what git-gateway says a task worktree changed against the Request's base ref.
type ReviewDiff struct {
	// Status is ready, invalid-base, unborn-head, no-merge-base, loading or error.
	Status     string
	BaseOID    string
	HeadOID    string
	CompareRef string
	MergeBase  string
	Entries    []domain.ChangedPath
}

// ReviewDiffReader reads the diff of one worktree. It is the only place a review touches git.
type ReviewDiffReader interface {
	BranchCompare(ctx context.Context, worktreeID, baseRef string) (ReviewDiff, error)
}

// ReviewImpactReader and ReviewGateReader are optional: without them the release risk reads unknown and the quality gate absent.
type ReviewImpactReader interface {
	// Risk returns low, medium, high or critical, and false when no assessment exists for the Request.
	Risk(ctx context.Context, requestID string) (string, bool, error)
}

type ReviewGateReader interface {
	// Gate returns pass, warn, fail or unknown, and false when the quality gate is not available.
	Gate(ctx context.Context, requestID string) (string, bool, error)
}

// ReviewBreakingReader says whether a contract check found a breaking change; false for found when nothing was measured.
type ReviewBreakingReader interface {
	Breaking(ctx context.Context, requestID string) (breaking bool, measured bool, err error)
}

// ReviewFactsCollector gathers what Orca can measure itself. A source that fails does not fail the review: the fact is left
// unread and a warning says so.
type ReviewFactsCollector struct {
	Tasks    TaskClient
	Outcomes TaskRunOutcomeRepository
	Coverage RequestCoverageRepository
	Diff     ReviewDiffReader
	Impact   ReviewImpactReader
	Gate     ReviewGateReader
	Breaking ReviewBreakingReader
	Log      *slog.Logger
}

// Collected is the facts and what the collector learned on the way.
type Collected struct {
	Facts domain.Facts
	// Finished is true when execution is over: the Request is completed, or every working task is done or cancelled.
	Finished bool
	// Tree is the task tree the facts were built from.
	Tree domain.ExecutionTree
}

func (c *ReviewFactsCollector) logger() *slog.Logger {
	if c.Log != nil {
		return c.Log
	}
	return slog.Default()
}

// Collect reads the Request's tasks, coverage and outcomes and, for a delivered review, the diff of each task worktree. An empty
// phase lets the collector decide: delivered once execution is over, design before that. A phase that is asked for is refused when
// the Request is not ready for it.
func (c *ReviewFactsCollector) Collect(ctx context.Context, req domain.Request, phase domain.ReviewPhase) (Collected, error) {
	return c.collect(ctx, req, phase, true)
}

// CollectDatabaseFacts is Collect without git: what a read may ask, because a read must stay cheap and must not wait for a dev server.
func (c *ReviewFactsCollector) CollectDatabaseFacts(ctx context.Context, req domain.Request, phase domain.ReviewPhase) (Collected, error) {
	return c.collect(ctx, req, phase, false)
}

func (c *ReviewFactsCollector) collect(ctx context.Context, req domain.Request, phase domain.ReviewPhase, withGit bool) (Collected, error) {
	content, err := domain.ContentFromRequest(req)
	if err != nil {
		return Collected{}, err
	}
	tasks, terr := c.Tasks.ListTasks(ctx, ListTasksQuery{RequestIDs: []string{req.ID}, TaskTypes: executionTaskTypes})
	if terr != nil {
		c.logger().WarnContext(ctx, "outcome review: tasks unreadable", slog.String("request_id", req.ID), slog.Any("error", terr))
	}
	tree := domain.BuildExecutionTreeOf(req.PlanTaskID, tasks)
	finished := executionFinished(req, tree)
	switch {
	case phase == "":
		phase = domain.ReviewPhaseDesign
		if finished {
			phase = domain.ReviewPhaseDelivered
		}
	case phase == domain.ReviewPhaseDelivered && !finished:
		return Collected{}, domain.ErrReviewPhaseNotReady(phase, "the after-code review (execution has not finished)")
	case !phase.Valid():
		return Collected{}, domain.ErrReviewDecisionInvalid("unknown review phase " + string(phase))
	}
	facts := domain.Facts{Phase: phase, Subject: domain.ReviewSubject{BaseRef: req.BaseRef, Engine: string(req.Engine), ExecutionMode: string(req.ExecutionMode)}}
	facts.Requests = domain.RequirementInputs{Phase: phase, Criteria: content.AcceptanceCriteria.Items, DataOK: true}
	if terr != nil {
		facts.Requests.DataOK = false
		facts.Warnings = append(facts.Warnings, "partial_facts:tasks")
	}

	failed := map[string]bool{}
	if c.Outcomes != nil {
		outcomes, oerr := c.Outcomes.ListByRequest(ctx, req.ID)
		if oerr != nil {
			c.logger().WarnContext(ctx, "outcome review: outcomes unreadable", slog.String("request_id", req.ID), slog.Any("error", oerr))
			facts.Requests.DataOK = false
			facts.Warnings = append(facts.Warnings, "partial_facts:outcomes")
		}
		last := map[string]domain.Outcome{}
		for _, o := range outcomes { // oldest first, so the last write is the newest outcome
			if o.Outcome != domain.OutcomeStarted {
				last[o.TaskID] = o.Outcome
			}
		}
		for id, o := range last {
			failed[id] = o == domain.OutcomeFailed
		}
	}
	facts.Requests.Tasks = map[string]domain.CoveringTask{}
	var descriptions []string
	for _, leaf := range tree.Leaves {
		facts.Requests.Tasks[leaf.ID] = domain.CoveringTask{Status: leaf.Status, Title: leaf.Title, Failed: failed[leaf.ID]}
		descriptions = append(descriptions, leaf.Description)
	}
	facts.PlanTargets, facts.PlanTargetsSet = domain.PlanTargets(descriptions), terr == nil

	rows, cerr := c.Coverage.ListByRequest(ctx, req.ID)
	if cerr != nil {
		c.logger().WarnContext(ctx, "outcome review: coverage unreadable", slog.String("request_id", req.ID), slog.Any("error", cerr))
		facts.Requests.DataOK = false
		facts.Warnings = append(facts.Warnings, "partial_facts:coverage")
	}
	facts.Requests.Coverage = rows

	out := Collected{Facts: facts, Tree: tree, Finished: finished}
	if phase == domain.ReviewPhaseDelivered && withGit {
		c.collectDiff(ctx, req, tree, &out.Facts)
		c.collectOptional(ctx, req, &out.Facts)
	}
	return out, nil
}

// executionFinished is the one place that says when a design review may become a delivered one.
func executionFinished(req domain.Request, tree domain.ExecutionTree) bool {
	switch req.Status {
	case domain.RequestStatusCompleted:
		return true
	case domain.RequestStatusExecuting:
		if len(tree.Leaves) == 0 {
			return false
		}
		for _, t := range tree.Leaves {
			if t.Status != domain.TaskStatusDone && t.Status != domain.TaskStatusCancelled {
				return false
			}
		}
		return true
	}
	return false
}

func (c *ReviewFactsCollector) collectDiff(ctx context.Context, req domain.Request, tree domain.ExecutionTree, f *domain.Facts) {
	if c.Diff == nil {
		f.Warnings = append(f.Warnings, "partial_facts:diff")
		return
	}
	type agg struct {
		domain.ChangedPath
	}
	merged := map[string]*agg{}
	var heads []domain.ReviewHead
	seenWorktree := map[string]bool{}
	read := 0
	for _, leaf := range tree.Leaves {
		if leaf.WorktreeID == "" || seenWorktree[leaf.WorktreeID] {
			continue
		}
		seenWorktree[leaf.WorktreeID] = true
		d, err := c.Diff.BranchCompare(ctx, leaf.WorktreeID, req.BaseRef)
		if err != nil || d.Status != "ready" {
			c.logger().WarnContext(ctx, "outcome review: diff unreadable", slog.String("request_id", req.ID), slog.String("task_id", leaf.ID),
				slog.String("status", d.Status), slog.Any("error", err))
			f.Warnings = append(f.Warnings, fmt.Sprintf("partial_facts:diff:%s", leaf.ID))
			continue
		}
		read++
		heads = append(heads, domain.ReviewHead{Branch: d.CompareRef, Commit: d.HeadOID, TaskID: leaf.ID})
		if f.Subject.BaseCommit == "" {
			f.Subject.BaseCommit = d.BaseOID
		}
		for _, e := range d.Entries {
			a, ok := merged[e.Path]
			if !ok {
				cp := e
				merged[e.Path] = &agg{cp}
				continue
			}
			a.Added += e.Added
			a.Removed += e.Removed
		}
	}
	if len(seenWorktree) == 0 {
		f.Warnings = append(f.Warnings, "partial_facts:diff:no_worktree")
		return
	}
	paths := make([]string, 0, len(merged))
	for p := range merged {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	f.Changed = make([]domain.ChangedPath, 0, len(paths))
	for _, p := range paths {
		f.Changed = append(f.Changed, merged[p].ChangedPath)
	}
	f.ChangedRead = read > 0 && read == len(seenWorktree)
	sort.Slice(heads, func(i, j int) bool { return heads[i].TaskID < heads[j].TaskID })
	f.Subject.Heads = heads
	if len(heads) == 1 {
		f.Subject.HeadCommit = heads[0].Commit
	} else {
		f.Subject.HeadCommit = domain.HeadKey(domain.ReviewPhaseDelivered, heads) // several branches: a key stands for the set
	}
}

func (c *ReviewFactsCollector) collectOptional(ctx context.Context, req domain.Request, f *domain.Facts) {
	if c.Impact != nil {
		if risk, ok, err := c.Impact.Risk(ctx, req.ID); err != nil {
			f.Warnings = append(f.Warnings, "partial_facts:impact")
		} else if ok {
			f.ImpactRisk = risk
		}
	}
	if c.Gate != nil {
		if gate, ok, err := c.Gate.Gate(ctx, req.ID); err != nil {
			f.Warnings = append(f.Warnings, "partial_facts:gate")
		} else if ok {
			f.QualityGate = gate
		}
	}
	if c.Breaking != nil {
		if b, measured, err := c.Breaking.Breaking(ctx, req.ID); err != nil {
			f.Warnings = append(f.Warnings, "partial_facts:breaking")
		} else if measured {
			f.Breaking = &b
		}
	}
}
