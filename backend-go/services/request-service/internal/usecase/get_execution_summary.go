package usecase

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// GetExecutionSummary answers, in one read that is safe to repeat every few seconds, how a run is going: progress, the tasks
// running now, the ones that failed and why, and what waits for the person asking. It writes nothing.
type GetExecutionSummary struct {
	Requests    RequestReader
	Tasks       TaskClient
	Outcomes    TaskRunOutcomeRepository
	PhaseStarts PhaseStartRepository
	Approvals   ApprovalGateReader
	// Authorizer decides which pending approvals the caller may decide; nil lists none, so nothing is shown to someone who may not act on it.
	Authorizer     ApprovalAuthorizer
	Clarifications ClarificationOpenReader
	// Changes is nil unless the OpenSpec engine is wired; the handover then stays "none".
	Changes     OpenSpecChangeReader
	Engine      *RequestEngineResolver
	MaxAttempts int
	Log         *slog.Logger
}

// ClarificationOpenReader is the slice of ClarificationRepository the summary needs.
type ClarificationOpenReader interface {
	GetOpenByRequest(ctx context.Context, requestID string) (*domain.Clarification, error)
}

// OpenSpecChangeReader is the slice of OpenSpecChangeRepository the summary needs.
type OpenSpecChangeReader interface {
	GetByRequest(ctx context.Context, requestID string) (domain.OpenSpecChange, bool, error)
}

type taskRunStats struct {
	started        int
	failedAttempts int // failures that used up an attempt; a dispatch error never reached the agent
	lastStartedAt  time.Time
	lastFailed     *domain.TaskRunOutcome
}

func (uc *GetExecutionSummary) Execute(ctx context.Context, requestID string) (domain.ExecutionSummary, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return domain.ExecutionSummary{}, domain.ErrRequestTenantRequired()
	}
	if _, err := uuid.Parse(requestID); err != nil {
		return domain.ExecutionSummary{}, domain.ErrRequestNotFound(requestID)
	}
	req, err := uc.Requests.Get(ctx, requestID)
	if err != nil {
		return domain.ExecutionSummary{}, err
	}
	one := []domain.Request{req}
	uc.Engine.Annotate(ctx, one)
	req = one[0]
	max := uc.MaxAttempts
	if max <= 0 {
		max = DefaultExecutionSettings().MaxTaskAttempts
	}
	sum := domain.ExecutionSummary{
		RequestID: req.ID, Status: req.Status, Engine: req.Engine, ExecutionMode: req.ExecutionMode, MaxAttempts: max,
		Handover: domain.Handover{State: domain.HandoverNone},
	}

	tasks, err := uc.Tasks.ListTasks(ctx, ListTasksQuery{RequestIDs: []string{req.ID}, TaskTypes: executionTaskTypes})
	if err != nil {
		return domain.ExecutionSummary{}, err
	}
	tree := domain.BuildExecutionTreeOf(req.PlanTaskID, tasks)
	outcomes, err := uc.Outcomes.ListByRequest(ctx, req.ID)
	if err != nil {
		return domain.ExecutionSummary{}, err
	}
	stats := statsByTask(outcomes)
	starts, err := uc.PhaseStarts.ListByRequest(ctx, req.ID)
	if err != nil {
		return domain.ExecutionSummary{}, err
	}
	started := make(map[string]bool, len(starts))
	for _, s := range starts {
		started[s.PhaseTaskID] = true
	}
	for _, o := range outcomes {
		if o.Outcome == domain.OutcomeStarted && (sum.StartedAt == nil || o.OccurredAt.Before(*sum.StartedAt)) {
			at := o.OccurredAt
			sum.StartedAt = &at
		}
	}

	uc.fillProgress(&sum, tree, stats, started, max)
	if err := uc.fillNeedsYou(ctx, &sum, req, tree); err != nil {
		return domain.ExecutionSummary{}, err
	}
	uc.fillHandover(ctx, &sum, req)
	return sum, nil
}

func statsByTask(outcomes []domain.TaskRunOutcome) map[string]*taskRunStats {
	out := map[string]*taskRunStats{}
	for i := range outcomes {
		o := outcomes[i]
		st := out[o.TaskID]
		if st == nil {
			st = &taskRunStats{}
			out[o.TaskID] = st
		}
		switch o.Outcome {
		case domain.OutcomeStarted:
			st.started++
			st.lastStartedAt = o.OccurredAt
		case domain.OutcomeFailed:
			if o.Cause != domain.CauseDispatchError {
				st.failedAttempts++
			}
			st.lastFailed = &outcomes[i]
		}
	}
	return out
}

func (uc *GetExecutionSummary) fillProgress(sum *domain.ExecutionSummary, tree domain.ExecutionTree, stats map[string]*taskRunStats, started map[string]bool, max int) {
	sum.Progress.PhasesTotal = len(tree.Phases)
	for _, p := range tree.Phases {
		if p.Status == domain.TaskStatusDone {
			sum.Progress.PhasesDone++
		}
		if sum.CurrentPhase == "" && started[p.ID] && !taskFinishedStatus(p.Status) {
			sum.CurrentPhase = p.ID
		}
	}
	if next, ok := tree.NextPhase(started); ok {
		sum.NextPhase = next.ID
	}
	for _, l := range tree.Leaves {
		if l.Status == domain.TaskStatusCancelled {
			continue
		}
		sum.Progress.TasksTotal++
		st := stats[l.ID]
		switch {
		case l.Status == domain.TaskStatusDone:
			sum.Progress.TasksDone++
		case l.Status == domain.TaskStatusInProgress:
			sum.Progress.TasksRunning++
			rt := domain.RunningTask{TaskID: l.ID, PhaseID: phaseOf(tree, l), Title: l.Title, MaxAttempts: max, Attempt: 1}
			if st != nil {
				rt.Attempt = st.failedAttempts + 1
				rt.StartedAt = st.lastStartedAt
			}
			sum.Running = append(sum.Running, rt)
		case st != nil && st.lastFailed != nil && l.Status != domain.TaskStatusReview:
			sum.Progress.TasksFailed++
			sum.Failed = append(sum.Failed, domain.FailedTask{
				TaskID: l.ID, PhaseID: phaseOf(tree, l), Title: l.Title, Attempts: st.failedAttempts, MaxAttempts: max,
				LastCause: domain.ClassifyFailureCause(st.lastFailed.Cause, st.lastFailed.ErrorMessage),
				LastError: truncateSummaryText(st.lastFailed.ErrorMessage, 400), LastAt: st.lastFailed.OccurredAt.UTC(),
			})
		}
	}
	sort.SliceStable(sum.Running, func(i, j int) bool { return sum.Running[i].StartedAt.Before(sum.Running[j].StartedAt) })
}

func (uc *GetExecutionSummary) fillNeedsYou(ctx context.Context, sum *domain.ExecutionSummary, req domain.Request, tree domain.ExecutionTree) error {
	tenantID, _ := tenant.TenantID(ctx)
	approvals, err := uc.Approvals.ListGateApprovals(ctx, tenantID, []string{req.ID})
	if err != nil {
		return err
	}
	for _, a := range approvals {
		if a.Status != domain.ApprovalStatusPending || uc.Authorizer == nil {
			continue
		}
		if uc.Authorizer.CanDecide(ctx, req, a) != nil {
			continue
		}
		item := domain.NeedsYouItem{Kind: domain.NeedsApproval, RefID: a.ID, SubjectType: string(a.SubjectType), Since: a.CreatedAt.UTC(), DueAt: a.DueAt}
		switch a.SubjectType {
		case domain.SubjectPhase:
			if p, ok := tree.Phase(a.SubjectID); ok {
				item.Title, item.PhaseID = p.Title, p.ID
			}
			if a.Stage == "drift_review" {
				item.Kind = domain.NeedsDrift
			}
		case domain.SubjectPreDeploy:
			item.Kind = domain.NeedsPreDeploy
			if t, ok := tree.Task(a.SubjectID); ok {
				item.Title = t.Title
			}
		default:
			if tree.Plan != nil {
				item.Title = tree.Plan.Title
			}
		}
		sum.NeedsYou = append(sum.NeedsYou, item)
	}
	if uc.Clarifications != nil {
		open, err := uc.Clarifications.GetOpenByRequest(ctx, req.ID)
		if err != nil {
			return err
		}
		if open != nil && callerMayAnswer(ctx, req, *open) {
			due := open.DueAt
			sum.NeedsYou = append(sum.NeedsYou, domain.NeedsYouItem{
				Kind: domain.NeedsClarification, RefID: open.ID, SubjectType: "clarification", Title: string(open.Source),
				Since: open.CreatedAt.UTC(), DueAt: &due,
			})
		}
	}
	// A task out of attempts is what sent the Request back, or what is about to.
	if len(sum.Failed) > 0 && (req.Status == domain.RequestStatusRequestBacklog || req.Status == domain.RequestStatusExecuting) {
		for _, f := range sum.Failed {
			if f.Attempts >= f.MaxAttempts {
				sum.NeedsYou = append(sum.NeedsYou, domain.NeedsYouItem{
					Kind: domain.NeedsFailedTask, RefID: f.TaskID, SubjectType: "task", Title: f.Title, PhaseID: f.PhaseID, Since: f.LastAt,
				})
			}
		}
	}
	sort.SliceStable(sum.NeedsYou, func(i, j int) bool { return sum.NeedsYou[i].Since.Before(sum.NeedsYou[j].Since) })
	return nil
}

// callerMayAnswer is true for the reporter, an administrator or a named assignee: the people for whom the question is a task.
func callerMayAnswer(ctx context.Context, req domain.Request, c domain.Clarification) bool {
	me := callerID(ctx)
	if me == "" {
		return false
	}
	if callerIsAdmin(ctx) || me == req.ReporterID {
		return true
	}
	for _, p := range c.Assignees {
		if p.Kind == domain.PrincipalKindUser && p.ID == me {
			return true
		}
	}
	return false
}

func (uc *GetExecutionSummary) fillHandover(ctx context.Context, sum *domain.ExecutionSummary, req domain.Request) {
	if req.Engine != domain.EngineOpenSpec || uc.Changes == nil {
		return
	}
	ch, found, err := uc.Changes.GetByRequest(ctx, req.ID)
	if err != nil {
		uc.log().WarnContext(ctx, "openspec change unreadable for the execution summary", slog.String("request_id", req.ID), slog.Any("error", err))
		return
	}
	if !found {
		return
	}
	sum.Handover.ArchiveAttempts = ch.SyncAttempts
	switch {
	case ch.Status == domain.ChangeStatusArchived:
		sum.Handover.State, sum.Handover.SummaryRef = domain.HandoverReady, ch.ChangeID
	case ch.SyncState == domain.SyncStateFailed && req.Status == domain.RequestStatusCompleted:
		sum.Handover.State = domain.HandoverFailed
	case req.Status == domain.RequestStatusCompleted:
		sum.Handover.State = domain.HandoverArchiving
	}
}

func (uc *GetExecutionSummary) log() *slog.Logger {
	if uc.Log != nil {
		return uc.Log
	}
	return slog.Default()
}

func truncateSummaryText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.ToValidUTF8(s[:n], "") + "…"
}
