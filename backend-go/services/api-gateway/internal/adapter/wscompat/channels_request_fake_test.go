package wscompat

import (
	"context"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

// requestCall is one RPC the fake saw.
type requestCall struct {
	rpc      string
	in       proto.Message
	deadline time.Duration // remaining budget when the RPC started
	md       metadata.MD
}

// fakeRequestClient implements both Request clients. Embedded nil interfaces make
// any RPC a test did not expect panic. A non-nil err is returned by every RPC.
type fakeRequestClient struct {
	requestv1.RequestServiceClient
	requestv1.ApprovalServiceClient

	mu    sync.Mutex
	calls []requestCall
	err   error
	// empty makes every list RPC return an empty page.
	empty bool
}

func (f *fakeRequestClient) rec(ctx context.Context, rpc string, in proto.Message) {
	var left time.Duration
	if d, ok := ctx.Deadline(); ok {
		left = time.Until(d)
	}
	md, _ := metadata.FromOutgoingContext(ctx)
	f.mu.Lock()
	f.calls = append(f.calls, requestCall{rpc: rpc, in: in, deadline: left, md: md})
	f.mu.Unlock()
}

func (f *fakeRequestClient) last() requestCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) == 0 {
		return requestCall{}
	}
	return f.calls[len(f.calls)-1]
}

func (f *fakeRequestClient) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

var fakeStamp = timestamppb.New(time.Date(2026, 10, 8, 1, 2, 3, 0, time.UTC))

func fakeRequestMsg() *requestv1.Request {
	conf := 0.9
	return &requestv1.Request{
		Id: "r1", ProjectId: "p1", Number: 7, Title: "T", Body: "B", SourceProvider: "manual", Type: "bug", TypeSource: "ai",
		Size: "S", Urgency: "normal", Confidence: &conf, Status: "analyzing", ReporterId: "u1", CreatedAt: fakeStamp, UpdatedAt: fakeStamp, Version: 3,
	}
}

func fakeApprovalMsg() *requestv1.Approval {
	return &requestv1.Approval{
		Id: "a1", RequestId: "r1", SubjectType: requestv1.ApprovalSubjectType_APPROVAL_SUBJECT_TYPE_SOLUTION, SubjectId: "s1",
		Status: requestv1.ApprovalStatus_APPROVAL_STATUS_PENDING, RequestedBy: "u1", Version: 2, SubjectDigest: "dg", CreatedAt: fakeStamp, DueAt: fakeStamp,
		RequestTitle: "T", RequestType: "bug", RequestNumber: 7,
	}
}

func (f *fakeRequestClient) CreateRequest(ctx context.Context, in *requestv1.CreateRequestRequest, _ ...grpc.CallOption) (*requestv1.CreateRequestResponse, error) {
	f.rec(ctx, "CreateRequest", in)
	return &requestv1.CreateRequestResponse{Request: fakeRequestMsg(), Created: true}, f.err
}

func (f *fakeRequestClient) GetRequest(ctx context.Context, in *requestv1.GetRequestRequest, _ ...grpc.CallOption) (*requestv1.GetRequestResponse, error) {
	f.rec(ctx, "GetRequest", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.GetRequestResponse{Request: fakeRequestMsg()}, nil
}

func (f *fakeRequestClient) ListRequests(ctx context.Context, in *requestv1.ListRequestsRequest, _ ...grpc.CallOption) (*requestv1.ListRequestsResponse, error) {
	f.rec(ctx, "ListRequests", in)
	if f.err != nil {
		return nil, f.err
	}
	if f.empty {
		return &requestv1.ListRequestsResponse{}, nil
	}
	return &requestv1.ListRequestsResponse{Requests: []*requestv1.Request{fakeRequestMsg()}, NextPageToken: "np"}, nil
}

func (f *fakeRequestClient) ListRequestTypeHistory(ctx context.Context, in *requestv1.ListRequestTypeHistoryRequest, _ ...grpc.CallOption) (*requestv1.ListRequestTypeHistoryResponse, error) {
	f.rec(ctx, "ListRequestTypeHistory", in)
	if f.err != nil {
		return nil, f.err
	}
	if f.empty {
		return &requestv1.ListRequestTypeHistoryResponse{}, nil
	}
	return &requestv1.ListRequestTypeHistoryResponse{Changes: []*requestv1.RequestTypeChange{{ToType: "bug", ActorKind: "ai", At: fakeStamp}}}, nil
}

func (f *fakeRequestClient) ClassifyRequest(ctx context.Context, in *requestv1.ClassifyRequestRequest, _ ...grpc.CallOption) (*requestv1.ClassifyRequestResponse, error) {
	f.rec(ctx, "ClassifyRequest", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.ClassifyRequestResponse{Request: fakeRequestMsg(), RunId: "run1"}, nil
}

func (f *fakeRequestClient) ConfirmRequestType(ctx context.Context, in *requestv1.ConfirmRequestTypeRequest, _ ...grpc.CallOption) (*requestv1.ConfirmRequestTypeResponse, error) {
	f.rec(ctx, "ConfirmRequestType", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.ConfirmRequestTypeResponse{Request: fakeRequestMsg()}, nil
}

func (f *fakeRequestClient) ChangeRequestType(ctx context.Context, in *requestv1.ChangeRequestTypeRequest, _ ...grpc.CallOption) (*requestv1.ChangeRequestTypeResponse, error) {
	f.rec(ctx, "ChangeRequestType", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.ChangeRequestTypeResponse{Request: fakeRequestMsg()}, nil
}

func (f *fakeRequestClient) ReturnToBacklog(ctx context.Context, in *requestv1.ReturnToBacklogRequest, _ ...grpc.CallOption) (*requestv1.ReturnToBacklogResponse, error) {
	f.rec(ctx, "ReturnToBacklog", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.ReturnToBacklogResponse{Request: fakeRequestMsg()}, nil
}

func (f *fakeRequestClient) ReopenRequest(ctx context.Context, in *requestv1.ReopenRequestRequest, _ ...grpc.CallOption) (*requestv1.ReopenRequestResponse, error) {
	f.rec(ctx, "ReopenRequest", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.ReopenRequestResponse{Request: fakeRequestMsg()}, nil
}

func (f *fakeRequestClient) CancelRequest(ctx context.Context, in *requestv1.CancelRequestRequest, _ ...grpc.CallOption) (*requestv1.CancelRequestResponse, error) {
	f.rec(ctx, "CancelRequest", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.CancelRequestResponse{Request: fakeRequestMsg()}, nil
}

func (f *fakeRequestClient) SpawnChildRequest(ctx context.Context, in *requestv1.SpawnChildRequestRequest, _ ...grpc.CallOption) (*requestv1.SpawnChildRequestResponse, error) {
	f.rec(ctx, "SpawnChildRequest", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.SpawnChildRequestResponse{Child: fakeRequestMsg(), Created: true}, nil
}

func (f *fakeRequestClient) GeneratePlan(ctx context.Context, in *requestv1.GeneratePlanRequest, _ ...grpc.CallOption) (*requestv1.GeneratePlanResponse, error) {
	f.rec(ctx, "GeneratePlan", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.GeneratePlanResponse{RunId: "plan-run"}, nil
}

func (f *fakeRequestClient) GetPlanProposal(ctx context.Context, in *requestv1.GetPlanProposalRequest, _ ...grpc.CallOption) (*requestv1.GetPlanProposalResponse, error) {
	f.rec(ctx, "GetPlanProposal", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.GetPlanProposalResponse{RunId: in.GetRunId(), Status: "succeeded", Proposal: &requestv1.PlanProposal{Title: "P", Phases: []*requestv1.PhaseProposal{{Title: "ph", Tasks: []*requestv1.TaskProposal{{Title: "t", TaskType: "task", EstimatedHours: wrapperspb.Double(2)}}}}}}, nil
}

func (f *fakeRequestClient) CleanupRequestProposal(ctx context.Context, in *requestv1.CleanupRequestProposalRequest, _ ...grpc.CallOption) (*requestv1.CleanupRequestProposalResponse, error) {
	f.rec(ctx, "CleanupRequestProposal", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.CleanupRequestProposalResponse{Removed: true, Branch: "request/1-proposal"}, nil
}

func (f *fakeRequestClient) GetProjectEngineSettings(ctx context.Context, in *requestv1.GetProjectEngineSettingsRequest, _ ...grpc.CallOption) (*requestv1.GetProjectEngineSettingsResponse, error) {
	f.rec(ctx, "GetProjectEngineSettings", in)
	if f.err != nil {
		return nil, f.err
	}
	resp := &requestv1.GetProjectEngineSettingsResponse{Settings: &requestv1.ProjectEngineSettings{ProjectId: in.GetProjectId(), SolutionEngine: "native"}}
	if in.GetCheck() {
		resp.Preflight = &requestv1.EnginePreflight{Ran: true, Ok: true, Gaps: []*requestv1.EnginePreflightNote{{Code: "REQUEST_KNOWLEDGE_NOT_INITIALIZED", Message: "m", FixHint: "h"}}}
	}
	return resp, nil
}

func (f *fakeRequestClient) SetProjectEngineSettings(ctx context.Context, in *requestv1.SetProjectEngineSettingsRequest, _ ...grpc.CallOption) (*requestv1.SetProjectEngineSettingsResponse, error) {
	f.rec(ctx, "SetProjectEngineSettings", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.SetProjectEngineSettingsResponse{Settings: &requestv1.ProjectEngineSettings{
		ProjectId: in.GetProjectId(), SolutionEngine: in.GetSolutionEngine(), SolutionAgent: in.GetSolutionAgent(), Version: in.GetExpectedVersion() + 1,
	}}, nil
}

func (f *fakeRequestClient) GetPlanBundleFile(ctx context.Context, in *requestv1.GetPlanBundleFileRequest, _ ...grpc.CallOption) (*requestv1.GetPlanBundleFileResponse, error) {
	f.rec(ctx, "GetPlanBundleFile", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.GetPlanBundleFileResponse{Path: in.GetPath(), Content: "# tasks\n"}, nil
}

func (f *fakeRequestClient) CommitPlan(ctx context.Context, in *requestv1.CommitPlanRequest, _ ...grpc.CallOption) (*requestv1.CommitPlanResponse, error) {
	f.rec(ctx, "CommitPlan", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.CommitPlanResponse{PlanTaskId: "pt", PhaseTaskIds: []string{"ph1"}, TaskIds: nil, AlreadyExists: false}, nil
}

func (f *fakeRequestClient) StartPhase(ctx context.Context, in *requestv1.StartPhaseRequest, _ ...grpc.CallOption) (*requestv1.StartPhaseResponse, error) {
	f.rec(ctx, "StartPhase", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.StartPhaseResponse{PhaseTaskId: "ph1"}, nil
}

func (f *fakeRequestClient) GetRequestFlowSettings(ctx context.Context, in *requestv1.GetRequestFlowSettingsRequest, _ ...grpc.CallOption) (*requestv1.GetRequestFlowSettingsResponse, error) {
	f.rec(ctx, "GetRequestFlowSettings", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.GetRequestFlowSettingsResponse{Enabled: true}, nil
}

func (f *fakeRequestClient) SetRequestFlowSettings(ctx context.Context, in *requestv1.SetRequestFlowSettingsRequest, _ ...grpc.CallOption) (*requestv1.SetRequestFlowSettingsResponse, error) {
	f.rec(ctx, "SetRequestFlowSettings", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.SetRequestFlowSettingsResponse{Enabled: in.GetEnabled()}, nil
}

func (f *fakeRequestClient) ListSolutions(ctx context.Context, in *requestv1.ListSolutionsRequest, _ ...grpc.CallOption) (*requestv1.ListSolutionsResponse, error) {
	f.rec(ctx, "ListSolutions", in)
	if f.err != nil {
		return nil, f.err
	}
	if f.empty {
		return &requestv1.ListSolutionsResponse{}, nil
	}
	return &requestv1.ListSolutionsResponse{
		Solutions: []*requestv1.Solution{{Id: "s1", RequestId: "r1", Kind: requestv1.SolutionKind_SOLUTION_KIND_SOLUTION,
			Status: requestv1.SolutionStatus_SOLUTION_STATUS_PROPOSED, OptionsJson: `{"options":[{"id":"opt-0"},{"id":"opt-1","risk_level":"low"}]}`, ChosenOption: 1, CreatedAt: fakeStamp, Version: 1,
			Seq: 2, RevisionFeedback: "keep the public API"}},
		Runs: []*requestv1.AnalysisRun{{Id: "run1", Kind: requestv1.SolutionKind_SOLUTION_KIND_SOLUTION, Status: "failed", ErrorCode: "REQUEST_SOLUTION_INVALID_OUTPUT", StartedAt: fakeStamp}},
	}, nil
}

func (f *fakeRequestClient) GenerateSolution(ctx context.Context, in *requestv1.GenerateSolutionRequest, _ ...grpc.CallOption) (*requestv1.GenerateSolutionResponse, error) {
	f.rec(ctx, "GenerateSolution", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.GenerateSolutionResponse{SolutionId: "s1", RunId: "run1"}, nil
}

func (f *fakeRequestClient) ChooseSolutionOption(ctx context.Context, in *requestv1.ChooseSolutionOptionRequest, _ ...grpc.CallOption) (*requestv1.ChooseSolutionOptionResponse, error) {
	f.rec(ctx, "ChooseSolutionOption", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.ChooseSolutionOptionResponse{Solution: &requestv1.Solution{Id: "s1", OptionsJson: `{"options":[{"id":"opt-0"}]}`, ChosenOption: 0}, ApprovalDigest: "newdigest"}, nil
}

func (f *fakeRequestClient) ListBacklog(ctx context.Context, in *requestv1.ListBacklogRequest, _ ...grpc.CallOption) (*requestv1.ListBacklogResponse, error) {
	f.rec(ctx, "ListBacklog", in)
	if f.err != nil {
		return nil, f.err
	}
	if f.empty {
		return &requestv1.ListBacklogResponse{}, nil
	}
	grp := &requestv1.BacklogGroup{RequestId: "r1", PlanId: "pl", Tasks: []*requestv1.BacklogTaskRow{{Id: "t1", Title: "x", EstimatedHours: wrapperspb.Double(1.5)}}}
	return &requestv1.ListBacklogResponse{
		RequestRows:   []*requestv1.BacklogRequestRow{{Id: "r1", Number: 7, Title: "T", Stage: "plan", ReturnedAt: fakeStamp}},
		TaskGroups:    []*requestv1.BacklogGroup{grp},
		ExecuteGroups: []*requestv1.BacklogGroup{grp},
		NextPageToken: "bn",
	}, nil
}

func (f *fakeRequestClient) ListRequestLinks(ctx context.Context, in *requestv1.ListRequestLinksRequest, _ ...grpc.CallOption) (*requestv1.ListRequestLinksResponse, error) {
	f.rec(ctx, "ListRequestLinks", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.ListRequestLinksResponse{Children: []*requestv1.RequestLink{{ParentRequestId: "r1", ChildRequestId: "c1", Reason: "escalation"}}}, nil
}

func (f *fakeRequestClient) ListRequestActivity(ctx context.Context, in *requestv1.ListRequestActivityRequest, _ ...grpc.CallOption) (*requestv1.ListRequestActivityResponse, error) {
	f.rec(ctx, "ListRequestActivity", in)
	if f.err != nil {
		return nil, f.err
	}
	if f.empty {
		return &requestv1.ListRequestActivityResponse{}, nil
	}
	return &requestv1.ListRequestActivityResponse{
		NextPageToken: "tok2",
		Entries: []*requestv1.ActivityEntry{{
			Id: "o1", Kind: "task.failed", OccurredAt: timestamppb.New(time.Date(2026, 10, 15, 10, 56, 29, 0, time.UTC)), ActorKind: "agent",
			SubjectType: "task", SubjectId: "t1", Summary: "Run the e2e acceptance failed (timed_out)", Attempt: 2, Severity: "error",
			DetailsJson: `{"cause":"timed_out","attempt":2}`,
		}, {Id: "o2", Kind: "request.created", OccurredAt: timestamppb.New(time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)), ActorKind: "user", Severity: "info", DetailsJson: "not json"}},
	}, nil
}

func (f *fakeRequestClient) GetExecutionSummary(ctx context.Context, in *requestv1.GetExecutionSummaryRequest, _ ...grpc.CallOption) (*requestv1.GetExecutionSummaryResponse, error) {
	f.rec(ctx, "GetExecutionSummary", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.GetExecutionSummaryResponse{Summary: &requestv1.ExecutionSummary{
		RequestId: in.GetRequestId(), Status: "request_backlog", Engine: "openspec", ExecutionMode: "continuous", MaxAttempts: 2,
		Progress: &requestv1.ExecutionProgress{PhasesTotal: 7, PhasesDone: 6, TasksTotal: 28, TasksDone: 27, TasksFailed: 1},
		Failed: []*requestv1.ExecutionFailedTask{{TaskId: "t1", Title: "Run the e2e acceptance", Attempts: 2, MaxAttempts: 2, LastCause: "timed_out"}},
		NeedsYou: []*requestv1.ExecutionNeedsYou{{Kind: "failed_task", RefId: "t1", Title: "Run the e2e acceptance", Since: timestamppb.New(time.Date(2026, 10, 15, 10, 56, 29, 0, time.UTC))}},
	}}, nil
}

func (f *fakeRequestClient) GetRequestFlow(ctx context.Context, in *requestv1.GetRequestFlowRequest, _ ...grpc.CallOption) (*requestv1.GetRequestFlowResponse, error) {
	f.rec(ctx, "GetRequestFlow", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.GetRequestFlowResponse{Type: in.GetType(), HasPhases: true, StatusPath: []string{"new", "analyzing"}}, nil
}

func (f *fakeRequestClient) ListRequestChecks(ctx context.Context, in *requestv1.ListRequestChecksRequest, _ ...grpc.CallOption) (*requestv1.ListRequestChecksResponse, error) {
	f.rec(ctx, "ListRequestChecks", in)
	if f.err != nil {
		return nil, f.err
	}
	if f.empty {
		return &requestv1.ListRequestChecksResponse{}, nil
	}
	return &requestv1.ListRequestChecksResponse{Checks: []*requestv1.RequestCheck{{Id: "k1", RequestId: "r1", Kind: "perf", MetricsJson: `{"p95_ms":12}`, CreatedAt: fakeStamp}}}, nil
}

func (f *fakeRequestClient) GetApproval(ctx context.Context, in *requestv1.GetApprovalRequest, _ ...grpc.CallOption) (*requestv1.GetApprovalResponse, error) {
	f.rec(ctx, "GetApproval", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.GetApprovalResponse{Approval: fakeApprovalMsg()}, nil
}

func (f *fakeRequestClient) ListApprovals(ctx context.Context, in *requestv1.ListApprovalsRequest, _ ...grpc.CallOption) (*requestv1.ListApprovalsResponse, error) {
	f.rec(ctx, "ListApprovals", in)
	if f.err != nil {
		return nil, f.err
	}
	if f.empty {
		return &requestv1.ListApprovalsResponse{}, nil
	}
	return &requestv1.ListApprovalsResponse{Approvals: []*requestv1.Approval{fakeApprovalMsg()}}, nil
}

func (f *fakeRequestClient) ListPendingForUser(ctx context.Context, in *requestv1.ListPendingForUserRequest, _ ...grpc.CallOption) (*requestv1.ListPendingForUserResponse, error) {
	f.rec(ctx, "ListPendingForUser", in)
	if f.err != nil {
		return nil, f.err
	}
	if f.empty {
		return &requestv1.ListPendingForUserResponse{}, nil
	}
	return &requestv1.ListPendingForUserResponse{Approvals: []*requestv1.Approval{fakeApprovalMsg()}, NextPageToken: "ap"}, nil
}

func (f *fakeRequestClient) Approve(ctx context.Context, in *requestv1.ApproveRequest, _ ...grpc.CallOption) (*requestv1.ApproveResponse, error) {
	f.rec(ctx, "Approve", in)
	if f.err != nil {
		return nil, f.err
	}
	// The double echoes the requested mode as the pinned one, like a Request that had none pinned.
	return &requestv1.ApproveResponse{Approval: fakeApprovalMsg(), RequestStatus: "planning", ExecutionMode: in.GetExecutionMode()}, nil
}

func (f *fakeRequestClient) Reject(ctx context.Context, in *requestv1.RejectRequest, _ ...grpc.CallOption) (*requestv1.RejectResponse, error) {
	f.rec(ctx, "Reject", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.RejectResponse{Approval: fakeApprovalMsg(), RequestStatus: "analyzing"}, nil
}

func (f *fakeRequestClient) Cancel(ctx context.Context, in *requestv1.ApprovalServiceCancelRequest, _ ...grpc.CallOption) (*requestv1.ApprovalServiceCancelResponse, error) {
	f.rec(ctx, "Cancel", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.ApprovalServiceCancelResponse{Approval: fakeApprovalMsg()}, nil
}

// The Outcome Review RPCs. They answer what the channel tests below need and record the request like every other RPC.

func (f *fakeRequestClient) EstimateOutcomeReview(ctx context.Context, in *requestv1.EstimateOutcomeReviewRequest, _ ...grpc.CallOption) (*requestv1.EstimateOutcomeReviewResponse, error) {
	f.rec(ctx, "EstimateOutcomeReview", in)
	if f.err != nil {
		return nil, f.err
	}
	out := &requestv1.EstimateOutcomeReviewResponse{Phase: "design", ReviewExists: true, ReviewId: "rv1", CurrentInputsDigest: "d1", ContextRunnable: true}
	if in.GetScope() != "" {
		out.Regeneration = &requestv1.RegenerationEstimate{AgentRuns: 2, DiagramsToDraw: 3, CostKnown: true, CostUsd: 0.52, ConfirmRequired: true}
	}
	return out, nil
}

func (f *fakeRequestClient) GenerateOutcomeReview(ctx context.Context, in *requestv1.GenerateOutcomeReviewRequest, _ ...grpc.CallOption) (*requestv1.GenerateOutcomeReviewResponse, error) {
	f.rec(ctx, "GenerateOutcomeReview", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.GenerateOutcomeReviewResponse{
		Review:  &requestv1.OutcomeReview{ReviewId: "rv1", RequestId: in.GetRequestId(), Phase: "design", State: "ready", ModelJson: `{"schemaVersion":1,"items":[]}`, Version: 1},
		Created: true, Run: &requestv1.OutcomeReviewRun{RunId: "run1", Kind: "context", Status: "running"}, DrawingsPending: 2,
	}, nil
}

func (f *fakeRequestClient) GetOutcomeReview(ctx context.Context, in *requestv1.GetOutcomeReviewRequest, _ ...grpc.CallOption) (*requestv1.GetOutcomeReviewResponse, error) {
	f.rec(ctx, "GetOutcomeReview", in)
	if f.err != nil {
		return nil, f.err
	}
	if f.empty {
		return &requestv1.GetOutcomeReviewResponse{}, nil
	}
	return &requestv1.GetOutcomeReviewResponse{Found: true, Review: &requestv1.OutcomeReview{ReviewId: "rv1", ModelJson: `not json`, ApprovedBy: "u1"}}, nil
}

func (f *fakeRequestClient) ListOutcomeReviews(ctx context.Context, in *requestv1.ListOutcomeReviewsRequest, _ ...grpc.CallOption) (*requestv1.ListOutcomeReviewsResponse, error) {
	f.rec(ctx, "ListOutcomeReviews", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.ListOutcomeReviewsResponse{Reviews: []*requestv1.OutcomeReview{{ReviewId: "rv1", ModelJson: `{}`}}, NextPageToken: "rv1"}, nil
}

func (f *fakeRequestClient) ListOutcomeReviewRuns(ctx context.Context, in *requestv1.ListOutcomeReviewRunsRequest, _ ...grpc.CallOption) (*requestv1.ListOutcomeReviewRunsResponse, error) {
	f.rec(ctx, "ListOutcomeReviewRuns", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.ListOutcomeReviewRunsResponse{Runs: []*requestv1.OutcomeReviewRun{{RunId: "run1", Kind: "draw", Status: "succeeded", CostActual: "0.1000 USD"}}}, nil
}

func (f *fakeRequestClient) GetReviewArtifact(ctx context.Context, in *requestv1.GetReviewArtifactRequest, _ ...grpc.CallOption) (*requestv1.GetReviewArtifactResponse, error) {
	f.rec(ctx, "GetReviewArtifact", in)
	if f.err != nil {
		return nil, f.err
	}
	out := &requestv1.GetReviewArtifactResponse{Artifact: &requestv1.ReviewArtifactMeta{ArtifactId: in.GetArtifactId(), Section: "flow", Variant: "design", Revision: 2, BlobDigest: "bd", ViewBox: "0 0 800 400"}}
	if in.GetIfDigest() == "bd" {
		out.NotModified = true
		return out, nil
	}
	out.MediaType, out.Content = "text/html", "<svg></svg>"
	return out, nil
}

func (f *fakeRequestClient) ListReviewArtifactRevisions(ctx context.Context, in *requestv1.ListReviewArtifactRevisionsRequest, _ ...grpc.CallOption) (*requestv1.ListReviewArtifactRevisionsResponse, error) {
	f.rec(ctx, "ListReviewArtifactRevisions", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.ListReviewArtifactRevisionsResponse{Revisions: []*requestv1.ReviewArtifactMeta{{ArtifactId: "a2", Revision: 2, State: "active"}, {ArtifactId: "a1", Revision: 1, State: "superseded"}}}, nil
}

func (f *fakeRequestClient) RegenerateReviewArtifacts(ctx context.Context, in *requestv1.RegenerateReviewArtifactsRequest, _ ...grpc.CallOption) (*requestv1.RegenerateReviewArtifactsResponse, error) {
	f.rec(ctx, "RegenerateReviewArtifacts", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.RegenerateReviewArtifactsResponse{DrawRun: &requestv1.OutcomeReviewRun{RunId: "run2", Kind: "draw", Status: "running"}, Replayed: in.GetClientRequestId() == "again"}, nil
}

func (f *fakeRequestClient) RestoreReviewArtifactRevision(ctx context.Context, in *requestv1.RestoreReviewArtifactRevisionRequest, _ ...grpc.CallOption) (*requestv1.RestoreReviewArtifactRevisionResponse, error) {
	f.rec(ctx, "RestoreReviewArtifactRevision", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.RestoreReviewArtifactRevisionResponse{Artifact: &requestv1.ReviewArtifactMeta{ArtifactId: in.GetArtifactId(), State: "active"}, HasDiff: true, Diff: &requestv1.ModelDiff{AddedNodes: []string{"n"}}}, nil
}

func (f *fakeRequestClient) CompareReviewArtifactRevisions(ctx context.Context, in *requestv1.CompareReviewArtifactRevisionsRequest, _ ...grpc.CallOption) (*requestv1.CompareReviewArtifactRevisionsResponse, error) {
	f.rec(ctx, "CompareReviewArtifactRevisions", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.CompareReviewArtifactRevisionsResponse{Diff: &requestv1.ModelDiff{}}, nil
}

func (f *fakeRequestClient) DecideOutcomeReviewItem(ctx context.Context, in *requestv1.DecideOutcomeReviewItemRequest, _ ...grpc.CallOption) (*requestv1.DecideOutcomeReviewItemResponse, error) {
	f.rec(ctx, "DecideOutcomeReviewItem", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.DecideOutcomeReviewItemResponse{Decision: &requestv1.OutcomeReviewDecision{DecisionId: "d1", ItemKey: in.GetItemKey(), Decision: in.GetDecision(), Active: true}, ReviewVersion: in.GetExpectedVersion() + 1, ReviewState: "ready", OpenDecisions: 1}, nil
}

func (f *fakeRequestClient) ApproveOutcomeReview(ctx context.Context, in *requestv1.ApproveOutcomeReviewRequest, _ ...grpc.CallOption) (*requestv1.ApproveOutcomeReviewResponse, error) {
	f.rec(ctx, "ApproveOutcomeReview", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.ApproveOutcomeReviewResponse{Review: &requestv1.OutcomeReview{ReviewId: in.GetReviewId(), State: "approved", ModelJson: `{}`, ApprovedBy: "u1", ApprovalNote: in.GetNote()}}, nil
}

func (f *fakeRequestClient) ListOutcomeReviewDecisions(ctx context.Context, in *requestv1.ListOutcomeReviewDecisionsRequest, _ ...grpc.CallOption) (*requestv1.ListOutcomeReviewDecisionsResponse, error) {
	f.rec(ctx, "ListOutcomeReviewDecisions", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.ListOutcomeReviewDecisionsResponse{Decisions: []*requestv1.OutcomeReviewDecision{{DecisionId: "d1", ItemKey: "k", Decision: "send_back", Active: true, Orphaned: true}}, OpenDecisions: 2}, nil
}

func (f *fakeRequestClient) LinkOutcomeReviewChild(ctx context.Context, in *requestv1.LinkOutcomeReviewChildRequest, _ ...grpc.CallOption) (*requestv1.LinkOutcomeReviewChildResponse, error) {
	f.rec(ctx, "LinkOutcomeReviewChild", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.LinkOutcomeReviewChildResponse{Linked: int32(len(in.GetItemKeys()))}, nil
}

func (f *fakeRequestClient) GetReviewDrawingProfile(ctx context.Context, in *requestv1.GetReviewDrawingProfileRequest, _ ...grpc.CallOption) (*requestv1.GetReviewDrawingProfileResponse, error) {
	f.rec(ctx, "GetReviewDrawingProfile", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.GetReviewDrawingProfileResponse{
		HasProjectLayer: in.GetProjectId() != "", ProjectLayer: &requestv1.ReviewDrawingProfileLayer{ProjectId: in.GetProjectId(), SpecJson: `{"schema":"review-drawing-profile/1"}`, Version: 3},
		EffectiveJson: `{"style":{"detail":"balanced"}}`, Sources: map[string]string{"style.detail": "project"},
	}, nil
}

func (f *fakeRequestClient) SaveReviewDrawingProfile(ctx context.Context, in *requestv1.SaveReviewDrawingProfileRequest, _ ...grpc.CallOption) (*requestv1.SaveReviewDrawingProfileResponse, error) {
	f.rec(ctx, "SaveReviewDrawingProfile", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.SaveReviewDrawingProfileResponse{Layer: &requestv1.ReviewDrawingProfileLayer{ProjectId: in.GetProjectId(), SpecJson: in.GetSpecJson(), Version: in.GetExpectedVersion() + 1}}, nil
}

func (f *fakeRequestClient) ResetReviewDrawingProfile(ctx context.Context, in *requestv1.ResetReviewDrawingProfileRequest, _ ...grpc.CallOption) (*requestv1.ResetReviewDrawingProfileResponse, error) {
	f.rec(ctx, "ResetReviewDrawingProfile", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.ResetReviewDrawingProfileResponse{}, nil
}

func (f *fakeRequestClient) GetTestSpecs(ctx context.Context, in *requestv1.GetTestSpecsRequest, _ ...grpc.CallOption) (*requestv1.GetTestSpecsResponse, error) {
	f.rec(ctx, "GetTestSpecs", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.GetTestSpecsResponse{RequestId: in.GetRequestId(), Enabled: true, Current: &requestv1.TestSpecRun{
		Id: "ts1", Attempt: 1, Status: "completed", ContextMode: "solution", ContextRef: "REQ-1", Feature: "TDD-NOTES",
		SummaryJson: `{"feature":"TDD-NOTES"}`,
	}}, nil
}

func (f *fakeRequestClient) StartTestSpecs(ctx context.Context, in *requestv1.StartTestSpecsRequest, _ ...grpc.CallOption) (*requestv1.StartTestSpecsResponse, error) {
	f.rec(ctx, "StartTestSpecs", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.StartTestSpecsResponse{Run: &requestv1.TestSpecRun{Id: "ts2", Attempt: 2, Status: "starting"}}, nil
}

func (f *fakeRequestClient) GetTestSpecDocument(ctx context.Context, in *requestv1.GetTestSpecDocumentRequest, _ ...grpc.CallOption) (*requestv1.GetTestSpecDocumentResponse, error) {
	f.rec(ctx, "GetTestSpecDocument", in)
	if f.err != nil {
		return nil, f.err
	}
	return &requestv1.GetTestSpecDocumentResponse{Document: in.GetDocument(), Version: 1, Status: "Draft", Length: 4, Text: "TC-1"}, nil
}
