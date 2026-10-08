package usecase

import (
	"context"
	"errors"
	"log/slog"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// maxBacklogTasksPerPlan caps one Plan's rows per call; the rest are cut and logged.
const maxBacklogTasksPerPlan = 100

var backlogTaskTypes = []string{"task", "bug", "feature", domain.TaskTypePlan, domain.TaskTypePhase}

type BacklogTaskRow struct {
	Task             domain.TaskView
	LastEngine       string
	LastLinkStatus   string
	FailedAttempts   int
	LastError        string
	BlockedByTaskIDs []string
}

type BacklogGroup struct {
	RequestID  string
	PlanID     string
	PhaseID    string
	Tasks      []BacklogTaskRow
	GroupTitle string
	PlanTitle  string
	PhaseTitle string
	// GroupStatus is the status of the Phase (or Plan) holding the tasks.
	GroupStatus string
	// TotalTasks counts every working task of the container, not just the rows shown.
	TotalTasks int
	GateStatus domain.GateStatus
}

type ListBacklogTasksInput struct {
	View         domain.BacklogView
	ProjectID    string
	RequestTypes []string
	RequestID    string
	PlanTaskID   string
	PhaseTaskID  string
	AssigneeID   string
	PageToken    string
	PageSize     int
}

// ListBacklogTasks builds the TASK and EXECUTE views. One call costs one ListTasks, one approvals query, and (for
// EXECUTE) one ListExecutionStates plus one failure lookup, however many Requests the page holds. A Task sits in
// TASK until the approvals it needs are in place, then in EXECUTE while it still has to run.
type ListBacklogTasks struct {
	Requests   BacklogRequestReader
	Approvals  ApprovalGateReader
	Tasks      TaskClient
	Outcomes   TaskRunOutcomeRepository
	Visibility RequestVisibility
	Log        *slog.Logger
}

func (uc *ListBacklogTasks) Execute(ctx context.Context, in ListBacklogTasksInput) ([]BacklogGroup, string, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, "", err
	}
	if in.View != domain.BacklogViewTask && in.View != domain.BacklogViewExecute {
		return nil, "", domain.ErrBacklogInvalidView()
	}
	limit := clampBacklogPageSize(in.PageSize)
	cursor, err := decodeBacklogCursor(in.PageToken)
	if err != nil {
		return nil, "", err
	}
	statuses := []domain.RequestStatus{domain.RequestStatusExecuting}
	if in.View == domain.BacklogViewTask {
		statuses = []domain.RequestStatus{domain.RequestStatusAwaitingPlanApproval, domain.RequestStatusExecuting}
	}
	reqs, err := uc.Requests.ListByStatus(ctx, tenantID, statuses, BacklogRequestFilter{
		ProjectID: in.ProjectID, Types: in.RequestTypes, RequestID: in.RequestID, Cursor: cursor, Limit: limit,
	})
	if err != nil {
		return nil, "", err
	}
	page, next := pageOf(reqs, limit)
	visible, err := filterVisible(ctx, uc.Visibility, page)
	if err != nil || len(visible) == 0 {
		return nil, next, err
	}

	ids := make([]string, len(visible))
	for i, r := range visible {
		ids[i] = r.ID
	}
	tasks, err := uc.Tasks.ListTasks(ctx, ListTasksQuery{RequestIDs: ids, TaskTypes: backlogTaskTypes})
	if err != nil {
		return nil, "", uc.taskServiceError(err)
	}
	approvals, err := uc.Approvals.ListGateApprovals(ctx, tenantID, ids)
	if err != nil {
		return nil, "", err
	}
	index := domain.NewApprovalIndex(approvals)
	byRequest := map[string][]domain.TaskView{}
	for _, t := range tasks {
		byRequest[t.RequestID] = append(byRequest[t.RequestID], t)
	}

	var entries []backlogEntry
	for _, r := range visible {
		entries = append(entries, uc.classify(r, domain.BuildExecutionTree(byRequest[r.ID]), index, in.View)...)
	}
	if in.View == domain.BacklogViewExecute {
		if entries, err = uc.selectForExecute(ctx, entries); err != nil {
			return nil, "", err
		}
	}
	return assembleGroups(entries, in), next, nil
}

func (uc *ListBacklogTasks) taskServiceError(err error) error {
	if errors.Is(err, domain.ErrTaskServiceDown) {
		return domain.ErrBacklogTaskServiceUnavailable(err)
	}
	return err
}

// backlogEntry is a working task with the container and gate it was judged under.
type backlogEntry struct {
	request   domain.Request
	task      domain.TaskView
	container *domain.TaskView
	plan      *domain.TaskView
	gate      domain.GateResolution
	row       BacklogTaskRow
	// totalInContainer counts the container's working tasks in every status.
	totalInContainer int
}

// classify keeps the tasks that belong in the view by status and gate. EXECUTE entries are refined later, once the
// execution states of the gate-approved candidates are known.
func (uc *ListBacklogTasks) classify(req domain.Request, tree domain.ExecutionTree, index domain.ApprovalIndex, view domain.BacklogView) []backlogEntry {
	var out []backlogEntry
	perContainer := map[string]int{}
	for _, l := range tree.Leaves {
		c, hasContainer := tree.Container(l)
		if !hasContainer && l.ParentID != "" {
			continue // nested deeper than a Phase or Plan: not shown in this version
		}
		perContainer[l.ParentID]++
		if len(out) >= maxBacklogTasksPerPlan {
			uc.log().Warn("backlog cut: too many tasks in one plan", slog.String("request_id", req.ID))
			continue
		}
		gate := domain.ResolveTaskGate(domain.TaskGateInput{Request: req, Task: l, Container: c, Plan: tree.Plan, Approvals: index})
		working := l.Status == domain.TaskStatusOpen || l.Status == domain.TaskStatusBlocked
		switch view {
		case domain.BacklogViewTask:
			if !working || gate.Approved {
				continue
			}
		default: // execute: the Request must be running and the gate open; the state rule comes later
			if req.Status != domain.RequestStatusExecuting || !gate.Approved || taskFinishedStatus(l.Status) {
				continue
			}
		}
		out = append(out, backlogEntry{request: req, task: l, container: c, plan: tree.Plan, gate: gate, row: BacklogTaskRow{Task: l}})
	}
	for i := range out {
		out[i].totalInContainer = perContainer[out[i].task.ParentID]
	}
	return out
}

func (uc *ListBacklogTasks) log() *slog.Logger {
	if uc.Log != nil {
		return uc.Log
	}
	return slog.Default()
}

// selectForExecute keeps open and blocked tasks, plus any task whose latest run failed, and fills the run details.
func (uc *ListBacklogTasks) selectForExecute(ctx context.Context, entries []backlogEntry) ([]backlogEntry, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	ids := make([]string, len(entries))
	for i, e := range entries {
		ids[i] = e.task.ID
	}
	states, err := uc.Tasks.ListExecutionStates(ctx, ids)
	if err != nil {
		return nil, uc.taskServiceError(err)
	}
	var kept []backlogEntry
	var keptIDs []string
	for _, e := range entries {
		st := states[e.task.ID]
		working := e.task.Status == domain.TaskStatusOpen || e.task.Status == domain.TaskStatusBlocked
		if !working && st.LastLinkStatus != "failed" {
			continue
		}
		e.row.LastEngine, e.row.LastLinkStatus, e.row.FailedAttempts, e.row.BlockedByTaskIDs = st.LastEngine, st.LastLinkStatus, st.FailedAttempts, st.BlockedByTaskIDs
		kept = append(kept, e)
		keptIDs = append(keptIDs, e.task.ID)
	}
	failures, err := uc.Outcomes.LatestFailed(ctx, keptIDs)
	if err != nil {
		return nil, err
	}
	for i := range kept {
		kept[i].row.LastError = failures[kept[i].task.ID].ErrorMessage
	}
	return kept, nil
}

// assembleGroups groups entries by container, in request order, and applies the optional plan, phase and assignee filters.
func assembleGroups(entries []backlogEntry, in ListBacklogTasksInput) []BacklogGroup {
	var groups []BacklogGroup
	index := map[string]int{}
	for _, e := range entries {
		if in.AssigneeID != "" && e.task.AssigneeID != in.AssigneeID {
			continue
		}
		planID, phaseID := "", ""
		if e.plan != nil {
			planID = e.plan.ID
		}
		if e.container != nil && e.container.Type == domain.TaskTypePhase {
			phaseID = e.container.ID
		}
		if (in.PlanTaskID != "" && planID != in.PlanTaskID) || (in.PhaseTaskID != "" && phaseID != in.PhaseTaskID) {
			continue
		}
		key := e.request.ID + "/" + e.task.ParentID
		i, ok := index[key]
		if !ok {
			g := BacklogGroup{RequestID: e.request.ID, PlanID: planID, PhaseID: phaseID, GateStatus: e.gate.Status, TotalTasks: e.totalInContainer}
			if e.plan != nil {
				g.PlanTitle = e.plan.Title
			}
			if e.container != nil {
				g.GroupTitle, g.GroupStatus = e.container.Title, e.container.Status
				if phaseID != "" {
					g.PhaseTitle = e.container.Title
				}
			} else {
				g.GroupTitle = e.task.Title
			}
			groups = append(groups, g)
			i = len(groups) - 1
			index[key] = i
		}
		groups[i].Tasks = append(groups[i].Tasks, e.row)
	}
	return groups
}
