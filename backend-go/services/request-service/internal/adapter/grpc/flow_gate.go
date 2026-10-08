package grpc

import (
	"context"
	"log/slog"
	"strings"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"google.golang.org/grpc"
)

// flowClass says how an RPC behaves while request_flow_enabled is off (CR-REQ-025 section 2.2).
type flowClass int

const (
	// classAdvance moves a Request forward or creates work; refused while the flow is off.
	classAdvance flowClass = iota
	// classRead only returns data, so users can still see history.
	classRead
	// classSafeExit lets a user leave the flow (cancel, return to backlog, reject).
	classSafeExit
	// classInternal is called by other services or consumers and must not lose results when the flag flips.
	classInternal
	// classAdmin is configuration, compliance and the flag itself; it works before and after enabling.
	classAdmin
)

// requestPackagePrefix is the proto package whose RPCs the flag governs.
const requestPackagePrefix = "/orca.request.v1."

const (
	requestSvc  = "/orca.request.v1.RequestService/"
	approvalSvc = "/orca.request.v1.ApprovalService/"
	policySvc   = "/orca.request.v1.ApprovalPolicyAdminService/"
	aiAdminSvc  = "/orca.request.v1.AiBudgetAdminService/"
)

// flowMethodClass has one row per RPC of every service in orca.request.v1; flow_gate_test.go fails when a
// proto RPC has no row, so a new RPC cannot ship unclassified.
var flowMethodClass = map[string]flowClass{
	// RequestService: reads.
	requestSvc + "GetRequest":                       classRead,
	requestSvc + "ListRequests":                     classRead,
	requestSvc + "ListBacklog":                      classRead,
	requestSvc + "ListRequestTypeHistory":           classRead,
	requestSvc + "ListRequestLinks":                 classRead,
	requestSvc + "GetRequestFlow":                   classRead,
	requestSvc + "GetRequestFlowSettings":           classRead,
	requestSvc + "ListSolutions":                    classRead,
	requestSvc + "GetProjectEngineSettings":         classRead,
	requestSvc + "GetPlanProposal":                  classRead,
	requestSvc + "ListRequestChecks":                classRead,
	requestSvc + "GetRequestReadiness":              classRead,
	requestSvc + "ListClarifications":               classRead,
	requestSvc + "GetClarification":                 classRead,
	requestSvc + "ListPendingClarificationsForUser": classRead,
	requestSvc + "ListDecisions":                    classRead,
	requestSvc + "GetDecision":                      classRead,
	requestSvc + "ListRequestRevisions":             classRead,
	requestSvc + "GetRequestRevision":               classRead,
	requestSvc + "GetRequestCoverage":               classRead,
	requestSvc + "GetArtifactGraph":                 classRead,
	requestSvc + "ExportArtifactProjection":         classRead,
	requestSvc + "ResolveArtifactRef":               classRead,
	requestSvc + "GetImpactAssessment":              classRead,
	requestSvc + "GetImpactGraph":                   classRead,
	requestSvc + "CompareImpact":                    classRead,
	requestSvc + "ListImpactFindings":               classRead,
	requestSvc + "GetImpactEvidence":                classRead,
	requestSvc + "GetPlanRiskHeatmap":               classRead,
	requestSvc + "GetImpactDrift":                   classRead,
	requestSvc + "GetRiskPolicy":                    classRead,
	requestSvc + "GetReadinessReport":               classRead,
	requestSvc + "ListReadiness":                    classRead,
	requestSvc + "ListContextSources":               classRead,
	requestSvc + "PreviewContextPack":               classRead,
	requestSvc + "GetEvidence":                      classRead,
	requestSvc + "ListEvidence":                     classRead,

	// RequestService: leaving the flow.
	requestSvc + "CancelRequest":       classSafeExit,
	requestSvc + "ReturnToBacklog":     classSafeExit,
	requestSvc + "CancelClarification": classSafeExit,

	// RequestService: other services call these; a late task result must still be recorded.
	requestSvc + "ReportTaskOutcome":     classInternal,
	requestSvc + "LookupRequestBySource": classInternal,

	// RequestService: configuration and compliance, independent of the flow.
	requestSvc + "SetRequestFlowSettings":   classAdmin,
	requestSvc + "SetProjectEngineSettings": classAdmin,
	requestSvc + "SetRiskPolicy":            classAdmin,
	requestSvc + "UpsertContextSource":      classAdmin,
	requestSvc + "SetContextSourceStatus":   classAdmin,
	requestSvc + "EraseRequest":             classAdmin,
	requestSvc + "ExportRequest":            classAdmin,
	requestSvc + "ExportTenantRequests":     classAdmin,

	// RequestService: moving a Request forward.
	requestSvc + "CreateRequest":           classAdvance,
	requestSvc + "ClassifyRequest":         classAdvance,
	requestSvc + "ConfirmRequestType":      classAdvance,
	requestSvc + "ChangeRequestType":       classAdvance,
	requestSvc + "ReopenRequest":           classAdvance,
	requestSvc + "SpawnChildRequest":       classAdvance,
	requestSvc + "GenerateSolution":        classAdvance,
	requestSvc + "ChooseSolutionOption":    classAdvance,
	requestSvc + "GeneratePlan":            classAdvance,
	requestSvc + "CommitPlan":              classAdvance,
	requestSvc + "StartPhase":              classAdvance,
	requestSvc + "RecordRequestCheck":      classAdvance,
	requestSvc + "RequestClarification":    classAdvance,
	requestSvc + "AnswerClarification":     classAdvance,
	requestSvc + "WaiveReadiness":          classAdvance,
	requestSvc + "ConfirmDecision":         classAdvance,
	requestSvc + "EditRequestContent":      classAdvance,
	requestSvc + "RequestImpactAssessment": classAdvance,
	requestSvc + "AcceptRisk":              classAdvance,
	requestSvc + "OverrideRiskGate":        classAdvance,
	requestSvc + "CheckReadiness":          classAdvance,

	// ApprovalService.
	approvalSvc + "GetApproval":        classRead,
	approvalSvc + "ListApprovals":      classRead,
	approvalSvc + "ListPendingForUser": classRead,
	approvalSvc + "Reject":             classSafeExit,
	approvalSvc + "Cancel":             classSafeExit,
	approvalSvc + "RequestApproval":    classAdvance,
	approvalSvc + "Approve":            classAdvance,
	approvalSvc + "ExtendApproval":     classAdvance,

	// ApprovalPolicyAdminService and AiBudgetAdminService: configuration.
	policySvc + "ListApprovalPolicies": classRead,
	policySvc + "UpsertApprovalPolicy": classAdmin,
	policySvc + "DeleteApprovalPolicy": classAdmin,
	aiAdminSvc + "ListAiBudgets":       classRead,
	aiAdminSvc + "ListAiStepPolicies":  classRead,
	aiAdminSvc + "GetAiTenantSettings": classRead,
	aiAdminSvc + "UpsertAiBudget":      classAdmin,
	aiAdminSvc + "DeleteAiBudget":      classAdmin,
	aiAdminSvc + "UpsertAiStepPolicy":  classAdmin,
	aiAdminSvc + "SetAiTenantSettings": classAdmin,
}

// flowGate decides one call. A method missing from the table is treated as advancing, so a new RPC is
// closed by default while the flow is off.
func flowGate(ctx context.Context, fullMethod string, effective func(context.Context) (bool, error)) error {
	// Health checks, reflection and any other infrastructure service are not part of the Request flow.
	if !strings.HasPrefix(fullMethod, requestPackagePrefix) {
		return nil
	}
	class, known := flowMethodClass[fullMethod]
	if !known {
		slog.WarnContext(ctx, "flow gate: RPC has no classification, treating as advancing", slog.String("method", fullMethod))
	}
	if class != classAdvance {
		return nil
	}
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return apperrors.ToGRPCStatus(domain.ErrRequestTenantRequired())
	}
	on, err := effective(ctx)
	if err != nil || !on {
		return apperrors.ToGRPCStatus(domain.ErrFlowDisabled())
	}
	return nil
}

// FlowGate refuses advancing RPCs unless effective reports the tenant's flow as enabled. A read error
// refuses too (fail closed). Install it after tenant extraction: the flag is per tenant.
// Expiry sweepers and outbox consumers do not go through gRPC and never consult the flag.
func FlowGate(effective func(ctx context.Context) (bool, error)) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if err := flowGate(ctx, info.FullMethod, effective); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// StreamFlowGate is FlowGate for streaming RPCs (only ExportTenantRequests today, an admin read).
func StreamFlowGate(effective func(ctx context.Context) (bool, error)) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if err := flowGate(ss.Context(), info.FullMethod, effective); err != nil {
			return err
		}
		return handler(srv, ss)
	}
}
