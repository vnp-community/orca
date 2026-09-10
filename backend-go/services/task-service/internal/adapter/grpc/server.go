// Package grpc implements the generated taskv1.TaskServiceServer interface
// by translating wire messages to/from usecase calls — no business logic
// here, per specs/backend-go/architecture/03-clean-architecture-guidelines.md's
// inbound-adapter contract.
package grpc

import (
	"context"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"

	taskv1 "github.com/stablyai/orca-go/proto/gen/go/orca/task/v1"
)

// Server implements taskv1.UnimplementedTaskServiceServer.
type Server struct {
	taskv1.UnimplementedTaskServiceServer

	createTask            *usecase.CreateTask
	getTask               *usecase.GetTask
	txRunner              usecase.TxRunner
	grant                 *usecase.Grant
	resolvePermission     *usecase.ResolvePermission
	executeTask           *usecase.ExecuteTask
	hasActiveExecutions   *usecase.HasActiveExecutions
	listTasks             *usecase.ListTasks
	updateTask            *usecase.UpdateTask
	deleteTask            *usecase.DeleteTask
	getDependencies       *usecase.GetDependencies
	aiDecompose           *usecase.AIDecompose
	aiApply               *usecase.AIApply
	generateAgentPrompt   *usecase.GenerateAgentPrompt
	revokeGrant           *usecase.RevokeGrant
	listGrants            *usecase.ListGrants
	createPublicLink      *usecase.CreatePublicLink
	revokePublicLink      *usecase.RevokePublicLink
	resolvePublicLink     *usecase.ResolvePublicLink
	getSubtree            *usecase.GetSubtree
	recalculateProgress   *usecase.RecalculateProgress
	addComment            *usecase.AddComment
	listComments          *usecase.ListComments
	reportExecutionResult *usecase.ReportTaskExecutionResult
	findTaskByNumber      *usecase.FindTaskByNumber
	// generateShareLink/getTaskByShareToken back TASK-TG-003-05's
	// GenerateShareLink/GetTaskByShareToken RPCs — a second,
	// independently-built share-link mechanism kept deliberately side by
	// side with CreatePublicLink/ResolvePublicLink above (see task.proto's
	// RPC doc comment for why).
	generateShareLink   *usecase.GenerateShareLink
	getTaskByShareToken *usecase.GetTaskByShareToken
}

func New(
	createTask *usecase.CreateTask,
	getTask *usecase.GetTask,
	txRunner usecase.TxRunner,
	grant *usecase.Grant,
	resolvePermission *usecase.ResolvePermission,
	executeTask *usecase.ExecuteTask,
	hasActiveExecutions *usecase.HasActiveExecutions,
	listTasks *usecase.ListTasks,
	updateTask *usecase.UpdateTask,
	deleteTask *usecase.DeleteTask,
	getDependencies *usecase.GetDependencies,
	aiDecompose *usecase.AIDecompose,
	aiApply *usecase.AIApply,
	generateAgentPrompt *usecase.GenerateAgentPrompt,
	revokeGrant *usecase.RevokeGrant,
	listGrants *usecase.ListGrants,
	createPublicLink *usecase.CreatePublicLink,
	revokePublicLink *usecase.RevokePublicLink,
	resolvePublicLink *usecase.ResolvePublicLink,
	getSubtree *usecase.GetSubtree,
	recalculateProgress *usecase.RecalculateProgress,
	addComment *usecase.AddComment,
	listComments *usecase.ListComments,
	reportExecutionResult *usecase.ReportTaskExecutionResult,
	findTaskByNumber *usecase.FindTaskByNumber,
	generateShareLink *usecase.GenerateShareLink,
	getTaskByShareToken *usecase.GetTaskByShareToken,
) *Server {
	return &Server{
		createTask:            createTask,
		getTask:               getTask,
		txRunner:              txRunner,
		grant:                 grant,
		resolvePermission:     resolvePermission,
		executeTask:           executeTask,
		hasActiveExecutions:   hasActiveExecutions,
		listTasks:             listTasks,
		updateTask:            updateTask,
		deleteTask:            deleteTask,
		getDependencies:       getDependencies,
		aiDecompose:           aiDecompose,
		aiApply:               aiApply,
		generateAgentPrompt:   generateAgentPrompt,
		revokeGrant:           revokeGrant,
		listGrants:            listGrants,
		createPublicLink:      createPublicLink,
		revokePublicLink:      revokePublicLink,
		resolvePublicLink:     resolvePublicLink,
		getSubtree:            getSubtree,
		recalculateProgress:   recalculateProgress,
		addComment:            addComment,
		listComments:          listComments,
		reportExecutionResult: reportExecutionResult,
		findTaskByNumber:      findTaskByNumber,
		generateShareLink:     generateShareLink,
		getTaskByShareToken:   getTaskByShareToken,
	}
}

func (s *Server) GenerateAgentPrompt(ctx context.Context, req *taskv1.GenerateAgentPromptRequest) (*taskv1.GenerateAgentPromptResponse, error) {
	prompt, err := s.generateAgentPrompt.Execute(ctx, usecase.GenerateAgentPromptInput{TaskID: req.GetTaskId(), Save: req.GetSave()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.GenerateAgentPromptResponse{Prompt: prompt}, nil
}

func (s *Server) CreateTask(ctx context.Context, req *taskv1.CreateTaskRequest) (*taskv1.CreateTaskResponse, error) {
	// No ID field on CreateTaskRequest — the usecase assigns one (uuid) when
	// Input.ID is left empty.
	task, err := s.createTask.Execute(ctx, usecase.CreateTaskInput{
		Title:     req.GetTitle(),
		ParentID:  req.GetParentId(),
		ProjectID: req.GetProjectId(),
		CreatorID: req.GetCreatorId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.CreateTaskResponse{Task: toProtoTask(task)}, nil
}

func (s *Server) GetTask(ctx context.Context, req *taskv1.GetTaskRequest) (*taskv1.GetTaskResponse, error) {
	task, err := s.getTask.Execute(ctx, req.GetId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.GetTaskResponse{Task: toProtoTask(task)}, nil
}

// AddEdge wraps the cycle-check + edge-write + auto-block sequence in one
// Postgres transaction via TxRunner — closing add_edge.go's previously
// admitted check-then-write race (see AddEdge usecase's doc comment) rather
// than constructing a single pool-scoped *usecase.AddEdge once at
// server-startup wiring time.
func (s *Server) AddEdge(ctx context.Context, req *taskv1.AddEdgeRequest) (*taskv1.AddEdgeResponse, error) {
	err := s.txRunner.RunInTx(ctx, func(ctx context.Context, tasks usecase.TaskRepository, edges usecase.EdgeRepository) error {
		_, err := usecase.NewAddEdge(tasks, edges).Execute(ctx, usecase.AddEdgeInput{
			FromTaskID: req.GetFromTaskId(),
			ToTaskID:   req.GetToTaskId(),
			Kind:       toDomainEdgeKind(req.GetType()),
		})
		return err
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.AddEdgeResponse{}, nil
}

func (s *Server) Grant(ctx context.Context, req *taskv1.GrantRequest) (*taskv1.GrantResponse, error) {
	in := usecase.GrantInput{
		TaskID:    req.GetTaskId(),
		SubjectID: req.GetSubjectId(),
		Level:     toDomainGrantLevel(req.GetLevel()),
		ApplyTree: req.GetApplyTree(),
	}
	if req.GetExpiresAt() != nil {
		t := req.GetExpiresAt().AsTime()
		in.ExpiresAt = &t
	}
	id, err := s.grant.Execute(ctx, in)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.GrantResponse{Id: id}, nil
}

func (s *Server) RevokeGrant(ctx context.Context, req *taskv1.RevokeGrantRequest) (*emptypb.Empty, error) {
	err := s.revokeGrant.Execute(ctx, usecase.RevokeGrantInput{
		TaskID:    req.GetTaskId(),
		SubjectID: req.GetSubjectId(),
		Level:     toDomainGrantLevel(req.GetLevel()),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) ListGrants(ctx context.Context, req *taskv1.ListGrantsRequest) (*taskv1.ListGrantsResponse, error) {
	grants, err := s.listGrants.Execute(ctx, req.GetTaskId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*taskv1.GrantView, 0, len(grants))
	for _, g := range grants {
		view := &taskv1.GrantView{SubjectId: g.SubjectID, Level: toProtoGrantLevel(g.Level), ApplyTree: g.ApplyTree}
		if g.ExpiresAt != nil {
			view.ExpiresAt = timestamppb.New(*g.ExpiresAt)
		}
		out = append(out, view)
	}
	return &taskv1.ListGrantsResponse{Grants: out}, nil
}

func (s *Server) CreatePublicLink(ctx context.Context, req *taskv1.CreatePublicLinkRequest) (*taskv1.CreatePublicLinkResponse, error) {
	id, token, err := s.createPublicLink.Execute(ctx, req.GetTaskId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.CreatePublicLinkResponse{Id: id, Token: token}, nil
}

func (s *Server) RevokePublicLink(ctx context.Context, req *taskv1.RevokePublicLinkRequest) (*emptypb.Empty, error) {
	if err := s.revokePublicLink.Execute(ctx, req.GetId()); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

// ResolvePublicLink is the one RPC in this service meaningfully callable
// without a JWT (spec: "anonymous read-only access via a random token") —
// see TASK-TG-03-08's Context section for why api-gateway is NOT yet wired
// to expose this to a browser (a new unauthenticated-route trust boundary,
// out of scope here) and why tenantID below still comes from
// tenant.RequireTenantID(ctx) rather than the wire request (which has no
// tenant_id field) in the meantime.
func (s *Server) ResolvePublicLink(ctx context.Context, req *taskv1.ResolvePublicLinkRequest) (*taskv1.ResolvePublicLinkResponse, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(apperrors.New(apperrors.KindUnauthenticated, "TASK_NO_TENANT", "no tenant in request context", err))
	}
	taskID, err := s.resolvePublicLink.Execute(ctx, tenantID, req.GetToken())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.ResolvePublicLinkResponse{TaskId: taskID}, nil
}

func (s *Server) ResolvePermission(ctx context.Context, req *taskv1.ResolvePermissionRequest) (*taskv1.ResolvePermissionResponse, error) {
	// action defaults to "read" when empty — either an older client built
	// against a pre-TASK-TG-003-06 proto that has no action field to send,
	// or a caller that legitimately wants the default. The usecase's own
	// input contract stays strict ("empty means empty"); this adapter layer
	// absorbs the rollout-compatibility shim, matching this codebase's
	// existing adapter-absorbs-wire-quirks convention.
	action := req.GetAction()
	if action == "" {
		action = "read"
	}
	level, err := s.resolvePermission.Execute(ctx, usecase.ResolvePermissionInput{
		TaskID: req.GetTaskId(),
		UserID: req.GetUserId(),
		Action: action,
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.ResolvePermissionResponse{EffectiveLevel: toProtoGrantLevel(level)}, nil
}

func (s *Server) Execute(ctx context.Context, req *taskv1.TaskServiceExecuteRequest) (*taskv1.TaskServiceExecuteResponse, error) {
	result, err := s.executeTask.Execute(ctx, usecase.ExecuteTaskInput{
		TaskID:    req.GetTaskId(),
		RequestID: req.GetRequestId(),
		Prompt:    req.GetPrompt(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.TaskServiceExecuteResponse{ExecutionRef: result.ExecutionRef, Async: result.Async}, nil
}

func (s *Server) HasActiveExecutions(ctx context.Context, req *taskv1.HasActiveExecutionsRequest) (*taskv1.HasActiveExecutionsResponse, error) {
	hasActive, err := s.hasActiveExecutions.Execute(ctx, usecase.HasActiveExecutionsInput{ProjectID: req.GetProjectId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.HasActiveExecutionsResponse{HasActive: hasActive}, nil
}

func (s *Server) ListTasks(ctx context.Context, req *taskv1.ListTasksRequest) (*taskv1.ListTasksResponse, error) {
	result, err := s.listTasks.Execute(ctx, usecase.ListTasksInput{
		ProjectID: req.GetProjectId(),
		PageToken: req.GetPageToken(),
		PageSize:  req.GetPageSize(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*taskv1.Task, 0, len(result.Tasks))
	for _, t := range result.Tasks {
		out = append(out, toProtoTask(t))
	}
	return &taskv1.ListTasksResponse{Tasks: out, NextPageToken: result.NextPageToken}, nil
}

func (s *Server) UpdateTask(ctx context.Context, req *taskv1.UpdateTaskRequest) (*taskv1.UpdateTaskResponse, error) {
	in := usecase.UpdateTaskInput{ID: req.GetId()}
	if req.GetTitle() != nil {
		v := req.GetTitle().GetValue()
		in.Title = &v
	}
	if req.GetStatus() != nil {
		v := domain.Status(req.GetStatus().GetValue())
		in.Status = &v
	}
	if req.GetPrUrl() != nil {
		v := req.GetPrUrl().GetValue()
		in.PRURL = &v
	}
	if req.GetWorktreeId() != nil {
		v := req.GetWorktreeId().GetValue()
		in.WorktreeID = &v
	}
	if req.GetWorkflowTemplateId() != nil {
		v := req.GetWorkflowTemplateId().GetValue()
		in.WorkflowTemplateID = &v
	}
	task, err := s.updateTask.Execute(ctx, in)
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.UpdateTaskResponse{Task: toProtoTask(task)}, nil
}

func (s *Server) DeleteTask(ctx context.Context, req *taskv1.DeleteTaskRequest) (*emptypb.Empty, error) {
	if err := s.deleteTask.Execute(ctx, usecase.DeleteTaskInput{ID: req.GetId()}); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) GetDependencies(ctx context.Context, req *taskv1.GetDependenciesRequest) (*taskv1.GetDependenciesResponse, error) {
	deps, err := s.getDependencies.Execute(ctx, usecase.GetDependenciesInput{TaskID: req.GetTaskId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*taskv1.Task, 0, len(deps))
	for _, t := range deps {
		out = append(out, toProtoTask(t))
	}
	return &taskv1.GetDependenciesResponse{Dependencies: out}, nil
}

func (s *Server) AIDecompose(ctx context.Context, req *taskv1.AIDecomposeRequest) (*taskv1.AIDecomposeResponse, error) {
	result, err := s.aiDecompose.Execute(ctx, usecase.AIDecomposeInput{TaskID: req.GetTaskId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.AIDecomposeResponse{
		Proposals:   toProtoSubtaskProposals(result.Proposals),
		RawResponse: result.RawResponse,
	}, nil
}

func (s *Server) AIApply(ctx context.Context, req *taskv1.AIApplyRequest) (*taskv1.AIApplyResponse, error) {
	created, err := s.aiApply.Execute(ctx, usecase.AIApplyInput{
		TaskID:        req.GetTaskId(),
		Proposals:     toDomainSubtaskProposals(req.GetProposals()),
		RawAIResponse: req.GetRawAiResponse(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*taskv1.Task, 0, len(created))
	for _, t := range created {
		out = append(out, toProtoTask(t))
	}
	return &taskv1.AIApplyResponse{CreatedSubtasks: out}, nil
}

func (s *Server) GetSubtree(ctx context.Context, req *taskv1.GetSubtreeRequest) (*taskv1.GetSubtreeResponse, error) {
	result, err := s.getSubtree.Execute(ctx, usecase.GetSubtreeInput{RootID: req.GetRootId()})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*taskv1.Task, 0, len(result.Tasks))
	for _, t := range result.Tasks {
		out = append(out, toProtoTask(t))
	}
	edges := make([]*taskv1.AddEdgeRequest, 0, len(result.DependsOnEdges))
	for _, e := range result.DependsOnEdges {
		edges = append(edges, &taskv1.AddEdgeRequest{FromTaskId: e.FromTaskID, ToTaskId: e.ToTaskID, Type: taskv1.EdgeType_EDGE_TYPE_DEPENDS_ON})
	}
	return &taskv1.GetSubtreeResponse{Tasks: out, DependsOnEdges: edges}, nil
}

func (s *Server) RecalculateProgress(ctx context.Context, req *taskv1.RecalculateProgressRequest) (*taskv1.RecalculateProgressResponse, error) {
	p, err := s.recalculateProgress.Execute(ctx, req.GetRootId())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.RecalculateProgressResponse{ProgressPercent: int32(p)}, nil
}

func (s *Server) AddComment(ctx context.Context, req *taskv1.AddCommentRequest) (*taskv1.AddCommentResponse, error) {
	c, err := s.addComment.Execute(ctx, req.GetTaskId(), req.GetContent())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.AddCommentResponse{Id: c.ID, AuthorId: c.AuthorID, Content: c.Content, CreatedAt: c.CreatedAt.Format(time.RFC3339)}, nil
}

func (s *Server) ListComments(ctx context.Context, req *taskv1.ListCommentsRequest) (*taskv1.ListCommentsResponse, error) {
	comments, next, err := s.listComments.Execute(ctx, req.GetTaskId(), req.GetPageToken(), req.GetPageSize())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	out := make([]*taskv1.AddCommentResponse, 0, len(comments))
	for _, c := range comments {
		out = append(out, &taskv1.AddCommentResponse{Id: c.ID, AuthorId: c.AuthorID, Content: c.Content, CreatedAt: c.CreatedAt.Format(time.RFC3339)})
	}
	return &taskv1.ListCommentsResponse{Comments: out, NextPageToken: next}, nil
}

// ReportTaskExecutionResult is orchestration-service/workflow-service's
// shared inbound completion callback (TASK-FT-002-04, generalizing
// SOL-TG-04/TASK-TG-04-05's Engine-2-only design — see task.proto's RPC
// doc comment).
//
// NOTE (open gap, not a placeholder guard asserting something false): this
// codebase's common/grpcmw has no service-identity/mTLS interceptor today
// (only TenantExtractionInterceptor/RecoveryInterceptor/LoggingInterceptor
// exist) — so, unlike this RPC's doc comment's stated intent ("api-gateway
// never routes to it"), nothing yet actually enforces that only
// orchestration-service/workflow-service can call this RPC at the mesh
// level. Flagged rather than asserting a check that doesn't exist.
func (s *Server) ReportTaskExecutionResult(ctx context.Context, req *taskv1.ReportTaskExecutionResultRequest) (*emptypb.Empty, error) {
	if err := s.reportExecutionResult.Execute(ctx, usecase.ReportTaskExecutionResultInput{
		TaskID:       req.GetTaskId(),
		ExecutionRef: req.GetExecutionRef(),
		Success:      req.GetSuccess(),
		ActualHours:  req.GetActualHours(),
		ErrorMessage: req.GetErrorMessage(),
		Engine:       req.GetEngine(),
	}); err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) FindTaskByNumber(ctx context.Context, req *taskv1.FindTaskByNumberRequest) (*taskv1.FindTaskByNumberResponse, error) {
	task, err := s.findTaskByNumber.Execute(ctx, usecase.FindTaskByNumberInput{
		ProjectID: req.GetProjectId(), TaskNumber: req.GetTaskNumber(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.FindTaskByNumberResponse{Task: toProtoTask(task)}, nil
}

// GenerateShareLink requires the caller to already hold admin-level
// permission (enforced inside the usecase, not here) — TASK-TG-003-05.
func (s *Server) GenerateShareLink(ctx context.Context, req *taskv1.GenerateShareLinkRequest) (*taskv1.GenerateShareLinkResponse, error) {
	token, err := s.generateShareLink.Execute(ctx, usecase.GenerateShareLinkInput{
		TaskID: req.GetTaskId(),
		UserID: req.GetUserId(),
	})
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.GenerateShareLinkResponse{ShareToken: token}, nil
}

// GetTaskByShareToken is task-service's first unauthenticated RPC —
// TASK-TG-003-05, SECURITY REVIEW REQUIRED before merge. Deliberately does
// NOT call anything from common/tenant — see usecase.GetTaskByShareToken's
// doc comment. Confirmed reachable without any special allowlisting:
// common/grpcmw.ChainUnary's TenantExtractionInterceptor
// (common/grpcmw/grpcmw.go:50-66) only OPTIONALLY populates tenant/user
// context from incoming metadata when present — it never rejects a call
// for missing metadata, so there is no default-deny gate at the gRPC layer
// for ANY RPC in this service today; the actual "auth" enforcement is
// exclusively each usecase's own tenant.RequireTenantID call. Since this
// usecase deliberately never calls that, it's reachable with zero metadata
// exactly like every other RPC's transport-level reachability — the only
// thing making it meaningfully different is that it's SAFE to reach that
// way, unlike every other RPC. Whether task-service's gRPC port itself is
// reachable from outside the internal mesh (bypassing api-gateway's own
// auth) is a deployment-topology question the security reviewer should
// confirm explicitly, not something resolved by this handler.
func (s *Server) GetTaskByShareToken(ctx context.Context, req *taskv1.GetTaskByShareTokenRequest) (*taskv1.GetTaskByShareTokenResponse, error) {
	view, err := s.getTaskByShareToken.Execute(ctx, req.GetShareToken())
	if err != nil {
		return nil, apperrors.ToGRPCStatus(err)
	}
	return &taskv1.GetTaskByShareTokenResponse{Task: &taskv1.TaskShareView{
		Id:          view.ID,
		Title:       view.Title,
		Status:      view.Status,
		Description: view.Description,
	}}, nil
}

func toProtoSubtaskProposals(proposals []domain.SubtaskProposal) []*taskv1.SubtaskProposal {
	out := make([]*taskv1.SubtaskProposal, 0, len(proposals))
	for _, p := range proposals {
		wire := &taskv1.SubtaskProposal{
			Title: p.Title, Description: p.Description, Type: p.Type,
			DependsOnIndices: toInt32Slice(p.DependsOnIndices), PromptTemplate: p.PromptTemplate,
		}
		if p.EstimatedHours != nil {
			wire.EstimatedHours = wrapperspb.Double(*p.EstimatedHours)
		}
		out = append(out, wire)
	}
	return out
}

func toDomainSubtaskProposals(proposals []*taskv1.SubtaskProposal) []domain.SubtaskProposal {
	out := make([]domain.SubtaskProposal, 0, len(proposals))
	for _, p := range proposals {
		proposal := domain.SubtaskProposal{
			Title: p.GetTitle(), Description: p.GetDescription(), Type: p.GetType(),
			DependsOnIndices: toIntSlice(p.GetDependsOnIndices()), PromptTemplate: p.GetPromptTemplate(),
		}
		if p.GetEstimatedHours() != nil {
			v := p.GetEstimatedHours().GetValue()
			proposal.EstimatedHours = &v
		}
		out = append(out, proposal)
	}
	return out
}

func toInt32Slice(in []int) []int32 {
	out := make([]int32, len(in))
	for i, v := range in {
		out[i] = int32(v)
	}
	return out
}

func toIntSlice(in []int32) []int {
	out := make([]int, len(in))
	for i, v := range in {
		out[i] = int(v)
	}
	return out
}

func toDomainEdgeKind(t taskv1.EdgeType) domain.EdgeKind {
	switch t {
	case taskv1.EdgeType_EDGE_TYPE_PARENT_CHILD:
		return domain.EdgeKindParentChild
	case taskv1.EdgeType_EDGE_TYPE_DEPENDS_ON:
		return domain.EdgeKindDependsOn
	default:
		return ""
	}
}

func toDomainGrantLevel(l taskv1.GrantLevel) domain.GrantLevel {
	switch l {
	case taskv1.GrantLevel_GRANT_LEVEL_OWNER:
		return domain.GrantLevelOwner
	case taskv1.GrantLevel_GRANT_LEVEL_ADMIN:
		return domain.GrantLevelAdmin
	case taskv1.GrantLevel_GRANT_LEVEL_USER:
		return domain.GrantLevelUser
	case taskv1.GrantLevel_GRANT_LEVEL_TEAM:
		return domain.GrantLevelTeam
	case taskv1.GrantLevel_GRANT_LEVEL_COMPANY:
		return domain.GrantLevelCompany
	default:
		return domain.GrantLevelUnspecified
	}
}

func toProtoGrantLevel(l domain.GrantLevel) taskv1.GrantLevel {
	switch l {
	case domain.GrantLevelOwner:
		return taskv1.GrantLevel_GRANT_LEVEL_OWNER
	case domain.GrantLevelAdmin:
		return taskv1.GrantLevel_GRANT_LEVEL_ADMIN
	case domain.GrantLevelUser:
		return taskv1.GrantLevel_GRANT_LEVEL_USER
	case domain.GrantLevelTeam:
		return taskv1.GrantLevel_GRANT_LEVEL_TEAM
	case domain.GrantLevelCompany:
		return taskv1.GrantLevel_GRANT_LEVEL_COMPANY
	default:
		return taskv1.GrantLevel_GRANT_LEVEL_UNSPECIFIED
	}
}

// timePtrToProto converts a nullable domain time field to the wire's
// google.protobuf.Timestamp — nil maps to an unset (nil) field.
func timePtrToProto(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

// float64PtrToDoubleValue converts a nullable domain float field to the
// wire's google.protobuf.DoubleValue wrapper — nil maps to an unset (nil)
// field, distinguishable from an explicit 0 on the wire.
func float64PtrToDoubleValue(f *float64) *wrapperspb.DoubleValue {
	if f == nil {
		return nil
	}
	return wrapperspb.Double(*f)
}

func toProtoTask(t domain.Task) *taskv1.Task {
	return &taskv1.Task{
		Id:                 t.ID,
		TenantId:           t.TenantID,
		Title:              t.Title,
		Status:             string(t.Status),
		ParentId:           t.ParentID,
		ProjectId:          t.ProjectID,
		Description:        t.Description,
		TaskType:           t.Type,
		Priority:           t.Priority,
		AssigneeId:         t.AssigneeID,
		OwnerId:            t.OwnerID,
		DueDate:            timePtrToProto(t.DueDate),
		EstimatedHours:     float64PtrToDoubleValue(t.EstimatedHours),
		ActualHours:        float64PtrToDoubleValue(t.ActualHours),
		PromptTemplate:     t.PromptTemplate,
		AiContext:          t.AIContext,
		AiPlanJson:         t.AIPlanJSON,
		Visibility:         t.Visibility,
		WorktreeId:         t.WorktreeID,
		AgentSessionId:     t.AgentSessionID,
		ProgressPercent:    int32(t.ProgressPercent),
		TaskNumber:         t.TaskNumber,
		PrUrl:              t.PRURL,
		WorkflowTemplateId: t.WorkflowTemplateID,
		Labels:             t.Labels,
		ReporterId:         t.ReporterID,
		WorkflowExecId:     t.WorkflowExecID,
		DoneSubtasks:       int32(t.DoneSubtasks),
		TotalSubtasks:      int32(t.TotalSubtasks),
		ShareToken:         t.ShareToken,
	}
}
