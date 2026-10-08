package domain

import (
	"fmt"
	"sort"
	"strings"

	"google.golang.org/grpc"
)

// Group is the action group of an RPC; request.rego maps (group, caller roles) to allow.
type Group string

const (
	GroupRead          Group = "read"
	GroupCreate        Group = "create"
	GroupTriage        Group = "triage"
	GroupAnalyze       Group = "analyze"
	GroupPlan          Group = "plan"
	GroupExecute       Group = "execute"
	GroupLifecycle     Group = "lifecycle"
	GroupDecide        Group = "decide"
	GroupAdmin         Group = "admin"
	GroupAuthenticated Group = "authenticated"
	GroupInternal      Group = "internal"
)

// Locator says how the interceptor finds the Request (and so the project and reporter) an RPC acts on.
type Locator string

const (
	LocNone       Locator = "none"
	LocRequestID  Locator = "request_id"
	LocApprovalID Locator = "approval_id"
	LocProjectID  Locator = "project_id"
	// LocEntityID is an id of a Request child (clarification, decision, finding...); a resolver per
	// Entity kind maps it to its Request. Without a resolver a non-admin is refused (fail closed).
	LocEntityID Locator = "entity_id"
)

// Entry classifies one RPC.
type Entry struct {
	Group   Group
	Locator Locator
	// Field is the string field of the request message that carries the id the Locator needs.
	Field string
	// Entity names the child kind for LocEntityID.
	Entity       string
	RateClass    string
	AgentAllowed bool
}

const (
	reqSvc   = "/orca.request.v1.RequestService/"
	apprSvc  = "/orca.request.v1.ApprovalService/"
	policSvc = "/orca.request.v1.ApprovalPolicyAdminService/"
	budgSvc  = "/orca.request.v1.AiBudgetAdminService/"
)

func onRequest(g Group, field, rate string, agent bool) Entry {
	return Entry{Group: g, Locator: LocRequestID, Field: field, RateClass: rate, AgentAllowed: agent}
}

func onProject(g Group, field, rate string, agent bool) Entry {
	return Entry{Group: g, Locator: LocProjectID, Field: field, RateClass: rate, AgentAllowed: agent}
}

func onEntity(g Group, entity, field, rate string) Entry {
	return Entry{Group: g, Locator: LocEntityID, Entity: entity, Field: field, RateClass: rate}
}

func unlocated(g Group, rate string, agent bool) Entry {
	return Entry{Group: g, Locator: LocNone, RateClass: rate, AgentAllowed: agent}
}

// Catalog maps every RPC of request-service to its action group. AgentAllowed must equal
// request.rego's agent_rpcs (checked by a test against the real policy). Agents (MCP sessions)
// may read, create and classify; ChangeRequestType, ReturnToBacklog and ReopenRequest are allowed
// because the gateway ships them as reversible MCP tools; deciding, planning, executing,
// cancelling, administering and erasing stay human-only.
var Catalog = map[string]Entry{
	reqSvc + "GetRequest":                       onRequest(GroupRead, "id", RateRead, true),
	reqSvc + "ListRequests":                     onProject(GroupRead, "project_id", RateRead, true),
	reqSvc + "ListBacklog":                      onProject(GroupRead, "project_id", RateRead, true),
	reqSvc + "CreateRequest":                    onProject(GroupCreate, "project_id", RateWrite, true),
	reqSvc + "ClassifyRequest":                  onRequest(GroupTriage, "request_id", RateAI, true),
	reqSvc + "ConfirmRequestType":               onRequest(GroupTriage, "request_id", RateWrite, false),
	reqSvc + "ChangeRequestType":                onRequest(GroupTriage, "request_id", RateWrite, true),
	reqSvc + "ListRequestTypeHistory":           onRequest(GroupRead, "request_id", RateRead, true),
	reqSvc + "ReturnToBacklog":                  onRequest(GroupLifecycle, "request_id", RateWrite, true),
	reqSvc + "ReopenRequest":                    onRequest(GroupLifecycle, "request_id", RateWrite, true),
	reqSvc + "CancelRequest":                    onRequest(GroupLifecycle, "request_id", RateWrite, false),
	reqSvc + "SpawnChildRequest":                onRequest(GroupCreate, "parent_request_id", RateWrite, true),
	reqSvc + "ListRequestLinks":                 onRequest(GroupRead, "request_id", RateRead, true),
	reqSvc + "GetRequestFlow":                   unlocated(GroupAuthenticated, RateRead, true),
	reqSvc + "LookupRequestBySource":            unlocated(GroupInternal, RateNone, false),
	reqSvc + "GetRequestFlowSettings":           unlocated(GroupAuthenticated, RateRead, true),
	reqSvc + "SetRequestFlowSettings":           unlocated(GroupAdmin, RateWrite, false),
	reqSvc + "GenerateSolution":                 onRequest(GroupAnalyze, "request_id", RateAI, true),
	reqSvc + "ListSolutions":                    onRequest(GroupRead, "request_id", RateRead, true),
	reqSvc + "ChooseSolutionOption":             onRequest(GroupAnalyze, "request_id", RateWrite, false),
	reqSvc + "GetProjectEngineSettings":         onProject(GroupRead, "project_id", RateRead, false),
	reqSvc + "SetProjectEngineSettings":         unlocated(GroupAdmin, RateWrite, false),
	reqSvc + "GeneratePlan":                     onRequest(GroupPlan, "request_id", RateAI, false),
	reqSvc + "GetPlanProposal":                  onRequest(GroupRead, "request_id", RateRead, false),
	reqSvc + "CommitPlan":                       onRequest(GroupPlan, "request_id", RateWrite, false),
	reqSvc + "StartPhase":                       onRequest(GroupExecute, "request_id", RateAI, false),
	reqSvc + "ReportTaskOutcome":                unlocated(GroupInternal, RateNone, false),
	reqSvc + "RecordRequestCheck":               unlocated(GroupInternal, RateNone, false),
	reqSvc + "ListRequestChecks":                onRequest(GroupRead, "request_id", RateRead, false),
	reqSvc + "GetRequestReadiness":              onRequest(GroupRead, "request_id", RateRead, false),
	reqSvc + "RequestClarification":             unlocated(GroupInternal, RateNone, false),
	reqSvc + "ListClarifications":               onRequest(GroupRead, "request_id", RateRead, false),
	reqSvc + "GetClarification":                 onEntity(GroupRead, "clarification", "id", RateRead),
	reqSvc + "AnswerClarification":              onEntity(GroupTriage, "clarification", "clarification_id", RateWrite),
	reqSvc + "CancelClarification":              onEntity(GroupTriage, "clarification", "id", RateWrite),
	reqSvc + "ListPendingClarificationsForUser": unlocated(GroupAuthenticated, RateRead, false),
	reqSvc + "WaiveReadiness":                   onRequest(GroupExecute, "request_id", RateWrite, false),
	reqSvc + "ListDecisions":                    onRequest(GroupRead, "request_id", RateRead, false),
	reqSvc + "GetDecision":                      onEntity(GroupRead, "decision", "id", RateRead),
	reqSvc + "ConfirmDecision":                  onEntity(GroupLifecycle, "decision", "decision_id", RateWrite),
	reqSvc + "EditRequestContent":               onRequest(GroupLifecycle, "request_id", RateWrite, false),
	reqSvc + "ListRequestRevisions":             onRequest(GroupRead, "request_id", RateRead, false),
	reqSvc + "GetRequestRevision":               onRequest(GroupRead, "request_id", RateRead, false),
	reqSvc + "GetRequestCoverage":               onRequest(GroupRead, "request_id", RateRead, false),
	reqSvc + "GetArtifactGraph":                 onRequest(GroupRead, "request_id", RateRead, false),
	reqSvc + "ExportArtifactProjection":         onRequest(GroupRead, "request_id", RateRead, false),
	reqSvc + "ResolveArtifactRef":               onEntity(GroupRead, "artifact_ref", "ref", RateRead),
	reqSvc + "RequestImpactAssessment":          onRequest(GroupAnalyze, "request_id", RateAI, false),
	reqSvc + "GetImpactAssessment":              onEntity(GroupRead, "impact_subject", "subject_id", RateRead),
	reqSvc + "GetImpactGraph":                   onRequest(GroupRead, "request_id", RateRead, false),
	reqSvc + "CompareImpact":                    onEntity(GroupRead, "solution", "solution_id", RateRead),
	reqSvc + "ListImpactFindings":               onEntity(GroupRead, "impact_assessment", "assessment_id", RateRead),
	reqSvc + "GetImpactEvidence":                onEntity(GroupRead, "impact_finding", "finding_id", RateRead),
	reqSvc + "GetPlanRiskHeatmap":               onEntity(GroupRead, "plan_task", "plan_task_id", RateRead),
	reqSvc + "GetImpactDrift":                   onEntity(GroupRead, "phase", "phase_id", RateRead),
	reqSvc + "AcceptRisk":                       unlocated(GroupDecide, RateWrite, false),
	reqSvc + "OverrideRiskGate":                 onRequest(GroupDecide, "request_id", RateWrite, false),
	reqSvc + "GetRiskPolicy":                    unlocated(GroupAuthenticated, RateRead, false),
	reqSvc + "SetRiskPolicy":                    unlocated(GroupAdmin, RateWrite, false),
	reqSvc + "CheckReadiness":                   onEntity(GroupExecute, "task", "task_id", RateWrite),
	reqSvc + "GetReadinessReport":               onEntity(GroupRead, "task", "task_id", RateRead),
	reqSvc + "ListReadiness":                    onEntity(GroupRead, "phase", "phase_id", RateRead),
	reqSvc + "ListContextSources":               unlocated(GroupAdmin, RateRead, false),
	reqSvc + "UpsertContextSource":              unlocated(GroupAdmin, RateWrite, false),
	reqSvc + "SetContextSourceStatus":           unlocated(GroupAdmin, RateWrite, false),
	reqSvc + "PreviewContextPack":               unlocated(GroupAdmin, RateRead, false),
	reqSvc + "GetEvidence":                      onEntity(GroupRead, "evidence", "id", RateRead),
	reqSvc + "ListEvidence":                     onRequest(GroupRead, "request_id", RateRead, false),
	reqSvc + "EraseRequest":                     unlocated(GroupAdmin, RateWrite, false),
	reqSvc + "ExportRequest":                    unlocated(GroupAdmin, RateWrite, false),
	reqSvc + "ExportTenantRequests":             unlocated(GroupAdmin, RateWrite, false),

	apprSvc + "RequestApproval":    unlocated(GroupInternal, RateNone, false),
	apprSvc + "Approve":            {Group: GroupDecide, Locator: LocApprovalID, Field: "id", RateClass: RateWrite},
	apprSvc + "Reject":             {Group: GroupDecide, Locator: LocApprovalID, Field: "id", RateClass: RateWrite},
	apprSvc + "Cancel":             {Group: GroupDecide, Locator: LocApprovalID, Field: "id", RateClass: RateWrite},
	apprSvc + "ExtendApproval":     {Group: GroupDecide, Locator: LocApprovalID, Field: "id", RateClass: RateWrite},
	apprSvc + "GetApproval":        {Group: GroupRead, Locator: LocApprovalID, Field: "id", RateClass: RateRead, AgentAllowed: true},
	apprSvc + "ListApprovals":      onRequest(GroupRead, "request_id", RateRead, true),
	apprSvc + "ListPendingForUser": unlocated(GroupAuthenticated, RateRead, true),

	policSvc + "ListApprovalPolicies": unlocated(GroupAdmin, RateRead, false),
	policSvc + "UpsertApprovalPolicy": unlocated(GroupAdmin, RateWrite, false),
	policSvc + "DeleteApprovalPolicy": unlocated(GroupAdmin, RateWrite, false),

	budgSvc + "ListAiBudgets":       unlocated(GroupAdmin, RateRead, false),
	budgSvc + "UpsertAiBudget":      unlocated(GroupAdmin, RateWrite, false),
	budgSvc + "DeleteAiBudget":      unlocated(GroupAdmin, RateWrite, false),
	budgSvc + "ListAiStepPolicies":  unlocated(GroupAdmin, RateRead, false),
	budgSvc + "UpsertAiStepPolicy":  unlocated(GroupAdmin, RateWrite, false),
	budgSvc + "GetAiTenantSettings": unlocated(GroupAdmin, RateRead, false),
	budgSvc + "SetAiTenantSettings": unlocated(GroupAdmin, RateWrite, false),
}

// ValidateCatalog fails when a method of descs has no Entry or an Entry names no method, so a new RPC
// cannot ship unclassified (and so unprotected).
func ValidateCatalog(descs ...*grpc.ServiceDesc) error {
	have := make(map[string]bool)
	for _, desc := range descs {
		for _, m := range desc.Methods {
			have[fmt.Sprintf("/%s/%s", desc.ServiceName, m.MethodName)] = true
		}
		for _, s := range desc.Streams {
			have[fmt.Sprintf("/%s/%s", desc.ServiceName, s.StreamName)] = true
		}
	}
	var missing, extra []string
	for m := range have {
		if _, ok := Catalog[m]; !ok {
			missing = append(missing, m)
		}
	}
	for m := range Catalog {
		if !have[m] {
			extra = append(extra, m)
		}
	}
	if len(missing) == 0 && len(extra) == 0 {
		return nil
	}
	sort.Strings(missing)
	sort.Strings(extra)
	return fmt.Errorf("rpc catalog mismatch: unclassified=%v unknown=%v", missing, extra)
}

// PublicMethods are the RPCs reachable by the gateway token; sorted for stable wiring and tests.
func PublicMethods() []string {
	return methodsWhere(func(e Entry) bool { return e.Group != GroupInternal })
}

// InternalMethods are the RPCs reserved for sibling services (service token).
func InternalMethods() []string {
	return methodsWhere(func(e Entry) bool { return e.Group == GroupInternal })
}

func methodsWhere(keep func(Entry) bool) []string {
	var out []string
	for m, e := range Catalog {
		if keep(e) {
			out = append(out, m)
		}
	}
	sort.Strings(out)
	return out
}

// RPCName is the method part of a full gRPC method ("/pkg.Svc/Method" -> "Method").
func RPCName(fullMethod string) string {
	if i := strings.LastIndex(fullMethod, "/"); i >= 0 {
		return fullMethod[i+1:]
	}
	return fullMethod
}

// GuardedServicePrefix selects the services the catalog governs. Health and reflection are registered beside
// them and carry no tenant data; an unknown method under this prefix is refused, not waved through.
const GuardedServicePrefix = "/orca.request.v1."

func IsGuardedMethod(fullMethod string) bool {
	return strings.HasPrefix(fullMethod, GuardedServicePrefix)
}
