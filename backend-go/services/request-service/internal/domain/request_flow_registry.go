package domain

// AnalysisKind is what the analyzing stage produces for a request type.
type AnalysisKind string

const (
	AnalysisNone      AnalysisKind = "none"
	AnalysisSolution  AnalysisKind = "solution"
	AnalysisDiagnosis AnalysisKind = "diagnosis"
	AnalysisFindings  AnalysisKind = "findings"
	AnalysisAnswer    AnalysisKind = "answer"
)

type PlanKind string

const (
	PlanNone       PlanKind = "none"
	PlanPlan       PlanKind = "plan"
	PlanTaskList   PlanKind = "task_list"
	PlanSingleTask PlanKind = "single_task"
)

type PhaseRule string

const (
	PhaseNever     PhaseRule = "never"
	PhaseAlways    PhaseRule = "always"
	PhaseWhenSizeL PhaseRule = "when_size_L"
)

// GateSubject names the Approval subject that blocks a stage. diagnosis maps to
// GateSolution because the README has no `diagnosis` subject.
type GateSubject string

const (
	GateNone      GateSubject = ""
	GateSolution  GateSubject = "solution"
	GateFindings  GateSubject = "findings"
	GateAnswer    GateSubject = "answer"
	GatePlan      GateSubject = "plan"
	GateTaskList  GateSubject = "task_list"
	GatePreDeploy GateSubject = "pre_deploy"
	GatePhase     GateSubject = "phase"
)

// FlowDefinition describes which stages a request type goes through (README v6 section 3.4).
type FlowDefinition struct {
	Type                 RequestType
	HumanConfirmRequired bool
	AnalysisKind         AnalysisKind
	// AnalysisGate is empty when analysis ends without an approval (hotfix).
	AnalysisGate GateSubject
	PlanKind     PlanKind
	PhaseRule    PhaseRule
	// StartGate is the approval that occupies awaiting_plan_approval.
	StartGate GateSubject
	// ExecutionGates block inside executing without changing the request status.
	ExecutionGates         []GateSubject
	CompletesAfterAnalysis bool
	OpenSpecProfile        OpenSpecProfile
}

var flowDefinitions = map[RequestType]FlowDefinition{
	RequestTypeChangeRequest: {AnalysisKind: AnalysisSolution, AnalysisGate: GateSolution, PlanKind: PlanPlan, PhaseRule: PhaseAlways, StartGate: GatePlan, ExecutionGates: []GateSubject{GatePhase}},
	RequestTypeBug:           {AnalysisKind: AnalysisDiagnosis, AnalysisGate: GateSolution, PlanKind: PlanPlan, PhaseRule: PhaseWhenSizeL, StartGate: GatePlan},
	RequestTypeHotfix:        {HumanConfirmRequired: true, AnalysisKind: AnalysisDiagnosis, PlanKind: PlanSingleTask, PhaseRule: PhaseNever, StartGate: GatePreDeploy},
	RequestTypeTask:          {AnalysisKind: AnalysisNone, PlanKind: PlanTaskList, PhaseRule: PhaseNever, StartGate: GateTaskList},
	RequestTypeSpike:         {AnalysisKind: AnalysisFindings, AnalysisGate: GateFindings, PlanKind: PlanNone, PhaseRule: PhaseNever, CompletesAfterAnalysis: true},
	RequestTypeQuestion:      {AnalysisKind: AnalysisAnswer, AnalysisGate: GateAnswer, PlanKind: PlanNone, PhaseRule: PhaseNever, CompletesAfterAnalysis: true},
	RequestTypeRefactor:      {AnalysisKind: AnalysisSolution, AnalysisGate: GateSolution, PlanKind: PlanPlan, PhaseRule: PhaseWhenSizeL, StartGate: GatePlan},
	RequestTypeSecurity:      {HumanConfirmRequired: true, AnalysisKind: AnalysisDiagnosis, AnalysisGate: GateSolution, PlanKind: PlanPlan, PhaseRule: PhaseNever, StartGate: GatePreDeploy},
	RequestTypePerformance:   {AnalysisKind: AnalysisDiagnosis, AnalysisGate: GateSolution, PlanKind: PlanPlan, PhaseRule: PhaseNever, StartGate: GatePlan},
	RequestTypeDocs:          {AnalysisKind: AnalysisNone, PlanKind: PlanTaskList, PhaseRule: PhaseNever, StartGate: GateTaskList},
	RequestTypeOpsRequest:    {AnalysisKind: AnalysisNone, PlanKind: PlanPlan, PhaseRule: PhaseNever, StartGate: GatePlan, ExecutionGates: []GateSubject{GatePreDeploy}},
}

// FlowFor returns a copy so callers cannot mutate the registry's ExecutionGates slice.
func FlowFor(t RequestType) (FlowDefinition, error) {
	f, ok := flowDefinitions[t]
	if !ok {
		return FlowDefinition{}, ErrFlowUnknownType(t)
	}
	f.Type = t
	f.OpenSpecProfile = OpenSpecProfileFor(t)
	f.ExecutionGates = append([]GateSubject(nil), f.ExecutionGates...)
	return f, nil
}

// PhasesFor reports whether the request splits into Phases; a missing size is treated as not L.
func (f FlowDefinition) PhasesFor(size RequestSize) bool {
	switch f.PhaseRule {
	case PhaseAlways:
		return true
	case PhaseWhenSizeL:
		return size == RequestSizeL
	default:
		return false
	}
}

// HasExecutionGate reports whether g blocks inside executing for this flow.
func (f FlowDefinition) HasExecutionGate(g GateSubject) bool {
	for _, x := range f.ExecutionGates {
		if x == g {
			return true
		}
	}
	return false
}

// AllFlowTypes lists the registered types in README order.
func AllFlowTypes() []RequestType {
	return []RequestType{
		RequestTypeChangeRequest, RequestTypeBug, RequestTypeHotfix, RequestTypeTask, RequestTypeSpike, RequestTypeQuestion,
		RequestTypeRefactor, RequestTypeSecurity, RequestTypePerformance, RequestTypeDocs, RequestTypeOpsRequest,
	}
}
