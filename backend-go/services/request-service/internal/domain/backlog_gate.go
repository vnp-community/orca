package domain

type GateStatus string

const (
	GateStatusApproved GateStatus = "approved"
	GateStatusPending  GateStatus = "pending"
	GateStatusRejected GateStatus = "rejected"
	GateStatusNone     GateStatus = "none"
)

type TaskView struct {
	ID             string
	ParentID       string
	Type           string
	Status         string
	RequestID      string
	Title          string
	EstimatedHours int
	AssigneeID     string
	Kind           string // For plan kind: plan or task_list
	Size           string
}

type TaskGateInput struct {
	Request   Request
	Task      TaskView
	Container *TaskView
	Plan      *TaskView
	Approvals ApprovalIndex
}

type GateResolution struct {
	Approved             bool
	Status               GateStatus
	WaitingForPhaseSplit bool
}

type ApprovalIndex map[SubjectType]map[string]Approval

func NewApprovalIndex(approvals []Approval) ApprovalIndex {
	idx := make(ApprovalIndex)
	for _, a := range approvals {
		if idx[a.SubjectType] == nil {
			idx[a.SubjectType] = make(map[string]Approval)
		}
		existing, ok := idx[a.SubjectType][a.SubjectID]
		if !ok {
			idx[a.SubjectType][a.SubjectID] = a
			continue
		}
		if a.CreatedAt.After(existing.CreatedAt) || (a.CreatedAt.Equal(existing.CreatedAt) && a.ID > existing.ID) {
			idx[a.SubjectType][a.SubjectID] = a
		}
	}
	return idx
}

func (idx ApprovalIndex) GetLatest(st SubjectType, id string) *Approval {
	if m, ok := idx[st]; ok {
		if a, ok := m[id]; ok {
			return &a
		}
	}
	return nil
}

func gateStatusFor(a *Approval) GateStatus {
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
	default:
		return GateStatusNone
	}
}

// Temporary stubs for FlowFor and PhasesFor
type FlowDefinition struct {
	ExecutionGates  []string
	OpenSpecProfile OpenSpecProfile
}

func FlowFor(reqType RequestType) FlowDefinition {
	var gates []string
	if reqType == RequestTypeChangeRequest {
		gates = []string{"plan", "phase"}
	} else {
		gates = []string{"plan"}
	}
	return FlowDefinition{
		ExecutionGates:  gates,
		OpenSpecProfile: OpenSpecProfileFor(reqType),
	}
}

func PhasesFor(size string) bool {
	return size == "L"
}

func ResolveTaskGate(in TaskGateInput) GateResolution {
	flow := FlowFor(in.Request.Type)
	needsPhase := false
	for _, g := range flow.ExecutionGates {
		if g == "phase" {
			needsPhase = true
			break
		}
	}

	var statuses []GateStatus

	if in.Container != nil && in.Container.Kind == "phase" {
		// Under a phase container
		if needsPhase {
			phaseStatus := gateStatusFor(in.Approvals.GetLatest(SubjectPhase, in.Container.ID))
			statuses = append(statuses, phaseStatus)
			if in.Plan != nil {
				planStatus := gateStatusFor(in.Approvals.GetLatest(SubjectPlan, in.Plan.ID))
				statuses = append(statuses, planStatus)
			}
		} else {
			if in.Plan != nil {
				planStatus := gateStatusFor(in.Approvals.GetLatest(SubjectPlan, in.Plan.ID))
				statuses = append(statuses, planStatus)
			}
		}
	} else if in.Container != nil && (in.Container.Kind == "plan" || in.Container.Kind == "task_list") {
		// Under a plan container
		if PhasesFor(in.Task.Size) {
			return GateResolution{WaitingForPhaseSplit: true, Approved: false, Status: GateStatusNone}
		}
		planStatus := gateStatusFor(in.Approvals.GetLatest(SubjectPlan, in.Container.ID))
		statuses = append(statuses, planStatus)
		if in.Task.Type == "security" {
			// security uses pre_deploy
			pdStatus := gateStatusFor(in.Approvals.GetLatest(SubjectPreDeploy, in.Container.ID))
			statuses = append(statuses, pdStatus)
		}
	} else {
		// No container (hotfix)
		pdStatus := gateStatusFor(in.Approvals.GetLatest(SubjectPreDeploy, in.Task.ID))
		statuses = append(statuses, pdStatus)
	}

	if len(statuses) == 0 {
		return GateResolution{Approved: true, Status: GateStatusNone}
	}

	overall := GateStatusApproved
	for _, s := range statuses {
		if s == GateStatusRejected {
			overall = GateStatusRejected
			break
		}
		if s == GateStatusPending || s == GateStatusNone {
			if overall != GateStatusRejected {
				overall = GateStatusPending
			}
		}
	}
	if overall == GateStatusPending && !hasStatus(statuses, GateStatusRejected) {
		// keep pending
	} else if overall != GateStatusRejected && hasStatus(statuses, GateStatusNone) {
		overall = GateStatusNone // wait... pending overrides none if there's an actual pending?
		// "nếu có pending thì pending, bản mới nhất rejected thì rejected, không có gì thì none, approved khi tất cả approved"
	}

	overall2 := GateStatusApproved
	hasPending := false
	hasRejected := false
	hasNone := false

	for _, s := range statuses {
		if s == GateStatusRejected {
			hasRejected = true
		} else if s == GateStatusPending {
			hasPending = true
		} else if s == GateStatusNone {
			hasNone = true
		}
	}

	if hasRejected {
		overall2 = GateStatusRejected
	} else if hasPending {
		overall2 = GateStatusPending
	} else if hasNone {
		overall2 = GateStatusNone
	}

	return GateResolution{
		Approved:             overall2 == GateStatusApproved,
		Status:               overall2,
		WaitingForPhaseSplit: false,
	}
}

func hasStatus(s []GateStatus, st GateStatus) bool {
	for _, x := range s {
		if x == st {
			return true
		}
	}
	return false
}
