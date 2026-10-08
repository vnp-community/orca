package domain

type GateStatus string

const (
	GateStatusApproved GateStatus = "approved"
	GateStatusPending  GateStatus = "pending"
	GateStatusRejected GateStatus = "rejected"
	GateStatusNone     GateStatus = "none"
)

// TaskView is the slice of a task-service Task the backlog views and the execution loop need.
type TaskView struct {
	ID             string
	ParentID       string
	Type           string
	Status         string
	RequestID      string
	Title          string
	AssigneeID     string
	WorktreeID     string
	EstimatedHours *float64
	Labels         []string
}

// Ref converts to the shape type policies read.
func (t TaskView) Ref() TaskRef {
	return TaskRef{ID: t.ID, Title: t.Title, ParentID: t.ParentID, Status: t.Status, Labels: t.Labels}
}

// IsWorkingTask is true for the tasks an agent runs (leaves), as opposed to Plan and Phase containers.
func (t TaskView) IsWorkingTask() bool { return !IsContainerTaskType(t.Type) && t.Type != "epic" }

type TaskGateInput struct {
	Request Request
	Task    TaskView
	// Container is the task's direct parent: a phase or a plan. Nil for a task with no parent (hotfix).
	Container *TaskView
	// Plan is the plan above the task; it equals Container when the task sits directly under the plan.
	Plan      *TaskView
	Approvals ApprovalIndex
}

type GateResolution struct {
	Approved bool
	Status   GateStatus
	// WaitingForPhaseSplit means the flow wants Phases but the task still hangs directly under the Plan.
	WaitingForPhaseSplit bool
}

// ApprovalIndex holds the newest Approval of each (subject type, subject id).
type ApprovalIndex map[SubjectType]map[string]Approval

// NewApprovalIndex keeps the row with the latest created_at per subject; ties go to the larger id.
func NewApprovalIndex(approvals []Approval) ApprovalIndex {
	idx := make(ApprovalIndex)
	for _, a := range approvals {
		if idx[a.SubjectType] == nil {
			idx[a.SubjectType] = make(map[string]Approval)
		}
		cur, ok := idx[a.SubjectType][a.SubjectID]
		if !ok || a.CreatedAt.After(cur.CreatedAt) || (a.CreatedAt.Equal(cur.CreatedAt) && a.ID > cur.ID) {
			idx[a.SubjectType][a.SubjectID] = a
		}
	}
	return idx
}

func (idx ApprovalIndex) GetLatest(st SubjectType, id string) *Approval {
	if a, ok := idx[st][id]; ok {
		return &a
	}
	return nil
}

func gateStatusOf(a *Approval) GateStatus {
	if a == nil {
		return GateStatusNone
	}
	switch a.Status {
	case ApprovalStatusApproved:
		return GateStatusApproved
	case ApprovalStatusPending:
		return GateStatusPending
	case ApprovalStatusRejected:
		return GateStatusRejected
	}
	return GateStatusNone // cancelled and expired leave the subject unapproved
}

// startGateSubject is the Approval that releases the Plan itself for this flow.
func startGateSubject(g GateSubject) (SubjectType, bool) {
	switch g {
	case GatePlan:
		return SubjectPlan, true
	case GateTaskList:
		return SubjectTaskList, true
	case GatePreDeploy:
		return SubjectPreDeploy, true
	}
	return "", false
}

// ResolveTaskGate says whether the approvals a task needs before it may run are all in place (README v6 3.8).
func ResolveTaskGate(in TaskGateInput) GateResolution {
	flow, err := FlowFor(in.Request.Type)
	if err != nil {
		return GateResolution{Status: GateStatusNone} // untyped request: no flow, nothing can have been approved
	}
	var statuses []GateStatus
	planGate := func(plan *TaskView) {
		subject, ok := startGateSubject(flow.StartGate)
		if !ok || plan == nil {
			statuses = append(statuses, GateStatusNone)
			return
		}
		statuses = append(statuses, gateStatusOf(in.Approvals.GetLatest(subject, plan.ID)))
	}
	switch {
	case in.Container == nil:
		if flow.PlanKind != PlanSingleTask {
			return GateResolution{Status: GateStatusNone}
		}
		statuses = append(statuses, gateStatusOf(in.Approvals.GetLatest(SubjectPreDeploy, in.Task.ID)))
	case in.Container.Type == TaskTypePhase:
		if flow.HasExecutionGate(GatePhase) {
			statuses = append(statuses, gateStatusOf(in.Approvals.GetLatest(SubjectPhase, in.Container.ID)))
		}
		planGate(in.Plan)
	default: // directly under the plan
		if flow.PhasesFor(in.Request.Size) {
			return GateResolution{Status: GateStatusNone, WaitingForPhaseSplit: true}
		}
		planGate(in.Container)
	}
	return combineGateStatuses(statuses)
}

// combineGateStatuses: any rejection wins, then any pending, then a missing approval; approved needs every one.
func combineGateStatuses(statuses []GateStatus) GateResolution {
	overall := GateStatusApproved
	rank := map[GateStatus]int{GateStatusApproved: 0, GateStatusNone: 1, GateStatusPending: 2, GateStatusRejected: 3}
	for _, s := range statuses {
		if rank[s] > rank[overall] {
			overall = s
		}
	}
	return GateResolution{Approved: overall == GateStatusApproved, Status: overall}
}
