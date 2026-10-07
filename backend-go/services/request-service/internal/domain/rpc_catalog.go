package domain

import (
	"fmt"
	"strings"

	"google.golang.org/grpc"
)

type Group string

const (
	GroupRead          Group = "read"
	GroupCreate        Group = "create"
	GroupTriage        Group = "triage"
	GroupSolution      Group = "solution"
	GroupDecide        Group = "decide"
	GroupExecute       Group = "execute"
	GroupAdmin         Group = "admin"
	GroupLifecycle     Group = "lifecycle"
	GroupPlan          Group = "plan"
	GroupAuthenticated Group = "authenticated"
	GroupInternal      Group = "internal"
)

type Locator string

const (
	LocNone       Locator = "none"
	LocRequestID  Locator = "request_id"
	LocApprovalID Locator = "approval_id"
	LocProjectID  Locator = "project_id"
)

type Entry struct {
	Group        Group
	Locator      Locator
	RateClass    string
	AgentAllowed bool
}

// Catalog defines the access control mapping for every RPC exposed by request-service.
// It maps the full gRPC method name to its required permission group and locator.
var Catalog = map[string]Entry{
	// RequestService - Read
	"/orca.request.v1.RequestService/GetRequest":          {Group: GroupRead, Locator: LocRequestID, RateClass: "read", AgentAllowed: true},
	"/orca.request.v1.RequestService/ListRequests":        {Group: GroupRead, Locator: LocProjectID, RateClass: "read", AgentAllowed: true},
	"/orca.request.v1.RequestService/ListBacklog":         {Group: GroupRead, Locator: LocProjectID, RateClass: "read", AgentAllowed: true},
	"/orca.request.v1.RequestService/ListRequestTimeline": {Group: GroupRead, Locator: LocRequestID, RateClass: "read", AgentAllowed: true},
	"/orca.request.v1.RequestService/GetSolution":         {Group: GroupRead, Locator: LocRequestID, RateClass: "read", AgentAllowed: true},
	"/orca.request.v1.RequestService/ListSolutions":       {Group: GroupRead, Locator: LocRequestID, RateClass: "read", AgentAllowed: true},
	"/orca.request.v1.RequestService/ListRequestLinks":    {Group: GroupRead, Locator: LocRequestID, RateClass: "read", AgentAllowed: true},
	"/orca.request.v1.RequestService/CheckSecretScanInfo": {Group: GroupRead, Locator: LocRequestID, RateClass: "read", AgentAllowed: true},
	"/orca.request.v1.RequestService/ExportTenantRequests":{Group: GroupAdmin, Locator: LocNone, RateClass: "read", AgentAllowed: false},

	// RequestService - Create
	"/orca.request.v1.RequestService/CreateRequest":     {Group: GroupCreate, Locator: LocProjectID, RateClass: "write", AgentAllowed: true},
	"/orca.request.v1.RequestService/SpawnChildRequest": {Group: GroupCreate, Locator: LocRequestID, RateClass: "write", AgentAllowed: true},

	// RequestService - Triage
	"/orca.request.v1.RequestService/ClassifyRequest":    {Group: GroupTriage, Locator: LocRequestID, RateClass: "ai", AgentAllowed: true},
	"/orca.request.v1.RequestService/ConfirmRequestType": {Group: GroupTriage, Locator: LocRequestID, RateClass: "write", AgentAllowed: false},

	// RequestService - Lifecycle
	"/orca.request.v1.RequestService/CancelRequest": {Group: GroupLifecycle, Locator: LocRequestID, RateClass: "write", AgentAllowed: false},

	// RequestService - Plan
	"/orca.request.v1.RequestService/GeneratePlan": {Group: GroupPlan, Locator: LocRequestID, RateClass: "ai", AgentAllowed: false},

	// RequestService - Solution
	"/orca.request.v1.RequestService/ProposeSolution":      {Group: GroupSolution, Locator: LocRequestID, RateClass: "write", AgentAllowed: true},
	"/orca.request.v1.RequestService/UpdateSolution":       {Group: GroupSolution, Locator: LocRequestID, RateClass: "write", AgentAllowed: true},
	"/orca.request.v1.RequestService/MarkSolutionApproved": {Group: GroupSolution, Locator: LocRequestID, RateClass: "write", AgentAllowed: true},
	"/orca.request.v1.RequestService/GenerateSolution":     {Group: GroupSolution, Locator: LocRequestID, RateClass: "ai", AgentAllowed: false},

	// RequestService - Execute
	"/orca.request.v1.RequestService/RecordExecutionPlan":   {Group: GroupExecute, Locator: LocRequestID, RateClass: "write", AgentAllowed: true},
	"/orca.request.v1.RequestService/RecordExecutionResult": {Group: GroupExecute, Locator: LocRequestID, RateClass: "write", AgentAllowed: true},
	"/orca.request.v1.RequestService/StartPhase":            {Group: GroupExecute, Locator: LocRequestID, RateClass: "ai", AgentAllowed: false},

	// ApprovalService
	"/orca.request.v1.ApprovalService/Approve":            {Group: GroupDecide, Locator: LocApprovalID, RateClass: "write", AgentAllowed: false},
	"/orca.request.v1.ApprovalService/Reject":             {Group: GroupDecide, Locator: LocApprovalID, RateClass: "write", AgentAllowed: false},
	"/orca.request.v1.ApprovalService/Cancel":             {Group: GroupDecide, Locator: LocApprovalID, RateClass: "write", AgentAllowed: false},
	"/orca.request.v1.ApprovalService/GetApproval":        {Group: GroupRead, Locator: LocApprovalID, RateClass: "read", AgentAllowed: true},
	"/orca.request.v1.ApprovalService/ListApprovals":      {Group: GroupRead, Locator: LocRequestID, RateClass: "read", AgentAllowed: true},
	"/orca.request.v1.ApprovalService/ListPendingForUser": {Group: GroupAuthenticated, Locator: LocNone, RateClass: "read", AgentAllowed: false},
	"/orca.request.v1.ApprovalService/RequestApproval":    {Group: GroupInternal, Locator: LocRequestID, RateClass: "write", AgentAllowed: false},

	// AiBudgetAdminService
	"/orca.request.v1.AiBudgetAdminService/SetRequestFlowSettings": {Group: GroupAdmin, Locator: LocNone, RateClass: "write", AgentAllowed: false},
}

// ValidateCatalog ensures all methods in the service descriptions are present in the Catalog,
// and vice-versa (except maybe unimplemented ones).
func ValidateCatalog(descs ...*grpc.ServiceDesc) error {
	descMap := make(map[string]bool)
	for _, desc := range descs {
		for _, m := range desc.Methods {
			descMap[fmt.Sprintf("/%s/%s", desc.ServiceName, m.MethodName)] = true
		}
		for _, s := range desc.Streams {
			descMap[fmt.Sprintf("/%s/%s", desc.ServiceName, s.StreamName)] = true
		}
	}

	var missing []string
	for method := range descMap {
		if _, ok := Catalog[method]; !ok {
			missing = append(missing, method)
		}
	}

	var extra []string
	for method := range Catalog {
		if !descMap[method] {
			extra = append(extra, method)
		}
	}

	if len(missing) > 0 || len(extra) > 0 {
		return fmt.Errorf("catalog mismatch: missing=%v, extra=%v", missing, extra)
	}

	return nil
}

// PublicMethods returns all RPCs not marked as GroupInternal.
func PublicMethods() []string {
	var methods []string
	for m, e := range Catalog {
		if e.Group != GroupInternal {
			methods = append(methods, m)
		}
	}
	return methods
}

// InternalMethods returns all RPCs marked as GroupInternal.
func InternalMethods() []string {
	var methods []string
	for m, e := range Catalog {
		if e.Group == GroupInternal {
			methods = append(methods, m)
		}
	}
	return methods
}

// RPCName extracts the method name from a full gRPC method path.
func RPCName(fullMethod string) string {
	idx := strings.LastIndex(fullMethod, "/")
	if idx == -1 {
		return fullMethod
	}
	return fullMethod[idx+1:]
}
